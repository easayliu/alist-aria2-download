package file

import (
	"github.com/easayliu/alist-aria2-download/internal/application/contracts"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/config"
	"github.com/easayliu/alist-aria2-download/internal/interfaces/telegram/types"
)

// Deps 定义 FileHandler 的依赖接口
type Deps interface {
	GetMessageUtils() types.MessageSender
	GetFileService() contracts.FileService
	GetConfig() *config.Config
	EncodeFilePath(path string) string
	DecodeFilePath(encoded string) string

	// 重命名相关（由 controller 实现，调用 BasicCommands）
	HandleRenameCommand(chatID int64, command string)
}
