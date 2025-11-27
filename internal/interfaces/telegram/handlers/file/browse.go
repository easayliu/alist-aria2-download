package file

import (
	"fmt"

	"github.com/easayliu/alist-aria2-download/internal/interfaces/telegram/utils"
	"github.com/easayliu/alist-aria2-download/pkg/logger"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// ================================
// 文件浏览功能
// ================================

// HandleBrowseFiles 处理文件浏览（支持分页和交互）
func (h *Handler) HandleBrowseFiles(chatID int64, path string, page int) {
	h.HandleBrowseFilesWithEdit(chatID, path, page, 0) // 0 表示发送新消息
}

// HandleBrowseFilesWithEdit 处理文件浏览（支持消息编辑和分页）
func (h *Handler) HandleBrowseFilesWithEdit(chatID int64, path string, page int, messageID int) {
	if path == "" {
		path = "/"
	}
	if page < 1 {
		page = 1
	}

	logger.Info("Browsing files", "path", path, "page", page, "messageID", messageID)

	msgUtils := h.deps.GetMessageUtils()

	// 仅在发送新消息时显示提示
	if messageID == 0 {
		msgUtils.SendMessage(chatID, "正在获取文件列表...")
	}

	// 获取文件列表（每页显示8个文件，为按钮布局预留空间）
	files, err := h.ListFilesSimple(path, page, 8)
	if err != nil {
		formatter := msgUtils.GetFormatter().(*utils.MessageFormatter)
		msgUtils.SendMessage(chatID, formatter.FormatError("获取文件列表", err))
		return
	}

	if len(files) == 0 {
		msgUtils.SendMessageHTMLWithAutoDelete(chatID, "当前目录为空", 30)
		return
	}

	// 统计文件信息
	dirCount := 0
	fileCount := 0
	videoCount := 0
	fileService := h.deps.GetFileService()
	for _, file := range files {
		if file.IsDir {
			dirCount++
		} else {
			fileCount++
			if fileService.IsVideoFile(file.Name) {
				videoCount++
			}
		}
	}

	// 使用统一格式化器
	formatter := msgUtils.GetFormatter().(*utils.MessageFormatter)
	browserData := utils.FileBrowserData{
		Path:       path,
		Page:       page,
		TotalPages: 1,
		TotalFiles: len(files),
		DirCount:   dirCount,
		FileCount:  fileCount,
		VideoCount: videoCount,
		EscapeHTML: msgUtils.EscapeHTML,
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
			fullPath := h.BuildFullPath(file, path)
			callbackData = fmt.Sprintf("browse_dir:%s:1", h.deps.EncodeFilePath(fullPath))
		} else if fileService.IsVideoFile(file.Name) {
			prefix = "🎬"
			fullPath := h.BuildFullPath(file, path)
			callbackData = fmt.Sprintf("file_menu:%s", h.deps.EncodeFilePath(fullPath))
		} else {
			prefix = "📄"
			fullPath := h.BuildFullPath(file, path)
			callbackData = fmt.Sprintf("file_menu:%s", h.deps.EncodeFilePath(fullPath))
		}

		fileName := file.Name
		maxWidth := 38
		if !file.IsDir {
			maxWidth = 30
		}

		btnFormatter := msgUtils.GetFormatter().(*utils.MessageFormatter)
		fileName = btnFormatter.TruncateButtonText(fileName, maxWidth)

		button := tgbotapi.NewInlineKeyboardButtonData(
			fmt.Sprintf("%s %s", prefix, fileName),
			callbackData,
		)

		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{button})
	}

	// 添加导航按钮
	navButtons := []tgbotapi.InlineKeyboardButton{}

	// 上一页按钮
	if page > 1 {
		navButtons = append(navButtons, tgbotapi.NewInlineKeyboardButtonData(
			"< 上一页",
			fmt.Sprintf("browse_page:%s:%d", h.deps.EncodeFilePath(path), page-1),
		))
	}

	// 下一页按钮（如果当前页已满，可能还有更多）
	if len(files) == 8 {
		navButtons = append(navButtons, tgbotapi.NewInlineKeyboardButtonData(
			"下一页 >",
			fmt.Sprintf("browse_page:%s:%d", h.deps.EncodeFilePath(path), page+1),
		))
	}

	if len(navButtons) > 0 {
		keyboard = append(keyboard, navButtons)
	}

	// 添加操作按钮 - 第一行：下载和刷新
	actionRow1 := []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("📥 下载目录", fmt.Sprintf("download_dir:%s", h.deps.EncodeFilePath(path))),
		tgbotapi.NewInlineKeyboardButtonData("📝 批量重命名", fmt.Sprintf("batch_rename:%s", h.deps.EncodeFilePath(path))),
		tgbotapi.NewInlineKeyboardButtonData("🔄 刷新", fmt.Sprintf("browse_refresh:%s:%d", h.deps.EncodeFilePath(path), page)),
	}
	keyboard = append(keyboard, actionRow1)

	// 添加导航按钮 - 第二行：上级目录、删除目录和主菜单
	actionRow2 := []tgbotapi.InlineKeyboardButton{}

	// 返回上级目录按钮
	if path != "/" {
		parentPath := h.GetParentPath(path)
		actionRow2 = append(actionRow2, tgbotapi.NewInlineKeyboardButtonData(
			"⬆️ 上级目录",
			fmt.Sprintf("browse_dir:%s:%d", h.deps.EncodeFilePath(parentPath), 1),
		))
	}

	// 删除目录按钮（仅非根目录）
	if path != "/" {
		actionRow2 = append(actionRow2, tgbotapi.NewInlineKeyboardButtonData(
			"🗑️ 删除目录",
			fmt.Sprintf("dir_delete_confirm:%s", h.deps.EncodeFilePath(path)),
		))
	}

	// 返回主菜单按钮
	actionRow2 = append(actionRow2, tgbotapi.NewInlineKeyboardButtonData("🏠 主菜单", "back_main"))

	if len(actionRow2) > 0 {
		keyboard = append(keyboard, actionRow2)
	}

	inlineKeyboard := tgbotapi.NewInlineKeyboardMarkup(keyboard...)

	if messageID > 0 {
		msgUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &inlineKeyboard)
	} else {
		msgUtils.SendMessageWithKeyboard(chatID, message, "HTML", &inlineKeyboard)
	}
}

// HandleFilesBrowseWithEdit 处理文件浏览（支持消息编辑）
func (h *Handler) HandleFilesBrowseWithEdit(chatID int64, messageID int) {
	defaultPath := h.deps.GetConfig().Alist.DefaultPath
	if defaultPath == "" {
		defaultPath = "/"
	}
	h.HandleBrowseFilesWithEdit(chatID, defaultPath, 1, messageID)
}

// HandleAlistFilesWithEdit 处理获取 Alist 文件列表（支持消息编辑）
func (h *Handler) HandleAlistFilesWithEdit(chatID int64, messageID int) {
	h.HandleBrowseFilesWithEdit(chatID, h.deps.GetConfig().Alist.DefaultPath, 1, messageID)
}
