package file

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/easayliu/alist-aria2-download/internal/domain/models/rename"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/tmdb"
	"github.com/easayliu/alist-aria2-download/pkg/logger"
)

// suggestMovieName 为电影生成重命名建议
func (rs *RenameSuggester) suggestMovieName(ctx context.Context, fullPath string, info *MediaInfo) ([]rename.Suggestion, error) {
	resp, err := rs.tmdbClient.SearchMovie(ctx, info.Title, info.Year)
	if err != nil {
		return nil, fmt.Errorf("failed to search movie: %w", err)
	}

	if len(resp.Results) == 0 {
		return nil, fmt.Errorf("TMDB数据库中未找到电影 '%s'，可能是因为：\n1. 电影名称不准确\n2. TMDB未收录该影片\n3. 需要使用英文名称搜索", info.Title)
	}

	suggestions := make([]rename.Suggestion, 0, len(resp.Results))
	for i, result := range resp.Results {
		year := 0
		if result.ReleaseDate != "" {
			if parsedYear, err := strconv.Atoi(result.ReleaseDate[:4]); err == nil {
				year = parsedYear
			}
		}

		confidence := 1.0 - (float64(i) * 0.1)
		if info.Year > 0 && year == info.Year {
			confidence += 0.2
		}

		details, err := rs.tmdbClient.GetMovieDetails(ctx, result.ID)
		if err != nil {
			logger.Warn("Failed to get movie details", "movieID", result.ID, "title", result.Title, "error", err)
			newName := fmt.Sprintf("%s (%d)%s", result.Title, year, info.Extension)
			newPath := rs.buildMoviePath(fullPath, result.Title, year, newName)

			suggestions = append(suggestions, rename.Suggestion{
				NewName:    newName,
				NewPath:    newPath,
				MediaType:  rename.FromTMDBMediaType(tmdb.MediaTypeMovie),
				TMDBID:     result.ID,
				Title:      result.Title,
				Year:       year,
				Confidence: confidence,
				Source:     rename.SourceTMDB,
			})
			continue
		}

		title := details.Title
		if details.OriginalTitle != "" && details.OriginalLanguage != "en" {
			title = details.OriginalTitle
		}

		newName := fmt.Sprintf("%s (%d)%s", title, year, info.Extension)
		newPath := rs.buildMoviePath(fullPath, title, year, newName)

		logger.Info("Generated movie rename suggestion",
			"originalPath", fullPath,
			"newName", newName,
			"newPath", newPath,
			"tmdbID", details.ID,
			"title", title,
			"originalTitle", details.OriginalTitle,
			"year", year,
			"runtime", details.Runtime)

		sug := rename.Suggestion{
			NewName:    newName,
			NewPath:    newPath,
			MediaType:  rename.FromTMDBMediaType(tmdb.MediaTypeMovie),
			TMDBID:     details.ID,
			Title:      title,
			Year:       year,
			Confidence: confidence,
			Source:     rename.SourceTMDB,
		}
		suggestions = append(suggestions, sug)
	}

	return suggestions, nil
}

// BatchSuggestMovieNames 批量生成电影重命名建议
func (rs *RenameSuggester) BatchSuggestMovieNames(ctx context.Context, paths []string) (map[string][]rename.Suggestion, error) {
	if len(paths) == 0 {
		return make(map[string][]rename.Suggestion), nil
	}

	result := make(map[string][]rename.Suggestion)
	skippedCount := 0

	for _, path := range paths {
		filename := filepath.Base(path)

		// 预过滤：跳过已符合 Emby 标准格式的文件
		if rs.IsAlreadyEmbyMovieFormat(filename) {
			logger.Info("电影文件已符合 Emby 标准格式，跳过",
				"path", path,
				"filename", filename)
			result[path] = []rename.Suggestion{rs.BuildSkippedSuggestion(path, "已符合 Emby 标准格式")}
			skippedCount++
			continue
		}

		info := rs.ParseFileName(path)

		if info.MediaType != tmdb.MediaTypeMovie {
			logger.Warn("Skipping non-movie file", "path", path, "detectedType", info.MediaType)
			continue
		}

		suggestions, err := rs.suggestMovieName(ctx, path, info)
		if err != nil {
			logger.Warn("Failed to suggest movie name", "path", path, "title", info.Title, "error", err)
			continue
		}

		result[path] = suggestions
	}

	if skippedCount > 0 {
		logger.Info("批量电影重命名预过滤完成",
			"totalFiles", len(paths),
			"skipped", skippedCount,
			"processed", len(result)-skippedCount)
	}

	// 检查是否有任何非跳过的结果
	hasNonSkippedResult := false
	for _, suggestions := range result {
		for _, sug := range suggestions {
			if !sug.Skipped {
				hasNonSkippedResult = true
				break
			}
		}
		if hasNonSkippedResult {
			break
		}
	}

	// 如果所有文件都被跳过（已符合标准），返回成功
	if len(result) > 0 && !hasNonSkippedResult {
		logger.Info("所有电影文件已符合标准格式，无需处理", "totalFiles", len(paths))
		return result, nil
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("未能为任何电影文件生成重命名建议")
	}

	return result, nil
}
