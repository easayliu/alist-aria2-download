package file

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/easayliu/alist-aria2-download/internal/application/contracts"
	"github.com/easayliu/alist-aria2-download/internal/interfaces/telegram/types"
	"github.com/easayliu/alist-aria2-download/internal/interfaces/telegram/utils"
	"github.com/easayliu/alist-aria2-download/pkg/logger"
	"github.com/easayliu/alist-aria2-download/pkg/utils/media"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// ================================
// 批量重命名功能
// ================================

// HandleBatchRename 处理批量重命名
func (h *Handler) HandleBatchRename(chatID int64, dirPath string) {
	h.HandleBatchRenameWithEdit(chatID, dirPath, 0)
}

// HandleBatchRenameWithEdit 处理批量重命名（支持消息编辑）
func (h *Handler) HandleBatchRenameWithEdit(chatID int64, dirPath string, messageID int) {
	ctx := context.Background()
	msgUtils := h.deps.GetMessageUtils()
	formatter := msgUtils.GetFormatter().(*utils.MessageFormatter)

	if messageID == 0 {
		messageID = msgUtils.SendMessageWithKeyboard(chatID, "正在扫描视频文件（最多2层）...", "", nil)
	}

	videoFiles, err := h.collectVideoFilesRecursive(dirPath, 0, 2)
	if err != nil {
		msg := formatter.FormatError("获取文件列表", err)
		if messageID > 0 {
			msgUtils.EditMessageWithKeyboard(chatID, messageID, msg, "HTML", nil)
			msgUtils.DeleteMessageAfterDelay(chatID, messageID, 30)
		} else {
			msgUtils.SendMessageHTMLWithAutoDelete(chatID, msg, 30)
		}
		return
	}

	if len(videoFiles) == 0 {
		msg := "当前目录中没有视频文件"
		if messageID > 0 {
			msgUtils.EditMessageWithKeyboard(chatID, messageID, msg, "HTML", nil)
			msgUtils.DeleteMessageAfterDelay(chatID, messageID, 30)
		} else {
			msgUtils.SendMessageHTMLWithAutoDelete(chatID, msg, 30)
		}
		return
	}

	limit := h.deps.GetConfig().TMDB.BatchRenameLimit
	if limit > 0 && len(videoFiles) > limit {
		msg := fmt.Sprintf("目录中有 %d 个视频文件，为避免超时，批量重命名限制为 %d 个文件。\n\n请考虑分批处理或使用单文件重命名。", len(videoFiles), limit)
		if messageID > 0 {
			msgUtils.EditMessageWithKeyboard(chatID, messageID, msg, "HTML", nil)
			msgUtils.DeleteMessageAfterDelay(chatID, messageID, 30)
		} else {
			msgUtils.SendMessageHTMLWithAutoDelete(chatID, msg, 30)
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
		msgUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", nil)
	}

	message = "<b>📝 批量重命名预览</b>\n\n"

	// 使用LLM批量重命名(LLM启用时纯LLM,未启用时用TMDB)
	fileService := h.deps.GetFileService()
	suggestionsMap, usedLLM, err := fileService.GetBatchRenameSuggestionsWithLLM(ctx, videoFiles)
	if usedLLM {
		message += "🤖 使用LLM智能重命名\n\n"
	} else {
		message += "🎬 使用TMDB重命名\n\n"
	}
	if err != nil {
		message += fmt.Sprintf("❌ 批量获取建议失败: %s\n", msgUtils.EscapeHTML(err.Error()))
		if messageID > 0 {
			msgUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", nil)
			msgUtils.DeleteMessageAfterDelay(chatID, messageID, 30)
		} else {
			msgUtils.SendMessageHTMLWithAutoDelete(chatID, message, 30)
		}
		return
	}

	const maxDisplayItems = types.MaxDisplayItems
	displayCount := 0
	successCount := 0
	skippedCount := 0      // 已符合标准格式的文件数
	unprocessableCount := 0 // 无法处理的文件数（特殊内容/无法识别）
	detailsMessage := ""

	for i, filePath := range videoFiles {
		suggestions, found := suggestionsMap[filePath]
		if !found || len(suggestions) == 0 {
			// 检查是否为特殊内容
			fileName := filepath.Base(filePath)
			isSpecial := media.IsSpecialContent(fileName)

			if isSpecial {
				logger.Info("LLM cannot process special content", "filePath", filePath)
			} else {
				logger.Warn("Failed to get rename suggestion", "filePath", filePath)
			}

			if displayCount < maxDisplayItems {
				reason := "未找到匹配的电影/剧集"
				if isSpecial {
					reason = "特殊内容暂不支持重命名"
				}
				detailsMessage += fmt.Sprintf("%d. ⚠️ <code>%s</code>\n   %s\n\n",
					i+1,
					msgUtils.EscapeHTML(filepath.Base(filePath)),
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

		// 处理跳过的文件
		if selected.Skipped {
			// 区分"已符合标准"和"无法处理"两种情况
			// 注：跳过原因常量定义在 file/rename_tv.go 中
			if selected.SkipReason == "已符合 Emby 标准格式" {
				// 已符合标准格式的文件，跳过不显示
				skippedCount++
				logger.Info("文件已符合标准格式，跳过显示",
					"filePath", filePath,
					"reason", selected.SkipReason)
			} else {
				// 特殊内容或无法识别的文件，显示警告
				unprocessableCount++
				logger.Info("文件无法处理",
					"filePath", filePath,
					"reason", selected.SkipReason)
				if displayCount < maxDisplayItems {
					detailsMessage += fmt.Sprintf("%d. ⚠️ <code>%s</code>\n   %s\n\n",
						i+1,
						msgUtils.EscapeHTML(filepath.Base(filePath)),
						selected.SkipReason)
					displayCount++
				}
			}
			continue
		}

		if displayCount < maxDisplayItems {
			detailsMessage += fmt.Sprintf("%d. <code>%s</code>\n   → <code>%s</code>\n\n", i+1, msgUtils.EscapeHTML(filePath), msgUtils.EscapeHTML(selected.NewPath))
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
		if skippedCount > 0 && unprocessableCount == 0 {
			message += fmt.Sprintf("\n✅ 所有 %d 个文件已符合标准格式，无需重命名", skippedCount)
		} else if skippedCount > 0 && unprocessableCount > 0 {
			message += fmt.Sprintf("\n✅ %d 个文件已符合标准格式\n⚠️ %d 个文件无法处理（特殊内容/无法识别）", skippedCount, unprocessableCount)
			message += "\n\n" + detailsMessage
		} else {
			message += "\n❌ 所有文件都无法获取重命名建议"
			if unprocessableCount > 0 {
				message += "\n\n" + detailsMessage
			}
		}
		if messageID > 0 {
			msgUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", nil)
			msgUtils.DeleteMessageAfterDelay(chatID, messageID, 30)
		} else {
			msgUtils.SendMessageHTMLWithAutoDelete(chatID, message, 30)
		}
		return
	}

	// 显示统计信息
	statsLine := fmt.Sprintf("✅ 需重命名: %d", successCount)
	if skippedCount > 0 {
		statsLine += fmt.Sprintf(" | ⏭️ 已标准化: %d", skippedCount)
	}
	if unprocessableCount > 0 {
		statsLine += fmt.Sprintf(" | ⚠️ 无法处理: %d", unprocessableCount)
	}
	statsLine += fmt.Sprintf(" | 📊 总计: %d\n\n", len(videoFiles))
	message += statsLine
	message += detailsMessage

	if len(videoFiles) > maxDisplayItems {
		message += fmt.Sprintf("\n... 还有 %d 个文件未显示\n", len(videoFiles)-maxDisplayItems)
	}

	message += "\n是否确认批量重命名？"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ 确认重命名", fmt.Sprintf("batch_rename_confirm:%s", h.deps.EncodeFilePath(dirPath))),
			tgbotapi.NewInlineKeyboardButtonData("❌ 取消", "rename_cancel"),
		),
	)

	if messageID > 0 {
		msgUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
	} else {
		msgUtils.SendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
	}
}

// HandleBatchRenameConfirm 确认执行批量重命名
func (h *Handler) HandleBatchRenameConfirm(chatID int64, dirPath string, messageID int) {
	ctx := context.Background()
	msgUtils := h.deps.GetMessageUtils()
	formatter := msgUtils.GetFormatter().(*utils.MessageFormatter)

	msgUtils.EditMessageWithKeyboard(chatID, messageID, "正在执行批量重命名...", "HTML", nil)

	videoFiles, err := h.collectVideoFilesRecursive(dirPath, 0, 2)
	if err != nil {
		msgUtils.EditMessageWithKeyboard(chatID, messageID,
			formatter.FormatError("获取文件列表", err), "HTML", nil)
		return
	}

	limit := h.deps.GetConfig().TMDB.BatchRenameLimit
	if len(videoFiles) == 0 || (limit > 0 && len(videoFiles) > limit) {
		msgUtils.EditMessageWithKeyboard(chatID, messageID, "文件列表已变更，请重新执行批量重命名", "HTML", nil)
		return
	}

	results := "<b>📝 批量重命名结果</b>\n\n"

	// 使用LLM批量重命名(LLM启用时纯LLM,未启用时用TMDB)
	fileService := h.deps.GetFileService()
	suggestionsMap, usedLLM, err := fileService.GetBatchRenameSuggestionsWithLLM(ctx, videoFiles)
	if usedLLM {
		results += "🤖 使用LLM智能重命名\n\n"
	} else {
		results += "🎬 使用TMDB重命名\n\n"
	}
	if err != nil {
		msgUtils.EditMessageWithKeyboard(chatID, messageID,
			fmt.Sprintf("❌ 批量获取建议失败: %s", err.Error()), "HTML", nil)
		return
	}

	// 构建重命名任务列表
	var tasks []contracts.RenameTask
	taskIndexMap := make(map[int]int)      // 记录任务索引到videoFiles索引的映射
	skippedFiles := make([]int, 0)         // 记录跳过的文件索引（无建议）
	alreadyStandardFiles := make([]int, 0) // 记录已符合标准的文件索引

	for i, filePath := range videoFiles {
		suggestions, found := suggestionsMap[filePath]
		if !found || len(suggestions) == 0 {
			skippedFiles = append(skippedFiles, i)
			continue
		}
		// 跳过已符合标准格式的文件
		if suggestions[0].Skipped {
			alreadyStandardFiles = append(alreadyStandardFiles, i)
			continue
		}
		taskIndexMap[len(tasks)] = i
		tasks = append(tasks, contracts.RenameTask{
			OldPath: filePath,
			NewPath: suggestions[0].NewPath,
		})
	}

	// 使用优化的批量重命名方法（智能选择移动策略）
	renameResults := fileService.BatchRenameAndMoveFilesOptimized(ctx, tasks)

	// 处理结果
	const maxDisplayItems = types.MaxDisplayItems
	displayCount := 0
	successCount := 0
	failCount := len(skippedFiles)                    // 无建议的文件计入失败
	alreadyStandardCount := len(alreadyStandardFiles) // 已符合标准的文件单独统计

	// 显示跳过的文件（无建议）
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
				msgUtils.EscapeHTML(filepath.Base(filePath)),
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
					msgUtils.EscapeHTML(result.OldPath),
					msgUtils.EscapeHTML(result.NewPath))
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
					msgUtils.EscapeHTML(result.OldPath),
					errMsg)
				displayCount++
			}
		}
	}

	if len(videoFiles) > maxDisplayItems {
		results += fmt.Sprintf("\n... 还有 %d 个文件未显示\n", len(videoFiles)-maxDisplayItems)
	}

	// 构建统计信息
	statsText := fmt.Sprintf("\n<b>统计</b>\n✅ 成功: %d", successCount)
	if alreadyStandardCount > 0 {
		statsText += fmt.Sprintf("\n⏭️ 已标准化: %d", alreadyStandardCount)
	}
	if failCount > 0 {
		statsText += fmt.Sprintf("\n❌ 失败: %d", failCount)
	}
	statsText += fmt.Sprintf("\n📊 总计: %d", len(videoFiles))
	results += statsText

	msgUtils.EditMessageWithKeyboard(chatID, messageID, results, "HTML", nil)
	msgUtils.DeleteMessageAfterDelay(chatID, messageID, 30)
}

// ================================
// 辅助方法
// ================================

// collectVideoFilesRecursive 递归收集视频文件
// dirPath: 目录路径
// currentDepth: 当前递归深度
// maxDepth: 最大递归深度
func (h *Handler) collectVideoFilesRecursive(dirPath string, currentDepth, maxDepth int) ([]string, error) {
	var videoFiles []string

	files, err := h.ListFilesSimple(dirPath, 1, 100)
	if err != nil {
		return nil, err
	}

	fileService := h.deps.GetFileService()
	for _, file := range files {
		fullPath := h.BuildFullPath(file, dirPath)

		if !file.IsDir {
			if fileService.IsVideoFile(file.Name) {
				videoFiles = append(videoFiles, fullPath)
			}
		} else if currentDepth < maxDepth {
			subFiles, err := h.collectVideoFilesRecursive(fullPath, currentDepth+1, maxDepth)
			if err != nil {
				logger.Warn("递归收集子目录失败", "path", fullPath, "error", err)
				continue
			}
			videoFiles = append(videoFiles, subFiles...)
		}
	}

	return videoFiles, nil
}
