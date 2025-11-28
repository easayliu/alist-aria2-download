package download

import (
	"github.com/easayliu/alist-aria2-download/internal/application/contracts"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/config"
	"github.com/easayliu/alist-aria2-download/internal/interfaces/telegram/types"
)

// DownloadDeps 定义 DownloadHandler 的依赖接口
type DownloadDeps interface {
	GetMessageUtils() types.MessageSender
	GetFileService() contracts.FileService
	GetDownloadService() contracts.DownloadService
	GetConfig() *config.Config
}
