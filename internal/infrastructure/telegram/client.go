package telegram

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/easayliu/alist-aria2-download/internal/infrastructure/config"
	"github.com/easayliu/alist-aria2-download/pkg/logger"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Client struct {
	config *config.TelegramConfig
	bot    *tgbotapi.BotAPI
}

func NewClient(cfg *config.TelegramConfig) *Client {
	bot, err := tgbotapi.NewBotAPI(cfg.BotToken)
	if err != nil {
		logger.Error("Failed to create Telegram bot", "error", err)
		return &Client{
			config: cfg,
			bot:    nil,
		}
	}

	logger.Info("Telegram bot connected successfully", "username", bot.Self.UserName)

	client := &Client{
		config: cfg,
		bot:    bot,
	}

	// 注册Bot命令菜单
	if err := client.RegisterBotCommands(); err != nil {
		logger.Error("Failed to register bot commands", "error", err)
	} else {
		logger.Info("Bot commands registered successfully")
	}

	return client
}

// GetBot 获取bot实例
func (c *Client) GetBot() *tgbotapi.BotAPI {
	return c.bot
}

func (c *Client) SendMessage(chatID int64, text string) error {
	return c.SendMessageWithParseMode(chatID, cleanUTF8(text), "")
}

func (c *Client) SendMessageWithParseMode(chatID int64, text, parseMode string) error {
	_, err := c.SendMessageWithKeyboard(chatID, cleanUTF8(text), parseMode, nil)
	return err
}

// cleanUTF8 确保文本是有效的UTF-8编码
func cleanUTF8(text string) string {
	if !utf8.ValidString(text) {
		// 替换无效的UTF-8字符
		return strings.ToValidUTF8(text, "?")
	}
	return text
}

func (c *Client) SendMessageWithKeyboard(chatID int64, text, parseMode string, keyboard *tgbotapi.InlineKeyboardMarkup) (int, error) {
	if c.bot == nil {
		return 0, fmt.Errorf("telegram bot not initialized")
	}

	cleanText := cleanUTF8(text)

	msg := tgbotapi.NewMessage(chatID, cleanText)
	if parseMode != "" {
		msg.ParseMode = parseMode
	}
	if keyboard != nil {
		msg.ReplyMarkup = keyboard
	}

	sentMsg, err := c.bot.Send(msg)
	if err != nil {
		return 0, fmt.Errorf("failed to send telegram message: %w", err)
	}

	return sentMsg.MessageID, nil
}

// SendMessageWithAutoDelete 发送消息并在指定时间后自动删除
// chatID: 目标聊天ID
// text: 消息文本
// parseMode: 解析模式(如 "HTML", "Markdown")
// deleteAfterSeconds: 多少秒后删除消息
func (c *Client) SendMessageWithAutoDelete(chatID int64, text, parseMode string, deleteAfterSeconds int) error {
	if c.bot == nil {
		return fmt.Errorf("telegram bot not initialized")
	}

	// 清理文本确保UTF-8编码有效
	cleanText := cleanUTF8(text)

	msg := tgbotapi.NewMessage(chatID, cleanText)
	if parseMode != "" {
		msg.ParseMode = parseMode
	}

	// 发送消息
	sentMsg, err := c.bot.Send(msg)
	if err != nil {
		return fmt.Errorf("failed to send telegram message: %w", err)
	}

	// 启动协程，延迟删除消息
	go c.deleteMessageAfterDelay(chatID, sentMsg.MessageID, deleteAfterSeconds)

	return nil
}

// deleteMessageAfterDelay 延迟删除消息
func (c *Client) deleteMessageAfterDelay(chatID int64, messageID int, delaySeconds int) {
	if delaySeconds <= 0 {
		return
	}

	// 等待指定时间
	time.Sleep(time.Duration(delaySeconds) * time.Second)

	// 删除消息
	deleteConfig := tgbotapi.NewDeleteMessage(chatID, messageID)
	_, err := c.bot.Request(deleteConfig)
	if err != nil {
		logger.Warn("Failed to delete message", "chatID", chatID, "messageID", messageID, "error", err)
	} else {
		logger.Debug("Message deleted successfully", "chatID", chatID, "messageID", messageID)
	}
}

func (c *Client) SendNotification(msg *NotificationMessage) error {
	if !c.config.Enabled || len(c.config.ChatIDs) == 0 {
		logger.Info("Telegram disabled or no chat IDs configured")
		return nil
	}

	text := c.formatNotification(msg)

	for _, chatID := range c.config.ChatIDs {
		if err := c.SendMessageWithParseMode(chatID, text, "Markdown"); err != nil {
			logger.Error("Failed to send notification", "chatID", chatID, "error", err)
			continue
		}
		logger.Info("Notification sent", "chatID", chatID, "type", msg.Type)
	}

	return nil
}

func (c *Client) formatNotification(msg *NotificationMessage) string {
	switch msg.Type {
	case "download_started":
		return fmt.Sprintf("🔄 *下载开始*\n\n📁 文件名: `%s`\n⏰ 开始时间: %s",
			msg.Title, msg.Timestamp.Format("2006-01-02 15:04:05"))

	case "download_completed":
		return fmt.Sprintf("✅ *下载完成*\n\n📁 文件名: `%s`\n⏰ 完成时间: %s\n%s",
			msg.Title, msg.Timestamp.Format("2006-01-02 15:04:05"), msg.Content)

	case "download_error":
		return fmt.Sprintf("❌ *下载失败*\n\n📁 文件名: `%s`\n⏰ 失败时间: %s\n🚨 错误信息: `%s`",
			msg.Title, msg.Timestamp.Format("2006-01-02 15:04:05"), msg.Content)

	case "download_progress":
		return fmt.Sprintf("📊 *下载进度*\n\n📁 文件名: `%s`\n%s\n⏰ 更新时间: %s",
			msg.Title, msg.Content, msg.Timestamp.Format("2006-01-02 15:04:05"))

	default:
		return fmt.Sprintf("*%s*\n\n%s\n\n⏰ %s",
			msg.Title, msg.Content, msg.Timestamp.Format("2006-01-02 15:04:05"))
	}
}

func (c *Client) GetUpdates(offset int64, timeout int) ([]tgbotapi.Update, error) {
	if c.bot == nil {
		return nil, fmt.Errorf("telegram bot not initialized")
	}

	updateConfig := tgbotapi.NewUpdate(int(offset))
	updateConfig.Timeout = timeout

	updates, err := c.bot.GetUpdates(updateConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to get telegram updates: %w", err)
	}

	return updates, nil
}

func (c *Client) IsAuthorized(userID int64) bool {
	if len(c.config.AdminIDs) == 0 {
		return true
	}

	for _, adminID := range c.config.AdminIDs {
		if adminID == userID {
			return true
		}
	}
	return false
}

func (c *Client) AnswerCallbackQuery(callbackQueryID string, text string) error {
	if c.bot == nil {
		return fmt.Errorf("telegram bot not initialized")
	}

	callback := tgbotapi.NewCallback(callbackQueryID, text)
	_, err := c.bot.Request(callback)
	if err != nil {
		return fmt.Errorf("failed to answer callback query: %w", err)
	}

	return nil
}

// RegisterBotCommands 注册Bot命令菜单
func (c *Client) RegisterBotCommands() error {
	if c.bot == nil {
		return fmt.Errorf("telegram bot not initialized")
	}

	commands := []tgbotapi.BotCommand{
		{
			Command:     "start",
			Description: "🏠 显示主菜单和欢迎信息",
		},
		{
			Command:     "help",
			Description: "❓ 显示帮助信息和可用命令",
		},
		{
			Command:     "status",
			Description: "📊 查看系统运行状态",
		},
		{
			Command:     "download",
			Description: "📥 开始下载文件 (用法: /download <URL>)",
		},
		{
			Command:     "list",
			Description: "📁 列出文件和目录 (用法: /list [路径])",
		},
		{
			Command:     "cancel",
			Description: "❌ 取消下载任务 (用法: /cancel <下载ID>)",
		},
		{
			Command:     "manage",
			Description: "⚡ 打开管理面板和快捷功能",
		},
	}

	setCommandsConfig := tgbotapi.NewSetMyCommands(commands...)
	_, err := c.bot.Request(setCommandsConfig)
	if err != nil {
		return fmt.Errorf("failed to set bot commands: %w", err)
	}

	return nil
}
