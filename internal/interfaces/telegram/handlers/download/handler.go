// Package download provides handlers for download-related Telegram operations.
// It handles download preview, confirmation, and batch download management.
package download

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
	timeutil "github.com/easayliu/alist-aria2-download/pkg/utils/time"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// ManualDownloadContext manual download context
type ManualDownloadContext struct {
	ChatID      int64
	Request     manualDownloadRequest
	Description string
	TimeArgs    []string
	CreatedAt   time.Time
}

// manualDownloadRequest manual download request
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

// Handler handles download-related functions
type Handler struct {
	deps DownloadDeps

	// Manual download context management
	manualMutex    sync.Mutex
	manualContexts map[string]*ManualDownloadContext
}

// NewHandler creates a new download handler
func NewHandler(deps DownloadDeps) *Handler {
	return &Handler{
		deps:           deps,
		manualContexts: make(map[string]*ManualDownloadContext),
	}
}

// ================================
// Time parsing and manual download core functions
// ================================

// parseTimeArguments parses time parameters
func (h *Handler) parseTimeArguments(args []string) (*TimeParseResult, error) {
	if len(args) == 0 {
		timeRange := timeutil.CreateTimeRangeFromHours(24)
		return &TimeParseResult{
			StartTime:   timeRange.Start,
			EndTime:     timeRange.End,
			Description: "最近24小时",
		}, nil
	}

	if len(args) == 1 {
		arg := args[0]

		if strings.HasSuffix(strings.ToLower(arg), "m") {
			minuteStr := strings.TrimSuffix(strings.ToLower(arg), "m")
			if minutes, err := strconv.Atoi(minuteStr); err == nil {
				if minutes <= 0 {
					return nil, fmt.Errorf("分钟数必须大于0")
				}
				if minutes > 525600 {
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

		if hours, err := parseHours(arg); err == nil {
			if hours <= 0 {
				return nil, fmt.Errorf("小时数必须大于0")
			}
			if hours > 8760 {
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

		timeRange, err := timeutil.ParseTimeRange(startStr, endStr)
		if err != nil {
			return nil, fmt.Errorf("无效的时间格式，支持的格式：\n• 日期范围：2025-09-01 2025-09-26\n• 时间范围：2025-09-01T00:00:00Z 2025-09-26T23:59:59Z")
		}

		description := fmt.Sprintf("从 %s 到 %s", timeRange.Start.Format("2006-01-02 15:04"), timeRange.End.Format("2006-01-02 15:04"))
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

// HandleManualDownload handles manual download function with time range parameters
func (h *Handler) HandleManualDownload(chatID int64, timeArgs []string, preview bool) {
	msgUtils := h.deps.GetMessageUtils()

	timeResult, err := h.parseTimeArguments(timeArgs)
	if err != nil {
		formatter := msgUtils.GetFormatter().(*utils.MessageFormatter)
		message := formatter.FormatTimeRangeHelp(err.Error())
		msgUtils.SendMessageHTML(chatID, message)
		return
	}

	path := ""
	if h.deps.GetConfig().Alist.DefaultPath != "" {
		path = h.deps.GetConfig().Alist.DefaultPath
	}
	if path == "" {
		path = "/"
	}

	timeRangeReq := contracts.TimeRangeFileRequest{
		Path:      path,
		StartTime: timeResult.StartTime,
		EndTime:   timeResult.EndTime,
		VideoOnly: true,
	}

	ctx := context.Background()
	timeRangeResp, err := h.deps.GetFileService().GetFilesByTimeRange(ctx, timeRangeReq)
	if err != nil {
		formatter := msgUtils.GetFormatter().(*utils.MessageFormatter)
		msgUtils.SendMessage(chatID, formatter.FormatError("处理", err))
		return
	}

	files := timeRangeResp.Files

	if len(files) == 0 {
		formatter := msgUtils.GetFormatter().(*utils.MessageFormatter)
		var title string
		if preview {
			title = "ℹ️ 手动下载预览"
		} else {
			title = "手动下载完成"
		}
		message := formatter.FormatTitle(title, "") + "\n\n" +
			formatter.FormatField("时间范围", timeResult.Description) + "\n" +
			formatter.FormatField("结果", "未找到符合条件的文件")
		msgUtils.SendMessageHTMLWithAutoDelete(chatID, message, 30)
		return
	}

	summary := timeRangeResp.Summary
	totalFiles := summary.TotalFiles
	totalSizeStr := summary.TotalSizeFormatted

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

		formatter := msgUtils.GetFormatter().(*utils.MessageFormatter)
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
			EscapeHTML:      msgUtils.EscapeHTML,
		})

		storedReq := manualDownloadRequest{
			Path:      path,
			StartTime: timeResult.StartTime.Format(time.RFC3339),
			EndTime:   timeResult.EndTime.Format(time.RFC3339),
			VideoOnly: true,
			Preview:   false,
		}

		manualCtx := &ManualDownloadContext{
			ChatID:      chatID,
			Request:     storedReq,
			Description: timeResult.Description,
			TimeArgs:    append([]string(nil), timeArgs...),
		}
		token := h.storeManualContext(manualCtx)

		confirmData := fmt.Sprintf("manual_confirm|%s", token)
		cancelData := fmt.Sprintf("manual_cancel|%s", token)

		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("✅ 确认开始下载", confirmData),
				tgbotapi.NewInlineKeyboardButtonData("✖️ 取消", cancelData),
			),
		)

		messageID := msgUtils.SendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
		if messageID > 0 {
			msgUtils.DeleteMessageAfterDelay(chatID, messageID, 30)
		}
		return
	}

	if !preview {
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

			_, err := h.deps.GetDownloadService().CreateDownload(ctx, downloadReq)
			if err != nil {
				failCount++
				failedFiles = append(failedFiles, file.Name)
				logger.Error("Failed to create download task", "file", file.Name, "error", err)
				continue
			}
			successCount++
		}

		formatter := msgUtils.GetFormatter().(*utils.MessageFormatter)
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
			EscapeHTML:      msgUtils.EscapeHTML,
		})

		msgUtils.SendMessageHTMLWithAutoDelete(chatID, message, 30)
	}
}

// HandleQuickPreview handles quick preview
func (h *Handler) HandleQuickPreview(chatID int64, timeArgs []string) {
	h.HandleManualDownload(chatID, timeArgs, true)
}

// ================================
// Manual download context management
// ================================

func (h *Handler) storeManualContext(ctx *ManualDownloadContext) string {
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

// GetManualContext retrieves manual download context
func (h *Handler) GetManualContext(token string) (*ManualDownloadContext, bool) {
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

// DeleteManualContext deletes manual download context
func (h *Handler) DeleteManualContext(token string) {
	h.manualMutex.Lock()
	delete(h.manualContexts, token)
	h.manualMutex.Unlock()
}

func (h *Handler) cleanupManualContexts() {
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
func (h *Handler) HandleManualConfirm(chatID int64, token string, messageID int) {
	msgUtils := h.deps.GetMessageUtils()

	ctx, ok := h.GetManualContext(token)
	if !ok {
		msgUtils.SendMessage(chatID, "预览已过期，请重新生成")
		return
	}

	if ctx.ChatID != chatID {
		msgUtils.SendMessage(chatID, "无效的确认请求")
		return
	}

	h.DeleteManualContext(token)
	msgUtils.ClearInlineKeyboard(chatID, messageID)

	msgUtils.SendMessageWithAutoDelete(chatID, "正在创建下载任务...", 30)

	req := ctx.Request

	startTime, err := timeutil.ParseTime(req.StartTime)
	if err != nil {
		formatter := msgUtils.GetFormatter().(*utils.MessageFormatter)
		msgUtils.SendMessage(chatID, formatter.FormatError("时间解析", err))
		return
	}
	endTime, err := timeutil.ParseTime(req.EndTime)
	if err != nil {
		formatter := msgUtils.GetFormatter().(*utils.MessageFormatter)
		msgUtils.SendMessage(chatID, formatter.FormatError("时间解析", err))
		return
	}

	timeRangeReq := contracts.TimeRangeFileRequest{
		Path:      req.Path,
		StartTime: startTime,
		EndTime:   endTime,
		VideoOnly: req.VideoOnly,
	}

	requestCtx := context.Background()
	timeRangeResp, err := h.deps.GetFileService().GetFilesByTimeRange(requestCtx, timeRangeReq)
	if err != nil {
		formatter := msgUtils.GetFormatter().(*utils.MessageFormatter)
		msgUtils.SendMessage(chatID, formatter.FormatError("创建下载任务", err))
		return
	}

	files := timeRangeResp.Files

	if len(files) == 0 {
		formatter := msgUtils.GetFormatter().(*utils.MessageFormatter)
		message := formatter.FormatNoFilesFound("手动下载完成", ctx.Description)
		msgUtils.SendMessageHTML(chatID, message)
		return
	}

	summary := timeRangeResp.Summary
	totalFiles := summary.TotalFiles
	totalSizeStr := summary.TotalSizeFormatted

	mediaStats := struct {
		TV    int
		Movie int
		Other int
	}{
		TV:    summary.TVFiles,
		Movie: summary.MovieFiles,
		Other: summary.OtherFiles,
	}

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

		_, err := h.deps.GetDownloadService().CreateDownload(requestCtx, downloadReq)
		if err != nil {
			failCount++
			failedFiles = append(failedFiles, file.Name)
			logger.Error("Failed to create download task", "file", file.Name, "error", err)
			continue
		}
		successCount++
	}

	formatter := msgUtils.GetFormatter().(*utils.MessageFormatter)
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
		EscapeHTML:      msgUtils.EscapeHTML,
	})

	msgUtils.SendMessageHTMLWithAutoDelete(chatID, message, 30)
}

// HandleManualCancel handles manual download cancel
func (h *Handler) HandleManualCancel(chatID int64, token string, messageID int) {
	msgUtils := h.deps.GetMessageUtils()

	ctx, ok := h.GetManualContext(token)
	if ok && ctx.ChatID == chatID {
		h.DeleteManualContext(token)
	}

	msgUtils.ClearInlineKeyboard(chatID, messageID)
	msgUtils.SendMessageWithAutoDelete(chatID, "已取消此次下载预览", 30)
}

func parseHours(s string) (int, error) {
	var hours int
	_, err := fmt.Sscanf(s, "%d", &hours)
	return hours, err
}
