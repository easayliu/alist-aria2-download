package types

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// DownloadResult 下载结果结构
type DownloadResult struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
	URL     string `json:"url"`
	Name    string `json:"name"`
}

// DownloadResultSummary 下载结果摘要
type DownloadResultSummary struct {
	DirectoryPath  string           `json:"directory_path"`
	TotalFiles     int              `json:"total_files"`
	VideoFiles     int              `json:"video_files"`
	SuccessCount   int              `json:"success_count"`
	FailureCount   int              `json:"failure_count"`
	Results        []DownloadResult `json:"results"`
}

// MessageSender 统一的消息发送接口
// 用于各个命令和回调处理器之间的通信
type MessageSender interface {
	// 基础消息发送
	SendMessage(chatID int64, text string)
	SendMessageHTML(chatID int64, text string)
	SendMessageMarkdown(chatID int64, text string)
	
	// 带键盘的消息发送
	SendMessageWithKeyboard(chatID int64, text, parseMode string, keyboard *tgbotapi.InlineKeyboardMarkup)
	SendMessageWithReplyKeyboard(chatID int64, text string)
	
	// 消息编辑
	EditMessageWithKeyboard(chatID int64, messageID int, text, parseMode string, keyboard *tgbotapi.InlineKeyboardMarkup)
	ClearInlineKeyboard(chatID int64, messageID int)
	
	// 工具方法
	EscapeHTML(text string) string
	FormatFileSize(size int64) string
	SplitMessage(text string, maxLength int) []string
	GetDefaultReplyKeyboard() tgbotapi.ReplyKeyboardMarkup
	
	// 下载结果格式化方法 - 统一格式
	FormatDownloadDirectoryResult(summary DownloadResultSummary) string
	FormatDownloadSingleFileResult(fileName, filePath, downloadPath string, success bool, errorMsg string) string
}

// DownloadCommandHandler 下载命令处理接口
type DownloadCommandHandler interface {
	HandleDownload(chatID int64, command string)
	HandleCancel(chatID int64, command string)
	HandleYesterdayFiles(chatID int64)
	HandleYesterdayDownload(chatID int64)
}