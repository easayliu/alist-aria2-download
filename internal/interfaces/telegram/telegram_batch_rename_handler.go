package telegram

import (
	"context"
	"fmt"

	"github.com/easayliu/alist-aria2-download/internal/interfaces/telegram/utils"
	"github.com/easayliu/alist-aria2-download/pkg/logger"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (h *FileHandler) HandleFileRename(chatID int64, filePath string) {
	h.controller.basicCommands.HandleRename(chatID, fmt.Sprintf("/rename %s", filePath))
}

func (h *FileHandler) HandleBatchRename(chatID int64, dirPath string) {
	h.HandleBatchRenameWithEdit(chatID, dirPath, 0)
}

func (h *FileHandler) HandleBatchRenameWithEdit(chatID int64, dirPath string, messageID int) {
	ctx := context.Background()
	formatter := h.controller.messageUtils.GetFormatter().(*utils.MessageFormatter)

	if messageID == 0 {
		messageID = h.controller.messageUtils.SendMessageWithKeyboard(chatID, "正在扫描视频文件（最多2层）...", "", nil)
	}

	videoFiles, err := h.collectVideoFilesRecursive(dirPath, 0, 2)
	if err != nil {
		msg := formatter.FormatError("获取文件列表", err)
		if messageID > 0 {
			h.controller.messageUtils.EditMessageWithKeyboard(chatID, messageID, msg, "HTML", nil)
			h.controller.messageUtils.DeleteMessageAfterDelay(chatID, messageID, 30)
		} else {
			h.controller.messageUtils.SendMessageHTMLWithAutoDelete(chatID, msg, 30)
		}
		return
	}

	if len(videoFiles) == 0 {
		msg := "当前目录中没有视频文件"
		if messageID > 0 {
			h.controller.messageUtils.EditMessageWithKeyboard(chatID, messageID, msg, "HTML", nil)
			h.controller.messageUtils.DeleteMessageAfterDelay(chatID, messageID, 30)
		} else {
			h.controller.messageUtils.SendMessageHTMLWithAutoDelete(chatID, msg, 30)
		}
		return
	}

	limit := h.controller.config.TMDB.BatchRenameLimit
	if limit > 0 && len(videoFiles) > limit {
		msg := fmt.Sprintf("目录中有 %d 个视频文件，为避免超时，批量重命名限制为 %d 个文件。\n\n请考虑分批处理或使用单文件重命名。", len(videoFiles), limit)
		if messageID > 0 {
			h.controller.messageUtils.EditMessageWithKeyboard(chatID, messageID, msg, "HTML", nil)
			h.controller.messageUtils.DeleteMessageAfterDelay(chatID, messageID, 30)
		} else {
			h.controller.messageUtils.SendMessageHTMLWithAutoDelete(chatID, msg, 30)
		}
		return
	}

	renamePairs := make([]struct {
		OriginalPath string
		NewName      string
		Success      bool
	}, 0, len(videoFiles))

	message := "<b>📝 批量重命名预览</b>\n\n"
	message += fmt.Sprintf("找到 %d 个视频文件，正在从 TMDB 获取建议...", len(videoFiles))

	if messageID > 0 {
		h.controller.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", nil)
	}

	message = "<b>📝 批量重命名预览</b>\n\n"

	suggestionsMap, err := h.controller.fileService.GetBatchRenameSuggestions(ctx, videoFiles)
	if err != nil {
		message += fmt.Sprintf("❌ 批量获取建议失败: %s\n", h.controller.messageUtils.EscapeHTML(err.Error()))
		if messageID > 0 {
			h.controller.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", nil)
			h.controller.messageUtils.DeleteMessageAfterDelay(chatID, messageID, 30)
		} else {
			h.controller.messageUtils.SendMessageHTMLWithAutoDelete(chatID, message, 30)
		}
		return
	}

	const maxDisplayItems = 15
	displayCount := 0
	successCount := 0
	detailsMessage := ""

	for i, filePath := range videoFiles {
		suggestions, found := suggestionsMap[filePath]
		if !found || len(suggestions) == 0 {
			logger.Warn("No TMDB suggestions found", "filePath", filePath)
			if displayCount < maxDisplayItems {
				detailsMessage += fmt.Sprintf("%d. ❌ <code>%s</code>\n   未找到匹配的电影/剧集\n\n",
					i+1,
					h.controller.messageUtils.EscapeHTML(filePath))
				displayCount++
			}
			renamePairs = append(renamePairs, struct {
				OriginalPath string
				NewName      string
				Success      bool
			}{filePath, "", false})
			continue
		}

		selected := suggestions[0]
		if displayCount < maxDisplayItems {
			detailsMessage += fmt.Sprintf("%d. <code>%s</code>\n   → <code>%s</code>\n\n", i+1, h.controller.messageUtils.EscapeHTML(filePath), h.controller.messageUtils.EscapeHTML(selected.NewPath))
			displayCount++
		}

		renamePairs = append(renamePairs, struct {
			OriginalPath string
			NewName      string
			Success      bool
		}{filePath, selected.NewPath, true})
		successCount++
	}

	if successCount == 0 {
		message += "\n❌ 所有文件都无法获取重命名建议"
		if messageID > 0 {
			h.controller.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", nil)
			h.controller.messageUtils.DeleteMessageAfterDelay(chatID, messageID, 30)
		} else {
			h.controller.messageUtils.SendMessageHTMLWithAutoDelete(chatID, message, 30)
		}
		return
	}

	message += fmt.Sprintf("✅ 成功: %d/%d\n\n", successCount, len(videoFiles))
	message += detailsMessage

	if len(videoFiles) > maxDisplayItems {
		message += fmt.Sprintf("\n... 还有 %d 个文件未显示\n", len(videoFiles)-maxDisplayItems)
	}

	message += "\n是否确认批量重命名？"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ 确认重命名", fmt.Sprintf("batch_rename_confirm:%s", h.controller.common.EncodeFilePath(dirPath))),
			tgbotapi.NewInlineKeyboardButtonData("❌ 取消", "rename_cancel"),
		),
	)

	if messageID > 0 {
		h.controller.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
	} else {
		h.controller.messageUtils.SendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
	}
}

func (h *FileHandler) HandleBatchRenameConfirm(chatID int64, dirPath string, messageID int) {
	ctx := context.Background()
	formatter := h.controller.messageUtils.GetFormatter().(*utils.MessageFormatter)

	h.controller.messageUtils.EditMessageWithKeyboard(chatID, messageID, "正在执行批量重命名...", "HTML", nil)

	videoFiles, err := h.collectVideoFilesRecursive(dirPath, 0, 2)
	if err != nil {
		h.controller.messageUtils.EditMessageWithKeyboard(chatID, messageID,
			formatter.FormatError("获取文件列表", err), "HTML", nil)
		return
	}

	limit := h.controller.config.TMDB.BatchRenameLimit
	if len(videoFiles) == 0 || (limit > 0 && len(videoFiles) > limit) {
		h.controller.messageUtils.EditMessageWithKeyboard(chatID, messageID, "文件列表已变更，请重新执行批量重命名", "HTML", nil)
		return
	}

	successCount := 0
	failCount := 0
	results := "<b>📝 批量重命名结果</b>\n\n"

	suggestionsMap, err := h.controller.fileService.GetBatchRenameSuggestions(ctx, videoFiles)
	if err != nil {
		h.controller.messageUtils.EditMessageWithKeyboard(chatID, messageID,
			fmt.Sprintf("❌ 批量获取建议失败: %s", err.Error()), "HTML", nil)
		return
	}

	const maxDisplayItems = 15
	displayCount := 0

	for i, filePath := range videoFiles {
		suggestions, found := suggestionsMap[filePath]
		if !found || len(suggestions) == 0 {
			logger.Warn("No TMDB suggestions for batch rename", "filePath", filePath)
			if displayCount < maxDisplayItems {
				results += fmt.Sprintf("%d. ❌ <code>%s</code>\n   未找到匹配的电影/剧集\n\n",
					i+1,
					h.controller.messageUtils.EscapeHTML(filePath))
				displayCount++
			}
			failCount++
			continue
		}

		selected := suggestions[0]
		if err := h.controller.fileService.RenameAndMoveFile(ctx, filePath, selected.NewPath); err != nil {
			if displayCount < maxDisplayItems {
				results += fmt.Sprintf("%d. ❌ <code>%s</code>\n   失败: %s\n\n", i+1, h.controller.messageUtils.EscapeHTML(filePath), err.Error())
				displayCount++
			}
			failCount++
			continue
		}

		if displayCount < maxDisplayItems {
			results += fmt.Sprintf("%d. ✅ <code>%s</code>\n   → <code>%s</code>\n\n", i+1, h.controller.messageUtils.EscapeHTML(filePath), h.controller.messageUtils.EscapeHTML(selected.NewPath))
			displayCount++
		}
		successCount++
	}

	if len(videoFiles) > maxDisplayItems {
		results += fmt.Sprintf("\n... 还有 %d 个文件未显示\n", len(videoFiles)-maxDisplayItems)
	}

	results += fmt.Sprintf("\n<b>统计</b>\n✅ 成功: %d\n❌ 失败: %d\n📊 总计: %d", successCount, failCount, len(videoFiles))

	h.controller.messageUtils.EditMessageWithKeyboard(chatID, messageID, results, "HTML", nil)
	h.controller.messageUtils.DeleteMessageAfterDelay(chatID, messageID, 30)
}

func (h *FileHandler) collectVideoFilesRecursive(dirPath string, currentDepth, maxDepth int) ([]string, error) {
	videoFiles := []string{}

	files, err := h.listFilesSimple(dirPath, 1, 100)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		fullPath := h.buildFullPath(file, dirPath)

		if !file.IsDir {
			if h.controller.fileService.IsVideoFile(file.Name) {
				videoFiles = append(videoFiles, fullPath)
			}
		} else if currentDepth < maxDepth {
			subFiles, err := h.collectVideoFilesRecursive(fullPath, currentDepth+1, maxDepth)
			if err != nil {
				logger.Warn("Failed to collect files from subdirectory", "path", fullPath, "error", err)
				continue
			}
			videoFiles = append(videoFiles, subFiles...)
		}
	}

	return videoFiles, nil
}
