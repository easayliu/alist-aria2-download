package telegram

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/easayliu/alist-aria2-download/internal/api/handlers/telegram/utils"
	"github.com/easayliu/alist-aria2-download/internal/application/contracts"
	"github.com/easayliu/alist-aria2-download/pkg/logger"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// FileHandler 处理文件浏览相关功能
type FileHandler struct {
	controller *TelegramController
}

// NewFileHandler 创建新的文件处理器
func NewFileHandler(controller *TelegramController) *FileHandler {
	return &FileHandler{
		controller: controller,
	}
}

// ================================
// 文件浏览功能
// ================================

// HandleBrowseFiles 处理文件浏览（支持分页和交互）
func (h *FileHandler) HandleBrowseFiles(chatID int64, path string, page int) {
	h.HandleBrowseFilesWithEdit(chatID, path, page, 0) // 0 表示发送新消息
}

// HandleBrowseFilesWithEdit 处理文件浏览（支持编辑消息和分页）
func (h *FileHandler) HandleBrowseFilesWithEdit(chatID int64, path string, page int, messageID int) {
	if path == "" {
		path = "/"
	}
	if page < 1 {
		page = 1
	}

	// 调试日志
	logger.Info("浏览文件", "path", path, "page", page, "messageID", messageID)

	// 只在发送新消息时显示提示
	if messageID == 0 {
		h.controller.messageUtils.SendMessage(chatID, "正在获取文件列表...")
	}

	// 获取文件列表 (每页显示8个文件，为按钮布局留出空间)
	files, err := h.listFilesSimple(path, page, 8)
	if err != nil {
		formatter := h.controller.messageUtils.GetFormatter().(*utils.MessageFormatter)
		h.controller.messageUtils.SendMessage(chatID, formatter.FormatError("获取文件列表", err))
		return
	}

	if len(files) == 0 {
		h.controller.messageUtils.SendMessage(chatID, "当前目录为空")
		return
	}

	// 统计文件信息
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

	// 使用统一格式化器
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

	// 构建内联键盘
	var keyboard [][]tgbotapi.InlineKeyboardButton

	for _, file := range files {
		var prefix string
		var callbackData string

		if file.IsDir {
			prefix = "📁"
			// 目录点击：进入子目录
			// 构建完整路径
			var fullPath string
			if file.Path != "" {
				fullPath = file.Path
			} else {
				if path == "/" {
					fullPath = "/" + file.Name
				} else {
					fullPath = path + "/" + file.Name
				}
			}
			callbackData = fmt.Sprintf("browse_dir:%s:%d", h.controller.common.EncodeFilePath(fullPath), 1)
		} else if h.controller.fileService.IsVideoFile(file.Name) {
			prefix = "🎬"
			// 视频文件点击：显示操作菜单
			// 构建完整路径
			var fullPath string
			if file.Path != "" {
				fullPath = file.Path
			} else {
				if path == "/" {
					fullPath = "/" + file.Name
				} else {
					fullPath = path + "/" + file.Name
				}
			}
			callbackData = fmt.Sprintf("file_menu:%s", h.controller.common.EncodeFilePath(fullPath))
		} else {
			prefix = "📄"
			// 其他文件点击：显示操作菜单
			// 构建完整路径
			var fullPath string
			if file.Path != "" {
				fullPath = file.Path
			} else {
				if path == "/" {
					fullPath = "/" + file.Name
				} else {
					fullPath = path + "/" + file.Name
				}
			}
			callbackData = fmt.Sprintf("file_menu:%s", h.controller.common.EncodeFilePath(fullPath))
		}

		fileName := file.Name
		// 使用智能截断，考虑中英文字符宽度
		// 确保按钮宽度与消息内容宽度一致（42字符）
		// emoji (📁/📄) 约占 2 字符
		maxWidth := 38  // 目录行: emoji(2) + 空格(1) + 文件名(38) = 41字符
		if !file.IsDir {
			maxWidth = 30 // 文件行: emoji(2) + 空格(1) + 文件名(30) + 按钮(📥约2) = 35字符
		}

		// 使用 formatter 的 TruncateButtonText 方法，考虑 emoji 占用
		formatter := h.controller.messageUtils.GetFormatter().(*utils.MessageFormatter)
		fileName = formatter.TruncateButtonText(fileName, maxWidth)

		button := tgbotapi.NewInlineKeyboardButtonData(
			fmt.Sprintf("%s %s", prefix, fileName),
			callbackData,
		)

		// 为文件（非目录）添加快捷下载按钮
		if !file.IsDir {
			// 文件行：文件名按钮 + 快捷下载按钮
			var fullPath string
			if file.Path != "" {
				fullPath = file.Path
			} else {
				if path == "/" {
					fullPath = "/" + file.Name
				} else {
					fullPath = path + "/" + file.Name
				}
			}

			downloadButton := tgbotapi.NewInlineKeyboardButtonData(
				"📥",
				fmt.Sprintf("file_download:%s", h.controller.common.EncodeFilePath(fullPath)),
			)

			keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{button, downloadButton})
		} else {
			// 目录行：只有目录按钮，占满整行
			keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{button})
		}
	}

	// 添加导航按钮
	navButtons := []tgbotapi.InlineKeyboardButton{}

	// 上一页按钮
	if page > 1 {
		navButtons = append(navButtons, tgbotapi.NewInlineKeyboardButtonData(
			"< 上一页",
			fmt.Sprintf("browse_page:%s:%d", h.controller.common.EncodeFilePath(path), page-1),
		))
	}

	// 下一页按钮 (如果当前页满了，可能还有下一页)
	if len(files) == 8 {
		navButtons = append(navButtons, tgbotapi.NewInlineKeyboardButtonData(
			"下一页 >",
			fmt.Sprintf("browse_page:%s:%d", h.controller.common.EncodeFilePath(path), page+1),
		))
	}

	if len(navButtons) > 0 {
		keyboard = append(keyboard, navButtons)
	}

	// 添加功能按钮 - 第一行：下载和刷新
	actionRow1 := []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("📥 下载目录", fmt.Sprintf("download_dir:%s", h.controller.common.EncodeFilePath(path))),
		tgbotapi.NewInlineKeyboardButtonData("🔄 刷新", fmt.Sprintf("browse_refresh:%s:%d", h.controller.common.EncodeFilePath(path), page)),
	}
	keyboard = append(keyboard, actionRow1)

	// 添加导航按钮 - 第二行：上级目录和主菜单
	actionRow2 := []tgbotapi.InlineKeyboardButton{}

	// 返回上级目录按钮
	if path != "/" {
		parentPath := h.getParentPath(path)
		actionRow2 = append(actionRow2, tgbotapi.NewInlineKeyboardButtonData(
			"⬆️ 上级目录",
			fmt.Sprintf("browse_dir:%s:%d", h.controller.common.EncodeFilePath(parentPath), 1),
		))
	}

	// 返回主菜单按钮
	actionRow2 = append(actionRow2, tgbotapi.NewInlineKeyboardButtonData("🏠 主菜单", "back_main"))

	if len(actionRow2) > 0 {
		keyboard = append(keyboard, actionRow2)
	}

	inlineKeyboard := tgbotapi.NewInlineKeyboardMarkup(keyboard...)

	if messageID > 0 {
		// 编辑现有消息
		h.controller.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &inlineKeyboard)
	} else {
		// 发送新消息
		h.controller.messageUtils.SendMessageWithKeyboard(chatID, message, "HTML", &inlineKeyboard)
	}
}

// HandleFileMenu 处理文件操作菜单
func (h *FileHandler) HandleFileMenu(chatID int64, filePath string) {
	h.HandleFileMenuWithEdit(chatID, filePath, 0) // 0 表示发送新消息
}

// HandleFileMenuWithEdit 处理文件操作菜单（支持消息编辑）
func (h *FileHandler) HandleFileMenuWithEdit(chatID int64, filePath string, messageID int) {
	// 获取文件信息
	fileName := filepath.Base(filePath)
	fileExt := strings.ToLower(filepath.Ext(fileName))

	// 根据文件类型选择图标
	var fileIcon string
	if h.controller.fileService.IsVideoFile(fileName) {
		fileIcon = "🎬"
	} else {
		fileIcon = "📄"
	}

	// 使用统一格式化器
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

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📥 立即下载", fmt.Sprintf("file_download:%s", h.controller.common.EncodeFilePath(filePath))),
			tgbotapi.NewInlineKeyboardButtonData("ℹ️ 文件信息", fmt.Sprintf("file_info:%s", h.controller.common.EncodeFilePath(filePath))),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔗 获取链接", fmt.Sprintf("file_link:%s", h.controller.common.EncodeFilePath(filePath))),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📁 返回目录", fmt.Sprintf("browse_dir:%s:%d", h.controller.common.EncodeFilePath(h.getParentPath(filePath)), 1)),
			tgbotapi.NewInlineKeyboardButtonData("🏠 主菜单", "back_main"),
		),
	)

	if messageID > 0 {
		// 编辑现有消息
		h.controller.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
	} else {
		// 发送新消息
		h.controller.messageUtils.SendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
	}
}

// HandleFileDownload 处理文件下载（使用/downloads命令机制）
func (h *FileHandler) HandleFileDownload(chatID int64, filePath string) {
	// 直接调用新的基于/downloads命令的文件下载处理函数
	h.handleDownloadFileByPath(chatID, filePath)
}

// handleDownloadFileByPath 通过路径下载单个文件
func (h *FileHandler) handleDownloadFileByPath(chatID int64, filePath string) {
	// 使用文件服务获取文件信息
	parentDir := filepath.Dir(filePath)
	fileName := filepath.Base(filePath)

	files, err := h.listFilesSimple(parentDir, 1, 1000)
	if err != nil {
		formatter := h.controller.messageUtils.GetFormatter().(*utils.MessageFormatter)
		h.controller.messageUtils.SendMessage(chatID, formatter.FormatError("获取文件信息", err))
		return
	}

	// 查找目标文件
	var targetFile *contracts.FileResponse
	for _, file := range files {
		if file.Name == fileName {
			targetFile = &file
			break
		}
	}

	if targetFile == nil {
		formatter := h.controller.messageUtils.GetFormatter().(*utils.MessageFormatter)
		h.controller.messageUtils.SendMessage(chatID, formatter.FormatSimpleError("文件未找到"))
		return
	}

	// 使用文件服务的智能分类功能
	fileInfo, err := h.getFilesFromPath(parentDir, false)
	if err != nil {
		formatter := h.controller.messageUtils.GetFormatter().(*utils.MessageFormatter)
		h.controller.messageUtils.SendMessage(chatID, formatter.FormatError("获取文件详细信息", err))
		return
	}

	// 找到对应的文件信息
	var targetFileInfo *contracts.FileResponse
	for _, info := range fileInfo {
		if info.Name == fileName {
			targetFileInfo = &info
			break
		}
	}

	if targetFileInfo == nil {
		formatter := h.controller.messageUtils.GetFormatter().(*utils.MessageFormatter)
		h.controller.messageUtils.SendMessage(chatID, formatter.FormatSimpleError("获取文件分类信息失败"))
		return
	}

	// 创建下载任务 - 使用contracts接口
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

	// 使用统一格式化器发送成功消息
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

// HandleFileInfo 处理文件信息查看
func (h *FileHandler) HandleFileInfo(chatID int64, filePath string) {
	h.HandleFileInfoWithEdit(chatID, filePath, 0) // 0 表示发送新消息
}

// HandleFileInfoWithEdit 处理文件信息查看（支持消息编辑）
func (h *FileHandler) HandleFileInfoWithEdit(chatID int64, filePath string, messageID int) {
	// 显示加载消息（仅在发送新消息时）
	if messageID == 0 {
		h.controller.messageUtils.SendMessage(chatID, "正在获取文件信息...")
	}

	// 获取文件信息
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

	// 查找对应的文件
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

	// 使用文件的修改时间
	modTime := targetFile.Modified

	// 判断文件类型
	fileType := "其他文件"
	if h.controller.fileService.IsVideoFile(targetFile.Name) {
		fileType = "视频文件"
	}

	// 使用统一格式化器
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

	// 构建信息消息
	message := formatter.FormatFileInfo(infoData)

	// 添加返回按钮
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

// HandleFileLink 处理获取文件链接
func (h *FileHandler) HandleFileLink(chatID int64, filePath string) {
	h.HandleFileLinkWithEdit(chatID, filePath, 0) // 0 表示发送新消息
}

// HandleFileLinkWithEdit 处理获取文件链接（支持消息编辑）
func (h *FileHandler) HandleFileLinkWithEdit(chatID int64, filePath string, messageID int) {
	// 显示加载消息（仅在发送新消息时）
	if messageID == 0 {
		h.controller.messageUtils.SendMessage(chatID, "正在获取文件链接...")
	}

	// 获取文件下载链接
	downloadURL := h.getFileDownloadURL(filepath.Dir(filePath), filepath.Base(filePath))

	// 使用统一格式化器
	formatter := h.controller.messageUtils.GetFormatter().(*utils.MessageFormatter)
	var lines []string

	// 标题
	lines = append(lines, formatter.FormatTitle("🔗", "文件链接"))
	lines = append(lines, "")

	// 文件信息
	lines = append(lines, formatter.FormatFieldCode("文件", h.controller.messageUtils.EscapeHTML(filepath.Base(filePath))))
	lines = append(lines, "")

	// 下载链接
	lines = append(lines, formatter.FormatField("下载链接", ""))
	lines = append(lines, fmt.Sprintf("<code>%s</code>", h.controller.messageUtils.EscapeHTML(downloadURL)))

	message := strings.Join(lines, "\n")

	// 添加返回按钮
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

// HandleDownloadDirectory 处理目录下载（使用/downloads命令机制）
func (h *FileHandler) HandleDownloadDirectory(chatID int64, dirPath string) {
	// 直接调用新的基于/downloads命令的目录下载处理函数
	h.handleDownloadDirectoryByPath(chatID, dirPath)
}

// handleDownloadDirectoryByPath 通过路径下载目录 - 使用重构后的新架构
func (h *FileHandler) handleDownloadDirectoryByPath(chatID int64, dirPath string) {
	ctx := context.Background()
	
	// 使用新架构的目录下载服务
	req := contracts.DirectoryDownloadRequest{
		DirectoryPath: dirPath,
		Recursive:     true,
		VideoOnly:     true,  // 只下载视频文件
		AutoClassify:  true,
	}
	
	result, err := h.controller.fileService.DownloadDirectory(ctx, req)
	if err != nil {
		formatter := h.controller.messageUtils.GetFormatter().(*utils.MessageFormatter)
		h.controller.messageUtils.SendMessage(chatID, formatter.FormatError("扫描目录", err))
		return
	}
	
	if result.SuccessCount == 0 {
		formatter := h.controller.messageUtils.GetFormatter().(*utils.MessageFormatter)
		if result.Summary.VideoFiles == 0 {
			h.controller.messageUtils.SendMessage(chatID, "🎬 目录中没有找到视频文件")
		} else {
			h.controller.messageUtils.SendMessage(chatID, formatter.FormatSimpleError("所有文件下载创建失败，请检查日志"))
		}
		return
	}
	
	// 发送结果消息（使用新架构的结果格式）
	h.sendBatchDownloadResult(chatID, dirPath, result)
}

// sendBatchDownloadResult 发送批量下载结果消息 - 新架构格式
func (h *FileHandler) sendBatchDownloadResult(chatID int64, dirPath string, result *contracts.BatchDownloadResponse) {
	// 防止空指针解引用
	if result == nil {
		h.controller.messageUtils.SendMessage(chatID, "❌ 批量下载结果为空")
		return
	}

	// 使用统一格式化器
	formatter := h.controller.messageUtils.GetFormatter().(*utils.MessageFormatter)
	var lines []string

	// 标题
	lines = append(lines, formatter.FormatTitle("📊", "目录下载任务创建完成"))
	lines = append(lines, "")

	// 基本信息
	lines = append(lines, formatter.FormatFieldCode("目录", h.controller.messageUtils.EscapeHTML(dirPath)))
	lines = append(lines, formatter.FormatField("扫描文件", fmt.Sprintf("%d 个", result.Summary.TotalFiles)))
	lines = append(lines, formatter.FormatField("视频文件", fmt.Sprintf("%d 个", result.Summary.VideoFiles)))
	lines = append(lines, formatter.FormatField("成功创建", fmt.Sprintf("%d 个任务", result.SuccessCount)))
	lines = append(lines, formatter.FormatField("失败", fmt.Sprintf("%d 个任务", result.FailureCount)))

	// 分类统计
	if result.Summary.MovieFiles > 0 || result.Summary.TVFiles > 0 {
		lines = append(lines, "")
		if result.Summary.MovieFiles > 0 {
			lines = append(lines, formatter.FormatField("电影", fmt.Sprintf("%d 个", result.Summary.MovieFiles)))
		}
		if result.Summary.TVFiles > 0 {
			lines = append(lines, formatter.FormatField("电视剧", fmt.Sprintf("%d 个", result.Summary.TVFiles)))
		}
	}

	// 失败文件详情
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

	// 成功提示
	if result.SuccessCount > 0 {
		lines = append(lines, "")
		lines = append(lines, "✅ 所有任务已使用自动路径分类功能")
		lines = append(lines, "📥 可通过「下载管理」查看任务状态")
	}

	message := strings.Join(lines, "\n")
	h.controller.messageUtils.SendMessageHTML(chatID, message)
}

// sendDirectoryDownloadResult 发送目录下载结果消息 - 为保持兼容性保留
func (h *FileHandler) sendDirectoryDownloadResult(chatID int64, dirPath string, result DirectoryDownloadResult) {
	// 构建消息数据
	resultData := utils.DirectoryDownloadResultData{
		DirectoryPath: dirPath,
		TotalFiles:    result.Stats.TotalFiles,
		VideoFiles:    result.Stats.VideoFiles,
		TotalSizeStr:  result.Stats.TotalSizeStr,
		MovieCount:    result.Stats.MovieCount,
		TVCount:       result.Stats.TVCount,
		OtherCount:    result.Stats.OtherCount,
		SuccessCount:  result.SuccessCount,
		FailedCount:   result.FailedCount,
		FailedFiles:   result.FailedFiles,
	}

	// 使用 MessageUtils 格式化消息
	message := h.controller.messageUtils.FormatDirectoryDownloadResult(resultData)

	// 创建回复键盘
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📥 下载管理", "download_list"),
			tgbotapi.NewInlineKeyboardButtonData("📁 返回目录", fmt.Sprintf("browse_dir:%s:%d", h.controller.common.EncodeFilePath(dirPath), 1)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🏠 主菜单", "back_main"),
		),
	)

	// 发送消息
	h.controller.messageUtils.SendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
}

// ================================
// 文件浏览菜单功能
// ================================

// HandleFilesBrowseWithEdit 处理文件浏览（支持消息编辑）
func (h *FileHandler) HandleFilesBrowseWithEdit(chatID int64, messageID int) {
	// 使用默认路径或根目录开始浏览
	defaultPath := h.controller.config.Alist.DefaultPath
	if defaultPath == "" {
		defaultPath = "/"
	}
	h.HandleBrowseFilesWithEdit(chatID, defaultPath, 1, messageID)
}

// HandleFilesSearchWithEdit 处理文件搜索（支持消息编辑）
func (h *FileHandler) HandleFilesSearchWithEdit(chatID int64, messageID int) {
	// 使用统一格式化器
	formatter := h.controller.messageUtils.GetFormatter().(*utils.MessageFormatter)
	var lines []string

	// 标题
	lines = append(lines, formatter.FormatTitle("🔍", "文件搜索功能"))
	lines = append(lines, "")

	// 搜索说明
	lines = append(lines, formatter.FormatSection("搜索说明"))
	lines = append(lines, formatter.FormatListItem("•", "支持文件名关键词搜索"))
	lines = append(lines, formatter.FormatListItem("•", "支持路径模糊匹配"))
	lines = append(lines, formatter.FormatListItem("•", "支持文件类型过滤"))
	lines = append(lines, "")

	// 使用说明
	lines = append(lines, formatter.FormatSection("请输入搜索关键词"))
	lines = append(lines, "格式: <code>/search &lt;关键词&gt;</code>")
	lines = append(lines, "")

	// 快速搜索
	lines = append(lines, formatter.FormatSection("快速搜索"))

	message := strings.Join(lines, "\n")

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("搜索电影", "search_movies"),
			tgbotapi.NewInlineKeyboardButtonData("搜索剧集", "search_tv"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("返回文件浏览", "menu_files"),
		),
	)

	h.controller.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
}

// HandleFilesInfoWithEdit 处理文件信息查看（支持消息编辑）
func (h *FileHandler) HandleFilesInfoWithEdit(chatID int64, messageID int) {
	// 使用统一格式化器
	formatter := h.controller.messageUtils.GetFormatter().(*utils.MessageFormatter)
	var lines []string

	// 标题
	lines = append(lines, formatter.FormatTitle("ℹ️", "文件信息查看"))
	lines = append(lines, "")

	// 可查看信息
	lines = append(lines, formatter.FormatSection("可查看信息"))
	lines = append(lines, formatter.FormatListItem("•", "文件基本属性"))
	lines = append(lines, formatter.FormatListItem("•", "文件大小和修改时间"))
	lines = append(lines, formatter.FormatListItem("•", "下载链接和路径"))
	lines = append(lines, formatter.FormatListItem("•", "媒体类型识别"))
	lines = append(lines, "")

	// 操作提示
	lines = append(lines, formatter.FormatSection("请选择操作方式"))

	message := strings.Join(lines, "\n")

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("浏览选择", "files_browse"),
			tgbotapi.NewInlineKeyboardButtonData("定时任务", "cmd_tasks"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("返回文件浏览", "menu_files"),
		),
	)

	h.controller.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
}

// HandleFilesDownloadWithEdit 处理路径下载功能（支持消息编辑）
func (h *FileHandler) HandleFilesDownloadWithEdit(chatID int64, messageID int) {
	// 使用统一格式化器
	formatter := h.controller.messageUtils.GetFormatter().(*utils.MessageFormatter)
	var lines []string

	// 标题
	lines = append(lines, formatter.FormatTitle("📥", "路径下载功能"))
	lines = append(lines, "")

	// 下载选项
	lines = append(lines, formatter.FormatSection("下载选项"))
	lines = append(lines, formatter.FormatListItem("•", "指定路径批量下载"))
	lines = append(lines, formatter.FormatListItem("•", "递归下载子目录"))
	lines = append(lines, formatter.FormatListItem("•", "预览模式（不下载）"))
	lines = append(lines, formatter.FormatListItem("•", "过滤文件类型"))
	lines = append(lines, "")

	// 使用格式
	lines = append(lines, formatter.FormatSection("使用格式"))
	lines = append(lines, "<code>/path_download /movies/2024</code>")
	lines = append(lines, "")

	// 快速下载
	lines = append(lines, formatter.FormatSection("快速下载"))

	message := strings.Join(lines, "\n")

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("定时任务", "cmd_tasks"),
			tgbotapi.NewInlineKeyboardButtonData("浏览下载", "files_browse"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("返回文件浏览", "menu_files"),
		),
	)

	h.controller.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
}

// HandleAlistFilesWithEdit 处理获取Alist文件列表（支持消息编辑）
func (h *FileHandler) HandleAlistFilesWithEdit(chatID int64, messageID int) {
	h.HandleBrowseFilesWithEdit(chatID, h.controller.config.Alist.DefaultPath, 1, messageID)
}

// ================================
// 辅助方法 - 兼容性适配
// ================================

// listFilesSimple 简单列出文件 - 适配contracts.FileService接口
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
	
	// 合并文件和目录
	var allItems []contracts.FileResponse
	allItems = append(allItems, resp.Directories...)
	allItems = append(allItems, resp.Files...)
	
	return allItems, nil
}

// getFilesFromPath 从指定路径获取文件 - 适配contracts.FileService接口
func (h *FileHandler) getFilesFromPath(basePath string, recursive bool) ([]contracts.FileResponse, error) {
	req := contracts.FileListRequest{
		Path:      basePath,
		Recursive: recursive,
		PageSize:  10000, // 获取所有文件
	}
	
	ctx := context.Background()
	resp, err := h.controller.fileService.ListFiles(ctx, req)
	if err != nil {
		return nil, err
	}
	
	return resp.Files, nil
}

// getFileDownloadURL 获取文件下载URL - 适配contracts.FileService接口
func (h *FileHandler) getFileDownloadURL(path, fileName string) string {
	// 构建完整路径
	fullPath := path + "/" + fileName
	if path == "/" {
		fullPath = "/" + fileName
	}

	ctx := context.Background()
	fileInfo, err := h.controller.fileService.GetFileInfo(ctx, fullPath)
	if err != nil {
		// 如果获取失败，回退到直接构建URL
		return h.controller.config.Alist.BaseURL + "/d" + fullPath
	}

	return fileInfo.InternalURL
}

// getParentPath 获取父目录路径
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

// DirectoryDownloadStats 目录下载统计信息 - 为保持兼容性保留
type DirectoryDownloadStats struct {
	TotalFiles   int
	VideoFiles   int
	TotalSize    int64
	MovieCount   int
	TVCount      int
	OtherCount   int
	TotalSizeStr string
}

// DirectoryDownloadResult 目录下载结果 - 为保持兼容性保留
type DirectoryDownloadResult struct {
	Stats        DirectoryDownloadStats
	SuccessCount int
	FailedCount  int
	FailedFiles  []string
}