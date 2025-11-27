package file

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/easayliu/alist-aria2-download/internal/interfaces/telegram/utils"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// ================================
// 文件/目录菜单功能
// ================================

// HandleFileMenu 处理文件操作菜单
func (h *Handler) HandleFileMenu(chatID int64, filePath string) {
	h.HandleFileMenuWithEdit(chatID, filePath, 0)
}

// HandleFileMenuWithEdit 处理文件操作菜单（支持消息编辑）
func (h *Handler) HandleFileMenuWithEdit(chatID int64, filePath string, messageID int) {
	fileName := filepath.Base(filePath)
	fileExt := strings.ToLower(filepath.Ext(fileName))

	msgUtils := h.deps.GetMessageUtils()
	fileService := h.deps.GetFileService()

	var fileIcon string
	if fileService.IsVideoFile(fileName) {
		fileIcon = "🎬"
	} else {
		fileIcon = "📄"
	}

	formatter := msgUtils.GetFormatter().(*utils.MessageFormatter)
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
		EscapeHTML: msgUtils.EscapeHTML,
	}
	message := formatter.FormatFileOperation(opData)

	isVideo := fileService.IsVideoFile(fileName)

	var keyboardRows [][]tgbotapi.InlineKeyboardButton

	keyboardRows = append(keyboardRows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("📥 立即下载", fmt.Sprintf("file_download:%s", h.deps.EncodeFilePath(filePath))),
		tgbotapi.NewInlineKeyboardButtonData("ℹ️ 文件信息", fmt.Sprintf("file_info:%s", h.deps.EncodeFilePath(filePath))),
	))

	keyboardRows = append(keyboardRows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔗 获取链接", fmt.Sprintf("file_link:%s", h.deps.EncodeFilePath(filePath))),
	))

	if isVideo {
		keyboardRows = append(keyboardRows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✏️ 智能重命名", fmt.Sprintf("file_rename:%s", h.deps.EncodeFilePath(filePath))),
		))
	}

	keyboardRows = append(keyboardRows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🗑️ 删除文件", fmt.Sprintf("file_delete_confirm:%s", h.deps.EncodeFilePath(filePath))),
	))

	keyboardRows = append(keyboardRows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("📁 返回目录", fmt.Sprintf("browse_dir:%s:%d", h.deps.EncodeFilePath(h.GetParentPath(filePath)), 1)),
		tgbotapi.NewInlineKeyboardButtonData("🏠 主菜单", "back_main"),
	))

	keyboard := tgbotapi.NewInlineKeyboardMarkup(keyboardRows...)

	if messageID > 0 {
		msgUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
	} else {
		msgUtils.SendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
	}
}

// HandleDirMenu 处理目录操作菜单
func (h *Handler) HandleDirMenu(chatID int64, dirPath string) {
	h.HandleDirMenuWithEdit(chatID, dirPath, 0)
}

// HandleDirMenuWithEdit 处理目录操作菜单（支持消息编辑）
func (h *Handler) HandleDirMenuWithEdit(chatID int64, dirPath string, messageID int) {
	dirName := filepath.Base(dirPath)
	if dirPath == "/" {
		dirName = "根目录"
	}

	msgUtils := h.deps.GetMessageUtils()
	formatter := msgUtils.GetFormatter().(*utils.MessageFormatter)

	opData := utils.FileOperationData{
		Icon:       "📁",
		FileName:   dirName,
		FilePath:   filepath.Dir(dirPath),
		FileType:   "目录",
		Prompt:     "请选择操作：",
		EscapeHTML: msgUtils.EscapeHTML,
	}
	message := formatter.FormatFileOperation(opData)

	var keyboardRows [][]tgbotapi.InlineKeyboardButton

	keyboardRows = append(keyboardRows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("📂 进入目录", fmt.Sprintf("browse_dir:%s:%d", h.deps.EncodeFilePath(dirPath), 1)),
		tgbotapi.NewInlineKeyboardButtonData("📥 下载目录", fmt.Sprintf("download_dir:%s", h.deps.EncodeFilePath(dirPath))),
	))

	keyboardRows = append(keyboardRows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("📝 批量重命名", fmt.Sprintf("batch_rename:%s", h.deps.EncodeFilePath(dirPath))),
	))

	if dirPath != "/" {
		keyboardRows = append(keyboardRows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🗑️ 删除目录", fmt.Sprintf("dir_delete_confirm:%s", h.deps.EncodeFilePath(dirPath))),
		))
	}

	keyboardRows = append(keyboardRows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("📁 返回上级", fmt.Sprintf("browse_dir:%s:%d", h.deps.EncodeFilePath(h.GetParentPath(dirPath)), 1)),
		tgbotapi.NewInlineKeyboardButtonData("🏠 主菜单", "back_main"),
	))

	keyboard := tgbotapi.NewInlineKeyboardMarkup(keyboardRows...)

	if messageID > 0 {
		msgUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
	} else {
		msgUtils.SendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
	}
}

// HandleFileInfo 处理文件信息查看
func (h *Handler) HandleFileInfo(chatID int64, filePath string) {
	h.HandleFileInfoWithEdit(chatID, filePath, 0)
}

// HandleFileInfoWithEdit 处理文件信息查看（支持消息编辑）
func (h *Handler) HandleFileInfoWithEdit(chatID int64, filePath string, messageID int) {
	msgUtils := h.deps.GetMessageUtils()
	fileService := h.deps.GetFileService()

	// 仅在发送新消息时显示加载提示
	if messageID == 0 {
		msgUtils.SendMessage(chatID, "正在获取文件信息...")
	}

	// 获取文件信息
	fileInfo, err := h.ListFilesSimple(filepath.Dir(filePath), 1, 1000)
	if err != nil {
		message := "获取文件信息失败: " + err.Error()
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("返回", fmt.Sprintf("browse_dir:%s:%d", h.deps.EncodeFilePath(filepath.Dir(filePath)), 1)),
			),
		)
		if messageID > 0 {
			msgUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
		} else {
			msgUtils.SendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
		}
		return
	}

	// 查找对应文件
	var targetFile *struct {
		Name     string
		Size     int64
		IsDir    bool
		Modified string
	}
	fileName := filepath.Base(filePath)
	for _, file := range fileInfo {
		if file.Name == fileName {
			targetFile = &struct {
				Name     string
				Size     int64
				IsDir    bool
				Modified string
			}{
				Name:     file.Name,
				Size:     file.Size,
				IsDir:    file.IsDir,
				Modified: file.Modified.Format("2006-01-02 15:04:05"),
			}
			break
		}
	}

	if targetFile == nil {
		message := "文件未找到"
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("返回", fmt.Sprintf("browse_dir:%s:%d", h.deps.EncodeFilePath(filepath.Dir(filePath)), 1)),
			),
		)
		if messageID > 0 {
			msgUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
		} else {
			msgUtils.SendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
		}
		return
	}

	// 确定文件类型
	fileType := "其他文件"
	if fileService.IsVideoFile(targetFile.Name) {
		fileType = "视频文件"
	}

	// 使用统一格式化器
	formatter := msgUtils.GetFormatter().(*utils.MessageFormatter)
	infoData := utils.FileInfoData{
		Icon:       "ℹ️",
		Name:       targetFile.Name,
		Path:       filePath,
		Type:       fileType,
		Size:       msgUtils.FormatFileSize(targetFile.Size),
		Modified:   targetFile.Modified,
		IsDir:      targetFile.IsDir,
		EscapeHTML: msgUtils.EscapeHTML,
	}

	message := formatter.FormatFileInfo(infoData)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("返回", fmt.Sprintf("browse_dir:%s:%d", h.deps.EncodeFilePath(filepath.Dir(filePath)), 1)),
		),
	)

	if messageID > 0 {
		msgUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
	} else {
		msgUtils.SendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
	}
}

// HandleFileLink 处理获取文件链接
func (h *Handler) HandleFileLink(chatID int64, filePath string) {
	h.HandleFileLinkWithEdit(chatID, filePath, 0)
}

// HandleFileLinkWithEdit 处理获取文件链接（支持消息编辑）
func (h *Handler) HandleFileLinkWithEdit(chatID int64, filePath string, messageID int) {
	msgUtils := h.deps.GetMessageUtils()

	// 仅在发送新消息时显示加载提示
	if messageID == 0 {
		msgUtils.SendMessage(chatID, "正在获取文件链接...")
	}

	// 获取文件下载链接
	downloadURL := h.GetFileDownloadURL(filepath.Dir(filePath), filepath.Base(filePath))

	// 使用统一格式化器
	formatter := msgUtils.GetFormatter().(*utils.MessageFormatter)
	var lines []string

	lines = append(lines, formatter.FormatTitle("🔗", "文件链接"))
	lines = append(lines, "")
	lines = append(lines, formatter.FormatFieldCode("文件", msgUtils.EscapeHTML(filepath.Base(filePath))))
	lines = append(lines, "")
	lines = append(lines, formatter.FormatField("下载链接", ""))
	lines = append(lines, fmt.Sprintf("<code>%s</code>", msgUtils.EscapeHTML(downloadURL)))

	message := strings.Join(lines, "\n")

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("返回", fmt.Sprintf("browse_dir:%s:%d", h.deps.EncodeFilePath(filepath.Dir(filePath)), 1)),
		),
	)

	if messageID > 0 {
		msgUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
	} else {
		msgUtils.SendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
	}
}
