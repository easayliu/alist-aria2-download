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
	
	// 确保AList客户端token有效（将自动处理登录和刷新）
	hasToken, isValid, _ := s.alistClient.GetTokenStatus()
	if !hasToken || !isValid {
		logger.Info("🔑 检测到token无效，将在请求时自动刷新", "hasToken", hasToken, "isValid", isValid)
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