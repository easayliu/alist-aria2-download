package filename

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/easayliu/alist-aria2-download/internal/application/contracts"
	"github.com/easayliu/alist-aria2-download/internal/domain/models/rename"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/config"
	"github.com/easayliu/alist-aria2-download/pkg/logger"
)

// LLMSuggester LLM文件名推断器
// 使用大语言模型推断媒体文件的标题、年份、季集数等信息
type LLMSuggester struct {
	llmService  contracts.LLMService
	batchConfig *config.LLMBatchConfig
}

// NewLLMSuggester 创建LLM推断器
func NewLLMSuggester(llmService contracts.LLMService, batchConfig *config.LLMBatchConfig) *LLMSuggester {
	// 设置默认值
	if batchConfig == nil {
		batchConfig = &config.LLMBatchConfig{
			BatchSize:            8,
			TokenLimit:           4000,
			MaxConcurrentBatches: 3,
			BaseTokens:           300,
			EnableSeasonGrouping: true,
		}
	}

	// 校验配置并设置默认值
	if batchConfig.BatchSize <= 0 {
		batchConfig.BatchSize = 8
	}
	if batchConfig.TokenLimit <= 0 {
		batchConfig.TokenLimit = 4000
	}
	if batchConfig.MaxConcurrentBatches <= 0 {
		batchConfig.MaxConcurrentBatches = 3
	}
	if batchConfig.BaseTokens <= 0 {
		batchConfig.BaseTokens = 300
	}

	return &LLMSuggester{
		llmService:  llmService,
		batchConfig: batchConfig,
	}
}

// FileNameRequest 文件名请求
type FileNameRequest struct {
	OriginalName string // 原始文件名
	FilePath     string // 文件路径（可选，提供更多上下文）
	Hint         string // 用户提示（可选）
}

// FileNameSuggestion 文件名建议
type FileNameSuggestion struct {
	MediaType    string `json:"media_type"`    // tv 或 movie
	Title        string `json:"title"`         // 英文标题
	TitleCN      string `json:"title_cn"`      // 中文标题
	Year         int    `json:"year"`          // 年份
	Season       *int   `json:"season"`        // 季度（仅剧集）
	Episode      *int   `json:"episode"`       // 集数（仅剧集）
	EpisodeTitle string `json:"episode_title"` // 集数标题（如果有）

	// 路径相关字段（由LLM直接生成完整路径）
	NewFileName   string `json:"new_file_name"`  // 新文件名（如: "Breaking Bad - S01E01.mkv"）
	DirectoryPath string `json:"directory_path"` // 目录路径（如: "/TVs/Breaking Bad/Season 01"）

	Confidence  float32 `json:"confidence"`   // 置信度 (0.0-1.0)
	RawResponse string  `json:"raw_response"` // LLM原始响应（调试用）
	Source      string  `json:"source"`       // 数据来源：llm
}

// ToEmbyFormat 转换为Emby命名格式
// 电影: Title (Year).ext
// 剧集: Title - S01E01 - Episode Name.ext (如果有集数标题)
// 剧集: Title - S01E01.ext (如果没有集数标题)
// 特殊版本: Title - S01E00 - Special Name.ext (episode=0表示特辑等)
func (s *FileNameSuggestion) ToEmbyFormat(extension string) string {
	if s.MediaType == "movie" {
		return fmt.Sprintf("%s (%d)%s", s.Title, s.Year, extension)
	}

	// 剧集格式
	season := 1
	episode := 1
	if s.Season != nil {
		season = *s.Season
	}
	if s.Episode != nil {
		episode = *s.Episode
	}

	title := s.Title
	if s.TitleCN != "" {
		title = s.TitleCN
	}

	// 如果有集数标题，包含在文件名中
	// episode=0时，通常会有集数标题说明这是什么特别版本
	if s.EpisodeTitle != "" {
		return fmt.Sprintf("%s - S%02dE%02d - %s%s", title, season, episode, s.EpisodeTitle, extension)
	}

	return fmt.Sprintf("%s - S%02dE%02d%s", title, season, episode, extension)
}

// SuggestFileName 推断文件名
func (s *LLMSuggester) SuggestFileName(ctx context.Context, req FileNameRequest) (*rename.Suggestion, error) {
	logger.Info("LLM filename inference started",
		"originalName", req.OriginalName,
		"filePath", req.FilePath,
		"hint", req.Hint)

	// 构建prompt
	prompt := s.buildPrompt(req)

	// 创建用于接收结果的结构体(临时用于JSON解析)
	var llmOutput FileNameSuggestion

	// 调用LLM生成结构化输出（新接口）
	err := s.llmService.GenerateStructured(ctx, prompt, &llmOutput,
		contracts.WithLLMTemperature(0.3), // 较低温度以保证准确性
		contracts.WithLLMMaxTokens(500))   // 限制token数
	if err != nil {
		logger.Error("LLM generation failed", "error", err)
		return nil, fmt.Errorf("LLM generation failed: %w", err)
	}

	// 验证输出合理性
	if err := s.validateSuggestion(&llmOutput); err != nil {
		logger.Warn("LLM output validation failed", "error", err, "suggestion", llmOutput)
		return nil, fmt.Errorf("LLM output validation failed: %w", err)
	}

	// 转换为统一的rename.Suggestion模型
	suggestion := s.convertToRenameSuggestion(req, &llmOutput)

	logger.Info("LLM filename inference succeeded",
		"mediaType", suggestion.MediaType,
		"title", suggestion.Title,
		"titleCN", suggestion.TitleCN,
		"year", suggestion.Year,
		"confidence", suggestion.Confidence)

	return suggestion, nil
}

// buildPrompt 构建LLM prompt
func (s *LLMSuggester) buildPrompt(req FileNameRequest) string {
	var prompt strings.Builder

	prompt.WriteString("你是一个专业的媒体文件命名专家。请分析以下文件名，提取媒体信息并生成标准的Emby/Plex目录结构路径。\n\n")
	prompt.WriteString("**要求**：\n")
	prompt.WriteString("1. 识别媒体类型（movie=电影，tv=剧集）\n")
	prompt.WriteString("2. 提取英文标题和中文标题（如果有）\n")
	prompt.WriteString("3. 提取年份\n")
	prompt.WriteString("4. 如果是剧集，提取季数(season)和集数(episode)\n")
	prompt.WriteString("5. 如果文件名包含集数标题（如 \"S01E02 - 集数标题\"），提取episode_title字段\n")
	prompt.WriteString("6. **特殊版本识别**：识别以下类型为特殊版本（episode设为0）而非常规剧集：\n")
	prompt.WriteString("   - 特辑、番外、精华版、幕后特辑、制作特辑\n")
	prompt.WriteString("   - 演唱会、见面会、发布会、粉丝见面会\n")
	prompt.WriteString("   - 花絮、删减片段、未播片段、片场花絮\n")
	prompt.WriteString("   - SP特别篇、OVA、剧场版、总集篇\n")
	prompt.WriteString("   - 导演剪辑版、未删减版、加长版\n")
	prompt.WriteString("   - 加更版、首映篇、特别企划、收官篇、先导片\n")
	prompt.WriteString("7. 生成标准的新文件名和目录路径\n")
	prompt.WriteString("8. 评估置信度（0.0-1.0）\n\n")

	prompt.WriteString("**命名规则**：\n")
	prompt.WriteString("- 剧集文件名: \"剧名 - S01E01.ext\" 或 \"剧名 - S01E01 - 集标题.ext\"\n")
	prompt.WriteString("- 特殊版本文件名: \"剧名 - S01E00 - 特辑名称.ext\" (episode设为0)\n")
	prompt.WriteString("- 剧集目录: \"/TVs/剧名/Season 01\" (注意：Season后面是两位数字，目录名不包含年份)\n")
	prompt.WriteString("- 电影文件名: \"片名 (年份).ext\"\n")
	prompt.WriteString("- 电影目录: \"/Movies/片名\" (目录名不包含年份)\n\n")

	prompt.WriteString("**输出格式**：严格的JSON格式\n")
	prompt.WriteString("```json\n")
	prompt.WriteString("{\n")
	prompt.WriteString("  \"media_type\": \"movie\" 或 \"tv\",\n")
	prompt.WriteString("  \"title\": \"英文标题\",\n")
	prompt.WriteString("  \"title_cn\": \"中文标题（可选）\",\n")
	prompt.WriteString("  \"year\": 年份数字,\n")
	prompt.WriteString("  \"season\": 季度数字（仅剧集，可为null）,\n")
	prompt.WriteString("  \"episode\": 集数数字（仅剧集，可为null）,\n")
	prompt.WriteString("  \"episode_title\": \"集数标题（可选）\",\n")
	prompt.WriteString("  \"new_file_name\": \"新文件名（含扩展名）\",\n")
	prompt.WriteString("  \"directory_path\": \"目录路径（如 /TVs/剧名/Season 01 或 /Movies/片名）\",\n")
	prompt.WriteString("  \"confidence\": 0.0到1.0之间的数字\n")
	prompt.WriteString("}\n")
	prompt.WriteString("```\n\n")

	prompt.WriteString(fmt.Sprintf("**文件名**: %s\n", req.OriginalName))

	if req.FilePath != "" {
		prompt.WriteString(fmt.Sprintf("**文件路径**: %s\n", req.FilePath))
	}

	if req.Hint != "" {
		prompt.WriteString(fmt.Sprintf("**用户提示**: %s\n", req.Hint))
	}

	prompt.WriteString("\n请分析并以JSON格式输出结果。只输出JSON，不要有任何额外的说明。")

	return prompt.String()
}

// parseResponse 解析LLM响应
func (s *LLMSuggester) parseResponse(rawResponse string) (*FileNameSuggestion, error) {
	// 清理响应（移除可能的markdown代码块标记）
	cleaned := s.cleanJSONResponse(rawResponse)

	var suggestion FileNameSuggestion
	if err := json.Unmarshal([]byte(cleaned), &suggestion); err != nil {
		return nil, fmt.Errorf("JSON解析失败: %w", err)
	}

	return &suggestion, nil
}

// cleanJSONResponse 清理JSON响应，移除markdown等标记
func (s *LLMSuggester) cleanJSONResponse(response string) string {
	// 移除markdown代码块标记
	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	// 尝试提取JSON对象（如果有额外文本）
	jsonRegex := regexp.MustCompile(`\{[\s\S]*\}`)
	if matches := jsonRegex.FindString(response); matches != "" {
		return matches
	}

	return response
}

// validateSuggestion 验证输出的合理性
// 只校验致命错误,放宽业务规则以提高灵活性
func (s *LLMSuggester) validateSuggestion(suggestion *FileNameSuggestion) error {
	// 1. 验证媒体类型(致命错误)
	if suggestion.MediaType != "movie" && suggestion.MediaType != "tv" {
		return fmt.Errorf("无效的媒体类型: %s，必须是 'movie' 或 'tv'", suggestion.MediaType)
	}

	// 2. 验证标题(致命错误)
	if suggestion.Title == "" && suggestion.TitleCN == "" {
		return fmt.Errorf("标题不能为空")
	}

	// 3. 验证年份(极宽松,只拦截明显错误)
	// 允许0(表示未知年份),1800-2200(覆盖老电影和未来电影)
	if suggestion.Year < 0 || suggestion.Year > 2200 {
		return fmt.Errorf("无效的年份: %d", suggestion.Year)
	}

	// 4. 验证季集数(极宽松,只拦截负数)
	// 允许0(特殊版本),不限制上限(长寿剧集)
	if suggestion.Season != nil && *suggestion.Season < 0 {
		return fmt.Errorf("无效的季数: %d", *suggestion.Season)
	}
	if suggestion.Episode != nil && *suggestion.Episode < 0 {
		return fmt.Errorf("无效的集数: %d", *suggestion.Episode)
	}

	// 5. 验证置信度(必须在0-1范围内)
	if suggestion.Confidence < 0.0 || suggestion.Confidence > 1.0 {
		return fmt.Errorf("无效的置信度: %f", suggestion.Confidence)
	}

	return nil
}

// convertToRenameSuggestion 将LLM输出转换为统一的rename.Suggestion模型
func (s *LLMSuggester) convertToRenameSuggestion(req FileNameRequest, llmOutput *FileNameSuggestion) *rename.Suggestion {
	// 构建新路径
	// 优先使用LLM生成的DirectoryPath，否则保留原始文件的目录
	newPath := req.FilePath
	if llmOutput.DirectoryPath != "" && llmOutput.NewFileName != "" {
		newPath = llmOutput.DirectoryPath + "/" + llmOutput.NewFileName
	} else if llmOutput.NewFileName != "" {
		// 保留原始目录，只替换文件名
		dir := ""
		if req.FilePath != "" {
			lastSlash := strings.LastIndex(req.FilePath, "/")
			if lastSlash >= 0 {
				dir = req.FilePath[:lastSlash+1]
			}
		}
		newPath = dir + llmOutput.NewFileName
	}

	suggestion := &rename.Suggestion{
		OriginalPath: req.FilePath,
		NewName:      llmOutput.NewFileName,
		NewPath:      newPath,
		MediaType:    rename.MediaType(llmOutput.MediaType),
		Title:        llmOutput.Title,
		TitleCN:      llmOutput.TitleCN,
		Year:         llmOutput.Year,
		Season:       llmOutput.Season,
		Episode:      llmOutput.Episode,
		EpisodeTitle: llmOutput.EpisodeTitle,
		Confidence:   float64(llmOutput.Confidence),
		Source:       rename.SourceLLM,
		RawResponse:  llmOutput.RawResponse,
	}

	return suggestion
}
