package file

import "fmt"

// ================================
// 文件重命名功能
// ================================

// HandleFileRename 处理单文件重命名
func (h *Handler) HandleFileRename(chatID int64, filePath string) {
	h.deps.HandleRenameCommand(chatID, fmt.Sprintf("/rename %s", filePath))
}
