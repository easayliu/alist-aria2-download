package handlers

import (
	"net/http"
	"strconv"

	"github.com/easayliu/alist-aria2-download/internal/application/contracts"
	"github.com/easayliu/alist-aria2-download/internal/application/services"
	httputil "github.com/easayliu/alist-aria2-download/pkg/utils/http"
	"github.com/gin-gonic/gin"
)

type NotificationHandler struct {
	container *services.ServiceContainer
}

func NewNotificationHandler(container *services.ServiceContainer) *NotificationHandler {
	return &NotificationHandler{
		container: container,
	}
}

// SendNotification 发送通知
// @Summary 发送通知
// @Description 发送单个通知消息
// @Tags 通知管理
// @Accept json
// @Produce json
// @Param request body contracts.NotificationRequest true "通知请求参数"
// @Success 200 {object} map[string]interface{} "通知发送成功"
// @Failure 400 {object} map[string]interface{} "请求参数错误"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /notifications/send [post]
func (h *NotificationHandler) SendNotification(c *gin.Context) {
	var req contracts.NotificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.ErrorWithStatus(c, http.StatusBadRequest, 400, "Invalid request: "+err.Error())
		return
	}

	notificationService := h.container.GetNotificationService()
	response, err := notificationService.SendNotification(c.Request.Context(), req)
	if err != nil {
		httputil.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to send notification: "+err.Error())
		return
	}

	httputil.Success(c, gin.H{
		"message":      "Notification sent successfully",
		"notification": response,
	})
}

// SendBatchNotifications 批量发送通知
// @Summary 批量发送通知
// @Description 批量发送多个通知消息
// @Tags 通知管理
// @Accept json
// @Produce json
// @Param request body contracts.BatchNotificationRequest true "批量通知请求参数"
// @Success 200 {object} map[string]interface{} "批量通知发送结果"
// @Failure 400 {object} map[string]interface{} "请求参数错误"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /notifications/batch [post]
func (h *NotificationHandler) SendBatchNotifications(c *gin.Context) {
	var req contracts.BatchNotificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.ErrorWithStatus(c, http.StatusBadRequest, 400, "Invalid request: "+err.Error())
		return
	}

	notificationService := h.container.GetNotificationService()
	response, err := notificationService.SendBatchNotifications(c.Request.Context(), req)
	if err != nil {
		httputil.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to send batch notifications: "+err.Error())
		return
	}

	httputil.Success(c, gin.H{
		"message":       "Batch notifications sent",
		"success_count": response.SuccessCount,
		"failure_count": response.FailureCount,
		"results":       response.Results,
		"summary":       response.Summary,
	})
}

// GetNotificationHistory 获取通知历史
// @Summary 获取通知历史
// @Description 获取历史通知记录列表
// @Tags 通知管理
// @Produce json
// @Param limit query int false "每页数量" default(20)
// @Param offset query int false "偏移量" default(0)
// @Success 200 {object} map[string]interface{} "通知历史列表"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /notifications/history [get]
func (h *NotificationHandler) GetNotificationHistory(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 20
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	notificationService := h.container.GetNotificationService()
	history, err := notificationService.GetNotificationHistory(c.Request.Context(), limit, offset)
	if err != nil {
		httputil.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to get notification history: "+err.Error())
		return
	}

	httputil.Success(c, gin.H{
		"limit":         limit,
		"offset":        offset,
		"count":         len(history),
		"notifications": history,
	})
}

// GetNotificationStats 获取通知统计
// @Summary 获取通知统计
// @Description 获取通知系统的统计信息
// @Tags 通知管理
// @Produce json
// @Success 200 {object} map[string]interface{} "通知统计信息"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /notifications/stats [get]
func (h *NotificationHandler) GetNotificationStats(c *gin.Context) {
	notificationService := h.container.GetNotificationService()
	stats, err := notificationService.GetNotificationStats(c.Request.Context())
	if err != nil {
		httputil.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to get notification stats: "+err.Error())
		return
	}

	httputil.Success(c, stats)
}

// NotifyDownloadComplete 下载完成通知
// @Summary 下载完成通知
// @Description 发送下载完成通知
// @Tags 通知管理
// @Accept json
// @Produce json
// @Param request body contracts.DownloadNotificationRequest true "下载通知请求参数"
// @Success 200 {object} map[string]string "通知发送成功"
// @Failure 400 {object} map[string]interface{} "请求参数错误"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /notifications/download-complete [post]
func (h *NotificationHandler) NotifyDownloadComplete(c *gin.Context) {
	var req contracts.DownloadNotificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.ErrorWithStatus(c, http.StatusBadRequest, 400, "Invalid request: "+err.Error())
		return
	}

	notificationService := h.container.GetNotificationService()
	err := notificationService.NotifyDownloadComplete(c.Request.Context(), req)
	if err != nil {
		httputil.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to send download complete notification: "+err.Error())
		return
	}

	httputil.Success(c, gin.H{
		"message": "Download complete notification sent successfully",
	})
}

// NotifyDownloadFailed 下载失败通知
// @Summary 下载失败通知
// @Description 发送下载失败通知
// @Tags 通知管理
// @Accept json
// @Produce json
// @Param request body contracts.DownloadNotificationRequest true "下载通知请求参数"
// @Success 200 {object} map[string]string "通知发送成功"
// @Failure 400 {object} map[string]interface{} "请求参数错误"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /notifications/download-failed [post]
func (h *NotificationHandler) NotifyDownloadFailed(c *gin.Context) {
	var req contracts.DownloadNotificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.ErrorWithStatus(c, http.StatusBadRequest, 400, "Invalid request: "+err.Error())
		return
	}

	notificationService := h.container.GetNotificationService()
	err := notificationService.NotifyDownloadFailed(c.Request.Context(), req)
	if err != nil {
		httputil.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to send download failed notification: "+err.Error())
		return
	}

	httputil.Success(c, gin.H{
		"message": "Download failed notification sent successfully",
	})
}

// NotifyTaskComplete 任务完成通知
// @Summary 任务完成通知
// @Description 发送任务完成通知
// @Tags 通知管理
// @Accept json
// @Produce json
// @Param request body contracts.TaskNotificationRequest true "任务通知请求参数"
// @Success 200 {object} map[string]string "通知发送成功"
// @Failure 400 {object} map[string]interface{} "请求参数错误"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /notifications/task-complete [post]
func (h *NotificationHandler) NotifyTaskComplete(c *gin.Context) {
	var req contracts.TaskNotificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.ErrorWithStatus(c, http.StatusBadRequest, 400, "Invalid request: "+err.Error())
		return
	}

	notificationService := h.container.GetNotificationService()
	err := notificationService.NotifyTaskComplete(c.Request.Context(), req)
	if err != nil {
		httputil.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to send task complete notification: "+err.Error())
		return
	}

	httputil.Success(c, gin.H{
		"message": "Task complete notification sent successfully",
	})
}

// NotifyTaskFailed 任务失败通知
// @Summary 任务失败通知
// @Description 发送任务失败通知
// @Tags 通知管理
// @Accept json
// @Produce json
// @Param request body contracts.TaskNotificationRequest true "任务通知请求参数"
// @Success 200 {object} map[string]string "通知发送成功"
// @Failure 400 {object} map[string]interface{} "请求参数错误"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /notifications/task-failed [post]
func (h *NotificationHandler) NotifyTaskFailed(c *gin.Context) {
	var req contracts.TaskNotificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.ErrorWithStatus(c, http.StatusBadRequest, 400, "Invalid request: "+err.Error())
		return
	}

	notificationService := h.container.GetNotificationService()
	err := notificationService.NotifyTaskFailed(c.Request.Context(), req)
	if err != nil {
		httputil.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to send task failed notification: "+err.Error())
		return
	}

	httputil.Success(c, gin.H{
		"message": "Task failed notification sent successfully",
	})
}

// NotifySystemEvent 系统事件通知
// @Summary 系统事件通知
// @Description 发送系统事件通知
// @Tags 通知管理
// @Accept json
// @Produce json
// @Param request body contracts.SystemNotificationRequest true "系统通知请求参数"
// @Success 200 {object} map[string]string "通知发送成功"
// @Failure 400 {object} map[string]interface{} "请求参数错误"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /notifications/system-event [post]
func (h *NotificationHandler) NotifySystemEvent(c *gin.Context) {
	var req contracts.SystemNotificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.ErrorWithStatus(c, http.StatusBadRequest, 400, "Invalid request: "+err.Error())
		return
	}

	notificationService := h.container.GetNotificationService()
	err := notificationService.NotifySystemEvent(c.Request.Context(), req)
	if err != nil {
		httputil.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to send system event notification: "+err.Error())
		return
	}

	httputil.Success(c, gin.H{
		"message": "System event notification sent successfully",
	})
}

// GetNotificationConfig 获取通知配置
// @Summary 获取通知配置
// @Description 获取通知系统的配置信息
// @Tags 通知管理
// @Produce json
// @Success 200 {object} map[string]interface{} "通知配置信息"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /notifications/config [get]
func (h *NotificationHandler) GetNotificationConfig(c *gin.Context) {
	notificationService := h.container.GetNotificationService()
	config, err := notificationService.GetConfig(c.Request.Context())
	if err != nil {
		httputil.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to get notification config: "+err.Error())
		return
	}

	httputil.Success(c, config)
}
