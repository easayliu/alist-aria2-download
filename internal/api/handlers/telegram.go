package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/easayliu/alist-aria2-download/internal/application/services"
	"github.com/easayliu/alist-aria2-download/internal/domain/entities"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/alist"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/aria2"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/config"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/telegram"
	"github.com/easayliu/alist-aria2-download/pkg/logger"
	"github.com/gin-gonic/gin"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type TelegramHandler struct {
	telegramClient      *telegram.Client
	notificationService *services.NotificationService
	fileService         *services.FileService
	downloadService     *services.DownloadService
	schedulerService    *services.SchedulerService
	config              *config.Config
	lastUpdateID        int
	ctx                 context.Context
	cancel              context.CancelFunc
}

func NewTelegramHandler(cfg *config.Config, notificationService *services.NotificationService, fileService *services.FileService, schedulerService *services.SchedulerService) *TelegramHandler {
	var telegramClient *telegram.Client
	if cfg.Telegram.Enabled {
		telegramClient = telegram.NewClient(&cfg.Telegram)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &TelegramHandler{
		telegramClient:      telegramClient,
		notificationService: notificationService,
		fileService:         fileService,
		downloadService:     services.NewDownloadService(cfg),
		schedulerService:    schedulerService,
		config:              cfg,
		ctx:                 ctx,
		cancel:              cancel,
	}
}

func (h *TelegramHandler) Webhook(c *gin.Context) {
	if !h.config.Telegram.Enabled {
		c.JSON(http.StatusNotFound, gin.H{"error": "Telegram integration disabled"})
		return
	}

	var update tgbotapi.Update
	if err := c.ShouldBindJSON(&update); err != nil {
		logger.Error("Failed to parse telegram update:", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid update format"})
		return
	}

	if update.Message != nil {
		h.handleMessage(&update)
	} else if update.CallbackQuery != nil {
		h.handleCallbackQuery(&update)
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *TelegramHandler) StartPolling() {
	if !h.config.Telegram.Enabled || h.telegramClient == nil {
		logger.Info("Telegram polling disabled")
		return
	}

	logger.Info("Starting Telegram polling...")

	go func() {
		for {
			select {
			case <-h.ctx.Done():
				logger.Info("Telegram polling stopped")
				return
			default:
				h.pollUpdates()
			}
		}
	}()
}

func (h *TelegramHandler) StopPolling() {
	if h.cancel != nil {
		h.cancel()
	}
}

func (h *TelegramHandler) pollUpdates() {
	updates, err := h.telegramClient.GetUpdates(int64(h.lastUpdateID+1), 30)
	if err != nil {
		logger.Error("Failed to get telegram updates:", err)
		time.Sleep(5 * time.Second)
		return
	}

	for _, update := range updates {
		if update.UpdateID > h.lastUpdateID {
			h.lastUpdateID = update.UpdateID
		}

		if update.Message != nil {
			h.handleMessage(&update)
		} else if update.CallbackQuery != nil {
			h.handleCallbackQuery(&update)
		}
	}
}

func (h *TelegramHandler) handleMessage(update *tgbotapi.Update) {
	msg := update.Message
	if msg == nil || msg.Text == "" {
		return
	}

	userID := msg.From.ID
	chatID := msg.Chat.ID

	if !h.telegramClient.IsAuthorized(userID) {
		h.sendMessage(chatID, "未授权访问")
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

	// 首先处理快捷按钮（Reply Keyboard）
	switch command {
	case "系统状态":
		h.handleStatus(chatID)
		return
	case "文件列表":
		h.handleList(chatID, "/list")
		return
	case "管理面板":
		h.handleManage(chatID)
		return
	case "下载状态":
		h.handleDownloadStatusAPI(chatID)
		return
	case "定时任务":
		h.handleTasks(chatID, msg.From.ID)
		return
	case "帮助":
		h.handleHelp(chatID)
		return
	case "主菜单":
		h.handleStart(chatID)
		return
	}

	// 处理核心斜杠命令
	switch {
	case strings.HasPrefix(command, "/start"):
		h.handleStart(chatID)
	case strings.HasPrefix(command, "/help"):
		h.handleHelp(chatID)
	// 保留下载和文件管理命令（需要参数）
	case strings.HasPrefix(command, "/download"):
		h.handleDownload(chatID, command)
	case strings.HasPrefix(command, "/list"):
		h.handleList(chatID, command)
	case strings.HasPrefix(command, "/cancel"):
		h.handleCancel(chatID, command)
	// 保留定时任务命令（需要参数或用户ID）
	case strings.HasPrefix(command, "/tasks"):
		h.handleTasks(chatID, msg.From.ID)
	case strings.HasPrefix(command, "/addtask"):
		h.handleAddTask(chatID, msg.From.ID, command)
	case strings.HasPrefix(command, "/quicktask"):
		h.handleQuickTask(chatID, msg.From.ID, command)
	case strings.HasPrefix(command, "/deltask"):
		h.handleDeleteTask(chatID, msg.From.ID, command)
	case strings.HasPrefix(command, "/runtask"):
		h.handleRunTask(chatID, msg.From.ID, command)
	// 处理回复键盘的快捷按钮
	case command == "昨日文件":
		h.handleYesterdayFiles(chatID)
	case command == "下载昨日":
		h.handleYesterdayDownload(chatID)
	case command == "下载状态":
		h.handleDownloadStatusAPI(chatID)
	case command == "定时任务":
		h.handleTasks(chatID, msg.From.ID)
	default:
		h.sendMessage(chatID, "未知命令，发送 /help 查看可用命令")
	}
}

func (h *TelegramHandler) handleStart(chatID int64) {
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

	// 同时设置回复键盘
	if h.telegramClient != nil && h.telegramClient.GetBot() != nil {
		msg := tgbotapi.NewMessage(chatID, message)
		msg.ParseMode = "HTML"
		msg.ReplyMarkup = h.getDefaultReplyKeyboard()
		if _, err := h.telegramClient.GetBot().Send(msg); err != nil {
			logger.Error("Failed to send start message:", err)
		}

		// 发送内联键盘消息
		inlineMsg := tgbotapi.NewMessage(chatID, "请选择功能：")
		inlineMsg.ReplyMarkup = keyboard
		if _, err := h.telegramClient.GetBot().Send(inlineMsg); err != nil {
			logger.Error("Failed to send inline keyboard:", err)
		}
	}
}

func (h *TelegramHandler) handleHelp(chatID int64) {
	message := "<b>使用帮助</b>\n\n" +
		"<b>快捷按钮:</b>\n" +
		"使用下方键盘按钮进行常用操作\n\n" +
		"<b>文件操作命令:</b>\n" +
		"/list [path] - 列出指定路径的文件\n" +
		"/download &lt;url&gt; - 开始下载任务\n" +
		"/cancel &lt;id&gt; - 取消下载任务\n\n" +
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

	h.sendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
}

func (h *TelegramHandler) handleStatus(chatID int64) {
	status, err := h.downloadService.GetSystemStatus()
	if err != nil {
		h.sendMessage(chatID, "获取系统状态失败: "+err.Error())
		return
	}

	aria2Info := status["aria2"].(map[string]interface{})
	telegramInfo := status["telegram"].(map[string]interface{})
	serverInfo := status["server"].(map[string]interface{})

	message := fmt.Sprintf("<b>系统状态</b>\n\n"+
		"<b>Telegram Bot:</b> %s\n"+
		"<b>Aria2:</b> %s (版本: %s)\n"+
		"<b>服务器:</b> 运行中 (端口: %s, 模式: %s)",
		telegramInfo["status"],
		aria2Info["status"],
		aria2Info["version"],
		serverInfo["port"],
		serverInfo["mode"])

	h.sendMessageHTML(chatID, message)
}

func (h *TelegramHandler) handleDownload(chatID int64, command string) {
	parts := strings.Fields(command)
	if len(parts) < 2 {
		h.sendMessage(chatID, "请提供下载链接\n示例: /download https://example.com/file.zip")
		return
	}

	url := parts[1]

	// 创建下载任务
	download, err := h.downloadService.CreateDownload(url, "", "", nil)
	if err != nil {
		h.sendMessage(chatID, "创建下载任务失败: "+err.Error())
		return
	}

	// 发送确认消息
	escapedURL := h.escapeHTML(url)
	escapedID := h.escapeHTML(download.ID)
	escapedFilename := h.escapeHTML(download.Filename)
	message := fmt.Sprintf("<b>下载任务已创建</b>\n\nURL: <code>%s</code>\nGID: <code>%s</code>\n文件名: <code>%s</code>",
		escapedURL, escapedID, escapedFilename)
	h.sendMessageHTML(chatID, message)

	// 发送通知
	h.notificationService.NotifyDownloadStarted(download)
}

func (h *TelegramHandler) handleList(chatID int64, command string) {
	parts := strings.Fields(command)

	// 使用配置中的默认路径，如果用户没有提供路径
	path := h.config.Alist.DefaultPath
	if path == "" {
		path = "/"
	}

	if len(parts) > 1 {
		path = strings.Join(parts[1:], " ")
	}

	// 获取文件列表
	files, err := h.fileService.ListFilesSimple(path, 1, 20)
	if err != nil {
		h.sendMessage(chatID, fmt.Sprintf("获取文件列表失败: %v", err))
		return
	}

	// 构建消息
	escapedPath := h.escapeHTML(path)
	message := fmt.Sprintf("<b>目录: %s</b>\n\n", escapedPath)

	// 统计
	videoCount := 0
	dirCount := 0
	otherCount := 0

	// 列出文件
	for _, file := range files {
		if file.IsDir {
			dirCount++
			message += fmt.Sprintf("[D] %s/\n", h.escapeHTML(file.Name))
		} else if h.fileService.IsVideoFile(file.Name) {
			videoCount++
			sizeStr := h.formatFileSize(file.Size)
			message += fmt.Sprintf("[V] %s (%s)\n", h.escapeHTML(file.Name), sizeStr)
		} else {
			otherCount++
			sizeStr := h.formatFileSize(file.Size)
			message += fmt.Sprintf("[F] %s (%s)\n", h.escapeHTML(file.Name), sizeStr)
		}

		// 限制消息长度
		if len(message) > 3500 {
			message += "\n... 更多文件未显示"
			break
		}
	}

	// 添加统计信息
	message += fmt.Sprintf("\n<b>统计:</b>\n")
	if dirCount > 0 {
		message += fmt.Sprintf("目录: %d\n", dirCount)
	}
	if videoCount > 0 {
		message += fmt.Sprintf("视频: %d\n", videoCount)
	}
	if otherCount > 0 {
		message += fmt.Sprintf("其他: %d\n", otherCount)
	}

	h.sendMessageHTML(chatID, message)
}

// formatFileSize 格式化文件大小
func (h *TelegramHandler) formatFileSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

func (h *TelegramHandler) handleCancel(chatID int64, command string) {
	parts := strings.Fields(command)
	if len(parts) < 2 {
		h.sendMessage(chatID, "请提供下载GID\n示例: /cancel abc123")
		return
	}

	gid := parts[1]

	// 取消下载任务
	if err := h.downloadService.CancelDownload(gid); err != nil {
		h.sendMessage(chatID, "取消下载失败: "+err.Error())
		return
	}

	escapedID := h.escapeHTML(gid)
	message := fmt.Sprintf("<b>下载已取消</b>\n\n下载GID: <code>%s</code>", escapedID)
	h.sendMessageHTML(chatID, message)
}

func (h *TelegramHandler) sendMessage(chatID int64, text string) {
	if h.telegramClient != nil {
		if err := h.telegramClient.SendMessage(chatID, text); err != nil {
			logger.Error("Failed to send telegram message:", err)
		}
	}
}

func (h *TelegramHandler) sendMessageMarkdown(chatID int64, text string) {
	if h.telegramClient != nil {
		if err := h.telegramClient.SendMessageWithParseMode(chatID, text, "Markdown"); err != nil {
			logger.Error("Failed to send telegram markdown message:", err)
		}
	}
}

// sendMessageWithReplyKeyboard 发送带有回复键盘的消息
func (h *TelegramHandler) sendMessageWithReplyKeyboard(chatID int64, text string) {
	if h.telegramClient != nil && h.telegramClient.GetBot() != nil {
		msg := tgbotapi.NewMessage(chatID, text)
		msg.ReplyMarkup = h.getDefaultReplyKeyboard()
		if _, err := h.telegramClient.GetBot().Send(msg); err != nil {
			logger.Error("Failed to send telegram message with reply keyboard:", err)
		}
	}
}

// getDefaultReplyKeyboard 获取默认的回复键盘
func (h *TelegramHandler) getDefaultReplyKeyboard() tgbotapi.ReplyKeyboardMarkup {
	keyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("系统状态"),
			tgbotapi.NewKeyboardButton("文件列表"),
			tgbotapi.NewKeyboardButton("管理面板"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("下载状态"),
			tgbotapi.NewKeyboardButton("定时任务"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("帮助"),
			tgbotapi.NewKeyboardButton("主菜单"),
		),
	)
	keyboard.ResizeKeyboard = true
	// keyboard.OneTimeKeyboard = false // 保持键盘常驻
	return keyboard
}

func (h *TelegramHandler) handleManage(chatID int64) {
	message := "<b>管理面板</b>\n\n请选择要执行的操作："

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("定时任务", "cmd_tasks"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("查看下载状态", "api_download_status"),
			tgbotapi.NewInlineKeyboardButtonData("连接Alist", "api_alist_login"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("系统健康检查", "api_health_check"),
		),
	)

	h.sendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
}

func (h *TelegramHandler) handleCallbackQuery(update *tgbotapi.Update) {
	callback := update.CallbackQuery
	if callback == nil {
		return
	}

	userID := callback.From.ID
	chatID := callback.Message.Chat.ID
	data := callback.Data

	// 权限验证
	if !h.telegramClient.IsAuthorized(userID) {
		h.telegramClient.AnswerCallbackQuery(callback.ID, "未授权访问")
		return
	}

	logger.Info("Received callback query:", "data", data, "from", callback.From.UserName, "chatID", chatID)

	// 先回应回调查询
	h.telegramClient.AnswerCallbackQuery(callback.ID, "")

	switch data {
	case "cmd_help":
		h.handleHelp(chatID)
	case "cmd_status":
		h.handleStatus(chatID)
	case "cmd_manage":
		h.handleManage(chatID)
	case "menu_download":
		h.handleDownloadMenu(chatID)
	case "menu_files":
		h.handleFilesMenu(chatID)
	case "menu_system":
		h.handleSystemMenu(chatID)
	case "menu_status":
		h.handleStatusMenu(chatID)
	case "show_yesterday_options", "api_yesterday_files", "api_yesterday_files_preview", "api_yesterday_download":
		// 昨日文件功能已移除，跳转到定时任务
		h.handleTasks(chatID, 0)
	case "api_download_status":
		h.handleDownloadStatusAPI(chatID)
	case "api_alist_login":
		h.handleAlistLogin(chatID)
	case "api_health_check":
		h.handleHealthCheck(chatID)
	case "back_main":
		h.handleStart(chatID)
	// 下载管理功能
	case "download_list":
		h.handleDownloadStatusAPI(chatID)
	case "download_create":
		h.handleDownloadCreate(chatID)
	case "download_control":
		h.handleDownloadControl(chatID)
	case "download_delete":
		h.handleDownloadDelete(chatID)
	// 文件浏览功能
	case "files_browse":
		h.handleFilesBrowse(chatID)
	case "files_search":
		h.handleFilesSearch(chatID)
	case "files_info":
		h.handleFilesInfo(chatID)
	case "files_download":
		h.handleFilesDownload(chatID)
	case "api_alist_files":
		h.handleAlistFiles(chatID)
	// 系统管理功能
	case "system_info":
		h.handleSystemInfo(chatID)
	// 状态监控功能
	case "status_realtime":
		h.handleStatusRealtime(chatID)
	case "status_storage":
		h.handleStatusStorage(chatID)
	case "status_history":
		h.handleStatusHistory(chatID)
	default:
		h.sendMessage(chatID, "未知操作")
	}
}

// handleAPICall 已被移除，改为直接调用服务层方法

func (h *TelegramHandler) handleYesterdayOptions(chatID int64) {
	message := "<b>昨日文件查看选项</b>\n\n请选择查看方式："

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("定时任务", "cmd_tasks"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("返回管理面板", "cmd_manage"),
		),
	)

	h.sendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
}

func (h *TelegramHandler) handleYesterdayFilesPreview(chatID int64) {
	h.sendMessage(chatID, "正在获取昨日文件预览...")

	// 构建完整的URL
	baseURL := fmt.Sprintf("http://localhost:%s", h.config.Server.Port)
	fullURL := baseURL + "/api/v1/files/yesterday"

	// 创建HTTP请求
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		h.sendMessage(chatID, fmt.Sprintf("创建请求失败: %v", err))
		return
	}

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		h.sendMessage(chatID, fmt.Sprintf("请求失败: %v", err))
		return
	}
	defer resp.Body.Close()

	// 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		h.sendMessage(chatID, fmt.Sprintf("读取响应失败: %v", err))
		return
	}

	// 解析响应数据
	var result struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Files []struct {
				Name         string `json:"name"`
				Path         string `json:"path"`
				Size         int64  `json:"size"`
				MediaType    string `json:"media_type"`
				DownloadPath string `json:"download_path"`
			} `json:"files"`
			Count        int            `json:"count"`
			TotalSize    int64          `json:"total_size"`
			InternalURLs []string       `json:"internal_urls"`
			SearchPath   string         `json:"search_path"`
			Date         string         `json:"date"`
			MediaStats   map[string]int `json:"media_stats"`
		} `json:"data"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		h.sendMessage(chatID, fmt.Sprintf("解析响应失败: %v", err))
		return
	}

	if result.Code != 0 {
		h.sendMessage(chatID, fmt.Sprintf("获取文件失败: %s", result.Message))
		return
	}

	// 格式化预览信息
	if result.Data.Count == 0 || len(result.Data.Files) == 0 {
		message := "<b>昨日文件预览</b>\n\n未找到昨日更新的文件"
		h.sendMessageHTML(chatID, message)
		return
	}

	// 使用API返回的统计数据
	movieCount := result.Data.MediaStats["movie"]
	tvCount := result.Data.MediaStats["tv"]
	otherCount := result.Data.MediaStats["other"]
	totalSize := result.Data.TotalSize

	movieFiles := []string{}
	tvFiles := []string{}

	for _, file := range result.Data.Files {
		switch file.MediaType {
		case "movie":
			if len(movieFiles) < 3 {
				movieFiles = append(movieFiles, file.Name)
			}
		case "tv":
			if len(tvFiles) < 3 {
				tvFiles = append(tvFiles, file.Name)
			}
		}
	}

	// 构建预览消息
	message := fmt.Sprintf("<b>昨日文件预览</b>\n\n")
	message += fmt.Sprintf("<b>统计信息:</b>\n")
	message += fmt.Sprintf("电影: %d 个\n", movieCount)
	message += fmt.Sprintf("剧集: %d 个\n", tvCount)
	message += fmt.Sprintf("其他: %d 个\n", otherCount)
	message += fmt.Sprintf("总大小: %.2f GB\n\n", float64(totalSize)/(1024*1024*1024))

	if len(movieFiles) > 0 {
		message += "<b>电影示例:</b>\n"
		for _, name := range movieFiles {
			escapedName := h.escapeHTML(name)
			if len(escapedName) > 40 {
				escapedName = escapedName[:40] + "..."
			}
			message += fmt.Sprintf("• %s\n", escapedName)
		}
		if movieCount > len(movieFiles) {
			message += fmt.Sprintf("• ... 还有 %d 个电影文件\n", movieCount-len(movieFiles))
		}
		message += "\n"
	}

	if len(tvFiles) > 0 {
		message += "<b>剧集示例:</b>\n"
		for _, name := range tvFiles {
			escapedName := h.escapeHTML(name)
			if len(escapedName) > 40 {
				escapedName = escapedName[:40] + "..."
			}
			message += fmt.Sprintf("• %s\n", escapedName)
		}
		if tvCount > len(tvFiles) {
			message += fmt.Sprintf("• ... 还有 %d 个剧集文件\n", tvCount-len(tvFiles))
		}
		message += "\n"
	}

	// 添加操作按钮
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("定时任务", "cmd_tasks"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("返回管理面板", "cmd_manage"),
		),
	)

	h.sendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
}

func (h *TelegramHandler) sendMessageWithKeyboard(chatID int64, text, parseMode string, keyboard *tgbotapi.InlineKeyboardMarkup) {
	if h.telegramClient != nil {
		if err := h.telegramClient.SendMessageWithKeyboard(chatID, text, parseMode, keyboard); err != nil {
			logger.Error("Failed to send telegram message with keyboard:", err)
		}
	}
}

func (h *TelegramHandler) escapeHTML(text string) string {
	// 转义HTML特殊字符
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		"\"", "&quot;",
	)
	return replacer.Replace(text)
}

func (h *TelegramHandler) sendMessageHTML(chatID int64, text string) {
	if h.telegramClient != nil {
		if err := h.telegramClient.SendMessageWithParseMode(chatID, text, "HTML"); err != nil {
			logger.Error("Failed to send telegram HTML message:", err)
		}
	}
}

// 下载管理功能处理
func (h *TelegramHandler) handleDownloadCreate(chatID int64) {
	message := "<b>创建新下载任务</b>\n\n" +
		"<b>使用方法:</b>\n" +
		"1. 直接发送文件URL\n" +
		"2. 或点击快速创建按钮\n\n" +
		"<b>支持的下载方式:</b>\n" +
		"• HTTP/HTTPS 直链下载\n" +
		"• 磁力链接下载\n" +
		"• BT种子下载\n\n" +
		"<b>请发送下载链接或选择快速操作:</b>"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("定时任务", "cmd_tasks"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("返回下载管理", "menu_download"),
		),
	)

	h.sendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
}

func (h *TelegramHandler) handleDownloadControl(chatID int64) {
	h.sendMessage(chatID, "正在获取当前下载任务...")

	// 获取下载列表并提供控制选项
	h.handleDownloadStatusAPI(chatID)

	// 提供控制选项
	message := "<b>下载控制选项</b>\n\n" +
		"<b>操作说明:</b>\n" +
		"• 使用 /cancel &lt;GID&gt; 取消下载\n" +
		"• GID 是下载任务的唯一标识符\n" +
		"• 可以从上方的状态列表中获取 GID"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("刷新状态", "download_list"),
			tgbotapi.NewInlineKeyboardButtonData("返回管理", "menu_download"),
		),
	)

	h.sendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
}

func (h *TelegramHandler) handleDownloadDelete(chatID int64) {
	message := "<b>删除下载任务</b>\n\n" +
		"<b>注意:</b> 删除操作将无法撤销\n\n" +
		"正在获取当前任务列表..."

	h.sendMessageHTML(chatID, message)

	// 获取下载列表并提供删除选项
	h.handleDownloadStatusAPI(chatID)
}

// 文件浏览功能处理
func (h *TelegramHandler) handleFilesBrowse(chatID int64) {
	message := "<b>浏览Alist目录</b>\n\n" +
		"<b>目录浏览功能:</b>\n" +
		"• 查看根目录文件列表\n" +
		"• 导航到子目录\n" +
		"• 查看文件详细信息\n\n" +
		"正在获取根目录文件列表..."

	h.sendMessageHTML(chatID, message)

	// 获取Alist根目录文件
	h.handleAlistFiles(chatID)
}

func (h *TelegramHandler) handleFilesSearch(chatID int64) {
	message := "<b>文件搜索功能</b>\n\n" +
		"<b>搜索说明:</b>\n" +
		"• 支持文件名关键词搜索\n" +
		"• 支持路径模糊匹配\n" +
		"• 支持文件类型过滤\n\n" +
		"<b>请输入搜索关键词:</b>\n" +
		"格式: /search &lt;关键词&gt;\n\n" +
		"<b>快速搜索:</b>"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("搜索电影", "search_movies"),
			tgbotapi.NewInlineKeyboardButtonData("搜索剧集", "search_tv"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("返回文件浏览", "menu_files"),
		),
	)

	h.sendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
}

func (h *TelegramHandler) handleFilesInfo(chatID int64) {
	message := "<b>文件信息查看</b>\n\n" +
		"<b>可查看信息:</b>\n" +
		"• 文件基本属性\n" +
		"• 文件大小和修改时间\n" +
		"• 下载链接和路径\n" +
		"• 媒体类型识别\n\n" +
		"<b>请选择操作方式:</b>"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("浏览选择", "files_browse"),
			tgbotapi.NewInlineKeyboardButtonData("定时任务", "cmd_tasks"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("返回文件浏览", "menu_files"),
		),
	)

	h.sendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
}

func (h *TelegramHandler) handleFilesDownload(chatID int64) {
	message := "<b>路径下载功能</b>\n\n" +
		"<b>下载选项:</b>\n" +
		"• 指定路径批量下载\n" +
		"• 递归下载子目录\n" +
		"• 预览模式（不下载）\n" +
		"• 过滤文件类型\n\n" +
		"<b>使用格式:</b>\n" +
		"<code>/path_download /movies/2024</code>\n\n" +
		"<b>快速下载:</b>"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("定时任务", "cmd_tasks"),
			tgbotapi.NewInlineKeyboardButtonData("浏览下载", "files_browse"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("返回文件浏览", "menu_files"),
		),
	)

	h.sendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
}

// 系统管理功能处理
func (h *TelegramHandler) handleSystemInfo(chatID int64) {
	message := "<b>系统信息</b>\n\n" +
		"<b>服务状态:</b>\n" +
		"• 服务器运行状态: 正常\n" +
		"• Telegram Bot: 已连接\n" +
		"• 配置加载状态: 正常\n\n" +
		"<b>版本信息:</b>\n" +
		"• 应用版本: v1.0.0\n" +
		"• Go 版本: " + runtime.Version() + "\n" +
		"• 构建时间: " + time.Now().Format("2006-01-02") + "\n\n"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("刷新信息", "system_info"),
			tgbotapi.NewInlineKeyboardButtonData("健康检查", "api_health_check"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("返回系统管理", "menu_system"),
		),
	)

	h.sendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
}

// 状态监控功能处理
func (h *TelegramHandler) handleStatusRealtime(chatID int64) {
	h.sendMessage(chatID, "正在获取实时状态数据...")

	// 获取当前下载状态
	h.handleDownloadStatusAPI(chatID)
}

func (h *TelegramHandler) handleStatusStorage(chatID int64) {
	message := "<b>存储状态监控</b>\n\n" +
		"<b>存储信息:</b>\n" +
		"• 下载目录: /downloads\n" +
		"• 可用空间: 计算中...\n" +
		"• 已用空间: 计算中...\n\n" +
		"<b>文件统计:</b>\n" +
		"• 总文件数: 获取中...\n" +
		"• 今日下载: 获取中...\n\n" +
		"详细存储信息正在计算中..."

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("刷新状态", "status_storage"),
			tgbotapi.NewInlineKeyboardButtonData("下载统计", "api_download_status"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("返回状态监控", "menu_status"),
		),
	)

	h.sendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
}

func (h *TelegramHandler) handleStatusHistory(chatID int64) {
	message := "<b>历史统计数据</b>\n\n" +
		"<b>下载历史:</b>\n" +
		"• 昨日下载任务: 查询中...\n" +
		"• 本周总下载: 查询中...\n" +
		"• 本月总下载: 查询中...\n\n" +
		"<b>文件统计:</b>\n" +
		"• 电影文件: 统计中...\n" +
		"• 电视剧集: 统计中...\n" +
		"• 其他文件: 统计中...\n\n"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("定时任务", "cmd_tasks"),
			tgbotapi.NewInlineKeyboardButtonData("当前状态", "api_download_status"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("返回状态监控", "menu_status"),
		),
	)

	h.sendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
}

// 下载管理菜单
func (h *TelegramHandler) handleDownloadMenu(chatID int64) {
	message := "<b>下载管理中心</b>\n\n" +
		"<b>可用功能:</b>\n" +
		"• 查看所有下载任务\n" +
		"• 创建新的下载任务\n" +
		"• 暂停/恢复下载\n" +
		"• 删除下载任务\n" +
		"• 昨日文件快速下载\n\n" +
		"选择操作："

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("下载列表", "download_list"),
			tgbotapi.NewInlineKeyboardButtonData("创建下载", "download_create"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("暂停/恢复", "download_control"),
			tgbotapi.NewInlineKeyboardButtonData("删除任务", "download_delete"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("定时任务", "cmd_tasks"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("返回主菜单", "back_main"),
		),
	)

	h.sendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
}

// 文件浏览菜单
func (h *TelegramHandler) handleFilesMenu(chatID int64) {
	message := "<b>文件浏览中心</b>\n\n" +
		"<b>可用功能:</b>\n" +
		"• 浏览Alist目录结构\n" +
		"• 搜索和查找文件\n" +
		"• 查看文件详细信息\n" +
		"• 从指定路径下载\n" +
		"• 批量下载操作\n\n" +
		"选择操作："

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("浏览目录", "files_browse"),
			tgbotapi.NewInlineKeyboardButtonData("搜索文件", "files_search"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("文件信息", "files_info"),
			tgbotapi.NewInlineKeyboardButtonData("路径下载", "files_download"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Alist状态", "api_alist_files"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("返回主菜单", "back_main"),
		),
	)

	h.sendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
}

// 系统管理菜单
func (h *TelegramHandler) handleSystemMenu(chatID int64) {
	message := "<b>系统管理中心</b>\n\n" +
		"<b>可用功能:</b>\n" +
		"• Alist服务登录\n" +
		"• 系统健康检查\n" +
		"• 服务状态监控\n" +
		"• 配置信息查看\n\n" +
		"选择操作："

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Alist登录", "api_alist_login"),
			tgbotapi.NewInlineKeyboardButtonData("健康检查", "api_health_check"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("服务状态", "cmd_status"),
			tgbotapi.NewInlineKeyboardButtonData("系统信息", "system_info"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("返回主菜单", "back_main"),
		),
	)

	h.sendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
}

// 状态监控菜单
func (h *TelegramHandler) handleStatusMenu(chatID int64) {
	message := "<b>状态监控中心</b>\n\n" +
		"<b>可用功能:</b>\n" +
		"• 实时下载状态\n" +
		"• 系统运行状态\n" +
		"• 存储空间监控\n" +
		"• 历史统计数据\n\n" +
		"选择操作："

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("实时状态", "status_realtime"),
			tgbotapi.NewInlineKeyboardButtonData("下载统计", "api_download_status"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(" 存储状态", "status_storage"),
			tgbotapi.NewInlineKeyboardButtonData(" 历史数据", "status_history"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("刷新状态", "cmd_status"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("返回主菜单", "back_main"),
		),
	)

	h.sendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
}

func (h *TelegramHandler) handleDownloadStatusAPI(chatID int64) {
	h.sendMessage(chatID, "正在获取下载状态...")

	downloads, err := h.downloadService.ListDownloads()
	if err != nil {
		h.sendMessage(chatID, "获取下载状态失败: "+err.Error())
		return
	}

	// 安全的类型断言，处理 []aria2.StatusResult 类型
	var activeCount, waitingCount, stoppedCount int

	// 处理active下载
	if activeVal, ok := downloads["active"]; ok {
		if active, ok := activeVal.([]aria2.StatusResult); ok {
			activeCount = len(active)
		}
	}

	// 处理waiting下载
	if waitingVal, ok := downloads["waiting"]; ok {
		if waiting, ok := waitingVal.([]aria2.StatusResult); ok {
			waitingCount = len(waiting)
		}
	}

	// 处理stopped下载
	if stoppedVal, ok := downloads["stopped"]; ok {
		if stopped, ok := stoppedVal.([]aria2.StatusResult); ok {
			stoppedCount = len(stopped)
		}
	}

	totalCount := downloads["total_count"].(int)

	message := fmt.Sprintf("<b>下载状态总览</b>\n\n"+
		"<b>统计:</b>\n"+
		"• 总任务数: %d\n"+
		"• 活动中: %d\n"+
		"• 等待中: %d\n"+
		"• 已停止: %d\n\n",
		totalCount, activeCount, waitingCount, stoppedCount)

	// 显示活动任务
	if activeVal, ok := downloads["active"]; ok {
		if active, ok := activeVal.([]aria2.StatusResult); ok && len(active) > 0 {
			message += "<b>活动任务:</b>\n"
			for i, task := range active {
				if i >= 3 { // 只显示前3个
					message += fmt.Sprintf("• ... 还有 %d 个任务\n", len(active)-3)
					break
				}

				gid := task.GID
				if len(gid) > 8 {
					gid = gid[:8] + "..."
				}

				// 获取文件名
				filename := "未知文件"
				if len(task.Files) > 0 && task.Files[0].Path != "" {
					path := task.Files[0].Path
					if idx := strings.LastIndex(path, "/"); idx != -1 {
						filename = path[idx+1:]
					} else {
						filename = path
					}
					if len(filename) > 30 {
						filename = filename[:30] + "..."
					}
				}

				message += fmt.Sprintf("• %s - %s\n", gid, h.escapeHTML(filename))
			}
			message += "\n"
		}
	}

	// 显示等待任务数量
	if waitingVal, ok := downloads["waiting"]; ok {
		if waiting, ok := waitingVal.([]aria2.StatusResult); ok && len(waiting) > 0 {
			message += fmt.Sprintf("<b>等待任务:</b> %d 个\n\n", len(waiting))
		}
	}

	// 显示停止任务数量
	if stoppedVal, ok := downloads["stopped"]; ok {
		if stopped, ok := stoppedVal.([]aria2.StatusResult); ok && len(stopped) > 0 {
			message += fmt.Sprintf("<b>已停止任务:</b> %d 个\n", len(stopped))
		}
	}

	h.sendMessageHTML(chatID, message)
}

func extractFilenameFromURL(url string) string {
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		filename := parts[len(parts)-1]
		if filename != "" {
			return filename
		}
	}
	return "unknown_file"
}

// handleTasks 处理查看定时任务
func (h *TelegramHandler) handleTasks(chatID int64, userID int64) {
	if h.schedulerService == nil {
		h.sendMessage(chatID, "定时任务服务未启用")
		return
	}

	tasks, err := h.schedulerService.GetUserTasks(userID)
	if err != nil {
		h.sendMessage(chatID, fmt.Sprintf("获取任务失败: %v", err))
		return
	}

	if len(tasks) == 0 {
		message := "<b>定时任务管理</b>\n\n" +
			"您还没有创建任何定时任务\n\n" +
			"<b>添加任务示例:</b>\n" +
			"<code>/addtask 下载昨日视频 0 2 * * * /movies 24 true</code>\n" +
			"格式: /addtask 名称 cron表达式 路径 小时数 是否只视频\n\n" +
			"<b>Cron表达式说明:</b>\n" +
			"• <code>0 2 * * *</code> - 每天凌晨2点\n" +
			"• <code>0 */6 * * *</code> - 每6小时\n" +
			"• <code>0 0 * * 1</code> - 每周一凌晨"
		h.sendMessageHTML(chatID, message)
		return
	}

	message := fmt.Sprintf("<b>您的定时任务 (%d个)</b>\n\n", len(tasks))

	for i, task := range tasks {
		status := "禁用"
		if task.Enabled {
			status = "启用"
		}

		// 计算时间描述
		timeDesc := fmt.Sprintf("%d小时", task.HoursAgo)
		if task.HoursAgo == 24 {
			timeDesc = "1天"
		} else if task.HoursAgo == 48 {
			timeDesc = "2天"
		} else if task.HoursAgo == 72 {
			timeDesc = "3天"
		} else if task.HoursAgo == 168 {
			timeDesc = "7天"
		} else if task.HoursAgo == 720 {
			timeDesc = "30天"
		}

		message += fmt.Sprintf(
			"<b>%d. %s</b> %s\n"+
				"   ID: <code>%s</code>\n"+
				"   Cron: <code>%s</code>\n"+
				"   路径: <code>%s</code>\n"+
				"   时间范围: 最近<b>%s</b>内修改的文件\n"+
				"   文件类型: %s\n",
			i+1, h.escapeHTML(task.Name), status,
			task.ID[:8], task.Cron, task.Path,
			timeDesc,
			func() string {
				if task.VideoOnly {
					return "仅视频"
				}
				return "所有文件"
			}(),
		)

		if task.LastRunAt != nil {
			message += fmt.Sprintf("   上次: %s\n", task.LastRunAt.Format("01-02 15:04"))
		}
		if task.NextRunAt != nil {
			message += fmt.Sprintf("   下次: %s\n", task.NextRunAt.Format("01-02 15:04"))
		}
		message += "\n"
	}

	message += "<b>命令:</b>\n" +
		"• 立即运行: <code>/runtask ID</code>\n" +
		"• 删除任务: <code>/deltask ID</code>\n" +
		"• 添加任务: <code>/addtask</code> 查看帮助"

	h.sendMessageHTML(chatID, message)
}

// handleAddTask 处理添加定时任务
func (h *TelegramHandler) handleAddTask(chatID int64, userID int64, command string) {
	if h.schedulerService == nil {
		h.sendMessage(chatID, "定时任务服务未启用")
		return
	}

	parts := strings.Fields(command)
	if len(parts) < 5 { // 最少需要5个参数（路径可选）
		defaultPath := h.config.Alist.DefaultPath
		if defaultPath == "" {
			defaultPath = "/"
		}
		message := "<b>添加定时下载任务</b>\n\n" +
			"<b>命令格式:</b>\n" +
			"<code>/addtask 名称 cron表达式 [路径] 小时数 是否只视频</code>\n\n" +
			"<b>参数说明:</b>\n" +
			"• <b>名称</b>: 任务的自定义名称\n" +
			"• <b>cron表达式</b>: 执行频率（需要引号）\n" +
			"• <b>路径</b>: 扫描路径（可选，默认: <code>" + defaultPath + "</code>）\n" +
			"• <b>小时数</b>: 下载最近N小时内修改的文件\n" +
			"• <b>是否只视频</b>: true(仅视频) 或 false(所有文件)\n\n" +
			"<b>详细示例:</b>\n\n" +
			"1. <code>/addtask 昨日视频 \"0 2 * * *\" 24 true</code>\n" +
			"  • 任务名: 昨日视频\n" +
			"  • 执行: 每天凌晨2:00\n" +
			"  • 扫描: 默认路径，最近24小时修改的视频\n\n" +
			"2. <code>/addtask 频繁同步 \"*/30 * * * *\" 2 true</code>\n" +
			"  • 任务名: 频繁同步\n" +
			"  • 执行: 每30分钟\n" +
			"  • 扫描: 默认路径，最近2小时修改的视频\n" +
			"  • 用途: 追踪频繁更新的内容\n\n" +
			"3. <code>/addtask 电影库 \"0 */6 * * *\" /movies 72 true</code>\n" +
			"  • 任务名: 电影库\n" +
			"  • 执行: 每6小时（0点、6点、12点、18点）\n" +
			"  • 扫描: /movies路径，最近72小时(3天)修改的视频\n\n" +
			"4. <code>/addtask 全量备份 \"0 3 * * 0\" /downloads 168 false</code>\n" +
			"  • 任务名: 全量备份\n" +
			"  • 执行: 每周日凌晨3:00\n" +
			"  • 扫描: /downloads路径，最近7天修改的所有文件\n\n" +
			"<b>时间范围说明:</b>\n" +
			"• <code>1</code> = 最近1小时\n" +
			"• <code>6</code> = 最近6小时\n" +
			"• <code>24</code> = 最近1天\n" +
			"• <code>72</code> = 最近3天\n" +
			"• <code>168</code> = 最近7天\n" +
			"• <code>720</code> = 最近30天\n\n" +
			"<b>Cron表达式说明:</b>\n" +
			"格式: <code>分 时 日 月 周</code>\n\n" +
			"<b>常用表达式:</b>\n" +
			"• <code>*/10 * * * *</code> → 每10分钟\n" +
			"• <code>*/30 * * * *</code> → 每30分钟\n" +
			"• <code>0 * * * *</code> → 每小时整点\n" +
			"• <code>0 */2 * * *</code> → 每2小时\n" +
			"• <code>0 */6 * * *</code> → 每6小时\n" +
			"• <code>0 2 * * *</code> → 每天凌晨2:00\n" +
			"• <code>30 18 * * *</code> → 每天18:30\n" +
			"• <code>0 9 * * 1</code> → 每周一9:00\n" +
			"• <code>0 0 1 * *</code> → 每月1号凌晨"
		h.sendMessageHTML(chatID, message)
		return
	}

	// 解析参数 - 需要处理cron表达式可能包含空格的情况
	name := parts[1]

	var cron, path string
	var hoursAgo int
	var videoOnly bool

	// 最后两个参数始终是 hoursAgo 和 videoOnly
	videoOnly = parts[len(parts)-1] == "true"
	hoursAgo, _ = strconv.Atoi(parts[len(parts)-2])

	// 检查倒数第三个参数是否是路径（以/开头）或是否是数字（如果是数字，说明没有提供路径）
	if len(parts) >= 6 && strings.HasPrefix(parts[len(parts)-3], "/") {
		// 有路径参数
		path = parts[len(parts)-3]
		// 中间的部分都是cron表达式
		cronParts := parts[2 : len(parts)-3]
		cron = strings.Join(cronParts, " ")
	} else {
		// 没有路径参数，使用默认路径
		path = h.config.Alist.DefaultPath
		if path == "" {
			path = "/"
		}
		// 中间的部分都是cron表达式
		cronParts := parts[2 : len(parts)-2]
		cron = strings.Join(cronParts, " ")
	}

	// 去除可能的引号
	cron = strings.Trim(cron, "\"'")

	// 创建任务
	task := &entities.ScheduledTask{
		Name:      name,
		Enabled:   true,
		Cron:      cron,
		Path:      path,
		HoursAgo:  hoursAgo,
		VideoOnly: videoOnly,
		CreatedBy: userID,
	}

	if err := h.schedulerService.CreateTask(task); err != nil {
		h.sendMessage(chatID, fmt.Sprintf("创建任务失败: %v", err))
		return
	}

	message := fmt.Sprintf(
		"<b>任务创建成功</b>\n\n"+
			"名称: %s\n"+
			"ID: <code>%s</code>\n"+
			"Cron: <code>%s</code>\n"+
			"路径: %s\n"+
			"时间范围: 最近%d小时\n"+
			"只下载视频: %v\n\n"+
			"使用 <code>/runtask %s</code> 立即运行",
		h.escapeHTML(name), task.ID[:8], cron, path, hoursAgo, videoOnly, task.ID[:8],
	)

	h.sendMessageHTML(chatID, message)
}

// handleQuickTask 处理快捷定时任务
func (h *TelegramHandler) handleQuickTask(chatID int64, userID int64, command string) {
	if h.schedulerService == nil {
		h.sendMessage(chatID, "定时任务服务未启用")
		return
	}

	parts := strings.Fields(command)
	if len(parts) < 2 {
		defaultPath := h.config.Alist.DefaultPath
		if defaultPath == "" {
			defaultPath = "/"
		}
		message := "<b>快捷定时任务</b>\n\n" +
			"<b>格式:</b>\n" +
			"<code>/quicktask 类型 [路径]</code>\n" +
			"路径可选，不填则使用默认路径: <code>" + defaultPath + "</code>\n\n" +
			"<b>可用类型:</b>\n" +
			"• <code>daily</code> - 每日下载（24小时）\n" +
			"• <code>recent</code> - 频繁同步（2小时）\n" +
			"• <code>weekly</code> - 每周汇总（7天）\n" +
			"• <code>realtime</code> - 实时同步（1小时）\n\n" +
			"<b>示例:</b>\n" +
			"<code>/quicktask daily</code>\n" +
			"  → 每天凌晨2点下载默认路径最近24小时的视频\n\n" +
			"<code>/quicktask recent /新剧</code>\n" +
			"  → 每2小时下载/新剧最近2小时的视频\n\n" +
			"<code>/quicktask weekly</code>\n" +
			"  → 每周一下载默认路径最近7天的视频\n\n" +
			"<code>/quicktask realtime /热门</code>\n" +
			"  → 每小时下载/热门最近1小时的视频"
		h.sendMessageHTML(chatID, message)
		return
	}

	taskType := parts[1]

	// 获取路径，如果没有指定则使用默认路径
	path := h.config.Alist.DefaultPath
	if path == "" {
		path = "/"
	}
	if len(parts) >= 3 {
		path = parts[2]
	}

	var task *entities.ScheduledTask

	switch taskType {
	case "daily", "每日":
		task = &entities.ScheduledTask{
			Name:      fmt.Sprintf("每日下载-%s", path),
			Enabled:   true,
			Cron:      "0 2 * * *", // 每天凌晨2点
			Path:      path,
			HoursAgo:  24,
			VideoOnly: true,
			CreatedBy: userID,
		}
	case "recent", "频繁":
		task = &entities.ScheduledTask{
			Name:      fmt.Sprintf("频繁同步-%s", path),
			Enabled:   true,
			Cron:      "0 */2 * * *", // 每2小时
			Path:      path,
			HoursAgo:  2,
			VideoOnly: true,
			CreatedBy: userID,
		}
	case "weekly", "每周":
		task = &entities.ScheduledTask{
			Name:      fmt.Sprintf("每周汇总-%s", path),
			Enabled:   true,
			Cron:      "0 9 * * 1", // 每周一早9点
			Path:      path,
			HoursAgo:  168, // 7天
			VideoOnly: true,
			CreatedBy: userID,
		}
	case "realtime", "实时":
		task = &entities.ScheduledTask{
			Name:      fmt.Sprintf("实时同步-%s", path),
			Enabled:   true,
			Cron:      "0 * * * *", // 每小时（整点）
			Path:      path,
			HoursAgo:  1,
			VideoOnly: true,
			CreatedBy: userID,
		}
	default:
		h.sendMessage(chatID, "未知的任务类型\n可用类型: daily, recent, weekly, realtime")
		return
	}

	if err := h.schedulerService.CreateTask(task); err != nil {
		h.sendMessage(chatID, fmt.Sprintf("创建任务失败: %v", err))
		return
	}

	var timeDesc string
	switch taskType {
	case "daily", "每日":
		timeDesc = "每天凌晨2点，下载最近24小时"
	case "recent", "频繁":
		timeDesc = "每2小时，下载最近2小时"
	case "weekly", "每周":
		timeDesc = "每周一早9点，下载最近7天"
	case "realtime", "实时":
		timeDesc = "每小时，下载最近1小时"
	}

	message := fmt.Sprintf(
		"<b>快捷任务创建成功</b>\n\n"+
			"名称: %s\n"+
			"路径: %s\n"+
			"时间: %s\n"+
			"ID: <code>%s</code>\n\n"+
			"使用 <code>/runtask %s</code> 立即运行\n"+
			"使用 <code>/tasks</code> 查看所有任务",
		h.escapeHTML(task.Name), path, timeDesc, task.ID[:8], task.ID[:8],
	)

	h.sendMessageHTML(chatID, message)
}

// handleDeleteTask 处理删除定时任务
func (h *TelegramHandler) handleDeleteTask(chatID int64, userID int64, command string) {
	if h.schedulerService == nil {
		h.sendMessage(chatID, "定时任务服务未启用")
		return
	}

	parts := strings.Fields(command)
	if len(parts) < 2 {
		h.sendMessage(chatID, "用法: /deltask &lt;任务ID&gt;\n示例: /deltask abc12345")
		return
	}

	taskID := parts[1]

	// 查找完整的任务ID
	tasks, _ := h.schedulerService.GetUserTasks(userID)
	var fullTaskID string
	for _, task := range tasks {
		if strings.HasPrefix(task.ID, taskID) {
			fullTaskID = task.ID
			break
		}
	}

	if fullTaskID == "" {
		h.sendMessage(chatID, "未找到任务")
		return
	}

	if err := h.schedulerService.DeleteTask(fullTaskID); err != nil {
		h.sendMessage(chatID, fmt.Sprintf("删除任务失败: %v", err))
		return
	}

	h.sendMessage(chatID, "任务已删除")
}

// handleRunTask 处理立即运行定时任务
func (h *TelegramHandler) handleRunTask(chatID int64, userID int64, command string) {
	if h.schedulerService == nil {
		h.sendMessage(chatID, "定时任务服务未启用")
		return
	}

	parts := strings.Fields(command)
	if len(parts) < 2 {
		h.sendMessage(chatID, "用法: /runtask &lt;任务ID&gt;\n示例: /runtask abc12345")
		return
	}

	taskID := parts[1]

	// 查找完整的任务ID
	tasks, _ := h.schedulerService.GetUserTasks(userID)
	var fullTaskID string
	var taskName string
	for _, task := range tasks {
		if strings.HasPrefix(task.ID, taskID) {
			fullTaskID = task.ID
			taskName = task.Name
			break
		}
	}

	if fullTaskID == "" {
		h.sendMessage(chatID, "未找到任务")
		return
	}

	if err := h.schedulerService.RunTaskNow(fullTaskID); err != nil {
		h.sendMessage(chatID, fmt.Sprintf("运行任务失败: %v", err))
		return
	}

	h.sendMessage(chatID, fmt.Sprintf("任务 '%s' 已开始运行，请稍后查看结果", taskName))
}

// handleYesterdayFiles 处理获取昨天文件
func (h *TelegramHandler) handleYesterdayFiles(chatID int64) {
	h.sendMessage(chatID, "正在获取昨天的文件...")

	// 使用配置的默认路径
	path := h.config.Alist.DefaultPath
	if path == "" {
		path = "/"
	}

	// 获取昨天的文件
	files, err := h.fileService.GetYesterdayFiles(path)
	if err != nil {
		h.sendMessage(chatID, fmt.Sprintf("获取昨天文件失败: %v", err))
		return
	}

	if len(files) == 0 {
		h.sendMessage(chatID, "昨天没有新文件")
		return
	}

	// 构建消息
	message := fmt.Sprintf("<b>昨天的文件 (%d个):</b>\n\n", len(files))

	// 统计
	var totalSize int64
	tvCount := 0
	movieCount := 0
	otherCount := 0

	for i, file := range files {
		if i < 10 { // 只显示前10个文件
			sizeStr := h.formatFileSize(file.Size)
			message += fmt.Sprintf("[%s] %s (%s)\n", file.MediaType, h.escapeHTML(file.Name), sizeStr)
		}

		totalSize += file.Size
		switch file.MediaType {
		case "tv":
			tvCount++
		case "movie":
			movieCount++
		default:
			otherCount++
		}
	}

	if len(files) > 10 {
		message += fmt.Sprintf("\n... 还有 %d 个文件未显示\n", len(files)-10)
	}

	// 添加统计信息
	message += fmt.Sprintf("\n<b>统计信息:</b>\n")
	message += fmt.Sprintf("总大小: %s\n", h.formatFileSize(totalSize))
	if tvCount > 0 {
		message += fmt.Sprintf("电视剧: %d\n", tvCount)
	}
	if movieCount > 0 {
		message += fmt.Sprintf("电影: %d\n", movieCount)
	}
	if otherCount > 0 {
		message += fmt.Sprintf("其他: %d\n", otherCount)
	}

	h.sendMessageHTML(chatID, message)
}

// handleYesterdayDownload 处理下载昨天的文件
func (h *TelegramHandler) handleYesterdayDownload(chatID int64) {
	h.sendMessage(chatID, "正在准备下载昨天的文件...")

	// 使用配置的默认路径
	path := h.config.Alist.DefaultPath
	if path == "" {
		path = "/"
	}

	// 获取昨天的文件
	files, err := h.fileService.GetYesterdayFiles(path)
	if err != nil {
		h.sendMessage(chatID, fmt.Sprintf("获取昨天文件失败: %v", err))
		return
	}

	if len(files) == 0 {
		h.sendMessage(chatID, "昨天没有新文件需要下载")
		return
	}

	// 批量添加下载任务
	successCount := 0
	failCount := 0

	for _, file := range files {
		// 创建下载任务
		_, err := h.downloadService.CreateDownload(
			file.InternalURL,
			file.Name,
			file.DownloadPath,
			nil, // 使用默认选项
		)
		if err != nil {
			failCount++
		} else {
			successCount++
		}
	}

	// 发送结果
	message := fmt.Sprintf("<b>下载任务创建完成</b>\n\n")
	message += fmt.Sprintf("成功: %d\n", successCount)
	if failCount > 0 {
		message += fmt.Sprintf("失败: %d\n", failCount)
	}
	message += fmt.Sprintf("总计: %d\n", len(files))

	h.sendMessageHTML(chatID, message)
}

// handleAlistLogin 处理Alist登录
func (h *TelegramHandler) handleAlistLogin(chatID int64) {
	h.sendMessage(chatID, "正在登录Alist...")

	// 创建Alist客户端
	alistClient := alist.NewClient(
		h.config.Alist.BaseURL,
		h.config.Alist.Username,
		h.config.Alist.Password,
	)

	// 执行登录
	err := alistClient.Login()
	if err != nil {
		h.sendMessage(chatID, fmt.Sprintf("Alist登录失败: %v", err))
		return
	}

	h.sendMessage(chatID, "Alist登录成功！")
}

// handleHealthCheck 处理健康检查
func (h *TelegramHandler) handleHealthCheck(chatID int64) {
	message := "<b>系统健康检查</b>\n\n"
	message += fmt.Sprintf("服务状态: 正常\n")
	message += fmt.Sprintf("端口: %s\n", h.config.Server.Port)
	message += fmt.Sprintf("模式: %s\n", h.config.Server.Mode)
	message += fmt.Sprintf("\nAlist配置:\n")
	message += fmt.Sprintf("地址: %s\n", h.config.Alist.BaseURL)
	message += fmt.Sprintf("默认路径: %s\n", h.config.Alist.DefaultPath)
	message += fmt.Sprintf("\nAria2配置:\n")
	message += fmt.Sprintf("RPC地址: %s\n", h.config.Aria2.RpcURL)
	message += fmt.Sprintf("下载目录: %s\n", h.config.Aria2.DownloadDir)

	// 添加系统运行信息
	message += fmt.Sprintf("\n系统信息:\n")
	message += fmt.Sprintf("运行时间: %s\n", runtime.GOOS)
	message += fmt.Sprintf("架构: %s\n", runtime.GOARCH)
	message += fmt.Sprintf("Go版本: %s\n", runtime.Version())

	h.sendMessageHTML(chatID, message)
}

// handleAlistFiles 处理获取Alist文件列表
func (h *TelegramHandler) handleAlistFiles(chatID int64) {
	h.sendMessage(chatID, "正在获取文件列表...")

	// 使用配置的默认路径
	path := h.config.Alist.DefaultPath
	if path == "" {
		path = "/"
	}

	// 获取文件列表
	files, err := h.fileService.ListFilesSimple(path, 1, 20)
	if err != nil {
		h.sendMessage(chatID, fmt.Sprintf("获取文件列表失败: %v", err))
		return
	}

	// 构建消息
	message := fmt.Sprintf("<b>文件列表 (%s):</b>\n\n", h.escapeHTML(path))

	videoCount := 0
	dirCount := 0
	otherCount := 0

	for _, file := range files {
		if file.IsDir {
			dirCount++
			message += fmt.Sprintf("[D] %s/\n", h.escapeHTML(file.Name))
		} else if h.fileService.IsVideoFile(file.Name) {
			videoCount++
			sizeStr := h.formatFileSize(file.Size)
			message += fmt.Sprintf("[V] %s (%s)\n", h.escapeHTML(file.Name), sizeStr)
		} else {
			otherCount++
			sizeStr := h.formatFileSize(file.Size)
			message += fmt.Sprintf("[F] %s (%s)\n", h.escapeHTML(file.Name), sizeStr)
		}

		if len(message) > 3500 {
			message += "\n... 更多文件未显示"
			break
		}
	}

	// 添加统计
	message += fmt.Sprintf("\n<b>统计:</b>\n")
	if dirCount > 0 {
		message += fmt.Sprintf("目录: %d\n", dirCount)
	}
	if videoCount > 0 {
		message += fmt.Sprintf("视频: %d\n", videoCount)
	}
	if otherCount > 0 {
		message += fmt.Sprintf("其他: %d\n", otherCount)
	}

	h.sendMessageHTML(chatID, message)
}
