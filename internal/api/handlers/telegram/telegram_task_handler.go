package telegram

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// TaskHandler 处理任务管理相关功能
type TaskHandler struct {
	controller *TelegramController
}

// NewTaskHandler 创建新的任务处理器
func NewTaskHandler(controller *TelegramController) *TaskHandler {
	return &TaskHandler{
		controller: controller,
	}
}

// ================================
// 任务管理功能
// ================================

// HandleTasksWithEdit 处理查看定时任务（支持消息编辑）
func (h *TaskHandler) HandleTasksWithEdit(chatID int64, userID int64, messageID int) {
	if h.controller.schedulerService == nil {
		message := "定时任务服务未启用"
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("返回主菜单", "back_main"),
			),
		)
		h.controller.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
		return
	}

	tasks, err := h.controller.schedulerService.GetUserTasks(userID)
	if err != nil {
		message := fmt.Sprintf("获取任务失败: %v", err)
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("返回主菜单", "back_main"),
			),
		)
		h.controller.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
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
		h.controller.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
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
			i+1, h.controller.messageUtils.EscapeHTML(task.Name), status,
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

	h.controller.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
}

// formatTaskTimeDescription 格式化任务时间描述
func (h *TaskHandler) formatTaskTimeDescription(hoursAgo int) string {
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