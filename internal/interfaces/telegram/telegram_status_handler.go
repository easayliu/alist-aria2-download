package telegram

import (
	"context"
	"runtime"
	"time"

	"github.com/easayliu/alist-aria2-download/internal/application/contracts"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/alist"
	"github.com/easayliu/alist-aria2-download/internal/interfaces/telegram/utils"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// StatusHandler handles status query related functions
type StatusHandler struct {
	controller *TelegramController
}

// NewStatusHandler creates a new status handler
func NewStatusHandler(controller *TelegramController) *StatusHandler {
	return &StatusHandler{
		controller: controller,
	}
}

// ================================
// Download Status Functions
// ================================

// HandleDownloadStatusAPIWithEdit handles download status API (supports message editing)
func (h *StatusHandler) HandleDownloadStatusAPIWithEdit(chatID int64, messageID int) {
	ctx := context.Background()
	listReq := contracts.DownloadListRequest{
		Limit: 100,
	}
	downloads, err := h.controller.downloadService.ListDownloads(ctx, listReq)
	if err != nil {
		message := "获取下载状态失败: " + err.Error()
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("重试", "api_download_status"),
				tgbotapi.NewInlineKeyboardButtonData("返回主菜单", "back_main"),
			),
		)
		h.controller.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
		return
	}

	// Build download list data
	var downloadItems []utils.DownloadItemData
	for _, d := range downloads.Downloads {
		// Get status emoji
		statusEmoji := "❓"
		switch string(d.Status) {
		case "active", "running":
			statusEmoji = "🔄"
		case "complete", "completed":
			statusEmoji = "✅"
		case "paused":
			statusEmoji = "⏸️"
		case "error", "failed":
			statusEmoji = "❌"
		case "waiting", "pending":
			statusEmoji = "⏳"
		}

		downloadItems = append(downloadItems, utils.DownloadItemData{
			StatusEmoji: statusEmoji,
			ID:          d.ID,
			Filename:    d.Filename,
			Progress:    d.Progress,
		})
	}

	// 使用统一格式化器
	formatter := h.controller.messageUtils.GetFormatter().(*utils.MessageFormatter)
	listData := utils.DownloadListData{
		TotalCount:  downloads.TotalCount,
		ActiveCount: downloads.ActiveCount,
		Downloads:   downloadItems,
	}
	message := formatter.FormatDownloadList(listData)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("刷新状态", "api_download_status"),
			tgbotapi.NewInlineKeyboardButtonData("下载管理", "menu_download"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("返回主菜单", "back_main"),
		),
	)

	h.controller.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
}

// ================================
// Alist and Health Check Functions
// ================================

// HandleAlistLoginWithEdit handles Alist login (supports message editing)
func (h *StatusHandler) HandleAlistLoginWithEdit(chatID int64, messageID int) {
	// Display testing connection message
	loadingMessage := "正在测试Alist连接..."
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("返回管理面板", "menu_system"),
		),
	)
	h.controller.messageUtils.EditMessageWithKeyboard(chatID, messageID, loadingMessage, "HTML", &keyboard)

	// Create Alist client
	alistClient := alist.NewClient(
		h.controller.config.Alist.BaseURL,
		h.controller.config.Alist.Username,
		h.controller.config.Alist.Password,
	)

	// Clear existing token to force re-login
	alistClient.ClearToken()

	// Test connection and login by calling API (client handles token refresh automatically)
	_, err := alistClient.ListFiles("/", 1, 1)

	// Use unified formatter
	formatter := h.controller.messageUtils.GetFormatter().(*utils.MessageFormatter)
	var message string

	if err != nil {
		message = formatter.FormatAlistConnectionResult(utils.AlistConnectionData{
			Success:  false,
			URL:      h.controller.config.Alist.BaseURL,
			Username: h.controller.config.Alist.Username,
			Error:    err.Error(),
		})
	} else {
		message = formatter.FormatAlistConnectionResult(utils.AlistConnectionData{
			Success:  true,
			URL:      h.controller.config.Alist.BaseURL,
			Username: h.controller.config.Alist.Username,
		})
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

	h.controller.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &finalKeyboard)
}

// HandleHealthCheckWithEdit handles health check (supports message editing)
func (h *StatusHandler) HandleHealthCheckWithEdit(chatID int64, messageID int) {
	// Build system health check data
	var telegramStatus string
	var telegramUsers, telegramAdmins int

	if h.controller.config.Telegram.Enabled {
		telegramStatus = "✅ 已启用"
		telegramUsers = len(h.controller.config.Telegram.ChatIDs) + len(h.controller.config.Telegram.AdminIDs)
		telegramAdmins = len(h.controller.config.Telegram.AdminIDs)
	} else {
		telegramStatus = "❌ 未启用"
	}

	// Use unified formatter
	formatter := h.controller.messageUtils.GetFormatter().(*utils.MessageFormatter)
	data := utils.SystemStatusData{
		ServiceStatus:  "✅ 正常运行",
		Port:           h.controller.config.Server.Port,
		Mode:           h.controller.config.Server.Mode,
		AlistURL:       h.controller.messageUtils.EscapeHTML(h.controller.config.Alist.BaseURL),
		AlistPath:      h.controller.messageUtils.EscapeHTML(h.controller.config.Alist.DefaultPath),
		Aria2RPC:       h.controller.messageUtils.EscapeHTML(h.controller.config.Aria2.RpcURL),
		Aria2Dir:       h.controller.messageUtils.EscapeHTML(h.controller.config.Aria2.DownloadDir),
		TelegramStatus: telegramStatus,
		TelegramUsers:  telegramUsers,
		TelegramAdmins: telegramAdmins,
		OS:             runtime.GOOS,
		Arch:           runtime.GOARCH,
	}

	message := formatter.FormatSystemStatus(data)

	// Add runtime information
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	runtimeInfo := formatter.FormatRuntimeInfo(utils.RuntimeInfoData{
		GoVersion:    runtime.Version(),
		CPUCores:     runtime.NumCPU(),
		MemoryUsage:  float64(m.Alloc) / 1024 / 1024,
		SystemMemory: float64(m.Sys) / 1024 / 1024,
		Goroutines:   runtime.NumGoroutine(),
		CheckTime:    time.Now().Format("2006-01-02 15:04:05"),
	})

	message += runtimeInfo

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

	h.controller.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
}

// ================================
// Status Monitoring Functions
// ================================

// HandleStatusRealtimeWithEdit handles real-time status (supports message editing)
func (h *StatusHandler) HandleStatusRealtimeWithEdit(chatID int64, messageID int) {
	// Get current download status
	h.HandleDownloadStatusAPIWithEdit(chatID, messageID)
}

// HandleStatusStorageWithEdit handles storage status monitoring (supports message editing)
func (h *StatusHandler) HandleStatusStorageWithEdit(chatID int64, messageID int) {
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

	h.controller.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
}

// HandleStatusHistoryWithEdit handles historical statistics (supports message editing)
func (h *StatusHandler) HandleStatusHistoryWithEdit(chatID int64, messageID int) {
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

	h.controller.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
}
