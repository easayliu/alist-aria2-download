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

// HandleCallbackQuery handles callback queries by routing to appropriate handlers.
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

	// Route to appropriate handler based on callback data prefix
	if h.handlePreviewCallbacks(callback, chatID, data) {
		return
	}
	if h.handleDownloadCallbacks(callback, chatID, data) {
		return
	}
	if h.handleRenameCallbacks(callback, chatID, data) {
		return
	}

	// Respond to callback query before processing file operations
	h.controller.telegramClient.AnswerCallbackQuery(callback.ID, "")

	if h.handleBrowseCallbacks(callback, chatID, data) {
		return
	}
	if h.handleFileCallbacks(callback, chatID, data) {
		return
	}
	if h.handleDirCallbacks(callback, chatID, data) {
		return
	}

	// Handle menu callbacks
	h.handleMenuCallbacks(callback, chatID, userID, data)
}

// handlePreviewCallbacks handles preview-related callbacks.
// Returns true if the callback was handled.
func (h *CallbackHandler) handlePreviewCallbacks(callback *tgbotapi.CallbackQuery, chatID int64, data string) bool {
	if hours, found := strings.CutPrefix(data, "preview_hours|"); found {
		h.controller.telegramClient.AnswerCallbackQuery(callback.ID, "正在生成预览")
		h.controller.downloadHandler.HandleQuickPreview(chatID, []string{hours})
		return true
	}

	if minutes, found := strings.CutPrefix(data, "preview_minutes|"); found {
		h.controller.telegramClient.AnswerCallbackQuery(callback.ID, "正在生成预览")
		h.controller.downloadHandler.HandleQuickPreview(chatID, []string{minutes + "m"})
		return true
	}

	if data == "preview_custom" {
		h.controller.telegramClient.AnswerCallbackQuery(callback.ID, "请输入自定义时间")
		message := "<b>自定义预览</b>\n\n" +
			"请发送以下格式之一：\n" +
			"• <code>/download &lt;数字&gt;m</code> （例如：/download 30m 表示30分钟）\n" +
			"• <code>/download &lt;数字&gt;</code> （例如：/download 6 表示6小时）\n" +
			"• <code>/download YYYY-MM-DD YYYY-MM-DD</code>\n" +
			"• <code>/download 2025-01-01T00:00:00Z 2025-01-01T12:00:00Z</code>"
		h.controller.messageUtils.SendMessageHTMLWithAutoDelete(chatID, message, 30)
		return true
	}

	if data == "preview_cancel" {
		h.controller.telegramClient.AnswerCallbackQuery(callback.ID, "已关闭")
		if callback.Message != nil {
			h.controller.messageUtils.DeleteMessage(chatID, callback.Message.MessageID)
		}
		return true
	}

	return false
}

// handleDownloadCallbacks handles manual download confirmation callbacks.
// Returns true if the callback was handled.
func (h *CallbackHandler) handleDownloadCallbacks(callback *tgbotapi.CallbackQuery, chatID int64, data string) bool {
	if token, found := strings.CutPrefix(data, "manual_confirm|"); found {
		h.controller.telegramClient.AnswerCallbackQuery(callback.ID, "开始创建下载任务")
		if callback.Message != nil {
			h.controller.downloadHandler.HandleManualConfirm(chatID, token, callback.Message.MessageID)
		}
		return true
	}

	if token, found := strings.CutPrefix(data, "manual_cancel|"); found {
		h.controller.telegramClient.AnswerCallbackQuery(callback.ID, "已取消")
		if callback.Message != nil {
			h.controller.downloadHandler.HandleManualCancel(chatID, token, callback.Message.MessageID)
		}
		return true
	}

	return false
}

// handleRenameCallbacks handles rename-related callbacks.
// Returns true if the callback was handled.
func (h *CallbackHandler) handleRenameCallbacks(callback *tgbotapi.CallbackQuery, chatID int64, data string) bool {
	if strings.HasPrefix(data, "rename_apply|") {
		h.controller.telegramClient.AnswerCallbackQuery(callback.ID, "正在重命名")
		if callback.Message != nil {
			h.controller.fileHandler.HandleRenameApply(chatID, data, callback.Message.MessageID)
		}
		return true
	}

	if data == "rename_cancel" {
		h.controller.telegramClient.AnswerCallbackQuery(callback.ID, "已取消")
		if callback.Message != nil {
			h.controller.messageUtils.ClearInlineKeyboard(chatID, callback.Message.MessageID)
			h.controller.messageUtils.DeleteMessageAfterDelay(chatID, callback.Message.MessageID, 30)
		}
		return true
	}

	return false
}

// handleBrowseCallbacks handles file browsing callbacks.
// Returns true if the callback was handled.
func (h *CallbackHandler) handleBrowseCallbacks(callback *tgbotapi.CallbackQuery, chatID int64, data string) bool {
	messageID := callback.Message.MessageID

	// Handle browse_dir, browse_page, browse_refresh with same logic
	for _, prefix := range []string{"browse_dir:", "browse_page:", "browse_refresh:"} {
		if strings.HasPrefix(data, prefix) {
			parts := strings.Split(data, ":")
			if len(parts) >= 3 {
				path := h.controller.common.DecodeFilePath(parts[1])
				page, err := strconv.Atoi(parts[2])
				if err != nil || page < 1 {
					page = 1
				}
				if prefix == "browse_dir:" {
					logger.Info("Directory clicked", "encodedPath", parts[1], "decodedPath", path, "page", page)
				}
				h.controller.fileHandler.HandleBrowseFilesWithEdit(chatID, path, page, messageID)
			}
			return true
		}
	}

	return false
}

// handleFileCallbacks handles file operation callbacks.
// Returns true if the callback was handled.
func (h *CallbackHandler) handleFileCallbacks(callback *tgbotapi.CallbackQuery, chatID int64, data string) bool {
	messageID := callback.Message.MessageID

	if filePath, found := strings.CutPrefix(data, "file_menu:"); found {
		h.controller.fileHandler.HandleFileMenuWithEdit(chatID, h.controller.common.DecodeFilePath(filePath), messageID)
		return true
	}

	if filePath, found := strings.CutPrefix(data, "file_download:"); found {
		h.controller.fileHandler.HandleFileDownload(chatID, h.controller.common.DecodeFilePath(filePath))
		return true
	}

	if filePath, found := strings.CutPrefix(data, "file_info:"); found {
		h.controller.fileHandler.HandleFileInfoWithEdit(chatID, h.controller.common.DecodeFilePath(filePath), messageID)
		return true
	}

	if filePath, found := strings.CutPrefix(data, "file_link:"); found {
		h.controller.fileHandler.HandleFileLinkWithEdit(chatID, h.controller.common.DecodeFilePath(filePath), messageID)
		return true
	}

	if filePath, found := strings.CutPrefix(data, "file_rename:"); found {
		h.controller.fileHandler.HandleFileRename(chatID, h.controller.common.DecodeFilePath(filePath))
		return true
	}

	if filePath, found := strings.CutPrefix(data, "file_delete_confirm:"); found {
		h.controller.fileHandler.HandleFileDeleteConfirm(chatID, h.controller.common.DecodeFilePath(filePath), messageID)
		return true
	}

	if filePath, found := strings.CutPrefix(data, "file_delete:"); found {
		h.controller.telegramClient.AnswerCallbackQuery(callback.ID, "正在删除文件")
		h.controller.fileHandler.HandleFileDelete(chatID, h.controller.common.DecodeFilePath(filePath), messageID)
		return true
	}

	return false
}

// handleDirCallbacks handles directory operation callbacks.
// Returns true if the callback was handled.
func (h *CallbackHandler) handleDirCallbacks(callback *tgbotapi.CallbackQuery, chatID int64, data string) bool {
	messageID := callback.Message.MessageID

	if dirPath, found := strings.CutPrefix(data, "dir_menu:"); found {
		h.controller.fileHandler.HandleDirMenuWithEdit(chatID, h.controller.common.DecodeFilePath(dirPath), messageID)
		return true
	}

	if dirPath, found := strings.CutPrefix(data, "dir_delete_confirm:"); found {
		h.controller.fileHandler.HandleDirDeleteConfirm(chatID, h.controller.common.DecodeFilePath(dirPath), messageID)
		return true
	}

	if dirPath, found := strings.CutPrefix(data, "dir_delete:"); found {
		h.controller.telegramClient.AnswerCallbackQuery(callback.ID, "正在删除目录")
		h.controller.fileHandler.HandleDirDelete(chatID, h.controller.common.DecodeFilePath(dirPath), messageID)
		return true
	}

	if dirPath, found := strings.CutPrefix(data, "batch_rename:"); found {
		h.controller.fileHandler.HandleBatchRename(chatID, h.controller.common.DecodeFilePath(dirPath))
		return true
	}

	if dirPath, found := strings.CutPrefix(data, "batch_rename_confirm:"); found {
		h.controller.fileHandler.HandleBatchRenameConfirm(chatID, h.controller.common.DecodeFilePath(dirPath), messageID)
		return true
	}

	if dirPath, found := strings.CutPrefix(data, "download_dir:"); found {
		h.controller.fileHandler.HandleDownloadDirectoryConfirm(chatID, h.controller.common.DecodeFilePath(dirPath), messageID)
		return true
	}

	if dirPath, found := strings.CutPrefix(data, "download_dir_confirm:"); found {
		h.controller.fileHandler.HandleDownloadDirectoryExecute(chatID, h.controller.common.DecodeFilePath(dirPath), messageID)
		return true
	}

	if data == "download_dir_cancel" {
		h.controller.messageUtils.DeleteMessage(chatID, messageID)
		return true
	}

	return false
}

// handleMenuCallbacks handles menu navigation callbacks.
func (h *CallbackHandler) handleMenuCallbacks(callback *tgbotapi.CallbackQuery, chatID int64, userID int64, data string) {
	messageID := callback.Message.MessageID

	switch data {
	case "cmd_help":
		h.controller.menuCallbacks.HandleHelpWithEdit(chatID, messageID)
	case "cmd_status":
		h.controller.menuCallbacks.HandleStatusWithEdit(chatID, messageID)
	case "cmd_tasks":
		h.controller.taskHandler.HandleTasksWithEdit(chatID, userID, messageID)
	case "system_status":
		h.controller.menuCallbacks.HandleSystemStatusWithEdit(chatID, messageID)
	case "back_main":
		h.controller.menuCallbacks.HandleStartWithEdit(chatID, messageID)
	case "download_list":
		h.controller.statusHandler.HandleDownloadStatusAPIWithEdit(chatID, messageID)
	case "files_browse":
		h.controller.fileHandler.HandleFilesBrowseWithEdit(chatID, messageID)
	case "api_alist_login":
		h.controller.statusHandler.HandleAlistLoginWithEdit(chatID, messageID)
	case "api_health_check":
		h.controller.statusHandler.HandleHealthCheckWithEdit(chatID, messageID)
	default:
		h.controller.messageUtils.SendMessage(chatID, "未知操作")
	}
}
