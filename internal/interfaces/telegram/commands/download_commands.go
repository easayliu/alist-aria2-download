package commands

import (
	"context"
	"strings"

	"github.com/easayliu/alist-aria2-download/internal/interfaces/telegram/types"
	"github.com/easayliu/alist-aria2-download/internal/interfaces/telegram/utils"
	"github.com/easayliu/alist-aria2-download/internal/application/contracts"
	"github.com/easayliu/alist-aria2-download/internal/application/services"
)

// DownloadCommands handles download-related commands - pure protocol conversion layer
type DownloadCommands struct {
	container    *services.ServiceContainer
	messageUtils types.MessageSender
}

// NewDownloadCommands creates a download command handler
func NewDownloadCommands(container *services.ServiceContainer, messageUtils types.MessageSender) *DownloadCommands {
	return &DownloadCommands{
		container:    container,
		messageUtils: messageUtils,
	}
}

// HandleDownload handles download command - Telegram protocol conversion
func (dc *DownloadCommands) HandleDownload(chatID int64, command string) {
	ctx := context.Background()
	parts := strings.Fields(command)

	// If no additional parameters, default to preview mode (last 24 hours)
	if len(parts) == 1 {
		dc.handleManualDownload(ctx, chatID, []string{}, true)
		return
	}

	// Check if first parameter is a URL (starts with http)
	if strings.HasPrefix(parts[1], "http") {
		dc.handleURLDownload(ctx, chatID, parts[1])
		return
	}

	// Check if first parameter is a file path (starts with /)
	if strings.HasPrefix(parts[1], "/") {
		filePath := parts[1]

		// Determine if it's a file or directory
		if strings.HasSuffix(filePath, "/") || dc.isDirectoryPath(ctx, filePath) {
			// Directory download
			dc.handleDownloadDirectoryByPath(ctx, chatID, filePath)
		} else {
			// File download
			dc.handleDownloadFileByPath(ctx, chatID, filePath)
		}
		return
	}

	// Handle manual download with time parameters
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

// HandleCancel handles cancel download command
func (dc *DownloadCommands) HandleCancel(chatID int64, command string) {
	ctx := context.Background()
	parts := strings.Fields(command)
	if len(parts) < 2 {
		dc.messageUtils.SendMessage(chatID, "请提供下载GID\\n示例: /cancel abc123")
		return
	}

	gid := parts[1]

	// Call application service to cancel download
	downloadService := dc.container.GetDownloadService()
	if err := downloadService.CancelDownload(ctx, gid); err != nil {
		formatter := dc.messageUtils.GetFormatter().(*utils.MessageFormatter)
		dc.messageUtils.SendMessage(chatID, formatter.FormatError("取消下载", err))
		return
	}

	// Send success message using unified formatter
	formatter := dc.messageUtils.GetFormatter().(*utils.MessageFormatter)
	message := formatter.FormatDownloadCancelled(gid)
	dc.messageUtils.SendMessageHTML(chatID, message)
}

// handleURLDownload handles URL download
func (dc *DownloadCommands) handleURLDownload(ctx context.Context, chatID int64, url string) {
	// Build download request
	req := contracts.DownloadRequest{
		URL:          url,
		AutoClassify: true,
	}

	// Call application service to create download
	downloadService := dc.container.GetDownloadService()
	response, err := downloadService.CreateDownload(ctx, req)
	if err != nil {
		formatter := dc.messageUtils.GetFormatter().(*utils.MessageFormatter)
		dc.messageUtils.SendMessage(chatID, formatter.FormatError("创建下载任务", err))
		return
	}

	// Send confirmation message using unified formatter
	formatter := dc.messageUtils.GetFormatter().(*utils.MessageFormatter)
	message := formatter.FormatDownloadCreated(utils.DownloadCreatedData{
		URL:      url,
		GID:      response.ID,
		Filename: response.Filename,
	})
	dc.messageUtils.SendMessageHTML(chatID, message)
}

// handleDownloadFileByPath downloads a single file by path
func (dc *DownloadCommands) handleDownloadFileByPath(ctx context.Context, chatID int64, filePath string) {
	// Build file download request
	req := contracts.FileDownloadRequest{
		FilePath:     filePath,
		AutoClassify: true,
	}

	// Call application service to download file
	fileService := dc.container.GetFileService()
	response, err := fileService.DownloadFile(ctx, req)
	if err != nil {
		formatter := dc.messageUtils.GetFormatter().(*utils.MessageFormatter)
		dc.messageUtils.SendMessage(chatID, formatter.FormatError("创建文件下载任务", err))
		return
	}

	// Send success message using unified formatter
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

// handleDownloadDirectoryByPath downloads a directory by path
func (dc *DownloadCommands) handleDownloadDirectoryByPath(ctx context.Context, chatID int64, dirPath string) {
	// Build directory download request
	req := contracts.DirectoryDownloadRequest{
		DirectoryPath: dirPath,
		VideoOnly:     true, // Only download video files
		AutoClassify:  true,
		Recursive:     true,
	}

	// Call application service to download directory
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

	// Convert to unified format result summary
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

	// Use unified formatter
	resultMessage := dc.messageUtils.FormatDownloadDirectoryResult(summary)
	dc.messageUtils.SendMessageHTML(chatID, resultMessage)
}

// isDirectoryPath determines if a path is a directory
func (dc *DownloadCommands) isDirectoryPath(ctx context.Context, path string) bool {
	// Call application service to get file info
	fileService := dc.container.GetFileService()
	fileInfo, err := fileService.GetFileInfo(ctx, path)
	return err == nil && fileInfo.IsDir
}