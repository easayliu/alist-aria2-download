// Package types defines shared types, interfaces, and constants for the telegram package.
// It provides common abstractions used across telegram sub-packages.
package types

import (
	"errors"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Sentinel errors for telegram package
var (
	// ErrUnauthorized indicates the user is not authorized to perform the action
	ErrUnauthorized = errors.New("unauthorized access")

	// ErrInvalidPath indicates an invalid file or directory path
	ErrInvalidPath = errors.New("invalid path")

	// ErrFileNotFound indicates the requested file was not found
	ErrFileNotFound = errors.New("file not found")

	// ErrOperationCancelled indicates the operation was cancelled by user
	ErrOperationCancelled = errors.New("operation cancelled")

	// ErrInvalidCallback indicates an invalid callback data format
	ErrInvalidCallback = errors.New("invalid callback data")
)

// Display and UI constants (shared across packages)
const (
	// MaxDisplayItems 批量操作时最多显示的项目数
	MaxDisplayItems = 15

	// MaxSuggestions 单文件重命名时最多显示的建议数
	MaxSuggestions = 5

	// HighConfidence 高置信度阈值（用于显示星级）
	HighConfidence = 0.9

	// MediumConfidence 中等置信度阈值（用于显示星级）
	MediumConfidence = 0.7

	// MessageAutoDeleteSeconds 消息自动删除时间（秒）
	MessageAutoDeleteSeconds = 30
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
	DirectoryPath string           `json:"directory_path"`
	TotalFiles    int              `json:"total_files"`
	VideoFiles    int              `json:"video_files"`
	SuccessCount  int              `json:"success_count"`
	FailureCount  int              `json:"failure_count"`
	Results       []DownloadResult `json:"results"`
}

// MessageSender unified message sending interface
// Used for communication between command and callback handlers
type MessageSender interface {
	// Basic message sending
	SendMessage(chatID int64, text string)
	SendMessageHTML(chatID int64, text string)
	SendMessageMarkdown(chatID int64, text string)

	// Message sending with auto deletion
	SendMessageWithAutoDelete(chatID int64, text string, deleteAfterSeconds int)
	SendMessageHTMLWithAutoDelete(chatID int64, text string, deleteAfterSeconds int)

	// Message sending with keyboard
	SendMessageWithKeyboard(chatID int64, text, parseMode string, keyboard *tgbotapi.InlineKeyboardMarkup) int
	SendMessageWithReplyKeyboard(chatID int64, text string)

	// Message editing
	EditMessageWithKeyboard(chatID int64, messageID int, text, parseMode string, keyboard *tgbotapi.InlineKeyboardMarkup) bool
	ClearInlineKeyboard(chatID int64, messageID int)

	// Message deletion
	DeleteMessage(chatID int64, messageID int)
	DeleteMessageAfterDelay(chatID int64, messageID int, delaySeconds int)

	// Utility methods
	EscapeHTML(text string) string
	FormatFileSize(size int64) string
	SplitMessage(text string, maxLength int) []string
	GetDefaultReplyKeyboard() tgbotapi.ReplyKeyboardMarkup
	GetFormatter() interface{}

	// Download result formatting methods - unified format
	FormatDownloadDirectoryResult(summary DownloadResultSummary) string
	FormatDownloadSingleFileResult(fileName, filePath, downloadPath string, success bool, errorMsg string) string
}

// DownloadCommandHandler download command handler interface
type DownloadCommandHandler interface {
	HandleDownload(chatID int64, command string)
	HandleCancel(chatID int64, command string)
}
