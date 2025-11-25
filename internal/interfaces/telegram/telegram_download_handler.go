package telegram

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/easayliu/alist-aria2-download/internal/application/contracts"
	"github.com/easayliu/alist-aria2-download/internal/interfaces/telegram/utils"
	"github.com/easayliu/alist-aria2-download/pkg/logger"
	"github.com/easayliu/alist-aria2-download/pkg/utils/time"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// ManualDownloadContext manual download context (backward compatible)
type ManualDownloadContext struct {
	ChatID      int64
	Request     manualDownloadRequest
	Description string
	TimeArgs    []string
	CreatedAt   time.Time
}

// manualDownloadRequest manual download request (backward compatible)
type manualDownloadRequest struct {
	Path      string `json:"path"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
	VideoOnly bool   `json:"video_only"`
	Preview   bool   `json:"preview"`
}

// TimeParseResult time parsing result
type TimeParseResult struct {
	StartTime   time.Time
	EndTime     time.Time
	Description string
}

// DownloadHandler handles download-related functions
type DownloadHandler struct {
	controller *TelegramController

	// Manual download context management
	manualMutex    sync.Mutex
	manualContexts map[string]*ManualDownloadContext
}

// NewDownloadHandler creates a new download handler
func NewDownloadHandler(controller *TelegramController) *DownloadHandler {
	return &DownloadHandler{
		controller:     controller,
		manualContexts: make(map[string]*ManualDownloadContext),
	}
}

// ================================
// Time parsing and manual download core functions
// ================================

// parseTimeArguments parses time parameters
// Supported formats:
// 1. Number - hours (e.g., 48)
// 2. Minutes - number with 'm' suffix (e.g., 30m)
// 3. Date range - two dates (e.g., 2025-09-01 2025-09-26)
// 4. Time range - two timestamps (e.g., 2025-09-01T00:00:00Z 2025-09-26T23:59:59Z)
func (h *DownloadHandler) parseTimeArguments(args []string) (*TimeParseResult, error) {
	if len(args) == 0 {
		// 默认24小时
		timeRange := timeutil.CreateTimeRangeFromHours(24)
		return &TimeParseResult{
			StartTime:   timeRange.Start,
			EndTime:     timeRange.End,
			Description: "最近24小时",
		}, nil
	}

	if len(args) == 1 {
		arg := args[0]

		// 检查是否为分钟格式（以m结尾）
		if strings.HasSuffix(strings.ToLower(arg), "m") {
			minuteStr := strings.TrimSuffix(strings.ToLower(arg), "m")
			if minutes, err := strconv.Atoi(minuteStr); err == nil {
				if minutes <= 0 {
					return nil, fmt.Errorf("分钟数必须大于0")
				}
				if minutes > 525600 { // Minutes in a year
					return nil, fmt.Errorf("分钟数不能超过525600（一年）")
				}
				timeRange := timeutil.CreateTimeRangeFromMinutes(minutes)
				return &TimeParseResult{
					StartTime:   timeRange.Start,
					EndTime:     timeRange.End,
					Description: fmt.Sprintf("最近%d分钟", minutes),
				}, nil
			}
		}

		// 尝试解析为小时数
		if hours, err := parseHours(arg); err == nil {
			if hours <= 0 {
				return nil, fmt.Errorf("小时数必须大于0")
			}
			if hours > 8760 { // 一年的小时数
				return nil, fmt.Errorf("小时数不能超过8760（一年）")
			}
			timeRange := timeutil.CreateTimeRangeFromHours(hours)
			return &TimeParseResult{
				StartTime:   timeRange.Start,
				EndTime:     timeRange.End,
				Description: fmt.Sprintf("最近%d小时", hours),
			}, nil
		}

		return nil, fmt.Errorf("无效的时间格式，应为小时数（如：48）或分钟数（如：30m）")
	}

	if len(args) == 2 {
		startStr, endStr := args[0], args[1]

		// 使用统一的时间解析工具
		timeRange, err := timeutil.ParseTimeRange(startStr, endStr)
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

	return nil, fmt.Errorf("参数过多，支持的格式：\n• /download\n• /download 30m\n• /download 48\n• /download 2025-09-01 2025-09-26\n• /download 2025-09-01T00:00:00Z 2025-09-26T23:59:59Z")
}

// handleManualDownload handles manual download function with time range parameters
func (h *DownloadHandler) handleManualDownload(chatID int64, timeArgs []string, preview bool) {
	// 解析时间参数
	timeResult, err := h.parseTimeArguments(timeArgs)
	if err != nil {
		formatter := h.controller.messageUtils.GetFormatter().(*utils.MessageFormatter)
		message := formatter.FormatTimeRangeHelp(err.Error())
		h.controller.messageUtils.SendMessageHTML(chatID, message)
		return
	}

	modeLabel := "下载"
	if preview {
		modeLabel = "预览"
	}

	formatter := h.controller.messageUtils.GetFormatter().(*utils.MessageFormatter)
	processingMsg := formatter.FormatTitle("⏳", fmt.Sprintf("正在处理手动%s任务", modeLabel)) + "\n\n" +
		formatter.FormatField("时间范围", timeResult.Description)
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
		formatter := h.controller.messageUtils.GetFormatter().(*utils.MessageFormatter)
		h.controller.messageUtils.SendMessage(chatID, formatter.FormatError("处理", err))
		return
	}

	files := timeRangeResp.Files

	if len(files) == 0 {
		formatter := h.controller.messageUtils.GetFormatter().(*utils.MessageFormatter)
		var title string
		if preview {
			title = "手动下载预览"
		} else {
			title = "手动下载完成"
		}
		message := formatter.FormatNoFilesFound(title, timeResult.Description)
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

		// 准备示例文件
		var exampleFiles []utils.ExampleFileData
		maxExamples := 5
		if len(files) < maxExamples {
			maxExamples = len(files)
		}
		for i := 0; i < maxExamples; i++ {
			file := files[i]
			filename := file.Name
			runes := []rune(filename)
			if len(runes) > 60 {
				filename = string(runes[:60]) + "..."
			}
			exampleFiles = append(exampleFiles, utils.ExampleFileData{
				Name:         filename,
				DownloadPath: file.DownloadPath,
			})
		}

		// 使用统一格式化器
		formatter := h.controller.messageUtils.GetFormatter().(*utils.MessageFormatter)
		message := formatter.FormatTimeRangeDownloadPreview(utils.TimeRangeDownloadPreviewData{
			TimeDescription: timeResult.Description,
			Path:            path,
			TotalFiles:      totalFiles,
			TotalSize:       totalSizeStr,
			MovieCount:      mediaStats.Movie,
			TVCount:         mediaStats.TV,
			OtherCount:      mediaStats.Other,
			ExampleFiles:    exampleFiles,
			ConfirmCommand:  confirmCommand,
			EscapeHTML:      h.controller.messageUtils.EscapeHTML,
		})

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
				URL:          file.InternalURL,
				Filename:     file.Name,
				Directory:    file.DownloadPath,
				AutoClassify: true,
			}

			_, err := h.controller.downloadService.CreateDownload(ctx, downloadReq)
			if err != nil {
				failCount++
				failedFiles = append(failedFiles, file.Name)
				logger.Error("Failed to create download task", "file", file.Name, "error", err)
				continue
			}
			successCount++
		}

		// 使用统一格式化器
		formatter := h.controller.messageUtils.GetFormatter().(*utils.MessageFormatter)
		message := formatter.FormatTimeRangeDownloadResult(utils.TimeRangeDownloadResultData{
			TimeDescription: timeResult.Description,
			Path:            path,
			TotalFiles:      totalFiles,
			TotalSize:       totalSizeStr,
			MovieCount:      mediaStats.Movie,
			TVCount:         mediaStats.TV,
			OtherCount:      mediaStats.Other,
			SuccessCount:    successCount,
			FailCount:       failCount,
			EscapeHTML:      h.controller.messageUtils.EscapeHTML,
		})

		h.controller.messageUtils.SendMessageHTML(chatID, message)
		return
	}
}

// HandleQuickPreview handles quick preview
func (h *DownloadHandler) HandleQuickPreview(chatID int64, timeArgs []string) {
	h.handleManualDownload(chatID, timeArgs, true)
}

// ================================
// Manual download context management
// ================================

// storeManualContext stores manual download context
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

// getManualContext retrieves manual download context
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

// deleteManualContext deletes manual download context
func (h *DownloadHandler) deleteManualContext(token string) {
	h.manualMutex.Lock()
	delete(h.manualContexts, token)
	h.manualMutex.Unlock()
}

// cleanupManualContexts cleans up expired manual download contexts
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

// HandleManualConfirm handles manual download confirmation
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
	startTime, err := timeutil.ParseTime(req.StartTime)
	if err != nil {
		formatter := h.controller.messageUtils.GetFormatter().(*utils.MessageFormatter)
		h.controller.messageUtils.SendMessage(chatID, formatter.FormatError("时间解析", err))
		return
	}
	endTime, err := timeutil.ParseTime(req.EndTime)
	if err != nil {
		formatter := h.controller.messageUtils.GetFormatter().(*utils.MessageFormatter)
		h.controller.messageUtils.SendMessage(chatID, formatter.FormatError("时间解析", err))
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
		formatter := h.controller.messageUtils.GetFormatter().(*utils.MessageFormatter)
		h.controller.messageUtils.SendMessage(chatID, formatter.FormatError("创建下载任务", err))
		return
	}

	files := timeRangeResp.Files

	if len(files) == 0 {
		formatter := h.controller.messageUtils.GetFormatter().(*utils.MessageFormatter)
		message := formatter.FormatNoFilesFound("手动下载完成", ctx.Description)
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
			URL:          file.InternalURL,
			Filename:     file.Name,
			Directory:    file.DownloadPath,
			AutoClassify: true,
		}

		_, err := h.controller.downloadService.CreateDownload(requestCtx, downloadReq)
		if err != nil {
			failCount++
			failedFiles = append(failedFiles, file.Name)
			logger.Error("Failed to create download task", "file", file.Name, "error", err)
			continue
		}
		successCount++
	}

	// 使用统一格式化器
	formatter := h.controller.messageUtils.GetFormatter().(*utils.MessageFormatter)
	message := formatter.FormatTimeRangeDownloadResult(utils.TimeRangeDownloadResultData{
		TimeDescription: ctx.Description,
		Path:            req.Path,
		TotalFiles:      totalFiles,
		TotalSize:       totalSizeStr,
		MovieCount:      mediaStats.Movie,
		TVCount:         mediaStats.TV,
		OtherCount:      mediaStats.Other,
		SuccessCount:    successCount,
		FailCount:       failCount,
		EscapeHTML:      h.controller.messageUtils.EscapeHTML,
	})

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

// ================================
// 辅助函数
// ================================

func parseHours(s string) (int, error) {
	var hours int
	_, err := fmt.Sscanf(s, "%d", &hours)
	return hours, err
}
