package file

import (
	"context"

	"github.com/easayliu/alist-aria2-download/internal/domain/models/rename"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/tmdb"
	"github.com/easayliu/alist-aria2-download/pkg/logger"
)

// 包级常量：TV根目录名称
var tvRootDirs = map[string]struct{}{
	"tvs":      {},
	"tv shows": {},
	"tvshows":  {},
	"剧集":       {},
	"电视剧":      {},
}

// 包级常量：中文数字映射
var chineseNumMap = map[string]int{
	"一": 1, "二": 2, "三": 3, "四": 4, "五": 5,
	"六": 6, "七": 7, "八": 8, "九": 9, "十": 10,
}

// RenameSuggester 重命名建议器
type RenameSuggester struct {
	tmdbClient         *tmdb.Client
	qualityDirPatterns []string
}

// NewRenameSuggester 创建重命名建议器
func NewRenameSuggester(tmdbClient *tmdb.Client, qualityDirPatterns []string) *RenameSuggester {
	return &RenameSuggester{
		tmdbClient:         tmdbClient,
		qualityDirPatterns: qualityDirPatterns,
	}
}

// MediaInfo 媒体信息
type MediaInfo struct {
	OriginalName string
	MediaType    tmdb.MediaType
	Title        string
	Year         int
	Season       int
	Episode      int
	Part         string
	Extension    string
	AirDate      string
	Version      string
	// 缓存字段：避免重复解析路径
	pathShowName   string // 从路径提取的剧名
	pathSeason     int    // 从路径提取的季度
	pathInfoParsed bool   // 路径信息是否已解析
}

// SearchAndSuggest 搜索并返回重命名建议（入口方法）
func (rs *RenameSuggester) SearchAndSuggest(ctx context.Context, fullPath string) ([]rename.Suggestion, error) {
	info := rs.ParseFileName(fullPath)

	// 对于TV剧集,使用缓存的路径信息
	if info.MediaType == tmdb.MediaTypeTV {
		showName, pathSeason := rs.getPathInfo(info, fullPath)

		// 如果从路径中成功提取了剧集名,使用它替代从文件名提取的标题
		if showName != "" {
			logger.Info("使用从路径提取的剧集名",
				"originalTitle", info.Title,
				"pathShowName", showName,
				"pathSeason", pathSeason,
				"fileNameSeason", info.Season)
			info.Title = showName
		}

		// 如果从路径中提取了季度,优先使用路径季度
		if pathSeason > 0 {
			info.Season = pathSeason
		}

		// 重置年份(TV剧集的年份应该从TMDB查询结果中获取)
		info.Year = 0
	}

	logger.Info("TMDB search started",
		"path", fullPath,
		"title", info.Title,
		"mediaType", info.MediaType,
		"season", info.Season,
		"episode", info.Episode,
		"part", info.Part,
		"airDate", info.AirDate,
		"version", info.Version,
		"year", info.Year)

	if info.MediaType == tmdb.MediaTypeTV {
		return rs.suggestTVName(ctx, fullPath, info)
	}
	return rs.suggestMovieName(ctx, fullPath, info)
}
