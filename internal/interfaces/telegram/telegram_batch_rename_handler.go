package telegram

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/easayliu/alist-aria2-download/internal/application/contracts"
	"github.com/easayliu/alist-aria2-download/internal/interfaces/telegram/utils"
	"github.com/easayliu/alist-aria2-download/pkg/logger"
	"github.com/easayliu/alist-aria2-download/pkg/utils/media"
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
	message += fmt.Sprintf("找到 %d 个视频文件，正在获取重命名建议...", len(videoFiles))

	if messageID > 0 {
		h.controller.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", nil)
	}

	message = "<b>📝 批量重命名预览</b>\n\n"

	// 使用LLM批量重命名(LLM启用时纯LLM,未启用时用TMDB)
	suggestionsMap, usedLLM, err := h.controller.fileService.GetBatchRenameSuggestionsWithLLM(ctx, videoFiles)
	if usedLLM {
		message += "🤖 使用LLM智能重命名\n\n"
	} else {
		message += "🎬 使用TMDB重命名\n\n"
	}
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

	const maxDisplayItems = MaxDisplayItems
	displayCount := 0
	successCount := 0
	detailsMessage := ""

	for i, filePath := range videoFiles {
		suggestions, found := suggestionsMap[filePath]
		if !found || len(suggestions) == 0 {
			// 检查是否为特殊内容
			fileName := filepath.Base(filePath)
			isSpecial := media.IsSpecialContent(fileName)

			if isSpecial {
				logger.Info("LLM无法处理特殊内容", "filePath", filePath)
			} else {
				logger.Warn("无法获取重命名建议", "filePath", filePath)
			}

			if displayCount < maxDisplayItems {
				reason := "未找到匹配的电影/剧集"
				if isSpecial {
					reason = "特殊内容暂不支持重命名"
				}
				detailsMessage += fmt.Sprintf("%d. ⚠️ <code>%s</code>\n   %s\n\n",
					i+1,
					h.controller.messageUtils.EscapeHTML(filepath.Base(filePath)),
					reason)
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

	results := "<b>📝 批量重命名结果</b>\n\n"

	// 使用LLM批量重命名(LLM启用时纯LLM,未启用时用TMDB)
	suggestionsMap, usedLLM, err := h.controller.fileService.GetBatchRenameSuggestionsWithLLM(ctx, videoFiles)
	if usedLLM {
		results += "🤖 使用LLM智能重命名\n\n"
	} else {
		results += "🎬 使用TMDB重命名\n\n"
	}
	if err != nil {
		h.controller.messageUtils.EditMessageWithKeyboard(chatID, messageID,
			fmt.Sprintf("❌ 批量获取建议失败: %s", err.Error()), "HTML", nil)
		return
	}

	// 构建重命名任务列表
	var tasks []contracts.RenameTask
	taskIndexMap := make(map[int]int) // 记录任务索引到videoFiles索引的映射
	skippedFiles := make([]int, 0)    // 记录跳过的文件索引

	for i, filePath := range videoFiles {
		suggestions, found := suggestionsMap[filePath]
		if !found || len(suggestions) == 0 {
			skippedFiles = append(skippedFiles, i)
			continue
		}
		taskIndexMap[len(tasks)] = i
		tasks = append(tasks, contracts.RenameTask{
			OldPath: filePath,
			NewPath: suggestions[0].NewPath,
		})
	}

	// 并发执行重命名
	renameResults := h.controller.fileService.BatchRenameAndMoveFiles(ctx, tasks)

	// 处理结果
	const maxDisplayItems = MaxDisplayItems
	displayCount := 0
	successCount := 0
	failCount := len(skippedFiles) // 跳过的文件计入失败

	// 显示跳过的文件
	for _, idx := range skippedFiles {
		if displayCount < maxDisplayItems {
			filePath := videoFiles[idx]
			fileName := filepath.Base(filePath)
			isSpecial := media.IsSpecialContent(fileName)

			reason := "未找到匹配的电影/剧集"
			if isSpecial {
				reason = "特殊内容暂不支持重命名"
			}
			results += fmt.Sprintf("%d. ⚠️ <code>%s</code>\n   %s\n\n",
				idx+1,
				h.controller.messageUtils.EscapeHTML(filepath.Base(filePath)),
				reason)
			displayCount++
		}
	}

	// 显示重命名结果
	for taskIdx, result := range renameResults {
		originalIdx := taskIndexMap[taskIdx]
		if result.Success {
			successCount++
			if displayCount < maxDisplayItems {
				results += fmt.Sprintf("%d. ✅ <code>%s</code>\n   → <code>%s</code>\n\n",
					originalIdx+1,
					h.controller.messageUtils.EscapeHTML(result.OldPath),
					h.controller.messageUtils.EscapeHTML(result.NewPath))
				displayCount++
			}
		} else {
			failCount++
			if displayCount < maxDisplayItems {
				errMsg := "未知错误"
				if result.Error != nil {
					errMsg = result.Error.Error()
				}
				results += fmt.Sprintf("%d. ❌ <code>%s</code>\n   失败: %s\n\n",
					originalIdx+1,
					h.controller.messageUtils.EscapeHTML(result.OldPath),
					errMsg)
				displayCount++
			}
		}
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
