package file

import (
	"github.com/easayliu/alist-aria2-download/internal/application/contracts"
	strutil "github.com/easayliu/alist-aria2-download/pkg/utils/string"
)

// IsVideoFile 检查是否为视频文件（委托给MediaClassifier）
func (s *AppFileService) IsVideoFile(filename string) bool {
	return s.mediaClassifier.IsVideoFile(filename)
}

// GetFileCategory 获取文件分类（委托给MediaClassifier）
func (s *AppFileService) GetFileCategory(filename string) string {
	return s.mediaClassifier.GetFileCategory(filename)
}

// GetMediaType 获取媒体类型（委托给MediaClassifier）
func (s *AppFileService) GetMediaType(filePath string) string {
	return s.mediaClassifier.GetMediaType(filePath)
}

// GenerateDownloadPath 生成下载路径（委托给PathGenerator）
func (s *AppFileService) GenerateDownloadPath(file contracts.FileResponse) string {
	if s.pathGenerator != nil {
		return s.pathGenerator.GenerateDownloadPath(file)
	}
	// 回退：如果pathGenerator未初始化，使用默认路径
	baseDir := s.config.Aria2.DownloadDir
	if baseDir == "" {
		baseDir = "/downloads"
	}
	return baseDir + "/others"
}

// GetCategoryFromPath 从路径中分析文件类型（委托给PathCategoryService）
// 保留此方法以保持向后兼容
func (s *AppFileService) GetCategoryFromPath(path string) string {
	return s.pathCategory.GetCategoryFromPath(path)
}

// updateMediaStats 更新媒体统计（委托给MediaClassifier）
func (s *AppFileService) updateMediaStats(summary *contracts.FileSummary, filePath, filename string) {
	s.mediaClassifier.UpdateMediaStats(summary, filePath, filename)
}

// cleanShowName 清理节目名（委托给工具函数，保持向后兼容）
func (s *AppFileService) cleanShowName(showName string) string {
	return strutil.CleanShowName(showName)
}

// FormatFileSize 格式化文件大小（委托给工具函数，保持接口兼容）
func (s *AppFileService) FormatFileSize(size int64) string {
	return strutil.FormatFileSize(size)
}
