package telegram

import (
	"github.com/easayliu/alist-aria2-download/internal/application/services"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/config"
	"github.com/gin-gonic/gin"
)

// TelegramHandler 兼容性包装器
// 保持与旧版本完全相同的公共接口，确保兼容性
type TelegramHandler struct {
	controller *TelegramController
}

// NewTelegramHandler 创建新的 Telegram 处理器（兼容性接口）
// 保持与原版本完全相同的函数签名
func NewTelegramHandler(cfg *config.Config, notificationService *services.NotificationService, fileService *services.FileService, schedulerService *services.SchedulerService) *TelegramHandler {
	controller := NewTelegramController(cfg, notificationService, fileService, schedulerService)
	return &TelegramHandler{
		controller: controller,
	}
}

// ================================
// 公共接口委托 - 保持完全兼容性
// ================================

// Webhook 处理 Webhook 请求（委托给控制器）
func (h *TelegramHandler) Webhook(c *gin.Context) {
	h.controller.Webhook(c)
}

// StartPolling 开始轮询（委托给控制器）
func (h *TelegramHandler) StartPolling() {
	h.controller.StartPolling()
}

// StopPolling 停止轮询（委托给控制器）
func (h *TelegramHandler) StopPolling() {
	h.controller.StopPolling()
}

// FormatFileSize 格式化文件大小（委托给控制器）
func (h *TelegramHandler) FormatFileSize(size int64) string {
	return h.controller.FormatFileSize(size)
}

// ================================
// 内部访问器 - 供测试和调试使用
// ================================

// GetController 获取内部控制器（用于测试和调试）
func (h *TelegramHandler) GetController() *TelegramController {
	return h.controller
}