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
		// 不清除键盘，保留菜单供用户继续使用
		h.controller.downloadHandler.HandleQuickPreview(chatID, []string{hours})
		return
	}

	if strings.HasPrefix(data, "preview_minutes|") {
		minutes := strings.TrimPrefix(data, "preview_minutes|")
		h.controller.telegramClient.AnswerCallbackQuery(callback.ID, "正在生成预览")
		// 不清除键盘，保留菜单供用户继续使用
		h.controller.downloadHandler.HandleQuickPreview(chatID, []string{minutes + "m"})
		return
	}

	if data == "preview_custom" {
		h.controller.telegramClient.AnswerCallbackQuery(callback.ID, "请输入自定义时间")
		// 不清除键盘，保留菜单供用户继续使用
		message := "<b>自定义预览</b>\n\n" +
			"请发送以下格式之一：\n" +
			"• <code>/download &lt;数字&gt;m</code> （例如：/download 30m 表示30分钟）\n" +
			"• <code>/download &lt;数字&gt;</code> （例如：/download 6 表示6小时）\n" +
			"• <code>/download YYYY-MM-DD YYYY-MM-DD</code>\n" +
			"• <code>/download 2025-01-01T00:00:00Z 2025-01-01T12:00:00Z</code>"
		h.controller.messageUtils.SendMessageHTMLWithAutoDelete(chatID, message, 30)
		return
	}

	if data == "preview_cancel" {
		h.controller.telegramClient.AnswerCallbackQuery(callback.ID, "已关闭")
		if callback.Message != nil {
			h.controller.messageUtils.DeleteMessage(chatID, callback.Message.MessageID)
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

	if strings.HasPrefix(data, "rename_apply|") {
		h.controller.telegramClient.AnswerCallbackQuery(callback.ID, "正在重命名")
		if callback.Message != nil {
			h.handleRenameApply(chatID, data, callback.Message.MessageID)
		}
		return
	}

	if data == "rename_cancel" {
		h.controller.telegramClient.AnswerCallbackQuery(callback.ID, "已取消")
		if callback.Message != nil {
			h.controller.messageUtils.ClearInlineKeyboard(chatID, callback.Message.MessageID)
			h.controller.messageUtils.DeleteMessageAfterDelay(chatID, callback.Message.MessageID, 30)
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

	if strings.HasPrefix(data, "dir_menu:") {
		dirPath := h.controller.common.DecodeFilePath(strings.TrimPrefix(data, "dir_menu:"))
		h.controller.fileHandler.HandleDirMenuWithEdit(chatID, dirPath, callback.Message.MessageID)
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

	if strings.HasPrefix(data, "file_rename:") {
		filePath := h.controller.common.DecodeFilePath(strings.TrimPrefix(data, "file_rename:"))
		h.controller.fileHandler.HandleFileRename(chatID, filePath)
		return
	}

	if strings.HasPrefix(data, "file_delete_confirm:") {
		filePath := h.controller.common.DecodeFilePath(strings.TrimPrefix(data, "file_delete_confirm:"))
		h.controller.telegramClient.AnswerCallbackQuery(callback.ID, "")
		h.controller.fileHandler.HandleFileDeleteConfirm(chatID, filePath, callback.Message.MessageID)
		return
	}

	if strings.HasPrefix(data, "file_delete:") {
		filePath := h.controller.common.DecodeFilePath(strings.TrimPrefix(data, "file_delete:"))
		h.controller.telegramClient.AnswerCallbackQuery(callback.ID, "正在删除文件")
		h.controller.fileHandler.HandleFileDelete(chatID, filePath, callback.Message.MessageID)
		return
	}

	if strings.HasPrefix(data, "dir_delete_confirm:") {
		dirPath := h.controller.common.DecodeFilePath(strings.TrimPrefix(data, "dir_delete_confirm:"))
		h.controller.telegramClient.AnswerCallbackQuery(callback.ID, "")
		h.controller.fileHandler.HandleDirDeleteConfirm(chatID, dirPath, callback.Message.MessageID)
		return
	}

	if strings.HasPrefix(data, "dir_delete:") {
		dirPath := h.controller.common.DecodeFilePath(strings.TrimPrefix(data, "dir_delete:"))
		h.controller.telegramClient.AnswerCallbackQuery(callback.ID, "正在删除目录")
		h.controller.fileHandler.HandleDirDelete(chatID, dirPath, callback.Message.MessageID)
		return
	}

	if strings.HasPrefix(data, "batch_rename:") {
		dirPath := h.controller.common.DecodeFilePath(strings.TrimPrefix(data, "batch_rename:"))
		h.controller.fileHandler.HandleBatchRename(chatID, dirPath)
		return
	}

	if strings.HasPrefix(data, "batch_rename_confirm:") {
		dirPath := h.controller.common.DecodeFilePath(strings.TrimPrefix(data, "batch_rename_confirm:"))
		h.controller.fileHandler.HandleBatchRenameConfirm(chatID, dirPath, callback.Message.MessageID)
		return
	}

	if strings.HasPrefix(data, "download_dir:") {
		dirPath := h.controller.common.DecodeFilePath(strings.TrimPrefix(data, "download_dir:"))
		h.controller.fileHandler.HandleDownloadDirectoryConfirm(chatID, dirPath, callback.Message.MessageID)
		return
	}

	if strings.HasPrefix(data, "download_dir_confirm:") {
		dirPath := h.controller.common.DecodeFilePath(strings.TrimPrefix(data, "download_dir_confirm:"))
		h.controller.fileHandler.HandleDownloadDirectoryExecute(chatID, dirPath, callback.Message.MessageID)
		return
	}

	if data == "download_dir_cancel" {
		// 直接删除确认消息
		h.controller.messageUtils.DeleteMessage(chatID, callback.Message.MessageID)
		return
	}

	// Handle menu callbacks
	switch data {
	case "cmd_help":
		h.controller.menuCallbacks.HandleHelpWithEdit(chatID, callback.Message.MessageID)
	case "cmd_status":
		h.controller.menuCallbacks.HandleStatusWithEdit(chatID, callback.Message.MessageID)
	case "cmd_tasks":
		h.controller.taskHandler.HandleTasksWithEdit(chatID, userID, callback.Message.MessageID)
	case "system_status":
		h.controller.menuCallbacks.HandleSystemStatusWithEdit(chatID, callback.Message.MessageID)
	case "back_main":
		h.controller.menuCallbacks.HandleStartWithEdit(chatID, callback.Message.MessageID)
	case "download_list":
		h.controller.statusHandler.HandleDownloadStatusAPIWithEdit(chatID, callback.Message.MessageID)
	case "files_browse":
		h.controller.fileHandler.HandleFilesBrowseWithEdit(chatID, callback.Message.MessageID)
	case "api_alist_login":
		h.controller.statusHandler.HandleAlistLoginWithEdit(chatID, callback.Message.MessageID)
	case "api_health_check":
		h.controller.statusHandler.HandleHealthCheckWithEdit(chatID, callback.Message.MessageID)
	default:
		h.controller.messageUtils.SendMessage(chatID, "未知操作")
	}
}
