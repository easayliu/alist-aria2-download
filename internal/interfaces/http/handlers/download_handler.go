package handlers

import (
	"net/http"

	"github.com/easayliu/alist-aria2-download/internal/application/contracts"
	"github.com/easayliu/alist-aria2-download/internal/application/services"
	httputil "github.com/easayliu/alist-aria2-download/pkg/utils/http"
	"github.com/gin-gonic/gin"
)

type DownloadHandler struct {
	container *services.ServiceContainer
}

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
// @Success 200 {object} map[string]interface{} "下载任务创建成功"
// @Failure 400 {object} map[string]interface{} "请求参数错误"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /downloads [post]
func (h *DownloadHandler) CreateDownload(c *gin.Context) {
	var req contracts.DownloadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.ErrorWithStatus(c, http.StatusBadRequest, 400, "Invalid request: "+err.Error())
		return
	}

	downloadService := h.container.GetDownloadService()
	response, err := downloadService.CreateDownload(c.Request.Context(), req)
	if err != nil {
		httputil.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to create download: "+err.Error())
		return
	}

	httputil.Success(c, gin.H{
		"message":  "Download created successfully",
		"download": response,
	})
}

// ListDownloads 获取下载列表
// @Summary 获取下载列表
// @Description 获取所有Aria2下载任务列表
// @Tags 下载管理
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /downloads [get]
func (h *DownloadHandler) ListDownloads(c *gin.Context) {
	var req contracts.DownloadListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		httputil.ErrorWithStatus(c, http.StatusBadRequest, 400, "Invalid request: "+err.Error())
		return
	}

	downloadService := h.container.GetDownloadService()
	response, err := downloadService.ListDownloads(c.Request.Context(), req)
	if err != nil {
		httputil.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to list downloads: "+err.Error())
		return
	}

	httputil.Success(c, response)
}

// GetDownload 获取单个下载详情
// @Summary 获取下载详情
// @Description 根据GID获取单个下载任务详情
// @Tags 下载管理
// @Produce json
// @Param id path string true "下载任务GID"
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /downloads/{id} [get]
func (h *DownloadHandler) GetDownload(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		httputil.ErrorWithStatus(c, http.StatusBadRequest, 400, "Download ID is required")
		return
	}

	downloadService := h.container.GetDownloadService()
	response, err := downloadService.GetDownload(c.Request.Context(), id)
	if err != nil {
		httputil.ErrorWithStatus(c, http.StatusNotFound, 404, "Download not found: "+err.Error())
		return
	}

	httputil.Success(c, response)
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
	id := c.Param("id")
	if id == "" {
		httputil.ErrorWithStatus(c, http.StatusBadRequest, 400, "Download ID is required")
		return
	}

	downloadService := h.container.GetDownloadService()
	if err := downloadService.CancelDownload(c.Request.Context(), id); err != nil {
		httputil.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to delete download: "+err.Error())
		return
	}

	httputil.Success(c, gin.H{
		"message": "Download deleted successfully",
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
	id := c.Param("id")
	if id == "" {
		httputil.ErrorWithStatus(c, http.StatusBadRequest, 400, "Download ID is required")
		return
	}

	downloadService := h.container.GetDownloadService()
	if err := downloadService.PauseDownload(c.Request.Context(), id); err != nil {
		httputil.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to pause download: "+err.Error())
		return
	}

	httputil.Success(c, gin.H{
		"message": "Download paused successfully",
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
	id := c.Param("id")
	if id == "" {
		httputil.ErrorWithStatus(c, http.StatusBadRequest, 400, "Download ID is required")
		return
	}

	downloadService := h.container.GetDownloadService()
	if err := downloadService.ResumeDownload(c.Request.Context(), id); err != nil {
		httputil.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to resume download: "+err.Error())
		return
	}

	httputil.Success(c, gin.H{
		"message": "Download resumed successfully",
	})
}

// CreateBatchDownload 批量创建下载任务
// @Summary 批量创建下载任务
// @Description 批量创建多个Aria2下载任务
// @Tags 下载管理
// @Accept json
// @Produce json
// @Param request body contracts.BatchDownloadRequest true "批量下载请求参数"
// @Success 200 {object} map[string]interface{} "批量下载任务创建成功"
// @Failure 400 {object} map[string]interface{} "请求参数错误"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /downloads/batch [post]
func (h *DownloadHandler) CreateBatchDownload(c *gin.Context) {
	var req contracts.BatchDownloadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.ErrorWithStatus(c, http.StatusBadRequest, 400, "Invalid request: "+err.Error())
		return
	}

	downloadService := h.container.GetDownloadService()
	response, err := downloadService.CreateBatchDownload(c.Request.Context(), req)
	if err != nil {
		httputil.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to create batch download: "+err.Error())
		return
	}

	httputil.Success(c, gin.H{
		"message": "Batch download created successfully",
		"result":  response,
	})
}

// PauseAllDownloads 暂停所有下载
// @Summary 暂停所有下载
// @Description 暂停所有正在进行的下载任务
// @Tags 下载管理
// @Produce json
// @Success 200 {object} map[string]string
// @Failure 500 {object} map[string]interface{}
// @Router /downloads/pause-all [post]
func (h *DownloadHandler) PauseAllDownloads(c *gin.Context) {
	downloadService := h.container.GetDownloadService()
	if err := downloadService.PauseAllDownloads(c.Request.Context()); err != nil {
		httputil.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to pause all downloads: "+err.Error())
		return
	}

	httputil.Success(c, gin.H{
		"message": "All downloads paused successfully",
	})
}

// ResumeAllDownloads 恢复所有下载
// @Summary 恢复所有下载
// @Description 恢复所有已暂停的下载任务
// @Tags 下载管理
// @Produce json
// @Success 200 {object} map[string]string
// @Failure 500 {object} map[string]interface{}
// @Router /downloads/resume-all [post]
func (h *DownloadHandler) ResumeAllDownloads(c *gin.Context) {
	downloadService := h.container.GetDownloadService()
	if err := downloadService.ResumeAllDownloads(c.Request.Context()); err != nil {
		httputil.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to resume all downloads: "+err.Error())
		return
	}

	httputil.Success(c, gin.H{
		"message": "All downloads resumed successfully",
	})
}

// GetDownloadStatistics 获取下载统计
// @Summary 获取下载统计
// @Description 获取下载任务的统计信息
// @Tags 下载管理
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /downloads/statistics [get]
func (h *DownloadHandler) GetDownloadStatistics(c *gin.Context) {
	downloadService := h.container.GetDownloadService()
	stats, err := downloadService.GetDownloadStatistics(c.Request.Context())
	if err != nil {
		httputil.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to get statistics: "+err.Error())
		return
	}

	httputil.Success(c, stats)
}

// GetSystemStatus 获取系统状态
// @Summary 获取系统状态
// @Description 获取Aria2系统的状态信息
// @Tags 下载管理
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /downloads/system-status [get]
func (h *DownloadHandler) GetSystemStatus(c *gin.Context) {
	downloadService := h.container.GetDownloadService()
	status, err := downloadService.GetSystemStatus(c.Request.Context())
	if err != nil {
		httputil.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to get system status: "+err.Error())
		return
	}

	httputil.Success(c, status)
}
