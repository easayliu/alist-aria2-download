package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/easayliu/alist-aria2-download/internal/api/handlers/telegram/types"
	"github.com/easayliu/alist-aria2-download/internal/application/contracts"
	"github.com/easayliu/alist-aria2-download/internal/application/services"
)

// DownloadCommands 下载相关命令处理器 - 纯协议转换层
type DownloadCommands struct {
	container    *services.ServiceContainer
	messageUtils types.MessageSender
}

// NewDownloadCommands 创建下载命令处理器
func NewDownloadCommands(container *services.ServiceContainer, messageUtils types.MessageSender) *DownloadCommands {
	return &DownloadCommands{
		container:    container,
		messageUtils: messageUtils,
	}
}

// HandleDownload 处理下载命令 - Telegram协议转换
func (dc *DownloadCommands) HandleDownload(chatID int64, command string) {
	ctx := context.Background()
	parts := strings.Fields(command)

	// 如果没有额外参数，默认进入预览模式（最近24小时）
	if len(parts) == 1 {
		dc.handleManualDownload(ctx, chatID, []string{}, true)
		return
	}

	// 检查第一个参数是否为URL（以http开头）
	if strings.HasPrefix(parts[1], "http") {
		dc.handleURLDownload(ctx, chatID, parts[1])
		return
	}

	// 检查第一个参数是否为文件路径（以/开头）
	if strings.HasPrefix(parts[1], "/") {
		filePath := parts[1]
		
		// 判断是文件还是目录
		if strings.HasSuffix(filePath, "/") || dc.isDirectoryPath(ctx, filePath) {
			// 目录下载
			dc.handleDownloadDirectoryByPath(ctx, chatID, filePath)
		} else {
			// 文件下载
			dc.handleDownloadFileByPath(ctx, chatID, filePath)
		}
		return
	}

	// 处理时间参数的手动下载
	preview := true
	timeArgs := parts[1:]
	if len(timeArgs) > 0 {
		subCommand := strings.ToLower(timeArgs[0])
		switch subCommand {
		case "confirm", "start", "run":
			preview = false
			timeArgs = timeArgs[1:]
		case "preview":
			preview = true
			timeArgs = timeArgs[1:]
		}
	}

	dc.handleManualDownload(ctx, chatID, timeArgs, preview)
}

// HandleCancel 处理取消下载命令
func (dc *DownloadCommands) HandleCancel(chatID int64, command string) {
	ctx := context.Background()
	parts := strings.Fields(command)
	if len(parts) < 2 {
		dc.messageUtils.SendMessage(chatID, "请提供下载GID\\n示例: /cancel abc123")
		return
	}

	gid := parts[1]

	// 调用应用服务取消下载
	downloadService := dc.container.GetDownloadService()
	if err := downloadService.CancelDownload(ctx, gid); err != nil {
		dc.messageUtils.SendMessage(chatID, "取消下载失败: "+err.Error())
		return
	}

	// 发送成功消息
	escapedID := dc.messageUtils.EscapeHTML(gid)
	message := fmt.Sprintf("<b>下载已取消</b>\\n\\n下载GID: <code>%s</code>", escapedID)
	dc.messageUtils.SendMessageHTML(chatID, message)
}

// handleURLDownload 处理URL下载
func (dc *DownloadCommands) handleURLDownload(ctx context.Context, chatID int64, url string) {
	// 构建下载请求
	req := contracts.DownloadRequest{
		URL:          url,
		AutoClassify: true,
	}

	// 调用应用服务创建下载
	downloadService := dc.container.GetDownloadService()
	response, err := downloadService.CreateDownload(ctx, req)
	if err != nil {
		dc.messageUtils.SendMessage(chatID, "创建下载任务失败: "+err.Error())
		return
	}

	// 发送确认消息 - Telegram格式转换
	escapedURL := dc.messageUtils.EscapeHTML(url)
	escapedID := dc.messageUtils.EscapeHTML(response.ID)
	escapedFilename := dc.messageUtils.EscapeHTML(response.Filename)
	message := fmt.Sprintf("<b>下载任务已创建</b>\\n\\nURL: <code>%s</code>\\nGID: <code>%s</code>\\n文件名: <code>%s</code>",
		escapedURL, escapedID, escapedFilename)
	dc.messageUtils.SendMessageHTML(chatID, message)
}

// handleDownloadFileByPath 通过路径下载单个文件
func (dc *DownloadCommands) handleDownloadFileByPath(ctx context.Context, chatID int64, filePath string) {
	dc.messageUtils.SendMessage(chatID, "📥 正在创建文件下载任务...")

	// 构建文件下载请求
	req := contracts.FileDownloadRequest{
		FilePath:     filePath,
		AutoClassify: true,
	}

	// 调用应用服务下载文件
	fileService := dc.container.GetFileService()
	response, err := fileService.DownloadFile(ctx, req)
	if err != nil {
		dc.messageUtils.SendMessage(chatID, fmt.Sprintf("❌ 创建文件下载任务失败: %v", err))
		return
	}

	// 发送成功消息 - Telegram格式转换
	message := fmt.Sprintf(
		"✅ <b>文件下载任务已创建</b>\\n\\n"+
			"<b>文件:</b> <code>%s</code>\\n"+
			"<b>路径:</b> <code>%s</code>\\n"+
			"<b>任务ID:</b> <code>%s</code>\\n",
		dc.messageUtils.EscapeHTML(response.Filename),
		dc.messageUtils.EscapeHTML(filePath),
		dc.messageUtils.EscapeHTML(response.ID))

	dc.messageUtils.SendMessageHTML(chatID, message)
}

// handleDownloadDirectoryByPath 通过路径下载目录
func (dc *DownloadCommands) handleDownloadDirectoryByPath(ctx context.Context, chatID int64, dirPath string) {
	dc.messageUtils.SendMessage(chatID, "📂 正在创建目录下载任务...")

	// 构建目录下载请求
	req := contracts.DirectoryDownloadRequest{
		DirectoryPath: dirPath,
		VideoOnly:     true, // 只下载视频文件
		AutoClassify:  true,
		Recursive:     true,
	}

	// 调用应用服务下载目录
	fileService := dc.container.GetFileService()
	response, err := fileService.DownloadDirectory(ctx, req)
	if err != nil {
		dc.messageUtils.SendMessage(chatID, fmt.Sprintf("❌ 扫描目录失败: %v", err))
		return
	}

	if response.SuccessCount == 0 {
		dc.messageUtils.SendMessage(chatID, "📁 目录中没有可下载的文件")
		return
	}

	// 转换为统一格式的结果摘要
	var downloadResults []types.DownloadResult
	for _, result := range response.Results {
		downloadResults = append(downloadResults, types.DownloadResult{
			Success: result.Success,
			Error:   result.Error,
			URL:     result.Request.URL,
			Name:    result.Request.Filename,
		})
	}

	summary := types.DownloadResultSummary{
		DirectoryPath: dirPath,
		TotalFiles:    response.Summary.TotalFiles,
		VideoFiles:    response.Summary.VideoFiles,
		SuccessCount:  response.SuccessCount,
		FailureCount:  response.FailureCount,
		Results:       downloadResults,
	}

	// 使用统一格式化器
	resultMessage := dc.messageUtils.FormatDownloadDirectoryResult(summary)
	dc.messageUtils.SendMessageHTML(chatID, resultMessage)
}

// isDirectoryPath 判断路径是否为目录
func (dc *DownloadCommands) isDirectoryPath(ctx context.Context, path string) bool {
	// 调用应用服务获取文件信息
	fileService := dc.container.GetFileService()
	fileInfo, err := fileService.GetFileInfo(ctx, path)
	return err == nil && fileInfo.IsDir
}