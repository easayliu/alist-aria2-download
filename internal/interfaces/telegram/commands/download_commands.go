package commands

import (
	"context"
	"strings"

	"github.com/easayliu/alist-aria2-download/internal/interfaces/telegram/types"
	"github.com/easayliu/alist-aria2-download/internal/interfaces/telegram/utils"
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
		formatter := dc.messageUtils.GetFormatter().(*utils.MessageFormatter)
		dc.messageUtils.SendMessage(chatID, formatter.FormatError("取消下载", err))
		return
	}

	// 使用统一格式化器发送成功消息
	formatter := dc.messageUtils.GetFormatter().(*utils.MessageFormatter)
	message := formatter.FormatDownloadCancelled(gid)
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
		formatter := dc.messageUtils.GetFormatter().(*utils.MessageFormatter)
		dc.messageUtils.SendMessage(chatID, formatter.FormatError("创建下载任务", err))
		return
	}

	// 使用统一格式化器发送确认消息
	formatter := dc.messageUtils.GetFormatter().(*utils.MessageFormatter)
	message := formatter.FormatDownloadCreated(utils.DownloadCreatedData{
		URL:      url,
		GID:      response.ID,
		Filename: response.Filename,
	})
	dc.messageUtils.SendMessageHTML(chatID, message)
}

// handleDownloadFileByPath 通过路径下载单个文件
func (dc *DownloadCommands) handleDownloadFileByPath(ctx context.Context, chatID int64, filePath string) {
	// 构建文件下载请求
	req := contracts.FileDownloadRequest{
		FilePath:     filePath,
		AutoClassify: true,
	}

	// 调用应用服务下载文件
	fileService := dc.container.GetFileService()
	response, err := fileService.DownloadFile(ctx, req)
	if err != nil {
		formatter := dc.messageUtils.GetFormatter().(*utils.MessageFormatter)
		dc.messageUtils.SendMessage(chatID, formatter.FormatError("创建文件下载任务", err))
		return
	}

	// 发送成功消息 - 使用统一格式化器
	formatter := dc.messageUtils.GetFormatter().(*utils.MessageFormatter)
	message := formatter.FormatFileDownloadSuccess(utils.FileDownloadSuccessData{
		Filename:     response.Filename,
		FilePath:     filePath,
		DownloadPath: response.Directory,
		TaskID:       response.ID,
		Size:         dc.messageUtils.FormatFileSize(response.TotalSize),
		EscapeHTML:   dc.messageUtils.EscapeHTML,
	})

	dc.messageUtils.SendMessageHTML(chatID, message)
}

// handleDownloadDirectoryByPath 通过路径下载目录
func (dc *DownloadCommands) handleDownloadDirectoryByPath(ctx context.Context, chatID int64, dirPath string) {
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
		formatter := dc.messageUtils.GetFormatter().(*utils.MessageFormatter)
		dc.messageUtils.SendMessage(chatID, formatter.FormatError("扫描目录", err))
		return
	}

	if response.SuccessCount == 0 {
		formatter := dc.messageUtils.GetFormatter().(*utils.MessageFormatter)
		dc.messageUtils.SendMessage(chatID, formatter.FormatSimpleError("目录中没有可下载的文件"))
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