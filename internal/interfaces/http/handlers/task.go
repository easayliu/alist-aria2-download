package handlers

import (
	"net/http"
	"strconv"

	"github.com/easayliu/alist-aria2-download/internal/application/contracts"
	"github.com/easayliu/alist-aria2-download/internal/application/services"
	httputil "github.com/easayliu/alist-aria2-download/pkg/utils/http"
	"github.com/gin-gonic/gin"
)

// TaskHandler REST任务处理器 - 纯协议转换层
type TaskHandler struct {
	container *services.ServiceContainer
}

// NewTaskHandler 创建任务处理器
func NewTaskHandler(container *services.ServiceContainer) *TaskHandler {
	return &TaskHandler{
		container: container,
	}
}

// CreateTask 创建定时任务
// @Summary 创建定时任务
// @Description 创建一个新的定时任务，按照cron表达式定期执行
// @Tags 定时任务
// @Accept json
// @Produce json
// @Param request body contracts.TaskRequest true "创建任务请求"
// @Success 200 {object} contracts.TaskResponse "任务创建成功"
// @Failure 400 {object} map[string]interface{} "请求参数错误"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /tasks [post]
func (h *TaskHandler) CreateTask(c *gin.Context) {
	// 1. 解析HTTP请求 - 协议转换
	var req contracts.TaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.ErrorWithStatus(c, http.StatusBadRequest, 400, "Invalid request parameters: "+err.Error())
		return
	}

	// 2. 调用应用服务 - 业务逻辑委托
	taskService := h.container.GetTaskService()
	response, err := taskService.CreateTask(c.Request.Context(), req)
	if err != nil {
		// 错误映射
		if serviceErr, ok := err.(*contracts.ServiceError); ok {
			statusCode := h.mapErrorCodeToHTTPStatus(serviceErr.Code)
			httputil.ErrorWithStatus(c, statusCode, statusCode, serviceErr.Message)
		} else {
			httputil.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to create task: "+err.Error())
		}
		return
	}

	// 3. 返回HTTP响应 - 协议转换
	httputil.Success(c, gin.H{
		"message": "Task created successfully",
		"task":    response,
	})
}

// GetTask 获取单个定时任务
// @Summary 获取定时任务详情
// @Description 根据任务ID获取定时任务的详细信息
// @Tags 定时任务
// @Accept json
// @Produce json
// @Param id path string true "任务ID"
// @Success 200 {object} contracts.TaskResponse "任务详情"
// @Failure 404 {object} map[string]interface{} "任务不存在"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /tasks/{id} [get]
func (h *TaskHandler) GetTask(c *gin.Context) {
	// 1. 提取路径参数
	taskID := c.Param("id")
	if taskID == "" {
		httputil.ErrorWithStatus(c, http.StatusBadRequest, 400, "Task ID is required")
		return
	}

	// 2. 调用应用服务
	taskService := h.container.GetTaskService()
	response, err := taskService.GetTask(c.Request.Context(), taskID)
	if err != nil {
		if serviceErr, ok := err.(*contracts.ServiceError); ok {
			statusCode := h.mapErrorCodeToHTTPStatus(serviceErr.Code)
			httputil.ErrorWithStatus(c, statusCode, statusCode, serviceErr.Message)
		} else {
			httputil.ErrorWithStatus(c, http.StatusNotFound, 404, "Task not found")
		}
		return
	}

	// 3. 返回响应
	httputil.Success(c, response)
}

// ListTasks 获取任务列表
// @Summary 获取定时任务列表
// @Description 获取所有定时任务的列表
// @Tags 定时任务
// @Accept json
// @Produce json
// @Param created_by query int false "创建者ID"
// @Param enabled query bool false "是否启用"
// @Param status query string false "任务状态"
// @Param limit query int false "限制数量" default(100)
// @Param offset query int false "偏移量" default(0)
// @Param sort_by query string false "排序字段"
// @Param sort_order query string false "排序方向"
// @Success 200 {object} contracts.TaskListResponse "任务列表"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /tasks [get]
func (h *TaskHandler) ListTasks(c *gin.Context) {
	// 1. 解析查询参数 - 协议转换
	req := contracts.TaskListRequest{
		Status:    c.Query("status"),
		SortBy:    c.Query("sort_by"),
		SortOrder: c.Query("sort_order"),
	}

	// 解析数值参数
	if createdByStr := c.Query("created_by"); createdByStr != "" {
		if createdBy, err := strconv.ParseInt(createdByStr, 10, 64); err == nil {
			req.CreatedBy = createdBy
		}
	}

	if enabledStr := c.Query("enabled"); enabledStr != "" {
		if enabled, err := strconv.ParseBool(enabledStr); err == nil {
			req.Enabled = &enabled
		}
	}

	if limitStr := c.Query("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil {
			req.Limit = limit
		} else {
			req.Limit = 100
		}
	} else {
		req.Limit = 100
	}

	if offsetStr := c.Query("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil {
			req.Offset = offset
		}
	}

	// 2. 调用应用服务
	taskService := h.container.GetTaskService()
	response, err := taskService.ListTasks(c.Request.Context(), req)
	if err != nil {
		if serviceErr, ok := err.(*contracts.ServiceError); ok {
			statusCode := h.mapErrorCodeToHTTPStatus(serviceErr.Code)
			httputil.ErrorWithStatus(c, statusCode, statusCode, serviceErr.Message)
		} else {
			httputil.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to get tasks: "+err.Error())
		}
		return
	}

	// 3. 返回响应
	httputil.Success(c, response)
}

// UpdateTask 更新定时任务
// @Summary 更新定时任务
// @Description 更新指定ID的定时任务信息
// @Tags 定时任务
// @Accept json
// @Produce json
// @Param id path string true "任务ID"
// @Param request body contracts.TaskUpdateRequest true "更新任务请求"
// @Success 200 {object} contracts.TaskResponse "任务更新成功"
// @Failure 400 {object} map[string]interface{} "请求参数错误"
// @Failure 404 {object} map[string]interface{} "任务不存在"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /tasks/{id} [put]
func (h *TaskHandler) UpdateTask(c *gin.Context) {
	// 1. 提取路径参数
	taskID := c.Param("id")
	if taskID == "" {
		httputil.ErrorWithStatus(c, http.StatusBadRequest, 400, "Task ID is required")
		return
	}

	// 2. 解析请求体
	var req contracts.TaskUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.ErrorWithStatus(c, http.StatusBadRequest, 400, "Invalid request parameters: "+err.Error())
		return
	}

	// 3. 调用应用服务
	taskService := h.container.GetTaskService()
	response, err := taskService.UpdateTask(c.Request.Context(), taskID, req)
	if err != nil {
		if serviceErr, ok := err.(*contracts.ServiceError); ok {
			statusCode := h.mapErrorCodeToHTTPStatus(serviceErr.Code)
			httputil.ErrorWithStatus(c, statusCode, statusCode, serviceErr.Message)
		} else {
			httputil.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to update task: "+err.Error())
		}
		return
	}

	// 4. 返回响应
	httputil.Success(c, gin.H{
		"message": "Task updated successfully",
		"task":    response,
	})
}

// DeleteTask 删除定时任务
// @Summary 删除定时任务
// @Description 删除指定ID的定时任务
// @Tags 定时任务
// @Accept json
// @Produce json
// @Param id path string true "任务ID"
// @Success 200 {object} map[string]interface{} "任务删除成功"
// @Failure 404 {object} map[string]interface{} "任务不存在"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /tasks/{id} [delete]
func (h *TaskHandler) DeleteTask(c *gin.Context) {
	// 1. 提取路径参数
	taskID := c.Param("id")
	if taskID == "" {
		httputil.ErrorWithStatus(c, http.StatusBadRequest, 400, "Task ID is required")
		return
	}

	// 2. 调用应用服务
	taskService := h.container.GetTaskService()
	err := taskService.DeleteTask(c.Request.Context(), taskID)
	if err != nil {
		if serviceErr, ok := err.(*contracts.ServiceError); ok {
			statusCode := h.mapErrorCodeToHTTPStatus(serviceErr.Code)
			httputil.ErrorWithStatus(c, statusCode, statusCode, serviceErr.Message)
		} else {
			httputil.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to delete task: "+err.Error())
		}
		return
	}

	// 3. 返回响应
	httputil.Success(c, gin.H{
		"message": "Task deleted successfully",
		"task_id": taskID,
	})
}

// RunTaskNow 立即执行任务
// @Summary 立即执行定时任务
// @Description 立即执行指定ID的定时任务，不等待下一个调度时间
// @Tags 定时任务
// @Accept json
// @Produce json
// @Param id path string true "任务ID"
// @Param request body contracts.TaskRunRequest false "执行选项"
// @Success 200 {object} contracts.TaskRunResponse "任务已开始执行或预览结果"
// @Failure 404 {object} map[string]interface{} "任务不存在"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /tasks/{id}/run [post]
func (h *TaskHandler) RunTaskNow(c *gin.Context) {
	// 1. 提取路径参数
	taskID := c.Param("id")
	if taskID == "" {
		httputil.ErrorWithStatus(c, http.StatusBadRequest, 400, "Task ID is required")
		return
	}

	// 2. 解析请求体（可选）
	var req contracts.TaskRunRequest
	req.TaskID = taskID // 确保设置任务ID
	if err := c.ShouldBindJSON(&req); err != nil {
		// 如果解析失败，使用默认值
		req = contracts.TaskRunRequest{
			TaskID:   taskID,
			Preview:  false,
			ForceRun: false,
		}
	}

	// 3. 调用应用服务
	taskService := h.container.GetTaskService()
	response, err := taskService.RunTaskNow(c.Request.Context(), req)
	if err != nil {
		if serviceErr, ok := err.(*contracts.ServiceError); ok {
			statusCode := h.mapErrorCodeToHTTPStatus(serviceErr.Code)
			httputil.ErrorWithStatus(c, statusCode, statusCode, serviceErr.Message)
		} else {
			httputil.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to run task: "+err.Error())
		}
		return
	}

	// 4. 返回响应
	httputil.Success(c, gin.H{
		"message": "Task execution started",
		"result":  response,
	})
}

// PreviewTask 预览定时任务将要下载的文件
// @Summary 预览定时任务
// @Description 预览定时任务将要下载的文件，不实际执行下载
// @Tags 定时任务
// @Accept json
// @Produce json
// @Param id path string true "任务ID"
// @Success 200 {object} contracts.TaskPreviewResponse "预览结果"
// @Failure 404 {object} map[string]interface{} "任务不存在"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /tasks/{id}/preview [get]
func (h *TaskHandler) PreviewTask(c *gin.Context) {
	// 1. 提取路径参数
	taskID := c.Param("id")
	if taskID == "" {
		httputil.ErrorWithStatus(c, http.StatusBadRequest, 400, "Task ID is required")
		return
	}

	// 2. 构建预览请求
	req := contracts.TaskPreviewRequest{
		TaskID: taskID,
	}

	// 可选的时间范围参数
	if startTime := c.Query("start_time"); startTime != "" {
		// 这里可以解析时间参数
	}
	if endTime := c.Query("end_time"); endTime != "" {
		// 这里可以解析时间参数
	}

	// 3. 调用应用服务
	taskService := h.container.GetTaskService()
	response, err := taskService.PreviewTask(c.Request.Context(), req)
	if err != nil {
		if serviceErr, ok := err.(*contracts.ServiceError); ok {
			statusCode := h.mapErrorCodeToHTTPStatus(serviceErr.Code)
			httputil.ErrorWithStatus(c, statusCode, statusCode, serviceErr.Message)
		} else {
			httputil.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to preview task: "+err.Error())
		}
		return
	}

	// 4. 返回响应
	httputil.Success(c, response)
}

// EnableTask 启用任务
// @Summary 启用定时任务
// @Description 启用指定ID的定时任务
// @Tags 定时任务
// @Accept json
// @Produce json
// @Param id path string true "任务ID"
// @Success 200 {object} map[string]interface{} "任务启用成功"
// @Failure 404 {object} map[string]interface{} "任务不存在"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /tasks/{id}/enable [post]
func (h *TaskHandler) EnableTask(c *gin.Context) {
	// 1. 提取路径参数
	taskID := c.Param("id")
	if taskID == "" {
		httputil.ErrorWithStatus(c, http.StatusBadRequest, 400, "Task ID is required")
		return
	}

	// 2. 调用应用服务
	taskService := h.container.GetTaskService()
	err := taskService.EnableTask(c.Request.Context(), taskID)
	if err != nil {
		if serviceErr, ok := err.(*contracts.ServiceError); ok {
			statusCode := h.mapErrorCodeToHTTPStatus(serviceErr.Code)
			httputil.ErrorWithStatus(c, statusCode, statusCode, serviceErr.Message)
		} else {
			httputil.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to enable task: "+err.Error())
		}
		return
	}

	// 3. 返回响应
	httputil.Success(c, gin.H{
		"message": "Task enabled successfully",
		"task_id": taskID,
	})
}

// DisableTask 禁用任务
// @Summary 禁用定时任务
// @Description 禁用指定ID的定时任务
// @Tags 定时任务
// @Accept json
// @Produce json
// @Param id path string true "任务ID"
// @Success 200 {object} map[string]interface{} "任务禁用成功"
// @Failure 404 {object} map[string]interface{} "任务不存在"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /tasks/{id}/disable [post]
func (h *TaskHandler) DisableTask(c *gin.Context) {
	// 1. 提取路径参数
	taskID := c.Param("id")
	if taskID == "" {
		httputil.ErrorWithStatus(c, http.StatusBadRequest, 400, "Task ID is required")
		return
	}

	// 2. 调用应用服务
	taskService := h.container.GetTaskService()
	err := taskService.DisableTask(c.Request.Context(), taskID)
	if err != nil {
		if serviceErr, ok := err.(*contracts.ServiceError); ok {
			statusCode := h.mapErrorCodeToHTTPStatus(serviceErr.Code)
			httputil.ErrorWithStatus(c, statusCode, statusCode, serviceErr.Message)
		} else {
			httputil.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to disable task: "+err.Error())
		}
		return
	}

	// 3. 返回响应
	httputil.Success(c, gin.H{
		"message": "Task disabled successfully",
		"task_id": taskID,
	})
}

// CreateQuickTask 创建快捷任务
// @Summary 创建快捷任务
// @Description 创建预定义的快捷任务（每日、最近等）
// @Tags 定时任务
// @Accept json
// @Produce json
// @Param request body contracts.QuickTaskRequest true "快捷任务请求"
// @Success 200 {object} contracts.TaskResponse "任务创建成功"
// @Failure 400 {object} map[string]interface{} "请求参数错误"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /tasks/quick [post]
func (h *TaskHandler) CreateQuickTask(c *gin.Context) {
	// 1. 解析请求
	var req contracts.QuickTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.ErrorWithStatus(c, http.StatusBadRequest, 400, "Invalid request parameters: "+err.Error())
		return
	}

	// 2. 调用应用服务
	taskService := h.container.GetTaskService()
	response, err := taskService.CreateQuickTask(c.Request.Context(), req)
	if err != nil {
		if serviceErr, ok := err.(*contracts.ServiceError); ok {
			statusCode := h.mapErrorCodeToHTTPStatus(serviceErr.Code)
			httputil.ErrorWithStatus(c, statusCode, statusCode, serviceErr.Message)
		} else {
			httputil.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to create quick task: "+err.Error())
		}
		return
	}

	// 3. 返回响应
	httputil.Success(c, gin.H{
		"message": "Quick task created successfully",
		"task":    response,
	})
}

// GetTaskStatistics 获取任务统计
// @Summary 获取任务统计
// @Description 获取任务系统的统计信息
// @Tags 定时任务
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /tasks/statistics [get]
func (h *TaskHandler) GetTaskStatistics(c *gin.Context) {
	// 1. 调用应用服务
	taskService := h.container.GetTaskService()
	stats, err := taskService.GetTaskStatistics(c.Request.Context())
	if err != nil {
		httputil.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to get task statistics: "+err.Error())
		return
	}

	// 2. 返回响应
	httputil.Success(c, stats)
}

// GetSchedulerStatus 获取调度器状态
// @Summary 获取调度器状态
// @Description 获取任务调度器的状态信息
// @Tags 定时任务
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /tasks/scheduler/status [get]
func (h *TaskHandler) GetSchedulerStatus(c *gin.Context) {
	// 1. 调用应用服务
	taskService := h.container.GetTaskService()
	status, err := taskService.GetSchedulerStatus(c.Request.Context())
	if err != nil {
		httputil.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to get scheduler status: "+err.Error())
		return
	}

	// 2. 返回响应
	httputil.Success(c, status)
}

// ========== 私有方法 ==========

// mapErrorCodeToHTTPStatus 将业务错误码映射到HTTP状态码
func (h *TaskHandler) mapErrorCodeToHTTPStatus(code contracts.ErrorCode) int {
	switch code {
	case contracts.ErrorCodeInvalidRequest:
		return http.StatusBadRequest
	case contracts.ErrorCodeNotFound:
		return http.StatusNotFound
	case contracts.ErrorCodeUnauthorized:
		return http.StatusUnauthorized
	case contracts.ErrorCodeForbidden:
		return http.StatusForbidden
	case contracts.ErrorCodeConflict:
		return http.StatusConflict
	case contracts.ErrorCodeServiceUnavailable:
		return http.StatusServiceUnavailable
	case contracts.ErrorCodeTimeout:
		return http.StatusRequestTimeout
	case contracts.ErrorCodeRateLimit:
		return http.StatusTooManyRequests
	case contracts.ErrorCodeQuotaExceeded:
		return http.StatusInsufficientStorage
	default:
		return http.StatusInternalServerError
	}
}