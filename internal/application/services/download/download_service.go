package download

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/easayliu/alist-aria2-download/internal/application/contracts"
	pathservices "github.com/easayliu/alist-aria2-download/internal/application/services/path"
	"github.com/easayliu/alist-aria2-download/internal/domain/valueobjects"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/aria2"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/config"
	"github.com/easayliu/alist-aria2-download/pkg/logger"
	strutil "github.com/easayliu/alist-aria2-download/pkg/utils/string"
	fileutil "github.com/easayliu/alist-aria2-download/pkg/utils/file"
)

// AppDownloadService 应用层下载服务 - 负责业务流程编排
type AppDownloadService struct {
	config       *config.Config
	aria2Client  *aria2.Client
	fileService  contracts.FileService
	pathStrategy *pathservices.PathStrategyService // 路径策略服务
}

// NewAppDownloadService 创建应用下载服务
func NewAppDownloadService(cfg *config.Config, fileService contracts.FileService) contracts.DownloadService {
	service := &AppDownloadService{
		config:      cfg,
		aria2Client: aria2.NewClient(cfg.Aria2.RpcURL, cfg.Aria2.Token),
		fileService: fileService,
	}

	// 初始化路径策略服务（需要fileService）
	if fileService != nil {
		service.pathStrategy = pathservices.NewPathStrategyService(cfg, fileService)
	}

	return service
}

// CreateDownload 创建下载任务 - 统一的业务逻辑
func (s *AppDownloadService) CreateDownload(ctx context.Context, req contracts.DownloadRequest) (*contracts.DownloadResponse, error) {
	logger.Info("Creating download", "url", req.URL, "filename", req.Filename, "directory", req.Directory)

	// 1. 参数验证
	if err := s.validateDownloadRequest(req); err != nil {
		logger.Error("Download request validation failed", "url", req.URL, "filename", req.Filename, "error", err)
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// 2. 应用业务规则
	if err := s.applyBusinessRules(&req); err != nil {
		return nil, fmt.Errorf("business rule violation: %w", err)
	}

	// 3. 准备下载选项
	options := s.prepareDownloadOptions(req)

	// 4. 创建Aria2下载任务
	gid, err := s.aria2Client.AddURI(req.URL, options)
	if err != nil {
		logger.Error("Failed to create aria2 download", "error", err, "url", req.URL)
		return nil, fmt.Errorf("failed to create download: %w", err)
	}

	// 5. 构建响应
	response := &contracts.DownloadResponse{
		ID:        gid,
		URL:       req.URL,
		Filename:  s.extractFilename(req.Filename, req.URL),
		Directory: s.resolveDirectory(req.Directory),
		Status:    valueobjects.DownloadStatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	logger.Info("Download created successfully", "id", gid, "filename", response.Filename)
	return response, nil
}

// GetDownload 获取下载状态
func (s *AppDownloadService) GetDownload(ctx context.Context, id string) (*contracts.DownloadResponse, error) {
	status, err := s.aria2Client.GetStatus(id)
	if err != nil {
		return nil, fmt.Errorf("failed to get download status: %w", err)
	}

	return s.convertToDownloadResponse(status), nil
}

// ListDownloads 获取下载列表
func (s *AppDownloadService) ListDownloads(ctx context.Context, req contracts.DownloadListRequest) (*contracts.DownloadListResponse, error) {
	// 并行获取各种状态的下载
	active, err := s.aria2Client.GetActive()
	if err != nil {
		return nil, fmt.Errorf("failed to get active downloads: %w", err)
	}

	waiting, err := s.aria2Client.GetWaiting(req.Offset, req.Limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get waiting downloads: %w", err)
	}

	stopped, err := s.aria2Client.GetStopped(req.Offset, req.Limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get stopped downloads: %w", err)
	}

	globalStats, err := s.aria2Client.GetGlobalStat()
	if err != nil {
		logger.Warn("Failed to get global stats", "error", err)
		globalStats = make(map[string]interface{})
	}

	// 转换并合并数据
	var downloads []contracts.DownloadResponse
	for _, d := range active {
		downloads = append(downloads, s.convertAriaDownloadToResponse(d))
	}
	for _, d := range waiting {
		downloads = append(downloads, s.convertAriaDownloadToResponse(d))
	}
	for _, d := range stopped {
		downloads = append(downloads, s.convertAriaDownloadToResponse(d))
	}

	// 应用过滤和排序
	downloads = s.filterDownloads(downloads, req)
	downloads = s.sortDownloads(downloads, req.SortBy, req.SortOrder)

	return &contracts.DownloadListResponse{
		Downloads:   downloads,
		TotalCount:  len(downloads),
		ActiveCount: len(active),
		GlobalStats: globalStats,
	}, nil
}

// PauseDownload 暂停下载
func (s *AppDownloadService) PauseDownload(ctx context.Context, id string) error {
	if err := s.aria2Client.Pause(id); err != nil {
		return fmt.Errorf("failed to pause download: %w", err)
	}
	logger.Info("Download paused", "id", id)
	return nil
}

// ResumeDownload 恢复下载
func (s *AppDownloadService) ResumeDownload(ctx context.Context, id string) error {
	if err := s.aria2Client.Resume(id); err != nil {
		return fmt.Errorf("failed to resume download: %w", err)
	}
	logger.Info("Download resumed", "id", id)
	return nil
}

// CancelDownload 取消下载
func (s *AppDownloadService) CancelDownload(ctx context.Context, id string) error {
	if err := s.aria2Client.Remove(id); err != nil {
		return fmt.Errorf("failed to cancel download: %w", err)
	}
	logger.Info("Download cancelled", "id", id)
	return nil
}

// RetryDownload 重试下载
func (s *AppDownloadService) RetryDownload(ctx context.Context, id string) (*contracts.DownloadResponse, error) {
	// 获取原始下载信息
	originalStatus, err := s.aria2Client.GetStatus(id)
	if err != nil {
		return nil, fmt.Errorf("failed to get original download: %w", err)
	}

	// 提取URL和选项
	var url string
	if len(originalStatus.Files) > 0 && len(originalStatus.Files[0].URI) > 0 {
		// 这里需要从Files中提取原始URL，实际实现可能需要存储原始URL
		url = originalStatus.Files[0].URI[0].URI
	}

	// 重新创建下载
	req := contracts.DownloadRequest{
		URL:      url,
		Filename: originalStatus.Files[0].Path,
	}

	return s.CreateDownload(ctx, req)
}

// CreateBatchDownload 批量创建下载
func (s *AppDownloadService) CreateBatchDownload(ctx context.Context, req contracts.BatchDownloadRequest) (*contracts.BatchDownloadResponse, error) {
	var results []contracts.DownloadResult
	var successCount, failureCount int
	summary := contracts.DownloadSummary{}

	// 磁盘空间预检功能已移除，交由 Aria2 处理

	for _, item := range req.Items {
		// 应用批量下载的全局设置
		if req.Directory != "" && item.Directory == "" {
			item.Directory = req.Directory
		}
		if req.VideoOnly {
			item.VideoOnly = true
		}
		if req.AutoClassify {
			item.AutoClassify = true
		}

		// 创建单个下载
		download, err := s.CreateDownload(ctx, item)
		result := contracts.DownloadResult{
			Request: item,
		}

		if err != nil {
			result.Success = false
			result.Error = err.Error()
			failureCount++
		} else {
			result.Success = true
			result.Download = download
			successCount++
			
			// 更新摘要统计 - 使用最终下载目录路径进行正确分类
			summary.TotalFiles++
			if s.isVideoFile(download.Filename) {
				summary.VideoFiles++
				// 使用最终的下载目录路径来判断分类
				downloadDir := strings.ToLower(download.Directory)
				if strings.Contains(downloadDir, "movies") {
					summary.MovieFiles++
				} else if strings.Contains(downloadDir, "tvs") {
					summary.TVFiles++
				} else {
					summary.OtherFiles++
				}
			} else {
				summary.OtherFiles++
			}
		}

		results = append(results, result)
	}

	return &contracts.BatchDownloadResponse{
		SuccessCount: successCount,
		FailureCount: failureCount,
		Results:      results,
		Summary:      summary,
	}, nil
}

// PauseAllDownloads 暂停所有下载
func (s *AppDownloadService) PauseAllDownloads(ctx context.Context) error {
	if err := s.aria2Client.PauseAll(); err != nil {
		return fmt.Errorf("failed to pause all downloads: %w", err)
	}
	logger.Info("All downloads paused")
	return nil
}

// ResumeAllDownloads 恢复所有下载
func (s *AppDownloadService) ResumeAllDownloads(ctx context.Context) error {
	if err := s.aria2Client.UnpauseAll(); err != nil {
		return fmt.Errorf("failed to resume all downloads: %w", err)
	}
	logger.Info("All downloads resumed")
	return nil
}

// GetSystemStatus 获取系统状态
func (s *AppDownloadService) GetSystemStatus(ctx context.Context) (map[string]interface{}, error) {
	// 检查Aria2连接
	globalStat, err := s.aria2Client.GetGlobalStat()
	aria2Status := "offline"
	if err == nil {
		aria2Status = "online"
	}

	// 获取版本信息
	version, err := s.aria2Client.GetVersion()
	versionStr := "unknown"
	if err == nil {
		versionStr = version.Version
	}

	return map[string]interface{}{
		"aria2": map[string]interface{}{
			"status":      aria2Status,
			"version":     versionStr,
			"global_stat": globalStat,
		},
		"service": map[string]interface{}{
			"name":    "download_service",
			"version": "2.0.0",
			"status":  "running",
		},
		"config": map[string]interface{}{
			"download_dir": s.config.Aria2.DownloadDir,
			"video_only":   s.config.Download.VideoOnly,
		},
	}, nil
}

// GetDownloadStatistics 获取下载统计
func (s *AppDownloadService) GetDownloadStatistics(ctx context.Context) (map[string]interface{}, error) {
	// 获取所有下载状态
	active, _ := s.aria2Client.GetActive()
	waiting, _ := s.aria2Client.GetWaiting(0, 1000)
	stopped, _ := s.aria2Client.GetStopped(0, 1000)

	var totalSize, completedSize int64
	var videoCount, movieCount, tvCount int

	// 统计活动下载
	for _, download := range active {
		// 这里需要根据实际的Aria2响应结构来实现统计逻辑
		if len(download.Files) > 0 {
			if s.isVideoFile(download.Files[0].Path) {
				videoCount++
				if s.isMovieFile(download.Files[0].Path) {
					movieCount++
				} else if s.isTVFile(download.Files[0].Path) {
					tvCount++
				}
			}
		}
	}

	return map[string]interface{}{
		"total_downloads": len(active) + len(waiting) + len(stopped),
		"active":          len(active),
		"waiting":         len(waiting),
		"completed":       len(stopped),
		"total_size":      totalSize,
		"completed_size":  completedSize,
		"video_files":     videoCount,
		"movie_files":     movieCount,
		"tv_files":        tvCount,
	}, nil
}

// ========== 私有方法 ==========

// validateDownloadRequest 验证下载请求
func (s *AppDownloadService) validateDownloadRequest(req contracts.DownloadRequest) error {
	if req.URL == "" {
		return fmt.Errorf("URL is required")
	}
	if !strings.HasPrefix(req.URL, "http") {
		return fmt.Errorf("invalid URL format")
	}
	return nil
}

// applyBusinessRules 应用业务规则
func (s *AppDownloadService) applyBusinessRules(req *contracts.DownloadRequest) error {
	// 应用视频过滤规则
	if s.config.Download.VideoOnly || req.VideoOnly {
		if req.Filename != "" && !s.isVideoFile(req.Filename) {
			return fmt.Errorf("only video files are allowed")
		}
	}
	return nil
}

// prepareDownloadOptions 准备下载选项
func (s *AppDownloadService) prepareDownloadOptions(req contracts.DownloadRequest) map[string]interface{} {
	options := make(map[string]interface{})

	// 合并用户选项
	for k, v := range req.Options {
		options[k] = v
	}

	// 设置下载目录
	downloadDir := ""
	if req.Directory != "" {
		downloadDir = req.Directory
	} else if s.config.Aria2.DownloadDir != "" {
		downloadDir = s.config.Aria2.DownloadDir
	}

	// 路径清理和规范化（如果启用了路径策略服务）
	if s.pathStrategy != nil && downloadDir != "" {
		// 清理路径中的特殊字符
		cleanDir := s.pathStrategy.CleanPath(downloadDir)
		if cleanDir != downloadDir {
			logger.Debug("Path cleaned", "original", downloadDir, "cleaned", cleanDir)
			downloadDir = cleanDir
		}
	}

	options["dir"] = downloadDir

	// 设置文件名
	if req.Filename != "" {
		options["out"] = req.Filename
	}

	logger.Debug("Download options prepared", "dir", options["dir"], "out", options["out"])

	return options
}

// resolveDirectory 解析目录路径
func (s *AppDownloadService) resolveDirectory(directory string) string {
	if directory != "" {
		return directory
	}
	return s.config.Aria2.DownloadDir
}

// extractFilename 提取文件名
func (s *AppDownloadService) extractFilename(filename, url string) string {
	if filename != "" {
		return filename
	}

	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		if name := parts[len(parts)-1]; name != "" {
			return name
		}
	}

	return "unknown_file"
}

// isVideoFile 检查是否为视频文件
func (s *AppDownloadService) isVideoFile(filename string) bool {
	return fileutil.IsVideoFile(filename, s.config.Download.VideoExts)
}

// isMovieFile 检查是否为电影文件 - 使用智能路径分类
func (s *AppDownloadService) isMovieFile(filepath string) bool {
	if filepath == "" {
		return false
	}
	
	// 使用文件服务的智能媒体类型判断
	mediaType := s.fileService.GetMediaType(filepath)
	return mediaType == "movie"
}

// isTVFile 检查是否为电视剧文件 - 使用智能路径分类
func (s *AppDownloadService) isTVFile(filepath string) bool {
	if filepath == "" {
		return false
	}
	
	// 使用文件服务的智能媒体类型判断
	mediaType := s.fileService.GetMediaType(filepath)
	return mediaType == "tv"
}

// convertToDownloadResponse 转换Aria2状态到下载响应
func (s *AppDownloadService) convertToDownloadResponse(status *aria2.StatusResult) *contracts.DownloadResponse {
	// 这里需要根据实际的aria2.StatusResult结构进行转换
	response := &contracts.DownloadResponse{
		ID:           status.GID,
		Status:       s.convertAriaStatus(status.Status),
		ErrorMessage: status.ErrorMessage,
		UpdatedAt:    time.Now(),
	}

	// 解析数值字段
	if totalLength, err := strutil.ParseInt64(status.TotalLength); err == nil {
		response.TotalSize = totalLength
	}
	if completedLength, err := strutil.ParseInt64(status.CompletedLength); err == nil {
		response.CompletedSize = completedLength
	}
	if downloadSpeed, err := strutil.ParseInt64(status.DownloadSpeed); err == nil {
		response.Speed = downloadSpeed
	}

	// 计算进度
	if response.TotalSize > 0 {
		response.Progress = float64(response.CompletedSize) / float64(response.TotalSize) * 100
	}

	// 提取文件信息
	if len(status.Files) > 0 {
		response.Filename = status.Files[0].Path
		if idx := strings.LastIndex(response.Filename, "/"); idx != -1 {
			response.Filename = response.Filename[idx+1:]
		}
	}

	return response
}

// convertAriaDownloadToResponse 转换Aria2下载对象到响应格式
func (s *AppDownloadService) convertAriaDownloadToResponse(download interface{}) contracts.DownloadResponse {
	// 这里需要根据实际的aria2下载对象结构进行转换
	// 临时实现，需要根据实际结构调整
	return contracts.DownloadResponse{}
}

// convertAriaStatus 转换Aria2状态
func (s *AppDownloadService) convertAriaStatus(status string) valueobjects.DownloadStatus {
	switch status {
	case "active":
		return valueobjects.DownloadStatusActive
	case "complete":
		return valueobjects.DownloadStatusComplete
	case "error":
		return valueobjects.DownloadStatusError
	case "paused":
		return valueobjects.DownloadStatusPaused
	case "removed":
		return valueobjects.DownloadStatusRemoved
	default:
		return valueobjects.DownloadStatusPending
	}
}

// filterDownloads 过滤下载列表
func (s *AppDownloadService) filterDownloads(downloads []contracts.DownloadResponse, req contracts.DownloadListRequest) []contracts.DownloadResponse {
	if req.Status == "" {
		return downloads
	}

	var filtered []contracts.DownloadResponse
	for _, download := range downloads {
		if download.Status == req.Status {
			filtered = append(filtered, download)
		}
	}
	return filtered
}

// sortDownloads 排序下载列表
func (s *AppDownloadService) sortDownloads(downloads []contracts.DownloadResponse, sortBy, sortOrder string) []contracts.DownloadResponse {
	// 简单实现，实际可以使用更复杂的排序逻辑
	return downloads
}

