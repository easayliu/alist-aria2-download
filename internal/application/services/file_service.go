package services

import (
	"time"

	"github.com/easayliu/alist-aria2-download/internal/infrastructure/alist"
	"github.com/easayliu/alist-aria2-download/pkg/utils"
)

// FileService 文件服务
type FileService struct {
	alistClient *alist.Client
	querySvc    *FileQueryService
	filterSvc   *FileFilterService
	mediaSvc    *FileMediaService
	pathSvc     *FilePathService
	statsSvc    *FileStatsService
}

// NewFileService 创建文件服务
func NewFileService(alistClient *alist.Client) *FileService {
	// 创建各个子服务
	filterSvc := NewFileFilterService()
	pathSvc := NewFilePathService()
	mediaSvc := NewFileMediaService(filterSvc, pathSvc)
	querySvc := NewFileQueryService(alistClient, filterSvc)
	statsSvc := NewFileStatsService(mediaSvc)


	return &FileService{
		alistClient: alistClient,
		querySvc:    querySvc,
		filterSvc:   filterSvc,
		mediaSvc:    mediaSvc,
		pathSvc:     pathSvc,
		statsSvc:    statsSvc,
	}
}

// ListFilesSimple 简单列出文件（用于Telegram等场景）
func (s *FileService) ListFilesSimple(path string, page, perPage int) ([]alist.FileItem, error) {
	return s.querySvc.ListFilesSimple(path, page, perPage)
}

// FetchFilesByTimeRange 获取指定时间范围内的文件
func (s *FileService) FetchFilesByTimeRange(path string, startTime, endTime time.Time, videoOnly bool) ([]alist.FileItem, error) {
	return s.querySvc.FetchFilesByTimeRange(path, startTime, endTime, videoOnly)
}

// GetFileDownloadURL 获取文件下载URL
func (s *FileService) GetFileDownloadURL(path, fileName string) string {
	return s.pathSvc.GetFileDownloadURL(s.alistClient.BaseURL, path, fileName)
}

// CreateDownloadTask 创建下载任务（需要依赖下载服务）
func (s *FileService) CreateDownloadTask(url, fileName string) (string, error) {
	// 这里暂时返回一个模拟的任务ID
	// 实际应该调用下载服务
	return "task-" + time.Now().Format("20060102150405"), nil
}

// GetYesterdayFiles 获取昨天修改的文件
func (s *FileService) GetYesterdayFiles(basePath string) ([]YesterdayFileInfo, error) {
	allYesterdayFiles, err := s.querySvc.GetYesterdayFiles(basePath)
	if err != nil {
		return nil, err
	}

	// 使用媒体服务判断媒体类型并生成下载路径
	for i := range allYesterdayFiles {
		mediaType, downloadPath := s.mediaSvc.DetermineMediaTypeAndPath(allYesterdayFiles[i].Path, allYesterdayFiles[i].Name)
		allYesterdayFiles[i].MediaType = mediaType
		allYesterdayFiles[i].DownloadPath = downloadPath
	}

	// 处理电影类型的同目录下载逻辑
	s.mediaSvc.ProcessYesterdayMovieDirectoryGrouping(&allYesterdayFiles)

	return allYesterdayFiles, nil
}

// GetFilesByTimeRange 获取指定时间范围内修改的文件（用于定时任务）
func (s *FileService) GetFilesByTimeRange(basePath string, startTime, endTime time.Time, videoOnly bool) ([]YesterdayFileInfo, error) {
	allFiles, err := s.querySvc.GetFilesByTimeRange(basePath, startTime, endTime, videoOnly)
	if err != nil {
		return nil, err
	}

	// 使用媒体服务判断媒体类型并生成下载路径
	for i := range allFiles {
		mediaType, downloadPath := s.mediaSvc.DetermineMediaTypeAndPath(allFiles[i].Path, allFiles[i].Name)
		allFiles[i].MediaType = mediaType
		allFiles[i].DownloadPath = downloadPath
	}

	// 处理电影类型的同目录下载逻辑
	s.mediaSvc.ProcessYesterdayMovieDirectoryGrouping(&allFiles)

	return allFiles, nil
}

// DetermineMediaTypeAndPath 根据文件路径判断媒体类型并生成下载路径（公开方法）
func (s *FileService) DetermineMediaTypeAndPath(fullPath, fileName string) (MediaType, string) {
	return s.mediaSvc.DetermineMediaTypeAndPath(fullPath, fileName)
}

// GetMediaType 获取文件的媒体类型（用于统计）
func (s *FileService) GetMediaType(filePath string) string {
	return s.mediaSvc.GetMediaType(filePath)
}

// IsVideoFile 检查文件名是否是视频文件（公开方法）
func (s *FileService) IsVideoFile(fileName string) bool {
	return s.filterSvc.IsVideoFile(fileName)
}

// GetFilesFromPath 从指定路径获取文件
func (s *FileService) GetFilesFromPath(basePath string, recursive bool) ([]FileInfo, error) {
	allFiles, err := s.querySvc.GetFilesFromPath(basePath, recursive)
	if err != nil {
		return nil, err
	}

	// 使用媒体服务判断媒体类型并生成下载路径
	for i := range allFiles {
		mediaType, downloadPath := s.mediaSvc.DetermineMediaTypeAndPath(allFiles[i].Path, allFiles[i].Name)
		allFiles[i].MediaType = mediaType
		allFiles[i].DownloadPath = downloadPath
	}

	// 处理电影类型的同目录下载逻辑
	s.mediaSvc.ProcessMovieDirectoryGrouping(&allFiles)

	return allFiles, nil
}

// CalculateFileStats 计算文件统计信息
func (s *FileService) CalculateFileStats(files []FileInfo) FileStats {
	return s.statsSvc.CalculateFileStats(files)
}

// CalculateYesterdayFileStats 计算昨天文件统计信息
func (s *FileService) CalculateYesterdayFileStats(files []YesterdayFileInfo) FileStats {
	return s.statsSvc.CalculateYesterdayFileStats(files)
}

// FormatFileSize 格式化文件大小
// 使用统一的工具函数
func (s *FileService) FormatFileSize(size int64) string {
	return utils.FormatFileSize(size)
}