package commands

import (
	"context"
	"fmt"
	"runtime"
	"strings"

	"github.com/easayliu/alist-aria2-download/internal/application/contracts"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/alist"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/config"
	"github.com/easayliu/alist-aria2-download/internal/interfaces/telegram/types"
	"github.com/easayliu/alist-aria2-download/internal/interfaces/telegram/utils"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// BasicCommands handles basic commands
type BasicCommands struct {
	downloadService contracts.DownloadService
	fileService     contracts.FileService
	config          *config.Config
	messageUtils    types.MessageSender
}

// NewBasicCommands creates a basic commands handler
func NewBasicCommands(downloadService contracts.DownloadService, fileService contracts.FileService, config *config.Config, messageUtils types.MessageSender) *BasicCommands {
	return &BasicCommands{
		downloadService: downloadService,
		fileService:     fileService,
		config:          config,
		messageUtils:    messageUtils,
	}
}

func (bc *BasicCommands) buildStartContent() (string, tgbotapi.InlineKeyboardMarkup) {
	message := "<b>欢迎使用 Alist-Aria2 下载管理器</b>\n\n" +
		"<b>快捷功能:</b>\n" +
		"• 浏览文件 - 浏览和下载Alist文件\n" +
		"• 下载状态 - 查看下载任务进度\n" +
		"• 定时任务 - 自动下载任务管理\n" +
		"• 系统状态 - 服务状态和健康检查\n\n" +
		"选择功能开始使用："

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📁 浏览文件", "files_browse"),
			tgbotapi.NewInlineKeyboardButtonData("📥 下载状态", "download_list"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⏰ 定时任务", "cmd_tasks"),
			tgbotapi.NewInlineKeyboardButtonData("⚙️ 系统", "system_status"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❓ 帮助", "cmd_help"),
		),
	)

	return message, keyboard
}

func (bc *BasicCommands) HandleStart(chatID int64) {
	message, keyboard := bc.buildStartContent()
	bc.messageUtils.SendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
}

func (bc *BasicCommands) HandleStartWithEdit(chatID int64, messageID int) {
	message, keyboard := bc.buildStartContent()
	bc.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
}

func (bc *BasicCommands) buildHelpContent(includeBackButton bool) (string, tgbotapi.InlineKeyboardMarkup) {
	message := "<b>使用帮助</b>\n\n" +
		"<b>快捷按钮:</b>\n" +
		"使用下方键盘按钮进行常用操作\n\n" +
		"<b>文件操作命令:</b>\n" +
		"/list [path] - 列出指定路径的文件\n" +
		"/rename &lt;path&gt; [--llm] [--strategy=xxx] - 智能重命名文件\n" +
		"/llmrename &lt;path&gt; [策略] - 使用LLM推断文件名\n" +
		"/cancel &lt;id&gt; - 取消下载任务\n\n" +
		"<b>LLM重命名说明:</b>\n" +
		"• /rename 默认使用TMDB，可添加 --llm 启用LLM\n" +
		"• /llmrename 专用LLM重命名命令\n" +
		"• 支持策略: tmdb_first, llm_first, llm_only, tmdb_only, compare\n\n" +
		"<b>下载命令（支持多种格式）:</b>\n" +
		"• <code>/download</code> - 预览最近24小时的视频文件（使用 <code>/download confirm</code> 开始下载）\n" +
		"• <code>/download 5m</code> - 预览最近5分钟的视频文件（使用 <code>/download confirm 5m</code> 下载）\n" +
		"• <code>/download 48</code> - 预览最近48小时的视频文件（使用 <code>/download confirm 48</code> 下载）\n" +
		"• <code>/download 2025-09-01 2025-09-26</code> - 预览指定日期范围的文件\n" +
		"• <code>/download confirm 2025-09-01 2025-09-26</code> - 下载指定日期范围的文件\n" +
		"• <code>/download 2025-09-01T00:00:00Z 2025-09-26T23:59:59Z</code> - 预览精确时间范围（加 <code>confirm</code> 下载）\n" +
		"• <code>/download https://example.com/file.zip</code> - 直接下载指定URL文件\n\n" +
		"<b>时间格式说明:</b>\n" +
		"• 分钟数：1m-525600m（最大一年），例如：5m, 30m, 120m\n" +
		"• 小时数：1-8760（最大一年），例如：1, 24, 168\n" +
		"• 日期格式：YYYY-MM-DD\n" +
		"• 时间格式：ISO 8601 (YYYY-MM-DDTHH:mm:ssZ)\n" +
		"• 底部按钮「预览文件」可快速选择 5/10/30 分钟或 1/3/6 小时\n\n" +
		"<b>定时任务命令:</b>\n" +
		"/tasks - 查看我的定时任务\n" +
		"/quicktask &lt;类型&gt; [路径] - 快捷创建任务\n" +
		"/addtask - 自定义任务（查看详细帮助）\n" +
		"/runtask &lt;id&gt; - 立即运行任务\n" +
		"/deltask &lt;id&gt; - 删除任务\n\n" +
		"<b>快捷任务类型:</b>\n" +
		"• <code>daily</code> - 每日下载（24小时内文件）\n" +
		"• <code>recent</code> - 频繁同步（2小时内文件）\n" +
		"• <code>weekly</code> - 每周汇总（7天内文件）\n" +
		"• <code>realtime</code> - 实时同步（1小时内文件）"

	var keyboard tgbotapi.InlineKeyboardMarkup
	if includeBackButton {
		keyboard = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("系统状态", "cmd_status"),
				tgbotapi.NewInlineKeyboardButtonData("管理面板", "cmd_manage"),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("返回主菜单", "back_main"),
			),
		)
	} else {
		keyboard = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("系统状态", "cmd_status"),
				tgbotapi.NewInlineKeyboardButtonData("管理面板", "cmd_manage"),
			),
		)
	}

	return message, keyboard
}

func (bc *BasicCommands) HandleHelp(chatID int64) {
	message, keyboard := bc.buildHelpContent(false)
	bc.messageUtils.SendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
}

func (bc *BasicCommands) HandleHelpWithEdit(chatID int64, messageID int) {
	message, keyboard := bc.buildHelpContent(true)
	bc.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
}

// HandleStatus handles status command
func (bc *BasicCommands) HandleStatus(chatID int64) {
	ctx := context.Background()
	status, err := bc.downloadService.GetSystemStatus(ctx)
	if err != nil {
		formatter := bc.messageUtils.GetFormatter().(*utils.MessageFormatter)
		bc.messageUtils.SendMessage(chatID, formatter.FormatError("获取系统状态", err))
		return
	}

	aria2Info := status["aria2"].(map[string]any)
	telegramInfo := status["telegram"].(map[string]any)
	serverInfo := status["server"].(map[string]any)

	// Use unified formatter
	formatter := bc.messageUtils.GetFormatter().(*utils.MessageFormatter)
	message := formatter.FormatSimpleSystemStatus(utils.SimpleSystemStatusData{
		TelegramStatus: telegramInfo["status"].(string),
		Aria2Status:    aria2Info["status"].(string),
		Aria2Version:   aria2Info["version"].(string),
		ServerPort:     serverInfo["port"].(string),
		ServerMode:     serverInfo["mode"].(string),
	})

	bc.messageUtils.SendMessageHTML(chatID, message)
}

// HandleList handles list command
func (bc *BasicCommands) HandleList(chatID int64, command string) {
	parts := strings.Fields(command)

	// Use default path from config if user didn't provide one
	path := bc.config.Alist.DefaultPath
	if path == "" {
		path = "/"
	}

	if len(parts) > 1 {
		path = strings.Join(parts[1:], " ")
	}

	// Get file list - using contracts interface
	req := contracts.FileListRequest{
		Path:     path,
		Page:     1,
		PageSize: 20,
	}
	ctx := context.Background()
	resp, err := bc.fileService.ListFiles(ctx, req)
	if err != nil {
		formatter := bc.messageUtils.GetFormatter().(*utils.MessageFormatter)
		bc.messageUtils.SendMessage(chatID, formatter.FormatError("获取文件列表", err))
		return
	}

	// Merge files and directories
	files := append(resp.Directories, resp.Files...)

	// Build message
	formatter := bc.messageUtils.GetFormatter().(*utils.MessageFormatter)
	escapedPath := bc.messageUtils.EscapeHTML(path)
	message := formatter.FormatTitle("📁", fmt.Sprintf("目录: %s", escapedPath)) + "\n\n"

	// Statistics
	videoCount := 0
	dirCount := 0
	otherCount := 0

	// List files
	for _, file := range files {
		if file.IsDir {
			dirCount++
			message += fmt.Sprintf("[D] %s/\n", bc.messageUtils.EscapeHTML(file.Name))
		} else if bc.fileService.IsVideoFile(file.Name) {
			videoCount++
			sizeStr := bc.fileService.FormatFileSize(file.Size)
			message += fmt.Sprintf("[V] %s (%s)\n", bc.messageUtils.EscapeHTML(file.Name), sizeStr)
		} else {
			otherCount++
			sizeStr := bc.fileService.FormatFileSize(file.Size)
			message += fmt.Sprintf("[F] %s (%s)\n", bc.messageUtils.EscapeHTML(file.Name), sizeStr)
		}

		// Limit message length
		if len(message) > 3500 {
			message += "\n... 更多文件未显示"
			break
		}
	}

	// Add statistics
	message += "\n" + formatter.FormatSection("统计") + "\n"
	if dirCount > 0 {
		message += formatter.FormatListItem("•", fmt.Sprintf("目录: %d", dirCount)) + "\n"
	}
	if videoCount > 0 {
		message += formatter.FormatListItem("•", fmt.Sprintf("视频: %d", videoCount)) + "\n"
	}
	if otherCount > 0 {
		message += formatter.FormatListItem("•", fmt.Sprintf("其他: %d", otherCount)) + "\n"
	}

	bc.messageUtils.SendMessageHTML(chatID, message)
}

// HandlePreviewMenu handles preview menu command
func (bc *BasicCommands) HandlePreviewMenu(chatID int64) {
	message := "<b>选择预览时间范围</b>\n\n" +
		"请选择要预览的时间范围：\n" +
		"• 预览 5/10/30 分钟内的文件\n" +
		"• 预览 1/3/6 小时内的文件\n\n" +
		"也可以直接输入命令：<code>/download &lt;数字&gt;</code>（小时）或 <code>/download &lt;数字&gt;m</code>（分钟）来自定义时间范围。"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("5分钟", "preview_minutes|5"),
			tgbotapi.NewInlineKeyboardButtonData("10分钟", "preview_minutes|10"),
			tgbotapi.NewInlineKeyboardButtonData("30分钟", "preview_minutes|30"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("1小时", "preview_hours|1"),
			tgbotapi.NewInlineKeyboardButtonData("3小时", "preview_hours|3"),
			tgbotapi.NewInlineKeyboardButtonData("6小时", "preview_hours|6"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("自定义时间", "preview_custom"),
			tgbotapi.NewInlineKeyboardButtonData("关闭", "preview_cancel"),
		),
	)

	bc.messageUtils.SendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
}

// HandleAlistLogin handles Alist login
func (bc *BasicCommands) HandleAlistLogin(chatID int64) {
	bc.messageUtils.SendMessage(chatID, "正在测试Alist连接...")

	// Create Alist client
	alistClient := alist.NewClient(
		bc.config.Alist.BaseURL,
		bc.config.Alist.Username,
		bc.config.Alist.Password,
	)

	// Clear existing token to force re-login
	alistClient.ClearToken()

	// Test connection and login by calling API (client will handle token refresh automatically)
	_, err := alistClient.ListFiles("/", 1, 1)
	if err != nil {
		formatter := bc.messageUtils.GetFormatter().(*utils.MessageFormatter)
		bc.messageUtils.SendMessage(chatID, formatter.FormatError("Alist连接", err))
		return
	}

	// Get token status
	hasToken, isValid, expiryTime := alistClient.GetTokenStatus()
	message := fmt.Sprintf("Alist连接成功！\n有效Token: %v\nToken有效: %v\n过期时间: %s",
		hasToken, isValid, expiryTime.Format("2006-01-02 15:04:05"))
	bc.messageUtils.SendMessage(chatID, message)
}

// HandleHealthCheck handles health check
func (bc *BasicCommands) HandleHealthCheck(chatID int64) {
	message := "<b>系统健康检查</b>\n\n"
	message += "服务状态: 正常\n"
	message += fmt.Sprintf("端口: %s\n", bc.config.Server.Port)
	message += fmt.Sprintf("模式: %s\n", bc.config.Server.Mode)
	message += "\nAlist配置:\n"
	message += fmt.Sprintf("地址: %s\n", bc.config.Alist.BaseURL)
	message += fmt.Sprintf("默认路径: %s\n", bc.config.Alist.DefaultPath)
	message += "\nAria2配置:\n"
	message += fmt.Sprintf("RPC地址: %s\n", bc.config.Aria2.RpcURL)
	message += fmt.Sprintf("下载目录: %s\n", bc.config.Aria2.DownloadDir)

	// Add system runtime information
	message += "\n系统信息:\n"
	message += fmt.Sprintf("运行时间: %s\n", runtime.GOOS)
	message += fmt.Sprintf("架构: %s\n", runtime.GOARCH)
	message += fmt.Sprintf("Go版本: %s\n", runtime.Version())

	bc.messageUtils.SendMessageHTML(chatID, message)
}
