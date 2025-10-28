package file

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/easayliu/alist-aria2-download/internal/application/contracts"
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

	suggestionsMap, err := s.renameSuggester.BatchSuggestTVNames(ctx, paths)
	if err != nil {
		logger.Error("Failed to get batch rename suggestions", "error", err)
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
