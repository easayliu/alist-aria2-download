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
	case strings.HasPrefix(command, "/llmrename"):
		h.handleLLMRenameCommand(chatID, command)
	case strings.HasPrefix(command, "/rename"):
		h.controller.basicCommands.HandleRename(chatID, command)
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
	default:
		h.controller.messageUtils.SendMessage(chatID, "未知命令，发送 /help 查看可用命令")
	}
}

// handleLLMRenameCommand 处理/llmrename命令
func (h *MessageHandler) handleLLMRenameCommand(chatID int64, command string) {
	parts := strings.Fields(command)
	if len(parts) < 2 {
		h.controller.messageUtils.SendMessageHTML(chatID,
			"<b>用法错误</b>\n\n"+
				"使用方式：<code>/llmrename &lt;文件路径&gt; [策略] [提示]</code>\n\n"+
				"示例：\n"+
				"<code>/llmrename /data/tvs/权力的游戏.S01E01.mkv</code>\n"+
				"<code>/llmrename /data/tvs/strange_name.mkv llm_only</code>\n"+
				"<code>/llmrename /data/movies/matrix.mkv compare</code>\n\n"+
				"<b>支持的策略：</b>\n"+
				"• <code>tmdb_first</code> (默认): TMDB优先，失败时使用LLM\n"+
				"• <code>llm_first</code>: LLM优先\n"+
				"• <code>llm_only</code>: 仅使用LLM\n"+
				"• <code>tmdb_only</code>: 仅使用TMDB\n"+
				"• <code>compare</code>: 同时使用两者，返回多个结果")
		return
	}

	// 解析参数
	strategy := "tmdb_first"

	if len(parts) >= 3 {
		strategy = parts[2]
	}

	// 注意：用户提示(userHint)功能保留给未来扩展
	// 当前版本暂不使用，但保留在命令语法中

	path := parts[1]

	// 调用LLM重命名处理
	h.controller.basicCommands.HandleLLMRename(chatID, path, strategy)
}
