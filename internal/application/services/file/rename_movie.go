package file

import (
	"context"
	"fmt"
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

	for _, path := range paths {
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

	if len(result) == 0 {
		return nil, fmt.Errorf("未能为任何电影文件生成重命名建议")
	}

	return result, nil
}
