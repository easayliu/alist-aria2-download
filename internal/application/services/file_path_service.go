package services

import (
	"path/filepath"
	"strings"
)

// FilePathService 文件路径服务
type FilePathService struct{}

// NewFilePathService 创建文件路径服务
func NewFilePathService() *FilePathService {
	return &FilePathService{}
}

// ApplyPathMapping 应用路径映射规则
func (s *FilePathService) ApplyPathMapping(sourcePath, defaultDownloadPath string) string {
	// 获取源路径的目录
	dirPath := filepath.Dir(sourcePath)
	
	// 查找 tvs 目录的位置
	if idx := strings.Index(dirPath, "/tvs/"); idx != -1 {
		// 提取 tvs 后面的路径部分
		tvsPath := dirPath[idx+1:] // 包含 "tvs/" 
		
		// 如果默认下载路径包含智能生成的季度信息，需要保留
		if strings.HasPrefix(defaultDownloadPath, "/downloads/tvs/") {
			// 从默认路径中提取剧名和季度信息
			pathAfterTvs := strings.TrimPrefix(defaultDownloadPath, "/downloads/tvs/")
			// 从源路径中提取剧名
			sourcePathParts := strings.Split(tvsPath, "/")
			if len(sourcePathParts) >= 2 && pathAfterTvs != "" {
				// 如果智能生成的路径包含季度信息，保留完整路径
				if strings.Contains(pathAfterTvs, "/") {
					return defaultDownloadPath
				}
			}
		}
		
		return "/downloads/" + tvsPath
	}
	
	// 查找 movies 目录的位置
	if idx := strings.Index(dirPath, "/movies/"); idx != -1 {
		// 提取 movies 后面的路径部分
		moviesPath := dirPath[idx+1:] // 包含 "movies/"
		
		// 如果默认下载路径包含智能生成的电影信息，需要保留
		if strings.HasPrefix(defaultDownloadPath, "/downloads/movies/") {
			pathAfterMovies := strings.TrimPrefix(defaultDownloadPath, "/downloads/movies/")
			if pathAfterMovies != "" && strings.Contains(pathAfterMovies, "/") {
				return defaultDownloadPath
			}
		}
		
		return "/downloads/" + moviesPath
	}
	
	// 对于其他路径，保持原有的智能生成逻辑
	return defaultDownloadPath
}

// ExtractFolderName 提取文件夹名称
func (s *FilePathService) ExtractFolderName(fullPath string) string {
	parts := strings.Split(fullPath, "/")
	if len(parts) > 1 {
		// 返回倒数第二个部分（通常是包含文件的文件夹）
		return s.CleanFolderName(parts[len(parts)-2])
	}
	return "unknown"
}

// CleanFolderName 清理文件夹名称
func (s *FilePathService) CleanFolderName(name string) string {
	// 移除特殊字符，保留字母数字和基本符号
	name = strings.TrimSpace(name)

	// 替换不适合作为文件夹名的字符
	replacer := strings.NewReplacer(
		":", "-",
		"?", "",
		"*", "",
		"<", "",
		">", "",
		"|", "",
		"\\", "",
		"/", "",
		"\"", "",
	)

	return replacer.Replace(name)
}

// GetFileDownloadURL 获取文件下载URL
func (s *FilePathService) GetFileDownloadURL(baseURL, path, fileName string) string {
	// 构建完整路径
	fullPath := path + "/" + fileName
	if path == "/" {
		fullPath = "/" + fileName
	}

	// 这里需要根据Alist的配置构建下载URL
	// 通常是 base_url + /d + path
	return baseURL + "/d" + fullPath
}