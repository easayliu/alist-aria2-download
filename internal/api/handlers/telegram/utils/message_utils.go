package utils

import (
	"fmt"
	"strings"

	"github.com/easayliu/alist-aria2-download/internal/api/handlers/telegram/types"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/telegram"
	"github.com/easayliu/alist-aria2-download/pkg/logger"
	pkgutils "github.com/easayliu/alist-aria2-download/pkg/utils"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// MessageUtils 消息处理工具类
type MessageUtils struct {
	telegramClient *telegram.Client
	formatter      *MessageFormatter
}

// NewMessageUtils 创建消息工具实例
func NewMessageUtils(telegramClient *telegram.Client) *MessageUtils {
	return &MessageUtils{
		telegramClient: telegramClient,
		formatter:      NewMessageFormatter(),
	}
}

// GetFormatter 获取消息格式化器 - 返回interface{}避免循环导入
func (mu *MessageUtils) GetFormatter() interface{} {
	return mu.formatter
}

// SendMessage 发送基础消息
func (mu *MessageUtils) SendMessage(chatID int64, text string) {
	if mu.telegramClient != nil {
		messages := mu.SplitMessage(text, 4000) // 留一些余量
		for _, msg := range messages {
			if err := mu.telegramClient.SendMessage(chatID, msg); err != nil {
				logger.Error("Failed to send telegram message:", err)
			}
		}
	}
}

// SendMessageHTML 发送HTML格式消息
func (mu *MessageUtils) SendMessageHTML(chatID int64, text string) {
	if mu.telegramClient != nil {
		messages := mu.SplitMessage(text, 4000) // 留一些余量
		for _, msg := range messages {
			if err := mu.telegramClient.SendMessageWithParseMode(chatID, msg, "HTML"); err != nil {
				logger.Error("Failed to send telegram HTML message:", err)
			}
		}
	}
}

// SendMessageMarkdown 发送Markdown格式消息
func (mu *MessageUtils) SendMessageMarkdown(chatID int64, text string) {
	if mu.telegramClient != nil {
		if err := mu.telegramClient.SendMessageWithParseMode(chatID, text, "Markdown"); err != nil {
			logger.Error("Failed to send telegram markdown message:", err)
		}
	}
}

// SendMessageWithKeyboard 发送带有内联键盘的消息
func (mu *MessageUtils) SendMessageWithKeyboard(chatID int64, text, parseMode string, keyboard *tgbotapi.InlineKeyboardMarkup) {
	if mu.telegramClient != nil {
		messages := mu.SplitMessage(text, 4000) // 留一些余量
		for i, msg := range messages {
			// 只在最后一条消息上附加键盘
			var kb *tgbotapi.InlineKeyboardMarkup
			if i == len(messages)-1 {
				kb = keyboard
			}
			if err := mu.telegramClient.SendMessageWithKeyboard(chatID, msg, parseMode, kb); err != nil {
				logger.Error("Failed to send telegram message with keyboard:", err)
			}
		}
	}
}

// SendMessageWithReplyKeyboard 发送带有回复键盘的消息
func (mu *MessageUtils) SendMessageWithReplyKeyboard(chatID int64, text string) {
	if mu.telegramClient != nil && mu.telegramClient.GetBot() != nil {
		msg := tgbotapi.NewMessage(chatID, text)
		msg.ReplyMarkup = mu.GetDefaultReplyKeyboard()
		if _, err := mu.telegramClient.GetBot().Send(msg); err != nil {
			logger.Error("Failed to send telegram message with reply keyboard:", err)
		}
	}
}

// EditMessageWithKeyboard 编辑消息并设置键盘
func (mu *MessageUtils) EditMessageWithKeyboard(chatID int64, messageID int, text, parseMode string, keyboard *tgbotapi.InlineKeyboardMarkup) {
	if mu.telegramClient != nil && mu.telegramClient.GetBot() != nil {
		// 编辑消息文本
		editMsg := tgbotapi.NewEditMessageText(chatID, messageID, text)
		editMsg.ParseMode = parseMode
		if _, err := mu.telegramClient.GetBot().Send(editMsg); err != nil {
			logger.Error("Failed to edit telegram message text:", err)
			return
		}
		
		// 编辑消息键盘
		if keyboard != nil {
			editKeyboard := tgbotapi.NewEditMessageReplyMarkup(chatID, messageID, *keyboard)
			if _, err := mu.telegramClient.GetBot().Send(editKeyboard); err != nil {
				logger.Error("Failed to edit telegram message keyboard:", err)
			}
		}
	}
}

// ClearInlineKeyboard 清除内联键盘
func (mu *MessageUtils) ClearInlineKeyboard(chatID int64, messageID int) {
	if mu.telegramClient == nil || mu.telegramClient.GetBot() == nil {
		return
	}

	empty := tgbotapi.InlineKeyboardMarkup{InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{}}
	edit := tgbotapi.NewEditMessageReplyMarkup(chatID, messageID, empty)
	if _, err := mu.telegramClient.GetBot().Send(edit); err != nil {
		logger.Warn("Failed to clear inline keyboard:", err)
	}
}

// SplitMessage 将长消息按指定长度分割成多个消息
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

// EscapeHTML 转义HTML特殊字符
// 使用统一的工具函数
func (mu *MessageUtils) EscapeHTML(text string) string {
	return pkgutils.EscapeHTML(text)
}

// FormatFileSize 格式化文件大小
// 使用统一的工具函数
func (mu *MessageUtils) FormatFileSize(size int64) string {
	return pkgutils.FormatFileSize(size)
}

// GetDefaultReplyKeyboard 获取默认的回复键盘
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

// ========== 下载结果格式化 ==========

// FormatDownloadDirectoryResult 格式化目录下载结果消息 - 统一格式
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

// FormatDownloadSingleFileResult 格式化单文件下载结果消息 - 统一格式
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

// DirectoryDownloadResultData 目录下载结果数据
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

// FormatDirectoryDownloadResult 格式化目录下载结果消息（与/download命令保持一致）
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