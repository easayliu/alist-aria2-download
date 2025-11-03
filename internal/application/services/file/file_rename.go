package file

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/easayliu/alist-aria2-download/internal/application/contracts"
	"github.com/easayliu/alist-aria2-download/internal/domain/services/filename"
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

	if err := s.removeEmptyDirectory(ctx, oldDir); err != nil {
		logger.Warn("Failed to remove old directory", "dir", oldDir, "error", err)
	}

	logger.Debug("File renamed and moved successfully", "oldPath", oldPath, "newPath", newPath)
	return nil
}

func (s *AppFileService) removeEmptyDirectory(ctx context.Context, dir string) error {
	listResp, err := s.alistClient.ListFilesWithContext(ctx, dir, 1, 1)
	if err != nil {
		return fmt.Errorf("failed to list directory: %w", err)
	}

	if len(listResp.Data.Content) == 0 {
		dirName := filepath.Base(dir)
		parentDir := filepath.Dir(dir)

		if err := s.alistClient.Remove(ctx, parentDir, []string{dirName}); err != nil {
			return fmt.Errorf("failed to remove empty directory: %w", err)
		}

		logger.Info("Removed empty directory", "dir", dir)
	}

	return nil
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

	result := make([]contracts.RenameSuggestion, 0, len(suggestions))
	for _, s := range suggestions {
		result = append(result, contracts.RenameSuggestion{
			NewName:    s.NewName,
			NewPath:    s.NewPath,
			MediaType:  string(s.MediaType),
			TMDBID:     s.TMDBID,
			Title:      s.Title,
			Year:       s.Year,
			Season:     s.Season,
			Episode:    s.Episode,
			Confidence: s.Confidence,
		})
	}

	logger.Debug("Got rename suggestions", "path", path, "count", len(result))
	return result, nil
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

	var suggestionsMap map[string][]SuggestedName
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

	result := make(map[string][]contracts.RenameSuggestion)
	for path, suggestions := range suggestionsMap {
		contractSuggestions := make([]contracts.RenameSuggestion, 0, len(suggestions))
		for _, s := range suggestions {
			contractSuggestions = append(contractSuggestions, contracts.RenameSuggestion{
				NewName:    s.NewName,
				NewPath:    s.NewPath,
				MediaType:  string(s.MediaType),
				TMDBID:     s.TMDBID,
				Title:      s.Title,
				Year:       s.Year,
				Season:     s.Season,
				Episode:    s.Episode,
				Confidence: s.Confidence,
			})
		}
		result[path] = contractSuggestions
	}

	logger.Info("Got batch rename suggestions", "successCount", len(result), "totalFiles", len(paths))
	return result, nil
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

		// 转换为contracts.RenameSuggestion
		suggestion := s.convertLLMSuggestionToContract(llmResult.Suggestion, originalPath)
		result[originalPath] = []contracts.RenameSuggestion{suggestion}

		logger.Debug("Successfully processed LLM result",
			"originalPath", originalPath,
			"newName", suggestion.NewName,
			"newPath", suggestion.NewPath)
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

// convertLLMSuggestionToContract 转换LLM建议为Contract格式
func (s *AppFileService) convertLLMSuggestionToContract(
	suggestion *filename.FileNameSuggestion,
	originalPath string,
) contracts.RenameSuggestion {
	// 保留原始目录,仅使用LLM生成的新文件名
	originalDir := filepath.Dir(originalPath)
	extension := filepath.Ext(originalPath)

	var newName string
	if suggestion.NewFileName != "" {
		// 使用LLM生成的文件名
		newName = suggestion.NewFileName
		logger.Debug("Using LLM generated file name",
			"originalPath", originalPath,
			"newFileName", newName)
	} else {
		// Fallback: 使用ToEmbyFormat方法生成文件名
		newName = suggestion.ToEmbyFormat(extension)
		logger.Debug("LLM did not generate file name, using ToEmbyFormat fallback",
			"originalPath", originalPath,
			"newFileName", newName)
	}

	// 新路径 = 原始目录 + 新文件名
	newPath := filepath.Join(originalDir, newName)

	result := contracts.RenameSuggestion{
		NewName:    newName,
		NewPath:    newPath,
		MediaType:  suggestion.MediaType,
		Title:      suggestion.Title,
		Year:       suggestion.Year,
		Confidence: float64(suggestion.Confidence),
	}

	if suggestion.Season != nil {
		result.Season = *suggestion.Season
	}
	if suggestion.Episode != nil {
		result.Episode = *suggestion.Episode
	}

	return result
}

// IsSpecialContent 检查文件名是否为特殊内容
// 特殊内容包括: 加更、花絮、预告、特辑、综艺衍生内容等
// 这些内容不适合用TMDB匹配,应该由LLM处理
func (s *AppFileService) IsSpecialContent(fileName string) bool {
	specialKeywords := []string{
		"加更", "花絮", "预告", "片花", "彩蛋", "幕后", "特辑",
		"番外", "访谈", "采访", "回顾", "精彩", "集锦", "合集",
		"首映", "特别企划", "收官", "先导",
		// 综艺衍生内容
		"超前vlog", "超前营业", "陪看记", "母带放送", "惊喜母带",
		"独家记忆", "全员花絮", "制作特辑",
		"vlog", "behind", "making",
		"trailer", "preview", "bonus", "extra", "special",
	}

	lowerFileName := strings.ToLower(fileName)
	for _, keyword := range specialKeywords {
		if strings.Contains(lowerFileName, keyword) {
			return true
		}
	}
	return false
}
