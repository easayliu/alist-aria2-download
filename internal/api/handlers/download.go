package handlers

import (
	"net/http"
	"strconv"

	"github.com/easayliu/alist-aria2-download/internal/application/contracts"
	"github.com/easayliu/alist-aria2-download/internal/application/services"
	"github.com/easayliu/alist-aria2-download/internal/domain/entities"
	"github.com/easayliu/alist-aria2-download/pkg/utils"
	"github.com/gin-gonic/gin"
)

// DownloadHandler REST下载处理器 - 纯协议转换层
type DownloadHandler struct {
	container *services.ServiceContainer
}

// NewDownloadHandler 创建下载处理器
func NewDownloadHandler(container *services.ServiceContainer) *DownloadHandler {
	return &DownloadHandler{
		container: container,
	}
}

// CreateDownload 创建下载任务
// @Summary 创建下载任务
// @Description 创建新的Aria2下载任务
// @Tags 下载管理
// @Accept json
// @Produce json
// @Param request body contracts.DownloadRequest true "下载请求参数"
// @Success 200 {object} contracts.DownloadResponse "下载任务创建成功"
// @Failure 400 {object} map[string]interface{} "请求参数错误"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /downloads [post]
func (h *DownloadHandler) CreateDownload(c *gin.Context) {
	// 1. 解析HTTP请求 - 协议转换
	var req contracts.DownloadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorWithStatus(c, http.StatusBadRequest, 400, "Invalid request: "+err.Error())
		return
	}

	// 2. 调用应用服务 - 业务逻辑委托
	downloadService := h.container.GetDownloadService()
	response, err := downloadService.CreateDownload(c.Request.Context(), req)
	if err != nil {
		// 错误映射
		if serviceErr, ok := err.(*contracts.ServiceError); ok {
			statusCode := h.mapErrorCodeToHTTPStatus(serviceErr.Code)
			utils.ErrorWithStatus(c, statusCode, statusCode, serviceErr.Message)
		} else {
			utils.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to create download: "+err.Error())
		}
		return
	}

	// 3. 返回HTTP响应 - 协议转换
	utils.Success(c, gin.H{
		"message":  "Download created successfully",
		"download": response,
	})
}

// ListDownloads 获取下载列表
// @Summary 获取下载列表
// @Description 获取所有Aria2下载任务列表
// @Tags 下载管理
// @Produce json
// @Param status query string false "过滤状态"
// @Param limit query int false "限制数量" default(100)
// @Param offset query int false "偏移量" default(0)
// @Param sort_by query string false "排序字段" Enums(name,size,status,created_at)
// @Param sort_order query string false "排序方向" Enums(asc,desc)
// @Success 200 {object} contracts.DownloadListResponse
// @Failure 500 {object} map[string]interface{}
// @Router /downloads [get]
func (h *DownloadHandler) ListDownloads(c *gin.Context) {
	// 1. 解析查询参数 - 协议转换
	var status entities.DownloadStatus
	if statusStr := c.Query("status"); statusStr != "" {
		status = entities.DownloadStatus(statusStr)
	}
	
	req := contracts.DownloadListRequest{
		Status:    status,
		SortBy:    c.Query("sort_by"),
		SortOrder: c.Query("sort_order"),
	}

	// 解析数值参数
	if limitStr := c.Query("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil {
			req.Limit = limit
		} else {
			req.Limit = 100 // 默认值
		}
	} else {
		req.Limit = 100
	}

	if offsetStr := c.Query("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil {
			req.Offset = offset
		}
	}

	// 2. 调用应用服务
	downloadService := h.container.GetDownloadService()
	response, err := downloadService.ListDownloads(c.Request.Context(), req)
	if err != nil {
		if serviceErr, ok := err.(*contracts.ServiceError); ok {
			statusCode := h.mapErrorCodeToHTTPStatus(serviceErr.Code)
			utils.ErrorWithStatus(c, statusCode, statusCode, serviceErr.Message)
		} else {
			utils.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to list downloads: "+err.Error())
		}
		return
	}

	// 3. 返回响应
	utils.Success(c, response)
}

// GetDownload 获取单个下载详情
// @Summary 获取下载详情
// @Description 根据GID获取单个下载任务详情
// @Tags 下载管理
// @Produce json
// @Param id path string true "下载任务GID"
// @Success 200 {object} contracts.DownloadResponse
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /downloads/{id} [get]
func (h *DownloadHandler) GetDownload(c *gin.Context) {
	// 1. 提取路径参数
	id := c.Param("id")
	if id == "" {
		utils.ErrorWithStatus(c, http.StatusBadRequest, 400, "Download ID is required")
		return
	}

	// 2. 调用应用服务
	downloadService := h.container.GetDownloadService()
	response, err := downloadService.GetDownload(c.Request.Context(), id)
	if err != nil {
		if serviceErr, ok := err.(*contracts.ServiceError); ok {
			statusCode := h.mapErrorCodeToHTTPStatus(serviceErr.Code)
			utils.ErrorWithStatus(c, statusCode, statusCode, serviceErr.Message)
		} else {
			utils.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to get download: "+err.Error())
		}
		return
	}

	// 3. 返回响应
	utils.Success(c, response)
}

// DeleteDownload 删除下载任务
// @Summary 删除下载任务
// @Description 根据GID删除下载任务
// @Tags 下载管理
// @Produce json
// @Param id path string true "下载任务GID"
// @Success 200 {object} map[string]string
// @Failure 500 {object} map[string]interface{}
// @Router /downloads/{id} [delete]
func (h *DownloadHandler) DeleteDownload(c *gin.Context) {
	// 1. 提取路径参数
	id := c.Param("id")
	if id == "" {
		utils.ErrorWithStatus(c, http.StatusBadRequest, 400, "Download ID is required")
		return
	}

	// 2. 调用应用服务
	downloadService := h.container.GetDownloadService()
	err := downloadService.CancelDownload(c.Request.Context(), id)
	if err != nil {
		if serviceErr, ok := err.(*contracts.ServiceError); ok {
			statusCode := h.mapErrorCodeToHTTPStatus(serviceErr.Code)
			utils.ErrorWithStatus(c, statusCode, statusCode, serviceErr.Message)
		} else {
			utils.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to delete download: "+err.Error())
		}
		return
	}

	// 3. 返回成功响应
	utils.Success(c, gin.H{
		"message": "Download deleted successfully",
		"id":      id,
	})
}

// PauseDownload 暂停下载
// @Summary 暂停下载
// @Description 暂停指定的下载任务
// @Tags 下载管理
// @Produce json
// @Param id path string true "下载任务GID"
// @Success 200 {object} map[string]string
// @Failure 500 {object} map[string]interface{}
// @Router /downloads/{id}/pause [post]
func (h *DownloadHandler) PauseDownload(c *gin.Context) {
	// 1. 提取路径参数
	id := c.Param("id")
	if id == "" {
		utils.ErrorWithStatus(c, http.StatusBadRequest, 400, "Download ID is required")
		return
	}

	// 2. 调用应用服务
	downloadService := h.container.GetDownloadService()
	err := downloadService.PauseDownload(c.Request.Context(), id)
	if err != nil {
		if serviceErr, ok := err.(*contracts.ServiceError); ok {
			statusCode := h.mapErrorCodeToHTTPStatus(serviceErr.Code)
			utils.ErrorWithStatus(c, statusCode, statusCode, serviceErr.Message)
		} else {
			utils.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to pause download: "+err.Error())
		}
		return
	}

	// 3. 返回成功响应
	utils.Success(c, gin.H{
		"message": "Download paused successfully",
		"id":      id,
	})
}

// ResumeDownload 恢复下载
// @Summary 恢复下载
// @Description 恢复指定的下载任务
// @Tags 下载管理
// @Produce json
// @Param id path string true "下载任务GID"
// @Success 200 {object} map[string]string
// @Failure 500 {object} map[string]interface{}
// @Router /downloads/{id}/resume [post]
func (h *DownloadHandler) ResumeDownload(c *gin.Context) {
	// 1. 提取路径参数
	id := c.Param("id")
	if id == "" {
		utils.ErrorWithStatus(c, http.StatusBadRequest, 400, "Download ID is required")
		return
	}

	// 2. 调用应用服务
	downloadService := h.container.GetDownloadService()
	err := downloadService.ResumeDownload(c.Request.Context(), id)
	if err != nil {
		if serviceErr, ok := err.(*contracts.ServiceError); ok {
			statusCode := h.mapErrorCodeToHTTPStatus(serviceErr.Code)
			utils.ErrorWithStatus(c, statusCode, statusCode, serviceErr.Message)
		} else {
			utils.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to resume download: "+err.Error())
		}
		return
	}

	// 3. 返回成功响应
	utils.Success(c, gin.H{
		"message": "Download resumed successfully",
		"id":      id,
	})
}

// CreateBatchDownload 批量创建下载
// @Summary 批量创建下载
// @Description 批量创建多个下载任务
// @Tags 下载管理
// @Accept json
// @Produce json
// @Param request body contracts.BatchDownloadRequest true "批量下载请求"
// @Success 200 {object} contracts.BatchDownloadResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /downloads/batch [post]
func (h *DownloadHandler) CreateBatchDownload(c *gin.Context) {
	// 1. 解析请求
	var req contracts.BatchDownloadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorWithStatus(c, http.StatusBadRequest, 400, "Invalid request: "+err.Error())
		return
	}

	// 2. 调用应用服务
	downloadService := h.container.GetDownloadService()
	response, err := downloadService.CreateBatchDownload(c.Request.Context(), req)
	if err != nil {
		if serviceErr, ok := err.(*contracts.ServiceError); ok {
			statusCode := h.mapErrorCodeToHTTPStatus(serviceErr.Code)
			utils.ErrorWithStatus(c, statusCode, statusCode, serviceErr.Message)
		} else {
			utils.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to create batch downloads: "+err.Error())
		}
		return
	}

	// 3. 返回响应
	utils.Success(c, gin.H{
		"message": "Batch downloads created",
		"result":  response,
	})
}

// GetSystemStatus 获取系统状态
// @Summary 获取系统状态
// @Description 获取下载系统的状态信息
// @Tags 下载管理
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /downloads/status [get]
func (h *DownloadHandler) GetSystemStatus(c *gin.Context) {
	// 1. 调用应用服务
	downloadService := h.container.GetDownloadService()
	status, err := downloadService.GetSystemStatus(c.Request.Context())
	if err != nil {
		utils.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to get system status: "+err.Error())
		return
	}

	// 2. 返回响应
	utils.Success(c, status)
}

// GetStatistics 获取下载统计
// @Summary 获取下载统计
// @Description 获取下载系统的统计信息
// @Tags 下载管理
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /downloads/statistics [get]
func (h *DownloadHandler) GetStatistics(c *gin.Context) {
	// 1. 调用应用服务
	downloadService := h.container.GetDownloadService()
	stats, err := downloadService.GetDownloadStatistics(c.Request.Context())
	if err != nil {
		utils.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to get statistics: "+err.Error())
		return
	}

	// 2. 返回响应
	utils.Success(c, stats)
}

// ========== 私有方法 ==========

// mapErrorCodeToHTTPStatus 将业务错误码映射到HTTP状态码
func (h *DownloadHandler) mapErrorCodeToHTTPStatus(code contracts.ErrorCode) int {
	switch code {
	case contracts.ErrorCodeInvalidRequest:
		return http.StatusBadRequest
	case contracts.ErrorCodeNotFound:
		return http.StatusNotFound
	case contracts.ErrorCodeUnauthorized:
		return http.StatusUnauthorized
	case contracts.ErrorCodeForbidden:
		return http.StatusForbidden
	case contracts.ErrorCodeConflict:
		return http.StatusConflict
	case contracts.ErrorCodeServiceUnavailable:
		return http.StatusServiceUnavailable
	case contracts.ErrorCodeTimeout:
		return http.StatusRequestTimeout
	case contracts.ErrorCodeRateLimit:
		return http.StatusTooManyRequests
	case contracts.ErrorCodeQuotaExceeded:
		return http.StatusInsufficientStorage
	default:
		return http.StatusInternalServerError
	}
}