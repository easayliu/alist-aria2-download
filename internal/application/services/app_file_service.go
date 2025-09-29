package services

import (
	"context"
	"fmt"
	"sort"
	"strconv"
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

// SetDownloadService 设置下载服务（用于解决循环依赖）
func (s *AppFileService) SetDownloadService(downloadService contracts.DownloadService) {
	s.downloadService = downloadService
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
			
			// 如果不是目录，获取实际的raw_url用于下载
			if !item.IsDir {
				logger.Info("🔽 GetFileInfo: 准备获取文件的真实下载URL", "file", fileName, "path", path)
				internalURL, externalURL := s.getRealDownloadURLs(path)
				fileResp.InternalURL = internalURL
				fileResp.ExternalURL = externalURL
				logger.Info("🔽 GetFileInfo: 已更新文件响应的URL", "internal", internalURL, "external", externalURL)
			}
			
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
	logger.Info("📁 开始下载单个文件", "filePath", req.FilePath)
	
	// 检查下载服务是否可用
	if s.downloadService == nil {
		return nil, fmt.Errorf("download service not available")
	}
	
	// 获取文件信息
	fileInfo, err := s.GetFileInfo(ctx, req.FilePath)
	if err != nil {
		logger.Error("❌ 获取文件信息失败", "filePath", req.FilePath, "error", err)
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	logger.Info("📋 文件信息获取成功", 
		"fileName", fileInfo.Name,
		"fileSize", fileInfo.Size,
		"downloadURL", fileInfo.InternalURL)

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

	logger.Info("🚀 准备创建下载任务", 
		"url", downloadReq.URL,
		"filename", downloadReq.Filename,
		"directory", downloadReq.Directory)

	return s.downloadService.CreateDownload(ctx, downloadReq)
}

// DownloadFiles 批量下载文件
func (s *AppFileService) DownloadFiles(ctx context.Context, req contracts.BatchFileDownloadRequest) (*contracts.BatchDownloadResponse, error) {
	// 检查下载服务是否可用
	if s.downloadService == nil {
		return nil, fmt.Errorf("download service not available")
	}
	
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
	// 检查下载服务是否可用
	if s.downloadService == nil {
		return nil, fmt.Errorf("download service not available")
	}
	
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
		// 动态获取真实的下载URL
		logger.Info("📂 获取目录中文件的下载URL", "file", file.Name, "path", file.Path)
		internalURL, _ := s.getRealDownloadURLs(file.Path)
		
		downloadReq := contracts.DownloadRequest{
			URL:          internalURL,
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

// GetMediaType 获取媒体类型（用于统计）
func (s *AppFileService) GetMediaType(filePath string) string {
	// 首先检查路径中的类型指示器（优先级）
	pathCategory := s.GetCategoryFromPath(filePath)
	if pathCategory != "" {
		switch pathCategory {
		case "movie":
			return "movie"
		case "tv":
			return "tv"
		case "variety":
			return "tv" // 综艺节目也算作TV类型
		default:
			return "other"
		}
	}

	// 回退到基于文件名的分类
	filename := utils.GetFileName(filePath)
	category := s.GetFileCategory(filename)
	switch category {
	case "movie":
		return "movie"
	case "tv":
		return "tv"
	case "variety":
		return "tv" // 综艺节目也算作TV类型
	default:
		return "other"
	}
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

	// 首先检查路径中的类型指示器（优先级最高）
	pathCategory := s.GetCategoryFromPath(file.Path)
	logger.Info("🏷️  路径分类分析", "path", file.Path, "pathCategory", pathCategory)
	
	if pathCategory != "" {
		// 对于电视剧，使用智能路径解析和重组
		if pathCategory == "tv" {
			smartPath := s.generateSmartTVPath(file.Path, baseDir)
			if smartPath != "" {
				logger.Info("🎯 使用智能电视剧路径", "file", file.Name, "path", file.Path, "smartPath", smartPath)
				return smartPath
			}
		}
		
		// 提取并保留原始路径结构
		targetDir := s.extractPathStructure(file.Path, pathCategory, baseDir)
		if targetDir != "" {
			logger.Info("✅ 使用路径分类结果（保留目录结构）", "file", file.Name, "path", file.Path, "pathCategory", pathCategory, "targetDir", targetDir)
			return targetDir
		}
	}

	// 如果路径分类失败，直接使用默认目录
	defaultDir := utils.JoinPath(baseDir, "others")
	logger.Info("⚠️  路径分类失败，使用默认目录", "file", file.Name, "path", file.Path, "defaultDir", defaultDir)
	return defaultDir
}

// extractPathStructure 从原始路径中提取并保留目录结构（过滤其他分类关键词）
func (s *AppFileService) extractPathStructure(filePath, pathCategory, baseDir string) string {
	// 将路径转为小写用于匹配
	pathLower := strings.ToLower(filePath)
	
	// 定义所有分类关键词
	allCategoryKeywords := []string{"tvs", "movies", "variety", "show", "综艺", "娱乐", "videos", "video", "视频"}
	
	// 根据分类找到对应的关键词和目标目录
	var keywordFound string
	var targetCategoryDir string
	
	switch pathCategory {
	case "tv":
		targetCategoryDir = "tvs"
		keywordFound = "tvs"
	case "movie":
		targetCategoryDir = "movies"
		keywordFound = "movies"
	case "variety":
		targetCategoryDir = "variety"
		// 对于 variety，选择第一个匹配的关键词
		varietyKeywords := []string{"variety", "show", "综艺", "娱乐"}
		for _, keyword := range varietyKeywords {
			if strings.Contains(pathLower, keyword) {
				keywordFound = keyword
				break
			}
		}
	case "video":
		targetCategoryDir = "videos"
		// 对于 video，选择第一个匹配的关键词
		videoKeywords := []string{"videos", "video", "视频"}
		for _, keyword := range videoKeywords {
			if strings.Contains(pathLower, keyword) {
				keywordFound = keyword
				break
			}
		}
	}
	
	if keywordFound == "" {
		logger.Warn("未找到匹配的关键词", "filePath", filePath, "pathCategory", pathCategory)
		return ""
	}
	
	// 在原始路径中找到关键词的位置（保持原始大小写）
	keywordIndex := strings.Index(pathLower, keywordFound)
	if keywordIndex == -1 {
		logger.Warn("无法在原始路径中找到关键词位置", "filePath", filePath, "keywordFound", keywordFound)
		return ""
	}
	
	// 提取关键词之后的路径部分
	afterKeywordStart := keywordIndex + len(keywordFound)
	if afterKeywordStart < len(filePath) && filePath[afterKeywordStart] == '/' {
		afterKeywordStart++ // 跳过关键词后的 /
	}
	
	afterKeyword := ""
	if afterKeywordStart < len(filePath) {
		afterKeyword = filePath[afterKeywordStart:]
	}
	
	logger.Info("🔍 提取路径片段", "keywordFound", keywordFound, "afterKeyword", afterKeyword)
	
	// 获取文件的父目录（去掉文件名）
	parentDir := utils.GetParentPath(afterKeyword)
	
	// 关键步骤：过滤掉路径中的其他分类关键词
	if parentDir != "" && parentDir != "/" {
		parentDir = s.filterCategoryKeywords(parentDir, allCategoryKeywords)
		logger.Info("🧹 过滤分类关键词后", "originalParentDir", utils.GetParentPath(afterKeyword), "filteredParentDir", parentDir)
	}
	
	// 构建最终路径：baseDir + 分类目录 + 过滤后的目录结构
	if parentDir == "" || parentDir == "/" {
		// 如果没有子目录，直接使用分类目录
		targetDir := utils.JoinPath(baseDir, targetCategoryDir)
		logger.Info("📁 无子目录，使用分类根目录", "targetDir", targetDir)
		return targetDir
	} else {
		// 保留过滤后的子目录结构
		targetDir := utils.JoinPath(baseDir, targetCategoryDir, parentDir)
		logger.Info("✅ 最终下载路径", "targetDir", targetDir)
		return targetDir
	}
}

// filterCategoryKeywords 过滤路径中的分类关键词目录
func (s *AppFileService) filterCategoryKeywords(path string, keywords []string) string {
	if path == "" || path == "/" {
		return path
	}
	
	logger.Info("🧹 开始过滤分类关键词", "originalPath", path, "keywords", keywords)
	
	// 分割路径为目录片段
	parts := strings.Split(strings.Trim(path, "/"), "/")
	var filteredParts []string
	
	for _, part := range parts {
		if part == "" {
			continue
		}
		
		partLower := strings.ToLower(part)
		isKeyword := false
		
		// 检查是否是完全匹配的分类关键词
		for _, keyword := range keywords {
			if partLower == keyword {
				logger.Info("🚫 过滤掉分类关键词目录（完全匹配）", "part", part, "keyword", keyword)
				isKeyword = true
				break
			}
		}
		
		// 如果不是关键词，保留这个目录
		if !isKeyword {
			logger.Info("✅ 保留目录", "part", part)
			filteredParts = append(filteredParts, part)
		}
	}
	
	// 重新组装路径
	if len(filteredParts) == 0 {
		logger.Info("⚠️  所有目录都被过滤，返回空路径")
		return ""
	}
	
	result := strings.Join(filteredParts, "/")
	logger.Info("🔧 路径过滤结果", "original", path, "filtered", result, "removedParts", len(parts)-len(filteredParts))
	return result
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
	logger.Info("Parsing time", "file", item.Name, "modifiedString", item.Modified)
	
	modifiedTime, err := utils.ParseTime(item.Modified)
	if err != nil {
		logger.Warn("Failed to parse time, using zero time", "file", item.Name, "modifiedString", item.Modified, "error", err)
		modifiedTime = time.Time{} // 零值时间
	} else {
		logger.Info("Time parsed successfully", "file", item.Name, "parsedTime", modifiedTime.Format("2006-01-02 15:04:05 -07:00"), "unix", modifiedTime.Unix(), "location", modifiedTime.Location().String())
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
		// 优先使用路径分类，回退到文件名分类
		pathCategory := s.GetCategoryFromPath(fullPath)
		if pathCategory != "" {
			resp.MediaType = pathCategory
			resp.Category = pathCategory
			logger.Info("📁 convertToFileResponse: 使用路径分类", "file", item.Name, "path", fullPath, "category", pathCategory)
		} else {
			// 回退到文件名分类（如果路径分类失败）
			fileCategory := s.GetFileCategory(item.Name)
			resp.MediaType = fileCategory
			resp.Category = fileCategory
			logger.Info("📁 convertToFileResponse: 使用文件名分类", "file", item.Name, "category", fileCategory)
		}
		
		resp.DownloadPath = s.GenerateDownloadPath(resp)
		
		// 直接获取真实的raw_url用于下载（采用延迟加载方式避免性能问题）
		// URL将在实际需要时通过getRealDownloadURLs方法获取
		resp.InternalURL = ""  // 将在需要时填充
		resp.ExternalURL = ""  // 将在需要时填充
	}

	return resp
}

// getRealDownloadURLs 获取实际的下载URL（参考旧实现的简单有效方法）
func (s *AppFileService) getRealDownloadURLs(filePath string) (internalURL, externalURL string) {
	logger.Info("🔍 开始获取文件的raw_url", "path", filePath)
	
	// 确保AList客户端已登录
	if s.alistClient.Token == "" {
		logger.Info("🔑 检测到未登录，开始登录AList", "baseURL", s.alistClient.BaseURL)
		if err := s.alistClient.Login(); err != nil {
			logger.Error("❌ AList登录失败", "error", err)
			fallbackInternal := s.generateInternalURL(filePath)
			fallbackExternal := s.generateExternalURL(filePath)
			logger.Info("🔄 登录失败，使用回退URL", "internal", fallbackInternal, "external", fallbackExternal)
			return fallbackInternal, fallbackExternal
		}
		logger.Info("✅ AList登录成功")
	}
	
	// 获取文件详细信息（包含raw_url）
	fileInfo, err := s.alistClient.GetFileInfo(filePath)
	if err != nil {
		logger.Warn("❌ 获取文件信息失败，使用回退URL", "path", filePath, "error", err)
		fallbackInternal := s.generateInternalURL(filePath)
		fallbackExternal := s.generateExternalURL(filePath)
		logger.Info("🔄 使用回退URL", "internal", fallbackInternal, "external", fallbackExternal)
		return fallbackInternal, fallbackExternal
	}
	
	// 使用旧实现的简单逻辑：直接获取raw_url并做域名替换
	originalURL := fileInfo.Data.RawURL
	logger.Info("🎯 获取到原始raw_url", "raw_url", originalURL)
	
	// 如果raw_url为空，使用回退逻辑
	if originalURL == "" {
		logger.Error("❌ raw_url为空，这不应该发生！", "path", filePath, "fileInfo", fileInfo.Data)
		fallbackInternal := s.generateInternalURL(filePath)
		fallbackExternal := s.generateExternalURL(filePath)
		logger.Error("🔄 使用回退URL", "internal", fallbackInternal, "external", fallbackExternal)
		return fallbackInternal, fallbackExternal
	}
	
	// 采用旧实现的简单替换逻辑：只在包含fcalist-public时替换
	internalURL = originalURL
	externalURL = originalURL
	
	if strings.Contains(originalURL, "fcalist-public") {
		internalURL = strings.ReplaceAll(originalURL, "fcalist-public", "fcalist-internal")
		logger.Info("🔄 URL替换完成（采用旧实现逻辑）", 
			"original", externalURL,
			"internal", internalURL,
			"replacement", "fcalist-public -> fcalist-internal")
	} else {
		logger.Info("ℹ️  无需URL替换", "internal", internalURL, "external", externalURL)
	}
	
	logger.Info("✅ 成功获取下载URL（采用旧实现的简单逻辑）", 
		"path", filePath,
		"internal_url", internalURL, 
		"external_url", externalURL,
		"url_replaced", strings.Contains(originalURL, "fcalist-public"))
	
	return internalURL, externalURL
}

// generateInternalURL 生成内部下载URL（回退方法）
func (s *AppFileService) generateInternalURL(path string) string {
	url := fmt.Sprintf("%s/d%s", s.config.Alist.BaseURL, path)
	logger.Info("🔄 生成回退下载URL", "url", url, "path", path)
	return url
}

// generateExternalURL 生成外部访问URL（回退方法）
func (s *AppFileService) generateExternalURL(path string) string {
	url := fmt.Sprintf("%s/p%s", s.config.Alist.BaseURL, path)
	logger.Info("🔄 生成回退外部URL", "url", url, "path", path)
	return url
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
	
	// 检查 TVs 和 Movies 的位置，选择最早出现的
	tvsIndex := strings.Index(pathLower, "tvs")
	moviesIndex := strings.Index(pathLower, "movies")
	
	// 如果两个都存在，选择最早出现的（路径层级更高的）
	if tvsIndex != -1 && moviesIndex != -1 {
		if tvsIndex < moviesIndex {
			logger.Info("🔍 路径同时包含 tvs 和 movies，选择更早出现的 tvs", "path", path, "tvsIndex", tvsIndex, "moviesIndex", moviesIndex)
			return "tv"
		} else {
			logger.Info("🔍 路径同时包含 tvs 和 movies，选择更早出现的 movies", "path", path, "tvsIndex", tvsIndex, "moviesIndex", moviesIndex)
			return "movie"
		}
	}
	
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

// updateMediaStats 更新媒体统计
func (s *AppFileService) updateMediaStats(summary *contracts.FileSummary, filePath, filename string) {
	if !s.IsVideoFile(filename) {
		summary.OtherFiles++
		return
	}

	summary.VideoFiles++
	
	// 使用 GetMediaType 方法，它会优先使用路径分类，然后回退到文件名分类
	mediaType := s.GetMediaType(filePath)
	logger.Info("📊 文件统计分类", "filePath", filePath, "filename", filename, "mediaType", mediaType)
	
	switch mediaType {
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
		// 传入完整路径用于路径分类
		s.updateMediaStats(&summary, file.Path, file.Name)
	}

	summary.TotalSizeFormatted = s.FormatFileSize(summary.TotalSize)
	return summary
}

// generateSmartTVPath 智能生成电视剧路径，将季度信息规范化
func (s *AppFileService) generateSmartTVPath(filePath, baseDir string) string {
	logger.Info("🎬 开始智能电视剧路径解析", "filePath", filePath)
	
	// 从路径中提取tvs之后的部分
	pathLower := strings.ToLower(filePath)
	tvsIndex := strings.Index(pathLower, "tvs")
	if tvsIndex == -1 {
		logger.Warn("⚠️  路径中未找到tvs关键词", "filePath", filePath)
		return ""
	}
	
	// 提取tvs之后的路径部分
	afterTvs := filePath[tvsIndex+3:] // 跳过"tvs"
	if strings.HasPrefix(afterTvs, "/") {
		afterTvs = afterTvs[1:] // 去掉开头的/
	}
	
	// 分割路径为各个部分
	pathParts := strings.Split(afterTvs, "/")
	if len(pathParts) < 2 {
		logger.Warn("⚠️  电视剧路径结构不完整", "afterTvs", afterTvs, "parts", pathParts)
		return ""
	}
	
	logger.Info("🔍 路径组件分析", "pathParts", pathParts)
	
	// 寻找包含季度信息的目录（从最深层开始检查）
	var smartPath string
	lastIndex := len(pathParts) - 1
	
	// 如果最后一个部分是文件（包含文件扩展名），则排除它
	if strings.Contains(pathParts[lastIndex], ".") {
		lastIndex-- 
	}
	
	for i := lastIndex; i >= 0; i-- {
		currentDir := pathParts[i]
		logger.Info("🔍 检查目录", "index", i, "dir", currentDir)
		
		// 先检查是否包含完整的节目名信息
		extractedShowName := s.extractFullShowName(currentDir)
		if extractedShowName != "" {
			// 检查是否是"宝藏行"或其他特殊系列（包含更多信息）
			if strings.Contains(extractedShowName, "宝藏行") || strings.Contains(extractedShowName, "公益季") {
				// 对于特殊系列，直接使用完整节目名
				smartPath = utils.JoinPath(baseDir, "tvs", extractedShowName)
				logger.Info("✅ 使用完整特殊节目名", 
					"原路径", filePath,
					"完整节目名", extractedShowName,
					"智能路径", smartPath)
				return smartPath
			}
		}
		
		// 尝试从当前目录提取季度信息并生成规范化路径
		seasonNumber := s.extractSeasonNumber(currentDir)
		if seasonNumber > 0 {
			// 使用第一层目录作为基础节目名，生成 节目名/S##
			baseShowName := pathParts[0]
			seasonCode := fmt.Sprintf("S%02d", seasonNumber)
			smartPath = utils.JoinPath(baseDir, "tvs", baseShowName, seasonCode)
			
			logger.Info("✅ 从目录生成季度路径", 
				"原路径", filePath,
				"基础节目名", baseShowName,
				"季度目录", currentDir,
				"季度", seasonNumber,
				"季度代码", seasonCode,
				"智能路径", smartPath)
			
			return smartPath
		}
		
		// 最后检查其他完整节目名
		if extractedShowName != "" {
			// 直接使用提取的完整节目名作为最终目录
			smartPath = utils.JoinPath(baseDir, "tvs", extractedShowName)
			
			logger.Info("✅ 使用完整节目名生成路径", 
				"原路径", filePath,
				"目标目录", currentDir,
				"提取节目名", extractedShowName,
				"智能路径", smartPath)
			
			return smartPath
		}
	}
	
	// 如果上述方法失败，尝试传统的季度解析方法
	showName := pathParts[0]
	seasonDir := pathParts[1]
	
	logger.Info("🔄 回退到传统解析", "showName", showName, "seasonDir", seasonDir)
	
	// 解析季度信息
	seasonNumber := s.extractSeasonNumber(seasonDir)
	if seasonNumber > 0 {
		// 构建规范化路径：/downloads/tvs/节目名/S##
		seasonCode := fmt.Sprintf("S%02d", seasonNumber)
		smartPath = utils.JoinPath(baseDir, "tvs", showName, seasonCode)
		
		logger.Info("✅ 传统方法生成路径", 
			"原路径", filePath,
			"节目名", showName, 
			"季度", seasonNumber,
			"季度代码", seasonCode,
			"智能路径", smartPath)
		
		return smartPath
	}
	
	logger.Info("⚠️  未能解析季度信息，使用原始逻辑", "seasonDir", seasonDir)
	return ""
}

// extractSeasonNumber 从目录名中提取季度编号
func (s *AppFileService) extractSeasonNumber(dirName string) int {
	if dirName == "" {
		return 0
	}
	
	dirLower := strings.ToLower(dirName)
	
	// 匹配各种季度格式
	patterns := []struct {
		pattern string
		extract func(string) int
	}{
		// 第X季 格式
		{"第", func(s string) int {
			if idx := strings.Index(s, "第"); idx != -1 {
				after := s[idx+len("第"):]
				if seasonIdx := strings.Index(after, "季"); seasonIdx != -1 {
					seasonStr := after[:seasonIdx]
					// 转换中文数字或阿拉伯数字
					return chineseOrArabicToNumber(seasonStr)
				}
			}
			return 0
		}},
		// Season X 格式
		{"season", func(s string) int {
			if idx := strings.Index(s, "season"); idx != -1 {
				after := strings.TrimSpace(s[idx+6:])
				// 提取数字部分
				var numStr string
				for _, char := range after {
					if char >= '0' && char <= '9' {
						numStr += string(char)
					} else {
						break
					}
				}
				if num, err := strconv.Atoi(numStr); err == nil && num > 0 {
					return num
				}
			}
			return 0
		}},
		// SXX 格式
		{"s", func(s string) int {
			if len(s) >= 2 && s[0] == 's' {
				numStr := ""
				for i := 1; i < len(s) && i < 4; i++ { // 最多取3位数字
					if s[i] >= '0' && s[i] <= '9' {
						numStr += string(s[i])
					} else {
						break
					}
				}
				if num, err := strconv.Atoi(numStr); err == nil && num > 0 {
					return num
				}
			}
			return 0
		}},
		// 直接包含年份+季度信息，如"极限挑战第9季2023"
		{"", func(s string) int {
			// 查找"第X季"模式
			for i := 0; i < len(s)-1; i++ {
				if s[i:i+1] == "第" && i+2 < len(s) && s[i+2:i+3] == "季" {
					seasonChar := s[i+1 : i+2]
					return chineseOrArabicToNumber(seasonChar)
				}
			}
			return 0
		}},
	}
	
	// 尝试各种模式
	for _, pattern := range patterns {
		if pattern.pattern == "" || strings.Contains(dirLower, pattern.pattern) {
			if num := pattern.extract(dirLower); num > 0 {
				logger.Info("🎯 成功提取季度编号", "dirName", dirName, "pattern", pattern.pattern, "seasonNumber", num)
				return num
			}
		}
	}
	
	logger.Info("⚠️  无法从目录名提取季度编号", "dirName", dirName)
	return 0
}

// extractFullShowName 提取完整的节目名（包含季度信息）
func (s *AppFileService) extractFullShowName(dirName string) string {
	if dirName == "" {
		return ""
	}
	
	logger.Info("🔍 分析节目名", "dirName", dirName)
	
	// 检查是否包含季度关键词，如果包含则认为这是完整的节目名
	seasonKeywords := []string{"第", "季", "season", "宝藏行", "公益季"}
	hasSeasonInfo := false
	
	dirLower := strings.ToLower(dirName)
	for _, keyword := range seasonKeywords {
		if strings.Contains(dirLower, strings.ToLower(keyword)) {
			hasSeasonInfo = true
			logger.Info("🎯 发现季度关键词", "dirName", dirName, "keyword", keyword)
			break
		}
	}
	
	if hasSeasonInfo {
		// 清理目录名，移除不必要的后缀信息
		cleanName := s.cleanShowName(dirName)
		if cleanName != "" {
			logger.Info("✅ 提取完整节目名", "原目录名", dirName, "清理后", cleanName)
			return cleanName
		}
	}
	
	logger.Info("⚠️  目录不包含季度信息", "dirName", dirName)
	return ""
}

// cleanShowName 清理节目名，移除不必要的后缀信息
func (s *AppFileService) cleanShowName(showName string) string {
	if showName == "" {
		return ""
	}
	
	// 移除常见的后缀信息
	suffixesToRemove := []string{
		"（", "(", // 移除括号及之后的内容
		"2021", "2022", "2023", "2024", "2025", // 移除年份
		"全", "期全", "完结", "[", "【", // 移除完结标记
	}
	
	cleaned := showName
	for _, suffix := range suffixesToRemove {
		if idx := strings.Index(cleaned, suffix); idx != -1 {
			cleaned = cleaned[:idx]
			logger.Info("🧹 移除后缀", "原名", showName, "后缀", suffix, "清理后", cleaned)
		}
	}
	
	// 去除前后空白
	cleaned = strings.TrimSpace(cleaned)
	
	// 如果清理后为空或太短，返回原名
	if len(cleaned) < 3 {
		logger.Info("⚠️  清理后名称太短，使用原名", "cleaned", cleaned, "original", showName)
		return showName
	}
	
	logger.Info("✅ 节目名清理完成", "原名", showName, "清理后", cleaned)
	return cleaned
}

// chineseOrArabicToNumber 转换中文数字或阿拉伯数字为整数
func chineseOrArabicToNumber(str string) int {
	if str == "" {
		return 0
	}
	
	// 先尝试直接转换阿拉伯数字
	if num, err := strconv.Atoi(str); err == nil {
		return num
	}
	
	// 转换中文数字
	chineseNumbers := map[string]int{
		"一": 1, "二": 2, "三": 3, "四": 4, "五": 5,
		"六": 6, "七": 7, "八": 8, "九": 9, "十": 10,
		"1": 1, "2": 2, "3": 3, "4": 4, "5": 5,
		"6": 6, "7": 7, "8": 8, "9": 9,
	}
	
	if num, exists := chineseNumbers[str]; exists {
		return num
	}
	
	return 0
}