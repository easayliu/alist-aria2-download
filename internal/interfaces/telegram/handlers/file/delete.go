package file

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/easayliu/alist-aria2-download/internal/interfaces/telegram/utils"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// ================================
// 文件/目录删除功能
// ================================

// HandleFileDeleteConfirm 处理文件删除确认
func (h *Handler) HandleFileDeleteConfirm(chatID int64, filePath string, messageID int) {
	fileName := filepath.Base(filePath)
	parentDir := filepath.Dir(filePath)

	msgUtils := h.deps.GetMessageUtils()
	formatter := msgUtils.GetFormatter().(*utils.MessageFormatter)
	message := formatter.FormatTitle("⚠️", "确认删除文件") + "\n\n" +
		formatter.FormatFieldCode("文件名", msgUtils.EscapeHTML(fileName)) + "\n" +
		formatter.FormatFieldCode("路径", msgUtils.EscapeHTML(parentDir)) + "\n\n" +
		"<b>⚠️ 此操作不可撤销，确认删除吗？</b>"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ 确认删除", fmt.Sprintf("file_delete:%s", h.deps.EncodeFilePath(filePath))),
			tgbotapi.NewInlineKeyboardButtonData("❌ 取消", fmt.Sprintf("file_menu:%s", h.deps.EncodeFilePath(filePath))),
		),
	)

	if messageID > 0 {
		msgUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
	} else {
		msgUtils.SendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
	}
}

// HandleFileDelete 处理文件删除
func (h *Handler) HandleFileDelete(chatID int64, filePath string, messageID int) {
	fileName := filepath.Base(filePath)
	parentDir := filepath.Dir(filePath)

	msgUtils := h.deps.GetMessageUtils()
	formatter := msgUtils.GetFormatter().(*utils.MessageFormatter)

	ctx := context.Background()
	if err := h.deps.GetFileService().DeleteFile(ctx, filePath); err != nil {
		msgUtils.SendMessage(chatID, formatter.FormatError("删除文件", err))
		return
	}

	message := formatter.FormatTitle("✅", "文件删除成功") + "\n\n" +
		formatter.FormatFieldCode("文件名", msgUtils.EscapeHTML(fileName)) + "\n" +
		formatter.FormatFieldCode("原路径", msgUtils.EscapeHTML(parentDir))

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📁 返回目录", fmt.Sprintf("browse_dir:%s:%d", h.deps.EncodeFilePath(parentDir), 1)),
			tgbotapi.NewInlineKeyboardButtonData("🏠 主菜单", "back_main"),
		),
	)

	if messageID > 0 {
		msgUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
	} else {
		msgUtils.SendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
	}
}

// HandleDirDeleteConfirm 处理目录删除确认
func (h *Handler) HandleDirDeleteConfirm(chatID int64, dirPath string, messageID int) {
	dirName := filepath.Base(dirPath)
	parentDir := filepath.Dir(dirPath)

	msgUtils := h.deps.GetMessageUtils()
	formatter := msgUtils.GetFormatter().(*utils.MessageFormatter)
	message := formatter.FormatTitle("⚠️", "确认删除目录") + "\n\n" +
		formatter.FormatFieldCode("目录名", msgUtils.EscapeHTML(dirName)) + "\n" +
		formatter.FormatFieldCode("路径", msgUtils.EscapeHTML(parentDir)) + "\n\n" +
		"<b>⚠️ 此操作不可撤销，将删除目录及其所有内容，确认删除吗？</b>"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ 确认删除", fmt.Sprintf("dir_delete:%s", h.deps.EncodeFilePath(dirPath))),
			tgbotapi.NewInlineKeyboardButtonData("❌ 取消", fmt.Sprintf("dir_menu:%s", h.deps.EncodeFilePath(dirPath))),
		),
	)

	if messageID > 0 {
		msgUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
	} else {
		msgUtils.SendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
	}
}

// HandleDirDelete 处理目录删除
func (h *Handler) HandleDirDelete(chatID int64, dirPath string, messageID int) {
	dirName := filepath.Base(dirPath)
	parentDir := filepath.Dir(dirPath)

	msgUtils := h.deps.GetMessageUtils()
	formatter := msgUtils.GetFormatter().(*utils.MessageFormatter)

	ctx := context.Background()
	if err := h.deps.GetFileService().DeleteFile(ctx, dirPath); err != nil {
		msgUtils.SendMessage(chatID, formatter.FormatError("删除目录", err))
		return
	}

	message := formatter.FormatTitle("✅", "目录删除成功") + "\n\n" +
		formatter.FormatFieldCode("目录名", msgUtils.EscapeHTML(dirName)) + "\n" +
		formatter.FormatFieldCode("原路径", msgUtils.EscapeHTML(parentDir))

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📁 返回上级", fmt.Sprintf("browse_dir:%s:%d", h.deps.EncodeFilePath(parentDir), 1)),
			tgbotapi.NewInlineKeyboardButtonData("🏠 主菜单", "back_main"),
		),
	)

	if messageID > 0 {
		msgUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
	} else {
		msgUtils.SendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
	}
}
