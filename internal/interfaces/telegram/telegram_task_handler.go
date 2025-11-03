package telegram

import (
	"fmt"

	"github.com/easayliu/alist-aria2-download/internal/interfaces/telegram/utils"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// TaskHandler handles task management related functions
type TaskHandler struct {
	controller *TelegramController
}

// NewTaskHandler creates a new task handler
func NewTaskHandler(controller *TelegramController) *TaskHandler {
	return &TaskHandler{
		controller: controller,
	}
}

// ================================
// Task Management Functions
// ================================

// HandleTasksWithEdit handles viewing scheduled tasks (supports message editing)
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
		formatter := h.controller.messageUtils.GetFormatter().(*utils.MessageFormatter)
		message := formatter.FormatError("获取任务", err)
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

	// 构建任务数据
	var taskItems []utils.TaskItemData
	for _, task := range tasks {
		statusEmoji := "⏸️"
		status := "禁用"
		if task.Enabled {
			statusEmoji = "✅"
			status = "启用"
		}

		// Calculate time description
		timeDesc := h.formatTaskTimeDescription(task.HoursAgo)
		schedule := fmt.Sprintf("%s (最近%s)", task.Cron, timeDesc)

		lastRun := ""
		if task.LastRunAt != nil {
			lastRun = task.LastRunAt.Format("01-02 15:04")
		}

		nextRun := ""
		if task.NextRunAt != nil {
			nextRun = task.NextRunAt.Format("01-02 15:04")
		}

		taskItems = append(taskItems, utils.TaskItemData{
			ID:          task.ID[:8],
			Name:        h.controller.messageUtils.EscapeHTML(task.Name),
			Schedule:    schedule,
			Status:      status,
			StatusEmoji: statusEmoji,
			LastRun:     lastRun,
			NextRun:     nextRun,
		})
	}

	// Use unified formatter
	formatter := h.controller.messageUtils.GetFormatter().(*utils.MessageFormatter)
	listData := utils.TaskListData{
		TotalCount: len(tasks),
		Tasks:      taskItems,
	}
	message := formatter.FormatTaskList(listData)

	// Add command instructions
	message += "\n\n" + formatter.FormatSection("命令")
	message += "\n" + formatter.FormatListItem("•", "立即运行: <code>/runtask ID</code>")
	message += "\n" + formatter.FormatListItem("•", "删除任务: <code>/deltask ID</code>")
	message += "\n" + formatter.FormatListItem("•", "添加任务: <code>/addtask</code> 查看帮助")

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("刷新任务", "cmd_tasks"),
			tgbotapi.NewInlineKeyboardButtonData("返回管理面板", "cmd_manage"),
		),
	)

	h.controller.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
}

// formatTaskTimeDescription formats task time description
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
