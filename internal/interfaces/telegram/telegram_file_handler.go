package telegram

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/easayliu/alist-aria2-download/internal/interfaces/telegram/utils"
	"github.com/easayliu/alist-aria2-download/internal/application/contracts"
	"github.com/easayliu/alist-aria2-download/pkg/logger"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// FileHandler handles file browsing functionality
type FileHandler struct {
	controller *TelegramController
}

// NewFileHandler creates a new file handler
func NewFileHandler(controller *TelegramController) *FileHandler {
	return &FileHandler{
		controller: controller,
	}
}

func (h *FileHandler) buildFullPath(file contracts.FileResponse, basePath string) string {
	if file.Path != "" {
		return file.Path
	}
	if basePath == "/" {
		return "/" + file.Name
	}
	return basePath + "/" + file.Name
}

// ================================
// File browsing functionality
// ================================

// HandleBrowseFiles handles file browsing (supports pagination and interaction)
func (h *FileHandler) HandleBrowseFiles(chatID int64, path string, page int) {
	h.HandleBrowseFilesWithEdit(chatID, path, page, 0) // 0 means send new message
}

// HandleBrowseFilesWithEdit handles file browsing (supports message editing and pagination)
func (h *FileHandler) HandleBrowseFilesWithEdit(chatID int64, path string, page int, messageID int) {
	if path == "" {
		path = "/"
	}
	if page < 1 {
		page = 1
	}

	// Debug log
	logger.Info("Browsing files", "path", path, "page", page, "messageID", messageID)

	// Only show prompt when sending new message
	if messageID == 0 {
		h.controller.messageUtils.SendMessage(chatID, "正在获取文件列表...")
	}

	// Get file list (display 8 files per page, leave space for button layout)
	files, err := h.listFilesSimple(path, page, 8)
	if err != nil {
		formatter := h.controller.messageUtils.GetFormatter().(*utils.MessageFormatter)
		h.controller.messageUtils.SendMessage(chatID, formatter.FormatError("获取文件列表", err))
		return
	}

	if len(files) == 0 {
		h.controller.messageUtils.SendMessageHTMLWithAutoDelete(chatID, "当前目录为空", 30)
		return
	}

	// Count file information
	dirCount := 0
	fileCount := 0
	videoCount := 0
	for _, file := range files {
		if file.IsDir {
			dirCount++
		} else {
			fileCount++
			if h.controller.fileService.IsVideoFile(file.Name) {
				videoCount++
			}
		}
	}

	// Use unified formatter
	formatter := h.controller.messageUtils.GetFormatter().(*utils.MessageFormatter)
	browserData := utils.FileBrowserData{
		Path:       path,
		Page:       page,
		TotalPages: 1, // 暂时设为1,如果需要可以计算总页数
		TotalFiles: len(files),
		DirCount:   dirCount,
		FileCount:  fileCount,
		VideoCount: videoCount,
		EscapeHTML: h.controller.messageUtils.EscapeHTML,
	}
	message := formatter.FormatFileBrowser(browserData)
	message += "\n"

	// Build inline keyboard
	var keyboard [][]tgbotapi.InlineKeyboardButton

	for _, file := range files {
		var prefix string
		var callbackData string

		if file.IsDir {
			prefix = "📁"
			fullPath := h.buildFullPath(file, path)
			callbackData = fmt.Sprintf("browse_dir:%s:1", h.controller.common.EncodeFilePath(fullPath))
		} else if h.controller.fileService.IsVideoFile(file.Name) {
			prefix = "🎬"
			fullPath := h.buildFullPath(file, path)
			callbackData = fmt.Sprintf("file_menu:%s", h.controller.common.EncodeFilePath(fullPath))
		} else {
			prefix = "📄"
			fullPath := h.buildFullPath(file, path)
			callbackData = fmt.Sprintf("file_menu:%s", h.controller.common.EncodeFilePath(fullPath))
		}

		fileName := file.Name
		maxWidth := 38
		if !file.IsDir {
			maxWidth = 30
		}

		formatter := h.controller.messageUtils.GetFormatter().(*utils.MessageFormatter)
		fileName = formatter.TruncateButtonText(fileName, maxWidth)

		button := tgbotapi.NewInlineKeyboardButtonData(
			fmt.Sprintf("%s %s", prefix, fileName),
			callbackData,
		)

		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{button})
	}

	// Add navigation buttons
	navButtons := []tgbotapi.InlineKeyboardButton{}

	// Previous page button
	if page > 1 {
		navButtons = append(navButtons, tgbotapi.NewInlineKeyboardButtonData(
			"< 上一页",
			fmt.Sprintf("browse_page:%s:%d", h.controller.common.EncodeFilePath(path), page-1),
		))
	}

	// Next page button (if current page is full, there may be more)
	if len(files) == 8 {
		navButtons = append(navButtons, tgbotapi.NewInlineKeyboardButtonData(
			"下一页 >",
			fmt.Sprintf("browse_page:%s:%d", h.controller.common.EncodeFilePath(path), page+1),
		))
	}

	if len(navButtons) > 0 {
		keyboard = append(keyboard, navButtons)
	}

	// Add action buttons - first row: download and refresh
	actionRow1 := []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("📥 下载目录", fmt.Sprintf("download_dir:%s", h.controller.common.EncodeFilePath(path))),
		tgbotapi.NewInlineKeyboardButtonData("📝 批量重命名", fmt.Sprintf("batch_rename:%s", h.controller.common.EncodeFilePath(path))),
		tgbotapi.NewInlineKeyboardButtonData("🔄 刷新", fmt.Sprintf("browse_refresh:%s:%d", h.controller.common.EncodeFilePath(path), page)),
	}
	keyboard = append(keyboard, actionRow1)

	// Add navigation buttons - second row: parent directory, delete directory and main menu
	actionRow2 := []tgbotapi.InlineKeyboardButton{}

	// Return to parent directory button
	if path != "/" {
		parentPath := h.getParentPath(path)
		actionRow2 = append(actionRow2, tgbotapi.NewInlineKeyboardButtonData(
			"⬆️ 上级目录",
			fmt.Sprintf("browse_dir:%s:%d", h.controller.common.EncodeFilePath(parentPath), 1),
		))
	}

	// Delete directory button (only for non-root directories)
	if path != "/" {
		actionRow2 = append(actionRow2, tgbotapi.NewInlineKeyboardButtonData(
			"🗑️ 删除目录",
			fmt.Sprintf("dir_delete_confirm:%s", h.controller.common.EncodeFilePath(path)),
		))
	}

	// Return to main menu button
	actionRow2 = append(actionRow2, tgbotapi.NewInlineKeyboardButtonData("🏠 主菜单", "back_main"))

	if len(actionRow2) > 0 {
		keyboard = append(keyboard, actionRow2)
	}

	inlineKeyboard := tgbotapi.NewInlineKeyboardMarkup(keyboard...)

	if messageID > 0 {
		// Edit existing message
		h.controller.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &inlineKeyboard)
	} else {
		// Send new message
		h.controller.messageUtils.SendMessageWithKeyboard(chatID, message, "HTML", &inlineKeyboard)
	}
}

// HandleFileMenu handles file operation menu
func (h *FileHandler) HandleFileMenu(chatID int64, filePath string) {
	h.HandleFileMenuWithEdit(chatID, filePath, 0)
}

// HandleFileMenuWithEdit handles file operation menu (supports message editing)
func (h *FileHandler) HandleFileMenuWithEdit(chatID int64, filePath string, messageID int) {
	fileName := filepath.Base(filePath)
	fileExt := strings.ToLower(filepath.Ext(fileName))

	var fileIcon string
	if h.controller.fileService.IsVideoFile(fileName) {
		fileIcon = "🎬"
	} else {
		fileIcon = "📄"
	}

	formatter := h.controller.messageUtils.GetFormatter().(*utils.MessageFormatter)
	fileType := ""
	if fileExt != "" {
		fileType = strings.ToUpper(fileExt[1:])
	}

	opData := utils.FileOperationData{
		Icon:       fileIcon,
		FileName:   fileName,
		FilePath:   filepath.Dir(filePath),
		FileType:   fileType,
		Prompt:     "请选择操作：",
		EscapeHTML: h.controller.messageUtils.EscapeHTML,
	}
	message := formatter.FormatFileOperation(opData)

	isVideo := h.controller.fileService.IsVideoFile(fileName)

	var keyboardRows [][]tgbotapi.InlineKeyboardButton

	keyboardRows = append(keyboardRows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("📥 立即下载", fmt.Sprintf("file_download:%s", h.controller.common.EncodeFilePath(filePath))),
		tgbotapi.NewInlineKeyboardButtonData("ℹ️ 文件信息", fmt.Sprintf("file_info:%s", h.controller.common.EncodeFilePath(filePath))),
	))

	keyboardRows = append(keyboardRows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔗 获取链接", fmt.Sprintf("file_link:%s", h.controller.common.EncodeFilePath(filePath))),
	))

	if isVideo {
		keyboardRows = append(keyboardRows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✏️ 智能重命名", fmt.Sprintf("file_rename:%s", h.controller.common.EncodeFilePath(filePath))),
		))
	}

	keyboardRows = append(keyboardRows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🗑️ 删除文件", fmt.Sprintf("file_delete_confirm:%s", h.controller.common.EncodeFilePath(filePath))),
	))

	keyboardRows = append(keyboardRows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("📁 返回目录", fmt.Sprintf("browse_dir:%s:%d", h.controller.common.EncodeFilePath(h.getParentPath(filePath)), 1)),
		tgbotapi.NewInlineKeyboardButtonData("🏠 主菜单", "back_main"),
	))

	keyboard := tgbotapi.NewInlineKeyboardMarkup(keyboardRows...)

	if messageID > 0 {
		h.controller.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
	} else {
		h.controller.messageUtils.SendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
	}
}

func (h *FileHandler) HandleDirMenu(chatID int64, dirPath string) {
	h.HandleDirMenuWithEdit(chatID, dirPath, 0)
}

func (h *FileHandler) HandleDirMenuWithEdit(chatID int64, dirPath string, messageID int) {
	dirName := filepath.Base(dirPath)
	if dirPath == "/" {
		dirName = "根目录"
	}

	formatter := h.controller.messageUtils.GetFormatter().(*utils.MessageFormatter)

	opData := utils.FileOperationData{
		Icon:       "📁",
		FileName:   dirName,
		FilePath:   filepath.Dir(dirPath),
		FileType:   "目录",
		Prompt:     "请选择操作：",
		EscapeHTML: h.controller.messageUtils.EscapeHTML,
	}
	message := formatter.FormatFileOperation(opData)

	var keyboardRows [][]tgbotapi.InlineKeyboardButton

	keyboardRows = append(keyboardRows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("📂 进入目录", fmt.Sprintf("browse_dir:%s:%d", h.controller.common.EncodeFilePath(dirPath), 1)),
		tgbotapi.NewInlineKeyboardButtonData("📥 下载目录", fmt.Sprintf("download_dir:%s", h.controller.common.EncodeFilePath(dirPath))),
	))

	keyboardRows = append(keyboardRows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("📝 批量重命名", fmt.Sprintf("batch_rename:%s", h.controller.common.EncodeFilePath(dirPath))),
	))

	if dirPath != "/" {
		keyboardRows = append(keyboardRows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🗑️ 删除目录", fmt.Sprintf("dir_delete_confirm:%s", h.controller.common.EncodeFilePath(dirPath))),
		))
	}

	keyboardRows = append(keyboardRows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("📁 返回上级", fmt.Sprintf("browse_dir:%s:%d", h.controller.common.EncodeFilePath(h.getParentPath(dirPath)), 1)),
		tgbotapi.NewInlineKeyboardButtonData("🏠 主菜单", "back_main"),
	))

	keyboard := tgbotapi.NewInlineKeyboardMarkup(keyboardRows...)

	if messageID > 0 {
		h.controller.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
	} else {
		h.controller.messageUtils.SendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
	}
}

// HandleFileDownload handles file download (using /downloads command mechanism)
func (h *FileHandler) HandleFileDownload(chatID int64, filePath string) {
	// Directly call new /downloads command based file download handler
	h.handleDownloadFileByPath(chatID, filePath)
}

// handleDownloadFileByPath downloads a single file by path
func (h *FileHandler) handleDownloadFileByPath(chatID int64, filePath string) {
	// Get file info using file service (uniformly use getFilesFromPath to ensure path consistency)
	parentDir := filepath.Dir(filePath)
	fileName := filepath.Base(filePath)

	// Get file information using file service's smart classification
	fileInfo, err := h.getFilesFromPath(parentDir, false)
	if err != nil {
		formatter := h.controller.messageUtils.GetFormatter().(*utils.MessageFormatter)
		h.controller.messageUtils.SendMessage(chatID, formatter.FormatError("获取文件信息", err))
		return
	}

	// Find corresponding file information
	var targetFileInfo *contracts.FileResponse
	for _, info := range fileInfo {
		if info.Name == fileName {
			targetFileInfo = &info
			break
		}
	}

	if targetFileInfo == nil {
		formatter := h.controller.messageUtils.GetFormatter().(*utils.MessageFormatter)
		h.controller.messageUtils.SendMessage(chatID, formatter.FormatSimpleError("文件未找到"))
		return
	}

	// Create download task - using contracts interface
	downloadReq := contracts.DownloadRequest{
		URL:         targetFileInfo.InternalURL,
		Filename:    targetFileInfo.Name,
		Directory:   targetFileInfo.DownloadPath,
		AutoClassify: true,
	}

	ctx := context.Background()
	download, err := h.controller.downloadService.CreateDownload(ctx, downloadReq)
	if err != nil {
		formatter := h.controller.messageUtils.GetFormatter().(*utils.MessageFormatter)
		h.controller.messageUtils.SendMessage(chatID, formatter.FormatError("创建下载任务", err))
		return
	}

	// Use unified formatter to send success message
	formatter := h.controller.messageUtils.GetFormatter().(*utils.MessageFormatter)
	message := formatter.FormatFileDownloadSuccess(utils.FileDownloadSuccessData{
		Filename:     targetFileInfo.Name,
		FilePath:     filePath,
		DownloadPath: targetFileInfo.DownloadPath,
		TaskID:       download.ID,
		Size:         h.controller.messageUtils.FormatFileSize(targetFileInfo.Size),
		EscapeHTML:   h.controller.messageUtils.EscapeHTML,
	})

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📥 下载管理", "download_list"),
			tgbotapi.NewInlineKeyboardButtonData("📁 返回目录", fmt.Sprintf("browse_dir:%s:%d", h.controller.common.EncodeFilePath(parentDir), 1)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🏠 主菜单", "back_main"),
		),
	)

	h.controller.messageUtils.SendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
}

func (h *FileHandler) HandleDirDeleteConfirm(chatID int64, dirPath string, messageID int) {
	dirName := filepath.Base(dirPath)
	parentDir := filepath.Dir(dirPath)

	formatter := h.controller.messageUtils.GetFormatter().(*utils.MessageFormatter)
	message := formatter.FormatTitle("⚠️", "确认删除目录") + "\n\n" +
		formatter.FormatFieldCode("目录名", h.controller.messageUtils.EscapeHTML(dirName)) + "\n" +
		formatter.FormatFieldCode("路径", h.controller.messageUtils.EscapeHTML(parentDir)) + "\n\n" +
		"<b>⚠️ 此操作不可撤销，将删除目录及其所有内容，确认删除吗？</b>"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ 确认删除", fmt.Sprintf("dir_delete:%s", h.controller.common.EncodeFilePath(dirPath))),
			tgbotapi.NewInlineKeyboardButtonData("❌ 取消", fmt.Sprintf("dir_menu:%s", h.controller.common.EncodeFilePath(dirPath))),
		),
	)

	if messageID > 0 {
		h.controller.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
	} else {
		h.controller.messageUtils.SendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
	}
}

func (h *FileHandler) HandleDirDelete(chatID int64, dirPath string, messageID int) {
	dirName := filepath.Base(dirPath)
	parentDir := filepath.Dir(dirPath)

	ctx := context.Background()
	if err := h.controller.fileService.DeleteFile(ctx, dirPath); err != nil {
		formatter := h.controller.messageUtils.GetFormatter().(*utils.MessageFormatter)
		h.controller.messageUtils.SendMessage(chatID, formatter.FormatError("删除目录", err))
		return
	}

	formatter := h.controller.messageUtils.GetFormatter().(*utils.MessageFormatter)
	message := formatter.FormatTitle("✅", "目录删除成功") + "\n\n" +
		formatter.FormatFieldCode("目录名", h.controller.messageUtils.EscapeHTML(dirName)) + "\n" +
		formatter.FormatFieldCode("原路径", h.controller.messageUtils.EscapeHTML(parentDir))

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📁 返回上级", fmt.Sprintf("browse_dir:%s:%d", h.controller.common.EncodeFilePath(parentDir), 1)),
			tgbotapi.NewInlineKeyboardButtonData("🏠 主菜单", "back_main"),
		),
	)

	if messageID > 0 {
		h.controller.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
	} else {
		h.controller.messageUtils.SendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
	}
}

// HandleFileInfo handles file information viewing
func (h *FileHandler) HandleFileInfo(chatID int64, filePath string) {
	h.HandleFileInfoWithEdit(chatID, filePath, 0) // 0 means send new message
}

// HandleFileInfoWithEdit handles file information viewing (supports message editing)
func (h *FileHandler) HandleFileInfoWithEdit(chatID int64, filePath string, messageID int) {
	// Show loading message (only when sending new message)
	if messageID == 0 {
		h.controller.messageUtils.SendMessage(chatID, "正在获取文件信息...")
	}

	// Get file information
	fileInfo, err := h.listFilesSimple(filepath.Dir(filePath), 1, 1000)
	if err != nil {
		message := "获取文件信息失败: " + err.Error()
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("返回", fmt.Sprintf("browse_dir:%s:%d", h.controller.common.EncodeFilePath(filepath.Dir(filePath)), 1)),
			),
		)
		if messageID > 0 {
			h.controller.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
		} else {
			h.controller.messageUtils.SendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
		}
		return
	}

	// Find corresponding file
	var targetFile *contracts.FileResponse
	fileName := filepath.Base(filePath)
	for _, file := range fileInfo {
		if file.Name == fileName {
			targetFile = &file
			break
		}
	}

	if targetFile == nil {
		message := "文件未找到"
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("返回", fmt.Sprintf("browse_dir:%s:%d", h.controller.common.EncodeFilePath(filepath.Dir(filePath)), 1)),
			),
		)
		if messageID > 0 {
			h.controller.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
		} else {
			h.controller.messageUtils.SendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
		}
		return
	}

	// Use file's modification time
	modTime := targetFile.Modified

	// Determine file type
	fileType := "其他文件"
	if h.controller.fileService.IsVideoFile(targetFile.Name) {
		fileType = "视频文件"
	}

	// Use unified formatter
	formatter := h.controller.messageUtils.GetFormatter().(*utils.MessageFormatter)
	infoData := utils.FileInfoData{
		Icon:       "ℹ️",
		Name:       targetFile.Name,
		Path:       filePath,
		Type:       fileType,
		Size:       h.controller.messageUtils.FormatFileSize(targetFile.Size),
		Modified:   modTime.Format("2006-01-02 15:04:05"),
		IsDir:      targetFile.IsDir,
		EscapeHTML: h.controller.messageUtils.EscapeHTML,
	}

	// Build info message
	message := formatter.FormatFileInfo(infoData)

	// Add return button
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("返回", fmt.Sprintf("browse_dir:%s:%d", h.controller.common.EncodeFilePath(filepath.Dir(filePath)), 1)),
		),
	)

	if messageID > 0 {
		h.controller.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
	} else {
		h.controller.messageUtils.SendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
	}
}

// HandleFileLink handles getting file link
func (h *FileHandler) HandleFileLink(chatID int64, filePath string) {
	h.HandleFileLinkWithEdit(chatID, filePath, 0) // 0 means send new message
}

// HandleFileLinkWithEdit handles getting file link (supports message editing)
func (h *FileHandler) HandleFileLinkWithEdit(chatID int64, filePath string, messageID int) {
	// Show loading message (only when sending new message)
	if messageID == 0 {
		h.controller.messageUtils.SendMessage(chatID, "正在获取文件链接...")
	}

	// Get file download link
	downloadURL := h.getFileDownloadURL(filepath.Dir(filePath), filepath.Base(filePath))

	// Use unified formatter
	formatter := h.controller.messageUtils.GetFormatter().(*utils.MessageFormatter)
	var lines []string

	// Title
	lines = append(lines, formatter.FormatTitle("🔗", "文件链接"))
	lines = append(lines, "")

	// File information
	lines = append(lines, formatter.FormatFieldCode("文件", h.controller.messageUtils.EscapeHTML(filepath.Base(filePath))))
	lines = append(lines, "")

	// Download link
	lines = append(lines, formatter.FormatField("下载链接", ""))
	lines = append(lines, fmt.Sprintf("<code>%s</code>", h.controller.messageUtils.EscapeHTML(downloadURL)))

	message := strings.Join(lines, "\n")

	// Add return button
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("返回", fmt.Sprintf("browse_dir:%s:%d", h.controller.common.EncodeFilePath(filepath.Dir(filePath)), 1)),
		),
	)

	if messageID > 0 {
		h.controller.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
	} else {
		h.controller.messageUtils.SendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
	}
}

// HandleDownloadDirectory handles directory download (using /downloads command mechanism)
func (h *FileHandler) HandleDownloadDirectory(chatID int64, dirPath string) {
	// Directly call new /downloads command based directory download handler
	h.handleDownloadDirectoryByPath(chatID, dirPath)
}

// handleDownloadDirectoryByPath downloads directory by path - using refactored new architecture
func (h *FileHandler) handleDownloadDirectoryByPath(chatID int64, dirPath string) {
	ctx := context.Background()

	formatter := h.controller.messageUtils.GetFormatter().(*utils.MessageFormatter)
	processingMsg := formatter.FormatTitle("⏳", "正在处理手动下载任务") + "\n\n" +
		formatter.FormatField("目录路径", dirPath)
	h.controller.messageUtils.SendMessageHTMLWithAutoDelete(chatID, processingMsg, 30)

	req := contracts.DirectoryDownloadRequest{
		DirectoryPath: dirPath,
		Recursive:     true,
		VideoOnly:     true,  // Only download video files
		AutoClassify:  true,
	}
	
	result, err := h.controller.fileService.DownloadDirectory(ctx, req)
	if err != nil {
		h.controller.messageUtils.SendMessage(chatID, formatter.FormatError("处理", err))
		return
	}

	if result.SuccessCount == 0 {
		if result.Summary.VideoFiles == 0 {
			message := formatter.FormatNoFilesFound("手动下载完成", dirPath)
			h.controller.messageUtils.SendMessageHTML(chatID, message)
		} else {
			h.controller.messageUtils.SendMessage(chatID, formatter.FormatSimpleError("所有文件下载创建失败，请检查日志"))
		}
		return
	}

	message := formatter.FormatTimeRangeDownloadResult(utils.TimeRangeDownloadResultData{
		TimeDescription: dirPath,
		Path:            dirPath,
		TotalFiles:      result.Summary.TotalFiles,
		TotalSize:       h.controller.messageUtils.FormatFileSize(result.Summary.TotalSize),
		MovieCount:      result.Summary.MovieFiles,
		TVCount:         result.Summary.TVFiles,
		OtherCount:      result.Summary.OtherFiles,
		SuccessCount:    result.SuccessCount,
		FailCount:       result.FailureCount,
		EscapeHTML:      h.controller.messageUtils.EscapeHTML,
	})

	h.controller.messageUtils.SendMessageHTMLWithAutoDelete(chatID, message, 30)
}

// sendBatchDownloadResult sends batch download result message - new architecture format
func (h *FileHandler) sendBatchDownloadResult(chatID int64, dirPath string, result *contracts.BatchDownloadResponse) {
	// Prevent nil pointer dereference
	if result == nil {
		h.controller.messageUtils.SendMessage(chatID, "❌ 批量下载结果为空")
		return
	}

	// Use unified formatter
	formatter := h.controller.messageUtils.GetFormatter().(*utils.MessageFormatter)
	var lines []string

	// Title
	lines = append(lines, formatter.FormatTitle("📊", "目录下载任务创建完成"))
	lines = append(lines, "")

	// Basic information
	lines = append(lines, formatter.FormatFieldCode("目录", h.controller.messageUtils.EscapeHTML(dirPath)))
	lines = append(lines, formatter.FormatField("扫描文件", fmt.Sprintf("%d 个", result.Summary.TotalFiles)))
	lines = append(lines, formatter.FormatField("视频文件", fmt.Sprintf("%d 个", result.Summary.VideoFiles)))
	lines = append(lines, formatter.FormatField("成功创建", fmt.Sprintf("%d 个任务", result.SuccessCount)))
	lines = append(lines, formatter.FormatField("失败", fmt.Sprintf("%d 个任务", result.FailureCount)))

	// Classification statistics
	if result.Summary.MovieFiles > 0 || result.Summary.TVFiles > 0 {
		lines = append(lines, "")
		if result.Summary.MovieFiles > 0 {
			lines = append(lines, formatter.FormatField("电影", fmt.Sprintf("%d 个", result.Summary.MovieFiles)))
		}
		if result.Summary.TVFiles > 0 {
			lines = append(lines, formatter.FormatField("电视剧", fmt.Sprintf("%d 个", result.Summary.TVFiles)))
		}
	}

	// Failed file details
	if result.FailureCount > 0 && len(result.Results) <= 3 {
		lines = append(lines, "")
		lines = append(lines, formatter.FormatSection("失败的文件"))
		failedCount := 0
		for _, downloadResult := range result.Results {
			if !downloadResult.Success && failedCount < 3 {
				filename := "未知文件"
				if downloadResult.Request.Filename != "" {
					filename = downloadResult.Request.Filename
				}
				lines = append(lines, formatter.FormatListItem("•", fmt.Sprintf("<code>%s</code>", h.controller.messageUtils.EscapeHTML(filename))))
				failedCount++
			}
		}
	} else if result.FailureCount > 3 {
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("⚠️ 有 %d 个文件下载失败", result.FailureCount))
	}

	// Success message
	if result.SuccessCount > 0 {
		lines = append(lines, "")
		lines = append(lines, "✅ 所有任务已使用自动路径分类功能")
		lines = append(lines, "📥 可通过「下载管理」查看任务状态")
	}

	message := strings.Join(lines, "\n")
	// Send message, auto-delete after 20 seconds
	h.controller.messageUtils.SendMessageHTMLWithAutoDelete(chatID, message, 20)
}

// ================================
// File browsing menu functionality
// ================================

// HandleFilesBrowseWithEdit handles file browsing (supports message editing)
func (h *FileHandler) HandleFilesBrowseWithEdit(chatID int64, messageID int) {
	// Start browsing with default path or root directory
	defaultPath := h.controller.config.Alist.DefaultPath
	if defaultPath == "" {
		defaultPath = "/"
	}
	h.HandleBrowseFilesWithEdit(chatID, defaultPath, 1, messageID)
}

// HandleAlistFilesWithEdit handles getting Alist file list (supports message editing)
func (h *FileHandler) HandleAlistFilesWithEdit(chatID int64, messageID int) {
	h.HandleBrowseFilesWithEdit(chatID, h.controller.config.Alist.DefaultPath, 1, messageID)
}

// ================================
// Helper methods - Compatibility adaptation
// ================================

// listFilesSimple lists files simply - adapting to contracts.FileService interface
func (h *FileHandler) listFilesSimple(path string, page, perPage int) ([]contracts.FileResponse, error) {
	req := contracts.FileListRequest{
		Path:     path,
		Page:     page,
		PageSize: perPage,
	}
	
	ctx := context.Background()
	resp, err := h.controller.fileService.ListFiles(ctx, req)
	if err != nil {
		return nil, err
	}

	// Merge files and directories
	var allItems []contracts.FileResponse
	allItems = append(allItems, resp.Directories...)
	allItems = append(allItems, resp.Files...)
	
	return allItems, nil
}

// getFilesFromPath gets files from specified path - adapting to contracts.FileService interface
func (h *FileHandler) getFilesFromPath(basePath string, recursive bool) ([]contracts.FileResponse, error) {
	req := contracts.FileListRequest{
		Path:      basePath,
		Recursive: recursive,
		PageSize:  10000, // Get all files
	}
	
	ctx := context.Background()
	resp, err := h.controller.fileService.ListFiles(ctx, req)
	if err != nil {
		return nil, err
	}
	
	return resp.Files, nil
}

// getFileDownloadURL gets file download URL - adapting to contracts.FileService interface
func (h *FileHandler) getFileDownloadURL(path, fileName string) string {
	// Build full path
	fullPath := path + "/" + fileName
	if path == "/" {
		fullPath = "/" + fileName
	}

	ctx := context.Background()
	fileInfo, err := h.controller.fileService.GetFileInfo(ctx, fullPath)
	if err != nil {
		// If fetch fails, fallback to directly building URL
		return h.controller.config.Alist.BaseURL + "/d" + fullPath
	}

	return fileInfo.InternalURL
}

// getParentPath gets parent directory path
func (h *FileHandler) getParentPath(path string) string {
	if path == "/" {
		return "/"
	}
	parentPath := filepath.Dir(path)
	if parentPath == "." {
		return "/"
	}
	return parentPath
}

// DirectoryDownloadStats directory download statistics - retained for compatibility
type DirectoryDownloadStats struct {
	TotalFiles   int
	VideoFiles   int
	TotalSize    int64
	MovieCount   int
	TVCount      int
	OtherCount   int
	TotalSizeStr string
}

// DirectoryDownloadResult directory download result - retained for compatibility
type DirectoryDownloadResult struct {
	Stats        DirectoryDownloadStats
	SuccessCount int
	FailedCount  int
	FailedFiles  []string
}

func (h *FileHandler) HandleFileDeleteConfirm(chatID int64, filePath string, messageID int) {
	fileName := filepath.Base(filePath)
	parentDir := filepath.Dir(filePath)

	formatter := h.controller.messageUtils.GetFormatter().(*utils.MessageFormatter)
	message := formatter.FormatTitle("⚠️", "确认删除文件") + "\n\n" +
		formatter.FormatFieldCode("文件名", h.controller.messageUtils.EscapeHTML(fileName)) + "\n" +
		formatter.FormatFieldCode("路径", h.controller.messageUtils.EscapeHTML(parentDir)) + "\n\n" +
		"<b>⚠️ 此操作不可撤销，确认删除吗？</b>"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ 确认删除", fmt.Sprintf("file_delete:%s", h.controller.common.EncodeFilePath(filePath))),
			tgbotapi.NewInlineKeyboardButtonData("❌ 取消", fmt.Sprintf("file_menu:%s", h.controller.common.EncodeFilePath(filePath))),
		),
	)

	if messageID > 0 {
		h.controller.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
	} else {
		h.controller.messageUtils.SendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
	}
}

func (h *FileHandler) HandleFileDelete(chatID int64, filePath string, messageID int) {
	fileName := filepath.Base(filePath)
	parentDir := filepath.Dir(filePath)

	ctx := context.Background()
	if err := h.controller.fileService.DeleteFile(ctx, filePath); err != nil {
		formatter := h.controller.messageUtils.GetFormatter().(*utils.MessageFormatter)
		h.controller.messageUtils.SendMessage(chatID, formatter.FormatError("删除文件", err))
		return
	}

	formatter := h.controller.messageUtils.GetFormatter().(*utils.MessageFormatter)
	message := formatter.FormatTitle("✅", "文件删除成功") + "\n\n" +
		formatter.FormatFieldCode("文件名", h.controller.messageUtils.EscapeHTML(fileName)) + "\n" +
		formatter.FormatFieldCode("原路径", h.controller.messageUtils.EscapeHTML(parentDir))

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📁 返回目录", fmt.Sprintf("browse_dir:%s:%d", h.controller.common.EncodeFilePath(parentDir), 1)),
			tgbotapi.NewInlineKeyboardButtonData("🏠 主菜单", "back_main"),
		),
	)

	if messageID > 0 {
		h.controller.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
	} else {
		h.controller.messageUtils.SendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
	}
}