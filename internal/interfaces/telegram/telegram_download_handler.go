package telegram

import (
	"github.com/easayliu/alist-aria2-download/internal/application/contracts"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/config"
	downloadhandler "github.com/easayliu/alist-aria2-download/internal/interfaces/telegram/handlers/download"
	"github.com/easayliu/alist-aria2-download/internal/interfaces/telegram/types"
)

// DownloadHandler handles download-related functions (adapter)
type DownloadHandler struct {
	controller *TelegramController
	handler    *downloadhandler.Handler
}

// NewDownloadHandler creates a new download handler
func NewDownloadHandler(controller *TelegramController) *DownloadHandler {
	dh := &DownloadHandler{
		controller: controller,
	}
	dh.handler = downloadhandler.NewHandler(dh)
	return dh
}

// ================================
// 实现 downloadhandler.Deps 接口
// ================================

func (h *DownloadHandler) GetMessageUtils() types.MessageSender {
	return h.controller.messageUtils
}

func (h *DownloadHandler) GetFileService() contracts.FileService {
	return h.controller.fileService
}

func (h *DownloadHandler) GetDownloadService() contracts.DownloadService {
	return h.controller.downloadService
}

func (h *DownloadHandler) GetConfig() *config.Config {
	return h.controller.config
}

// ================================
// 代理方法
// ================================

func (h *DownloadHandler) HandleQuickPreview(chatID int64, timeArgs []string) {
	h.handler.HandleQuickPreview(chatID, timeArgs)
}

func (h *DownloadHandler) HandleManualConfirm(chatID int64, token string, messageID int) {
	h.handler.HandleManualConfirm(chatID, token, messageID)
}

func (h *DownloadHandler) HandleManualCancel(chatID int64, token string, messageID int) {
	h.handler.HandleManualCancel(chatID, token, messageID)
}
