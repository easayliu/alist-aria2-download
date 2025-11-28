package telegram

import (
	"github.com/easayliu/alist-aria2-download/internal/application/services"
	taskhandler "github.com/easayliu/alist-aria2-download/internal/interfaces/telegram/handlers/task"
	"github.com/easayliu/alist-aria2-download/internal/interfaces/telegram/types"
)

// TaskHandler handles task management related functions (adapter)
type TaskHandler struct {
	controller *TelegramController
	handler    *taskhandler.Handler
}

// NewTaskHandler creates a new task handler
func NewTaskHandler(controller *TelegramController) *TaskHandler {
	th := &TaskHandler{
		controller: controller,
	}
	th.handler = taskhandler.NewHandler(th)
	return th
}

// ================================
// 实现 taskhandler.TaskDeps 接口
// ================================

func (h *TaskHandler) GetMessageUtils() types.MessageSender {
	return h.controller.messageUtils
}

func (h *TaskHandler) GetSchedulerService() *services.SchedulerService {
	return h.controller.schedulerService
}

// ================================
// 代理方法
// ================================

func (h *TaskHandler) HandleTasksWithEdit(chatID int64, userID int64, messageID int) {
	h.handler.HandleTasksWithEdit(chatID, userID, messageID)
}
