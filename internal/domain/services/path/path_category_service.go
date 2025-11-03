package path

import (
	"strings"
	"sync"
)

// PathCategoryService 路径分类服务 - 统一的路径分类逻辑（带缓存优化）
type PathCategoryService struct {
	// 缓存路径分类结果（路径 -> 分类）
	categoryCache sync.Map
	// 缓存小写路径结果（路径 -> 小写路径）
	lowerCache sync.Map
}

// NewPathCategoryService 创建路径分类服务
func NewPathCategoryService() *PathCategoryService {
	return &PathCategoryService{}
}

// GetCategoryFromPath 从路径中分析文件类型（统一实现，带缓存优化）
// 优先级：路径中的类型指示器高于文件名分析
func (s *PathCategoryService) GetCategoryFromPath(path string) string {
	if path == "" {
		return ""
	}

	// 检查缓存
	if cached, ok := s.categoryCache.Load(path); ok {
		return cached.(string)
	}

	// 获取小写路径（使用缓存）
	pathLower := s.getPathLower(path)

	// 检查 TVs 和 Movies 的位置，选择最早出现的
	tvsIndex := strings.Index(pathLower, "tvs")
	moviesIndex := strings.Index(pathLower, "movies")

	// 如果两个都存在，选择最早出现的（路径层级更高的）
	if tvsIndex != -1 && moviesIndex != -1 {
		category := "tv"
		if tvsIndex > moviesIndex {
			category = "movie"
		}
		// 缓存并返回
		s.categoryCache.Store(path, category)
		return category
	}

	// 计算分类结果
	category := s.computeCategory(pathLower, tvsIndex, moviesIndex)

	// 缓存结果（只缓存有效分类）
	if category != "" {
		s.categoryCache.Store(path, category)
	}

	return category
}

// computeCategory 计算路径分类（内部方法）
func (s *PathCategoryService) computeCategory(pathLower string, tvsIndex, moviesIndex int) string {
	// 简化的 TVs 判断：只要路径包含 tvs 就判断为 tv
	if tvsIndex != -1 {
		return "tv"
	}

	// 简化的 Movies 判断：只要路径包含 movies 就判断为 movie
	if moviesIndex != -1 {
		return "movie"
	}

	// 综艺类型指示器
	varietyPathKeywords := []string{"/variety/", "/show/", "/综艺/", "/娱乐/"}
	for _, keyword := range varietyPathKeywords {
		if strings.Contains(pathLower, keyword) {
			return "variety"
		}
	}

	// 一般视频类型指示器
	videoPathKeywords := []string{"/videos/", "/video/", "/视频/"}
	for _, keyword := range videoPathKeywords {
		if strings.Contains(pathLower, keyword) {
			return "video"
		}
	}

	// 如果路径中没有明确的类型指示器，返回空字符串
	return ""
}

// getPathLower 获取小写路径（带缓存）
func (s *PathCategoryService) getPathLower(path string) string {
	if cached, ok := s.lowerCache.Load(path); ok {
		return cached.(string)
	}

	pathLower := strings.ToLower(path)
	s.lowerCache.Store(path, pathLower)
	return pathLower
}

// GetMediaType 获取媒体类型（用于统计）
// 将分类转换为标准的媒体类型
func (s *PathCategoryService) GetMediaType(category string) string {
	switch category {
	case "movie":
		return "movie"
	case "tv", "variety":
		return "tv" // 综艺节目也算作TV类型
	default:
		return "other"
	}
}

// GetCategoryFromPathWithFallback 从路径获取分类，如果失败则使用文件名分类作为回退
func (s *PathCategoryService) GetCategoryFromPathWithFallback(path, filename string, filenameCategoryFn func(string) string) string {
	// 优先使用路径分类
	pathCategory := s.GetCategoryFromPath(path)
	if pathCategory != "" {
		return pathCategory
	}

	// 回退到文件名分类
	if filenameCategoryFn != nil {
		return filenameCategoryFn(filename)
	}

	return ""
}
