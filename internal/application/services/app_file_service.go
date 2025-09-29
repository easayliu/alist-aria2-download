package services

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/easayliu/alist-aria2-download/internal/application/contracts"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/alist"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/config"
	"github.com/easayliu/alist-aria2-download/pkg/logger"
	"github.com/easayliu/alist-aria2-download/pkg/utils"
)

// AppFileService 应用层文件服务 - 负责文件业务流程编排
type AppFileService struct {
	config        *config.Config
	alistClient   *alist.Client
	downloadService contracts.DownloadService
}

// NewAppFileService 创建应用文件服务
func NewAppFileService(cfg *config.Config, downloadService contracts.DownloadService) contracts.FileService {
	return &AppFileService{
		config:        cfg,
		alistClient:   alist.NewClient(cfg.Alist.BaseURL, cfg.Alist.Username, cfg.Alist.Password),
		downloadService: downloadService,
	}
}

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

	// 2. 从AList获取文件列表
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
		} else {
			// 应用视频过滤
			if req.VideoOnly && !s.IsVideoFile(item.Name) {
				continue
			}

			files = append(files, fileResp)
			summary.TotalFiles++
			summary.TotalSize += item.Size

			// 媒体分类统计
			s.updateMediaStats(&summary, item.Name)
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

// GetFileInfo 获取文件详细信息
func (s *AppFileService) GetFileInfo(ctx context.Context, path string) (*contracts.FileResponse, error) {
	// 从路径中提取目录和文件名
	parentDir := utils.GetParentPath(path)
	fileName := utils.GetFileName(path)

	// 获取父目录列表
	listResp, err := s.alistClient.ListFiles(parentDir, 1, 1000)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	// 查找目标文件
	for _, item := range listResp.Data.Content {
		if item.Name == fileName {
			fileResp := s.convertToFileResponse(item, parentDir)
			return &fileResp, nil
		}
	}

	return nil, fmt.Errorf("file not found: %s", path)
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
	// 获取所有文件
	listReq := contracts.FileListRequest{
		Path:      req.Path,
		Recursive: true,
		VideoOnly: req.VideoOnly,
		PageSize:  10000, // 大页面，获取所有文件
	}

	listResp, err := s.ListFiles(ctx, listReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get files: %w", err)
	}

	// 按时间范围过滤
	var filteredFiles []contracts.FileResponse
	for _, file := range listResp.Files {
		if file.Modified.After(req.StartTime) && file.Modified.Before(req.EndTime) {
			filteredFiles = append(filteredFiles, file)
		}
	}

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

// GetRecentFiles 获取最近文件
func (s *AppFileService) GetRecentFiles(ctx context.Context, req contracts.RecentFilesRequest) (*contracts.FileListResponse, error) {
	endTime := time.Now()
	startTime := endTime.Add(-time.Duration(req.HoursAgo) * time.Hour)

	timeRangeReq := contracts.TimeRangeFileRequest{
		Path:      req.Path,
		StartTime: startTime,
		EndTime:   endTime,
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
	now := time.Now()
	startOfYesterday := time.Date(now.Year(), now.Month(), now.Day()-1, 0, 0, 0, 0, now.Location())
	endOfYesterday := startOfYesterday.Add(24 * time.Hour)

	_ = contracts.RecentFilesRequest{
		Path:      path,
		HoursAgo:  24,
		VideoOnly: true,
	}

	// 手动设置时间范围为昨天
	timeRangeReq := contracts.TimeRangeFileRequest{
		Path:      path,
		StartTime: startOfYesterday,
		EndTime:   endOfYesterday,
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

// ClassifyFiles 文件分类
func (s *AppFileService) ClassifyFiles(ctx context.Context, req contracts.FileClassificationRequest) (*contracts.FileClassificationResponse, error) {
	classified := make(map[string][]contracts.FileResponse)
	summary := contracts.ClassificationSummary{
		Categories: make(map[string]int),
	}

	for _, file := range req.Files {
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

	return &contracts.FileClassificationResponse{
		ClassifiedFiles: classified,
		Summary:         summary,
	}, nil
}

// GetFilesByCategory 根据分类获取文件
func (s *AppFileService) GetFilesByCategory(ctx context.Context, path string, category string) (*contracts.FileListResponse, error) {
	listReq := contracts.FileListRequest{
		Path:      path,
		Recursive: true,
		PageSize:  10000,
	}

	listResp, err := s.ListFiles(ctx, listReq)
	if err != nil {
		return nil, err
	}

	// 按分类过滤
	var filteredFiles []contracts.FileResponse
	for _, file := range listResp.Files {
		if s.GetFileCategory(file.Name) == category {
			filteredFiles = append(filteredFiles, file)
		}
	}

	listResp.Files = filteredFiles
	listResp.TotalCount = len(filteredFiles)
	listResp.Summary = s.calculateFileSummary(filteredFiles)

	return listResp, nil
}

// DownloadFile 下载单个文件
func (s *AppFileService) DownloadFile(ctx context.Context, req contracts.FileDownloadRequest) (*contracts.DownloadResponse, error) {
	// 获取文件信息
	fileInfo, err := s.GetFileInfo(ctx, req.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	// 构建下载请求
	downloadReq := contracts.DownloadRequest{
		URL:          fileInfo.InternalURL,
		Filename:     fileInfo.Name,
		Directory:    req.TargetDir,
		Options:      req.Options,
		AutoClassify: req.AutoClassify,
	}

	if downloadReq.Directory == "" {
		downloadReq.Directory = s.GenerateDownloadPath(*fileInfo)
	}

	return s.downloadService.CreateDownload(ctx, downloadReq)
}

// DownloadFiles 批量下载文件
func (s *AppFileService) DownloadFiles(ctx context.Context, req contracts.BatchFileDownloadRequest) (*contracts.BatchDownloadResponse, error) {
	var downloadRequests []contracts.DownloadRequest

	for _, fileReq := range req.Files {
		fileInfo, err := s.GetFileInfo(ctx, fileReq.FilePath)
		if err != nil {
			logger.Warn("Failed to get file info", "path", fileReq.FilePath, "error", err)
			continue
		}

		downloadReq := contracts.DownloadRequest{
			URL:          fileInfo.InternalURL,
			Filename:     fileInfo.Name,
			Directory:    fileReq.TargetDir,
			Options:      fileReq.Options,
			AutoClassify: fileReq.AutoClassify,
		}

		// 应用全局设置
		if req.TargetDir != "" && downloadReq.Directory == "" {
			downloadReq.Directory = req.TargetDir
		}
		if req.AutoClassify {
			downloadReq.AutoClassify = true
		}

		if downloadReq.Directory == "" {
			downloadReq.Directory = s.GenerateDownloadPath(*fileInfo)
		}

		downloadRequests = append(downloadRequests, downloadReq)
	}

	batchReq := contracts.BatchDownloadRequest{
		Items:        downloadRequests,
		Directory:    req.TargetDir,
		VideoOnly:    req.VideoOnly,
		AutoClassify: req.AutoClassify,
	}

	return s.downloadService.CreateBatchDownload(ctx, batchReq)
}

// DownloadDirectory 下载目录
func (s *AppFileService) DownloadDirectory(ctx context.Context, req contracts.DirectoryDownloadRequest) (*contracts.BatchDownloadResponse, error) {
	// 获取目录下的所有文件
	listReq := contracts.FileListRequest{
		Path:      req.DirectoryPath,
		Recursive: req.Recursive,
		VideoOnly: req.VideoOnly,
		PageSize:  10000,
	}

	listResp, err := s.ListFiles(ctx, listReq)
	if err != nil {
		return nil, fmt.Errorf("failed to list directory: %w", err)
	}

	// 转换为下载请求
	var downloadRequests []contracts.DownloadRequest
	for _, file := range listResp.Files {
		downloadReq := contracts.DownloadRequest{
			URL:          file.InternalURL,
			Filename:     file.Name,
			Directory:    req.TargetDir,
			AutoClassify: req.AutoClassify,
		}

		if downloadReq.Directory == "" {
			downloadReq.Directory = s.GenerateDownloadPath(file)
		}

		downloadRequests = append(downloadRequests, downloadReq)
	}

	batchReq := contracts.BatchDownloadRequest{
		Items:        downloadRequests,
		Directory:    req.TargetDir,
		VideoOnly:    req.VideoOnly,
		AutoClassify: req.AutoClassify,
	}

	return s.downloadService.CreateBatchDownload(ctx, batchReq)
}

// IsVideoFile 检查是否为视频文件
func (s *AppFileService) IsVideoFile(filename string) bool {
	if filename == "" {
		return false
	}

	ext := strings.ToLower(filename)
	if idx := strings.LastIndex(ext, "."); idx != -1 {
		ext = ext[idx+1:]
	}

	for _, videoExt := range s.config.Download.VideoExts {
		if ext == strings.ToLower(videoExt) {
			return true
		}
	}

	return false
}

// GetFileCategory 获取文件分类
func (s *AppFileService) GetFileCategory(filename string) string {
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

// FormatFileSize 格式化文件大小
func (s *AppFileService) FormatFileSize(size int64) string {
	return utils.FormatFileSize(size)
}

// GenerateDownloadPath 生成下载路径
func (s *AppFileService) GenerateDownloadPath(file contracts.FileResponse) string {
	baseDir := s.config.Aria2.DownloadDir
	if baseDir == "" {
		baseDir = "/downloads"
	}

	// 首先检查路径中的类型指示器（优先级）
	pathCategory := s.GetCategoryFromPath(file.Path)
	if pathCategory != "" {
		switch pathCategory {
		case "movie":
			return utils.JoinPath(baseDir, "movies")
		case "tv":
			return utils.JoinPath(baseDir, "tvs")  // 使用 tvs 目录匹配用户期望
		case "variety":
			return utils.JoinPath(baseDir, "variety")
		case "video":
			return utils.JoinPath(baseDir, "videos")
		}
	}

	// 回退到基于文件名的分类
	category := s.GetFileCategory(file.Name)
	switch category {
	case "movie":
		return utils.JoinPath(baseDir, "movies")
	case "tv":
		return utils.JoinPath(baseDir, "tvs")  // 统一使用 tvs 目录
	case "variety":
		return utils.JoinPath(baseDir, "variety")
	case "video":
		return utils.JoinPath(baseDir, "videos")
	default:
		return utils.JoinPath(baseDir, "others")
	}
}

// GetStorageInfo 获取存储信息
func (s *AppFileService) GetStorageInfo(ctx context.Context, path string) (map[string]interface{}, error) {
	// 获取目录统计信息
	listReq := contracts.FileListRequest{
		Path:      path,
		Recursive: true,
		PageSize:  10000,
	}

	listResp, err := s.ListFiles(ctx, listReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get storage info: %w", err)
	}

	return map[string]interface{}{
		"path":              path,
		"total_files":       listResp.Summary.TotalFiles,
		"total_directories": listResp.Summary.TotalDirs,
		"total_size":        listResp.Summary.TotalSize,
		"total_size_formatted": listResp.Summary.TotalSizeFormatted,
		"video_files":       listResp.Summary.VideoFiles,
		"movie_files":       listResp.Summary.MovieFiles,
		"tv_files":          listResp.Summary.TVFiles,
		"other_files":       listResp.Summary.OtherFiles,
	}, nil
}

// ========== 私有方法 ==========

// convertToFileResponse 转换AList文件对象到响应格式
func (s *AppFileService) convertToFileResponse(item alist.FileItem, basePath string) contracts.FileResponse {
	fullPath := utils.JoinPath(basePath, item.Name)
	
	// 解析修改时间
	modifiedTime, err := time.Parse("2006-01-02T15:04:05.999999999Z", item.Modified)
	if err != nil {
		// 尝试其他时间格式
		modifiedTime, _ = time.Parse("2006-01-02 15:04:05", item.Modified)
	}
	
	resp := contracts.FileResponse{
		Name:          item.Name,
		Path:          fullPath,
		Size:          item.Size,
		SizeFormatted: s.FormatFileSize(item.Size),
		Modified:      modifiedTime,
		IsDir:         item.IsDir,
	}

	if !item.IsDir {
		resp.MediaType = s.GetFileCategory(item.Name)
		resp.Category = resp.MediaType
		resp.DownloadPath = s.GenerateDownloadPath(resp)
		resp.InternalURL = s.generateInternalURL(fullPath)
		resp.ExternalURL = s.generateExternalURL(fullPath)
	}

	return resp
}

// generateInternalURL 生成内部下载URL
func (s *AppFileService) generateInternalURL(path string) string {
	return fmt.Sprintf("%s/d%s", s.config.Alist.BaseURL, path)
}

// generateExternalURL 生成外部访问URL
func (s *AppFileService) generateExternalURL(path string) string {
	return fmt.Sprintf("%s/p%s", s.config.Alist.BaseURL, path)
}

// getParentPath 获取父路径
func (s *AppFileService) getParentPath(path string) string {
	if path == "/" || path == "" {
		return ""
	}
	return utils.GetParentPath(path)
}

// GetCategoryFromPath 从路径中分析文件类型（优先级高于文件名分析）
func (s *AppFileService) GetCategoryFromPath(path string) string {
	if path == "" {
		return ""
	}

	// 将路径转为小写以便匹配
	pathLower := strings.ToLower(path)
	
	// 电视剧类型指示器 - 优先级最高
	tvPathKeywords := []string{"/tvs/", "/tv/", "/series/", "/电视剧/", "/连续剧/", "/剧集/"}
	for _, keyword := range tvPathKeywords {
		if strings.Contains(pathLower, keyword) {
			return "tv"
		}
	}

	// 电影类型指示器
	moviePathKeywords := []string{"/movies/", "/movie/", "/film/", "/电影/", "/影片/"}
	for _, keyword := range moviePathKeywords {
		if strings.Contains(pathLower, keyword) {
			return "movie"
		}
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

// updateMediaStats 更新媒体统计
func (s *AppFileService) updateMediaStats(summary *contracts.FileSummary, filename string) {
	if !s.IsVideoFile(filename) {
		summary.OtherFiles++
		return
	}

	summary.VideoFiles++
	category := s.GetFileCategory(filename)
	switch category {
	case "movie":
		summary.MovieFiles++
	case "tv":
		summary.TVFiles++
	default:
		summary.OtherFiles++
	}
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
		s.updateMediaStats(&summary, file.Name)
	}

	summary.TotalSizeFormatted = s.FormatFileSize(summary.TotalSize)
	return summary
}