package filename

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/easayliu/alist-aria2-download/internal/application/contracts"
	"github.com/easayliu/alist-aria2-download/pkg/logger"
)

// BatchFileNameRequest 批量文件名请求
type BatchFileNameRequest struct {
	Files []FileNameRequest // 文件列表

	// 上下文共享(可选,提高效率)
	SharedContext *SharedContext
}

// SharedContext 共享上下文(用于批量处理优化)
type SharedContext struct {
	ShowName  string // 剧集名(TV)
	Season    *int   // 季度(TV)
	MediaType string // tv, movie
}

// BatchFileNameSuggestion 批量建议结果
type BatchFileNameSuggestion struct {
	OriginalName string              `json:"original_name"` // 原始文件名
	Suggestion   *FileNameSuggestion `json:"suggestion"`    // 推断结果
	Error        string              `json:"error"`         // 错误信息(如果失败)
}

// BatchSuggestFileNames 批量推断文件名
// 智能批量处理:
// 1. 自动分批(仅基于Token限制,无硬性数量限制)
// 2. 智能季度分组(小批次自动合并)
// 3. 自适应并发
func (s *LLMSuggester) BatchSuggestFileNames(
	ctx context.Context,
	req BatchFileNameRequest,
) ([]BatchFileNameSuggestion, error) {
	logger.Info("Batch LLM inference started",
		"fileCount", len(req.Files),
		"tokenLimit", s.batchConfig.TokenLimit)

	if len(req.Files) == 0 {
		return []BatchFileNameSuggestion{}, nil
	}

	// 智能分批(仅基于Token限制)
	batches := s.smartBatchFiles(req.Files)

	// 自适应并发数(根据批次数量动态调整)
	maxConcurrent := s.calculateOptimalConcurrency(len(batches))

	logger.Info("File batching completed",
		"totalBatches", len(batches),
		"concurrency", maxConcurrent)

	// 并发处理批次
	results := s.processBatchesConcurrently(ctx, batches, req.SharedContext, maxConcurrent)

	logger.Info("Batch LLM inference completed",
		"totalFiles", len(req.Files),
		"successCount", countSuccessful(results))

	return results, nil
}

// smartBatchFiles 智能分批(新版,仅基于Token限制)
// 策略:
// 1. 按季度初步分组
// 2. 小批次智能合并(如S01:2个 + S02:3个 → 合并为1批)
// 3. 大批次按Token动态分割
func (s *LLMSuggester) smartBatchFiles(files []FileNameRequest) [][]FileNameRequest {
	tokenLimit := s.batchConfig.TokenLimit

	// 按季度分组
	seasonGroups := make(map[int][]FileNameRequest)
	for _, f := range files {
		season := extractSeasonFromPath(f.FilePath)
		seasonGroups[season] = append(seasonGroups[season], f)
	}

	// 对每个季度组估算Token
	type seasonBatch struct {
		season       int
		files        []FileNameRequest
		estimatedTokens int
	}

	var seasonBatches []seasonBatch
	for season, group := range seasonGroups {
		tokens := s.batchConfig.BaseTokens
		for _, file := range group {
			tokens += s.estimateFileTokens(file)
		}
		seasonBatches = append(seasonBatches, seasonBatch{
			season:       season,
			files:        group,
			estimatedTokens: tokens,
		})
	}

	// 智能合并策略
	var finalBatches [][]FileNameRequest
	currentBatch := []FileNameRequest{}
	currentTokens := s.batchConfig.BaseTokens

	for _, sb := range seasonBatches {
		// 检查是否可以合并到当前批次
		if len(currentBatch) == 0 {
			// 空批次,直接添加
			currentBatch = sb.files
			currentTokens = sb.estimatedTokens
		} else if currentTokens+sb.estimatedTokens-s.batchConfig.BaseTokens <= tokenLimit {
			// Token充足,合并跨季度批次
			currentBatch = append(currentBatch, sb.files...)
			currentTokens = currentTokens + sb.estimatedTokens - s.batchConfig.BaseTokens
			logger.Debug("Cross-season batch merge",
				"season", sb.season,
				"addedFiles", len(sb.files),
				"totalFiles", len(currentBatch),
				"totalTokens", currentTokens)
		} else {
			// Token不足,保存当前批次
			if len(currentBatch) > 0 {
				finalBatches = append(finalBatches, currentBatch)
			}
			// 检查当前季度组是否需要拆分
			if sb.estimatedTokens > tokenLimit {
				// 需要拆分
				splitBatches := s.splitLargeBatch(sb.files, tokenLimit)
				finalBatches = append(finalBatches, splitBatches...)
				currentBatch = []FileNameRequest{}
				currentTokens = s.batchConfig.BaseTokens
			} else {
				// 作为新批次
				currentBatch = sb.files
				currentTokens = sb.estimatedTokens
			}
		}
	}

	// 添加最后一个批次
	if len(currentBatch) > 0 {
		finalBatches = append(finalBatches, currentBatch)
	}

	return finalBatches
}

// splitLargeBatch 拆分超大批次
func (s *LLMSuggester) splitLargeBatch(files []FileNameRequest, tokenLimit int) [][]FileNameRequest {
	var batches [][]FileNameRequest
	currentBatch := []FileNameRequest{}
	estimatedTokens := s.batchConfig.BaseTokens

	for _, file := range files {
		fileTokens := s.estimateFileTokens(file)

		if estimatedTokens+fileTokens > tokenLimit && len(currentBatch) > 0 {
			batches = append(batches, currentBatch)
			currentBatch = []FileNameRequest{}
			estimatedTokens = s.batchConfig.BaseTokens
		}

		currentBatch = append(currentBatch, file)
		estimatedTokens += fileTokens
	}

	if len(currentBatch) > 0 {
		batches = append(batches, currentBatch)
	}

	return batches
}

// calculateOptimalConcurrency 计算最优并发数
func (s *LLMSuggester) calculateOptimalConcurrency(batchCount int) int {
	if batchCount == 0 {
		return 1
	}
	if batchCount == 1 {
		return 1
	}
	if batchCount <= 3 {
		return batchCount
	}
	// 批次较多时,使用配置的最大值
	return s.batchConfig.MaxConcurrentBatches
}

// batchFiles 智能分批(旧版,保留兼容)
// 策略: 按季度分组,每组内再按批次大小分割
func (s *LLMSuggester) batchFiles(
	files []FileNameRequest,
	batchSize int,
	tokenLimit int,
) [][]FileNameRequest {
	// 按季度分组(优化:同季度文件共享更多上下文)
	seasonGroups := make(map[int][]FileNameRequest)
	for _, f := range files {
		season := extractSeasonFromPath(f.FilePath)
		seasonGroups[season] = append(seasonGroups[season], f)
	}

	var batches [][]FileNameRequest

	// 对每个季度的文件进行分批
	for _, group := range seasonGroups {
		currentBatch := []FileNameRequest{}
		estimatedTokens := s.batchConfig.BaseTokens // 从配置读取基础Token数

		for _, file := range group {
			// 估算添加这个文件需要的Token数
			fileTokens := s.estimateFileTokens(file)

			// 如果添加会超限,开始新批次
			if estimatedTokens+fileTokens > tokenLimit && len(currentBatch) > 0 {
				batches = append(batches, currentBatch)
				currentBatch = []FileNameRequest{}
				estimatedTokens = s.batchConfig.BaseTokens
			}

			// 添加文件到当前批次
			currentBatch = append(currentBatch, file)
			estimatedTokens += fileTokens

			// 达到批次大小限制
			if len(currentBatch) >= batchSize {
				batches = append(batches, currentBatch)
				currentBatch = []FileNameRequest{}
				estimatedTokens = s.batchConfig.BaseTokens
			}
		}

		// 添加最后一个批次
		if len(currentBatch) > 0 {
			batches = append(batches, currentBatch)
		}
	}

	return batches
}

// estimateFileTokens 估算单个文件需要的Token数
func (s *LLMSuggester) estimateFileTokens(file FileNameRequest) int {
	// 粗略估算: 1个字符 ≈ 1.3 tokens (英文为主)
	// 中文字符 ≈ 2-3 tokens
	charsCount := len(file.OriginalName) + len(file.FilePath)

	// 考虑中文字符
	chineseCount := countChinese(file.OriginalName)
	tokens := int(float64(charsCount-chineseCount)*1.3 + float64(chineseCount)*2.5)

	// 加上JSON输出的Token数(约150 tokens/文件)
	return tokens + 150
}

// processBatchesConcurrently 并发处理多个批次
func (s *LLMSuggester) processBatchesConcurrently(
	ctx context.Context,
	batches [][]FileNameRequest,
	sharedCtx *SharedContext,
	maxConcurrent int,
) []BatchFileNameSuggestion {
	var (
		results []BatchFileNameSuggestion
		mu      sync.Mutex
		wg      sync.WaitGroup
		sem     = make(chan struct{}, maxConcurrent)
	)

	for batchIdx, batch := range batches {
		wg.Add(1)
		sem <- struct{}{} // 获取信号量

		go func(idx int, b []FileNameRequest) {
			defer func() {
				<-sem // 释放信号量
				wg.Done()
			}()

			logger.Info("Processing batch", "batchIndex", idx, "fileCount", len(b))

			// 处理单个批次
			batchResults, err := s.processSingleBatch(ctx, b, sharedCtx)
			if err != nil {
				logger.Error("Batch processing failed", "batchIndex", idx, "error", err)
				// 失败时返回错误结果
				for _, file := range b {
					batchResults = append(batchResults, BatchFileNameSuggestion{
						OriginalName: filepath.Base(file.OriginalName),
						Error:        err.Error(),
					})
				}
			}

			// 追加到结果
			mu.Lock()
			results = append(results, batchResults...)
			mu.Unlock()

		}(batchIdx, batch)
	}

	wg.Wait()
	return results
}

// processSingleBatch 处理单个批次
func (s *LLMSuggester) processSingleBatch(
	ctx context.Context,
	batch []FileNameRequest,
	sharedCtx *SharedContext,
) ([]BatchFileNameSuggestion, error) {
	// 构建批量Prompt
	prompt := s.buildBatchPrompt(batch, sharedCtx)

	// 记录请求信息
	fileNames := make([]string, 0, len(batch))
	for _, f := range batch {
		fileNames = append(fileNames, f.OriginalName)
	}
	logger.Info("LLM batch request started",
		"fileCount", len(batch),
		"files", fileNames)
	logger.Debug("LLM batch prompt", "prompt", prompt)

	// 定义输出结构
	type BatchOutput struct {
		Results []struct {
			OriginalName  string  `json:"original_name"`
			MediaType     string  `json:"media_type"`
			Title         string  `json:"title"`
			TitleCN       string  `json:"title_cn"`
			Year          *int    `json:"year"`          // 改为指针类型,允许null/空
			Season        *int    `json:"season"`
			Episode       *int    `json:"episode"`
			EpisodeTitle  string  `json:"episode_title"`
			NewFileName   string  `json:"new_file_name"`   // LLM生成的新文件名
			DirectoryPath string  `json:"directory_path"`  // LLM生成的目录路径
			Confidence    float32 `json:"confidence"`
		} `json:"results"`
	}

	var output BatchOutput

	// 调用LLM生成结构化输出
	// 动态计算max_tokens: 每个文件约250 tokens (考虑中文和episode_title)
	// 基础overhead 1000 tokens (包括JSON结构、换行等)
	maxTokens := len(batch)*250 + 1000

	// 设置合理的上下限
	if maxTokens < 2000 {
		maxTokens = 2000 // 最小2000
	}

	// 根据模型能力设置上限
	// doubao-1.5-pro-32k: 输出可达20k tokens
	// gpt-4: 输出通常8k-16k tokens
	// 保守设置为20000,为响应头等预留空间
	if maxTokens > 20000 {
		maxTokens = 20000
	}

	logger.Debug("LLM batch request parameters",
		"fileCount", len(batch),
		"estimatedTokens", maxTokens)

	err := s.llmService.GenerateStructured(ctx, prompt, &output,
		contracts.WithLLMTemperature(0.3),
		contracts.WithLLMMaxTokens(maxTokens))

	if err != nil {
		logger.Error("LLM batch request failed", "error", err, "fileCount", len(batch))
		return nil, fmt.Errorf("LLM批量生成失败: %w", err)
	}

	// 记录响应信息
	logger.Info("LLM batch response received",
		"requestedFiles", len(batch),
		"returnedResults", len(output.Results))

	// 检查返回结果数量
	if len(output.Results) < len(batch) {
		logger.Warn("LLM returned fewer results than requested",
			"requested", len(batch),
			"returned", len(output.Results),
			"missing", len(batch)-len(output.Results))
	}

	// 创建一个map记录已处理的文件
	processedFiles := make(map[string]bool)

	// 记录详细的响应结果
	for i, r := range output.Results {
		// 安全地解引用指针类型
		year := 0
		if r.Year != nil {
			year = *r.Year
		}
		season := 0
		if r.Season != nil {
			season = *r.Season
		}
		episode := 0
		if r.Episode != nil {
			episode = *r.Episode
		}

		logger.Debug("LLM result detail",
			"index", i,
			"originalName", r.OriginalName,
			"mediaType", r.MediaType,
			"title", r.Title,
			"titleCN", r.TitleCN,
			"year", year,
			"season", season,
			"episode", episode,
			"episodeTitle", r.EpisodeTitle,
			"newFileName", r.NewFileName,
			"directoryPath", r.DirectoryPath,
			"confidence", r.Confidence)
	}

	// 转换为BatchFileNameSuggestion
	results := make([]BatchFileNameSuggestion, 0, len(output.Results))
	for _, r := range output.Results {
		// 处理year字段(可能为nil)
		year := 0
		if r.Year != nil {
			year = *r.Year
		}

		suggestion := &FileNameSuggestion{
			MediaType:     r.MediaType,
			Title:         r.Title,
			TitleCN:       r.TitleCN,
			Year:          year,
			Season:        r.Season,
			Episode:       r.Episode,
			EpisodeTitle:  r.EpisodeTitle,
			NewFileName:   r.NewFileName,   // 使用LLM返回的新文件名
			DirectoryPath: r.DirectoryPath, // 使用LLM返回的目录路径
			Confidence:    r.Confidence,
			Source:        "llm_batch",
		}

		// 验证输出(使用统一的简化校验)
		if err := s.validateSuggestion(suggestion); err != nil {
			logger.Warn("Batch inference result validation failed",
				"originalName", r.OriginalName,
				"error", err)
			results = append(results, BatchFileNameSuggestion{
				OriginalName: r.OriginalName,
				Error:        err.Error(),
			})
			continue
		}

		results = append(results, BatchFileNameSuggestion{
			OriginalName: r.OriginalName,
			Suggestion:   suggestion,
		})
		processedFiles[r.OriginalName] = true
	}

	// 检查是否有未处理的文件(LLM没有返回结果的)
	for _, file := range batch {
		if !processedFiles[file.OriginalName] {
			logger.Warn("LLM did not return result for file",
				"fileName", file.OriginalName)
			results = append(results, BatchFileNameSuggestion{
				OriginalName: file.OriginalName,
				Error:        "LLM未返回此文件的结果",
			})
		}
	}

	return results, nil
}

// buildBatchPrompt 构建批量Prompt
func (s *LLMSuggester) buildBatchPrompt(
	batch []FileNameRequest,
	sharedCtx *SharedContext,
) string {
	var prompt strings.Builder

	prompt.WriteString("你是一个专业的媒体文件命名专家。请分析以下**批量文件**，为每个文件提取媒体信息并生成标准的Emby/Plex目录结构路径。\n\n")

	// 上下文信息部分
	if sharedCtx != nil && sharedCtx.ShowName != "" {
		prompt.WriteString("### 上下文信息（适用于所有文件）\n")
		prompt.WriteString(fmt.Sprintf("- 剧集名称: %s\n", sharedCtx.ShowName))
		if sharedCtx.Season != nil {
			prompt.WriteString(fmt.Sprintf("- 季度: %d\n", *sharedCtx.Season))
		}
		if sharedCtx.MediaType != "" {
			prompt.WriteString(fmt.Sprintf("- 媒体类型: %s\n", sharedCtx.MediaType))
		}
		prompt.WriteString("\n")
	}

	// 文件列表
	prompt.WriteString("### 待分析文件列表\n")
	for i, file := range batch {
		if file.FilePath != "" {
			prompt.WriteString(fmt.Sprintf("%d. %s (路径: %s)\n", i+1, file.OriginalName, file.FilePath))
		} else {
			prompt.WriteString(fmt.Sprintf("%d. %s\n", i+1, file.OriginalName))
		}
	}
	prompt.WriteString("\n")

	// 任务要求
	prompt.WriteString("### 任务要求\n")
	prompt.WriteString("请为每个文件输出一个独立的JSON对象，提取并生成以下字段：\n")
	prompt.WriteString("- **original_name**: 原始文件名\n")
	prompt.WriteString("- **media_type**: 识别媒体类型（movie=电影，tv=剧集）\n")
	prompt.WriteString("- **title**: 英文标题（如无则自动转写拼音）\n")
	prompt.WriteString("- **title_cn**: 中文标题\n")
	prompt.WriteString("- **year**: 文件年份（从文件名中提取或推断）\n")
	prompt.WriteString("- **season**: 智能推断季数（规则如下）\n")
	prompt.WriteString("  - 若文件名中包含季度标识（如\"S02\"\"第二季\"），则直接使用\n")
	prompt.WriteString("  - 若无，则根据年份推断（例如2024→第1季，2025→第2季）\n")
	prompt.WriteString("- **episode**: 提取集数编号（规则如下）\n")
	prompt.WriteString("  - 识别\"第01期\"、\"E01\"、\"第1集\"等常见格式\n")
	prompt.WriteString("  - 对于\"上/中/下\"结构，应分配连续的编号（如第11期(上)=11, 第11期(中)=12, 第11期(下)=13）\n")
	prompt.WriteString("  - 按文件整体顺序或日期推断连续编号\n")
	prompt.WriteString("- **episode_title**: 保留完整的集数标题（包含冒号及描述部分）\n")
	prompt.WriteString("- **特殊版本识别**（episode=0）：包含以下关键词的文件需标记为特殊版本\n")
	prompt.WriteString("  - 特辑、番外、精华版、幕后特辑、制作特辑、未播片段、加长版、首映篇、先导片\n")
	prompt.WriteString("  - 超前vlog、花絮、母带放送、收官篇、特别企划、发布会、见面会、陪看记等\n")
	prompt.WriteString("  - 即使含\"第XX期\"也不应计入常规集数\n")
	prompt.WriteString("- **new_file_name**: 生成规范化文件名，格式如下：\n")
	prompt.WriteString("  - 剧集: \"{title_cn} - S{season:02d}E{episode:02d} - {episode_title}.{ext}\"\n")
	prompt.WriteString("  - 特殊版本: episode=0 → \"{title_cn} - S{season:02d}E00 - {episode_title}.{ext}\"\n")
	prompt.WriteString("  - 电影: \"{title_cn} ({year}).{ext}\"\n")
	prompt.WriteString("- **confidence**: 结果置信度 (0.0~1.0)\n\n")

	// 智能规则
	prompt.WriteString("### 智能规则\n")
	prompt.WriteString("1. **目录隔离规则**：不同目录的文件在计算episode编号时必须分开处理\n")
	prompt.WriteString("   - 同一目录下的文件应该按日期/序号连续编号\n")
	prompt.WriteString("   - 不同目录的文件各自独立编号，不要互相影响\n")
	prompt.WriteString("   - 例如：目录A的文件编号1-10，目录B的文件也从1开始编号（或根据文件名中的期数编号）\n")
	prompt.WriteString("2. 若年份为2025且节目类型为综艺，则默认season=2（上一年为第1季）\n")
	prompt.WriteString("3. 文件名中若包含日期，按日期顺序推断播出顺序并编号（仅限同一目录内）\n")
	prompt.WriteString("4. episode_title与new_file_name必须保留完整标题描述，不得省略\n")
	prompt.WriteString("5. 对于上/中/下集，必须使用连续编号，不能共用同一episode编号\n\n")

	// 输出格式
	prompt.WriteString("### 输出格式\n")
	prompt.WriteString("统一使用JSON数组格式，只输出**纯JSON**结果，不得包含额外说明或文字：\n")
	prompt.WriteString("```json\n")
	prompt.WriteString("{\n")
	prompt.WriteString("  \"results\": [\n")
	prompt.WriteString("    {\n")
	prompt.WriteString("      \"original_name\": \"2025-09-19 先导1：节目介绍.mp4\",\n")
	prompt.WriteString("      \"media_type\": \"tv\",\n")
	prompt.WriteString("      \"title\": \"Show Title\",\n")
	prompt.WriteString("      \"title_cn\": \"节目名称\",\n")
	prompt.WriteString("      \"year\": 2025,\n")
	prompt.WriteString("      \"season\": 2,\n")
	prompt.WriteString("      \"episode\": 0,\n")
	prompt.WriteString("      \"episode_title\": \"先导1：节目介绍\",\n")
	prompt.WriteString("      \"new_file_name\": \"节目名称 - S02E00 - 先导1：节目介绍.mp4\",\n")
	prompt.WriteString("      \"confidence\": 0.95\n")
	prompt.WriteString("    }\n")
	prompt.WriteString("  ]\n")
	prompt.WriteString("}\n")
	prompt.WriteString("```\n")

	return prompt.String()
}

// 辅助函数

// extractSeasonFromPath 从路径提取季度
func extractSeasonFromPath(path string) int {
	parts := strings.Split(path, "/")
	for _, part := range parts {
		// 匹配 S01, Season 01 等
		part = strings.ToLower(part)
		if strings.HasPrefix(part, "s") && len(part) >= 2 {
			if season := parseInt(part[1:]); season > 0 && season < 100 {
				return season
			}
		}
		if strings.HasPrefix(part, "season") {
			if season := parseInt(strings.TrimPrefix(part, "season")); season > 0 {
				return season
			}
		}
	}
	return 1 // 默认第一季
}

// parseInt 安全解析整数
func parseInt(s string) int {
	s = strings.TrimSpace(s)
	var num int
	fmt.Sscanf(s, "%d", &num)
	return num
}

// countChinese 统计中文字符数
func countChinese(s string) int {
	count := 0
	for _, r := range s {
		if r >= 0x4E00 && r <= 0x9FFF {
			count++
		}
	}
	return count
}

// countSuccessful 统计成功的数量
func countSuccessful(results []BatchFileNameSuggestion) int {
	count := 0
	for _, r := range results {
		if r.Suggestion != nil && r.Error == "" {
			count++
		}
	}
	return count
}
