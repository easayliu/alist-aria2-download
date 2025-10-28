package file

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/easayliu/alist-aria2-download/pkg/logger"
)

func (s *AppFileService) DeleteFile(ctx context.Context, path string) error {
	if s.alistClient == nil {
		return fmt.Errorf("alist client not initialized")
	}

	logger.Info("Deleting file", "path", path)

	dir := filepath.Dir(path)
	fileName := filepath.Base(path)

	if err := s.alistClient.Remove(ctx, dir, []string{fileName}); err != nil {
		logger.Error("Failed to delete file", "path", path, "error", err)
		return fmt.Errorf("failed to delete file: %w", err)
	}

	logger.Info("File deleted successfully", "path", path)
	return nil
}

func (s *AppFileService) DeleteFiles(ctx context.Context, paths []string) error {
	if s.alistClient == nil {
		return fmt.Errorf("alist client not initialized")
	}

	if len(paths) == 0 {
		return nil
	}

	logger.Info("Deleting files", "count", len(paths))

	pathMap := make(map[string][]string)
	for _, path := range paths {
		dir := filepath.Dir(path)
		fileName := filepath.Base(path)
		pathMap[dir] = append(pathMap[dir], fileName)
	}

	var lastErr error
	successCount := 0

	for dir, fileNames := range pathMap {
		if err := s.alistClient.Remove(ctx, dir, fileNames); err != nil {
			logger.Error("Failed to delete files in directory", "dir", dir, "files", fileNames, "error", err)
			lastErr = err
		} else {
			successCount += len(fileNames)
			logger.Info("Files deleted successfully", "dir", dir, "count", len(fileNames))
		}
	}

	if lastErr != nil {
		return fmt.Errorf("failed to delete some files (deleted: %d/%d): %w", successCount, len(paths), lastErr)
	}

	logger.Info("All files deleted successfully", "count", len(paths))
	return nil
}
