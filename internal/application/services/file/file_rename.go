package file

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
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

	if oldDir == newDir {
		if err := s.alistClient.RenameWithContext(ctx, oldPath, newFileName); err != nil {
			logger.Error("Failed to rename file", "oldPath", oldPath, "newFileName", newFileName, "error", err)
			return fmt.Errorf("failed to rename file: %w", err)
		}
		logger.Debug("File renamed successfully", "oldPath", oldPath, "newFileName", newFileName)
		return nil
	}

	if err := s.alistClient.Mkdir(ctx, newDir); err != nil {
		logger.Warn("Failed to create directory (may already exist)", "dir", newDir, "error", err)
	}

	if fileName != newFileName {
		if err := s.alistClient.RenameWithContext(ctx, oldPath, newFileName); err != nil {
			logger.Error("Failed to rename file", "oldPath", oldPath, "newFileName", newFileName, "error", err)
			return fmt.Errorf("failed to rename file: %w", err)
		}
		oldPath = filepath.Join(oldDir, newFileName)
	}

	if err := s.alistClient.Move(ctx, oldDir, newDir, []string{newFileName}); err != nil {
		logger.Error("Failed to move file", "srcDir", oldDir, "dstDir", newDir, "fileName", newFileName, "error", err)
		return fmt.Errorf("failed to move file: %w", err)
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
		results   = make([]contracts.RenameResult, len(tasks))
		wg        sync.WaitGroup
		sem       = make(chan struct{}, maxConcurrent)
		oldDirsMu sync.Mutex
		oldDirs   = make(map[string]struct{}) // 收集所有涉及的源目录
	)

	for i, task := range tasks {
		wg.Add(1)
		sem <- struct{}{} // 获取信号量

		go func(idx int, t contracts.RenameTask) {
			defer func() {
				<-sem // 释放信号量
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

	// 统计结果
	successCount := 0
	for _, r := range results {
		if r.Success {
			successCount++
		}
	}

	logger.Info("Batch rename completed",
		"total", len(tasks),
		"success", successCount,
		"failed", len(tasks)-successCount)

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
