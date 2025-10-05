package media

import (
	"strings"

	"github.com/easayliu/alist-aria2-download/internal/application/contracts"
	pathservices "github.com/easayliu/alist-aria2-download/internal/domain/services/path"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/config"
	"github.com/easayliu/alist-aria2-download/pkg/logger"
	fileutil "github.com/easayliu/alist-aria2-download/pkg/utils/file"
	pathutil "github.com/easayliu/alist-aria2-download/pkg/utils/path"
)

// MediaClassificationService 媒体分类服务 - 专注于文件的媒体类型判断和分类
type MediaClassificationService struct {
	config       *config.Config
	pathCategory *pathservices.PathCategoryService
}

// NewMediaClassificationService 创建媒体分类服务
func NewMediaClassificationService(cfg *config.Config, pathCategory *pathservices.PathCategoryService) *MediaClassificationService {
	return &MediaClassificationService{
		config:       cfg,
		pathCategory: pathCategory,
	}
}

// IsVideoFile 检查是否为视频文件
func (s *MediaClassificationService) IsVideoFile(filename string) bool {
	return fileutil.IsVideoFile(filename, s.config.Download.VideoExts)
}

// GetFileCategory 获取文件分类（基于文件名）
func (s *MediaClassificationService) GetFileCategory(filename string) string {
	if !s.IsVideoFile(filename) {
		return "other"
	}

	filename = strings.ToLower(filename)

	// 电影关键词
	movieKeywords := []string{"movie", "film", "电影", "蓝光", "bluray", "bd", "4k", "1080p", "720p"}
	for _, keyword := range movieKeywords {
		if strings.Contains(filename, keyword) {
			return "movie"
		}
	}

	// 电视剧关键词
	tvKeywords := []string{"tv", "series", "episode", "ep", "s01", "s02", "s03", "season", "电视剧", "连续剧"}
	for _, keyword := range tvKeywords {
		if strings.Contains(filename, keyword) {
			return "tv"
		}
	}

	// 综艺关键词
	varietyKeywords := []string{"variety", "show", "综艺", "娱乐"}
	for _, keyword := range varietyKeywords {
		if strings.Contains(filename, keyword) {
			return "variety"
		}
	}

	return "video"
}

// GetMediaType 获取媒体类型（用于统计）
// 优先使用路径分类，回退到文件名分类
func (s *MediaClassificationService) GetMediaType(filePath string) string {
	// 使用路径分类服务
	pathCategory := s.pathCategory.GetCategoryFromPath(filePath)

	// 如果路径分类成功，直接转换为媒体类型
	if pathCategory != "" {
		return s.pathCategory.GetMediaType(pathCategory)
	}

	// 回退到基于文件名的分类
	filename := pathutil.GetFileName(filePath)
	category := s.GetFileCategory(filename)
	return s.pathCategory.GetMediaType(category)
}

// UpdateMediaStats 更新媒体统计
func (s *MediaClassificationService) UpdateMediaStats(summary *contracts.FileSummary, filePath, filename string) {
	if !s.IsVideoFile(filename) {
		summary.OtherFiles++
		return
	}

	summary.VideoFiles++

	// 使用 GetMediaType 方法，它会优先使用路径分类，然后回退到文件名分类
	mediaType := s.GetMediaType(filePath)
	logger.Debug("文件媒体类型已确定", "file", filename, "mediaType", mediaType)

	switch mediaType {
	case "movie":
		summary.MovieFiles++
	case "tv":
		summary.TVFiles++
	default:
		summary.OtherFiles++
	}
}

// ClassifyFiles 文件分类
func (s *MediaClassificationService) ClassifyFiles(files []contracts.FileResponse) (map[string][]contracts.FileResponse, contracts.ClassificationSummary) {
	classified := make(map[string][]contracts.FileResponse)
	summary := contracts.ClassificationSummary{
		Categories: make(map[string]int),
	}

	for _, file := range files {
		category := s.GetFileCategory(file.Name)
		classified[category] = append(classified[category], file)
		summary.Categories[category]++

		// 特殊分类统计
		switch category {
		case "movie":
			summary.MovieCount++
		case "tv":
			summary.TVCount++
		default:
			summary.OtherCount++
		}
	}

	return classified, summary
}

// FilterVideoFiles 过滤视频文件
func (s *MediaClassificationService) FilterVideoFiles(files []contracts.FileResponse) []contracts.FileResponse {
	var videoFiles []contracts.FileResponse
	for _, file := range files {
		if s.IsVideoFile(file.Name) {
			videoFiles = append(videoFiles, file)
		}
	}
	return videoFiles
}

// FilterByCategory 按分类过滤文件
func (s *MediaClassificationService) FilterByCategory(files []contracts.FileResponse, category string) []contracts.FileResponse {
	var filtered []contracts.FileResponse
	for _, file := range files {
		if s.GetFileCategory(file.Name) == category {
			filtered = append(filtered, file)
		}
	}
	return filtered
}
