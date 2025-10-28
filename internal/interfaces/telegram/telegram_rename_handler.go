package telegram

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/easayliu/alist-aria2-download/internal/interfaces/telegram/utils"
)

func (h *CallbackHandler) handleRenameApply(chatID int64, callbackData string, messageID int) {
	ctx := context.Background()
	formatter := h.controller.messageUtils.GetFormatter().(*utils.MessageFormatter)

	parts := strings.Split(callbackData, "|")
	if len(parts) < 3 {
		h.controller.messageUtils.EditMessageWithKeyboard(chatID, messageID, "回调数据格式错误", "HTML", nil)
		return
	}

	indexStr := parts[1]
	encodedPath := parts[2]

	pathBytes, err := base64.URLEncoding.DecodeString(encodedPath)
	if err != nil {
		h.controller.messageUtils.EditMessageWithKeyboard(chatID, messageID,
			formatter.FormatError("解码路径", err), "HTML", nil)
		return
	}
	path := string(pathBytes)

	suggestions, err := h.controller.fileService.GetRenameSuggestions(ctx, path)
	if err != nil {
		h.controller.messageUtils.EditMessageWithKeyboard(chatID, messageID,
			formatter.FormatError("获取重命名建议", err), "HTML", nil)
		return
	}

	index := 0
	fmt.Sscanf(indexStr, "%d", &index)

	if index < 0 || index >= len(suggestions) {
		h.controller.messageUtils.EditMessageWithKeyboard(chatID, messageID, "建议索引无效", "HTML", nil)
		return
	}

	selected := suggestions[index]

	if err := h.controller.fileService.RenameFile(ctx, path, selected.NewName); err != nil {
		h.controller.messageUtils.EditMessageWithKeyboard(chatID, messageID,
			formatter.FormatError("重命名文件", err), "HTML", nil)
		return
	}

	message := fmt.Sprintf("<b>重命名成功</b>\n\n原名称：<code>%s</code>\n\n新名称：<code>%s</code>\n\n类型：%s\nTMDB ID：%d",
		path, selected.NewName, selected.MediaType, selected.TMDBID)

	h.controller.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", nil)
}
