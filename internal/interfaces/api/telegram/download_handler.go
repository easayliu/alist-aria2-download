package telegram

import (
	"context"
	"fmt"
	"strings"

	"github.com/easayliu/alist-aria2-download/internal/application/contracts"
	"github.com/easayliu/alist-aria2-download/internal/api/handlers/telegram/types"
)

// TelegramDownloadHandler Telegram下载处理器 - 专注于协议转换
type TelegramDownloadHandler struct {
	downloadService contracts.DownloadService
	fileService     contracts.FileService
	messageUtils    types.MessageSender
}

// NewTelegramDownloadHandler 创建Telegram下载处理器
func NewTelegramDownloadHandler(
	downloadService contracts.DownloadService,
	fileService contracts.FileService,
	messageUtils types.MessageSender,
) *TelegramDownloadHandler {
	return &TelegramDownloadHandler{
		downloadService: downloadService,
		fileService:     fileService,
		messageUtils:    messageUtils,
	}
}

// HandleDownload 处理下载命令 - 统一业务逻辑调用
func (h *TelegramDownloadHandler) HandleDownload(chatID int64, command string) {
	ctx := context.Background()
	parts := strings.Fields(command)

	// 如果没有额外参数，显示帮助信息
	if len(parts) == 1 {
		h.sendDownloadHelp(chatID)
		return
	}

	// 解析命令参数
	arg := parts[1]

	// 1. URL下载
	if strings.HasPrefix(arg, "http") {
		h.handleURLDownload(ctx, chatID, arg)
		return
	}

	// 2. 文件路径下载
	if strings.HasPrefix(arg, "/") {
		if strings.HasSuffix(arg, "/") || h.isDirectoryPath(ctx, arg) {
			h.handleDirectoryDownload(ctx, chatID, arg)
		} else {
			h.handleFileDownload(ctx, chatID, arg)
		}
		return
	}

	// 3. 时间范围下载命令
	h.handleTimeRangeDownload(ctx, chatID, parts[1:])
}

// HandleCancel 处理取消下载命令
func (h *TelegramDownloadHandler) HandleCancel(chatID int64, command string) {
	ctx := context.Background()
	parts := strings.Fields(command)
	
	if len(parts) < 2 {
		h.messageUtils.SendMessage(chatID, "请提供下载ID\\n示例: /cancel abc123")
		return
	}

	downloadID := parts[1]

	// 调用业务服务取消下载
	err := h.downloadService.CancelDownload(ctx, downloadID)
	if err != nil {
		h.messageUtils.SendMessage(chatID, "取消下载失败: "+err.Error())
		return
	}

	message := fmt.Sprintf("<b>下载已取消</b>\\n\\n下载ID: <code>%s</code>", 
		h.messageUtils.EscapeHTML(downloadID))
	h.messageUtils.SendMessageHTML(chatID, message)
}

// HandleDownloadStatus 处理下载状态查询
func (h *TelegramDownloadHandler) HandleDownloadStatus(chatID int64, command string) {
	ctx := context.Background()
	parts := strings.Fields(command)
	
	if len(parts) < 2 {
		// 显示下载列表
		h.handleListDownloads(ctx, chatID)
		return
	}

	downloadID := parts[1]

	// 获取特定下载状态
	download, err := h.downloadService.GetDownload(ctx, downloadID)
	if err != nil {
		h.messageUtils.SendMessage(chatID, "获取下载状态失败: "+err.Error())
		return
	}

	// 格式化下载状态消息
	message := h.formatDownloadStatus(download)
	h.messageUtils.SendMessageHTML(chatID, message)
}

// HandleYesterdayFiles 处理昨天文件命令
func (h *TelegramDownloadHandler) HandleYesterdayFiles(chatID int64, defaultPath string) {
	ctx := context.Background()

	// 调用业务服务获取昨天的文件
	files, err := h.fileService.GetYesterdayFiles(ctx, defaultPath)
	if err != nil {
		h.messageUtils.SendMessage(chatID, fmt.Sprintf("获取昨天文件失败: %v", err))
		return
	}

	if len(files.Files) == 0 {
		h.messageUtils.SendMessage(chatID, "昨天没有新文件")
		return
	}

	// 格式化文件列表消息
	message := h.formatFilesList("昨天的文件", files.Files, files.Summary)
	h.messageUtils.SendMessageHTML(chatID, message)
}

// HandleYesterdayDownload 处理下载昨天文件命令
func (h *TelegramDownloadHandler) HandleYesterdayDownload(chatID int64, defaultPath string) {
	ctx := context.Background()

	h.messageUtils.SendMessage(chatID, "正在准备下载昨天的文件...")

	// 获取昨天的文件
	files, err := h.fileService.GetYesterdayFiles(ctx, defaultPath)
	if err != nil {
		h.messageUtils.SendMessage(chatID, fmt.Sprintf("获取昨天文件失败: %v", err))
		return
	}

	if len(files.Files) == 0 {
		h.messageUtils.SendMessage(chatID, "昨天没有新文件需要下载")
		return
	}

	// 构建批量下载请求
	var downloadRequests []contracts.DownloadRequest
	for _, file := range files.Files {
		downloadRequests = append(downloadRequests, contracts.DownloadRequest{
			URL:          file.InternalURL,
			Filename:     file.Name,
			Directory:    file.DownloadPath,
			VideoOnly:    true,
			AutoClassify: true,
		})
	}

	batchReq := contracts.BatchDownloadRequest{
		Items:        downloadRequests,
		VideoOnly:    true,
		AutoClassify: true,
	}

	// 调用业务服务批量下载
	result, err := h.downloadService.CreateBatchDownload(ctx, batchReq)
	if err != nil {
		h.messageUtils.SendMessage(chatID, fmt.Sprintf("创建批量下载失败: %v", err))
		return
	}

	// 发送结果消息
	message := h.formatBatchDownloadResult("昨天文件下载", result)
	h.messageUtils.SendMessageHTML(chatID, message)
}

// ========== 私有方法 ==========

// handleURLDownload 处理URL下载
func (h *TelegramDownloadHandler) handleURLDownload(ctx context.Context, chatID int64, url string) {
	req := contracts.DownloadRequest{
		URL:          url,
		VideoOnly:    true,
		AutoClassify: true,
	}

	download, err := h.downloadService.CreateDownload(ctx, req)
	if err != nil {
		h.messageUtils.SendMessage(chatID, "创建下载任务失败: "+err.Error())
		return
	}

	message := fmt.Sprintf(
		"<b>下载任务已创建</b>\\n\\n"+
			"URL: <code>%s</code>\\n"+
			"ID: <code>%s</code>\\n"+
			"文件名: <code>%s</code>",
		h.messageUtils.EscapeHTML(url),
		h.messageUtils.EscapeHTML(download.ID),
		h.messageUtils.EscapeHTML(download.Filename))

	h.messageUtils.SendMessageHTML(chatID, message)
}

// handleFileDownload 处理文件下载
func (h *TelegramDownloadHandler) handleFileDownload(ctx context.Context, chatID int64, filePath string) {
	h.messageUtils.SendMessage(chatID, "📥 正在创建文件下载任务...")

	req := contracts.FileDownloadRequest{
		FilePath:     filePath,
		AutoClassify: true,
	}

	download, err := h.fileService.DownloadFile(ctx, req)
	if err != nil {
		h.messageUtils.SendMessage(chatID, fmt.Sprintf("❌ 创建下载任务失败: %v", err))
		return
	}

	message := fmt.Sprintf(
		"✅ <b>文件下载任务已创建</b>\\n\\n"+
			"<b>文件:</b> <code>%s</code>\\n"+
			"<b>路径:</b> <code>%s</code>\\n"+
			"<b>下载路径:</b> <code>%s</code>\\n"+
			"<b>任务ID:</b> <code>%s</code>",
		h.messageUtils.EscapeHTML(download.Filename),
		h.messageUtils.EscapeHTML(filePath),
		h.messageUtils.EscapeHTML(download.Directory),
		h.messageUtils.EscapeHTML(download.ID))

	h.messageUtils.SendMessageHTML(chatID, message)
}

// handleDirectoryDownload 处理目录下载
func (h *TelegramDownloadHandler) handleDirectoryDownload(ctx context.Context, chatID int64, dirPath string) {
	h.messageUtils.SendMessage(chatID, "📂 正在扫描目录并创建下载任务...")

	req := contracts.DirectoryDownloadRequest{
		DirectoryPath: dirPath,
		Recursive:     true,
		VideoOnly:     true,
		AutoClassify:  true,
	}

	result, err := h.fileService.DownloadDirectory(ctx, req)
	if err != nil {
		h.messageUtils.SendMessage(chatID, fmt.Sprintf("❌ 扫描目录失败: %v", err))
		return
	}

	message := h.formatBatchDownloadResult("目录下载", result)
	h.messageUtils.SendMessageHTML(chatID, message)
}

// handleTimeRangeDownload 处理时间范围下载
func (h *TelegramDownloadHandler) handleTimeRangeDownload(ctx context.Context, chatID int64, args []string) {
	// 解析时间参数，这里可以实现复杂的时间解析逻辑
	// 目前简化实现，默认为预览模式
	h.messageUtils.SendMessage(chatID, "⏰ 时间范围下载功能开发中...")
}

// handleListDownloads 处理下载列表查询
func (h *TelegramDownloadHandler) handleListDownloads(ctx context.Context, chatID int64) {
	req := contracts.DownloadListRequest{
		Limit: 10, // Telegram消息限制，只显示最近10个
	}

	downloads, err := h.downloadService.ListDownloads(ctx, req)
	if err != nil {
		h.messageUtils.SendMessage(chatID, "获取下载列表失败: "+err.Error())
		return
	}

	if len(downloads.Downloads) == 0 {
		h.messageUtils.SendMessage(chatID, "暂无下载任务")
		return
	}

	message := h.formatDownloadsList(downloads)
	h.messageUtils.SendMessageHTML(chatID, message)
}

// isDirectoryPath 判断是否为目录路径
func (h *TelegramDownloadHandler) isDirectoryPath(ctx context.Context, path string) bool {
	// 尝试获取文件信息判断是否为目录
	listReq := contracts.FileListRequest{
		Path:     path,
		PageSize: 1,
	}
	
	_, err := h.fileService.ListFiles(ctx, listReq)
	return err == nil
}

// formatDownloadStatus 格式化下载状态信息
func (h *TelegramDownloadHandler) formatDownloadStatus(download *contracts.DownloadResponse) string {
	statusEmoji := h.getStatusEmoji(download.Status)
	
	message := fmt.Sprintf(
		"<b>%s 下载状态</b>\\n\\n"+
			"<b>ID:</b> <code>%s</code>\\n"+
			"<b>文件名:</b> <code>%s</code>\\n"+
			"<b>状态:</b> %s %s\\n"+
			"<b>进度:</b> %.1f%%\\n",
		statusEmoji,
		h.messageUtils.EscapeHTML(download.ID),
		h.messageUtils.EscapeHTML(download.Filename),
		statusEmoji,
		h.getStatusText(download.Status))

	if download.TotalSize > 0 {
		message += fmt.Sprintf(
			"<b>大小:</b> %s / %s\\n",
			h.messageUtils.FormatFileSize(download.CompletedSize),
			h.messageUtils.FormatFileSize(download.TotalSize))
	}

	if download.Speed > 0 {
		message += fmt.Sprintf("<b>速度:</b> %s/s\\n", h.messageUtils.FormatFileSize(download.Speed))
	}

	if download.ErrorMessage != "" {
		message += fmt.Sprintf("\\n<b>错误:</b> <code>%s</code>", h.messageUtils.EscapeHTML(download.ErrorMessage))
	}

	return message
}

// formatFilesList 格式化文件列表
func (h *TelegramDownloadHandler) formatFilesList(title string, files []contracts.FileResponse, summary contracts.FileSummary) string {
	message := fmt.Sprintf("<b>%s (%d个):</b>\\n\\n", title, len(files))

	// 只显示前10个文件
	displayCount := len(files)
	if displayCount > 10 {
		displayCount = 10
	}

	for i := 0; i < displayCount; i++ {
		file := files[i]
		category := h.getCategoryEmoji(file.Category)
		sizeStr := h.messageUtils.FormatFileSize(file.Size)
		message += fmt.Sprintf("%s %s (%s)\\n", 
			category,
			h.messageUtils.EscapeHTML(file.Name), 
			sizeStr)
	}

	if len(files) > 10 {
		message += fmt.Sprintf("\\n... 还有 %d 个文件未显示\\n", len(files)-10)
	}

	// 添加统计信息
	message += fmt.Sprintf("\\n<b>统计信息:</b>\\n")
	message += fmt.Sprintf("总大小: %s\\n", summary.TotalSizeFormatted)
	if summary.VideoFiles > 0 {
		message += fmt.Sprintf("视频文件: %d\\n", summary.VideoFiles)
	}
	if summary.MovieFiles > 0 {
		message += fmt.Sprintf("电影: %d\\n", summary.MovieFiles)
	}
	if summary.TVFiles > 0 {
		message += fmt.Sprintf("电视剧: %d\\n", summary.TVFiles)
	}

	return message
}

// formatBatchDownloadResult 格式化批量下载结果
func (h *TelegramDownloadHandler) formatBatchDownloadResult(title string, result *contracts.BatchDownloadResponse) string {
	message := fmt.Sprintf("<b>%s完成</b>\\n\\n", title)
	message += fmt.Sprintf("成功: %d\\n", result.SuccessCount)
	if result.FailureCount > 0 {
		message += fmt.Sprintf("失败: %d\\n", result.FailureCount)
	}
	message += fmt.Sprintf("总计: %d\\n", len(result.Results))

	if result.Summary.TotalFiles > 0 {
		message += fmt.Sprintf("\\n<b>统计:</b>\\n")
		message += fmt.Sprintf("视频文件: %d\\n", result.Summary.VideoFiles)
		if result.Summary.MovieFiles > 0 {
			message += fmt.Sprintf("电影: %d\\n", result.Summary.MovieFiles)
		}
		if result.Summary.TVFiles > 0 {
			message += fmt.Sprintf("电视剧: %d\\n", result.Summary.TVFiles)
		}
	}

	if result.SuccessCount > 0 {
		message += "\\n✅ 所有任务已使用自动路径分类功能"
	}

	return message
}

// formatDownloadsList 格式化下载列表
func (h *TelegramDownloadHandler) formatDownloadsList(downloads *contracts.DownloadListResponse) string {
	message := fmt.Sprintf("<b>下载任务列表 (%d个)</b>\\n\\n", downloads.TotalCount)

	for i, download := range downloads.Downloads {
		if i >= 10 { // 限制显示数量
			break
		}

		statusEmoji := h.getStatusEmoji(download.Status)
		message += fmt.Sprintf(
			"%d. %s <code>%s</code>\\n   %s (%.1f%%)\\n\\n",
			i+1,
			statusEmoji,
			download.ID[:8],
			h.messageUtils.EscapeHTML(download.Filename),
			download.Progress)
	}

	if downloads.TotalCount > 10 {
		message += fmt.Sprintf("... 还有 %d 个任务\\n\\n", downloads.TotalCount-10)
	}

	if downloads.ActiveCount > 0 {
		message += fmt.Sprintf("活跃下载: %d 个", downloads.ActiveCount)
	}

	return message
}

// sendDownloadHelp 发送下载帮助信息
func (h *TelegramDownloadHandler) sendDownloadHelp(chatID int64) {
	message := "<b>下载命令帮助</b>\\n\\n" +
		"<b>基本用法:</b>\\n" +
		"• <code>/download URL</code> - 下载网络文件\\n" +
		"• <code>/download /path/file</code> - 下载指定文件\\n" +
		"• <code>/download /path/dir/</code> - 下载整个目录\\n\\n" +
		"<b>状态查询:</b>\\n" +
		"• <code>/status</code> - 查看下载列表\\n" +
		"• <code>/status ID</code> - 查看特定下载状态\\n\\n" +
		"<b>下载控制:</b>\\n" +
		"• <code>/cancel ID</code> - 取消下载\\n\\n" +
		"<b>快捷下载:</b>\\n" +
		"• <code>/yesterday</code> - 查看昨天的文件\\n" +
		"• <code>/yesterday_download</code> - 下载昨天的文件\\n\\n" +
		"所有下载都会自动分类到对应目录 📁"

	h.messageUtils.SendMessageHTML(chatID, message)
}

// getStatusEmoji 获取状态表情
func (h *TelegramDownloadHandler) getStatusEmoji(status interface{}) string {
	switch status {
	case "active", "running":
		return "🔄"
	case "complete", "completed":
		return "✅"
	case "paused":
		return "⏸️"
	case "error", "failed":
		return "❌"
	case "waiting", "pending":
		return "⏳"
	default:
		return "❓"
	}
}

// getStatusText 获取状态文本
func (h *TelegramDownloadHandler) getStatusText(status interface{}) string {
	switch status {
	case "active", "running":
		return "下载中"
	case "complete", "completed":
		return "已完成"
	case "paused":
		return "已暂停"
	case "error", "failed":
		return "下载失败"
	case "waiting", "pending":
		return "等待中"
	default:
		return "未知状态"
	}
}

// getCategoryEmoji 获取分类表情
func (h *TelegramDownloadHandler) getCategoryEmoji(category string) string {
	switch category {
	case "movie":
		return "🎬"
	case "tv":
		return "📺"
	case "variety":
		return "🎭"
	case "video":
		return "🎥"
	default:
		return "📄"
	}
}