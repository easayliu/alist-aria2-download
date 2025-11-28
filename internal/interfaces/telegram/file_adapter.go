package telegram

import (
	"github.com/easayliu/alist-aria2-download/internal/application/contracts"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/config"
	filehandler "github.com/easayliu/alist-aria2-download/internal/interfaces/telegram/handlers/file"
	"github.com/easayliu/alist-aria2-download/internal/interfaces/telegram/types"
)

// FileHandler 文件浏览处理器（适配器模式）
// 代理调用 handlers/file.Handler，同时保留对 controller 的引用以支持批量重命名等功能
type FileHandler struct {
	controller *TelegramController
	handler    *filehandler.Handler
}

// NewFileHandler 创建文件处理器
func NewFileHandler(controller *TelegramController) *FileHandler {
	fh := &FileHandler{
		controller: controller,
	}
	// 创建新的 handler，传入适配器作为依赖
	fh.handler = filehandler.NewHandler(fh)
	return fh
}

// ================================
// 实现 filehandler.FileDeps 接口
// ================================

func (h *FileHandler) GetMessageUtils() types.MessageSender {
	return h.controller.messageUtils
}

func (h *FileHandler) GetFileService() contracts.FileService {
	return h.controller.fileService
}

func (h *FileHandler) GetConfig() *config.Config {
	return h.controller.config
}

func (h *FileHandler) EncodeFilePath(path string) string {
	return h.controller.common.EncodeFilePath(path)
}

func (h *FileHandler) DecodeFilePath(encoded string) string {
	return h.controller.common.DecodeFilePath(encoded)
}

func (h *FileHandler) HandleRenameCommand(chatID int64, command string) {
	h.controller.basicCommands.HandleRename(chatID, command)
}

// ================================
// 代理方法 - 文件浏览
// ================================

func (h *FileHandler) HandleBrowseFiles(chatID int64, path string, page int) {
	h.handler.HandleBrowseFiles(chatID, path, page)
}

func (h *FileHandler) HandleBrowseFilesWithEdit(chatID int64, path string, page int, messageID int) {
	h.handler.HandleBrowseFilesWithEdit(chatID, path, page, messageID)
}

func (h *FileHandler) HandleFilesBrowseWithEdit(chatID int64, messageID int) {
	h.handler.HandleFilesBrowseWithEdit(chatID, messageID)
}

func (h *FileHandler) HandleAlistFilesWithEdit(chatID int64, messageID int) {
	h.handler.HandleAlistFilesWithEdit(chatID, messageID)
}

// ================================
// 代理方法 - 文件菜单
// ================================

func (h *FileHandler) HandleFileMenu(chatID int64, filePath string) {
	h.handler.HandleFileMenu(chatID, filePath)
}

func (h *FileHandler) HandleFileMenuWithEdit(chatID int64, filePath string, messageID int) {
	h.handler.HandleFileMenuWithEdit(chatID, filePath, messageID)
}

func (h *FileHandler) HandleDirMenu(chatID int64, dirPath string) {
	h.handler.HandleDirMenu(chatID, dirPath)
}

func (h *FileHandler) HandleDirMenuWithEdit(chatID int64, dirPath string, messageID int) {
	h.handler.HandleDirMenuWithEdit(chatID, dirPath, messageID)
}

func (h *FileHandler) HandleFileInfo(chatID int64, filePath string) {
	h.handler.HandleFileInfo(chatID, filePath)
}

func (h *FileHandler) HandleFileInfoWithEdit(chatID int64, filePath string, messageID int) {
	h.handler.HandleFileInfoWithEdit(chatID, filePath, messageID)
}

func (h *FileHandler) HandleFileLink(chatID int64, filePath string) {
	h.handler.HandleFileLink(chatID, filePath)
}

func (h *FileHandler) HandleFileLinkWithEdit(chatID int64, filePath string, messageID int) {
	h.handler.HandleFileLinkWithEdit(chatID, filePath, messageID)
}

// ================================
// 代理方法 - 文件删除
// ================================

func (h *FileHandler) HandleFileDeleteConfirm(chatID int64, filePath string, messageID int) {
	h.handler.HandleFileDeleteConfirm(chatID, filePath, messageID)
}

func (h *FileHandler) HandleFileDelete(chatID int64, filePath string, messageID int) {
	h.handler.HandleFileDelete(chatID, filePath, messageID)
}

func (h *FileHandler) HandleDirDeleteConfirm(chatID int64, dirPath string, messageID int) {
	h.handler.HandleDirDeleteConfirm(chatID, dirPath, messageID)
}

func (h *FileHandler) HandleDirDelete(chatID int64, dirPath string, messageID int) {
	h.handler.HandleDirDelete(chatID, dirPath, messageID)
}

// ================================
// 代理方法 - 文件下载
// ================================

func (h *FileHandler) HandleFileDownload(chatID int64, filePath string) {
	h.handler.HandleFileDownload(chatID, filePath)
}

func (h *FileHandler) HandleDownloadDirectory(chatID int64, dirPath string) {
	h.handler.HandleDownloadDirectory(chatID, dirPath)
}

func (h *FileHandler) HandleDownloadDirectoryConfirm(chatID int64, dirPath string, messageID int) {
	h.handler.HandleDownloadDirectoryConfirm(chatID, dirPath, messageID)
}

func (h *FileHandler) HandleDownloadDirectoryExecute(chatID int64, dirPath string, messageID int) {
	h.handler.HandleDownloadDirectoryExecute(chatID, dirPath, messageID)
}

// ================================
// 代理方法 - 文件重命名（单文件）
// ================================

func (h *FileHandler) HandleFileRename(chatID int64, filePath string) {
	h.handler.HandleFileRename(chatID, filePath)
}

func (h *FileHandler) HandleRenameApply(chatID int64, callbackData string, messageID int) {
	h.handler.HandleRenameApply(chatID, callbackData, messageID)
}

// ================================
// 代理方法 - 批量重命名
// ================================

func (h *FileHandler) HandleBatchRename(chatID int64, dirPath string) {
	h.handler.HandleBatchRename(chatID, dirPath)
}

func (h *FileHandler) HandleBatchRenameWithEdit(chatID int64, dirPath string, messageID int) {
	h.handler.HandleBatchRenameWithEdit(chatID, dirPath, messageID)
}

func (h *FileHandler) HandleBatchRenameConfirm(chatID int64, dirPath string, messageID int) {
	h.handler.HandleBatchRenameConfirm(chatID, dirPath, messageID)
}

// ================================
// 兼容类型定义（保留）
// ================================

// DirectoryDownloadStats 目录下载统计
type DirectoryDownloadStats struct {
	TotalFiles   int
	VideoFiles   int
	TotalSize    int64
	MovieCount   int
	TVCount      int
	OtherCount   int
	TotalSizeStr string
}

// DirectoryDownloadResult 目录下载结果
type DirectoryDownloadResult struct {
	Stats        DirectoryDownloadStats
	SuccessCount int
	FailedCount  int
	FailedFiles  []string
}
