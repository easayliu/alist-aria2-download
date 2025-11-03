package task

import (
	"context"
	"fmt"
	"time"

	"github.com/easayliu/alist-aria2-download/internal/application/contracts"
	"github.com/easayliu/alist-aria2-download/internal/domain/entities"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/config"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/repository"
	"github.com/easayliu/alist-aria2-download/pkg/logger"
	"github.com/robfig/cron/v3"
)

// AppTaskService 应用层任务服务 - 负责任务业务流程编排
type AppTaskService struct {
	config           *config.Config
	taskRepo         *repository.TaskRepository
	schedulerService *SchedulerService
	downloadService  contracts.DownloadService
	fileService      contracts.FileService
	cron             *cron.Cron
}

// NewAppTaskService 创建应用任务服务
func NewAppTaskService(
	cfg *config.Config,
	taskRepo *repository.TaskRepository,
	schedulerService *SchedulerService,
	downloadService contracts.DownloadService,
	fileService contracts.FileService,
) contracts.TaskService {
	return &AppTaskService{
		config:           cfg,
		taskRepo:         taskRepo,
		schedulerService: schedulerService,
		downloadService:  downloadService,
		fileService:      fileService,
		cron:             cron.New(),
	}
}

// CreateTask 创建任务 - 统一的业务逻辑
func (s *AppTaskService) CreateTask(ctx context.Context, req contracts.TaskRequest) (*contracts.TaskResponse, error) {
	logger.Info("Creating task", "name", req.Name, "cron", req.CronExpr)

	// 1. 参数验证
	if err := s.validateTaskRequest(req); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// 2. 验证Cron表达式
	if _, err := cron.ParseStandard(req.CronExpr); err != nil {
		return nil, fmt.Errorf("invalid cron expression: %w", err)
	}

	// 3. 创建任务实体
	task := &entities.ScheduledTask{
		Name:        req.Name,
		Path:        req.Path,
		Cron:        req.CronExpr,
		HoursAgo:    req.HoursAgo,
		VideoOnly:   req.VideoOnly,
		AutoPreview: req.AutoPreview,
		Enabled:     req.Enabled,
		CreatedBy:   req.CreatedBy,
		Status:      entities.TaskStatusIdle,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// 4. 保存到数据库
	if err := s.schedulerService.CreateTask(task); err != nil {
		return nil, fmt.Errorf("failed to create task: %w", err)
	}

	// 5. 计算下次执行时间
	if task.Enabled {
		s.calculateNextRunTime(task)
	}

	response := s.convertToTaskResponse(task)
	logger.Info("Task created successfully", "id", task.ID, "name", task.Name)
	return response, nil
}

// GetTask 获取任务详情
func (s *AppTaskService) GetTask(ctx context.Context, id string) (*contracts.TaskResponse, error) {
	task, err := s.taskRepo.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("task not found: %w", err)
	}

	return s.convertToTaskResponse(task), nil
}

// UpdateTask 更新任务
func (s *AppTaskService) UpdateTask(ctx context.Context, id string, req contracts.TaskUpdateRequest) (*contracts.TaskResponse, error) {
	// 获取现有任务
	task, err := s.taskRepo.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("task not found: %w", err)
	}

	// 更新字段
	updated := false
	if req.Name != nil && *req.Name != task.Name {
		task.Name = *req.Name
		updated = true
	}
	if req.Path != nil && *req.Path != task.Path {
		task.Path = *req.Path
		updated = true
	}
	if req.CronExpr != nil && *req.CronExpr != task.Cron {
		// 验证新的Cron表达式
		if _, err := cron.ParseStandard(*req.CronExpr); err != nil {
			return nil, fmt.Errorf("invalid cron expression: %w", err)
		}
		task.Cron = *req.CronExpr
		updated = true
	}
	if req.HoursAgo != nil && *req.HoursAgo != task.HoursAgo {
		task.HoursAgo = *req.HoursAgo
		updated = true
	}
	if req.VideoOnly != nil && *req.VideoOnly != task.VideoOnly {
		task.VideoOnly = *req.VideoOnly
		updated = true
	}
	if req.AutoPreview != nil && *req.AutoPreview != task.AutoPreview {
		task.AutoPreview = *req.AutoPreview
		updated = true
	}
	if req.Enabled != nil && *req.Enabled != task.Enabled {
		task.Enabled = *req.Enabled
		updated = true
	}

	if !updated {
		return s.convertToTaskResponse(task), nil
	}

	task.UpdatedAt = time.Now()

	// 更新任务
	if err := s.schedulerService.UpdateTask(task); err != nil {
		return nil, fmt.Errorf("failed to update task: %w", err)
	}

	// 重新计算下次执行时间
	if task.Enabled {
		s.calculateNextRunTime(task)
	}

	return s.convertToTaskResponse(task), nil
}

// DeleteTask 删除任务
func (s *AppTaskService) DeleteTask(ctx context.Context, id string) error {
	if err := s.schedulerService.DeleteTask(id); err != nil {
		return fmt.Errorf("failed to delete task: %w", err)
	}
	logger.Info("Task deleted", "id", id)
	return nil
}

// ListTasks 获取任务列表
func (s *AppTaskService) ListTasks(ctx context.Context, req contracts.TaskListRequest) (*contracts.TaskListResponse, error) {
	// 从数据库获取任务
	tasks, err := s.taskRepo.GetAll()
	if err != nil {
		return nil, fmt.Errorf("failed to get tasks: %w", err)
	}

	// 应用过滤
	filteredTasks := s.filterTasks(tasks, req)

	// 转换为响应格式
	var responses []contracts.TaskResponse
	summary := contracts.TaskSummary{}

	for _, task := range filteredTasks {
		responses = append(responses, *s.convertToTaskResponse(task))

		// 统计摘要
		if task.Enabled {
			summary.EnabledCount++
		} else {
			summary.DisabledCount++
		}

		switch task.Status {
		case entities.TaskStatusRunning:
			summary.RunningCount++
		case entities.TaskStatusError:
			summary.ErrorCount++
		}
	}

	return &contracts.TaskListResponse{
		Tasks:      responses,
		TotalCount: len(responses),
		Summary:    summary,
	}, nil
}

// EnableTask 启用任务
func (s *AppTaskService) EnableTask(ctx context.Context, id string) error {
	return s.schedulerService.ToggleTask(id, true)
}

// DisableTask 禁用任务
func (s *AppTaskService) DisableTask(ctx context.Context, id string) error {
	return s.schedulerService.ToggleTask(id, false)
}

// RunTaskNow 立即运行任务
func (s *AppTaskService) RunTaskNow(ctx context.Context, req contracts.TaskRunRequest) (*contracts.TaskRunResponse, error) {
	// 获取任务
	task, err := s.taskRepo.GetByID(req.TaskID)
	if err != nil {
		return nil, fmt.Errorf("task not found: %w", err)
	}

	runID := fmt.Sprintf("run_%s_%d", req.TaskID[:8], time.Now().Unix())
	startTime := time.Now()

	// 如果是预览模式
	if req.Preview {
		preview, err := s.previewTaskExecution(ctx, task)
		if err != nil {
			return nil, fmt.Errorf("failed to preview task: %w", err)
		}

		return &contracts.TaskRunResponse{
			TaskID:    req.TaskID,
			RunID:     runID,
			StartedAt: startTime,
			Status:    "preview",
			Preview:   preview,
		}, nil
	}

	// 实际执行任务
	downloadIDs, err := s.executeTask(ctx, task)
	if err != nil {
		return nil, fmt.Errorf("failed to execute task: %w", err)
	}

	// 更新任务状态
	task.LastRunAt = &startTime
	task.RunCount++
	if err == nil {
		task.SuccessCount++
	} else {
		task.FailureCount++
	}
	s.taskRepo.Update(task)

	return &contracts.TaskRunResponse{
		TaskID:      req.TaskID,
		RunID:       runID,
		StartedAt:   startTime,
		Status:      "running",
		DownloadIDs: downloadIDs,
	}, nil
}

// StopTask 停止任务
func (s *AppTaskService) StopTask(ctx context.Context, id string) error {
	// 这里需要实现任务停止逻辑
	logger.Info("Stopping task", "id", id)
	return nil
}

// PreviewTask 预览任务
func (s *AppTaskService) PreviewTask(ctx context.Context, req contracts.TaskPreviewRequest) (*contracts.TaskPreviewResponse, error) {
	task, err := s.taskRepo.GetByID(req.TaskID)
	if err != nil {
		return nil, fmt.Errorf("task not found: %w", err)
	}

	return s.previewTaskExecution(ctx, task)
}

// CreateQuickTask 创建快捷任务
func (s *AppTaskService) CreateQuickTask(ctx context.Context, req contracts.QuickTaskRequest) (*contracts.TaskResponse, error) {
	// 解析路径
	path := req.Path
	if path == "" {
		path = s.config.Alist.DefaultPath
		if path == "" {
			path = "/"
		}
	}

	var taskReq contracts.TaskRequest

	switch req.Type {
	case "daily":
		taskReq = contracts.TaskRequest{
			Name:      fmt.Sprintf("每日下载-%s", path),
			Path:      path,
			CronExpr:  "0 2 * * *", // 每天凌晨2点
			HoursAgo:  24,
			VideoOnly: true,
			Enabled:   true,
			CreatedBy: req.CreatedBy,
		}
	case "recent":
		taskReq = contracts.TaskRequest{
			Name:      fmt.Sprintf("频繁同步-%s", path),
			Path:      path,
			CronExpr:  "0 */2 * * *", // 每2小时
			HoursAgo:  2,
			VideoOnly: true,
			Enabled:   true,
			CreatedBy: req.CreatedBy,
		}
	case "weekly":
		taskReq = contracts.TaskRequest{
			Name:      fmt.Sprintf("每周汇总-%s", path),
			Path:      path,
			CronExpr:  "0 9 * * 1", // 每周一早9点
			HoursAgo:  168,         // 7天
			VideoOnly: true,
			Enabled:   true,
			CreatedBy: req.CreatedBy,
		}
	case "realtime":
		taskReq = contracts.TaskRequest{
			Name:      fmt.Sprintf("实时同步-%s", path),
			Path:      path,
			CronExpr:  "0 * * * *", // 每小时
			HoursAgo:  1,
			VideoOnly: true,
			Enabled:   true,
			CreatedBy: req.CreatedBy,
		}
	default:
		return nil, fmt.Errorf("unknown task type: %s", req.Type)
	}

	return s.CreateTask(ctx, taskReq)
}

// GetUserTasks 获取用户任务
func (s *AppTaskService) GetUserTasks(ctx context.Context, userID int64) (*contracts.TaskListResponse, error) {
	req := contracts.TaskListRequest{
		CreatedBy: userID,
	}
	return s.ListTasks(ctx, req)
}

// GetTaskStatistics 获取任务统计
func (s *AppTaskService) GetTaskStatistics(ctx context.Context) (map[string]interface{}, error) {
	tasks, err := s.taskRepo.GetAll()
	if err != nil {
		return nil, fmt.Errorf("failed to get tasks: %w", err)
	}

	stats := map[string]interface{}{
		"total_tasks":    len(tasks),
		"enabled_tasks":  0,
		"disabled_tasks": 0,
		"running_tasks":  0,
		"error_tasks":    0,
		"total_runs":     0,
		"success_runs":   0,
		"failure_runs":   0,
	}

	for _, task := range tasks {
		if task.Enabled {
			stats["enabled_tasks"] = stats["enabled_tasks"].(int) + 1
		} else {
			stats["disabled_tasks"] = stats["disabled_tasks"].(int) + 1
		}

		switch task.Status {
		case entities.TaskStatusRunning:
			stats["running_tasks"] = stats["running_tasks"].(int) + 1
		case entities.TaskStatusError:
			stats["error_tasks"] = stats["error_tasks"].(int) + 1
		}

		stats["total_runs"] = stats["total_runs"].(int) + task.RunCount
		stats["success_runs"] = stats["success_runs"].(int) + task.SuccessCount
		stats["failure_runs"] = stats["failure_runs"].(int) + task.FailureCount
	}

	return stats, nil
}

// GetSchedulerStatus 获取调度器状态
func (s *AppTaskService) GetSchedulerStatus(ctx context.Context) (map[string]interface{}, error) {
	return map[string]interface{}{
		"status":      "running",
		"version":     "2.0.0",
		"active_jobs": len(s.cron.Entries()),
		"uptime":      time.Since(time.Now()).String(), // 这里需要实际的启动时间
	}, nil
}

// ========== 私有方法 ==========

// validateTaskRequest 验证任务请求
func (s *AppTaskService) validateTaskRequest(req contracts.TaskRequest) error {
	if req.Name == "" {
		return fmt.Errorf("task name is required")
	}
	if req.Path == "" {
		return fmt.Errorf("task path is required")
	}
	if req.CronExpr == "" {
		return fmt.Errorf("cron expression is required")
	}
	if req.HoursAgo <= 0 {
		return fmt.Errorf("hours_ago must be positive")
	}
	return nil
}

// calculateNextRunTime 计算下次执行时间
func (s *AppTaskService) calculateNextRunTime(task *entities.ScheduledTask) {
	if schedule, err := cron.ParseStandard(task.Cron); err == nil {
		nextTime := schedule.Next(time.Now())
		task.NextRunAt = &nextTime
	}
}

// convertToTaskResponse 转换任务实体到响应格式
func (s *AppTaskService) convertToTaskResponse(task *entities.ScheduledTask) *contracts.TaskResponse {
	return &contracts.TaskResponse{
		ID:           task.ID,
		Name:         task.Name,
		Path:         task.Path,
		CronExpr:     task.Cron,
		HoursAgo:     task.HoursAgo,
		VideoOnly:    task.VideoOnly,
		AutoPreview:  task.AutoPreview,
		Enabled:      task.Enabled,
		CreatedBy:    task.CreatedBy,
		Status:       task.Status,
		LastRunAt:    task.LastRunAt,
		NextRunAt:    task.NextRunAt,
		RunCount:     task.RunCount,
		SuccessCount: task.SuccessCount,
		FailureCount: task.FailureCount,
		CreatedAt:    task.CreatedAt,
		UpdatedAt:    task.UpdatedAt,
	}
}

// filterTasks 过滤任务列表
func (s *AppTaskService) filterTasks(tasks []*entities.ScheduledTask, req contracts.TaskListRequest) []*entities.ScheduledTask {
	var filtered []*entities.ScheduledTask

	for _, task := range tasks {
		// 按用户过滤
		if req.CreatedBy != 0 && task.CreatedBy != req.CreatedBy {
			continue
		}

		// 按启用状态过滤
		if req.Enabled != nil && task.Enabled != *req.Enabled {
			continue
		}

		// 按状态过滤
		if req.Status != "" && string(task.Status) != req.Status {
			continue
		}

		filtered = append(filtered, task)
	}

	return filtered
}

// previewTaskExecution 预览任务执行
func (s *AppTaskService) previewTaskExecution(ctx context.Context, task *entities.ScheduledTask) (*contracts.TaskPreviewResponse, error) {
	// 计算时间范围
	endTime := time.Now()
	startTime := endTime.Add(-time.Duration(task.HoursAgo) * time.Hour)

	// 获取文件列表
	fileReq := contracts.TimeRangeFileRequest{
		Path:      task.Path,
		StartTime: startTime,
		EndTime:   endTime,
		VideoOnly: task.VideoOnly,
	}

	fileResp, err := s.fileService.GetFilesByTimeRange(ctx, fileReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get files: %w", err)
	}

	// 转换文件格式
	var filePreviews []contracts.FilePreview
	for _, file := range fileResp.Files {
		filePreviews = append(filePreviews, contracts.FilePreview{
			Name:         file.Name,
			Path:         file.Path,
			Size:         file.Size,
			Modified:     file.Modified,
			MediaType:    file.MediaType,
			DownloadPath: file.DownloadPath,
			InternalURL:  file.InternalURL,
		})
	}

	// 构建预览摘要
	summary := contracts.PreviewSummary{
		TotalFiles: len(filePreviews),
		TotalSize:  fileResp.Summary.TotalSizeFormatted,
		VideoFiles: fileResp.Summary.VideoFiles,
		MovieFiles: fileResp.Summary.MovieFiles,
		TVFiles:    fileResp.Summary.TVFiles,
		OtherFiles: fileResp.Summary.OtherFiles,
	}

	return &contracts.TaskPreviewResponse{
		Task:    *s.convertToTaskResponse(task),
		Files:   filePreviews,
		Summary: summary,
		TimeRange: contracts.TimeRange{
			Start: startTime,
			End:   endTime,
		},
	}, nil
}

// executeTask 执行任务
func (s *AppTaskService) executeTask(ctx context.Context, task *entities.ScheduledTask) ([]string, error) {
	// 获取要下载的文件
	endTime := time.Now()
	startTime := endTime.Add(-time.Duration(task.HoursAgo) * time.Hour)

	fileReq := contracts.TimeRangeFileRequest{
		Path:      task.Path,
		StartTime: startTime,
		EndTime:   endTime,
		VideoOnly: task.VideoOnly,
	}

	fileResp, err := s.fileService.GetFilesByTimeRange(ctx, fileReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get files: %w", err)
	}

	// 批量创建下载任务
	var downloadRequests []contracts.DownloadRequest
	for _, file := range fileResp.Files {
		downloadRequests = append(downloadRequests, contracts.DownloadRequest{
			URL:          file.InternalURL,
			Filename:     file.Name,
			Directory:    file.DownloadPath,
			VideoOnly:    task.VideoOnly,
			AutoClassify: true,
		})
	}

	if len(downloadRequests) == 0 {
		return []string{}, nil
	}

	batchReq := contracts.BatchDownloadRequest{
		Items:        downloadRequests,
		VideoOnly:    task.VideoOnly,
		AutoClassify: true,
	}

	batchResp, err := s.downloadService.CreateBatchDownload(ctx, batchReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create batch download: %w", err)
	}

	// 收集成功的下载ID
	var downloadIDs []string
	for _, result := range batchResp.Results {
		if result.Success && result.Download != nil {
			downloadIDs = append(downloadIDs, result.Download.ID)
		}
	}

	logger.Info("Task executed",
		"task_id", task.ID,
		"files_found", len(fileResp.Files),
		"downloads_created", len(downloadIDs),
		"success_count", batchResp.SuccessCount,
		"failure_count", batchResp.FailureCount)

	return downloadIDs, nil
}
