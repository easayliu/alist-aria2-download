package services

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/easayliu/alist-aria2-download/internal/application/contracts"
	"github.com/easayliu/alist-aria2-download/pkg/logger"
	"github.com/easayliu/alist-aria2-download/pkg/utils"
)

// ListFiles 获取文件列表 - 统一的业务逻辑
func (s *AppFileService) ListFiles(ctx context.Context, req contracts.FileListRequest) (*contracts.FileListResponse, error) {
	logger.Info("Listing files", "path", req.Path, "page", req.Page, "recursive", req.Recursive)

	// 1. 参数验证和默认值设置
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 50
	} else if req.PageSize > 1000 {
		req.PageSize = 1000
	}

	// 2. 确保AList客户端已登录并获取文件列表
	if s.alistClient.Token == "" {
		logger.Info("🔑 ListFiles: 检测到未登录，开始登录AList", "baseURL", s.alistClient.BaseURL)
		if err := s.alistClient.Login(); err != nil {
			return nil, fmt.Errorf("failed to login to AList: %w", err)
		}
		logger.Info("✅ ListFiles: AList登录成功")
	}
	
	alistResp, err := s.alistClient.ListFiles(req.Path, req.Page, req.PageSize)
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	// 3. 转换并分类文件
	var files, directories []contracts.FileResponse
	summary := contracts.FileSummary{}

	for _, item := range alistResp.Data.Content {
		fileResp := s.convertToFileResponse(item, req.Path)

		if item.IsDir {
			directories = append(directories, fileResp)
			summary.TotalDirs++
			logger.Info("Added directory", "name", item.Name)
		} else {
			// 应用视频过滤
			if req.VideoOnly && !s.IsVideoFile(item.Name) {
				logger.Info("File filtered out by VideoOnly", "name", item.Name, "isVideo", s.IsVideoFile(item.Name))
				continue
			}

			files = append(files, fileResp)
			summary.TotalFiles++
			summary.TotalSize += item.Size
			logger.Info("Added file", "name", item.Name, "isVideo", s.IsVideoFile(item.Name))

			// 媒体分类统计 - 传入完整路径用于路径分类
			s.updateMediaStats(&summary, fileResp.Path, item.Name)
		}
	}

	// 4. 递归处理子目录（如果需要）
	if req.Recursive {
		for _, dir := range directories {
			subReq := req
			subReq.Path = dir.Path
			subReq.Recursive = false // 避免无限递归
			
			subResp, err := s.ListFiles(ctx, subReq)
			if err != nil {
				logger.Warn("Failed to list subdirectory", "path", dir.Path, "error", err)
				continue
			}
			
			files = append(files, subResp.Files...)
			summary.TotalFiles += subResp.Summary.TotalFiles
			summary.TotalSize += subResp.Summary.TotalSize
			summary.VideoFiles += subResp.Summary.VideoFiles
			summary.MovieFiles += subResp.Summary.MovieFiles
			summary.TVFiles += subResp.Summary.TVFiles
			summary.OtherFiles += subResp.Summary.OtherFiles
		}
	}

	// 5. 应用排序
	s.sortFiles(files, req.SortBy, req.SortOrder)

	// 6. 应用分页（对于递归结果）
	if req.Recursive {
		start := (req.Page - 1) * req.PageSize
		end := start + req.PageSize
		if start >= len(files) {
			files = []contracts.FileResponse{}
		} else if end > len(files) {
			files = files[start:]
		} else {
			files = files[start:end]
		}
	}

	// 7. 构建响应
	summary.TotalSizeFormatted = s.FormatFileSize(summary.TotalSize)
	parentPath := s.getParentPath(req.Path)

	return &contracts.FileListResponse{
		Files:       files,
		Directories: directories,
		CurrentPath: req.Path,
		ParentPath:  parentPath,
		TotalCount:  summary.TotalFiles,
		Summary:     summary,
		Pagination: contracts.Pagination{
			Page:     req.Page,
			PageSize: req.PageSize,
			Total:    summary.TotalFiles,
			HasNext:  req.Page*req.PageSize < summary.TotalFiles,
			HasPrev:  req.Page > 1,
		},
	}, nil
}

// SearchFiles 搜索文件
func (s *AppFileService) SearchFiles(ctx context.Context, req contracts.FileSearchRequest) (*contracts.FileListResponse, error) {
	// 简单实现：在指定路径下递归搜索
	searchPath := req.Path
	if searchPath == "" {
		searchPath = s.config.Alist.DefaultPath
		if searchPath == "" {
			searchPath = "/"
		}
	}

	listReq := contracts.FileListRequest{
		Path:      searchPath,
		Recursive: true,
		PageSize:  req.Limit,
	}

	listResp, err := s.ListFiles(ctx, listReq)
	if err != nil {
		return nil, fmt.Errorf("failed to search files: %w", err)
	}

	// 应用搜索过滤
	var filteredFiles []contracts.FileResponse
	query := strings.ToLower(req.Query)

	for _, file := range listResp.Files {
		// 文件名匹配
		if !strings.Contains(strings.ToLower(file.Name), query) {
			continue
		}

		// 文件类型过滤
		if req.FileType != "" && s.GetFileCategory(file.Name) != req.FileType {
			continue
		}

		// 文件大小过滤
		if req.MinSize > 0 && file.Size < req.MinSize {
			continue
		}
		if req.MaxSize > 0 && file.Size > req.MaxSize {
			continue
		}

		// 修改时间过滤
		if req.ModifiedAfter != nil && file.Modified.Before(*req.ModifiedAfter) {
			continue
		}
		if req.ModifiedBefore != nil && file.Modified.After(*req.ModifiedBefore) {
			continue
		}

		filteredFiles = append(filteredFiles, file)
	}

	listResp.Files = filteredFiles
	listResp.TotalCount = len(filteredFiles)
	return listResp, nil
}

// GetFilesByTimeRange 根据时间范围获取文件
func (s *AppFileService) GetFilesByTimeRange(ctx context.Context, req contracts.TimeRangeFileRequest) (*contracts.TimeRangeFileResponse, error) {
	logger.Info("GetFilesByTimeRange called", 
		"path", req.Path,
		"startTime", req.StartTime.Format("2006-01-02 15:04:05 -07:00"), 
		"endTime", req.EndTime.Format("2006-01-02 15:04:05 -07:00"),
		"startUnix", req.StartTime.Unix(),
		"endUnix", req.EndTime.Unix(),
		"videoOnly", req.VideoOnly)

	// 使用自定义递归逻辑，先检查目录时间再决定是否递归
	var filteredFiles []contracts.FileResponse
	err := s.collectFilesInTimeRange(ctx, req.Path, req.StartTime, req.EndTime, req.VideoOnly, &filteredFiles)
	if err != nil {
		return nil, fmt.Errorf("failed to collect files: %w", err)
	}

	logger.Info("Time range filtering completed", "filteredCount", len(filteredFiles))

	// 重新计算摘要
	summary := s.calculateFileSummary(filteredFiles)

	return &contracts.TimeRangeFileResponse{
		Files: filteredFiles,
		TimeRange: contracts.TimeRange{
			Start: req.StartTime,
			End:   req.EndTime,
		},
		Summary: summary,
	}, nil
}

// collectFilesInTimeRange 递归收集在时间范围内的文件
func (s *AppFileService) collectFilesInTimeRange(ctx context.Context, path string, startTime, endTime time.Time, videoOnly bool, result *[]contracts.FileResponse) error {
	logger.Info("Collecting files in path", "path", path)

	// 获取当前目录的文件列表（非递归）
	alistResp, err := s.alistClient.ListFiles(path, 1, 1000)
	if err != nil {
		return fmt.Errorf("failed to list files in %s: %w", path, err)
	}

	for _, item := range alistResp.Data.Content {
		fileResp := s.convertToFileResponse(item, path)
		
		// 检查时间范围
		inTimeRange := utils.IsInRange(fileResp.Modified, startTime, endTime)
		
		logger.Info("Checking item", 
			"name", item.Name, 
			"isDir", item.IsDir,
			"modified", fileResp.Modified.Format("2006-01-02 15:04:05 -07:00"),
			"modifiedUnix", fileResp.Modified.Unix(),
			"inTimeRange", inTimeRange)

		if item.IsDir {
			// 对于目录，如果目录修改时间在范围内，则递归搜索
			if inTimeRange {
				logger.Info("Directory in time range, recursing", "dir", item.Name)
				subPath := utils.JoinPath(path, item.Name)
				err := s.collectFilesInTimeRange(ctx, subPath, startTime, endTime, videoOnly, result)
				if err != nil {
					logger.Warn("Failed to recurse into directory", "dir", item.Name, "error", err)
					// 继续处理其他目录，不因单个目录失败而停止
				}
			} else {
				logger.Info("Directory not in time range, skipping", "dir", item.Name)
			}
		} else {
			// 对于文件，检查时间范围和视频过滤
			if inTimeRange {
				if !videoOnly || s.IsVideoFile(item.Name) {
					logger.Info("File matches criteria, adding", "file", item.Name, "isVideo", s.IsVideoFile(item.Name))
					
					// 为符合条件的文件获取真实的下载URL
					filePath := utils.JoinPath(path, item.Name)
					internalURL, externalURL := s.getRealDownloadURLs(filePath)
					fileResp.InternalURL = internalURL
					fileResp.ExternalURL = externalURL
					logger.Info("🎯 已为时间范围文件获取真实下载URL", "file", item.Name, "url", internalURL)
					
					*result = append(*result, fileResp)
				} else {
					logger.Info("File not video, skipping", "file", item.Name)
				}
			} else {
				logger.Info("File not in time range, skipping", "file", item.Name)
			}
		}
	}

	return nil
}

// GetRecentFiles 获取最近文件
func (s *AppFileService) GetRecentFiles(ctx context.Context, req contracts.RecentFilesRequest) (*contracts.FileListResponse, error) {
	// 使用时间工具创建时间范围
	timeRange := utils.CreateTimeRangeFromHours(req.HoursAgo)

	timeRangeReq := contracts.TimeRangeFileRequest{
		Path:      req.Path,
		StartTime: timeRange.Start,
		EndTime:   timeRange.End,
		VideoOnly: req.VideoOnly,
	}

	timeRangeResp, err := s.GetFilesByTimeRange(ctx, timeRangeReq)
	if err != nil {
		return nil, err
	}

	// 转换为列表响应格式
	files := timeRangeResp.Files
	if req.Limit > 0 && len(files) > req.Limit {
		files = files[:req.Limit]
	}

	return &contracts.FileListResponse{
		Files:       files,
		CurrentPath: req.Path,
		TotalCount:  len(files),
		Summary:     timeRangeResp.Summary,
	}, nil
}

// GetYesterdayFiles 获取昨天的文件
func (s *AppFileService) GetYesterdayFiles(ctx context.Context, path string) (*contracts.FileListResponse, error) {
	// 使用时间工具创建昨天的时间范围
	yesterdayRange := utils.CreateYesterdayRange()

	timeRangeReq := contracts.TimeRangeFileRequest{
		Path:      path,
		StartTime: yesterdayRange.Start,
		EndTime:   yesterdayRange.End,
		VideoOnly: true,
	}

	timeRangeResp, err := s.GetFilesByTimeRange(ctx, timeRangeReq)
	if err != nil {
		return nil, err
	}

	return &contracts.FileListResponse{
		Files:       timeRangeResp.Files,
		CurrentPath: path,
		TotalCount:  len(timeRangeResp.Files),
		Summary:     timeRangeResp.Summary,
	}, nil
}

// sortFiles 文件排序
func (s *AppFileService) sortFiles(files []contracts.FileResponse, sortBy, sortOrder string) {
	if sortBy == "" {
		sortBy = "name"
	}
	if sortOrder == "" {
		sortOrder = "asc"
	}

	sort.Slice(files, func(i, j int) bool {
		var result bool
		switch sortBy {
		case "size":
			result = files[i].Size < files[j].Size
		case "modified":
			result = files[i].Modified.Before(files[j].Modified)
		default: // name
			result = strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
		}

		if sortOrder == "desc" {
			result = !result
		}

		return result
	})
}

// calculateFileSummary 计算文件摘要
func (s *AppFileService) calculateFileSummary(files []contracts.FileResponse) contracts.FileSummary {
	summary := contracts.FileSummary{}

	for _, file := range files {
		summary.TotalFiles++
		summary.TotalSize += file.Size
		// 传入完整路径用于路径分类
		s.updateMediaStats(&summary, file.Path, file.Name)
	}

	summary.TotalSizeFormatted = s.FormatFileSize(summary.TotalSize)
	return summary
}