package types

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// DownloadResult download result structure
type DownloadResult struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
	URL     string `json:"url"`
	Name    string `json:"name"`
}

// DownloadResultSummary download result summary
type DownloadResultSummary struct {
	DirectoryPath  string           `json:"directory_path"`
	TotalFiles     int              `json:"total_files"`
	VideoFiles     int              `json:"video_files"`
	SuccessCount   int              `json:"success_count"`
	FailureCount   int              `json:"failure_count"`
	Results        []DownloadResult `json:"results"`
}

// MessageSender unified message sending interface
// Used for communication between command and callback handlers
type MessageSender interface {
	// Basic message sending
	SendMessage(chatID int64, text string)
	SendMessageHTML(chatID int64, text string)
	SendMessageMarkdown(chatID int64, text string)
	
	// Message sending with keyboard
	SendMessageWithKeyboard(chatID int64, text, parseMode string, keyboard *tgbotapi.InlineKeyboardMarkup)
	SendMessageWithReplyKeyboard(chatID int64, text string)
	
	// Message editing
	EditMessageWithKeyboard(chatID int64, messageID int, text, parseMode string, keyboard *tgbotapi.InlineKeyboardMarkup)
	ClearInlineKeyboard(chatID int64, messageID int)
	
	// Utility methods
	EscapeHTML(text string) string
	FormatFileSize(size int64) string
	SplitMessage(text string, maxLength int) []string
	GetDefaultReplyKeyboard() tgbotapi.ReplyKeyboardMarkup
	GetFormatter() interface{} // 返回 *MessageFormatter，避免循环导入

	// Download result formatting methods - unified format
	FormatDownloadDirectoryResult(summary DownloadResultSummary) string
	FormatDownloadSingleFileResult(fileName, filePath, downloadPath string, success bool, errorMsg string) string
}

// DownloadCommandHandler download command handler interface
type DownloadCommandHandler interface {
	HandleDownload(chatID int64, command string)
	HandleCancel(chatID int64, command string)
	HandleYesterdayFiles(chatID int64)
	HandleYesterdayDownload(chatID int64)
}