package telegram

import (
	"strconv"
	"strings"

	"github.com/easayliu/alist-aria2-download/pkg/logger"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// CallbackHandler handles Telegram callback queries
type CallbackHandler struct {
	controller *TelegramController
}

// NewCallbackHandler creates a new callback query handler
func NewCallbackHandler(controller *TelegramController) *CallbackHandler {
	return &CallbackHandler{
		controller: controller,
	}
}

// HandleCallbackQuery handles callback queries
func (h *CallbackHandler) HandleCallbackQuery(update *tgbotapi.Update) {
	callback := update.CallbackQuery
	if callback == nil {
		return
	}

	userID := callback.From.ID
	chatID := callback.Message.Chat.ID
	data := callback.Data

	// Authorization check
	if !h.controller.telegramClient.IsAuthorized(userID) {
		h.controller.telegramClient.AnswerCallbackQuery(callback.ID, "未授权访问")
		return
	}

	logger.Info("Received callback query:", "data", data, "from", callback.From.UserName, "chatID", chatID)

	// Handle preview-related callbacks
	if strings.HasPrefix(data, "preview_hours|") {
		hours := strings.TrimPrefix(data, "preview_hours|")
		h.controller.telegramClient.AnswerCallbackQuery(callback.ID, "正在生成预览")
		if callback.Message != nil {
			h.controller.messageUtils.ClearInlineKeyboard(chatID, callback.Message.MessageID)
		}
		h.controller.downloadHandler.HandleQuickPreview(chatID, []string{hours})
		return
	}

	if data == "preview_custom" {
		h.controller.telegramClient.AnswerCallbackQuery(callback.ID, "请输入自定义时间")
		if callback.Message != nil {
			h.controller.messageUtils.ClearInlineKeyboard(chatID, callback.Message.MessageID)
		}
		message := "<b>自定义预览</b>\n\n" +
			"请发送以下格式之一：\n" +
			"• <code>/download &lt;小时数&gt;</code> （例如：/download 6）\n" +
			"• <code>/download YYYY-MM-DD YYYY-MM-DD</code>\n" +
			"• <code>/download 2025-01-01T00:00:00Z 2025-01-01T12:00:00Z</code>"
		h.controller.messageUtils.SendMessageHTML(chatID, message)
		return
	}

	if data == "preview_cancel" {
		h.controller.telegramClient.AnswerCallbackQuery(callback.ID, "已关闭")
		if callback.Message != nil {
			h.controller.messageUtils.ClearInlineKeyboard(chatID, callback.Message.MessageID)
		}
		return
	}

	// Handle manual download confirmation callbacks
	if strings.HasPrefix(data, "manual_confirm|") {
		token := strings.TrimPrefix(data, "manual_confirm|")
		h.controller.telegramClient.AnswerCallbackQuery(callback.ID, "开始创建下载任务")
		if callback.Message != nil {
			h.controller.downloadHandler.HandleManualConfirm(chatID, token, callback.Message.MessageID)
		}
		return
	}

	if strings.HasPrefix(data, "manual_cancel|") {
		token := strings.TrimPrefix(data, "manual_cancel|")
		h.controller.telegramClient.AnswerCallbackQuery(callback.ID, "已取消")
		if callback.Message != nil {
			h.controller.downloadHandler.HandleManualCancel(chatID, token, callback.Message.MessageID)
		}
		return
	}

	// First respond to callback query
	h.controller.telegramClient.AnswerCallbackQuery(callback.ID, "")

	// Handle file browsing related callbacks
	if strings.HasPrefix(data, "browse_dir:") {
		parts := strings.Split(data, ":")
		if len(parts) >= 3 {
			encodedPath := parts[1]
			path := h.controller.common.DecodeFilePath(encodedPath)
			page, _ := strconv.Atoi(parts[2])
			if page < 1 {
				page = 1
			}
			logger.Info("Directory clicked", "encodedPath", encodedPath, "decodedPath", path, "page", page)
			h.controller.fileHandler.HandleBrowseFilesWithEdit(chatID, path, page, callback.Message.MessageID)
		}
		return
	}

	if strings.HasPrefix(data, "browse_page:") {
		parts := strings.Split(data, ":")
		if len(parts) >= 3 {
			path := h.controller.common.DecodeFilePath(parts[1])
			page, _ := strconv.Atoi(parts[2])
			if page < 1 {
				page = 1
			}
			h.controller.fileHandler.HandleBrowseFilesWithEdit(chatID, path, page, callback.Message.MessageID)
		}
		return
	}

	if strings.HasPrefix(data, "browse_refresh:") {
		parts := strings.Split(data, ":")
		if len(parts) >= 3 {
			path := h.controller.common.DecodeFilePath(parts[1])
			page, _ := strconv.Atoi(parts[2])
			if page < 1 {
				page = 1
			}
			h.controller.fileHandler.HandleBrowseFilesWithEdit(chatID, path, page, callback.Message.MessageID)
		}
		return
	}

	if strings.HasPrefix(data, "file_menu:") {
		filePath := h.controller.common.DecodeFilePath(strings.TrimPrefix(data, "file_menu:"))
		h.controller.fileHandler.HandleFileMenuWithEdit(chatID, filePath, callback.Message.MessageID)
		return
	}

	if strings.HasPrefix(data, "file_download:") {
		filePath := h.controller.common.DecodeFilePath(strings.TrimPrefix(data, "file_download:"))
		h.controller.fileHandler.HandleFileDownload(chatID, filePath)
		return
	}

	if strings.HasPrefix(data, "file_info:") {
		filePath := h.controller.common.DecodeFilePath(strings.TrimPrefix(data, "file_info:"))
		h.controller.fileHandler.HandleFileInfoWithEdit(chatID, filePath, callback.Message.MessageID)
		return
	}

	if strings.HasPrefix(data, "file_link:") {
		filePath := h.controller.common.DecodeFilePath(strings.TrimPrefix(data, "file_link:"))
		h.controller.fileHandler.HandleFileLinkWithEdit(chatID, filePath, callback.Message.MessageID)
		return
	}

	if strings.HasPrefix(data, "download_dir:") {
		dirPath := h.controller.common.DecodeFilePath(strings.TrimPrefix(data, "download_dir:"))
		h.controller.fileHandler.HandleDownloadDirectory(chatID, dirPath)
		return
	}

	// Handle menu callbacks
	switch data {
	case "cmd_help":
		h.controller.menuCallbacks.HandleHelpWithEdit(chatID, callback.Message.MessageID)
	case "cmd_status":
		h.controller.menuCallbacks.HandleStatusWithEdit(chatID, callback.Message.MessageID)
	case "cmd_manage":
		h.controller.menuCallbacks.HandleManageWithEdit(chatID, callback.Message.MessageID)
	case "menu_download":
		h.controller.menuCallbacks.HandleDownloadMenuWithEdit(chatID, callback.Message.MessageID)
	case "menu_files":
		h.controller.menuCallbacks.HandleFilesMenuWithEdit(chatID, callback.Message.MessageID)
	case "menu_system":
		h.controller.menuCallbacks.HandleSystemMenuWithEdit(chatID, callback.Message.MessageID)
	case "menu_status":
		h.controller.menuCallbacks.HandleStatusMenuWithEdit(chatID, callback.Message.MessageID)
	case "show_yesterday_options", "api_yesterday_files", "api_yesterday_files_preview", "api_yesterday_download":
		// Yesterday files feature removed, redirect to scheduled tasks
		h.controller.taskHandler.HandleTasksWithEdit(chatID, userID, callback.Message.MessageID)
	case "cmd_tasks":
		h.controller.taskHandler.HandleTasksWithEdit(chatID, userID, callback.Message.MessageID)
	case "api_download_status":
		h.controller.statusHandler.HandleDownloadStatusAPIWithEdit(chatID, callback.Message.MessageID)
	case "api_alist_login":
		h.controller.statusHandler.HandleAlistLoginWithEdit(chatID, callback.Message.MessageID)
	case "api_health_check":
		h.controller.statusHandler.HandleHealthCheckWithEdit(chatID, callback.Message.MessageID)
	case "back_main":
		h.controller.menuCallbacks.HandleStartWithEdit(chatID, callback.Message.MessageID)
	// Download management functions
	case "download_list":
		h.controller.statusHandler.HandleDownloadStatusAPIWithEdit(chatID, callback.Message.MessageID)
	case "download_create":
		h.controller.downloadHandler.HandleDownloadCreateWithEdit(chatID, callback.Message.MessageID)
	case "download_control":
		h.controller.downloadHandler.HandleDownloadControlWithEdit(chatID, callback.Message.MessageID)
	case "download_delete":
		h.controller.downloadHandler.HandleDownloadDeleteWithEdit(chatID, callback.Message.MessageID)
	// File browsing functions
	case "files_browse":
		h.controller.fileHandler.HandleFilesBrowseWithEdit(chatID, callback.Message.MessageID)
	case "files_search":
		h.controller.fileHandler.HandleFilesSearchWithEdit(chatID, callback.Message.MessageID)
	case "files_info":
		h.controller.fileHandler.HandleFilesInfoWithEdit(chatID, callback.Message.MessageID)
	case "files_download":
		h.controller.fileHandler.HandleFilesDownloadWithEdit(chatID, callback.Message.MessageID)
	case "api_alist_files":
		h.controller.fileHandler.HandleAlistFilesWithEdit(chatID, callback.Message.MessageID)
	// System management functions
	case "system_info":
		h.controller.menuCallbacks.HandleSystemInfoWithEdit(chatID, callback.Message.MessageID)
	// Status monitoring functions
	case "status_realtime":
		h.controller.statusHandler.HandleStatusRealtimeWithEdit(chatID, callback.Message.MessageID)
	case "status_storage":
		h.controller.statusHandler.HandleStatusStorageWithEdit(chatID, callback.Message.MessageID)
	case "status_history":
		h.controller.statusHandler.HandleStatusHistoryWithEdit(chatID, callback.Message.MessageID)
	default:
		h.controller.messageUtils.SendMessage(chatID, "未知操作")
	}
}