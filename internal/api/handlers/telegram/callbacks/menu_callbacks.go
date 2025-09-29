package callbacks

import (
	"context"
	"fmt"
	"runtime"

	"github.com/easayliu/alist-aria2-download/internal/api/handlers/telegram/types"
	"github.com/easayliu/alist-aria2-download/internal/application/contracts"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/config"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// MenuCallbacks 菜单回调处理器
type MenuCallbacks struct {
	downloadService contracts.DownloadService
	config          *config.Config
	messageUtils    types.MessageSender
}

// NewMenuCallbacks 创建菜单回调处理器
func NewMenuCallbacks(downloadService contracts.DownloadService, config *config.Config, messageUtils types.MessageSender) *MenuCallbacks {
	return &MenuCallbacks{
		downloadService: downloadService,
		config:          config,
		messageUtils:    messageUtils,
	}
}

// HandleStartWithEdit 处理开始命令（支持消息编辑）
func (mc *MenuCallbacks) HandleStartWithEdit(chatID int64, messageID int) {
	message := "<b>欢迎使用 Alist-Aria2 下载管理器</b>\n\n" +
		"<b>功能模块:</b>\n" +
		"• 下载管理 - 创建、监控、控制下载任务\n" +
		"• 文件浏览 - 浏览和搜索Alist文件\n" +
		"• 系统管理 - 登录、健康检查、设置\n" +
		"• 状态监控 - 实时状态和下载统计\n\n" +
		"选择功能模块开始使用："

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

	mc.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
}

// HandleHelpWithEdit 处理帮助命令（支持消息编辑）
func (mc *MenuCallbacks) HandleHelpWithEdit(chatID int64, messageID int) {
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

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("系统状态", "cmd_status"),
			tgbotapi.NewInlineKeyboardButtonData("管理面板", "cmd_manage"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("返回主菜单", "back_main"),
		),
	)

	mc.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
}

// HandleStatusWithEdit 处理状态命令（支持消息编辑）
func (mc *MenuCallbacks) HandleStatusWithEdit(chatID int64, messageID int) {
	ctx := context.Background()
	status, err := mc.downloadService.GetSystemStatus(ctx)
	if err != nil {
		message := "获取系统状态失败: " + err.Error()
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("返回主菜单", "back_main"),
			),
		)
		mc.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
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

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("刷新状态", "cmd_status"),
			tgbotapi.NewInlineKeyboardButtonData("返回主菜单", "back_main"),
		),
	)

	mc.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
}

// HandleManageWithEdit 处理管理面板（支持消息编辑）
func (mc *MenuCallbacks) HandleManageWithEdit(chatID int64, messageID int) {
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
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("返回主菜单", "back_main"),
		),
	)

	mc.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
}

// HandleDownloadMenuWithEdit 处理下载管理菜单（支持消息编辑）
func (mc *MenuCallbacks) HandleDownloadMenuWithEdit(chatID int64, messageID int) {
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

	mc.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
}

// HandleFilesMenuWithEdit 处理文件浏览菜单（支持消息编辑）
func (mc *MenuCallbacks) HandleFilesMenuWithEdit(chatID int64, messageID int) {
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

	mc.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
}

// HandleSystemMenuWithEdit 处理系统管理菜单（支持消息编辑）
func (mc *MenuCallbacks) HandleSystemMenuWithEdit(chatID int64, messageID int) {
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

	mc.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
}

// HandleStatusMenuWithEdit 处理状态监控菜单（支持消息编辑）
func (mc *MenuCallbacks) HandleStatusMenuWithEdit(chatID int64, messageID int) {
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

	mc.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
}

// HandleSystemInfoWithEdit 处理系统信息（支持消息编辑）
func (mc *MenuCallbacks) HandleSystemInfoWithEdit(chatID int64, messageID int) {
	message := "<b>系统信息</b>\n\n" +
		"<b>服务状态:</b>\n" +
		"• 服务器运行状态: 正常\n" +
		"• Telegram Bot: 已连接\n" +
		"• 配置加载状态: 正常\n\n" +
		"<b>版本信息:</b>\n" +
		"• 应用版本: v1.0.0\n" +
		"• Go 版本: " + runtime.Version() + "\n" +
		"• 构建时间: " + fmt.Sprintf("%d", 2024) + "\n\n"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("刷新信息", "system_info"),
			tgbotapi.NewInlineKeyboardButtonData("健康检查", "api_health_check"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("返回系统管理", "menu_system"),
		),
	)

	mc.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
}