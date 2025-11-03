package filename

import (
	"context"

	"github.com/easayliu/alist-aria2-download/internal/infrastructure/tmdb"
)

// TMDBSuggester TMDB推断器接口
// 定义TMDB推断能力的抽象，避免直接依赖application层
type TMDBSuggester interface {
	// SearchAndSuggest 搜索并生成重命名建议
	SearchAndSuggest(ctx context.Context, fullPath string) ([]TMDBSuggestedName, error)
}

// TMDBSuggestedName TMDB建议的文件名（领域层数据结构）
type TMDBSuggestedName struct {
	NewName    string
	NewPath    string
	MediaType  tmdb.MediaType
	TMDBID     int
	Title      string
	Year       int
	Season     int
	Episode    int
	Confidence float64
}
