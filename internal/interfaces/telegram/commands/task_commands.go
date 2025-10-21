package commands

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/easayliu/alist-aria2-download/internal/interfaces/telegram/types"
	"github.com/easayliu/alist-aria2-download/internal/interfaces/telegram/utils"
	"github.com/easayliu/alist-aria2-download/internal/application/services/task"
	"github.com/easayliu/alist-aria2-download/internal/domain/entities"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/config"
)

// TaskCommands handles scheduled task commands
type TaskCommands struct {
	schedulerService *task.SchedulerService
	config           *config.Config
	messageUtils     types.MessageSender
}

// NewTaskCommands creates a scheduled task command handler
func NewTaskCommands(schedulerService *task.SchedulerService, config *config.Config, messageUtils types.MessageSender) *TaskCommands {
	return &TaskCommands{
		schedulerService: schedulerService,
		config:           config,
		messageUtils:     messageUtils,
	}
}

// HandleTasks handles viewing scheduled tasks
func (tc *TaskCommands) HandleTasks(chatID int64, userID int64) {
	if tc.schedulerService == nil {
		tc.messageUtils.SendMessage(chatID, "定时任务服务未启用")
		return
	}

	tasks, err := tc.schedulerService.GetUserTasks(userID)
	if err != nil {
		formatter := tc.messageUtils.GetFormatter().(*utils.MessageFormatter)
		tc.messageUtils.SendMessage(chatID, formatter.FormatError("获取任务", err))
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
		tc.messageUtils.SendMessageHTML(chatID, message)
		return
	}

	formatter := tc.messageUtils.GetFormatter().(*utils.MessageFormatter)
	message := formatter.FormatTitle("⏰", fmt.Sprintf("定时任务 (%d个)", len(tasks))) + "\n\n"

	for i, task := range tasks {
		status := "禁用"
		if task.Enabled {
			status = "启用"
		}

		// Calculate time description
		timeDesc := tc.formatTaskTimeDescription(task.HoursAgo)

		message += fmt.Sprintf(
			"<b>%d. %s</b> %s\n"+
				"   ID: <code>%s</code>\n"+
				"   Cron: <code>%s</code>\n"+
				"   路径: <code>%s</code>\n"+
				"   时间范围: 最近<b>%s</b>内修改的文件\n"+
				"   文件类型: %s\n",
			i+1, tc.messageUtils.EscapeHTML(task.Name), status,
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

	tc.messageUtils.SendMessageHTML(chatID, message)
}

// HandleAddTask handles adding a scheduled task
func (tc *TaskCommands) HandleAddTask(chatID int64, userID int64, command string) {
	if tc.schedulerService == nil {
		tc.messageUtils.SendMessage(chatID, "定时任务服务未启用")
		return
	}

	parts := strings.Fields(command)
	if len(parts) < 5 { // Minimum 5 parameters required (path is optional)
		tc.sendAddTaskHelp(chatID)
		return
	}

	// Parse parameters - need to handle cron expressions that may contain spaces
	name := parts[1]

	var cron, path string
	var hoursAgo int
	var videoOnly bool

	// Last two parameters are always hoursAgo and videoOnly
	videoOnly = parts[len(parts)-1] == "true"
	hoursAgo, _ = strconv.Atoi(parts[len(parts)-2])

	// Check if third-to-last parameter is a path (starts with /) or a number (if number, path not provided)
	if len(parts) >= 6 && strings.HasPrefix(parts[len(parts)-3], "/") {
		// Has path parameter
		path = parts[len(parts)-3]
		// Middle parts are cron expression
		cronParts := parts[2 : len(parts)-3]
		cron = strings.Join(cronParts, " ")
	} else {
		// No path parameter, use default path
		path = tc.config.Alist.DefaultPath
		if path == "" {
			path = "/"
		}
		// Middle parts are cron expression
		cronParts := parts[2 : len(parts)-2]
		cron = strings.Join(cronParts, " ")
	}

	// Remove possible quotes
	cron = strings.Trim(cron, "\"'")

	// Create task
	task := &entities.ScheduledTask{
		Name:      name,
		Enabled:   true,
		Cron:      cron,
		Path:      path,
		HoursAgo:  hoursAgo,
		VideoOnly: videoOnly,
		CreatedBy: userID,
	}

	if err := tc.schedulerService.CreateTask(task); err != nil {
		formatter := tc.messageUtils.GetFormatter().(*utils.MessageFormatter)
		tc.messageUtils.SendMessage(chatID, formatter.FormatError("创建任务", err))
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
		tc.messageUtils.EscapeHTML(name), task.ID[:8], cron, path, hoursAgo, videoOnly, task.ID[:8],
	)

	tc.messageUtils.SendMessageHTML(chatID, message)
}

// HandleQuickTask handles quick scheduled task creation
func (tc *TaskCommands) HandleQuickTask(chatID int64, userID int64, command string) {
	if tc.schedulerService == nil {
		tc.messageUtils.SendMessage(chatID, "定时任务服务未启用")
		return
	}

	parts := strings.Fields(command)
	if len(parts) < 2 {
		tc.sendQuickTaskHelp(chatID)
		return
	}

	taskType := parts[1]

	// Get path, use default path if not specified
	path := tc.config.Alist.DefaultPath
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
			Cron:      "0 2 * * *", // Every day at 2 AM
			Path:      path,
			HoursAgo:  24,
			VideoOnly: true,
			CreatedBy: userID,
		}
	case "recent", "频繁":
		task = &entities.ScheduledTask{
			Name:      fmt.Sprintf("频繁同步-%s", path),
			Enabled:   true,
			Cron:      "0 */2 * * *", // Every 2 hours
			Path:      path,
			HoursAgo:  2,
			VideoOnly: true,
			CreatedBy: userID,
		}
	case "weekly", "每周":
		task = &entities.ScheduledTask{
			Name:      fmt.Sprintf("每周汇总-%s", path),
			Enabled:   true,
			Cron:      "0 9 * * 1", // Every Monday at 9 AM
			Path:      path,
			HoursAgo:  168, // 7 days
			VideoOnly: true,
			CreatedBy: userID,
		}
	case "realtime", "实时":
		task = &entities.ScheduledTask{
			Name:      fmt.Sprintf("实时同步-%s", path),
			Enabled:   true,
			Cron:      "0 * * * *", // Every hour (on the hour)
			Path:      path,
			HoursAgo:  1,
			VideoOnly: true,
			CreatedBy: userID,
		}
	default:
		tc.messageUtils.SendMessage(chatID, "未知的任务类型\n可用类型: daily, recent, weekly, realtime")
		return
	}

	if err := tc.schedulerService.CreateTask(task); err != nil {
		formatter := tc.messageUtils.GetFormatter().(*utils.MessageFormatter)
		tc.messageUtils.SendMessage(chatID, formatter.FormatError("创建任务", err))
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
		tc.messageUtils.EscapeHTML(task.Name), path, timeDesc, task.ID[:8], task.ID[:8],
	)

	tc.messageUtils.SendMessageHTML(chatID, message)
}

// HandleDeleteTask handles deleting a scheduled task
func (tc *TaskCommands) HandleDeleteTask(chatID int64, userID int64, command string) {
	if tc.schedulerService == nil {
		tc.messageUtils.SendMessage(chatID, "定时任务服务未启用")
		return
	}

	parts := strings.Fields(command)
	if len(parts) < 2 {
		tc.messageUtils.SendMessage(chatID, "用法: /deltask &lt;任务ID&gt;\n示例: /deltask abc12345")
		return
	}

	taskID := parts[1]

	// Find complete task ID
	tasks, _ := tc.schedulerService.GetUserTasks(userID)
	var fullTaskID string
	for _, task := range tasks {
		if strings.HasPrefix(task.ID, taskID) {
			fullTaskID = task.ID
			break
		}
	}

	if fullTaskID == "" {
		tc.messageUtils.SendMessage(chatID, "未找到任务")
		return
	}

	if err := tc.schedulerService.DeleteTask(fullTaskID); err != nil {
		formatter := tc.messageUtils.GetFormatter().(*utils.MessageFormatter)
		tc.messageUtils.SendMessage(chatID, formatter.FormatError("删除任务", err))
		return
	}

	tc.messageUtils.SendMessage(chatID, "任务已删除")
}

// HandleRunTask handles running a scheduled task immediately
func (tc *TaskCommands) HandleRunTask(chatID int64, userID int64, command string) {
	if tc.schedulerService == nil {
		tc.messageUtils.SendMessage(chatID, "定时任务服务未启用")
		return
	}

	parts := strings.Fields(command)
	if len(parts) < 2 {
		tc.messageUtils.SendMessage(chatID, "用法: /runtask &lt;任务ID&gt;\n示例: /runtask abc12345")
		return
	}

	taskID := parts[1]

	// Find complete task ID
	tasks, _ := tc.schedulerService.GetUserTasks(userID)
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
		tc.messageUtils.SendMessage(chatID, "未找到任务")
		return
	}

	if err := tc.schedulerService.RunTaskNow(fullTaskID); err != nil {
		formatter := tc.messageUtils.GetFormatter().(*utils.MessageFormatter)
		tc.messageUtils.SendMessage(chatID, formatter.FormatError("运行任务", err))
		return
	}

	tc.messageUtils.SendMessage(chatID, fmt.Sprintf("任务 '%s' 已开始运行，请稍后查看结果", taskName))
}

// formatTaskTimeDescription formats task time description
func (tc *TaskCommands) formatTaskTimeDescription(hoursAgo int) string {
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

// sendAddTaskHelp sends add task help message
func (tc *TaskCommands) sendAddTaskHelp(chatID int64) {
	defaultPath := tc.config.Alist.DefaultPath
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
	
	tc.messageUtils.SendMessageHTML(chatID, message)
}

// sendQuickTaskHelp sends quick task help message
func (tc *TaskCommands) sendQuickTaskHelp(chatID int64) {
	defaultPath := tc.config.Alist.DefaultPath
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
	
	tc.messageUtils.SendMessageHTML(chatID, message)
}