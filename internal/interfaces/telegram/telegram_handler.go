package telegram

import (
	"github.com/easayliu/alist-aria2-download/internal/application/services"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/config"
	telegramInfra "github.com/easayliu/alist-aria2-download/internal/infrastructure/telegram"
	"github.com/gin-gonic/gin"
)

// TelegramHandler is a compatibility wrapper
// Maintains the exact same public interface as legacy version to ensure compatibility
type TelegramHandler struct {
	controller *TelegramController
}

func NewTelegramHandler(cfg *config.Config, notificationService *services.NotificationService, fileService *services.FileService, schedulerService *services.SchedulerService, container *services.ServiceContainer, telegramClient *telegramInfra.Client) *TelegramHandler {
	controller := NewTelegramController(cfg, notificationService, fileService, schedulerService, container, telegramClient)
	return &TelegramHandler{
		controller: controller,
	}
}

// ================================
// Public interface delegation - maintains full compatibility
// ================================

// Webhook handles webhook requests (delegates to internal controller)
func (h *TelegramHandler) Webhook(c *gin.Context) {
	h.controller.Webhook(c)
}

// StartPolling starts update polling (delegates to internal controller)
func (h *TelegramHandler) StartPolling() {
	h.controller.StartPolling()
}

// StopPolling stops update polling (delegates to internal controller)
func (h *TelegramHandler) StopPolling() {
	h.controller.StopPolling()
}

// FormatFileSize formats file size (delegates to internal controller)
func (h *TelegramHandler) FormatFileSize(size int64) string {
	return h.controller.FormatFileSize(size)
}

// ================================
// Internal accessors - for testing and debugging only
// ================================

// GetController provides access to internal controller (for testing and debugging)
func (h *TelegramHandler) GetController() *TelegramController {
	return h.controller
}