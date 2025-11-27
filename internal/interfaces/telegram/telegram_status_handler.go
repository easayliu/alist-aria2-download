package telegram

import (
	"github.com/easayliu/alist-aria2-download/internal/application/contracts"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/config"
	statushandler "github.com/easayliu/alist-aria2-download/internal/interfaces/telegram/handlers/status"
	"github.com/easayliu/alist-aria2-download/internal/interfaces/telegram/types"
)

// StatusHandler handles status query related functions (adapter)
type StatusHandler struct {
	controller *TelegramController
	handler    *statushandler.Handler
}

// NewStatusHandler creates a new status handler
func NewStatusHandler(controller *TelegramController) *StatusHandler {
	sh := &StatusHandler{
		controller: controller,
	}
	sh.handler = statushandler.NewHandler(sh)
	return sh
}

// ================================
// 实现 statushandler.Deps 接口
// ================================

func (h *StatusHandler) GetMessageUtils() types.MessageSender {
	return h.controller.messageUtils
}

func (h *StatusHandler) GetDownloadService() contracts.DownloadService {
	return h.controller.downloadService
}

func (h *StatusHandler) GetConfig() *config.Config {
	return h.controller.config
}

// ================================
// 代理方法
// ================================

func (h *StatusHandler) HandleDownloadStatusAPIWithEdit(chatID int64, messageID int) {
	h.handler.HandleDownloadStatusAPIWithEdit(chatID, messageID)
}

func (h *StatusHandler) HandleAlistLoginWithEdit(chatID int64, messageID int) {
	h.handler.HandleAlistLoginWithEdit(chatID, messageID)
}

func (h *StatusHandler) HandleHealthCheckWithEdit(chatID int64, messageID int) {
	h.handler.HandleHealthCheckWithEdit(chatID, messageID)
}

func (h *StatusHandler) HandleStatusRealtimeWithEdit(chatID int64, messageID int) {
	h.handler.HandleStatusRealtimeWithEdit(chatID, messageID)
}

func (h *StatusHandler) HandleStatusStorageWithEdit(chatID int64, messageID int) {
	h.handler.HandleStatusStorageWithEdit(chatID, messageID)
}

func (h *StatusHandler) HandleStatusHistoryWithEdit(chatID int64, messageID int) {
	h.handler.HandleStatusHistoryWithEdit(chatID, messageID)
}
