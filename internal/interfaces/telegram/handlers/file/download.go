package file

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/easayliu/alist-aria2-download/internal/application/contracts"
	"github.com/easayliu/alist-aria2-download/internal/interfaces/telegram/utils"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// ================================
// 文件下载功能
// ================================

// HandleFileDownload 处理文件下载
func (h *Handler) HandleFileDownload(chatID int64, filePath string) {
	h.handleDownloadFileByPath(chatID, filePath)
}

// handleDownloadFileByPath 通过路径下载单个文件
func (h *Handler) handleDownloadFileByPath(chatID int64, filePath string) {
	ctx := context.Background()

	req := contracts.FileDownloadRequest{
		FilePath:     filePath,
		AutoClassify: true,
	}

	msgUtils := h.deps.GetMessageUtils()

	response, err := h.deps.GetFileService().DownloadFile(ctx, req)
	if err != nil {
		formatter := msgUtils.GetFormatter().(*utils.MessageFormatter)
		msgUtils.SendMessage(chatID, formatter.FormatError("创建文件下载任务", err))
		return
	}

	formatter := msgUtils.GetFormatter().(*utils.MessageFormatter)
	message := formatter.FormatFileDownloadSuccess(utils.FileDownloadSuccessData{
		Filename:     response.Filename,
		FilePath:     filePath,
		DownloadPath: response.Directory,
		TaskID:       response.ID,
		Size:         msgUtils.FormatFileSize(response.TotalSize),
		EscapeHTML:   msgUtils.EscapeHTML,
	})

	parentDir := filepath.Dir(filePath)
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📥 下载管理", "download_list"),
			tgbotapi.NewInlineKeyboardButtonData("📁 返回目录", fmt.Sprintf("browse_dir:%s:%d", h.deps.EncodeFilePath(parentDir), 1)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🏠 主菜单", "back_main"),
		),
	)

	msgUtils.SendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
}

// HandleDownloadDirectory 处理目录下载
func (h *Handler) HandleDownloadDirectory(chatID int64, dirPath string) {
	h.handleDownloadDirectoryByPath(chatID, dirPath)
}

// HandleDownloadDirectoryConfirm 显示下载目录确认对话框（发送新消息，保留主菜单）
func (h *Handler) HandleDownloadDirectoryConfirm(chatID int64, dirPath string, _ int) {
	msgUtils := h.deps.GetMessageUtils()

	message := "<b>📥 确认下载目录</b>\n\n"
	message += fmt.Sprintf("📂 目录: <code>%s</code>\n\n", msgUtils.EscapeHTML(dirPath))
	message += "⚠️ 将下载该目录下的所有视频文件（递归2层）\n\n"
	message += "是否确认下载？"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ 确认下载", fmt.Sprintf("download_dir_confirm:%s", h.deps.EncodeFilePath(dirPath))),
			tgbotapi.NewInlineKeyboardButtonData("❌ 取消", "download_dir_cancel"),
		),
	)

	msgUtils.SendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
}

// HandleDownloadDirectoryExecute 执行目录下载
func (h *Handler) HandleDownloadDirectoryExecute(chatID int64, dirPath string, messageID int) {
	msgUtils := h.deps.GetMessageUtils()
	msgUtils.EditMessageWithKeyboard(chatID, messageID, "⏳ 正在处理下载任务...", "HTML", nil)
	h.handleDownloadDirectoryByPathWithEdit(chatID, dirPath, messageID)
}

// handleDownloadDirectoryByPath 通过路径下载目录
func (h *Handler) handleDownloadDirectoryByPath(chatID int64, dirPath string) {
	ctx := context.Background()

	msgUtils := h.deps.GetMessageUtils()
	formatter := msgUtils.GetFormatter().(*utils.MessageFormatter)
	processingMsg := formatter.FormatTitle("⏳", "正在处理手动下载任务") + "\n\n" +
		formatter.FormatField("目录路径", dirPath)
	msgUtils.SendMessageHTMLWithAutoDelete(chatID, processingMsg, 30)

	req := contracts.DirectoryDownloadRequest{
		DirectoryPath: dirPath,
		Recursive:     true,
		VideoOnly:     true,
		AutoClassify:  true,
	}

	result, err := h.deps.GetFileService().DownloadDirectory(ctx, req)
	if err != nil {
		msgUtils.SendMessage(chatID, formatter.FormatError("处理", err))
		return
	}

	if result.SuccessCount == 0 {
		if result.Summary.VideoFiles == 0 {
			message := formatter.FormatNoFilesFound("手动下载完成", dirPath)
			msgUtils.SendMessageHTML(chatID, message)
		} else {
			msgUtils.SendMessage(chatID, formatter.FormatSimpleError("所有文件下载创建失败，请检查日志"))
		}
		return
	}

	message := formatter.FormatTimeRangeDownloadResult(utils.TimeRangeDownloadResultData{
		TimeDescription: dirPath,
		Path:            dirPath,
		TotalFiles:      result.Summary.TotalFiles,
		TotalSize:       msgUtils.FormatFileSize(result.Summary.TotalSize),
		MovieCount:      result.Summary.MovieFiles,
		TVCount:         result.Summary.TVFiles,
		OtherCount:      result.Summary.OtherFiles,
		SuccessCount:    result.SuccessCount,
		FailCount:       result.FailureCount,
		EscapeHTML:      msgUtils.EscapeHTML,
	})

	msgUtils.SendMessageHTMLWithAutoDelete(chatID, message, 30)
}

// handleDownloadDirectoryByPathWithEdit 下载目录并在指定消息上编辑显示结果
func (h *Handler) handleDownloadDirectoryByPathWithEdit(chatID int64, dirPath string, messageID int) {
	ctx := context.Background()
	msgUtils := h.deps.GetMessageUtils()
	formatter := msgUtils.GetFormatter().(*utils.MessageFormatter)

	req := contracts.DirectoryDownloadRequest{
		DirectoryPath: dirPath,
		Recursive:     true,
		VideoOnly:     true,
		AutoClassify:  true,
	}

	result, err := h.deps.GetFileService().DownloadDirectory(ctx, req)
	if err != nil {
		msgUtils.EditMessageWithKeyboard(chatID, messageID, formatter.FormatError("处理", err), "HTML", nil)
		msgUtils.DeleteMessageAfterDelay(chatID, messageID, 30)
		return
	}

	if result.SuccessCount == 0 {
		var message string
		if result.Summary.VideoFiles == 0 {
			message = formatter.FormatNoFilesFound("手动下载完成", dirPath)
		} else {
			message = formatter.FormatSimpleError("所有文件下载创建失败，请检查日志")
		}
		msgUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", nil)
		msgUtils.DeleteMessageAfterDelay(chatID, messageID, 30)
		return
	}

	message := formatter.FormatTimeRangeDownloadResult(utils.TimeRangeDownloadResultData{
		TimeDescription: dirPath,
		Path:            dirPath,
		TotalFiles:      result.Summary.TotalFiles,
		TotalSize:       msgUtils.FormatFileSize(result.Summary.TotalSize),
		MovieCount:      result.Summary.MovieFiles,
		TVCount:         result.Summary.TVFiles,
		OtherCount:      result.Summary.OtherFiles,
		SuccessCount:    result.SuccessCount,
		FailCount:       result.FailureCount,
		EscapeHTML:      msgUtils.EscapeHTML,
	})

	msgUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", nil)
	msgUtils.DeleteMessageAfterDelay(chatID, messageID, 30)
}
