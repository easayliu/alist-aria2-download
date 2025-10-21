package telegram

import (
	"strings"

	"github.com/easayliu/alist-aria2-download/pkg/logger"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// MessageHandler handles Telegram messages
type MessageHandler struct {
	controller *TelegramController
}

// NewMessageHandler creates a new message handler
func NewMessageHandler(controller *TelegramController) *MessageHandler {
	return &MessageHandler{
		controller: controller,
	}
}

// HandleMessage handles messages
func (h *MessageHandler) HandleMessage(update *tgbotapi.Update) {
	msg := update.Message
	if msg == nil || msg.Text == "" {
		return
	}

	userID := msg.From.ID
	chatID := msg.Chat.ID

	// Authorization check
	if !h.controller.telegramClient.IsAuthorized(userID) {
		h.controller.messageUtils.SendMessage(chatID, "未授权访问")
		username := ""
		if msg.From.UserName != "" {
			username = msg.From.UserName
		}
		logger.Warn("Unauthorized telegram access attempt:", "userID", userID, "username", username)
		return
	}

	command := strings.TrimSpace(msg.Text)
	username := ""
	if msg.From.UserName != "" {
		username = msg.From.UserName
	}
	logger.Info("Received telegram command:", "command", command, "from", username, "chatID", chatID)

	// Handle quick buttons (Reply Keyboard)
	switch command {
	case "定时任务":
		h.controller.taskCommands.HandleTasks(chatID, msg.From.ID)
		return
	case "预览文件":
		h.controller.basicCommands.HandlePreviewMenu(chatID)
		return
	case "帮助":
		h.controller.basicCommands.HandleHelp(chatID)
		return
	case "主菜单":
		h.controller.basicCommands.HandleStart(chatID)
		return
	}

	// Handle core slash commands
	switch {
	case strings.HasPrefix(command, "/start"):
		h.controller.basicCommands.HandleStart(chatID)
	case strings.HasPrefix(command, "/help"):
		h.controller.basicCommands.HandleHelp(chatID)
	case strings.HasPrefix(command, "/download"):
		h.controller.downloadCommands.HandleDownload(chatID, command)
	case strings.HasPrefix(command, "/list"):
		h.controller.basicCommands.HandleList(chatID, command)
	case strings.HasPrefix(command, "/cancel"):
		h.controller.downloadCommands.HandleCancel(chatID, command)
	case strings.HasPrefix(command, "/tasks"):
		h.controller.taskCommands.HandleTasks(chatID, msg.From.ID)
	case strings.HasPrefix(command, "/addtask"):
		h.controller.taskCommands.HandleAddTask(chatID, msg.From.ID, command)
	case strings.HasPrefix(command, "/quicktask"):
		h.controller.taskCommands.HandleQuickTask(chatID, msg.From.ID, command)
	case strings.HasPrefix(command, "/deltask"):
		h.controller.taskCommands.HandleDeleteTask(chatID, msg.From.ID, command)
	case strings.HasPrefix(command, "/runtask"):
		h.controller.taskCommands.HandleRunTask(chatID, msg.From.ID, command)
	case command == "昨日文件":
		h.controller.downloadCommands.HandleYesterdayFiles(chatID)
	case command == "下载昨日":
		h.controller.downloadCommands.HandleYesterdayDownload(chatID)
	default:
		h.controller.messageUtils.SendMessage(chatID, "未知命令，发送 /help 查看可用命令")
	}
}