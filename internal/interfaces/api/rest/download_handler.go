package rest

import (
	"net/http"
	"strconv"

	"github.com/easayliu/alist-aria2-download/internal/api/utils"
	"github.com/easayliu/alist-aria2-download/internal/application/contracts"
	"github.com/easayliu/alist-aria2-download/internal/domain/entities"
	"github.com/easayliu/alist-aria2-download/pkg/logger"
	"github.com/gin-gonic/gin"
)

// DownloadHandler REST API下载处理器 - 专注于协议转换
type DownloadHandler struct {
	downloadService contracts.DownloadService
}

// NewDownloadHandler 创建下载处理器
func NewDownloadHandler(downloadService contracts.DownloadService) *DownloadHandler {
	return &DownloadHandler{
		downloadService: downloadService,
	}
}

// CreateDownload 创建下载任务
// @Summary 创建下载任务
// @Description 创建新的下载任务，支持自动分类和视频过滤
// @Tags 下载管理
// @Accept json
// @Produce json
// @Param request body contracts.DownloadRequest true "下载请求参数"
// @Success 200 {object} contracts.DownloadResponse "下载任务创建成功"
// @Failure 400 {object} map[string]interface{} "请求参数错误"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /api/v1/downloads [post]
func (h *DownloadHandler) CreateDownload(c *gin.Context) {
	var req contracts.DownloadRequest

	// 1. 绑定和验证请求参数
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Warn("Invalid download request", "error", err)
		utils.ErrorWithStatus(c, http.StatusBadRequest, 400, "Invalid request: "+err.Error())
		return
	}

	// 2. 调用业务服务
	response, err := h.downloadService.CreateDownload(c.Request.Context(), req)
	if err != nil {
		logger.Error("Failed to create download", "error", err, "url", req.URL)
		utils.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to create download: "+err.Error())
		return
	}

	// 3. 返回成功响应
	logger.Info("Download created via REST API", "id", response.ID, "url", req.URL)
	utils.Success(c, response)
}

// GetDownload 获取下载详情
// @Summary 获取下载详情
// @Description 根据下载ID获取下载任务的详细信息
// @Tags 下载管理
// @Produce json
// @Param id path string true "下载任务ID"
// @Success 200 {object} contracts.DownloadResponse "下载任务详情"
// @Failure 404 {object} map[string]interface{} "下载任务不存在"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /api/v1/downloads/{id} [get]
func (h *DownloadHandler) GetDownload(c *gin.Context) {
	id := c.Param("id")

	// 调用业务服务
	response, err := h.downloadService.GetDownload(c.Request.Context(), id)
	if err != nil {
		logger.Warn("Download not found", "id", id, "error", err)
		utils.ErrorWithStatus(c, http.StatusNotFound, 404, "Download not found: "+err.Error())
		return
	}

	utils.Success(c, response)
}

// ListDownloads 获取下载列表
// @Summary 获取下载列表
// @Description 获取下载任务列表，支持分页和过滤
// @Tags 下载管理
// @Produce json
// @Param status query string false "下载状态过滤"
// @Param limit query int false "每页数量" default(50)
// @Param offset query int false "偏移量" default(0)
// @Param sort_by query string false "排序字段" default(created_at)
// @Param sort_order query string false "排序方向" default(desc)
// @Success 200 {object} contracts.DownloadListResponse "下载列表"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /api/v1/downloads [get]
func (h *DownloadHandler) ListDownloads(c *gin.Context) {
	// 1. 解析查询参数
	req := contracts.DownloadListRequest{
		SortBy:    c.DefaultQuery("sort_by", "created_at"),
		SortOrder: c.DefaultQuery("sort_order", "desc"),
	}

	// 处理状态参数转换
	if statusStr := c.Query("status"); statusStr != "" {
		req.Status = entities.DownloadStatus(statusStr)
	}

	// 解析数值参数
	if limit := c.Query("limit"); limit != "" {
		if val, err := strconv.Atoi(limit); err == nil && val > 0 {
			req.Limit = val
		} else {
			req.Limit = 50
		}
	} else {
		req.Limit = 50
	}

	if offset := c.Query("offset"); offset != "" {
		if val, err := strconv.Atoi(offset); err == nil && val >= 0 {
			req.Offset = val
		}
	}

	// 2. 调用业务服务
	response, err := h.downloadService.ListDownloads(c.Request.Context(), req)
	if err != nil {
		logger.Error("Failed to list downloads", "error", err)
		utils.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to list downloads: "+err.Error())
		return
	}

	utils.Success(c, response)
}

// PauseDownload 暂停下载
// @Summary 暂停下载
// @Description 暂停指定的下载任务
// @Tags 下载管理
// @Produce json
// @Param id path string true "下载任务ID"
// @Success 200 {object} map[string]string "操作成功"
// @Failure 404 {object} map[string]interface{} "下载任务不存在"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /api/v1/downloads/{id}/pause [post]
func (h *DownloadHandler) PauseDownload(c *gin.Context) {
	id := c.Param("id")

	err := h.downloadService.PauseDownload(c.Request.Context(), id)
	if err != nil {
		logger.Error("Failed to pause download", "id", id, "error", err)
		utils.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to pause download: "+err.Error())
		return
	}

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
// @Param id path string true "下载任务ID"
// @Success 200 {object} map[string]string "操作成功"
// @Failure 404 {object} map[string]interface{} "下载任务不存在"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /api/v1/downloads/{id}/resume [post]
func (h *DownloadHandler) ResumeDownload(c *gin.Context) {
	id := c.Param("id")

	err := h.downloadService.ResumeDownload(c.Request.Context(), id)
	if err != nil {
		logger.Error("Failed to resume download", "id", id, "error", err)
		utils.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to resume download: "+err.Error())
		return
	}

	utils.Success(c, gin.H{
		"message": "Download resumed successfully",
		"id":      id,
	})
}

// CancelDownload 取消下载
// @Summary 取消下载
// @Description 取消指定的下载任务
// @Tags 下载管理
// @Produce json
// @Param id path string true "下载任务ID"
// @Success 200 {object} map[string]string "操作成功"
// @Failure 404 {object} map[string]interface{} "下载任务不存在"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /api/v1/downloads/{id}/cancel [delete]
func (h *DownloadHandler) CancelDownload(c *gin.Context) {
	id := c.Param("id")

	err := h.downloadService.CancelDownload(c.Request.Context(), id)
	if err != nil {
		logger.Error("Failed to cancel download", "id", id, "error", err)
		utils.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to cancel download: "+err.Error())
		return
	}

	utils.Success(c, gin.H{
		"message": "Download cancelled successfully",
		"id":      id,
	})
}

// RetryDownload 重试下载
// @Summary 重试下载
// @Description 重试失败的下载任务
// @Tags 下载管理
// @Produce json
// @Param id path string true "下载任务ID"
// @Success 200 {object} contracts.DownloadResponse "重试后的下载任务"
// @Failure 404 {object} map[string]interface{} "下载任务不存在"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /api/v1/downloads/{id}/retry [post]
func (h *DownloadHandler) RetryDownload(c *gin.Context) {
	id := c.Param("id")

	response, err := h.downloadService.RetryDownload(c.Request.Context(), id)
	if err != nil {
		logger.Error("Failed to retry download", "id", id, "error", err)
		utils.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to retry download: "+err.Error())
		return
	}

	utils.Success(c, response)
}

// CreateBatchDownload 批量创建下载
// @Summary 批量创建下载
// @Description 批量创建多个下载任务
// @Tags 下载管理
// @Accept json
// @Produce json
// @Param request body contracts.BatchDownloadRequest true "批量下载请求"
// @Success 200 {object} contracts.BatchDownloadResponse "批量下载结果"
// @Failure 400 {object} map[string]interface{} "请求参数错误"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /api/v1/downloads/batch [post]
func (h *DownloadHandler) CreateBatchDownload(c *gin.Context) {
	var req contracts.BatchDownloadRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Warn("Invalid batch download request", "error", err)
		utils.ErrorWithStatus(c, http.StatusBadRequest, 400, "Invalid request: "+err.Error())
		return
	}

	response, err := h.downloadService.CreateBatchDownload(c.Request.Context(), req)
	if err != nil {
		logger.Error("Failed to create batch download", "error", err, "items_count", len(req.Items))
		utils.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to create batch download: "+err.Error())
		return
	}

	logger.Info("Batch download created via REST API", 
		"total_items", len(req.Items),
		"success_count", response.SuccessCount,
		"failure_count", response.FailureCount)
	
	utils.Success(c, response)
}

// PauseAllDownloads 暂停所有下载
// @Summary 暂停所有下载
// @Description 暂停所有正在进行的下载任务
// @Tags 下载管理
// @Produce json
// @Success 200 {object} map[string]string "操作成功"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /api/v1/downloads/pause-all [post]
func (h *DownloadHandler) PauseAllDownloads(c *gin.Context) {
	err := h.downloadService.PauseAllDownloads(c.Request.Context())
	if err != nil {
		logger.Error("Failed to pause all downloads", "error", err)
		utils.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to pause all downloads: "+err.Error())
		return
	}

	utils.Success(c, gin.H{
		"message": "All downloads paused successfully",
	})
}

// ResumeAllDownloads 恢复所有下载
// @Summary 恢复所有下载
// @Description 恢复所有已暂停的下载任务
// @Tags 下载管理
// @Produce json
// @Success 200 {object} map[string]string "操作成功"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /api/v1/downloads/resume-all [post]
func (h *DownloadHandler) ResumeAllDownloads(c *gin.Context) {
	err := h.downloadService.ResumeAllDownloads(c.Request.Context())
	if err != nil {
		logger.Error("Failed to resume all downloads", "error", err)
		utils.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to resume all downloads: "+err.Error())
		return
	}

	utils.Success(c, gin.H{
		"message": "All downloads resumed successfully",
	})
}

// GetSystemStatus 获取系统状态
// @Summary 获取系统状态
// @Description 获取下载系统的整体状态信息
// @Tags 下载管理
// @Produce json
// @Success 200 {object} map[string]interface{} "系统状态"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /api/v1/downloads/system/status [get]
func (h *DownloadHandler) GetSystemStatus(c *gin.Context) {
	status, err := h.downloadService.GetSystemStatus(c.Request.Context())
	if err != nil {
		logger.Error("Failed to get system status", "error", err)
		utils.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to get system status: "+err.Error())
		return
	}

	utils.Success(c, status)
}

// GetDownloadStatistics 获取下载统计
// @Summary 获取下载统计
// @Description 获取下载系统的统计信息
// @Tags 下载管理
// @Produce json
// @Success 200 {object} map[string]interface{} "下载统计"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /api/v1/downloads/statistics [get]
func (h *DownloadHandler) GetDownloadStatistics(c *gin.Context) {
	stats, err := h.downloadService.GetDownloadStatistics(c.Request.Context())
	if err != nil {
		logger.Error("Failed to get download statistics", "error", err)
		utils.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to get download statistics: "+err.Error())
		return
	}

	utils.Success(c, stats)
}