package commands

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/easayliu/alist-aria2-download/internal/api/handlers/telegram/utils"
	"github.com/easayliu/alist-aria2-download/internal/application/contracts"
	timeutils "github.com/easayliu/alist-aria2-download/pkg/utils"
)

// TimeParseResult 时间解析结果
type TimeParseResult struct {
	StartTime   time.Time
	EndTime     time.Time
	Description string
}

// HandleYesterdayFiles 处理获取昨天文件
func (dc *DownloadCommands) HandleYesterdayFiles(chatID int64) {
	ctx := context.Background()
	dc.messageUtils.SendMessage(chatID, "正在获取昨天的文件...")

	// 使用配置的默认路径
	config := dc.container.GetConfig()
	path := config.Alist.DefaultPath
	if path == "" {
		path = "/"
	}

	// 调用应用服务获取昨天的文件
	fileService := dc.container.GetFileService()
	response, err := fileService.GetYesterdayFiles(ctx, path)
	if err != nil {
		formatter := dc.messageUtils.GetFormatter().(*utils.MessageFormatter)
		dc.messageUtils.SendMessage(chatID, formatter.FormatError("获取昨天文件", err))
		return
	}

	if len(response.Files) == 0 {
		formatter := dc.messageUtils.GetFormatter().(*utils.MessageFormatter)
		dc.messageUtils.SendMessage(chatID, formatter.FormatSimpleError("昨天没有新文件"))
		return
	}

	// 准备显示的文件列表
	var displayFiles []utils.YesterdayFileItem
	maxDisplay := 10
	if len(response.Files) < maxDisplay {
		maxDisplay = len(response.Files)
	}
	for i := 0; i < maxDisplay; i++ {
		file := response.Files[i]
		displayFiles = append(displayFiles, utils.YesterdayFileItem{
			MediaType:     file.MediaType,
			Name:          file.Name,
			SizeFormatted: file.SizeFormatted,
		})
	}

	// 计算剩余文件数
	remainingCount := 0
	if len(response.Files) > 10 {
		remainingCount = len(response.Files) - 10
	}

	// 使用统一格式化器
	formatter := dc.messageUtils.GetFormatter().(*utils.MessageFormatter)
	message := formatter.FormatYesterdayFiles(utils.YesterdayFilesData{
		TotalCount:     len(response.Files),
		DisplayFiles:   displayFiles,
		TotalSize:      response.Summary.TotalSizeFormatted,
		TVCount:        response.Summary.TVFiles,
		MovieCount:     response.Summary.MovieFiles,
		OtherCount:     response.Summary.OtherFiles,
		RemainingCount: remainingCount,
		EscapeHTML:     dc.messageUtils.EscapeHTML,
	})

	dc.messageUtils.SendMessageHTML(chatID, message)
}

// HandleYesterdayDownload 处理下载昨天的文件
func (dc *DownloadCommands) HandleYesterdayDownload(chatID int64) {
	ctx := context.Background()
	dc.messageUtils.SendMessage(chatID, "正在准备下载昨天的文件...")

	// 使用配置的默认路径
	config := dc.container.GetConfig()
	path := config.Alist.DefaultPath
	if path == "" {
		path = "/"
	}

	// 获取昨天的文件
	fileService := dc.container.GetFileService()
	response, err := fileService.GetYesterdayFiles(ctx, path)
	if err != nil {
		formatter := dc.messageUtils.GetFormatter().(*utils.MessageFormatter)
		dc.messageUtils.SendMessage(chatID, formatter.FormatError("获取昨天文件", err))
		return
	}

	if len(response.Files) == 0 {
		dc.messageUtils.SendMessage(chatID, "昨天没有新文件需要下载")
		return
	}

	// 构建批量下载请求
	var downloadItems []contracts.DownloadRequest
	for _, file := range response.Files {
		downloadItems = append(downloadItems, contracts.DownloadRequest{
			URL:          file.InternalURL,
			Filename:     file.Name,
			Directory:    file.DownloadPath,
			AutoClassify: true,
		})
	}

	batchRequest := contracts.BatchDownloadRequest{
		Items:        downloadItems,
		VideoOnly:    config.Download.VideoOnly,
		AutoClassify: true,
	}

	// 调用应用服务批量创建下载
	downloadService := dc.container.GetDownloadService()
	batchResponse, err := downloadService.CreateBatchDownload(ctx, batchRequest)
	if err != nil {
		formatter := dc.messageUtils.GetFormatter().(*utils.MessageFormatter)
		dc.messageUtils.SendMessage(chatID, formatter.FormatError("批量下载", err))
		return
	}

	// 使用统一格式化器发送结果
	formatter := dc.messageUtils.GetFormatter().(*utils.MessageFormatter)
	message := formatter.FormatBatchDownloadResult2(utils.BatchDownloadResult2Data{
		SuccessCount: batchResponse.SuccessCount,
		FailCount:    batchResponse.FailureCount,
		TotalCount:   len(response.Files),
	})

	dc.messageUtils.SendMessageHTML(chatID, message)
}

// handleManualDownload 处理手动下载功能
func (dc *DownloadCommands) handleManualDownload(ctx context.Context, chatID int64, timeArgs []string, preview bool) {
	// 解析时间参数
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

	// 获取配置的默认路径
	config := dc.container.GetConfig()
	path := config.Alist.DefaultPath
	if path == "" {
		path = "/"
	}

	// 构建时间范围文件请求
	req := contracts.TimeRangeFileRequest{
		Path:      path,
		StartTime: timeResult.StartTime,
		EndTime:   timeResult.EndTime,
		VideoOnly: true, // 只处理视频文件
	}

	// 调用应用服务获取时间范围内的文件
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
		// 预览模式：显示文件信息和确认按钮
		dc.sendManualDownloadPreview(chatID, response, timeResult, timeArgs)
	} else {
		// 直接下载模式：创建下载任务
		dc.executeManualDownload(ctx, chatID, response, timeResult)
	}
}

// parseTimeArguments 解析时间参数
// 支持的格式：
// 1. 数字 - 小时数（如：48）
// 2. 日期范围 - 两个日期（如：2025-09-01 2025-09-26）
// 3. 时间范围 - 两个时间戳（如：2025-09-01T00:00:00Z 2025-09-26T23:59:59Z）
func (dc *DownloadCommands) parseTimeArguments(args []string) (*TimeParseResult, error) {
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
		if hours, err := strconv.Atoi(args[0]); err == nil {
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

// sendManualDownloadPreview 发送手动下载预览
func (dc *DownloadCommands) sendManualDownloadPreview(chatID int64, response *contracts.TimeRangeFileResponse, timeResult *TimeParseResult, timeArgs []string) {
	// 获取配置的默认路径
	config := dc.container.GetConfig()
	path := config.Alist.DefaultPath
	if path == "" {
		path = "/"
	}

	// 构建预览消息
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
			// 限制文件名长度
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

	// 构建确认命令
	confirmCommand := "/download confirm"
	if len(timeArgs) > 0 {
		confirmCommand += " " + strings.Join(timeArgs, " ")
	}

	message += fmt.Sprintf("\n\n⚠️ 预览有效期 10 分钟。发送 <code>%s</code> 开始下载。", confirmCommand)

	dc.messageUtils.SendMessageHTML(chatID, message)
}

// executeManualDownload 执行手动下载
func (dc *DownloadCommands) executeManualDownload(ctx context.Context, chatID int64, response *contracts.TimeRangeFileResponse, timeResult *TimeParseResult) {
	if len(response.Files) == 0 {
		formatter := dc.messageUtils.GetFormatter().(*utils.MessageFormatter)
		message := formatter.FormatNoFilesFound("手动下载完成", timeResult.Description)
		dc.messageUtils.SendMessageHTML(chatID, message)
		return
	}

	// 构建批量下载请求
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

	// 调用应用服务批量创建下载
	downloadService := dc.container.GetDownloadService()
	batchResponse, err := downloadService.CreateBatchDownload(ctx, batchRequest)
	if err != nil {
		formatter := dc.messageUtils.GetFormatter().(*utils.MessageFormatter)
		dc.messageUtils.SendMessage(chatID, formatter.FormatError("批量下载", err))
		return
	}

	// 获取配置的默认路径
	path := config.Alist.DefaultPath
	if path == "" {
		path = "/"
	}

	// 发送结果
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