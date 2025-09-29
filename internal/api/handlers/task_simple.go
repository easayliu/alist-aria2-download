package handlers

import (
	"fmt"
	"net/http"
	"time"

	// apiutils "github.com/easayliu/alist-aria2-download/internal/api/utils"
	"github.com/easayliu/alist-aria2-download/pkg/utils"
	"github.com/easayliu/alist-aria2-download/internal/application/services"
	"github.com/easayliu/alist-aria2-download/internal/domain/entities"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/repository"
	"github.com/gin-gonic/gin"
)

// CreateTaskRequest 创建定时任务请求
type CreateTaskRequest struct {
	Name        string `json:"name" binding:"required" example:"每日同步"`
	Path        string `json:"path" binding:"required" example:"/data/来自：分享"`
	CronExpr    string `json:"cron_expr" binding:"required" example:"0 2 * * *"`
	HoursAgo    int    `json:"hours_ago" binding:"required,min=1" example:"24"`
	VideoOnly   bool   `json:"video_only" example:"true"`
	AutoPreview bool   `json:"auto_preview" example:"false"`
	Enabled     bool   `json:"enabled" example:"true"`
	CreatedBy   int64  `json:"created_by" example:"63401853"`
}

// UpdateTaskRequest 更新定时任务请求
type UpdateTaskRequest struct {
	Name        string `json:"name" example:"每日同步"`
	Path        string `json:"path" example:"/data/来自：分享"`
	CronExpr    string `json:"cron_expr" example:"0 2 * * *"`
	HoursAgo    int    `json:"hours_ago,omitempty" example:"24"`
	VideoOnly   bool   `json:"video_only" example:"true"`
	AutoPreview bool   `json:"auto_preview" example:"false"`
	Enabled     bool   `json:"enabled" example:"true"`
}

// CreateTask 创建定时任务
// @Summary 创建定时任务
// @Description 创建一个新的定时任务，按照cron表达式定期执行
// @Tags 定时任务
// @Accept json
// @Produce json
// @Param request body CreateTaskRequest true "创建任务请求"
// @Success 200 {object} map[string]interface{} "任务创建成功"
// @Failure 400 {object} map[string]interface{} "请求参数错误"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /tasks [post]
func CreateTask(c *gin.Context) {
	var req CreateTaskRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request parameters: " + err.Error(), "code": 400})
		return
	}

	// 获取scheduler服务实例（从上下文中）
	schedulerService := c.MustGet("schedulerService").(*services.SchedulerService)

	// 创建任务实体
	task := &entities.ScheduledTask{
		Name:        req.Name,
		Path:        req.Path,
		Cron:        req.CronExpr,
		HoursAgo:    req.HoursAgo,
		VideoOnly:   req.VideoOnly,
		AutoPreview: req.AutoPreview,
		Enabled:     req.Enabled,
		CreatedBy:   req.CreatedBy,
	}

	// 创建任务
	if err := schedulerService.CreateTask(task); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create task: " + err.Error(), "code": 500})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Task created successfully",
		"task":    task,
	})
}

// GetTask 获取单个定时任务
// @Summary 获取定时任务详情
// @Description 根据任务ID获取定时任务的详细信息
// @Tags 定时任务
// @Accept json
// @Produce json
// @Param id path string true "任务ID"
// @Success 200 {object} entities.ScheduledTask "任务详情"
// @Failure 404 {object} map[string]interface{} "任务不存在"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /tasks/{id} [get]
func GetTask(c *gin.Context) {
	taskID := c.Param("id")

	// 获取任务仓库（从上下文中）
	taskRepo := c.MustGet("taskRepo").(*repository.TaskRepository)

	task, err := taskRepo.GetByID(taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Task not found", "code": 404})
		return
	}

	c.JSON(http.StatusOK, task)
}

// ListTasks 获取任务列表
// @Summary 获取定时任务列表
// @Description 获取所有定时任务的列表
// @Tags 定时任务
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "任务列表"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /tasks [get]
func ListTasks(c *gin.Context) {
	// 获取任务仓库（从上下文中）
	taskRepo := c.MustGet("taskRepo").(*repository.TaskRepository)

	tasks, err := taskRepo.GetAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get tasks: " + err.Error(), "code": 500})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"total": len(tasks),
		"tasks": tasks,
	})
}

// UpdateTask 更新定时任务
// @Summary 更新定时任务
// @Description 更新指定ID的定时任务信息
// @Tags 定时任务
// @Accept json
// @Produce json
// @Param id path string true "任务ID"
// @Param request body UpdateTaskRequest true "更新任务请求"
// @Success 200 {object} map[string]interface{} "任务更新成功"
// @Failure 400 {object} map[string]interface{} "请求参数错误"
// @Failure 404 {object} map[string]interface{} "任务不存在"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /tasks/{id} [put]
func UpdateTask(c *gin.Context) {
	taskID := c.Param("id")
	var req UpdateTaskRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request parameters: " + err.Error(), "code": 400})
		return
	}

	// 获取任务仓库和调度服务
	taskRepo := c.MustGet("taskRepo").(*repository.TaskRepository)
	schedulerService := c.MustGet("schedulerService").(*services.SchedulerService)

	// 获取现有任务
	task, err := taskRepo.GetByID(taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Task not found", "code": 404})
		return
	}

	// 更新任务字段
	if req.Name != "" {
		task.Name = req.Name
	}
	if req.Path != "" {
		task.Path = req.Path
	}
	if req.CronExpr != "" {
		task.Cron = req.CronExpr
	}
	if req.HoursAgo > 0 {
		task.HoursAgo = req.HoursAgo
	}
	task.VideoOnly = req.VideoOnly
	task.AutoPreview = req.AutoPreview
	task.Enabled = req.Enabled

	// 更新任务
	if err := schedulerService.UpdateTask(task); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update task: " + err.Error(), "code": 500})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Task updated successfully",
		"task":    task,
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
func DeleteTask(c *gin.Context) {
	taskID := c.Param("id")

	// 获取调度服务
	schedulerService := c.MustGet("schedulerService").(*services.SchedulerService)

	// 删除任务
	if err := schedulerService.DeleteTask(taskID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete task: " + err.Error(), "code": 500})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Task deleted successfully",
		"task_id": taskID,
	})
}

// RunTaskRequest 执行任务请求
type RunTaskRequest struct {
	Preview bool `json:"preview" example:"false"` // 是否仅预览，不实际下载
}

// RunTaskNow 立即执行任务
// @Summary 立即执行定时任务
// @Description 立即执行指定ID的定时任务，不等待下一个调度时间
// @Tags 定时任务
// @Accept json
// @Produce json
// @Param id path string true "任务ID"
// @Param request body RunTaskRequest false "执行选项"
// @Success 200 {object} map[string]interface{} "任务已开始执行或预览结果"
// @Failure 404 {object} map[string]interface{} "任务不存在"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /tasks/{id}/run [post]
func RunTaskNow(c *gin.Context) {
	taskID := c.Param("id")
	var req RunTaskRequest

	// 尝试绑定请求体（可选）
	c.ShouldBindJSON(&req)

	// 如果是预览模式，重定向到预览接口
	if req.Preview {
		PreviewTask(c)
		return
	}

	// 获取调度服务
	schedulerService := c.MustGet("schedulerService").(*services.SchedulerService)

	// 立即执行任务
	if err := schedulerService.RunTaskNow(taskID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to run task: " + err.Error(), "code": 500})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Task started successfully",
		"task_id": taskID,
	})
}

// PreviewTask 预览定时任务将要下载的文件
// @Summary 预览定时任务
// @Description 预览定时任务将要下载的文件，不实际执行下载
// @Tags 定时任务
// @Accept json
// @Produce json
// @Param id path string true "任务ID"
// @Success 200 {object} map[string]interface{} "预览结果"
// @Failure 404 {object} map[string]interface{} "任务不存在"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /tasks/{id}/preview [get]
func PreviewTask(c *gin.Context) {
	taskID := c.Param("id")

	// 获取任务仓库和文件服务
	taskRepo := c.MustGet("taskRepo").(*repository.TaskRepository)
	fileService := c.MustGet("fileService").(*services.FileService)

	// 获取任务
	task, err := taskRepo.GetByID(taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Task not found", "code": 404})
		return
	}

	// 计算时间范围
	endTime := time.Now()
	startTime := endTime.Add(-time.Duration(task.HoursAgo) * time.Hour)

	// 获取文件列表（与 /api/v1/files/yesterday/download 一致的实现）
	files, err := fileService.GetFilesByTimeRange(task.Path, startTime, endTime, task.VideoOnly)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch files: " + err.Error(), "code": 500})
		return
	}

	// 构建预览结果
	previewResults := make([]map[string]interface{}, 0, len(files))
	var totalSize int64
	var tvCount, movieCount, otherCount int

	for _, file := range files {
		totalSize += file.Size

		// 统计媒体类型
		switch file.MediaType {
		case "tv":
			tvCount++
		case "movie":
			movieCount++
		default:
			otherCount++
		}

		previewResults = append(previewResults, map[string]interface{}{
			"name":          file.Name,
			"path":          file.Path,
			"size":          file.Size,
			"modified":      file.Modified,
			"media_type":    file.MediaType,
			"download_path": file.DownloadPath,
			"internal_url":  file.InternalURL,
		})
	}

	// 格式化文件大小
	sizeStr := ""
	if totalSize < 1024*1024*1024 {
		sizeStr = fmt.Sprintf("%.2f MB", float64(totalSize)/(1024*1024))
	} else if totalSize < 1024*1024*1024*1024 {
		sizeStr = fmt.Sprintf("%.2f GB", float64(totalSize)/(1024*1024*1024))
	} else {
		sizeStr = fmt.Sprintf("%.2f TB", float64(totalSize)/(1024*1024*1024*1024))
	}

	c.JSON(http.StatusOK, gin.H{
		"task": gin.H{
			"id":           task.ID,
			"name":         task.Name,
			"path":         task.Path,
			"hours_ago":    task.HoursAgo,
			"video_only":   task.VideoOnly,
			"auto_preview": task.AutoPreview,
			"cron":         task.Cron,
		},
		"preview": gin.H{
			"total_files": len(files),
			"total_size":  sizeStr,
			"time_range": gin.H{
				"start": startTime.Format("2006-01-02 15:04:05"),
				"end":   endTime.Format("2006-01-02 15:04:05"),
			},
			"media_stats": utils.BuildMediaStats(tvCount, movieCount, otherCount),
			"files": previewResults,
		},
	})
}

// ToggleTask 启用/禁用任务
// @Summary 启用或禁用定时任务
// @Description 切换定时任务的启用状态
// @Tags 定时任务
// @Accept json
// @Produce json
// @Param id path string true "任务ID"
// @Param enabled query bool true "是否启用"
// @Success 200 {object} map[string]interface{} "状态更新成功"
// @Failure 404 {object} map[string]interface{} "任务不存在"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /tasks/{id}/toggle [post]
func ToggleTask(c *gin.Context) {
	taskID := c.Param("id")
	enabled := c.Query("enabled") == "true"

	// 获取调度服务
	schedulerService := c.MustGet("schedulerService").(*services.SchedulerService)

	// 切换任务状态
	if err := schedulerService.ToggleTask(taskID, enabled); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to toggle task: " + err.Error(), "code": 500})
		return
	}

	status := "disabled"
	if enabled {
		status = "enabled"
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Task " + status + " successfully",
		"task_id": taskID,
		"enabled": enabled,
	})
}
