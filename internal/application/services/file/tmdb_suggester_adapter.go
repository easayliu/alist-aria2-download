package file

import (
	"context"

	"github.com/easayliu/alist-aria2-download/internal/domain/services/filename"
)

// TMDBSuggesterAdapter 适配器：将RenameSuggester适配为filename.TMDBSuggester接口
// 用于桥接应用层的RenameSuggester和领域层的TMDBSuggester接口
type TMDBSuggesterAdapter struct {
	renameSuggester *RenameSuggester
}

// NewTMDBSuggesterAdapter 创建TMDB推断器适配器
func NewTMDBSuggesterAdapter(renameSuggester *RenameSuggester) filename.TMDBSuggester {
	return &TMDBSuggesterAdapter{
		renameSuggester: renameSuggester,
	}
}

// SearchAndSuggest 实现filename.TMDBSuggester接口
// 将SuggestedName转换为TMDBSuggestedName
func (a *TMDBSuggesterAdapter) SearchAndSuggest(ctx context.Context, path string) ([]filename.TMDBSuggestedName, error) {
	// 调用RenameSuggester的方法
	suggestions, err := a.renameSuggester.SearchAndSuggest(ctx, path)
	if err != nil {
		return nil, err
	}

	// 转换类型
	result := make([]filename.TMDBSuggestedName, len(suggestions))
	for i, s := range suggestions {
		result[i] = convertSuggestedNameToTMDBSuggestedName(s)
	}

	return result, nil
}

// convertSuggestedNameToTMDBSuggestedName 转换SuggestedName到TMDBSuggestedName
func convertSuggestedNameToTMDBSuggestedName(s SuggestedName) filename.TMDBSuggestedName {
	return filename.TMDBSuggestedName{
		NewName:    s.NewName,
		NewPath:    s.NewPath,
		MediaType:  s.MediaType,  // 类型已经兼容
		TMDBID:     s.TMDBID,
		Title:      s.Title,
		Year:       s.Year,
		Season:     s.Season,
		Episode:    s.Episode,
		Confidence: s.Confidence,
	}
}
