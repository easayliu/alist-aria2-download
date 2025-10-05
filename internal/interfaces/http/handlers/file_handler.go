package handlers

import (
	"context"
	"net/http"

	"github.com/easayliu/alist-aria2-download/internal/application/contracts"
	"github.com/easayliu/alist-aria2-download/internal/application/services"
	httputil "github.com/easayliu/alist-aria2-download/pkg/utils/http"
	"github.com/gin-gonic/gin"
)

// FileHandler 文件管理处理器 - 使用ServiceContainer和contracts接口
type FileHandler struct {
	container *services.ServiceContainer
}

// NewFileHandler 创建文件处理器
func NewFileHandler(container *services.ServiceContainer) *FileHandler {
	return &FileHandler{
		container: container,
	}
}

// GetYesterdayFiles 获取昨天的文件
// @Summary 获取昨天的文件
// @Description 获取昨天修改的文件列表
// @Tags 文件管理
// @Accept json
// @Produce json
// @Param path query string false "搜索路径（留空使用配置的默认路径）"
// @Success 200 {object} map[string]interface{} "昨天的文件列表"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /files/yesterday [get]
func (h *FileHandler) GetYesterdayFiles(c *gin.Context) {
	ctx := context.Background()
	path := c.Query("path")

	// 如果path为空,使用默认路径
	if path == "" {
		path = h.container.GetConfig().Alist.DefaultPath
	}

	// 从容器获取文件服务
	fileService := h.container.GetFileService()

	// 调用服务获取昨天的文件
	response, err := fileService.GetYesterdayFiles(ctx, path)
	if err != nil {
		httputil.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to get yesterday files: "+err.Error())
		return
	}

	// 返回成功响应
	httputil.Success(c, gin.H{
		"files":       response.Files,
		"count":       response.TotalCount,
		"total_size":  response.Summary.TotalSizeFormatted,
		"search_path": path,
		"date":        "yesterday",
		"summary":     response.Summary,
	})
}

// DownloadYesterdayFiles 批量下载昨天的文件
// @Summary 批量下载昨天文件
// @Description 将昨天修改的文件批量添加到Aria2下载队列
// @Tags 文件管理
// @Accept json
// @Produce json
// @Param path query string false "搜索路径（留空使用配置的默认路径）"
// @Param preview query bool false "预览模式，只生成路径不下载"
// @Success 200 {object} map[string]interface{} "下载任务创建结果或预览信息"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /files/yesterday/download [post]
func (h *FileHandler) DownloadYesterdayFiles(c *gin.Context) {
	ctx := context.Background()
	path := c.Query("path")
	preview := c.Query("preview") == "true"

	if path == "" {
		path = h.container.GetConfig().Alist.DefaultPath
	}

	fileService := h.container.GetFileService()

	// 先获取昨天的文件列表
	filesResp, err := fileService.GetYesterdayFiles(ctx, path)
	if err != nil {
		httputil.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to get yesterday files: "+err.Error())
		return
	}

	if len(filesResp.Files) == 0 {
		httputil.Success(c, gin.H{
			"message":       "No files found from yesterday",
			"total":         0,
			"success_count": 0,
			"fail_count":    0,
		})
		return
	}

	// 如果是预览模式，只返回文件列表
	if preview {
		httputil.Success(c, gin.H{
			"message": "Preview mode - files that would be downloaded",
			"mode":    "preview",
			"total":   len(filesResp.Files),
			"files":   filesResp.Files,
			"summary": filesResp.Summary,
		})
		return
	}

	// 构建批量下载请求
	var downloadItems []contracts.DownloadRequest
	for _, file := range filesResp.Files {
		downloadItems = append(downloadItems, contracts.DownloadRequest{
			URL:          file.InternalURL,
			Filename:     file.Name,
			Directory:    file.DownloadPath,
			AutoClassify: true,
		})
	}

	batchRequest := contracts.BatchDownloadRequest{
		Items:        downloadItems,
		VideoOnly:    true,
		AutoClassify: true,
	}

	// 调用下载服务批量创建下载
	downloadService := h.container.GetDownloadService()
	batchResponse, err := downloadService.CreateBatchDownload(ctx, batchRequest)
	if err != nil {
		httputil.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to create batch download: "+err.Error())
		return
	}

	httputil.Success(c, gin.H{
		"message":       "Batch download created successfully",
		"mode":          "download",
		"total":         len(filesResp.Files),
		"success_count": batchResponse.SuccessCount,
		"fail_count":    batchResponse.FailureCount,
		"summary":       batchResponse.Summary,
		"results":       batchResponse.Results,
	})
}

// DownloadFilesFromPath 从指定路径下载文件
// @Summary 从指定路径下载文件
// @Description 获取指定路径下的所有文件并添加到Aria2下载队列，支持递归下载子目录
// @Tags 文件管理
// @Accept json
// @Produce json
// @Param request body contracts.DirectoryDownloadRequest true "下载路径请求"
// @Success 200 {object} map[string]interface{} "下载任务创建结果或预览信息"
// @Failure 400 {object} map[string]interface{} "请求参数错误"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /files/download [post]
func (h *FileHandler) DownloadFilesFromPath(c *gin.Context) {
	ctx := context.Background()
	var req contracts.DirectoryDownloadRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.ErrorWithStatus(c, http.StatusBadRequest, 400, "Invalid request parameters: "+err.Error())
		return
	}

	fileService := h.container.GetFileService()

	// 调用目录下载服务
	batchResponse, err := fileService.DownloadDirectory(ctx, req)
	if err != nil {
		httputil.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to download files: "+err.Error())
		return
	}

	httputil.Success(c, gin.H{
		"message":       "Directory download created successfully",
		"source_path":   req.DirectoryPath,
		"recursive":     req.Recursive,
		"total":         len(batchResponse.Results),
		"success_count": batchResponse.SuccessCount,
		"fail_count":    batchResponse.FailureCount,
		"summary":       batchResponse.Summary,
		"results":       batchResponse.Results,
	})
}

// ListFilesHandler 列出指定路径的文件
// @Summary 列出指定路径的文件
// @Description 获取指定路径下的文件列表，支持分页和视频文件过滤
// @Tags 文件管理
// @Accept json
// @Produce json
// @Param request body contracts.FileListRequest true "列出文件请求"
// @Success 200 {object} map[string]interface{} "文件列表"
// @Failure 400 {object} map[string]interface{} "请求参数错误"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /files/list [post]
func (h *FileHandler) ListFilesHandler(c *gin.Context) {
	ctx := context.Background()
	var req contracts.FileListRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.ErrorWithStatus(c, http.StatusBadRequest, 400, "Invalid request parameters: "+err.Error())
		return
	}

	// 设置默认值
	if req.Path == "" {
		req.Path = h.container.GetConfig().Alist.DefaultPath
	}
	if req.Page == 0 {
		req.Page = 1
	}
	if req.PageSize == 0 {
		req.PageSize = 100
	}

	fileService := h.container.GetFileService()

	// 调用文件列表服务
	response, err := fileService.ListFiles(ctx, req)
	if err != nil {
		httputil.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to list files: "+err.Error())
		return
	}

	httputil.Success(c, gin.H{
		"path":        req.Path,
		"page":        req.Page,
		"page_size":   req.PageSize,
		"total":       response.TotalCount,
		"video_only":  req.VideoOnly,
		"files":       response.Files,
		"directories": response.Directories,
		"summary":     response.Summary,
		"pagination":  response.Pagination,
	})
}

// ManualDownloadFiles 手动执行文件下载
// @Summary 手动执行文件下载
// @Description 手动执行指定时间范围内的文件下载，支持预览模式
// @Tags 文件管理
// @Accept json
// @Produce json
// @Param request body contracts.TimeRangeFileRequest true "下载参数"
// @Success 200 {object} map[string]interface{} "下载结果"
// @Failure 400 {object} map[string]interface{} "请求参数错误"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /files/manual-download [post]
func (h *FileHandler) ManualDownloadFiles(c *gin.Context) {
	ctx := context.Background()

	// 定义请求结构体，包含时间范围和预览标志
	var req struct {
		contracts.TimeRangeFileRequest
		Preview bool `json:"preview,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.ErrorWithStatus(c, http.StatusBadRequest, 400, "Invalid request parameters: "+err.Error())
		return
	}

	// 设置默认路径
	if req.Path == "" {
		req.Path = h.container.GetConfig().Alist.DefaultPath
	}

	fileService := h.container.GetFileService()

	// 调用时间范围文件查询服务
	timeRangeResp, err := fileService.GetFilesByTimeRange(ctx, req.TimeRangeFileRequest)
	if err != nil {
		httputil.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to get files by time range: "+err.Error())
		return
	}

	if len(timeRangeResp.Files) == 0 {
		httputil.Success(c, gin.H{
			"message":    "No files found in the specified time range",
			"time_range": timeRangeResp.TimeRange,
			"total":      0,
		})
		return
	}

	// 如果是预览模式，只返回文件列表
	if req.Preview {
		httputil.Success(c, gin.H{
			"message":    "Preview mode - files that would be downloaded",
			"mode":       "preview",
			"path":       req.Path,
			"time_range": timeRangeResp.TimeRange,
			"total":      len(timeRangeResp.Files),
			"files":      timeRangeResp.Files,
			"summary":    timeRangeResp.Summary,
		})
		return
	}

	// 构建批量下载请求
	var downloadItems []contracts.DownloadRequest
	for _, file := range timeRangeResp.Files {
		downloadItems = append(downloadItems, contracts.DownloadRequest{
			URL:          file.InternalURL,
			Filename:     file.Name,
			Directory:    file.DownloadPath,
			AutoClassify: true,
		})
	}

	batchRequest := contracts.BatchDownloadRequest{
		Items:        downloadItems,
		VideoOnly:    req.VideoOnly,
		AutoClassify: true,
	}

	// 调用下载服务批量创建下载
	downloadService := h.container.GetDownloadService()
	batchResponse, err := downloadService.CreateBatchDownload(ctx, batchRequest)
	if err != nil {
		httputil.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to create batch download: "+err.Error())
		return
	}

	httputil.Success(c, gin.H{
		"message":       "Batch download created successfully",
		"mode":          "download",
		"path":          req.Path,
		"time_range":    timeRangeResp.TimeRange,
		"video_only":    req.VideoOnly,
		"total":         len(timeRangeResp.Files),
		"success_count": batchResponse.SuccessCount,
		"fail_count":    batchResponse.FailureCount,
		"summary":       batchResponse.Summary,
		"results":       batchResponse.Results,
	})
}
