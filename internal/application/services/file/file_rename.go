package file

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/easayliu/alist-aria2-download/internal/application/contracts"
	"github.com/easayliu/alist-aria2-download/internal/domain/models/rename"
	"github.com/easayliu/alist-aria2-download/internal/domain/services/filename"
	fileutil "github.com/easayliu/alist-aria2-download/pkg/utils/file"
	"github.com/easayliu/alist-aria2-download/pkg/logger"
)

func (s *AppFileService) RenameFile(ctx context.Context, path, newName string) error {
	if s.alistClient == nil {
		return fmt.Errorf("alist client not initialized")
	}

	logger.Debug("Renaming file", "path", path, "newName", newName)

	if err := s.alistClient.RenameWithContext(ctx, path, newName); err != nil {
		logger.Error("Failed to rename file", "path", path, "newName", newName, "error", err)
		return fmt.Errorf("failed to rename file: %w", err)
	}

	logger.Debug("File renamed successfully", "path", path, "newName", newName)
	return nil
}

func (s *AppFileService) RenameAndMoveFile(ctx context.Context, oldPath, newPath string) error {
	return s.renameAndMoveFileInternal(ctx, oldPath, newPath, true)
}

// renameAndMoveFileInternal 内部重命名和移动文件方法
// skipCleanup: 是否跳过目录清理（批量操作时使用）
// 策略: 先Move到目标目录，再在目标目录Rename，减少并发冲突
func (s *AppFileService) renameAndMoveFileInternal(ctx context.Context, oldPath, newPath string, cleanup bool) error {
	if s.alistClient == nil {
		return fmt.Errorf("alist client not initialized")
	}

	logger.Debug("Renaming and moving file", "oldPath", oldPath, "newPath", newPath)

	if oldPath == newPath {
		logger.Info("Paths are the same, skip")
		return nil
	}

	oldDir := filepath.Dir(oldPath)
	newDir := filepath.Dir(newPath)
	fileName := filepath.Base(oldPath)
	newFileName := filepath.Base(newPath)

	// 情况1: 同目录，只需重命名
	if oldDir == newDir {
		if err := s.alistClient.RenameWithContext(ctx, oldPath, newFileName); err != nil {
			logger.Error("Failed to rename file", "oldPath", oldPath, "newFileName", newFileName, "error", err)
			return fmt.Errorf("failed to rename file: %w", err)
		}
		logger.Debug("File renamed successfully", "oldPath", oldPath, "newFileName", newFileName)
		return nil
	}

	// 情况2: 跨目录操作，先Move再Rename（减少并发冲突）
	if err := s.alistClient.Mkdir(ctx, newDir); err != nil {
		logger.Warn("Failed to create directory (may already exist)", "dir", newDir, "error", err)
	}

	// 先移动文件（使用原文件名）
	if err := s.alistClient.Move(ctx, oldDir, newDir, []string{fileName}); err != nil {
		logger.Error("Failed to move file", "srcDir", oldDir, "dstDir", newDir, "fileName", fileName, "error", err)
		return fmt.Errorf("failed to move file: %w", err)
	}

	// 如果需要重命名，在目标目录进行
	if fileName != newFileName {
		movedPath := filepath.Join(newDir, fileName)
		if err := s.alistClient.RenameWithContext(ctx, movedPath, newFileName); err != nil {
			logger.Error("Failed to rename file after move", "movedPath", movedPath, "newFileName", newFileName, "error", err)
			return fmt.Errorf("failed to rename file after move: %w", err)
		}
	}

	// 只有非批量操作时才立即清理目录
	if cleanup {
		if err := s.removeEmptyDirectory(ctx, oldDir); err != nil {
			logger.Warn("Failed to remove old directory", "dir", oldDir, "error", err)
		}
	}

	logger.Debug("File renamed and moved successfully", "oldPath", oldPath, "newPath", newPath)
	return nil
}

// BatchRenameAndMoveFiles 并发批量重命名文件
// 使用信号量模式控制并发数，复用 Alist QPS 配置
func (s *AppFileService) BatchRenameAndMoveFiles(ctx context.Context, tasks []contracts.RenameTask) []contracts.RenameResult {
	if len(tasks) == 0 {
		return []contracts.RenameResult{}
	}

	// 使用 Alist QPS 配置作为最大并发数，默认 10
	maxConcurrent := 10
	if s.config != nil && s.config.Alist.QPS > 0 {
		// 使用 QPS 的一半作为并发数，避免超限
		maxConcurrent = s.config.Alist.QPS / 2
		if maxConcurrent < 1 {
			maxConcurrent = 1
		}
		if maxConcurrent > 20 {
			maxConcurrent = 20
		}
	}

	logger.Info("Starting batch rename",
		"taskCount", len(tasks),
		"maxConcurrent", maxConcurrent)

	var (
		results        = make([]contracts.RenameResult, len(tasks))
		wg             sync.WaitGroup
		sem            = make(chan struct{}, maxConcurrent)
		oldDirsMu      sync.Mutex
		oldDirs        = make(map[string]struct{}) // 收集所有涉及的源目录
		processedCount int32                       // 原子计数器
		startTime      = time.Now()
		lastReportTime = startTime
		lastProgress   float64
	)

	// 启动进度报告协程(智能报告:时间+百分比结合)
	stopProgress := make(chan struct{})
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				current := atomic.LoadInt32(&processedCount)
				progress := float64(current) / float64(len(tasks)) * 100
				elapsed := time.Since(startTime)

				// 每10%或每10秒报告一次(取较晚者)
				if progress-lastProgress >= 10.0 || time.Since(lastReportTime) >= 10*time.Second {
					logger.Info("Batch rename progress",
						"processed", current,
						"total", len(tasks),
						"progress_pct", fmt.Sprintf("%.1f%%", progress),
						"elapsed", elapsed.String())
					lastReportTime = time.Now()
					lastProgress = progress
				}
			case <-stopProgress:
				return
			}
		}
	}()

	for i, task := range tasks {
		wg.Add(1)
		sem <- struct{}{} // 获取信号量

		go func(idx int, t contracts.RenameTask) {
			defer func() {
				atomic.AddInt32(&processedCount, 1) // 更新进度
				<-sem                                // 释放信号量
				wg.Done()
			}()

			// 记录源目录
			oldDir := filepath.Dir(t.OldPath)
			newDir := filepath.Dir(t.NewPath)
			if oldDir != newDir {
				oldDirsMu.Lock()
				oldDirs[oldDir] = struct{}{}
				oldDirsMu.Unlock()
			}

			// 批量操作时跳过单个文件的目录清理，统一在最后清理
			err := s.renameAndMoveFileInternal(ctx, t.OldPath, t.NewPath, false)
			results[idx] = contracts.RenameResult{
				OldPath: t.OldPath,
				NewPath: t.NewPath,
				Success: err == nil,
				Error:   err,
			}

			if err != nil {
				logger.Warn("Rename failed",
					"oldPath", t.OldPath,
					"newPath", t.NewPath,
					"error", err)
			} else {
				logger.Debug("Rename success",
					"oldPath", t.OldPath,
					"newPath", t.NewPath)
			}
		}(i, task)
	}

	wg.Wait()
	close(stopProgress) // 停止进度报告

	// 统计结果
	duration := time.Since(startTime)
	successCount := 0
	for _, r := range results {
		if r.Success {
			successCount++
		}
	}

	// 计算成功率和平均耗时
	successRate := float64(successCount) / float64(len(tasks)) * 100
	avgPerFile := duration / time.Duration(len(tasks))

	logger.Info("Batch rename completed",
		"total", len(tasks),
		"success", successCount,
		"failed", len(tasks)-successCount,
		"success_rate", fmt.Sprintf("%.1f%%", successRate),
		"duration", duration.String(),
		"avg_per_file", avgPerFile.String())

	// 批量重命名完成后，统一清理源目录
	if len(oldDirs) > 0 {
		// 等待 Alist 缓存更新（避免缓存导致的误判）
		logger.Info("Waiting for Alist cache to update before cleanup")
		time.Sleep(2 * time.Second)

		logger.Info("Cleaning up source directories", "dirCount", len(oldDirs))
		for dir := range oldDirs {
			if err := s.removeEmptyDirectory(ctx, dir); err != nil {
				logger.Warn("Failed to remove directory", "dir", dir, "error", err)
			}
		}
	}

	return results
}

// removeEmptyDirectory 移除没有视频文件的目录
// 递归检查目录及其子目录，如果都没有视频文件，则删除整个目录
func (s *AppFileService) removeEmptyDirectory(ctx context.Context, dir string) error {
	hasVideo, err := s.hasVideoFilesRecursive(ctx, dir)
	if err != nil {
		return fmt.Errorf("failed to check directory: %w", err)
	}

	if !hasVideo {
		dirName := filepath.Base(dir)
		parentDir := filepath.Dir(dir)

		if err := s.alistClient.Remove(ctx, parentDir, []string{dirName}); err != nil {
			return fmt.Errorf("failed to remove directory: %w", err)
		}

		logger.Info("Removed directory without video files", "dir", dir)
	} else {
		logger.Debug("Directory has video files, skipping removal", "dir", dir)
	}

	return nil
}

// hasVideoFilesRecursive 递归检查目录及其子目录是否包含视频文件
// 返回 true 表示存在视频文件，false 表示不存在
func (s *AppFileService) hasVideoFilesRecursive(ctx context.Context, dir string) (bool, error) {
	// 列出目录中的所有文件
	listResp, err := s.alistClient.ListFilesWithContext(ctx, dir, 1, 100)
	if err != nil {
		return false, fmt.Errorf("failed to list directory: %w", err)
	}

	var videoFiles []string
	var subDirs []string

	for _, file := range listResp.Data.Content {
		if file.IsDir {
			subDirs = append(subDirs, file.Name)
		} else if s.isVideoFile(file.Name) {
			videoFiles = append(videoFiles, file.Name)
		}
	}

	logger.Debug("Checking directory for videos",
		"dir", dir,
		"videoFiles", len(videoFiles),
		"subDirs", len(subDirs))

	// 如果当前目录有视频文件，验证这些文件是否真实存在（解决时序问题）
	if len(videoFiles) > 0 {
		actualVideoCount := 0
		for _, videoFile := range videoFiles {
			videoPath := filepath.Join(dir, videoFile)

			// 检查文件是否真实存在
			exists, err := s.fileExists(ctx, videoPath)

			// 如果是 Emby 格式文件且验证失败，可能是 Alist 缓存问题
			if s.isEmbyFormatFile(videoFile) && (err != nil || !exists) {
				logger.Warn("Found Emby format file but verification failed, likely Alist cache issue",
					"path", videoPath,
					"fileName", videoFile,
					"exists", exists,
					"error", err)
				// 不计入实际文件数，认为是缓存
				continue
			}

			if err == nil && exists {
				actualVideoCount++
				logger.Debug("Video file exists", "path", videoPath)
			} else {
				logger.Debug("Video file does not exist (already moved)", "path", videoPath, "error", err)
			}
		}

		if actualVideoCount > 0 {
			logger.Debug("Directory has real video files",
				"dir", dir,
				"count", actualVideoCount)
			return true, nil
		}
	}

	// 递归检查子目录
	for _, subDir := range subDirs {
		subDirPath := filepath.Join(dir, subDir)
		hasVideo, err := s.hasVideoFilesRecursive(ctx, subDirPath)
		if err != nil {
			logger.Warn("Failed to check subdirectory",
				"subDir", subDirPath,
				"error", err)
			continue
		}
		if hasVideo {
			logger.Debug("Subdirectory has video files",
				"subDir", subDirPath)
			return true, nil
		}
	}

	logger.Debug("No video files found in directory tree", "dir", dir)
	return false, nil
}

// isEmbyFormatFile 检查文件名是否为 Emby/Plex 格式
// 格式：剧名 - S01E01 - 标题.ext 或 剧名 - S01E01.ext
func (s *AppFileService) isEmbyFormatFile(filename string) bool {
	// 匹配模式：任意字符 - S数字数字E数字数字 (- 任意字符).扩展名
	matched, _ := regexp.MatchString(`\s-\sS\d{2}E\d{2}(\s-\s.+)?\.\w+$`, filename)
	return matched
}

// fileExists 检查文件是否存在
func (s *AppFileService) fileExists(ctx context.Context, path string) (bool, error) {
	_, err := s.alistClient.GetFileInfoWithContext(ctx, path)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "404") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s *AppFileService) GetRenameSuggestions(ctx context.Context, path string) ([]contracts.RenameSuggestion, error) {
	if s.renameSuggester == nil {
		return nil, fmt.Errorf("TMDB not configured, please set TMDB API key in config")
	}

	logger.Debug("Getting rename suggestions", "path", path)

	suggestions, err := s.renameSuggester.SearchAndSuggest(ctx, path)
	if err != nil {
		logger.Error("Failed to get rename suggestions", "path", path, "error", err)
		return nil, fmt.Errorf("failed to get rename suggestions: %w", err)
	}

	logger.Debug("Got rename suggestions", "path", path, "count", len(suggestions))
	return suggestions, nil
}

func (s *AppFileService) GetBatchRenameSuggestions(ctx context.Context, paths []string) (map[string][]contracts.RenameSuggestion, error) {
	if s.renameSuggester == nil {
		return nil, fmt.Errorf("TMDB not configured, please set TMDB API key in config")
	}

	if len(paths) == 0 {
		return make(map[string][]contracts.RenameSuggestion), nil
	}

	logger.Info("Getting batch rename suggestions", "fileCount", len(paths))

	firstPath := paths[0]
	info := s.renameSuggester.ParseFileName(firstPath)

	var suggestionsMap map[string][]rename.Suggestion
	var err error

	if info.MediaType == "movie" {
		suggestionsMap, err = s.renameSuggester.BatchSuggestMovieNames(ctx, paths)
	} else {
		suggestionsMap, err = s.renameSuggester.BatchSuggestTVNames(ctx, paths)
	}

	if err != nil {
		logger.Error("Failed to get batch rename suggestions", "mediaType", info.MediaType, "error", err)
		return nil, fmt.Errorf("failed to get batch rename suggestions: %w", err)
	}

	logger.Info("Got batch rename suggestions", "successCount", len(suggestionsMap), "totalFiles", len(paths))
	return suggestionsMap, nil
}

// GetBatchRenameSuggestionsWithLLM 批量重命名建议
// 策略:
// 1. LLM启用时: 完全使用LLM推断,不fallback到TMDB
// 2. LLM未启用: 使用TMDB批量模式
func (s *AppFileService) GetBatchRenameSuggestionsWithLLM(ctx context.Context, paths []string) (map[string][]contracts.RenameSuggestion, bool, error) {
	if len(paths) == 0 {
		return make(map[string][]contracts.RenameSuggestion), false, nil
	}

	// 检查LLM是否启用
	if s.llmSuggester == nil || s.llmService == nil || !s.llmService.IsEnabled() {
		logger.Info("LLM未启用,使用TMDB批量模式", "fileCount", len(paths))
		result, err := s.GetBatchRenameSuggestions(ctx, paths)
		return result, false, err
	}

	logger.Info("使用LLM批量推断模式", "fileCount", len(paths))

	// 提取共享上下文(剧集名、季度等)
	sharedCtx := s.extractSharedContext(paths)

	// 调用批量LLM推断
	llmResults, err := s.llmSuggester.BatchSuggestFileNames(ctx, filename.BatchFileNameRequest{
		Files:         buildFileNameRequests(paths),
		SharedContext: sharedCtx,
	})

	if err != nil {
		logger.Error("批量LLM推断失败", "error", err, "fileCount", len(paths))
		return nil, true, fmt.Errorf("LLM批量推断失败: %w", err)
	}

	// 处理结果
	result := make(map[string][]contracts.RenameSuggestion)
	skippedCount := 0

	logger.Debug("Processing LLM results matching",
		"totalPaths", len(paths),
		"totalLLMResults", len(llmResults))

	// 创建一个map用于快速查找: original_name -> path
	pathMap := make(map[string]string)
	for _, path := range paths {
		pathMap[filepath.Base(path)] = path
	}

	// 使用文件名匹配(而不是索引)
	for _, llmResult := range llmResults {
		// 通过文件名查找对应的完整路径
		originalPath, found := pathMap[llmResult.OriginalName]
		if !found {
			logger.Warn("Cannot find path for LLM result",
				"originalName", llmResult.OriginalName)
			continue
		}

		logger.Debug("Matched by name",
			"originalName", llmResult.OriginalName,
			"originalPath", originalPath)

		// 检查是否成功
		if llmResult.Error != "" || llmResult.Suggestion == nil {
			logger.Info("LLM无法处理此文件",
				"path", originalPath,
				"error", llmResult.Error)
			skippedCount++
			// 不添加到result,让用户知道此文件无法处理
			continue
		}

		// 直接使用 rename.Suggestion（contracts.RenameSuggestion 现在是它的别名）
		result[originalPath] = []contracts.RenameSuggestion{*llmResult.Suggestion}

		logger.Debug("Successfully processed LLM result",
			"originalPath", originalPath,
			"newName", llmResult.Suggestion.NewName,
			"newPath", llmResult.Suggestion.NewPath)
	}

	logger.Info("批量LLM推断完成",
		"totalFiles", len(paths),
		"successCount", len(result),
		"skippedCount", skippedCount)

	return result, true, nil
}

// extractSharedContext 提取共享上下文
func (s *AppFileService) extractSharedContext(paths []string) *filename.SharedContext {
	if len(paths) == 0 {
		return nil
	}

	// 尝试从第一个文件提取剧集名
	firstPath := paths[0]

	// 使用renameSuggester的extractTVInfoFromPath方法
	if s.renameSuggester != nil {
		showName, season := s.renameSuggester.extractTVInfoFromPath(firstPath)
		if showName != "" {
			// 只传递剧集名,让LLM自己从文件名推断季度
			// 除非路径中明确包含季度信息(season > 1或路径中有Season目录)
			ctx := &filename.SharedContext{
				ShowName:  showName,
				MediaType: "tv",
			}

			// 只有当season不是默认值1时才传递(说明路径中真的有季度信息)
			// 或者路径中包含 "Season" 关键字
			if season > 1 || strings.Contains(firstPath, "Season") || strings.Contains(firstPath, "season") {
				ctx.Season = &season
				logger.Debug("Found explicit season in path, passing to LLM",
					"season", season,
					"path", firstPath)
			} else {
				logger.Debug("No explicit season in path, let LLM infer from filenames",
					"showName", showName,
					"path", firstPath)
			}

			return ctx
		}
	}

	return nil
}

// buildFileNameRequests 构建FileNameRequest列表
func buildFileNameRequests(paths []string) []filename.FileNameRequest {
	requests := make([]filename.FileNameRequest, 0, len(paths))
	for _, path := range paths {
		requests = append(requests, filename.FileNameRequest{
			OriginalName: filepath.Base(path),
			FilePath:     path,
		})
	}
	return requests
}

// isVideoFile 检查文件是否为视频文件
func (s *AppFileService) isVideoFile(filename string) bool {
	return fileutil.IsVideoFile(filename)
}

// directoryGroup 目录分组信息
type directoryGroup struct {
	srcDir     string
	dstDir     string
	moveFiles  []string
	items      []directoryGroupItem
	totalFiles int
	coverage   float64
}

// directoryGroupItem 分组中的单个文件信息
type directoryGroupItem struct {
	fileName    string
	newFileName string
	oldPath     string
	newPath     string
	taskIndex   int
}

// analyzeAndGroupTasks 分析任务并按目录分组
func (s *AppFileService) analyzeAndGroupTasks(ctx context.Context, tasks []contracts.RenameTask) map[string]*directoryGroup {
	groupMap := make(map[string]*directoryGroup)

	// 第一步：按源目录和目标目录分组
	for i, task := range tasks {
		srcDir := filepath.Dir(task.OldPath)
		dstDir := filepath.Dir(task.NewPath)
		oldFile := filepath.Base(task.OldPath)
		newFile := filepath.Base(task.NewPath)

		groupKey := fmt.Sprintf("%s->%s", srcDir, dstDir)

		if groupMap[groupKey] == nil {
			groupMap[groupKey] = &directoryGroup{
				srcDir:    srcDir,
				dstDir:    dstDir,
				moveFiles: []string{},
				items:     []directoryGroupItem{},
			}
		}

		group := groupMap[groupKey]
		group.moveFiles = append(group.moveFiles, oldFile)
		group.items = append(group.items, directoryGroupItem{
			fileName:    oldFile,
			newFileName: newFile,
			oldPath:     task.OldPath,
			newPath:     task.NewPath,
			taskIndex:   i,
		})
	}

	// 第二步：计算每个组的覆盖率（用于决定使用哪种移动策略）
	for _, group := range groupMap {
		// 跳过同目录的组（不需要移动）
		if group.srcDir == group.dstDir {
			group.coverage = 0
			group.totalFiles = len(group.moveFiles)
			continue
		}

		// 获取源目录的总视频文件数
		if listResp, err := s.alistClient.ListFilesWithContext(ctx, group.srcDir, 1, 1000); err == nil {
			videoCount := 0
			for _, file := range listResp.Data.Content {
				if !file.IsDir && s.isVideoFile(file.Name) {
					videoCount++
				}
			}
			group.totalFiles = videoCount
			if videoCount > 0 {
				group.coverage = float64(len(group.moveFiles)) / float64(videoCount)
			}

			logger.Info("目录分组分析完成",
				"srcDir", group.srcDir,
				"dstDir", group.dstDir,
				"moveFiles", len(group.moveFiles),
				"totalFiles", group.totalFiles,
				"coverage", fmt.Sprintf("%.1f%%", group.coverage*100))
		} else {
			logger.Warn("无法获取源目录文件列表，假设覆盖率为100%",
				"srcDir", group.srcDir,
				"error", err)
			group.totalFiles = len(group.moveFiles)
			group.coverage = 1.0
		}
	}

	return groupMap
}

// BatchRenameAndMoveFilesOptimized 优化的批量重命名（智能选择移动策略）
func (s *AppFileService) BatchRenameAndMoveFilesOptimized(
	ctx context.Context,
	tasks []contracts.RenameTask,
) []contracts.RenameResult {

	if len(tasks) == 0 {
		return []contracts.RenameResult{}
	}

	logger.Info("开始优化的批量重命名", "taskCount", len(tasks))

	results := make([]contracts.RenameResult, len(tasks))
	groups := s.analyzeAndGroupTasks(ctx, tasks)

	// 批量创建所有目标目录
	targetDirs := make(map[string]bool)
	for _, group := range groups {
		if group.srcDir != group.dstDir {
			targetDirs[group.dstDir] = true
		}
	}

	var wg sync.WaitGroup
	for dir := range targetDirs {
		wg.Add(1)
		go func(d string) {
			defer wg.Done()
			if err := s.alistClient.Mkdir(ctx, d); err != nil {
				logger.Warn("创建目录失败", "dir", d, "error", err)
			}
		}(dir)
	}
	wg.Wait()

	// 处理每个分组
	var resultsMu sync.Mutex

	for _, group := range groups {
		wg.Add(1)
		go func(g *directoryGroup) {
			defer wg.Done()

			// 策略选择：高覆盖率（≥80%）且文件数≥3 → 使用 recursive_move
			useRecursiveMove := g.srcDir != g.dstDir &&
				g.coverage >= 0.8 &&
				len(g.moveFiles) >= 3

			if useRecursiveMove {
				// 策略A: 使用 recursive_move（整个目录移动）
				logger.Info("使用 RecursiveMove 策略",
					"srcDir", g.srcDir,
					"dstDir", g.dstDir,
					"coverage", fmt.Sprintf("%.1f%%", g.coverage*100),
					"fileCount", len(g.moveFiles))

				if err := s.alistClient.RecursiveMove(ctx, g.srcDir, g.dstDir); err != nil {
					logger.Error("RecursiveMove 失败", "error", err)
					// 标记所有任务失败
					for _, item := range g.items {
						resultsMu.Lock()
						results[item.taskIndex] = contracts.RenameResult{
							OldPath: item.oldPath,
							NewPath: item.newPath,
							Success: false,
							Error:   err,
						}
						resultsMu.Unlock()
					}
					return
				}

				logger.Info("RecursiveMove 成功，开始处理重命名",
					"fileCount", len(g.items))

				// 移动成功，处理需要重命名的文件
				for _, item := range g.items {
					var err error
					if item.fileName != item.newFileName {
						movedPath := filepath.Join(g.dstDir, item.fileName)
						err = s.alistClient.RenameWithContext(ctx, movedPath, item.newFileName)
						if err != nil {
							logger.Warn("重命名失败",
								"movedPath", movedPath,
								"newFileName", item.newFileName,
								"error", err)
						}
					}

					resultsMu.Lock()
					results[item.taskIndex] = contracts.RenameResult{
						OldPath: item.oldPath,
						NewPath: item.newPath,
						Success: err == nil,
						Error:   err,
					}
					resultsMu.Unlock()
				}

			} else {
				// 策略B: 使用批量 Move 或同目录 Rename
				if g.srcDir == g.dstDir {
					// 同目录，只需重命名
					logger.Info("使用同目录重命名策略",
						"dir", g.srcDir,
						"fileCount", len(g.items))

					for _, item := range g.items {
						err := s.alistClient.RenameWithContext(ctx, item.oldPath, item.newFileName)
						if err != nil {
							logger.Warn("重命名失败",
								"oldPath", item.oldPath,
								"newFileName", item.newFileName,
								"error", err)
						}

						resultsMu.Lock()
						results[item.taskIndex] = contracts.RenameResult{
							OldPath: item.oldPath,
							NewPath: item.newPath,
							Success: err == nil,
							Error:   err,
						}
						resultsMu.Unlock()
					}
				} else {
					// 跨目录，使用批量 Move
					logger.Info("使用批量 Move 策略",
						"srcDir", g.srcDir,
						"dstDir", g.dstDir,
						"fileCount", len(g.moveFiles),
						"coverage", fmt.Sprintf("%.1f%%", g.coverage*100))

					if err := s.alistClient.Move(ctx, g.srcDir, g.dstDir, g.moveFiles); err != nil {
						logger.Error("批量 Move 失败", "error", err)
						for _, item := range g.items {
							resultsMu.Lock()
							results[item.taskIndex] = contracts.RenameResult{
								OldPath: item.oldPath,
								NewPath: item.newPath,
								Success: false,
								Error:   err,
							}
							resultsMu.Unlock()
						}
						return
					}

					logger.Info("批量 Move 成功，开始处理重命名",
						"fileCount", len(g.items))

					// Move成功，处理需要重命名的文件
					for _, item := range g.items {
						var err error
						if item.fileName != item.newFileName {
							movedPath := filepath.Join(g.dstDir, item.fileName)
							err = s.alistClient.RenameWithContext(ctx, movedPath, item.newFileName)
							if err != nil {
								logger.Warn("重命名失败",
									"movedPath", movedPath,
									"newFileName", item.newFileName,
									"error", err)
							}
						}

						resultsMu.Lock()
						results[item.taskIndex] = contracts.RenameResult{
							OldPath: item.oldPath,
							NewPath: item.newPath,
							Success: err == nil,
							Error:   err,
						}
						resultsMu.Unlock()
					}
				}
			}

		}(group)
	}

	wg.Wait()

	// 统计结果并清理空目录
	successCount := 0
	oldDirs := make(map[string]bool)
	for i, result := range results {
		if result.Success {
			successCount++
			oldDir := filepath.Dir(tasks[i].OldPath)
			newDir := filepath.Dir(tasks[i].NewPath)
			if oldDir != newDir {
				oldDirs[oldDir] = true
			}
		}
	}

	logger.Info("批量重命名完成",
		"total", len(tasks),
		"success", successCount,
		"failed", len(tasks)-successCount)

	// 清理空目录
	if len(oldDirs) > 0 {
		logger.Info("等待缓存更新后清理源目录")
		time.Sleep(2 * time.Second)

		logger.Info("开始清理源目录", "dirCount", len(oldDirs))
		for dir := range oldDirs {
			if err := s.removeEmptyDirectory(ctx, dir); err != nil {
				logger.Warn("删除目录失败", "dir", dir, "error", err)
			}
		}
	}

	return results
}
