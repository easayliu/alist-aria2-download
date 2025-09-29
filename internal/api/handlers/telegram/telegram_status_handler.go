package telegram

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/easayliu/alist-aria2-download/internal/application/contracts"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/alist"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// StatusHandler 处理状态查询相关功能
type StatusHandler struct {
	controller *TelegramController
}

// NewStatusHandler 创建新的状态处理器
func NewStatusHandler(controller *TelegramController) *StatusHandler {
	return &StatusHandler{
		controller: controller,
	}
}

// ================================
// 下载状态功能
// ================================

// HandleDownloadStatusAPIWithEdit 处理下载状态API（支持消息编辑）
func (h *StatusHandler) HandleDownloadStatusAPIWithEdit(chatID int64, messageID int) {
	ctx := context.Background()
	listReq := contracts.DownloadListRequest{
		Limit: 100, // 获取最近100个下载
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

				message += fmt.Sprintf("• %s - %s\n", gid, h.controller.messageUtils.EscapeHTML(filename))
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

	h.controller.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
}

// ================================
// Alist和健康检查功能
// ================================

// HandleAlistLoginWithEdit 处理Alist登录（支持消息编辑）
func (h *StatusHandler) HandleAlistLoginWithEdit(chatID int64, messageID int) {
	// 显示正在登录的消息
	loadingMessage := "正在登录Alist..."
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("返回管理面板", "menu_system"),
		),
	)
	h.controller.messageUtils.EditMessageWithKeyboard(chatID, messageID, loadingMessage, "HTML", &keyboard)

	// 创建Alist客户端
	alistClient := alist.NewClient(
		h.controller.config.Alist.BaseURL,
		h.controller.config.Alist.Username,
		h.controller.config.Alist.Password,
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
			h.controller.messageUtils.EscapeHTML(err.Error()),
			h.controller.messageUtils.EscapeHTML(h.controller.config.Alist.BaseURL),
			h.controller.messageUtils.EscapeHTML(h.controller.config.Alist.Username))
	} else {
		message = fmt.Sprintf("<b>✅ Alist登录成功！</b>\n\n"+
			"<b>服务器信息:</b>\n"+
			"• 地址: <code>%s</code>\n"+
			"• 用户名: <code>%s</code>\n"+
			"• 登录时间: %s",
			h.controller.messageUtils.EscapeHTML(h.controller.config.Alist.BaseURL),
			h.controller.messageUtils.EscapeHTML(h.controller.config.Alist.Username),
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

	h.controller.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &finalKeyboard)
}

// HandleHealthCheckWithEdit 处理健康检查（支持消息编辑）
func (h *StatusHandler) HandleHealthCheckWithEdit(chatID int64, messageID int) {
	// 构建系统健康检查信息
	message := "<b>🏥 系统健康检查</b>\n\n"

	// 服务状态
	message += "<b>📊 服务状态:</b> ✅ 正常运行\n"
	message += fmt.Sprintf("<b>🚪 端口:</b> <code>%s</code>\n", h.controller.config.Server.Port)
	message += fmt.Sprintf("<b>🔧 模式:</b> <code>%s</code>\n", h.controller.config.Server.Mode)

	// Alist配置信息
	message += "\n<b>📂 Alist配置:</b>\n"
	message += fmt.Sprintf("• 地址: <code>%s</code>\n", h.controller.messageUtils.EscapeHTML(h.controller.config.Alist.BaseURL))
	message += fmt.Sprintf("• 默认路径: <code>%s</code>\n", h.controller.messageUtils.EscapeHTML(h.controller.config.Alist.DefaultPath))

	// Aria2配置信息
	message += "\n<b>⬇️ Aria2配置:</b>\n"
	message += fmt.Sprintf("• RPC地址: <code>%s</code>\n", h.controller.messageUtils.EscapeHTML(h.controller.config.Aria2.RpcURL))
	message += fmt.Sprintf("• 下载目录: <code>%s</code>\n", h.controller.messageUtils.EscapeHTML(h.controller.config.Aria2.DownloadDir))

	// Telegram配置信息
	message += "\n<b>📱 Telegram配置:</b>\n"
	if h.controller.config.Telegram.Enabled {
		message += "• 状态: ✅ 已启用\n"
		totalUsers := len(h.controller.config.Telegram.ChatIDs) + len(h.controller.config.Telegram.AdminIDs)
		message += fmt.Sprintf("• 授权用户数: %d\n", totalUsers)
		message += fmt.Sprintf("• 管理员数: %d\n", len(h.controller.config.Telegram.AdminIDs))
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

	h.controller.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
}

// ================================
// 状态监控功能
// ================================

// HandleStatusRealtimeWithEdit 处理实时状态（支持消息编辑）
func (h *StatusHandler) HandleStatusRealtimeWithEdit(chatID int64, messageID int) {
	// 获取当前下载状态
	h.HandleDownloadStatusAPIWithEdit(chatID, messageID)
}

// HandleStatusStorageWithEdit 处理存储状态监控（支持消息编辑）
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

// HandleStatusHistoryWithEdit 处理历史统计数据（支持消息编辑）
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