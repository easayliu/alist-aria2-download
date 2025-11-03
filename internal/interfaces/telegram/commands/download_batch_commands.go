package commands

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/easayliu/alist-aria2-download/internal/application/contracts"
	"github.com/easayliu/alist-aria2-download/internal/interfaces/telegram/utils"
	"github.com/easayliu/alist-aria2-download/pkg/utils/time"
)

// TimeParseResult represents the result of time parsing
type TimeParseResult struct {
	StartTime   time.Time
	EndTime     time.Time
	Description string
}

// handleManualDownload handles manual download functionality
func (dc *DownloadCommands) handleManualDownload(ctx context.Context, chatID int64, timeArgs []string, preview bool) {
	// Parse time parameters
	timeResult, err := dc.parseTimeArguments(timeArgs)
	if err != nil {
		formatter := dc.messageUtils.GetFormatter().(*utils.MessageFormatter)
		message := formatter.FormatTimeRangeHelp(err.Error())
		dc.messageUtils.SendMessageHTML(chatID, message)
		return
	}

	modeLabel := "下载"
	if preview {
		modeLabel = "预览"
	}

	formatter := dc.messageUtils.GetFormatter().(*utils.MessageFormatter)
	processingMsg := formatter.FormatTitle("⏳", fmt.Sprintf("正在处理手动%s任务", modeLabel)) + "\n\n" +
		formatter.FormatField("时间范围", timeResult.Description)
	dc.messageUtils.SendMessageHTML(chatID, processingMsg)

	// Get configured default path
	config := dc.container.GetConfig()
	path := config.Alist.DefaultPath
	if path == "" {
		path = "/"
	}

	// Build time range file request
	req := contracts.TimeRangeFileRequest{
		Path:      path,
		StartTime: timeResult.StartTime,
		EndTime:   timeResult.EndTime,
		VideoOnly: true, // Only process video files
	}

	// Call application service to get files by time range
	fileService := dc.container.GetFileService()
	response, err := fileService.GetFilesByTimeRange(ctx, req)
	if err != nil {
		formatter := dc.messageUtils.GetFormatter().(*utils.MessageFormatter)
		dc.messageUtils.SendMessage(chatID, formatter.FormatError("处理", err))
		return
	}

	if len(response.Files) == 0 {
		formatter := dc.messageUtils.GetFormatter().(*utils.MessageFormatter)
		var title string
		if preview {
			title = "手动下载预览"
		} else {
			title = "手动下载完成"
		}
		message := formatter.FormatNoFilesFound(title, timeResult.Description)
		dc.messageUtils.SendMessageHTML(chatID, message)
		return
	}

	if preview {
		// Preview mode: display file info and confirmation button
		dc.sendManualDownloadPreview(chatID, response, timeResult, timeArgs)
	} else {
		// Direct download mode: create download tasks
		dc.executeManualDownload(ctx, chatID, response, timeResult)
	}
}

// parseTimeArguments parses time parameters
// Supported formats:
// 1. Number - hours (e.g., 48)
// 2. Date range - two dates (e.g., 2025-09-01 2025-09-26)
// 3. Time range - two timestamps (e.g., 2025-09-01T00:00:00Z 2025-09-26T23:59:59Z)
func (dc *DownloadCommands) parseTimeArguments(args []string) (*TimeParseResult, error) {
	if len(args) == 0 {
		// Default 24 hours
		timeRange := timeutil.CreateTimeRangeFromHours(24)
		return &TimeParseResult{
			StartTime:   timeRange.Start,
			EndTime:     timeRange.End,
			Description: "最近24小时",
		}, nil
	}

	if len(args) == 1 {
		// Try parsing as hours
		if hours, err := strconv.Atoi(args[0]); err == nil {
			if hours <= 0 {
				return nil, fmt.Errorf("小时数必须大于0")
			}
			if hours > 8760 { // Hours in a year
				return nil, fmt.Errorf("小时数不能超过8760（一年）")
			}
			timeRange := timeutil.CreateTimeRangeFromHours(hours)
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

		// Use unified time parsing utility
		timeRange, err := timeutil.ParseTimeRange(startStr, endStr)
		if err != nil {
			return nil, fmt.Errorf("无效的时间格式，支持的格式：\n• 日期范围：2025-09-01 2025-09-26\n• 时间范围：2025-09-01T00:00:00Z 2025-09-26T23:59:59Z")
		}

		// Generate description based on time format
		description := fmt.Sprintf("从 %s 到 %s", timeRange.Start.Format("2006-01-02 15:04"), timeRange.End.Format("2006-01-02 15:04"))
		// If date format (time is 00:00), use date format description
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

// sendManualDownloadPreview sends manual download preview
func (dc *DownloadCommands) sendManualDownloadPreview(chatID int64, response *contracts.TimeRangeFileResponse, timeResult *TimeParseResult, timeArgs []string) {
	// Get configured default path
	config := dc.container.GetConfig()
	path := config.Alist.DefaultPath
	if path == "" {
		path = "/"
	}

	// Build preview message
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
		dc.messageUtils.EscapeHTML(path),
		response.Summary.TotalFiles,
		response.Summary.TotalSizeFormatted,
		response.Summary.MovieFiles,
		response.Summary.TVFiles,
		response.Summary.OtherFiles,
	)

	if len(response.Files) > 0 {
		message += "\n\n<b>示例文件:</b>\n"
		displayCount := len(response.Files)
		if displayCount > 5 {
			displayCount = 5
		}
		for i := 0; i < displayCount; i++ {
			file := response.Files[i]
			filename := dc.messageUtils.EscapeHTML(file.Name)
			// Limit filename length
			if len([]rune(filename)) > 40 {
				runes := []rune(filename)
				filename = string(runes[:40]) + "..."
			}
			downloadPath := dc.messageUtils.EscapeHTML(file.DownloadPath)
			message += fmt.Sprintf("• %s → <code>%s</code>\n", filename, downloadPath)
		}
		if len(response.Files) > 5 {
			message += fmt.Sprintf("• ... 还有 %d 个文件\n", len(response.Files)-5)
		}
	}

	// Build confirmation command
	confirmCommand := "/download confirm"
	if len(timeArgs) > 0 {
		confirmCommand += " " + strings.Join(timeArgs, " ")
	}

	message += fmt.Sprintf("\n\n⚠️ 预览有效期 10 分钟。发送 <code>%s</code> 开始下载。", confirmCommand)

	dc.messageUtils.SendMessageHTML(chatID, message)
}

// executeManualDownload executes manual download
func (dc *DownloadCommands) executeManualDownload(ctx context.Context, chatID int64, response *contracts.TimeRangeFileResponse, timeResult *TimeParseResult) {
	if len(response.Files) == 0 {
		formatter := dc.messageUtils.GetFormatter().(*utils.MessageFormatter)
		message := formatter.FormatNoFilesFound("手动下载完成", timeResult.Description)
		dc.messageUtils.SendMessageHTML(chatID, message)
		return
	}

	// Build batch download request
	var downloadItems []contracts.DownloadRequest
	for _, file := range response.Files {
		downloadItems = append(downloadItems, contracts.DownloadRequest{
			URL:          file.InternalURL,
			Filename:     file.Name,
			Directory:    file.DownloadPath,
			AutoClassify: true,
		})
	}

	config := dc.container.GetConfig()
	batchRequest := contracts.BatchDownloadRequest{
		Items:        downloadItems,
		VideoOnly:    config.Download.VideoOnly,
		AutoClassify: true,
	}

	// Call application service to create batch download
	downloadService := dc.container.GetDownloadService()
	batchResponse, err := downloadService.CreateBatchDownload(ctx, batchRequest)
	if err != nil {
		formatter := dc.messageUtils.GetFormatter().(*utils.MessageFormatter)
		dc.messageUtils.SendMessage(chatID, formatter.FormatError("批量下载", err))
		return
	}

	// Get configured default path
	path := config.Alist.DefaultPath
	if path == "" {
		path = "/"
	}

	// Send result
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
		dc.messageUtils.EscapeHTML(path),
		response.Summary.TotalFiles,
		response.Summary.TotalSizeFormatted,
		response.Summary.MovieFiles,
		response.Summary.TVFiles,
		response.Summary.OtherFiles,
		batchResponse.SuccessCount,
		batchResponse.FailureCount,
	)

	if batchResponse.FailureCount > 0 {
		message += fmt.Sprintf("\n\n⚠️ 有 %d 个文件下载失败，请检查日志获取详细信息", batchResponse.FailureCount)
	}

	dc.messageUtils.SendMessageHTML(chatID, message)
}
