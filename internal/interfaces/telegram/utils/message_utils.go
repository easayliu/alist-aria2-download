package utils

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/easayliu/alist-aria2-download/internal/infrastructure/telegram"
	"github.com/easayliu/alist-aria2-download/internal/interfaces/telegram/types"
	"github.com/easayliu/alist-aria2-download/pkg/logger"
	"github.com/easayliu/alist-aria2-download/pkg/utils/string"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// MessageUtils message processing utility
type MessageUtils struct {
	telegramClient *telegram.Client
	formatter      *MessageFormatter
}

// NewMessageUtils creates message utility instance
func NewMessageUtils(telegramClient *telegram.Client) *MessageUtils {
	return &MessageUtils{
		telegramClient: telegramClient,
		formatter:      NewMessageFormatter(),
	}
}

// GetFormatter gets message formatter - returns interface{} to avoid circular import
func (mu *MessageUtils) GetFormatter() interface{} {
	return mu.formatter
}

// SendMessage sends basic message
func (mu *MessageUtils) SendMessage(chatID int64, text string) {
	if mu.telegramClient != nil {
		messages := mu.SplitMessage(text, 4000) // 留一些余量
		for _, msg := range messages {
			if err := mu.telegramClient.SendMessage(chatID, msg); err != nil {
				logger.Error("Failed to send telegram message", "error", err)
			}
		}
	}
}

// SendMessageWithAutoDelete sends basic message with auto deletion
func (mu *MessageUtils) SendMessageWithAutoDelete(chatID int64, text string, deleteAfterSeconds int) {
	if mu.telegramClient != nil {
		messages := mu.SplitMessage(text, 4000) // 留一些余量
		for _, msg := range messages {
			if err := mu.telegramClient.SendMessageWithAutoDelete(chatID, msg, "", deleteAfterSeconds); err != nil {
				logger.Error("Failed to send telegram message with auto delete", "error", err)
			}
		}
	}
}

// SendMessageHTML sends HTML formatted message
func (mu *MessageUtils) SendMessageHTML(chatID int64, text string) {
	if mu.telegramClient != nil {
		messages := mu.SplitMessage(text, 4000) // 留一些余量
		for _, msg := range messages {
			if err := mu.telegramClient.SendMessageWithParseMode(chatID, msg, "HTML"); err != nil {
				logger.Error("Failed to send telegram HTML message", "error", err)
			}
		}
	}
}

// SendMessageHTMLWithAutoDelete sends HTML formatted message with auto deletion
func (mu *MessageUtils) SendMessageHTMLWithAutoDelete(chatID int64, text string, deleteAfterSeconds int) {
	if mu.telegramClient != nil {
		messages := mu.SplitMessage(text, 4000) // 留一些余量
		for _, msg := range messages {
			if err := mu.telegramClient.SendMessageWithAutoDelete(chatID, msg, "HTML", deleteAfterSeconds); err != nil {
				logger.Error("Failed to send telegram HTML message with auto delete", "error", err)
			}
		}
	}
}

// SendMessageMarkdown sends Markdown formatted message
func (mu *MessageUtils) SendMessageMarkdown(chatID int64, text string) {
	if mu.telegramClient != nil {
		if err := mu.telegramClient.SendMessageWithParseMode(chatID, text, "Markdown"); err != nil {
			logger.Error("Failed to send telegram markdown message", "error", err)
		}
	}
}

// SendMessageWithKeyboard sends message with inline keyboard
func (mu *MessageUtils) SendMessageWithKeyboard(chatID int64, text, parseMode string, keyboard *tgbotapi.InlineKeyboardMarkup) int {
	if mu.telegramClient != nil {
		messages := mu.SplitMessage(text, 4000)
		var lastMessageID int
		for i, msg := range messages {
			var kb *tgbotapi.InlineKeyboardMarkup
			if i == len(messages)-1 {
				kb = keyboard
			}
			if msgID, err := mu.telegramClient.SendMessageWithKeyboard(chatID, msg, parseMode, kb); err != nil {
				logger.Error("Failed to send telegram message with keyboard", "error", err)
			} else {
				lastMessageID = msgID
			}
		}
		return lastMessageID
	}
	return 0
}

// SendMessageWithReplyKeyboard sends message with reply keyboard
func (mu *MessageUtils) SendMessageWithReplyKeyboard(chatID int64, text string) {
	if mu.telegramClient != nil && mu.telegramClient.GetBot() != nil {
		msg := tgbotapi.NewMessage(chatID, text)
		msg.ReplyMarkup = mu.GetDefaultReplyKeyboard()
		if _, err := mu.telegramClient.GetBot().Send(msg); err != nil {
			logger.Error("Failed to send telegram message with reply keyboard", "error", err)
		}
	}
}

// EditMessageWithKeyboard edits message and sets keyboard
// 使用单次API调用同时更新文本和键盘，避免闪动
func (mu *MessageUtils) EditMessageWithKeyboard(chatID int64, messageID int, text, parseMode string, keyboard *tgbotapi.InlineKeyboardMarkup) bool {
	if mu.telegramClient == nil || mu.telegramClient.GetBot() == nil {
		return false
	}

	const maxLength = 4000
	if len(text) > maxLength {
		lastNewline := maxLength - 50
		cutPos := maxLength - 30

		for i := maxLength - 1; i >= lastNewline && i >= 0; i-- {
			if text[i] == '\n' {
				cutPos = i
				break
			}
		}

		text = text[:cutPos]

		if parseMode == "HTML" {
			openTags := []string{}
			for i := 0; i < len(text); i++ {
				if text[i] == '<' {
					endTag := strings.Index(text[i:], ">")
					if endTag > 0 {
						tag := text[i+1 : i+endTag]
						if strings.HasPrefix(tag, "/") {
							tagName := tag[1:]
							if len(openTags) > 0 && openTags[len(openTags)-1] == tagName {
								openTags = openTags[:len(openTags)-1]
							}
						} else {
							tagName := strings.Split(tag, " ")[0]
							if tagName != "br" {
								openTags = append(openTags, tagName)
							}
						}
					}
				}
			}

			for i := len(openTags) - 1; i >= 0; i-- {
				text += "</" + openTags[i] + ">"
			}
		}

		text += "\n\n... (内容过长已截断)"
	}

	if !utf8.ValidString(text) {
		text = strings.ToValidUTF8(text, "?")
	}

	// 单次API调用同时更新文本和键盘，避免闪动
	editMsg := tgbotapi.NewEditMessageText(chatID, messageID, text)
	editMsg.ParseMode = parseMode
	if keyboard != nil {
		editMsg.ReplyMarkup = keyboard
	}

	if _, err := mu.telegramClient.GetBot().Send(editMsg); err != nil {
		logger.Error("Failed to edit telegram message", "error", err)
		return false
	}

	return true
}

// ClearInlineKeyboard clears inline keyboard
func (mu *MessageUtils) ClearInlineKeyboard(chatID int64, messageID int) {
	if mu.telegramClient == nil || mu.telegramClient.GetBot() == nil {
		return
	}

	empty := tgbotapi.InlineKeyboardMarkup{InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{}}
	edit := tgbotapi.NewEditMessageReplyMarkup(chatID, messageID, empty)
	if _, err := mu.telegramClient.GetBot().Send(edit); err != nil {
		logger.Warn("Failed to clear inline keyboard", "error", err)
	}
}

// DeleteMessage deletes a message immediately
func (mu *MessageUtils) DeleteMessage(chatID int64, messageID int) {
	if mu.telegramClient == nil || mu.telegramClient.GetBot() == nil {
		return
	}

	deleteConfig := tgbotapi.NewDeleteMessage(chatID, messageID)
	if _, err := mu.telegramClient.GetBot().Request(deleteConfig); err != nil {
		logger.Warn("Failed to delete message", "chatID", chatID, "messageID", messageID, "error", err)
	} else {
		logger.Debug("Message deleted successfully", "chatID", chatID, "messageID", messageID)
	}
}

// DeleteMessageAfterDelay deletes message after specified seconds
func (mu *MessageUtils) DeleteMessageAfterDelay(chatID int64, messageID int, delaySeconds int) {
	if mu.telegramClient == nil || mu.telegramClient.GetBot() == nil || delaySeconds <= 0 {
		return
	}

	go func() {
		time.Sleep(time.Duration(delaySeconds) * time.Second)
		deleteConfig := tgbotapi.NewDeleteMessage(chatID, messageID)
		if _, err := mu.telegramClient.GetBot().Request(deleteConfig); err != nil {
			logger.Warn("Failed to delete message", "chatID", chatID, "messageID", messageID, "error", err)
		} else {
			logger.Debug("Message deleted successfully", "chatID", chatID, "messageID", messageID)
		}
	}()
}

// SplitMessage splits long messages into multiple messages by specified length
func (mu *MessageUtils) SplitMessage(text string, maxLength int) []string {
	if len(text) <= maxLength {
		return []string{text}
	}

	var messages []string
	runes := []rune(text)

	for len(runes) > 0 {
		end := maxLength
		if end > len(runes) {
			end = len(runes)
		}

		// 尝试在换行符处分割
		if end < len(runes) {
			for i := end - 1; i >= maxLength*3/4; i-- { // 在后1/4处查找换行符
				if runes[i] == '\n' {
					end = i + 1
					break
				}
			}
		}

		messages = append(messages, string(runes[:end]))
		runes = runes[end:]
	}

	return messages
}

// EscapeHTML escapes HTML special characters
// 使用统一的工具函数
func (mu *MessageUtils) EscapeHTML(text string) string {
	return strutil.EscapeHTML(text)
}

// FormatFileSize formats file size
// 使用统一的工具函数
func (mu *MessageUtils) FormatFileSize(size int64) string {
	return strutil.FormatFileSize(size)
}

// GetDefaultReplyKeyboard gets default reply keyboard
func (mu *MessageUtils) GetDefaultReplyKeyboard() tgbotapi.ReplyKeyboardMarkup {
	keyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("定时任务"),
			tgbotapi.NewKeyboardButton("预览文件"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("帮助"),
			tgbotapi.NewKeyboardButton("主菜单"),
		),
	)
	keyboard.ResizeKeyboard = true
	return keyboard
}

// ========== Download result formatting ==========

// FormatDownloadDirectoryResult formats directory download result message - unified format
func (mu *MessageUtils) FormatDownloadDirectoryResult(summary types.DownloadResultSummary) string {
	// 基础结果消息 - 使用标准格式
	resultMessage := fmt.Sprintf(
		"📊 <b>目录下载任务创建完成</b>\\n\\n"+
			"<b>目录:</b> <code>%s</code>\\n"+
			"<b>扫描文件:</b> %d 个\\n"+
			"<b>视频文件:</b> %d 个\\n"+
			"<b>成功创建:</b> %d 个任务\\n"+
			"<b>失败:</b> %d 个任务\\n\\n",
		mu.EscapeHTML(summary.DirectoryPath),
		summary.TotalFiles,
		summary.VideoFiles,
		summary.SuccessCount,
		summary.FailureCount)

	// 添加失败文件详情（最多显示3个）
	if summary.FailureCount > 0 {
		failedFiles := make([]types.DownloadResult, 0)
		for _, result := range summary.Results {
			if !result.Success {
				failedFiles = append(failedFiles, result)
			}
		}

		if len(failedFiles) <= 3 {
			resultMessage += "<b>失败的文件:</b>\\n"
			for _, result := range failedFiles {
				fileName := result.Name
				if fileName == "" && result.URL != "" {
					// 从URL提取文件名
					parts := strings.Split(result.URL, "/")
					if len(parts) > 0 {
						fileName = parts[len(parts)-1]
					}
				}
				resultMessage += fmt.Sprintf("• <code>%s</code>: %s\\n",
					mu.EscapeHTML(fileName),
					result.Error)
			}
		} else {
			resultMessage += fmt.Sprintf("<b>有 %d 个文件下载失败</b>\\n", summary.FailureCount)
		}
	}

	// 添加成功提示
	if summary.SuccessCount > 0 {
		resultMessage += "\\n✅ 所有任务已使用自动路径分类功能\\n📥 可通过「下载管理」查看任务状态"
	}

	return resultMessage
}

// FormatDownloadSingleFileResult formats single file download result message - unified format
func (mu *MessageUtils) FormatDownloadSingleFileResult(fileName, filePath, downloadPath string, success bool, errorMsg string) string {
	if success {
		return fmt.Sprintf(
			"✅ <b>文件下载任务已创建</b>\\n\\n"+
				"<b>文件:</b> <code>%s</code>\\n"+
				"<b>路径:</b> <code>%s</code>\\n"+
				"<b>下载路径:</b> <code>%s</code>\\n\\n"+
				"📥 可通过「下载管理」查看任务状态",
			mu.EscapeHTML(fileName),
			mu.EscapeHTML(filePath),
			mu.EscapeHTML(downloadPath))
	} else {
		formatter := mu.GetFormatter().(*MessageFormatter)
		return formatter.FormatSimpleError(fmt.Sprintf("创建下载任务失败: %s", errorMsg))
	}
}

// DirectoryDownloadResultData directory download result data
type DirectoryDownloadResultData struct {
	DirectoryPath string
	TotalFiles    int
	VideoFiles    int
	TotalSizeStr  string
	MovieCount    int
	TVCount       int
	OtherCount    int
	SuccessCount  int
	FailedCount   int
	FailedFiles   []string
}

// FormatDirectoryDownloadResult formats directory download result message (consistent with /download command)
func (mu *MessageUtils) FormatDirectoryDownloadResult(data DirectoryDownloadResultData) string {
	// 使用统一格式化器
	batchData := BatchResultData{
		Title:        "目录下载任务已创建",
		TotalFiles:   data.TotalFiles,
		VideoFiles:   data.VideoFiles,
		SuccessCount: data.SuccessCount,
		FailureCount: data.FailedCount,
		MovieCount:   data.MovieCount,
		TVCount:      data.TVCount,
		OtherCount:   data.OtherCount,
		TotalSize:    data.TotalSizeStr,
	}

	message := mu.formatter.FormatBatchResult(batchData)

	// 添加目录信息
	dirInfo := fmt.Sprintf("\n\n<b>目录:</b> <code>%s</code>", mu.EscapeHTML(data.DirectoryPath))
	// 在标题后插入目录信息
	lines := strings.Split(message, "\n")
	if len(lines) > 2 {
		lines = append(lines[:2], append([]string{dirInfo}, lines[2:]...)...)
		message = strings.Join(lines, "\n")
	}

	return message
}
