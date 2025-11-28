package task

import (
	"github.com/easayliu/alist-aria2-download/internal/application/services"
	"github.com/easayliu/alist-aria2-download/internal/interfaces/telegram/types"
)

// TaskDeps 定义 TaskHandler 的依赖接口
type TaskDeps interface {
	GetMessageUtils() types.MessageSender
	GetSchedulerService() *services.SchedulerService
}
