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
	// 如果智能解析已经生成了有效的下载路径，直接使用
	if defaultDownloadPath != "" && defaultDownloadPath != "/downloads" {
		// 检查是否是智能生成的路径（包含剧名/电影名）
		if strings.HasPrefix(defaultDownloadPath, "/downloads/tvs/") || 
		   strings.HasPrefix(defaultDownloadPath, "/downloads/movies/") {
			pathAfterDownloads := strings.TrimPrefix(defaultDownloadPath, "/downloads/")
			// 如果包含有意义的内容（不只是 tvs/ 或 movies/），直接使用
			if pathAfterDownloads != "tvs" && pathAfterDownloads != "movies" && 
			   pathAfterDownloads != "tvs/" && pathAfterDownloads != "movies/" {
				return defaultDownloadPath
			}
		}
	}
	
	// 回退逻辑：从源路径提取
	dirPath := filepath.Dir(sourcePath)
	
	// 查找 tvs 目录的位置
	if idx := strings.Index(dirPath, "/tvs/"); idx != -1 {
		// 提取 tvs 后面的路径部分
		tvsPath := dirPath[idx+1:] // 包含 "tvs/" 
		return "/downloads/" + tvsPath
	}
	
	// 查找 movies 目录的位置
	if idx := strings.Index(dirPath, "/movies/"); idx != -1 {
		// 提取 movies 后面的路径部分
		moviesPath := dirPath[idx+1:] // 包含 "movies/"
		return "/downloads/" + moviesPath
	}
	
	// 对于其他路径，使用默认下载路径
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