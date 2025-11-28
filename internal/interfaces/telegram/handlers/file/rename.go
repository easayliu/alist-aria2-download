package file

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/easayliu/alist-aria2-download/internal/interfaces/telegram/utils"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// ================================
// 文件重命名功能
// ================================

// HandleFileRename 处理单文件重命名
func (h *Handler) HandleFileRename(chatID int64, filePath string) {
	h.deps.HandleRenameCommand(chatID, fmt.Sprintf("/rename %s", filePath))
}

// HandleRenameApply 处理重命名应用回调
// 当用户从重命名建议列表中选择某个建议时调用
func (h *Handler) HandleRenameApply(chatID int64, callbackData string, messageID int) {
	ctx := context.Background()
	msgUtils := h.deps.GetMessageUtils()
	formatter := msgUtils.GetFormatter().(*utils.MessageFormatter)

	// 解析回调数据: rename_apply|索引|base64编码的路径
	parts := strings.Split(callbackData, "|")
	if len(parts) < 3 {
		msgUtils.EditMessageWithKeyboard(chatID, messageID, "回调数据格式错误", "HTML", nil)
		return
	}

	indexStr := parts[1]
	encodedPath := parts[2]

	// 解码路径
	pathBytes, err := base64.URLEncoding.DecodeString(encodedPath)
	if err != nil {
		msgUtils.EditMessageWithKeyboard(chatID, messageID,
			formatter.FormatError("解码路径", err), "HTML", nil)
		return
	}
	path := string(pathBytes)

	// 获取重命名建议
	suggestions, err := h.deps.GetFileService().GetRenameSuggestions(ctx, path)
	if err != nil {
		msgUtils.EditMessageWithKeyboard(chatID, messageID,
			formatter.FormatError("获取重命名建议", err), "HTML", nil)
		return
	}

	// 解析并验证索引
	index := 0
	fmt.Sscanf(indexStr, "%d", &index)

	if index < 0 || index >= len(suggestions) {
		msgUtils.EditMessageWithKeyboard(chatID, messageID, "建议索引无效", "HTML", nil)
		return
	}

	// 获取选中的建议
	selected := suggestions[index]

	// 执行重命名
	if err := h.deps.GetFileService().RenameFile(ctx, path, selected.NewName); err != nil {
		msgUtils.EditMessageWithKeyboard(chatID, messageID,
			formatter.FormatError("重命名文件", err), "HTML", nil)
		return
	}

	// 构建成功消息
	message := fmt.Sprintf("<b>重命名成功</b>\n\n原名称：<code>%s</code>\n\n新名称：<code>%s</code>\n\n类型：%s\nTMDB ID：%d",
		path, selected.NewName, selected.MediaType, selected.TMDBID)

	// 添加返回按钮
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🏠 主菜单", "back_main"),
		),
	)

	msgUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
}
