package file

import (
	"fmt"
	"strings"
	"time"

	"github.com/easayliu/alist-aria2-download/internal/infrastructure/alist"
	"github.com/easayliu/alist-aria2-download/internal/shared/utils"
	"github.com/easayliu/alist-aria2-download/pkg/logger"
	timeutil "github.com/easayliu/alist-aria2-download/pkg/utils/time"
)

// FileQueryService 文件查询服务
type FileQueryService struct {
	alistClient *alist.Client
	filterSvc   *utils.FileFilterService
}

// NewFileQueryService 创建文件查询服务
func NewFileQueryService(alistClient *alist.Client, filterSvc *utils.FileFilterService) *FileQueryService {
	return &FileQueryService{
		alistClient: alistClient,
		filterSvc:   filterSvc,
	}
}

// ListFilesSimple 简单列出文件（用于Telegram等场景）
func (s *FileQueryService) ListFilesSimple(path string, page, perPage int) ([]alist.FileItem, error) {
	fileList, err := s.alistClient.ListFiles(path, page, perPage)
	if err != nil {
		return nil, err
	}
	return fileList.Data.Content, nil
}

// FetchFilesByTimeRange 获取指定时间范围内的文件
func (s *FileQueryService) FetchFilesByTimeRange(path string, startTime, endTime time.Time, videoOnly bool) ([]alist.FileItem, error) {
	var allFiles []alist.FileItem

	// 递归获取所有文件
	if err := s.fetchFilesRecursiveByTime(path, startTime, endTime, videoOnly, &allFiles); err != nil {
		return nil, err
	}

	return allFiles, nil
}

// fetchFilesRecursiveByTime 递归获取时间范围内的文件
func (s *FileQueryService) fetchFilesRecursiveByTime(path string, startTime, endTime time.Time, videoOnly bool, files *[]alist.FileItem) error {
	fileList, err := s.alistClient.ListFiles(path, 1, 1000)
	if err != nil {
		return fmt.Errorf("获取文件列表失败: %w", err)
	}

	for _, file := range fileList.Data.Content {
		fileTime := timeutil.ParseTimeOrZero(file.Modified)

		if file.IsDir {
			// 递归处理子目录
			subPath := path + "/" + file.Name
			if path == "/" {
				subPath = "/" + file.Name
			}
			s.fetchFilesRecursiveByTime(subPath, startTime, endTime, videoOnly, files)
		} else {
			// 检查文件时间和类型
			if timeutil.IsInRange(fileTime, startTime, endTime) {
				if !videoOnly || (videoOnly && s.filterSvc.IsVideoFile(file.Name)) {
					*files = append(*files, file)
				}
			}
		}
	}

	return nil
}

// GetYesterdayFiles 获取昨天修改的文件
func (s *FileQueryService) GetYesterdayFiles(basePath string) ([]YesterdayFileInfo, error) {
	var allYesterdayFiles []YesterdayFileInfo

	// 使用时间工具创建昨天的时间范围
	yesterdayRange := timeutil.CreateYesterdayRange()

	// 递归获取文件
	if err := s.fetchYesterdayFilesRecursive(basePath, yesterdayRange.Start, yesterdayRange.End, &allYesterdayFiles); err != nil {
		return nil, err
	}

	return allYesterdayFiles, nil
}

// GetFilesByTimeRange 获取指定时间范围内修改的文件（用于定时任务）
func (s *FileQueryService) GetFilesByTimeRange(basePath string, startTime, endTime time.Time, videoOnly bool) ([]YesterdayFileInfo, error) {
	var allFiles []YesterdayFileInfo

	// 递归获取文件
	if err := s.fetchFilesRecursiveWithInfo(basePath, startTime, endTime, videoOnly, &allFiles); err != nil {
		return nil, err
	}

	return allFiles, nil
}

// fetchFilesRecursiveWithInfo 递归获取指定时间范围的文件（通用方法）
func (s *FileQueryService) fetchFilesRecursiveWithInfo(path string, startTime, endTime time.Time, videoOnly bool, result *[]YesterdayFileInfo) error {
	page := 1
	perPage := 100

	for {
		// 获取文件列表
		fileList, err := s.alistClient.ListFiles(path, page, perPage)
		if err != nil {
			return err
		}

		// 处理每个文件/目录
		for _, file := range fileList.Data.Content {
			// 解析修改时间
			modTime := timeutil.ParseTimeOrZero(file.Modified)
			if modTime.IsZero() {
				continue
			}

			// 构建完整路径
			fullPath := file.Path
			if fullPath == "" {
				if path == "/" {
					fullPath = "/" + file.Name
				} else {
					fullPath = path + "/" + file.Name
				}
			}

			if file.IsDir {
				// 如果是目录，递归处理
				if err := s.fetchFilesRecursiveWithInfo(fullPath, startTime, endTime, videoOnly, result); err != nil {
					return err
				}
			} else {
				// 如果需要过滤视频文件
				if videoOnly && !s.filterSvc.IsVideoFile(file.Name) {
					continue
				}

				// 检查是否在时间范围内
				if timeutil.IsInRange(modTime, startTime, endTime) {
					// 获取文件详细信息（包含下载链接）
					fileInfo, err := s.alistClient.GetFileInfo(fullPath)
					if err != nil {
						continue
					}

					// 替换URL（只在包含fcalist-public时替换）
					originalURL := fileInfo.Data.RawURL
					logger.Info("🎯 FileQueryService获取到raw_url", "path", fullPath, "raw_url", originalURL)
					
					internalURL := originalURL
					if strings.Contains(originalURL, "fcalist-public") {
						internalURL = strings.ReplaceAll(originalURL, "fcalist-public", "fcalist-internal")
						logger.Info("🔄 FileQueryService URL替换", "original", originalURL, "internal", internalURL)
					} else {
						logger.Info("ℹ️  FileQueryService无需URL替换", "url", originalURL)
					}

					// 判断媒体类型并生成下载路径（这里需要依赖媒体服务）
					mediaType := MediaTypeOther
					downloadPath := "/downloads"

					*result = append(*result, YesterdayFileInfo{
						Name:         file.Name,
						Path:         fullPath,
						Size:         file.Size,
						Modified:     modTime,
						OriginalURL:  originalURL,
						InternalURL:  internalURL,
						MediaType:    mediaType,
						DownloadPath: downloadPath,
					})
				}
			}
		}

		// 检查是否还有更多页
		if len(fileList.Data.Content) < perPage {
			break
		}
		page++
	}

	return nil
}

// fetchYesterdayFilesRecursive 递归获取昨天的文件
func (s *FileQueryService) fetchYesterdayFilesRecursive(path string, yesterdayStart, yesterdayEnd time.Time, result *[]YesterdayFileInfo) error {
	page := 1
	perPage := 100

	for {
		// 获取文件列表
		fileList, err := s.alistClient.ListFiles(path, page, perPage)
		if err != nil {
			return err
		}

		// 处理每个文件/目录
		for _, file := range fileList.Data.Content {
			// 解析修改时间
			modTime := timeutil.ParseTimeOrZero(file.Modified)
			if modTime.IsZero() {
				continue
			}

			// 构建完整路径
			fullPath := file.Path
			if fullPath == "" {
				if path == "/" {
					fullPath = "/" + file.Name
				} else {
					fullPath = path + "/" + file.Name
				}
			}

			if file.IsDir {
				// 如果是目录，递归处理
				if err := s.fetchYesterdayFilesRecursive(fullPath, yesterdayStart, yesterdayEnd, result); err != nil {
					return err
				}
			} else {
				// 如果是文件，先检查是否为视频文件
				if !s.filterSvc.IsVideoFile(file.Name) {
					continue
				}

				// 检查是否是昨天修改的
				if timeutil.IsInRange(modTime, yesterdayStart, yesterdayEnd) {
					// 获取文件详细信息（包含下载链接）
					fileInfo, err := s.alistClient.GetFileInfo(fullPath)
					if err != nil {
						continue
					}

					// 替换URL（只在包含fcalist-public时替换）
					originalURL := fileInfo.Data.RawURL
					internalURL := originalURL
					if strings.Contains(originalURL, "fcalist-public") {
						internalURL = strings.ReplaceAll(originalURL, "fcalist-public", "fcalist-internal")
					}

					// 判断媒体类型并生成下载路径（这里需要依赖媒体服务）
					mediaType := MediaTypeOther
					downloadPath := "/downloads"

					*result = append(*result, YesterdayFileInfo{
						Name:         file.Name,
						Path:         fullPath,
						Size:         file.Size,
						Modified:     modTime,
						OriginalURL:  originalURL,
						InternalURL:  internalURL,
						MediaType:    mediaType,
						DownloadPath: downloadPath,
					})
				}
			}
		}

		// 检查是否还有更多页
		if len(fileList.Data.Content) < perPage {
			break
		}
		page++
	}

	return nil
}

// GetFilesFromPath 从指定路径获取文件
func (s *FileQueryService) GetFilesFromPath(basePath string, recursive bool) ([]FileInfo, error) {
	var allFiles []FileInfo

	if recursive {
		// 递归获取所有文件
		if err := s.fetchFilesRecursive(basePath, &allFiles); err != nil {
			return nil, err
		}
	} else {
		// 只获取当前目录的文件
		if err := s.fetchFilesFromDirectory(basePath, &allFiles); err != nil {
			return nil, err
		}
	}

	return allFiles, nil
}

// fetchFilesFromDirectory 获取目录中的文件（不递归）
func (s *FileQueryService) fetchFilesFromDirectory(path string, result *[]FileInfo) error {
	page := 1
	perPage := 100

	for {
		// 获取文件列表
		fileList, err := s.alistClient.ListFiles(path, page, perPage)
		if err != nil {
			return err
		}

		// 处理每个文件
		for _, file := range fileList.Data.Content {
			// 跳过目录
			if file.IsDir {
				continue
			}

			// 跳过非视频文件
			if !s.filterSvc.IsVideoFile(file.Name) {
				continue
			}

			// 解析修改时间
			modTime := timeutil.ParseTimeOrNow(file.Modified)

			// 构建完整路径
			fullPath := file.Path
			if fullPath == "" {
				if path == "/" {
					fullPath = "/" + file.Name
				} else {
					fullPath = path + "/" + file.Name
				}
			}

			// 获取文件详细信息（包含下载链接）
			fileInfo, err := s.alistClient.GetFileInfo(fullPath)
			if err != nil {
				continue
			}

			// 替换URL（只在包含fcalist-public时替换）
			originalURL := fileInfo.Data.RawURL
			internalURL := originalURL
			if strings.Contains(originalURL, "fcalist-public") {
				internalURL = strings.ReplaceAll(originalURL, "fcalist-public", "fcalist-internal")
			}

			// 判断媒体类型并生成下载路径（这里需要依赖媒体服务）
			mediaType := MediaTypeOther
			downloadPath := "/downloads"

			*result = append(*result, FileInfo{
				Name:         file.Name,
				Path:         fullPath,
				Size:         file.Size,
				Modified:     modTime,
				OriginalURL:  originalURL,
				InternalURL:  internalURL,
				MediaType:    mediaType,
				DownloadPath: downloadPath,
			})
		}

		// 检查是否还有更多页
		if len(fileList.Data.Content) < perPage {
			break
		}
		page++
	}

	return nil
}

// fetchFilesRecursive 递归获取所有文件
func (s *FileQueryService) fetchFilesRecursive(path string, result *[]FileInfo) error {
	page := 1
	perPage := 100

	for {
		// 获取文件列表
		fileList, err := s.alistClient.ListFiles(path, page, perPage)
		if err != nil {
			return err
		}

		// 处理每个文件/目录
		for _, file := range fileList.Data.Content {
			// 解析修改时间
			modTime := timeutil.ParseTimeOrNow(file.Modified)

			// 构建完整路径
			fullPath := file.Path
			if fullPath == "" {
				if path == "/" {
					fullPath = "/" + file.Name
				} else {
					fullPath = path + "/" + file.Name
				}
			}

			if file.IsDir {
				// 如果是目录，递归处理
				if err := s.fetchFilesRecursive(fullPath, result); err != nil {
					return err
				}
			} else {
				// 如果是文件，先检查是否为视频文件
				if !s.filterSvc.IsVideoFile(file.Name) {
					continue
				}

				// 添加到结果
				fileInfo, err := s.alistClient.GetFileInfo(fullPath)
				if err != nil {
					continue
				}

				// 替换URL（只在包含fcalist-public时替换）
				originalURL := fileInfo.Data.RawURL
				internalURL := originalURL
				if strings.Contains(originalURL, "fcalist-public") {
					internalURL = strings.ReplaceAll(originalURL, "fcalist-public", "fcalist-internal")
				}

				// 判断媒体类型并生成下载路径（这里需要依赖媒体服务）
				mediaType := MediaTypeOther
				downloadPath := "/downloads"

				*result = append(*result, FileInfo{
					Name:         file.Name,
					Path:         fullPath,
					Size:         file.Size,
					Modified:     modTime,
					OriginalURL:  originalURL,
					InternalURL:  internalURL,
					MediaType:    mediaType,
					DownloadPath: downloadPath,
				})
			}
		}

		// 检查是否还有更多页
		if len(fileList.Data.Content) < perPage {
			break
		}
		page++
	}

	return nil
}