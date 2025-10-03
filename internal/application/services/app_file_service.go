package services

import (
	"context"
	"fmt"
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
	config          *config.Config
	alistClient     *alist.Client
	downloadService contracts.DownloadService
	pathStrategy    *PathStrategyService // 路径策略服务
}

// NewAppFileService 创建应用文件服务
func NewAppFileService(cfg *config.Config, downloadService contracts.DownloadService) contracts.FileService {
	service := &AppFileService{
		config:          cfg,
		alistClient:     alist.NewClient(cfg.Alist.BaseURL, cfg.Alist.Username, cfg.Alist.Password),
		downloadService: downloadService,
	}

	// 延迟初始化 pathStrategy（避免循环依赖）
	// 将在 SetDownloadService 中初始化

	return service
}

// SetDownloadService 设置下载服务（用于解决循环依赖）
func (s *AppFileService) SetDownloadService(downloadService contracts.DownloadService) {
	s.downloadService = downloadService

	// 初始化路径策略服务（现在可以安全使用 self 引用）
	if s.pathStrategy == nil {
		s.pathStrategy = NewPathStrategyService(s.config, s)
		logger.Debug("PathStrategyService initialized")
	}
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
				logger.Debug("Getting real download URL", "file", fileName, "path", path)
				internalURL, externalURL := s.getRealDownloadURLs(path)
				fileResp.InternalURL = internalURL
				fileResp.ExternalURL = externalURL
				logger.Debug("File response URLs updated", "internal", internalURL, "external", externalURL)
			}
			
			return &fileResp, nil
		}
	}

	return nil, fmt.Errorf("file not found: %s", path)
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

// convertToFileResponse 转换AList文件对象到响应格式
func (s *AppFileService) convertToFileResponse(item alist.FileItem, basePath string) contracts.FileResponse {
	fullPath := utils.JoinPath(basePath, item.Name)
	
	// 解析修改时间
	logger.Debug("Parsing time", "file", item.Name, "modifiedString", item.Modified)

	modifiedTime, err := utils.ParseTime(item.Modified)
	if err != nil {
		logger.Warn("Failed to parse time, using zero time", "file", item.Name, "modifiedString", item.Modified, "error", err)
		modifiedTime = time.Time{} // 零值时间
	} else {
		logger.Debug("Time parsed successfully", "file", item.Name, "parsedTime", modifiedTime.Format("2006-01-02 15:04:05 -07:00"), "unix", modifiedTime.Unix(), "location", modifiedTime.Location().String())
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
			logger.Debug("Using path-based category", "file", item.Name, "path", fullPath, "category", pathCategory)
		} else {
			// 回退到文件名分类（如果路径分类失败）
			fileCategory := s.GetFileCategory(item.Name)
			resp.MediaType = fileCategory
			resp.Category = fileCategory
			logger.Debug("Using filename-based category", "file", item.Name, "category", fileCategory)
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
	logger.Debug("Getting raw URL", "path", filePath)

	// 确保AList客户端token有效（将自动处理登录和刷新）
	hasToken, isValid, _ := s.alistClient.GetTokenStatus()
	if !hasToken || !isValid {
		logger.Debug("Token invalid, will refresh on request", "hasToken", hasToken, "isValid", isValid)
	}
	
	// 获取文件详细信息（包含raw_url）
	fileInfo, err := s.alistClient.GetFileInfo(filePath)
	if err != nil {
		logger.Warn("Failed to get file info, using fallback URL", "path", filePath, "error", err)
		fallbackInternal := s.generateInternalURL(filePath)
		fallbackExternal := s.generateExternalURL(filePath)
		logger.Debug("Using fallback URL", "internal", fallbackInternal, "external", fallbackExternal)
		return fallbackInternal, fallbackExternal
	}

	// 使用旧实现的简单逻辑：直接获取raw_url并做域名替换
	originalURL := fileInfo.Data.RawURL
	logger.Debug("Got original raw URL", "raw_url", originalURL)
	
	// 如果raw_url为空，使用回退逻辑
	if originalURL == "" {
		logger.Error("Raw URL is empty, this should not happen", "path", filePath, "fileInfo", fileInfo.Data)
		fallbackInternal := s.generateInternalURL(filePath)
		fallbackExternal := s.generateExternalURL(filePath)
		logger.Debug("Using fallback URL", "internal", fallbackInternal, "external", fallbackExternal)
		return fallbackInternal, fallbackExternal
	}
	
	// 采用旧实现的简单替换逻辑：只在包含fcalist-public时替换
	internalURL = originalURL
	externalURL = originalURL

	if strings.Contains(originalURL, "fcalist-public") {
		internalURL = strings.ReplaceAll(originalURL, "fcalist-public", "fcalist-internal")
		logger.Debug("URL replacement completed",
			"original", externalURL,
			"internal", internalURL,
			"replacement", "fcalist-public -> fcalist-internal")
	} else {
		logger.Debug("No URL replacement needed", "internal", internalURL, "external", externalURL)
	}

	logger.Debug("Download URLs obtained",
		"path", filePath,
		"internal_url", internalURL,
		"external_url", externalURL,
		"url_replaced", strings.Contains(originalURL, "fcalist-public"))
	
	return internalURL, externalURL
}

// generateInternalURL 生成内部下载URL（回退方法）
func (s *AppFileService) generateInternalURL(path string) string {
	url := fmt.Sprintf("%s/d%s", s.config.Alist.BaseURL, path)
	logger.Debug("Generated fallback download URL", "url", url, "path", path)
	return url
}

// generateExternalURL 生成外部访问URL（回退方法）
func (s *AppFileService) generateExternalURL(path string) string {
	url := fmt.Sprintf("%s/p%s", s.config.Alist.BaseURL, path)
	logger.Debug("Generated fallback external URL", "url", url, "path", path)
	return url
}

// getParentPath 获取父路径
func (s *AppFileService) getParentPath(path string) string {
	if path == "/" || path == "" {
		return ""
	}
	return utils.GetParentPath(path)
}