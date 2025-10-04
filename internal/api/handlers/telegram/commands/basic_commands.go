package commands

import (
	"context"
	"fmt"
	"runtime"
	"strings"

	"github.com/easayliu/alist-aria2-download/internal/api/handlers/telegram/types"
	"github.com/easayliu/alist-aria2-download/internal/api/handlers/telegram/utils"
	"github.com/easayliu/alist-aria2-download/internal/application/contracts"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/alist"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/config"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// BasicCommands 基础命令处理器
type BasicCommands struct {
	downloadService contracts.DownloadService
	fileService     contracts.FileService
	config          *config.Config
	messageUtils    types.MessageSender
}

// NewBasicCommands 创建基础命令处理器
func NewBasicCommands(downloadService contracts.DownloadService, fileService contracts.FileService, config *config.Config, messageUtils types.MessageSender) *BasicCommands {
	return &BasicCommands{
		downloadService: downloadService,
		fileService:     fileService,
		config:          config,
		messageUtils:    messageUtils,
	}
}

// HandleStart 处理开始命令
func (bc *BasicCommands) HandleStart(chatID int64) {
	message := "<b>欢迎使用 Alist-Aria2 下载管理器</b>\n\n" +
		"<b>功能模块:</b>\n" +
		"• 下载管理 - 创建、监控、控制下载任务\n" +
		"• 文件浏览 - 浏览和搜索Alist文件\n" +
		"• 系统管理 - 登录、健康检查、设置\n" +
		"• 状态监控 - 实时状态和下载统计\n\n" +
		"选择功能模块开始使用："

	// 发送带有内联键盘的欢迎消息
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("下载管理", "menu_download"),
			tgbotapi.NewInlineKeyboardButtonData("文件浏览", "menu_files"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("系统管理", "menu_system"),
			tgbotapi.NewInlineKeyboardButtonData("状态监控", "menu_status"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("帮助说明", "cmd_help"),
		),
	)

	bc.messageUtils.SendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
}

// HandleHelp 处理帮助命令
func (bc *BasicCommands) HandleHelp(chatID int64) {
	message := "<b>使用帮助</b>\n\n" +
		"<b>快捷按钮:</b>\n" +
		"使用下方键盘按钮进行常用操作\n\n" +
		"<b>文件操作命令:</b>\n" +
		"/list [path] - 列出指定路径的文件\n" +
		"/cancel &lt;id&gt; - 取消下载任务\n\n" +
		"<b>下载命令（支持多种格式）:</b>\n" +
		"• <code>/download</code> - 预览最近24小时的视频文件（使用 <code>/download confirm</code> 开始下载）\n" +
		"• <code>/download 48</code> - 预览最近48小时的视频文件（使用 <code>/download confirm 48</code> 下载）\n" +
		"• <code>/download 2025-09-01 2025-09-26</code> - 预览指定日期范围的文件\n" +
		"• <code>/download confirm 2025-09-01 2025-09-26</code> - 下载指定日期范围的文件\n" +
		"• <code>/download 2025-09-01T00:00:00Z 2025-09-26T23:59:59Z</code> - 预览精确时间范围（加 <code>confirm</code> 下载）\n" +
		"• <code>/download https://example.com/file.zip</code> - 直接下载指定URL文件\n\n" +
		"<b>时间格式说明:</b>\n" +
		"• 小时数：1-8760（最大一年）\n" +
		"• 日期格式：YYYY-MM-DD\n" +
		"• 时间格式：ISO 8601 (YYYY-MM-DDTHH:mm:ssZ)\n" +
		"• 底部按钮「预览文件」可快速选择 1/3/6 小时\n\n" +
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

	// 创建快捷操作键盘
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("系统状态", "cmd_status"),
			tgbotapi.NewInlineKeyboardButtonData("管理面板", "cmd_manage"),
		),
	)

	bc.messageUtils.SendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
}

// HandleStatus 处理状态命令
func (bc *BasicCommands) HandleStatus(chatID int64) {
	ctx := context.Background()
	status, err := bc.downloadService.GetSystemStatus(ctx)
	if err != nil {
		bc.messageUtils.SendMessage(chatID, "获取系统状态失败: "+err.Error())
		return
	}

	aria2Info := status["aria2"].(map[string]interface{})
	telegramInfo := status["telegram"].(map[string]interface{})
	serverInfo := status["server"].(map[string]interface{})

	// 使用统一格式化器
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

// HandleList 处理列表命令
func (bc *BasicCommands) HandleList(chatID int64, command string) {
	parts := strings.Fields(command)

	// 使用配置中的默认路径，如果用户没有提供路径
	path := bc.config.Alist.DefaultPath
	if path == "" {
		path = "/"
	}

	if len(parts) > 1 {
		path = strings.Join(parts[1:], " ")
	}

	// 获取文件列表 - 使用contracts接口
	req := contracts.FileListRequest{
		Path:     path,
		Page:     1,
		PageSize: 20,
	}
	ctx := context.Background()
	resp, err := bc.fileService.ListFiles(ctx, req)
	if err != nil {
		bc.messageUtils.SendMessage(chatID, fmt.Sprintf("获取文件列表失败: %v", err))
		return
	}
	
	// 合并文件和目录
	files := append(resp.Directories, resp.Files...)

	// 构建消息
	formatter := bc.messageUtils.GetFormatter().(*utils.MessageFormatter)
	escapedPath := bc.messageUtils.EscapeHTML(path)
	message := formatter.FormatTitle("📁", fmt.Sprintf("目录: %s", escapedPath)) + "\n\n"

	// 统计
	videoCount := 0
	dirCount := 0
	otherCount := 0

	// 列出文件
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

		// 限制消息长度
		if len(message) > 3500 {
			message += "\n... 更多文件未显示"
			break
		}
	}

	// 添加统计信息
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

// HandlePreviewMenu 处理预览菜单命令
func (bc *BasicCommands) HandlePreviewMenu(chatID int64) {
	message := "<b>选择预览时间范围</b>\n\n" +
		"请选择要预览的时间范围：\n" +
		"• 预览 1 小时内的文件\n" +
		"• 预览 3 小时内的文件\n" +
		"• 预览 6 小时内的文件\n\n" +
		"也可以直接输入命令：<code>/download &lt;小时数&gt;</code> 或 <code>/download YYYY-MM-DD YYYY-MM-DD</code> 来自定义时间范围。"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("预览 1 小时", "preview_hours|1"),
			tgbotapi.NewInlineKeyboardButtonData("预览 3 小时", "preview_hours|3"),
			tgbotapi.NewInlineKeyboardButtonData("预览 6 小时", "preview_hours|6"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("自定义时间", "preview_custom"),
			tgbotapi.NewInlineKeyboardButtonData("关闭", "preview_cancel"),
		),
	)

	bc.messageUtils.SendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
}

// HandleAlistLogin 处理Alist登录
func (bc *BasicCommands) HandleAlistLogin(chatID int64) {
	bc.messageUtils.SendMessage(chatID, "正在测试Alist连接...")

	// 创建Alist客户端
	alistClient := alist.NewClient(
		bc.config.Alist.BaseURL,
		bc.config.Alist.Username,
		bc.config.Alist.Password,
	)

	// 清除现有token强制重新登录
	alistClient.ClearToken()

	// 通过调用API测试连接和登录（客户端会自动处理token刷新）
	_, err := alistClient.ListFiles("/", 1, 1)
	if err != nil {
		bc.messageUtils.SendMessage(chatID, fmt.Sprintf("Alist连接失败: %v", err))
		return
	}

	// 获取token状态
	hasToken, isValid, expiryTime := alistClient.GetTokenStatus()
	message := fmt.Sprintf("Alist连接成功！\n有效Token: %v\nToken有效: %v\n过期时间: %s", 
		hasToken, isValid, expiryTime.Format("2006-01-02 15:04:05"))
	bc.messageUtils.SendMessage(chatID, message)
}

// HandleHealthCheck 处理健康检查
func (bc *BasicCommands) HandleHealthCheck(chatID int64) {
	message := "<b>系统健康检查</b>\n\n"
	message += fmt.Sprintf("服务状态: 正常\n")
	message += fmt.Sprintf("端口: %s\n", bc.config.Server.Port)
	message += fmt.Sprintf("模式: %s\n", bc.config.Server.Mode)
	message += fmt.Sprintf("\nAlist配置:\n")
	message += fmt.Sprintf("地址: %s\n", bc.config.Alist.BaseURL)
	message += fmt.Sprintf("默认路径: %s\n", bc.config.Alist.DefaultPath)
	message += fmt.Sprintf("\nAria2配置:\n")
	message += fmt.Sprintf("RPC地址: %s\n", bc.config.Aria2.RpcURL)
	message += fmt.Sprintf("下载目录: %s\n", bc.config.Aria2.DownloadDir)

	// 添加系统运行信息
	message += fmt.Sprintf("\n系统信息:\n")
	message += fmt.Sprintf("运行时间: %s\n", runtime.GOOS)
	message += fmt.Sprintf("架构: %s\n", runtime.GOARCH)
	message += fmt.Sprintf("Go版本: %s\n", runtime.Version())

	bc.messageUtils.SendMessageHTML(chatID, message)
}