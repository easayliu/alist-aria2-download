package telegram

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/easayliu/alist-aria2-download/internal/application/contracts"
	"github.com/easayliu/alist-aria2-download/pkg/logger"
	timeutils "github.com/easayliu/alist-aria2-download/pkg/utils"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

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

// DownloadHandler 处理下载相关功能
type DownloadHandler struct {
	controller *TelegramController
	
	// 手动下载上下文管理
	manualMutex    sync.Mutex
	manualContexts map[string]*ManualDownloadContext
}

// NewDownloadHandler 创建新的下载处理器
func NewDownloadHandler(controller *TelegramController) *DownloadHandler {
	return &DownloadHandler{
		controller:     controller,
		manualContexts: make(map[string]*ManualDownloadContext),
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
func (h *DownloadHandler) parseTimeArguments(args []string) (*TimeParseResult, error) {
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
		if hours, err := parseHours(args[0]); err == nil {
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
func (h *DownloadHandler) handleManualDownload(chatID int64, timeArgs []string, preview bool) {
	// 解析时间参数
	timeResult, err := h.parseTimeArguments(timeArgs)
	if err != nil {
		message := fmt.Sprintf("<b>时间参数错误</b>\n\n%s\n\n<b>支持的格式：</b>\n• /download - 预览最近24小时\n• /download 48 - 预览最近48小时\n• /download 2025-09-01 2025-09-26 - 预览指定日期范围\n• /download 2025-09-01T00:00:00Z 2025-09-26T23:59:59Z - 预览精确时间范围\n\n<b>提示:</b> 在命令后添加 <code>confirm</code> 可直接开始下载", err.Error())
		h.controller.messageUtils.SendMessageHTML(chatID, message)
		return
	}

	modeLabel := "下载"
	if preview {
		modeLabel = "预览"
	}

	processingMsg := fmt.Sprintf("<b>正在处理手动%s任务</b>\n\n时间范围: %s", modeLabel, timeResult.Description)
	h.controller.messageUtils.SendMessageHTML(chatID, processingMsg)

	path := ""
	if h.controller.config.Alist.DefaultPath != "" {
		path = h.controller.config.Alist.DefaultPath
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
	timeRangeResp, err := h.controller.fileService.GetFilesByTimeRange(ctx, timeRangeReq)
	if err != nil {
		h.controller.messageUtils.SendMessage(chatID, fmt.Sprintf("处理失败: %s", err.Error()))
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
		h.controller.messageUtils.SendMessageHTML(chatID, message)
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
			h.controller.messageUtils.EscapeHTML(path),
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
				filename := h.controller.messageUtils.EscapeHTML(file.Name)
				runes := []rune(filename)
				if len(runes) > 60 {
					filename = string(runes[:60]) + "..."
				}
				downloadPath := h.controller.messageUtils.EscapeHTML(file.DownloadPath)
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

		h.controller.messageUtils.SendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
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
			
			_, err := h.controller.downloadService.CreateDownload(ctx, downloadReq)
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
			h.controller.messageUtils.EscapeHTML(path),
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

		h.controller.messageUtils.SendMessageHTML(chatID, message)
		return
	}
}

// HandleQuickPreview 处理快速预览
func (h *DownloadHandler) HandleQuickPreview(chatID int64, timeArgs []string) {
	h.handleManualDownload(chatID, timeArgs, true)
}

// ================================
// 手动下载上下文管理
// ================================

// storeManualContext 存储手动下载上下文
func (h *DownloadHandler) storeManualContext(ctx *ManualDownloadContext) string {
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
func (h *DownloadHandler) getManualContext(token string) (*ManualDownloadContext, bool) {
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
func (h *DownloadHandler) deleteManualContext(token string) {
	h.manualMutex.Lock()
	delete(h.manualContexts, token)
	h.manualMutex.Unlock()
}

// cleanupManualContexts 清理过期的手动下载上下文
func (h *DownloadHandler) cleanupManualContexts() {
	cutoff := time.Now().Add(-10 * time.Minute)
	h.manualMutex.Lock()
	for token, ctx := range h.manualContexts {
		if ctx.CreatedAt.Before(cutoff) {
			delete(h.manualContexts, token)
		}
	}
	h.manualMutex.Unlock()
}

// HandleManualConfirm 处理手动下载确认
func (h *DownloadHandler) HandleManualConfirm(chatID int64, token string, messageID int) {
	ctx, ok := h.getManualContext(token)
	if !ok {
		h.controller.messageUtils.SendMessage(chatID, "预览已过期，请重新生成")
		return
	}

	if ctx.ChatID != chatID {
		h.controller.messageUtils.SendMessage(chatID, "无效的确认请求")
		return
	}

	h.deleteManualContext(token)
	h.controller.messageUtils.ClearInlineKeyboard(chatID, messageID)

	h.controller.messageUtils.SendMessage(chatID, "正在创建下载任务...")

	req := ctx.Request

	// 使用统一的时间解析工具
	startTime, err := timeutils.ParseTime(req.StartTime)
	if err != nil {
		h.controller.messageUtils.SendMessage(chatID, fmt.Sprintf("时间解析失败: %v", err))
		return
	}
	endTime, err := timeutils.ParseTime(req.EndTime)
	if err != nil {
		h.controller.messageUtils.SendMessage(chatID, fmt.Sprintf("时间解析失败: %v", err))
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
	timeRangeResp, err := h.controller.fileService.GetFilesByTimeRange(requestCtx, timeRangeReq)
	if err != nil {
		h.controller.messageUtils.SendMessage(chatID, fmt.Sprintf("创建下载任务失败: %v", err))
		return
	}
	
	files := timeRangeResp.Files

	if len(files) == 0 {
		message := fmt.Sprintf("<b>手动下载完成</b>\n\n时间范围: %s\n\n<b>结果:</b> 未找到符合条件的文件", ctx.Description)
		h.controller.messageUtils.SendMessageHTML(chatID, message)
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
		
		_, err := h.controller.downloadService.CreateDownload(requestCtx, downloadReq)
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
		ctx.Description,
		h.controller.messageUtils.EscapeHTML(req.Path),
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

	h.controller.messageUtils.SendMessageHTML(chatID, message)
}

// HandleManualCancel 处理手动下载取消
func (h *DownloadHandler) HandleManualCancel(chatID int64, token string, messageID int) {
	ctx, ok := h.getManualContext(token)
	if ok && ctx.ChatID == chatID {
		h.deleteManualContext(token)
	}

	h.controller.messageUtils.ClearInlineKeyboard(chatID, messageID)
	h.controller.messageUtils.SendMessage(chatID, "已取消此次下载预览")
}

// ================================
// 下载管理功能
// ================================

// HandleDownloadCreateWithEdit 处理创建下载（支持消息编辑）
func (h *DownloadHandler) HandleDownloadCreateWithEdit(chatID int64, messageID int) {
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

	h.controller.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
}

// HandleDownloadControlWithEdit 处理下载控制（支持消息编辑）
func (h *DownloadHandler) HandleDownloadControlWithEdit(chatID int64, messageID int) {
	// 先获取下载列表数据
	ctx := context.Background()
	listReq := contracts.DownloadListRequest{
		Limit: 100, // 获取最近100个下载
	}
	downloads, err := h.controller.downloadService.ListDownloads(ctx, listReq)
	if err != nil {
		message := "获取下载状态失败: " + err.Error()
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("返回下载管理", "menu_download"),
			),
		)
		h.controller.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
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

	h.controller.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
}

// HandleDownloadDeleteWithEdit 处理删除下载（支持消息编辑）
func (h *DownloadHandler) HandleDownloadDeleteWithEdit(chatID int64, messageID int) {
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

	h.controller.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
}

// ================================
// 辅助函数
// ================================

func parseHours(s string) (int, error) {
	var hours int
	_, err := fmt.Sscanf(s, "%d", &hours)
	return hours, err
}