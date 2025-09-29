package telegram

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/easayliu/alist-aria2-download/internal/api/handlers/telegram/callbacks"
	"github.com/easayliu/alist-aria2-download/internal/api/handlers/telegram/commands"
	"github.com/easayliu/alist-aria2-download/internal/api/handlers/telegram/types"
	"github.com/easayliu/alist-aria2-download/internal/api/handlers/telegram/utils"
	"github.com/easayliu/alist-aria2-download/internal/application/contracts"
	"github.com/easayliu/alist-aria2-download/internal/application/services"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/alist"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/config"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/telegram"
	"github.com/easayliu/alist-aria2-download/pkg/logger"
	timeutils "github.com/easayliu/alist-aria2-download/pkg/utils"
	"github.com/gin-gonic/gin"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// TelegramHandler 重构后的 Telegram 处理器
// 保持与旧版本完全相同的公共接口，确保兼容性
type TelegramHandler struct {
	// 核心依赖 - 使用contracts接口实现API First架构
	telegramClient      *telegram.Client
	notificationService *services.NotificationService
	fileService         contracts.FileService      // 使用契约接口
	downloadService     contracts.DownloadService  // 使用契约接口
	schedulerService    *services.SchedulerService
	container           *services.ServiceContainer  // 服务容器
	config              *config.Config

	// 状态管理 - 与旧版本兼容
	lastUpdateID int
	ctx          context.Context
	cancel       context.CancelFunc

	// 手动下载上下文管理 - 与旧版本兼容
	manualMutex    sync.Mutex
	manualContexts map[string]*ManualDownloadContext

	// 路径缓存相关 - 与旧版本兼容
	pathMutex        sync.RWMutex
	pathCache        map[string]string // token -> path
	pathReverseCache map[string]string // path -> token
	pathTokenCounter int

	// 重构后的模块化组件
	messageUtils     *utils.MessageUtils
	basicCommands    *commands.BasicCommands
	downloadCommands types.DownloadCommandHandler
	taskCommands     *commands.TaskCommands
	menuCallbacks    *callbacks.MenuCallbacks
}

// ManualDownloadContext 手动下载上下文（兼容旧版本）
type ManualDownloadContext struct {
	ChatID      int64
	Request     manualDownloadRequest
	Description string
	TimeArgs    []string
	CreatedAt   time.Time
}

// manualDownloadRequest 手动下载请求（兼容旧版本）
type manualDownloadRequest struct {
	Path      string `json:"path"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
	VideoOnly bool   `json:"video_only"`
	Preview   bool   `json:"preview"`
}

// TimeParseResult 时间解析结果
type TimeParseResult struct {
	StartTime   time.Time
	EndTime     time.Time
	Description string
}


// NewTelegramHandler 创建新的 Telegram 处理器
// 使用API First架构，通过ServiceContainer获取契约接口
func NewTelegramHandler(cfg *config.Config, notificationService *services.NotificationService, fileService *services.FileService, schedulerService *services.SchedulerService) *TelegramHandler {
	var telegramClient *telegram.Client
	if cfg.Telegram.Enabled {
		telegramClient = telegram.NewClient(&cfg.Telegram)
	}

	ctx, cancel := context.WithCancel(context.Background())

	// 创建服务容器
	container, err := services.NewServiceContainer(cfg)
	if err != nil {
		logger.Error("Failed to create service container:", err)
		panic("Service container initialization failed")
	}

	// 创建主处理器实例
	handler := &TelegramHandler{
		telegramClient:      telegramClient,
		notificationService: notificationService,
		// 使用容器获取契约接口，稍后在initializeModules中设置
		schedulerService:    schedulerService,
		container:           container,
		config:              cfg,
		ctx:                 ctx,
		cancel:              cancel,
		manualContexts:      make(map[string]*ManualDownloadContext),
		pathCache:           make(map[string]string),
		pathReverseCache:    make(map[string]string),
		pathTokenCounter:    1,
	}

	// 初始化模块化组件
	handler.initializeModules()

	return handler
}

// initializeModules 初始化所有模块化组件
func (h *TelegramHandler) initializeModules() {
	// 创建消息工具
	h.messageUtils = utils.NewMessageUtils(h.telegramClient)

	// 从服务容器获取契约接口，实现API First架构
	h.fileService = h.container.GetFileService()
	h.downloadService = h.container.GetDownloadService()

	// 使用契约接口初始化基础命令模块
	h.basicCommands = commands.NewBasicCommands(h.downloadService, h.fileService, h.config, h.messageUtils)
	h.downloadCommands = commands.NewDownloadCommands(h.container, h.messageUtils)
	h.taskCommands = commands.NewTaskCommands(h.schedulerService, h.config, h.messageUtils)

	// 创建回调处理器
	h.menuCallbacks = callbacks.NewMenuCallbacks(h.downloadService, h.config, h.messageUtils)
}

// ================================
// 公共接口实现 - 与旧版本完全兼容
// ================================

// Webhook 处理 Webhook 请求（与旧版本完全兼容）
func (h *TelegramHandler) Webhook(c *gin.Context) {
	if !h.config.Telegram.Enabled {
		c.JSON(200, gin.H{"error": "Telegram integration disabled"})
		return
	}

	var update tgbotapi.Update
	if err := c.ShouldBindJSON(&update); err != nil {
		logger.Error("Failed to parse telegram update:", err)
		c.JSON(400, gin.H{"error": "Invalid update format"})
		return
	}

	if update.Message != nil {
		h.handleMessage(&update)
	} else if update.CallbackQuery != nil {
		h.handleCallbackQuery(&update)
	}

	c.JSON(200, gin.H{"ok": true})
}

// StartPolling 开始轮询（与旧版本完全兼容）
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

// StopPolling 停止轮询（与旧版本完全兼容）
func (h *TelegramHandler) StopPolling() {
	if h.cancel != nil {
		h.cancel()
	}
}

// ================================
// 消息处理 - 使用模块化组件
// ================================

// pollUpdates 轮询更新
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

// handleMessage 处理消息
func (h *TelegramHandler) handleMessage(update *tgbotapi.Update) {
	msg := update.Message
	if msg == nil || msg.Text == "" {
		return
	}

	userID := msg.From.ID
	chatID := msg.Chat.ID

	// 权限验证
	if !h.telegramClient.IsAuthorized(userID) {
		h.messageUtils.SendMessage(chatID, "未授权访问")
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

	// 处理快捷按钮（Reply Keyboard）
	switch command {
	case "定时任务":
		h.taskCommands.HandleTasks(chatID, msg.From.ID)
		return
	case "预览文件":
		h.basicCommands.HandlePreviewMenu(chatID)
		return
	case "帮助":
		h.basicCommands.HandleHelp(chatID)
		return
	case "主菜单":
		h.basicCommands.HandleStart(chatID)
		return
	}

	// 处理核心斜杠命令
	switch {
	case strings.HasPrefix(command, "/start"):
		h.basicCommands.HandleStart(chatID)
	case strings.HasPrefix(command, "/help"):
		h.basicCommands.HandleHelp(chatID)
	case strings.HasPrefix(command, "/download"):
		h.downloadCommands.HandleDownload(chatID, command)
	case strings.HasPrefix(command, "/list"):
		h.basicCommands.HandleList(chatID, command)
	case strings.HasPrefix(command, "/cancel"):
		h.downloadCommands.HandleCancel(chatID, command)
	case strings.HasPrefix(command, "/tasks"):
		h.taskCommands.HandleTasks(chatID, msg.From.ID)
	case strings.HasPrefix(command, "/addtask"):
		h.taskCommands.HandleAddTask(chatID, msg.From.ID, command)
	case strings.HasPrefix(command, "/quicktask"):
		h.taskCommands.HandleQuickTask(chatID, msg.From.ID, command)
	case strings.HasPrefix(command, "/deltask"):
		h.taskCommands.HandleDeleteTask(chatID, msg.From.ID, command)
	case strings.HasPrefix(command, "/runtask"):
		h.taskCommands.HandleRunTask(chatID, msg.From.ID, command)
	case command == "昨日文件":
		h.downloadCommands.HandleYesterdayFiles(chatID)
	case command == "下载昨日":
		h.downloadCommands.HandleYesterdayDownload(chatID)
	default:
		h.messageUtils.SendMessage(chatID, "未知命令，发送 /help 查看可用命令")
	}
}

// handleCallbackQuery 处理回调查询
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

	// 处理预览相关回调
	if strings.HasPrefix(data, "preview_hours|") {
		hours := strings.TrimPrefix(data, "preview_hours|")
		h.telegramClient.AnswerCallbackQuery(callback.ID, "正在生成预览")
		if callback.Message != nil {
			h.messageUtils.ClearInlineKeyboard(chatID, callback.Message.MessageID)
		}
		h.handleQuickPreview(chatID, []string{hours})
		return
	}

	if data == "preview_custom" {
		h.telegramClient.AnswerCallbackQuery(callback.ID, "请输入自定义时间")
		if callback.Message != nil {
			h.messageUtils.ClearInlineKeyboard(chatID, callback.Message.MessageID)
		}
		message := "<b>自定义预览</b>\n\n" +
			"请发送以下格式之一：\n" +
			"• <code>/download &lt;小时数&gt;</code> （例如：/download 6）\n" +
			"• <code>/download YYYY-MM-DD YYYY-MM-DD</code>\n" +
			"• <code>/download 2025-01-01T00:00:00Z 2025-01-01T12:00:00Z</code>"
		h.messageUtils.SendMessageHTML(chatID, message)
		return
	}

	if data == "preview_cancel" {
		h.telegramClient.AnswerCallbackQuery(callback.ID, "已关闭")
		if callback.Message != nil {
			h.messageUtils.ClearInlineKeyboard(chatID, callback.Message.MessageID)
		}
		return
	}

	// 处理手动下载确认回调
	if strings.HasPrefix(data, "manual_confirm|") {
		token := strings.TrimPrefix(data, "manual_confirm|")
		h.telegramClient.AnswerCallbackQuery(callback.ID, "开始创建下载任务")
		if callback.Message != nil {
			h.handleManualConfirm(chatID, token, callback.Message.MessageID)
		}
		return
	}

	if strings.HasPrefix(data, "manual_cancel|") {
		token := strings.TrimPrefix(data, "manual_cancel|")
		h.telegramClient.AnswerCallbackQuery(callback.ID, "已取消")
		if callback.Message != nil {
			h.handleManualCancel(chatID, token, callback.Message.MessageID)
		}
		return
	}

	// 先回应回调查询
	h.telegramClient.AnswerCallbackQuery(callback.ID, "")

	// 处理文件浏览相关的回调
	if strings.HasPrefix(data, "browse_dir:") {
		parts := strings.Split(data, ":")
		if len(parts) >= 3 {
			encodedPath := parts[1]
			path := h.decodeFilePath(encodedPath)
			page, _ := strconv.Atoi(parts[2])
			if page < 1 {
				page = 1
			}
			logger.Info("点击目录", "encodedPath", encodedPath, "decodedPath", path, "page", page)
			h.handleBrowseFilesWithEdit(chatID, path, page, callback.Message.MessageID)
		}
		return
	}

	if strings.HasPrefix(data, "browse_page:") {
		parts := strings.Split(data, ":")
		if len(parts) >= 3 {
			path := h.decodeFilePath(parts[1])
			page, _ := strconv.Atoi(parts[2])
			if page < 1 {
				page = 1
			}
			h.handleBrowseFilesWithEdit(chatID, path, page, callback.Message.MessageID)
		}
		return
	}

	if strings.HasPrefix(data, "browse_refresh:") {
		parts := strings.Split(data, ":")
		if len(parts) >= 3 {
			path := h.decodeFilePath(parts[1])
			page, _ := strconv.Atoi(parts[2])
			if page < 1 {
				page = 1
			}
			h.handleBrowseFilesWithEdit(chatID, path, page, callback.Message.MessageID)
		}
		return
	}

	if strings.HasPrefix(data, "file_menu:") {
		filePath := h.decodeFilePath(strings.TrimPrefix(data, "file_menu:"))
		h.handleFileMenuWithEdit(chatID, filePath, callback.Message.MessageID)
		return
	}

	if strings.HasPrefix(data, "file_download:") {
		filePath := h.decodeFilePath(strings.TrimPrefix(data, "file_download:"))
		h.handleFileDownload(chatID, filePath)
		return
	}

	if strings.HasPrefix(data, "file_info:") {
		filePath := h.decodeFilePath(strings.TrimPrefix(data, "file_info:"))
		h.handleFileInfoWithEdit(chatID, filePath, callback.Message.MessageID)
		return
	}

	if strings.HasPrefix(data, "file_link:") {
		filePath := h.decodeFilePath(strings.TrimPrefix(data, "file_link:"))
		h.handleFileLinkWithEdit(chatID, filePath, callback.Message.MessageID)
		return
	}

	if strings.HasPrefix(data, "download_dir:") {
		dirPath := h.decodeFilePath(strings.TrimPrefix(data, "download_dir:"))
		h.handleDownloadDirectory(chatID, dirPath)
		return
	}

	// 处理菜单回调
	switch data {
	case "cmd_help":
		h.menuCallbacks.HandleHelpWithEdit(chatID, callback.Message.MessageID)
	case "cmd_status":
		h.menuCallbacks.HandleStatusWithEdit(chatID, callback.Message.MessageID)
	case "cmd_manage":
		h.menuCallbacks.HandleManageWithEdit(chatID, callback.Message.MessageID)
	case "menu_download":
		h.menuCallbacks.HandleDownloadMenuWithEdit(chatID, callback.Message.MessageID)
	case "menu_files":
		h.menuCallbacks.HandleFilesMenuWithEdit(chatID, callback.Message.MessageID)
	case "menu_system":
		h.menuCallbacks.HandleSystemMenuWithEdit(chatID, callback.Message.MessageID)
	case "menu_status":
		h.menuCallbacks.HandleStatusMenuWithEdit(chatID, callback.Message.MessageID)
	case "show_yesterday_options", "api_yesterday_files", "api_yesterday_files_preview", "api_yesterday_download":
		// 昨日文件功能已移除，跳转到定时任务
		h.handleTasksWithEdit(chatID, userID, callback.Message.MessageID)
	case "cmd_tasks":
		h.handleTasksWithEdit(chatID, userID, callback.Message.MessageID)
	case "api_download_status":
		h.handleDownloadStatusAPIWithEdit(chatID, callback.Message.MessageID)
	case "api_alist_login":
		h.handleAlistLoginWithEdit(chatID, callback.Message.MessageID)
	case "api_health_check":
		h.handleHealthCheckWithEdit(chatID, callback.Message.MessageID)
	case "back_main":
		h.menuCallbacks.HandleStartWithEdit(chatID, callback.Message.MessageID)
	// 下载管理功能
	case "download_list":
		h.handleDownloadStatusAPIWithEdit(chatID, callback.Message.MessageID)
	case "download_create":
		h.handleDownloadCreateWithEdit(chatID, callback.Message.MessageID)
	case "download_control":
		h.handleDownloadControlWithEdit(chatID, callback.Message.MessageID)
	case "download_delete":
		h.handleDownloadDeleteWithEdit(chatID, callback.Message.MessageID)
	// 文件浏览功能
	case "files_browse":
		h.handleFilesBrowseWithEdit(chatID, callback.Message.MessageID)
	case "files_search":
		h.handleFilesSearchWithEdit(chatID, callback.Message.MessageID)
	case "files_info":
		h.handleFilesInfoWithEdit(chatID, callback.Message.MessageID)
	case "files_download":
		h.handleFilesDownloadWithEdit(chatID, callback.Message.MessageID)
	case "api_alist_files":
		h.handleAlistFilesWithEdit(chatID, callback.Message.MessageID)
	// 系统管理功能
	case "system_info":
		h.menuCallbacks.HandleSystemInfoWithEdit(chatID, callback.Message.MessageID)
	// 状态监控功能
	case "status_realtime":
		h.handleStatusRealtimeWithEdit(chatID, callback.Message.MessageID)
	case "status_storage":
		h.handleStatusStorageWithEdit(chatID, callback.Message.MessageID)
	case "status_history":
		h.handleStatusHistoryWithEdit(chatID, callback.Message.MessageID)
	default:
		h.messageUtils.SendMessage(chatID, "未知操作")
	}
}

// ================================
// 时间解析和手动下载核心功能
// ================================

// parseTimeArguments 解析时间参数
// 支持的格式：
// 1. 数字 - 小时数（如：48）
// 2. 日期范围 - 两个日期（如：2025-09-01 2025-09-26）
// 3. 时间范围 - 两个时间戳（如：2025-09-01T00:00:00Z 2025-09-26T23:59:59Z）
func (h *TelegramHandler) parseTimeArguments(args []string) (*TimeParseResult, error) {
	if len(args) == 0 {
		// 默认24小时
		timeRange := timeutils.CreateTimeRangeFromHours(24)
		return &TimeParseResult{
			StartTime:   timeRange.Start,
			EndTime:     timeRange.End,
			Description: "最近24小时",
		}, nil
	}

	if len(args) == 1 {
		// 尝试解析为小时数
		if hours, err := strconv.Atoi(args[0]); err == nil {
			if hours <= 0 {
				return nil, fmt.Errorf("小时数必须大于0")
			}
			if hours > 8760 { // 一年的小时数
				return nil, fmt.Errorf("小时数不能超过8760（一年）")
			}
			timeRange := timeutils.CreateTimeRangeFromHours(hours)
			return &TimeParseResult{
				StartTime:   timeRange.Start,
				EndTime:     timeRange.End,
				Description: fmt.Sprintf("最近%d小时", hours),
			}, nil
		}

		return nil, fmt.Errorf("无效的时间格式，应为小时数（如：48）")
	}

	if len(args) == 2 {
		startStr, endStr := args[0], args[1]

		// 使用统一的时间解析工具
		timeRange, err := timeutils.ParseTimeRange(startStr, endStr)
		if err != nil {
			return nil, fmt.Errorf("无效的时间格式，支持的格式：\n• 日期范围：2025-09-01 2025-09-26\n• 时间范围：2025-09-01T00:00:00Z 2025-09-26T23:59:59Z")
		}

		// 根据时间格式生成描述
		description := fmt.Sprintf("从 %s 到 %s", timeRange.Start.Format("2006-01-02 15:04"), timeRange.End.Format("2006-01-02 15:04"))
		// 如果是日期格式（时间都是0点），使用日期格式描述
		if timeRange.Start.Hour() == 0 && timeRange.Start.Minute() == 0 && timeRange.Start.Second() == 0 &&
			(timeRange.End.Hour() == 23 && timeRange.End.Minute() == 59) {
			description = fmt.Sprintf("从 %s 到 %s", timeRange.Start.Format("2006-01-02"), timeRange.End.Format("2006-01-02"))
		}

		return &TimeParseResult{
			StartTime:   timeRange.Start,
			EndTime:     timeRange.End,
			Description: description,
		}, nil
	}

	return nil, fmt.Errorf("参数过多，支持的格式：\n• /download\n• /download 48\n• /download 2025-09-01 2025-09-26\n• /download 2025-09-01T00:00:00Z 2025-09-26T23:59:59Z")
}


// handleManualDownload 处理手动下载功能，支持时间范围参数
func (h *TelegramHandler) handleManualDownload(chatID int64, timeArgs []string, preview bool) {
	// 解析时间参数
	timeResult, err := h.parseTimeArguments(timeArgs)
	if err != nil {
		message := fmt.Sprintf("<b>时间参数错误</b>\n\n%s\n\n<b>支持的格式：</b>\n• /download - 预览最近24小时\n• /download 48 - 预览最近48小时\n• /download 2025-09-01 2025-09-26 - 预览指定日期范围\n• /download 2025-09-01T00:00:00Z 2025-09-26T23:59:59Z - 预览精确时间范围\n\n<b>提示:</b> 在命令后添加 <code>confirm</code> 可直接开始下载", err.Error())
		h.messageUtils.SendMessageHTML(chatID, message)
		return
	}

	modeLabel := "下载"
	if preview {
		modeLabel = "预览"
	}

	processingMsg := fmt.Sprintf("<b>正在处理手动%s任务</b>\n\n时间范围: %s", modeLabel, timeResult.Description)
	h.messageUtils.SendMessageHTML(chatID, processingMsg)

	path := ""
	if h.config.Alist.DefaultPath != "" {
		path = h.config.Alist.DefaultPath
	}
	if path == "" {
		path = "/"
	}

	// 使用contracts.FileService接口获取文件列表
	timeRangeReq := contracts.TimeRangeFileRequest{
		Path:      path,
		StartTime: timeResult.StartTime,
		EndTime:   timeResult.EndTime,
		VideoOnly: true,
	}
	
	ctx := context.Background()
	timeRangeResp, err := h.fileService.GetFilesByTimeRange(ctx, timeRangeReq)
	if err != nil {
		h.messageUtils.SendMessage(chatID, fmt.Sprintf("处理失败: %s", err.Error()))
		return
	}
	
	files := timeRangeResp.Files

	if len(files) == 0 {
		var message string
		if preview {
			message = fmt.Sprintf("<b>手动下载预览</b>\n\n时间范围: %s\n\n<b>结果:</b> 未找到符合条件的文件", timeResult.Description)
		} else {
			message = fmt.Sprintf("<b>手动下载完成</b>\n\n时间范围: %s\n\n<b>结果:</b> 未找到符合条件的文件", timeResult.Description)
		}
		h.messageUtils.SendMessageHTML(chatID, message)
		return
	}

	// 使用contracts返回的统计信息
	summary := timeRangeResp.Summary
	totalFiles := summary.TotalFiles
	totalSizeStr := summary.TotalSizeFormatted
	
	// 重新构建媒体统计结构以保持兼容性
	mediaStats := struct {
		TV    int
		Movie int
		Other int
	}{
		TV:    summary.TVFiles,
		Movie: summary.MovieFiles,
		Other: summary.OtherFiles,
	}

	if preview {
		confirmCommand := "/download confirm"
		if len(timeArgs) > 0 {
			confirmCommand += " " + strings.Join(timeArgs, " ")
		}

		message := fmt.Sprintf(
			"<b>手动下载预览</b>\n\n"+
				"<b>时间范围:</b> %s\n"+
				"<b>路径:</b> <code>%s</code>\n\n"+
				"<b>文件统计:</b>\n"+
				"• 总文件: %d 个\n"+
				"• 总大小: %s\n"+
				"• 电影: %d 个\n"+
				"• 剧集: %d 个\n"+
				"• 其他: %d 个",
			timeResult.Description,
			h.messageUtils.EscapeHTML(path),
			totalFiles,
			totalSizeStr,
			mediaStats.Movie,
			mediaStats.TV,
			mediaStats.Other,
		)

		if len(files) > 0 {
			message += "\n\n<b>示例文件:</b>\n"
			// 显示前几个文件作为示例
			maxExamples := 5
			if len(files) < maxExamples {
				maxExamples = len(files)
			}
			for i := 0; i < maxExamples; i++ {
				file := files[i]
				filename := h.messageUtils.EscapeHTML(file.Name)
				runes := []rune(filename)
				if len(runes) > 60 {
					filename = string(runes[:60]) + "..."
				}
				downloadPath := h.messageUtils.EscapeHTML(file.DownloadPath)
				message += fmt.Sprintf("• %s → <code>%s</code>\n", filename, downloadPath)
			}
		}

		message += fmt.Sprintf("\n\n⚠️ 预览有效期 10 分钟。也可以发送 <code>%s</code> 开始下载。", confirmCommand)

		// 存储预览结果用于确认下载
		storedReq := manualDownloadRequest{
			Path:      path,
			StartTime: timeResult.StartTime.Format(time.RFC3339),
			EndTime:   timeResult.EndTime.Format(time.RFC3339),
			VideoOnly: true,
			Preview:   false,
		}

		ctx := &ManualDownloadContext{
			ChatID:      chatID,
			Request:     storedReq,
			Description: timeResult.Description,
			TimeArgs:    append([]string(nil), timeArgs...),
		}
		token := h.storeManualContext(ctx)

		confirmData := fmt.Sprintf("manual_confirm|%s", token)
		cancelData := fmt.Sprintf("manual_cancel|%s", token)

		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("✅ 确认开始下载", confirmData),
				tgbotapi.NewInlineKeyboardButtonData("✖️ 取消", cancelData),
			),
		)

		h.messageUtils.SendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
		return
	}

	// 如果不是预览模式，创建实际的下载任务
	if !preview {
		successCount := 0
		failCount := 0
		var failedFiles []string

		// 创建下载任务 - 使用contracts接口
		for _, file := range files {
			downloadReq := contracts.DownloadRequest{
				URL:         file.InternalURL,
				Filename:    file.Name,
				Directory:   file.DownloadPath,
				AutoClassify: true,
			}
			
			_, err := h.downloadService.CreateDownload(ctx, downloadReq)
			if err != nil {
				failCount++
				failedFiles = append(failedFiles, file.Name)
				logger.Error("创建下载任务失败", "file", file.Name, "error", err)
				continue
			}
			successCount++
		}

		message := fmt.Sprintf(
			"<b>手动下载任务已创建</b>\n\n"+
				"<b>时间范围:</b> %s\n"+
				"<b>路径:</b> <code>%s</code>\n\n"+
				"<b>文件统计:</b>\n"+
				"• 总文件: %d 个\n"+
				"• 总大小: %s\n"+
				"• 电影: %d 个\n"+
				"• 剧集: %d 个\n"+
				"• 其他: %d 个\n\n"+
				"<b>下载结果:</b>\n"+
				"• 成功: %d\n"+
				"• 失败: %d",
			timeResult.Description,
			h.messageUtils.EscapeHTML(path),
			totalFiles,
			totalSizeStr,
			mediaStats.Movie,
			mediaStats.TV,
			mediaStats.Other,
			successCount,
			failCount,
		)

		if failCount > 0 {
			message += fmt.Sprintf("\n\n⚠️ 有 %d 个文件下载失败，请检查日志获取详细信息", failCount)
		}

		h.messageUtils.SendMessageHTML(chatID, message)
		return
	}
}

// formatFileSize 格式化文件大小（委托给MessageUtils）
func (h *TelegramHandler) formatFileSize(size int64) string {
	return h.messageUtils.FormatFileSize(size)
}

// FormatFileSize 公共方法：格式化文件大小
func (h *TelegramHandler) FormatFileSize(size int64) string {
	return h.formatFileSize(size)
}

// handleQuickPreview 处理快速预览
func (h *TelegramHandler) handleQuickPreview(chatID int64, timeArgs []string) {
	h.handleManualDownload(chatID, timeArgs, true)
}

// ================================
// 手动下载上下文管理（兼容旧版本）
// ================================

// storeManualContext 存储手动下载上下文
func (h *TelegramHandler) storeManualContext(ctx *ManualDownloadContext) string {
	h.cleanupManualContexts()

	ctxCopy := *ctx
	ctxCopy.TimeArgs = append([]string(nil), ctx.TimeArgs...)
	ctxCopy.CreatedAt = time.Now()

	token := fmt.Sprintf("md-%d-%d", ctx.ChatID, time.Now().UnixNano())

	h.manualMutex.Lock()
	h.manualContexts[token] = &ctxCopy
	h.manualMutex.Unlock()

	return token
}

// getManualContext 获取手动下载上下文
func (h *TelegramHandler) getManualContext(token string) (*ManualDownloadContext, bool) {
	h.manualMutex.Lock()
	defer h.manualMutex.Unlock()

	ctx, ok := h.manualContexts[token]
	if !ok {
		return nil, false
	}

	copyCtx := *ctx
	copyCtx.TimeArgs = append([]string(nil), ctx.TimeArgs...)
	return &copyCtx, true
}

// deleteManualContext 删除手动下载上下文
func (h *TelegramHandler) deleteManualContext(token string) {
	h.manualMutex.Lock()
	delete(h.manualContexts, token)
	h.manualMutex.Unlock()
}

// cleanupManualContexts 清理过期的手动下载上下文
func (h *TelegramHandler) cleanupManualContexts() {
	cutoff := time.Now().Add(-10 * time.Minute)
	h.manualMutex.Lock()
	for token, ctx := range h.manualContexts {
		if ctx.CreatedAt.Before(cutoff) {
			delete(h.manualContexts, token)
		}
	}
	h.manualMutex.Unlock()
}

// handleManualConfirm 处理手动下载确认
func (h *TelegramHandler) handleManualConfirm(chatID int64, token string, messageID int) {
	ctx, ok := h.getManualContext(token)
	if !ok {
		h.messageUtils.SendMessage(chatID, "预览已过期，请重新生成")
		return
	}

	if ctx.ChatID != chatID {
		h.messageUtils.SendMessage(chatID, "无效的确认请求")
		return
	}

	h.deleteManualContext(token)
	h.messageUtils.ClearInlineKeyboard(chatID, messageID)

	h.messageUtils.SendMessage(chatID, "正在创建下载任务...")

	req := ctx.Request

	// 使用统一的时间解析工具
	startTime, err := timeutils.ParseTime(req.StartTime)
	if err != nil {
		h.messageUtils.SendMessage(chatID, fmt.Sprintf("时间解析失败: %v", err))
		return
	}
	endTime, err := timeutils.ParseTime(req.EndTime)
	if err != nil {
		h.messageUtils.SendMessage(chatID, fmt.Sprintf("时间解析失败: %v", err))
		return
	}

	// 使用contracts.FileService接口获取文件列表
	timeRangeReq := contracts.TimeRangeFileRequest{
		Path:      req.Path,
		StartTime: startTime,
		EndTime:   endTime,
		VideoOnly: req.VideoOnly,
	}
	
	requestCtx := context.Background()
	timeRangeResp, err := h.fileService.GetFilesByTimeRange(requestCtx, timeRangeReq)
	if err != nil {
		h.messageUtils.SendMessage(chatID, fmt.Sprintf("创建下载任务失败: %v", err))
		return
	}
	
	files := timeRangeResp.Files

	if len(files) == 0 {
		message := fmt.Sprintf("<b>手动下载完成</b>\n\n时间范围: %s\n\n<b>结果:</b> 未找到符合条件的文件", ctx.Description)
		h.messageUtils.SendMessageHTML(chatID, message)
		return
	}

	// 使用contracts返回的统计信息
	summary := timeRangeResp.Summary
	totalFiles := summary.TotalFiles
	totalSizeStr := summary.TotalSizeFormatted
	
	// 重新构建媒体统计结构以保持兼容性
	mediaStats := struct {
		TV    int
		Movie int
		Other int
	}{
		TV:    summary.TVFiles,
		Movie: summary.MovieFiles,
		Other: summary.OtherFiles,
	}

	// 创建下载任务 - 使用contracts接口
	successCount := 0
	failCount := 0
	var failedFiles []string

	for _, file := range files {
		downloadReq := contracts.DownloadRequest{
			URL:         file.InternalURL,
			Filename:    file.Name,
			Directory:   file.DownloadPath,
			AutoClassify: true,
		}
		
		_, err := h.downloadService.CreateDownload(requestCtx, downloadReq)
		if err != nil {
			failCount++
			failedFiles = append(failedFiles, file.Name)
			logger.Error("创建下载任务失败", "file", file.Name, "error", err)
			continue
		}
		successCount++
	}

	// totalSizeStr已在上面从summary中获取

	message := fmt.Sprintf(
		"<b>手动下载任务已创建</b>\n\n"+
			"<b>时间范围:</b> %s\n"+
			"<b>路径:</b> <code>%s</code>\n\n"+
			"<b>文件统计:</b>\n"+
			"• 总文件: %d 个\n"+
			"• 总大小: %s\n"+
			"• 电影: %d 个\n"+
			"• 剧集: %d 个\n"+
			"• 其他: %d 个\n\n"+
			"<b>下载结果:</b>\n"+
			"• 成功: %d\n"+
			"• 失败: %d",
		ctx.Description,
		h.messageUtils.EscapeHTML(req.Path),
		totalFiles,
		totalSizeStr,
		mediaStats.Movie,
		mediaStats.TV,
		mediaStats.Other,
		successCount,
		failCount,
	)

	if failCount > 0 {
		message += fmt.Sprintf("\n\n⚠️ 有 %d 个文件下载失败，请检查日志获取详细信息", failCount)
	}

	h.messageUtils.SendMessageHTML(chatID, message)
}

// handleManualCancel 处理手动下载取消
func (h *TelegramHandler) handleManualCancel(chatID int64, token string, messageID int) {
	ctx, ok := h.getManualContext(token)
	if ok && ctx.ChatID == chatID {
		h.deleteManualContext(token)
	}

	h.messageUtils.ClearInlineKeyboard(chatID, messageID)
	h.messageUtils.SendMessage(chatID, "已取消此次下载预览")
}

// ================================
// 文件浏览功能（已完成迁移）
// ================================

// handleBrowseFiles 处理文件浏览（支持分页和交互）
func (h *TelegramHandler) handleBrowseFiles(chatID int64, path string, page int) {
	h.handleBrowseFilesWithEdit(chatID, path, page, 0) // 0 表示发送新消息
}

// handleBrowseFilesWithEdit 处理文件浏览（支持编辑消息和分页）
func (h *TelegramHandler) handleBrowseFilesWithEdit(chatID int64, path string, page int, messageID int) {
	if path == "" {
		path = "/"
	}
	if page < 1 {
		page = 1
	}

	// 调试日志
	logger.Info("浏览文件", "path", path, "page", page, "messageID", messageID)

	// 只在发送新消息时显示提示
	if messageID == 0 {
		h.messageUtils.SendMessage(chatID, "正在获取文件列表...")
	}

	// 获取文件列表 (每页显示8个文件，为按钮布局留出空间)
	files, err := h.listFilesSimple(path, page, 8)
	if err != nil {
		h.messageUtils.SendMessage(chatID, fmt.Sprintf("获取文件列表失败: %v", err))
		return
	}

	if len(files) == 0 {
		h.messageUtils.SendMessage(chatID, "当前目录为空")
		return
	}

	// 构建消息
	message := fmt.Sprintf("<b>文件浏览器</b>\n\n")
	message += fmt.Sprintf("<b>当前路径:</b> <code>%s</code>\n", h.messageUtils.EscapeHTML(path))
	message += fmt.Sprintf("<b>第 %d 页</b>\n\n", page)

	// 构建内联键盘
	var keyboard [][]tgbotapi.InlineKeyboardButton

	for _, file := range files {
		var prefix string
		var callbackData string

		if file.IsDir {
			prefix = "📁"
			// 目录点击：进入子目录
			// 构建完整路径
			var fullPath string
			if file.Path != "" {
				fullPath = file.Path
			} else {
				if path == "/" {
					fullPath = "/" + file.Name
				} else {
					fullPath = path + "/" + file.Name
				}
			}
			callbackData = fmt.Sprintf("browse_dir:%s:%d", h.encodeFilePath(fullPath), 1)
		} else if h.fileService.IsVideoFile(file.Name) {
			prefix = "🎬"
			// 视频文件点击：显示操作菜单
			// 构建完整路径
			var fullPath string
			if file.Path != "" {
				fullPath = file.Path
			} else {
				if path == "/" {
					fullPath = "/" + file.Name
				} else {
					fullPath = path + "/" + file.Name
				}
			}
			callbackData = fmt.Sprintf("file_menu:%s", h.encodeFilePath(fullPath))
		} else {
			prefix = "📄"
			// 其他文件点击：显示操作菜单
			// 构建完整路径
			var fullPath string
			if file.Path != "" {
				fullPath = file.Path
			} else {
				if path == "/" {
					fullPath = "/" + file.Name
				} else {
					fullPath = path + "/" + file.Name
				}
			}
			callbackData = fmt.Sprintf("file_menu:%s", h.encodeFilePath(fullPath))
		}

		fileName := file.Name
		// 为文件列表中的快捷下载按钮预留空间，缩短显示长度
		maxLen := 22
		if !file.IsDir {
			maxLen = 18 // 文件行需要预留下载按钮空间
		}
		if len(fileName) > maxLen {
			fileName = fileName[:maxLen-3] + "..."
		}

		button := tgbotapi.NewInlineKeyboardButtonData(
			fmt.Sprintf("%s %s", prefix, fileName),
			callbackData,
		)

		// 为文件（非目录）添加快捷下载按钮
		if !file.IsDir {
			// 文件行：文件名按钮 + 快捷下载按钮
			var fullPath string
			if file.Path != "" {
				fullPath = file.Path
			} else {
				if path == "/" {
					fullPath = "/" + file.Name
				} else {
					fullPath = path + "/" + file.Name
				}
			}

			downloadButton := tgbotapi.NewInlineKeyboardButtonData(
				"📥",
				fmt.Sprintf("file_download:%s", h.encodeFilePath(fullPath)),
			)

			keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{button, downloadButton})
		} else {
			// 目录行：只有目录按钮，占满整行
			keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{button})
		}
	}

	// 添加导航按钮
	navButtons := []tgbotapi.InlineKeyboardButton{}

	// 上一页按钮
	if page > 1 {
		navButtons = append(navButtons, tgbotapi.NewInlineKeyboardButtonData(
			"< 上一页",
			fmt.Sprintf("browse_page:%s:%d", h.encodeFilePath(path), page-1),
		))
	}

	// 下一页按钮 (如果当前页满了，可能还有下一页)
	if len(files) == 8 {
		navButtons = append(navButtons, tgbotapi.NewInlineKeyboardButtonData(
			"下一页 >",
			fmt.Sprintf("browse_page:%s:%d", h.encodeFilePath(path), page+1),
		))
	}

	if len(navButtons) > 0 {
		keyboard = append(keyboard, navButtons)
	}

	// 添加功能按钮 - 第一行：下载和刷新
	actionRow1 := []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("📥 下载目录", fmt.Sprintf("download_dir:%s", h.encodeFilePath(path))),
		tgbotapi.NewInlineKeyboardButtonData("🔄 刷新", fmt.Sprintf("browse_refresh:%s:%d", h.encodeFilePath(path), page)),
	}
	keyboard = append(keyboard, actionRow1)

	// 添加导航按钮 - 第二行：上级目录和主菜单
	actionRow2 := []tgbotapi.InlineKeyboardButton{}

	// 返回上级目录按钮
	if path != "/" {
		parentPath := h.getParentPath(path)
		actionRow2 = append(actionRow2, tgbotapi.NewInlineKeyboardButtonData(
			"⬆️ 上级目录",
			fmt.Sprintf("browse_dir:%s:%d", h.encodeFilePath(parentPath), 1),
		))
	}

	// 返回主菜单按钮
	actionRow2 = append(actionRow2, tgbotapi.NewInlineKeyboardButtonData("🏠 主菜单", "back_main"))

	if len(actionRow2) > 0 {
		keyboard = append(keyboard, actionRow2)
	}

	inlineKeyboard := tgbotapi.NewInlineKeyboardMarkup(keyboard...)

	if messageID > 0 {
		// 编辑现有消息
		h.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &inlineKeyboard)
	} else {
		// 发送新消息
		h.messageUtils.SendMessageWithKeyboard(chatID, message, "HTML", &inlineKeyboard)
	}
}

// handleFileMenu 处理文件操作菜单
func (h *TelegramHandler) handleFileMenu(chatID int64, filePath string) {
	h.handleFileMenuWithEdit(chatID, filePath, 0) // 0 表示发送新消息
}

// handleFileMenuWithEdit 处理文件操作菜单（支持消息编辑）
func (h *TelegramHandler) handleFileMenuWithEdit(chatID int64, filePath string, messageID int) {
	// 获取文件信息
	fileName := filepath.Base(filePath)
	fileExt := strings.ToLower(filepath.Ext(fileName))

	// 根据文件类型选择图标
	var fileIcon string
	if h.fileService.IsVideoFile(fileName) {
		fileIcon = "🎬"
	} else {
		fileIcon = "📄"
	}

	message := fmt.Sprintf("%s <b>文件操作</b>\n\n", fileIcon)
	message += fmt.Sprintf("<b>文件:</b> <code>%s</code>\n", h.messageUtils.EscapeHTML(fileName))
	message += fmt.Sprintf("<b>路径:</b> <code>%s</code>\n", h.messageUtils.EscapeHTML(filepath.Dir(filePath)))
	if fileExt != "" {
		message += fmt.Sprintf("<b>类型:</b> <code>%s</code>\n", strings.ToUpper(fileExt[1:]))
	}
	message += "\n请选择操作："

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📥 立即下载", fmt.Sprintf("file_download:%s", h.encodeFilePath(filePath))),
			tgbotapi.NewInlineKeyboardButtonData("ℹ️ 文件信息", fmt.Sprintf("file_info:%s", h.encodeFilePath(filePath))),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔗 获取链接", fmt.Sprintf("file_link:%s", h.encodeFilePath(filePath))),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📁 返回目录", fmt.Sprintf("browse_dir:%s:%d", h.encodeFilePath(h.getParentPath(filePath)), 1)),
			tgbotapi.NewInlineKeyboardButtonData("🏠 主菜单", "back_main"),
		),
	)

	if messageID > 0 {
		// 编辑现有消息
		h.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
	} else {
		// 发送新消息
		h.messageUtils.SendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
	}
}

// handleFileDownload 处理文件下载（使用/downloads命令机制）
func (h *TelegramHandler) handleFileDownload(chatID int64, filePath string) {
	// 直接调用新的基于/downloads命令的文件下载处理函数
	h.handleDownloadFileByPath(chatID, filePath)
}

// handleDownloadFileByPath 通过路径下载单个文件
func (h *TelegramHandler) handleDownloadFileByPath(chatID int64, filePath string) {
	h.messageUtils.SendMessage(chatID, "📥 正在通过/downloads命令创建文件下载任务...")

	// 使用文件服务获取文件信息
	parentDir := filepath.Dir(filePath)
	fileName := filepath.Base(filePath)

	files, err := h.listFilesSimple(parentDir, 1, 1000)
	if err != nil {
		h.messageUtils.SendMessage(chatID, fmt.Sprintf("❌ 获取文件信息失败: %v", err))
		return
	}

	// 查找目标文件
	var targetFile *contracts.FileResponse
	for _, file := range files {
		if file.Name == fileName {
			targetFile = &file
			break
		}
	}

	if targetFile == nil {
		h.messageUtils.SendMessage(chatID, "❌ 文件未找到")
		return
	}

	// 使用文件服务的智能分类功能
	fileInfo, err := h.getFilesFromPath(parentDir, false)
	if err != nil {
		h.messageUtils.SendMessage(chatID, fmt.Sprintf("❌ 获取文件详细信息失败: %v", err))
		return
	}

	// 找到对应的文件信息
	var targetFileInfo *contracts.FileResponse
	for _, info := range fileInfo {
		if info.Name == fileName {
			targetFileInfo = &info
			break
		}
	}

	if targetFileInfo == nil {
		h.messageUtils.SendMessage(chatID, "❌ 获取文件分类信息失败")
		return
	}

	// 创建下载任务 - 使用contracts接口
	downloadReq := contracts.DownloadRequest{
		URL:         targetFileInfo.InternalURL,
		Filename:    targetFileInfo.Name,
		Directory:   targetFileInfo.DownloadPath,
		AutoClassify: true,
	}
	
	ctx := context.Background()
	download, err := h.downloadService.CreateDownload(ctx, downloadReq)
	if err != nil {
		h.messageUtils.SendMessage(chatID, fmt.Sprintf("❌ 创建下载任务失败: %v", err))
		return
	}

	// 发送成功消息
	message := fmt.Sprintf(
		"✅ <b>文件下载任务已创建</b>\n\n"+
			"<b>文件:</b> <code>%s</code>\n"+
			"<b>路径:</b> <code>%s</code>\n"+
			"<b>下载路径:</b> <code>%s</code>\n"+
			"<b>任务ID:</b> <code>%s</code>\n"+
			"<b>大小:</b> %s",
		h.messageUtils.EscapeHTML(targetFileInfo.Name),
		h.messageUtils.EscapeHTML(filePath),
		h.messageUtils.EscapeHTML(targetFileInfo.DownloadPath),
		h.messageUtils.EscapeHTML(download.ID),
		h.messageUtils.FormatFileSize(targetFileInfo.Size))

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📥 下载管理", "download_list"),
			tgbotapi.NewInlineKeyboardButtonData("📁 返回目录", fmt.Sprintf("browse_dir:%s:%d", h.encodeFilePath(parentDir), 1)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🏠 主菜单", "back_main"),
		),
	)

	h.messageUtils.SendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
}

// handleFileInfo 处理文件信息查看
func (h *TelegramHandler) handleFileInfo(chatID int64, filePath string) {
	h.handleFileInfoWithEdit(chatID, filePath, 0) // 0 表示发送新消息
}

// handleFileInfoWithEdit 处理文件信息查看（支持消息编辑）
func (h *TelegramHandler) handleFileInfoWithEdit(chatID int64, filePath string, messageID int) {
	// 显示加载消息（仅在发送新消息时）
	if messageID == 0 {
		h.messageUtils.SendMessage(chatID, "正在获取文件信息...")
	}

	// 获取文件信息
	fileInfo, err := h.listFilesSimple(filepath.Dir(filePath), 1, 1000)
	if err != nil {
		message := "获取文件信息失败: " + err.Error()
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("返回", fmt.Sprintf("browse_dir:%s:%d", h.encodeFilePath(filepath.Dir(filePath)), 1)),
			),
		)
		if messageID > 0 {
			h.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
		} else {
			h.messageUtils.SendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
		}
		return
	}

	// 查找对应的文件
	var targetFile *contracts.FileResponse
	fileName := filepath.Base(filePath)
	for _, file := range fileInfo {
		if file.Name == fileName {
			targetFile = &file
			break
		}
	}

	if targetFile == nil {
		message := "文件未找到"
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("返回", fmt.Sprintf("browse_dir:%s:%d", h.encodeFilePath(filepath.Dir(filePath)), 1)),
			),
		)
		if messageID > 0 {
			h.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
		} else {
			h.messageUtils.SendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
		}
		return
	}

	// 使用文件的修改时间
	modTime := targetFile.Modified

	// 构建信息消息
	message := fmt.Sprintf("<b>文件信息</b>\n\n"+
		"<b>名称:</b> <code>%s</code>\n"+
		"<b>路径:</b> <code>%s</code>\n"+
		"<b>大小:</b> %s\n"+
		"<b>修改时间:</b> %s\n"+
		"<b>类型:</b> %s",
		h.messageUtils.EscapeHTML(targetFile.Name),
		h.messageUtils.EscapeHTML(filePath),
		h.messageUtils.FormatFileSize(targetFile.Size),
		modTime.Format("2006-01-02 15:04:05"),
		func() string {
			if h.fileService.IsVideoFile(targetFile.Name) {
				return "视频文件"
			}
			return "其他文件"
		}())

	// 添加返回按钮
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("返回", fmt.Sprintf("browse_dir:%s:%d", h.encodeFilePath(filepath.Dir(filePath)), 1)),
		),
	)

	if messageID > 0 {
		h.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
	} else {
		h.messageUtils.SendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
	}
}

// handleFileLink 处理获取文件链接
func (h *TelegramHandler) handleFileLink(chatID int64, filePath string) {
	h.handleFileLinkWithEdit(chatID, filePath, 0) // 0 表示发送新消息
}

// handleFileLinkWithEdit 处理获取文件链接（支持消息编辑）
func (h *TelegramHandler) handleFileLinkWithEdit(chatID int64, filePath string, messageID int) {
	// 显示加载消息（仅在发送新消息时）
	if messageID == 0 {
		h.messageUtils.SendMessage(chatID, "正在获取文件链接...")
	}

	// 获取文件下载链接
	downloadURL := h.getFileDownloadURL(filepath.Dir(filePath), filepath.Base(filePath))

	// 构建消息
	message := fmt.Sprintf("<b>文件链接</b>\n\n"+
		"<b>文件:</b> <code>%s</code>\n\n"+
		"<b>下载链接:</b>\n<code>%s</code>",
		h.messageUtils.EscapeHTML(filepath.Base(filePath)),
		h.messageUtils.EscapeHTML(downloadURL))

	// 添加返回按钮
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("返回", fmt.Sprintf("browse_dir:%s:%d", h.encodeFilePath(filepath.Dir(filePath)), 1)),
		),
	)

	if messageID > 0 {
		h.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
	} else {
		h.messageUtils.SendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
	}
}

// handleDownloadDirectory 处理目录下载（使用/downloads命令机制）
func (h *TelegramHandler) handleDownloadDirectory(chatID int64, dirPath string) {
	// 直接调用新的基于/downloads命令的目录下载处理函数
	h.handleDownloadDirectoryByPath(chatID, dirPath)
}

// DirectoryDownloadStats 目录下载统计信息
type DirectoryDownloadStats struct {
	TotalFiles   int
	VideoFiles   int
	TotalSize    int64
	MovieCount   int
	TVCount      int
	OtherCount   int
	TotalSizeStr string
}

// DirectoryDownloadResult 目录下载结果
type DirectoryDownloadResult struct {
	Stats        DirectoryDownloadStats
	SuccessCount int
	FailedCount  int
	FailedFiles  []string
}

// calculateDirectoryStats 计算目录统计信息
func (h *TelegramHandler) calculateDirectoryStats(files []contracts.FileResponse) DirectoryDownloadStats {
	stats := DirectoryDownloadStats{}
	
	// 过滤出视频文件并计算统计
	for _, file := range files {
		if h.fileService.IsVideoFile(file.Name) {
			stats.VideoFiles++
			stats.TotalSize += file.Size
			
			// 根据文件分类统计媒体类型
			category := h.fileService.GetFileCategory(file.Name)
			switch category {
			case "movie":
				stats.MovieCount++
			case "tv":
				stats.TVCount++
			default:
				stats.OtherCount++
			}
		}
	}
	
	stats.TotalFiles = len(files)
	stats.TotalSizeStr = h.messageUtils.FormatFileSize(stats.TotalSize)
	
	return stats
}

// [已删除] createDownloadTasks - 旧方法，已被新架构的DownloadDirectory替代

// handleDownloadDirectoryByPath 通过路径下载目录 - 使用重构后的新架构
func (h *TelegramHandler) handleDownloadDirectoryByPath(chatID int64, dirPath string) {
	h.messageUtils.SendMessage(chatID, "📂 正在创建目录下载任务...")

	ctx := context.Background()
	
	// 使用新架构的目录下载服务
	req := contracts.DirectoryDownloadRequest{
		DirectoryPath: dirPath,
		Recursive:     true,
		VideoOnly:     true,  // 只下载视频文件
		AutoClassify:  true,
	}
	
	result, err := h.fileService.DownloadDirectory(ctx, req)
	if err != nil {
		h.messageUtils.SendMessage(chatID, fmt.Sprintf("❌ 扫描目录失败: %v", err))
		return
	}
	
	if result.SuccessCount == 0 {
		if result.Summary.VideoFiles == 0 {
			h.messageUtils.SendMessage(chatID, "🎬 目录中没有找到视频文件")
		} else {
			h.messageUtils.SendMessage(chatID, "❌ 所有文件下载创建失败，请检查日志")
		}
		return
	}
	
	// 发送结果消息（使用新架构的结果格式）
	h.sendBatchDownloadResult(chatID, dirPath, result)
}

// sendBatchDownloadResult 发送批量下载结果消息 - 新架构格式
func (h *TelegramHandler) sendBatchDownloadResult(chatID int64, dirPath string, result *contracts.BatchDownloadResponse) {
	// 防止空指针解引用
	if result == nil {
		h.messageUtils.SendMessage(chatID, "❌ 批量下载结果为空")
		return
	}
	
	// 构建结果消息
	message := fmt.Sprintf(
		"📊 <b>目录下载任务创建完成</b>\n\n"+
			"<b>目录:</b> <code>%s</code>\n"+
			"<b>扫描文件:</b> %d 个\n"+
			"<b>视频文件:</b> %d 个\n"+
			"<b>成功创建:</b> %d 个任务\n"+
			"<b>失败:</b> %d 个任务\n\n",
		h.messageUtils.EscapeHTML(dirPath),
		result.Summary.TotalFiles,
		result.Summary.VideoFiles,
		result.SuccessCount,
		result.FailureCount)

	if result.Summary.MovieFiles > 0 {
		message += fmt.Sprintf("<b>电影:</b> %d 个\n", result.Summary.MovieFiles)
	}
	if result.Summary.TVFiles > 0 {
		message += fmt.Sprintf("<b>电视剧:</b> %d 个\n", result.Summary.TVFiles)
	}

	if result.FailureCount > 0 && len(result.Results) <= 3 {
		message += "\n<b>失败的文件:</b>\n"
		failedCount := 0
		for _, downloadResult := range result.Results {
			if !downloadResult.Success && failedCount < 3 {
				// 安全地获取文件名，避免空指针解引用
				filename := "未知文件"
				if downloadResult.Request.Filename != "" {
					filename = downloadResult.Request.Filename
				}
				message += fmt.Sprintf("• <code>%s</code>\n", h.messageUtils.EscapeHTML(filename))
				failedCount++
			}
		}
	} else if result.FailureCount > 3 {
		message += fmt.Sprintf("\n<b>有 %d 个文件下载失败</b>\n", result.FailureCount)
	}

	if result.SuccessCount > 0 {
		message += "\n✅ 所有任务已使用自动路径分类功能\n📥 可通过「下载管理」查看任务状态"
	}

	h.messageUtils.SendMessageHTML(chatID, message)
}

// sendDirectoryDownloadResult 发送目录下载结果消息
func (h *TelegramHandler) sendDirectoryDownloadResult(chatID int64, dirPath string, result DirectoryDownloadResult) {
	// 构建消息数据
	resultData := utils.DirectoryDownloadResultData{
		DirectoryPath: dirPath,
		TotalFiles:    result.Stats.TotalFiles,
		VideoFiles:    result.Stats.VideoFiles,
		TotalSizeStr:  result.Stats.TotalSizeStr,
		MovieCount:    result.Stats.MovieCount,
		TVCount:       result.Stats.TVCount,
		OtherCount:    result.Stats.OtherCount,
		SuccessCount:  result.SuccessCount,
		FailedCount:   result.FailedCount,
		FailedFiles:   result.FailedFiles,
	}

	// 使用 MessageUtils 格式化消息
	message := h.messageUtils.FormatDirectoryDownloadResult(resultData)

	// 创建回复键盘
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📥 下载管理", "download_list"),
			tgbotapi.NewInlineKeyboardButtonData("📁 返回目录", fmt.Sprintf("browse_dir:%s:%d", h.encodeFilePath(dirPath), 1)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🏠 主菜单", "back_main"),
		),
	)

	// 发送消息
	h.messageUtils.SendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
}

// ================================
// 任务管理功能（委托给模块化组件）
// ================================

// handleTasksWithEdit 处理查看定时任务（支持消息编辑）
func (h *TelegramHandler) handleTasksWithEdit(chatID int64, userID int64, messageID int) {
	if h.schedulerService == nil {
		message := "定时任务服务未启用"
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("返回主菜单", "back_main"),
			),
		)
		h.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
		return
	}

	tasks, err := h.schedulerService.GetUserTasks(userID)
	if err != nil {
		message := fmt.Sprintf("获取任务失败: %v", err)
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("返回主菜单", "back_main"),
			),
		)
		h.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
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

		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("返回管理面板", "cmd_manage"),
			),
		)
		h.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
		return
	}

	message := fmt.Sprintf("<b>您的定时任务 (%d个)</b>\n\n", len(tasks))

	for i, task := range tasks {
		status := "禁用"
		if task.Enabled {
			status = "启用"
		}

		// 计算时间描述
		timeDesc := h.formatTaskTimeDescription(task.HoursAgo)

		message += fmt.Sprintf(
			"<b>%d. %s</b> %s\n"+
				"   ID: <code>%s</code>\n"+
				"   Cron: <code>%s</code>\n"+
				"   路径: <code>%s</code>\n"+
				"   时间范围: 最近<b>%s</b>内修改的文件\n"+
				"   文件类型: %s\n",
			i+1, h.messageUtils.EscapeHTML(task.Name), status,
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

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("刷新任务", "cmd_tasks"),
			tgbotapi.NewInlineKeyboardButtonData("返回管理面板", "cmd_manage"),
		),
	)

	h.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
}

// formatTaskTimeDescription 格式化任务时间描述
func (h *TelegramHandler) formatTaskTimeDescription(hoursAgo int) string {
	switch hoursAgo {
	case 24:
		return "1天"
	case 48:
		return "2天"
	case 72:
		return "3天"
	case 168:
		return "7天"
	case 720:
		return "30天"
	default:
		return fmt.Sprintf("%d小时", hoursAgo)
	}
}

// ================================
// 下载状态功能（临时实现）
// ================================

// handleDownloadStatusAPIWithEdit 处理下载状态API（支持消息编辑）
func (h *TelegramHandler) handleDownloadStatusAPIWithEdit(chatID int64, messageID int) {
	ctx := context.Background()
	listReq := contracts.DownloadListRequest{
		Limit: 100, // 获取最近100个下载
	}
	downloads, err := h.downloadService.ListDownloads(ctx, listReq)
	if err != nil {
		message := "获取下载状态失败: " + err.Error()
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("重试", "api_download_status"),
				tgbotapi.NewInlineKeyboardButtonData("返回主菜单", "back_main"),
			),
		)
		h.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
		return
	}

	// 使用contracts返回的结构化数据
	activeCount := downloads.ActiveCount
	totalCount := downloads.TotalCount
	
	// 从GlobalStats中获取其他统计信息
	waitingCount := 0
	stoppedCount := 0
	if stats := downloads.GlobalStats; stats != nil {
		if w, ok := stats["waiting_count"].(int); ok {
			waitingCount = w
		}
		if s, ok := stats["stopped_count"].(int); ok {
			stoppedCount = s
		}
	}

	message := fmt.Sprintf("<b>下载状态总览</b>\n\n"+
		"<b>统计:</b>\n"+
		"• 总任务数: %d\n"+
		"• 活动中: %d\n"+
		"• 等待中: %d\n"+
		"• 已停止: %d\n\n",
		totalCount, activeCount, waitingCount, stoppedCount)

	// 显示活动任务
	if len(downloads.Downloads) > 0 {
		message += "<b>活动任务:</b>\n"
		shownCount := 0
		for _, download := range downloads.Downloads {
			if string(download.Status) == "active" && shownCount < 3 {
				gid := download.ID
				if len(gid) > 8 {
					gid = gid[:8] + "..."
				}

				filename := download.Filename
				if filename == "" {
					filename = "未知文件"
				}
				if len(filename) > 30 {
					filename = filename[:30] + "..."
				}

				message += fmt.Sprintf("• %s - %s\n", gid, h.messageUtils.EscapeHTML(filename))
				shownCount++
			}
		}
		if activeCount > 3 {
			message += fmt.Sprintf("• ... 还有 %d 个任务\n", activeCount-3)
		}
		message += "\n"
	}

	// 显示等待和停止任务数量
	if waitingCount > 0 {
		message += fmt.Sprintf("<b>等待任务:</b> %d 个\n\n", waitingCount)
	}

	if stoppedCount > 0 {
		message += fmt.Sprintf("<b>已停止任务:</b> %d 个\n", stoppedCount)
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("刷新状态", "api_download_status"),
			tgbotapi.NewInlineKeyboardButtonData("下载管理", "menu_download"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("返回主菜单", "back_main"),
		),
	)

	h.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
}

// ================================
// 其他功能（临时实现，需要移植完整逻辑）
// ================================

// handleAlistLoginWithEdit 处理Alist登录（支持消息编辑）
func (h *TelegramHandler) handleAlistLoginWithEdit(chatID int64, messageID int) {
	// 显示正在登录的消息
	loadingMessage := "正在登录Alist..."
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("返回管理面板", "menu_system"),
		),
	)
	h.messageUtils.EditMessageWithKeyboard(chatID, messageID, loadingMessage, "HTML", &keyboard)

	// 创建Alist客户端
	alistClient := alist.NewClient(
		h.config.Alist.BaseURL,
		h.config.Alist.Username,
		h.config.Alist.Password,
	)

	// 执行登录
	err := alistClient.Login()

	var message string
	if err != nil {
		message = fmt.Sprintf("<b>❌ Alist登录失败</b>\n\n"+
			"<b>错误信息:</b> <code>%s</code>\n\n"+
			"<b>配置信息:</b>\n"+
			"• 地址: <code>%s</code>\n"+
			"• 用户名: <code>%s</code>\n\n"+
			"请检查配置是否正确",
			h.messageUtils.EscapeHTML(err.Error()),
			h.messageUtils.EscapeHTML(h.config.Alist.BaseURL),
			h.messageUtils.EscapeHTML(h.config.Alist.Username))
	} else {
		message = fmt.Sprintf("<b>✅ Alist登录成功！</b>\n\n"+
			"<b>服务器信息:</b>\n"+
			"• 地址: <code>%s</code>\n"+
			"• 用户名: <code>%s</code>\n"+
			"• 登录时间: %s",
			h.messageUtils.EscapeHTML(h.config.Alist.BaseURL),
			h.messageUtils.EscapeHTML(h.config.Alist.Username),
			time.Now().Format("2006-01-02 15:04:05"))
	}

	finalKeyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("重新登录", "api_alist_login"),
			tgbotapi.NewInlineKeyboardButtonData("健康检查", "api_health_check"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("返回管理面板", "menu_system"),
			tgbotapi.NewInlineKeyboardButtonData("返回主菜单", "back_main"),
		),
	)

	h.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &finalKeyboard)
}

// handleHealthCheckWithEdit 处理健康检查（支持消息编辑）
func (h *TelegramHandler) handleHealthCheckWithEdit(chatID int64, messageID int) {
	// 构建系统健康检查信息
	message := "<b>🏥 系统健康检查</b>\n\n"

	// 服务状态
	message += "<b>📊 服务状态:</b> ✅ 正常运行\n"
	message += fmt.Sprintf("<b>🚪 端口:</b> <code>%s</code>\n", h.config.Server.Port)
	message += fmt.Sprintf("<b>🔧 模式:</b> <code>%s</code>\n", h.config.Server.Mode)

	// Alist配置信息
	message += "\n<b>📂 Alist配置:</b>\n"
	message += fmt.Sprintf("• 地址: <code>%s</code>\n", h.messageUtils.EscapeHTML(h.config.Alist.BaseURL))
	message += fmt.Sprintf("• 默认路径: <code>%s</code>\n", h.messageUtils.EscapeHTML(h.config.Alist.DefaultPath))

	// Aria2配置信息
	message += "\n<b>⬇️ Aria2配置:</b>\n"
	message += fmt.Sprintf("• RPC地址: <code>%s</code>\n", h.messageUtils.EscapeHTML(h.config.Aria2.RpcURL))
	message += fmt.Sprintf("• 下载目录: <code>%s</code>\n", h.messageUtils.EscapeHTML(h.config.Aria2.DownloadDir))

	// Telegram配置信息
	message += "\n<b>📱 Telegram配置:</b>\n"
	if h.config.Telegram.Enabled {
		message += "• 状态: ✅ 已启用\n"
		totalUsers := len(h.config.Telegram.ChatIDs) + len(h.config.Telegram.AdminIDs)
		message += fmt.Sprintf("• 授权用户数: %d\n", totalUsers)
		message += fmt.Sprintf("• 管理员数: %d\n", len(h.config.Telegram.AdminIDs))
	} else {
		message += "• 状态: ❌ 未启用\n"
	}

	// 系统运行信息
	message += "\n<b>💻 系统信息:</b>\n"
	message += fmt.Sprintf("• 操作系统: <code>%s</code>\n", runtime.GOOS)
	message += fmt.Sprintf("• 系统架构: <code>%s</code>\n", runtime.GOARCH)
	message += fmt.Sprintf("• Go版本: <code>%s</code>\n", runtime.Version())
	message += fmt.Sprintf("• CPU核心数: <code>%d</code>\n", runtime.NumCPU())

	// 内存使用情况
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	message += fmt.Sprintf("• 内存使用: <code>%.2f MB</code>\n", float64(m.Alloc)/1024/1024)
	message += fmt.Sprintf("• 系统内存: <code>%.2f MB</code>\n", float64(m.Sys)/1024/1024)

	// Goroutine数量
	message += fmt.Sprintf("• Goroutine数: <code>%d</code>\n", runtime.NumGoroutine())

	// 检查时间
	message += fmt.Sprintf("\n<b>🕐 检查时间:</b> %s", time.Now().Format("2006-01-02 15:04:05"))

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔄 刷新", "api_health_check"),
			tgbotapi.NewInlineKeyboardButtonData("🔐 Alist登录", "api_alist_login"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📥 下载状态", "api_download_status"),
			tgbotapi.NewInlineKeyboardButtonData("⚙️ 管理面板", "menu_system"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🏠 返回主菜单", "back_main"),
		),
	)

	h.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
}

// handleDownloadCreateWithEdit 处理创建下载（支持消息编辑）
func (h *TelegramHandler) handleDownloadCreateWithEdit(chatID int64, messageID int) {
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

	h.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
}

// handleDownloadControlWithEdit 处理下载控制（支持消息编辑）
func (h *TelegramHandler) handleDownloadControlWithEdit(chatID int64, messageID int) {
	// 先获取下载列表数据
	ctx := context.Background()
	listReq := contracts.DownloadListRequest{
		Limit: 100, // 获取最近100个下载
	}
	downloads, err := h.downloadService.ListDownloads(ctx, listReq)
	if err != nil {
		message := "获取下载状态失败: " + err.Error()
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("返回下载管理", "menu_download"),
			),
		)
		h.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
		return
	}

	// 使用contracts返回的结构化数据
	activeCount := downloads.ActiveCount
	
	// 从GlobalStats中获取其他统计信息
	waitingCount := 0
	stoppedCount := 0
	if stats := downloads.GlobalStats; stats != nil {
		if w, ok := stats["waiting_count"].(int); ok {
			waitingCount = w
		}
		if s, ok := stats["stopped_count"].(int); ok {
			stoppedCount = s
		}
	}

	message := fmt.Sprintf("<b>下载控制中心</b>\n\n"+
		"<b>当前状态:</b>\n"+
		"• 活动任务: %d 个\n"+
		"• 等待任务: %d 个\n"+
		"• 已停止: %d 个\n\n"+
		"<b>控制说明:</b>\n"+
		"• 使用 /cancel &lt;GID&gt; 取消下载\n"+
		"• GID 是下载任务的唯一标识符\n"+
		"• 可以从下载列表中获取 GID",
		activeCount, waitingCount, stoppedCount)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("刷新状态", "download_list"),
			tgbotapi.NewInlineKeyboardButtonData("返回管理", "menu_download"),
		),
	)

	h.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
}

// handleDownloadDeleteWithEdit 处理删除下载（支持消息编辑）
func (h *TelegramHandler) handleDownloadDeleteWithEdit(chatID int64, messageID int) {
	message := "<b>删除下载任务</b>\n\n" +
		"<b>注意:</b> 删除操作将无法撤销\n\n" +
		"<b>操作说明:</b>\n" +
		"• 使用 /cancel &lt;GID&gt; 删除指定任务\n" +
		"• 先查看下载列表获取任务 GID\n" +
		"• 支持删除已完成和失败的任务"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("查看下载列表", "download_list"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("返回下载管理", "menu_download"),
		),
	)

	h.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
}

// ================================
// 下载管理功能的非编辑版本（兼容旧版本）
// ================================

// handleDownloadCreate 处理创建下载（兼容旧版本）
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

	h.messageUtils.SendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
}

// handleDownloadControl 处理下载控制（兼容旧版本）
func (h *TelegramHandler) handleDownloadControl(chatID int64) {
	h.messageUtils.SendMessage(chatID, "正在获取当前下载任务...")

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

	h.messageUtils.SendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
}

// handleDownloadDelete 处理删除下载（兼容旧版本）
func (h *TelegramHandler) handleDownloadDelete(chatID int64) {
	message := "<b>删除下载任务</b>\n\n" +
		"<b>注意:</b> 删除操作将无法撤销\n\n" +
		"正在获取当前任务列表..."

	h.messageUtils.SendMessageHTML(chatID, message)

	// 获取下载列表并提供删除选项
	h.handleDownloadStatusAPI(chatID)
}

// handleDownloadStatusAPI 处理下载状态API（兼容旧版本）
func (h *TelegramHandler) handleDownloadStatusAPI(chatID int64) {
	ctx := context.Background()
	listReq := contracts.DownloadListRequest{
		Limit: 100, // 获取最近100个下载
	}
	downloads, err := h.downloadService.ListDownloads(ctx, listReq)
	if err != nil {
		h.messageUtils.SendMessage(chatID, "获取下载状态失败: "+err.Error())
		return
	}

	// 使用contracts返回的结构化数据
	activeCount := downloads.ActiveCount
	totalCount := downloads.TotalCount
	
	// 从GlobalStats中获取其他统计信息
	waitingCount := 0
	stoppedCount := 0
	if stats := downloads.GlobalStats; stats != nil {
		if w, ok := stats["waiting_count"].(int); ok {
			waitingCount = w
		}
		if s, ok := stats["stopped_count"].(int); ok {
			stoppedCount = s
		}
	}

	message := fmt.Sprintf("<b>下载状态总览</b>\n\n"+
		"<b>统计:</b>\n"+
		"• 总任务数: %d\n"+
		"• 活动中: %d\n"+
		"• 等待中: %d\n"+
		"• 已停止: %d\n\n",
		totalCount, activeCount, waitingCount, stoppedCount)

	// 显示活动任务
	if len(downloads.Downloads) > 0 {
		message += "<b>活动任务:</b>\n"
		shownCount := 0
		for _, download := range downloads.Downloads {
			if string(download.Status) == "active" && shownCount < 3 {
				gid := download.ID
				if len(gid) > 8 {
					gid = gid[:8] + "..."
				}

				filename := download.Filename
				if filename == "" {
					filename = "未知文件"
				}
				if len(filename) > 30 {
					filename = filename[:30] + "..."
				}

				message += fmt.Sprintf("• %s - %s\n", gid, h.messageUtils.EscapeHTML(filename))
				shownCount++
			}
		}
		if activeCount > 3 {
			message += fmt.Sprintf("• ... 还有 %d 个任务\n", activeCount-3)
		}
		message += "\n"
	}

	// 显示等待和停止任务数量
	if waitingCount > 0 {
		message += fmt.Sprintf("<b>等待任务:</b> %d 个\n\n", waitingCount)
	}

	if stoppedCount > 0 {
		message += fmt.Sprintf("<b>已停止任务:</b> %d 个\n", stoppedCount)
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("刷新状态", "api_download_status"),
			tgbotapi.NewInlineKeyboardButtonData("下载管理", "menu_download"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("返回主菜单", "back_main"),
		),
	)

	h.messageUtils.SendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
}

// handleDownloadMenu 处理下载管理菜单（兼容旧版本）
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

	h.messageUtils.SendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
}

// handleCancel 处理取消下载命令（兼容旧版本）
func (h *TelegramHandler) handleCancel(chatID int64, command string) {
	parts := strings.Fields(command)
	if len(parts) < 2 {
		h.messageUtils.SendMessage(chatID, "请提供下载GID\n示例: /cancel abc123")
		return
	}

	gid := parts[1]

	// 取消下载任务
	ctx := context.Background()
	if err := h.downloadService.CancelDownload(ctx, gid); err != nil {
		h.messageUtils.SendMessage(chatID, "取消下载失败: "+err.Error())
		return
	}

	escapedID := h.messageUtils.EscapeHTML(gid)
	message := fmt.Sprintf("<b>下载已取消</b>\n\n下载GID: <code>%s</code>", escapedID)
	h.messageUtils.SendMessageHTML(chatID, message)
}

// handleFilesBrowseWithEdit 处理文件浏览（支持消息编辑）
func (h *TelegramHandler) handleFilesBrowseWithEdit(chatID int64, messageID int) {
	// 使用默认路径或根目录开始浏览
	defaultPath := h.config.Alist.DefaultPath
	if defaultPath == "" {
		defaultPath = "/"
	}
	h.handleBrowseFilesWithEdit(chatID, defaultPath, 1, messageID)
}

// handleFilesSearchWithEdit 处理文件搜索（支持消息编辑）
func (h *TelegramHandler) handleFilesSearchWithEdit(chatID int64, messageID int) {
	message := "<b>文件搜索功能</b>\n\n" +
		"<b>搜索说明:</b>\n" +
		"• 支持文件名关键词搜索\n" +
		"• 支持路径模糊匹配\n" +
		"• 支持文件类型过滤\n\n" +
		"<b>请输入搜索关键词:</b>\n" +
		"格式: /search <关键词>\n\n" +
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

	h.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
}

// handleFilesInfoWithEdit 处理文件信息查看（支持消息编辑）
func (h *TelegramHandler) handleFilesInfoWithEdit(chatID int64, messageID int) {
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

	h.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
}

// handleFilesDownloadWithEdit 处理路径下载功能（支持消息编辑）
func (h *TelegramHandler) handleFilesDownloadWithEdit(chatID int64, messageID int) {
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

	h.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
}

// handleAlistFilesWithEdit 处理获取Alist文件列表（支持消息编辑）
func (h *TelegramHandler) handleAlistFilesWithEdit(chatID int64, messageID int) {
	h.handleBrowseFilesWithEdit(chatID, h.config.Alist.DefaultPath, 1, messageID)
}

// handleStatusRealtimeWithEdit 处理实时状态（支持消息编辑）
func (h *TelegramHandler) handleStatusRealtimeWithEdit(chatID int64, messageID int) {
	// 获取当前下载状态
	h.handleDownloadStatusAPIWithEdit(chatID, messageID)
}

// handleStatusStorageWithEdit 处理存储状态监控（支持消息编辑）
func (h *TelegramHandler) handleStatusStorageWithEdit(chatID int64, messageID int) {
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

	h.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
}

// handleStatusHistoryWithEdit 处理历史统计数据（支持消息编辑）
func (h *TelegramHandler) handleStatusHistoryWithEdit(chatID int64, messageID int) {
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

	h.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
}

// ================================
// 文件浏览菜单功能（兼容旧版本）
// ================================

// handleFilesBrowse 处理文件浏览（兼容旧版本）
func (h *TelegramHandler) handleFilesBrowse(chatID int64) {
	// 使用默认路径或根目录开始浏览
	defaultPath := h.config.Alist.DefaultPath
	if defaultPath == "" {
		defaultPath = "/"
	}
	h.handleBrowseFiles(chatID, defaultPath, 1)
}

// handleFilesSearch 处理文件搜索（兼容旧版本）
func (h *TelegramHandler) handleFilesSearch(chatID int64) {
	message := "<b>文件搜索功能</b>\n\n" +
		"<b>搜索说明:</b>\n" +
		"• 支持文件名关键词搜索\n" +
		"• 支持路径模糊匹配\n" +
		"• 支持文件类型过滤\n\n" +
		"<b>请输入搜索关键词:</b>\n" +
		"格式: /search <关键词>\n\n" +
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

	h.messageUtils.SendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
}

// handleFilesInfo 处理文件信息查看（兼容旧版本）
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

	h.messageUtils.SendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
}

// handleFilesDownload 处理路径下载功能（兼容旧版本）
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

	h.messageUtils.SendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
}

// handleAlistFiles 处理获取Alist文件列表（兼容旧版本）
func (h *TelegramHandler) handleAlistFiles(chatID int64) {
	h.handleBrowseFiles(chatID, h.config.Alist.DefaultPath, 1)
}

// ================================
// 路径缓存管理（兼容旧版本）
// ================================

// encodeFilePath 编码文件路径用于callback data（使用缓存机制避免64字节限制）
func (h *TelegramHandler) encodeFilePath(path string) string {
	h.pathMutex.Lock()
	defer h.pathMutex.Unlock()

	// 检查是否已有缓存
	if token, exists := h.pathReverseCache[path]; exists {
		return token
	}

	// 创建新的短token
	h.pathTokenCounter++
	token := fmt.Sprintf("p%d", h.pathTokenCounter)

	// 存储到缓存
	h.pathCache[token] = path
	h.pathReverseCache[path] = token

	// 清理过期缓存（保持缓存大小合理）
	if len(h.pathCache) > 1000 {
		h.cleanupPathCache()
	}

	return token
}

// decodeFilePath 解码文件路径
func (h *TelegramHandler) decodeFilePath(encoded string) string {
	h.pathMutex.RLock()
	defer h.pathMutex.RUnlock()

	if path, exists := h.pathCache[encoded]; exists {
		return path
	}

	logger.Warn("路径token未找到:", "token", encoded)
	return "/" // 未找到时返回根目录
}

// cleanupPathCache 清理路径缓存（保留最近的500个）
func (h *TelegramHandler) cleanupPathCache() {
	// 这是一个简单的清理策略，实际应用中可以使用LRU等更复杂的策略
	if len(h.pathCache) <= 500 {
		return
	}

	// 清空缓存，重新开始（简单但有效）
	h.pathCache = make(map[string]string)
	h.pathReverseCache = make(map[string]string)
	h.pathTokenCounter = 1

	logger.Info("路径缓存已清理")
}

// getParentPath 获取父目录路径
func (h *TelegramHandler) getParentPath(path string) string {
	if path == "/" {
		return "/"
	}
	parentPath := filepath.Dir(path)
	if parentPath == "." {
		return "/"
	}
	return parentPath
}

// isDirectoryPath 判断路径是否为目录
func (h *TelegramHandler) isDirectoryPath(path string) bool {
	// 尝试获取文件列表来判断是否为目录
	files, err := h.listFilesSimple(path, 1, 1)
	return err == nil && len(files) >= 0
}

// ================================
// 状态监控功能（兼容旧版本）
// ================================

// handleStatusStorage 处理存储状态监控（兼容旧版本）
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

	h.messageUtils.SendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
}

// handleStatusHistory 处理历史统计数据（兼容旧版本）
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

	h.messageUtils.SendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
}

// handleStatusRealtime 处理实时状态监控（兼容旧版本）
func (h *TelegramHandler) handleStatusRealtime(chatID int64) {
	h.messageUtils.SendMessage(chatID, "正在获取实时状态数据...")

	// 获取当前下载状态
	h.handleDownloadStatusAPI(chatID)
}

// ================================
// 辅助方法 - 兼容性适配
// ================================

// listFilesSimple 简单列出文件 - 适配contracts.FileService接口
func (h *TelegramHandler) listFilesSimple(path string, page, perPage int) ([]contracts.FileResponse, error) {
	req := contracts.FileListRequest{
		Path:     path,
		Page:     page,
		PageSize: perPage,
	}
	
	ctx := context.Background()
	resp, err := h.fileService.ListFiles(ctx, req)
	if err != nil {
		return nil, err
	}
	
	// 合并文件和目录
	var allItems []contracts.FileResponse
	allItems = append(allItems, resp.Directories...)
	allItems = append(allItems, resp.Files...)
	
	return allItems, nil
}

// getFilesFromPath 从指定路径获取文件 - 适配contracts.FileService接口
func (h *TelegramHandler) getFilesFromPath(basePath string, recursive bool) ([]contracts.FileResponse, error) {
	req := contracts.FileListRequest{
		Path:      basePath,
		Recursive: recursive,
		PageSize:  10000, // 获取所有文件
	}
	
	ctx := context.Background()
	resp, err := h.fileService.ListFiles(ctx, req)
	if err != nil {
		return nil, err
	}
	
	return resp.Files, nil
}

// getFileDownloadURL 获取文件下载URL - 适配contracts.FileService接口
func (h *TelegramHandler) getFileDownloadURL(path, fileName string) string {
	// 构建完整路径
	fullPath := path + "/" + fileName
	if path == "/" {
		fullPath = "/" + fileName
	}

	ctx := context.Background()
	fileInfo, err := h.fileService.GetFileInfo(ctx, fullPath)
	if err != nil {
		// 如果获取失败，回退到直接构建URL
		return h.config.Alist.BaseURL + "/d" + fullPath
	}

	return fileInfo.InternalURL
}
