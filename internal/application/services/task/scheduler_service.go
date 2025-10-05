package task

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/easayliu/alist-aria2-download/internal/application/contracts"
	"github.com/easayliu/alist-aria2-download/internal/domain/entities"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/repository"
	"github.com/easayliu/alist-aria2-download/pkg/logger"
	"github.com/robfig/cron/v3"
)

type SchedulerService struct {
	cron            *cron.Cron
	taskRepo        *repository.TaskRepository
	fileService     contracts.FileService
	notificationSvc contracts.NotificationService
	downloadService contracts.DownloadService
	jobs            map[string]cron.EntryID
	mu              sync.RWMutex
	running         bool
}

func NewSchedulerService(taskRepo *repository.TaskRepository, fileService contracts.FileService, notificationSvc contracts.NotificationService, downloadService contracts.DownloadService) *SchedulerService {
	return &SchedulerService{
		cron:            cron.New(), // 使用标准5字段格式（分 时 日 月 周）
		taskRepo:        taskRepo,
		fileService:     fileService,
		notificationSvc: notificationSvc,
		downloadService: downloadService,
		jobs:            make(map[string]cron.EntryID),
		running:         false,
	}
}

// Start 启动调度器
func (s *SchedulerService) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("scheduler already running")
	}

	// 加载所有启用的任务
	tasks, err := s.taskRepo.GetAll()
	if err != nil {
		return fmt.Errorf("failed to load tasks: %w", err)
	}

	// 注册所有启用的任务
	for _, task := range tasks {
		if task.Enabled {
			if err := s.scheduleTask(task); err != nil {
				logger.Error("Failed to schedule task:", task.Name, "error:", err)
			}
		}
	}

	s.cron.Start()
	s.running = true
	logger.Info("Scheduler service started")

	return nil
}

// Stop 停止调度器
func (s *SchedulerService) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		s.cron.Stop()
		s.running = false
		logger.Info("Scheduler service stopped")
	}
}

// CreateTask 创建新任务
func (s *SchedulerService) CreateTask(task *entities.ScheduledTask) error {
	// 验证cron表达式
	if _, err := cron.ParseStandard(task.Cron); err != nil {
		return fmt.Errorf("invalid cron expression: %w", err)
	}

	// 保存任务
	if err := s.taskRepo.Create(task); err != nil {
		return fmt.Errorf("failed to save task: %w", err)
	}

	// 如果任务启用且调度器正在运行，立即调度
	if task.Enabled && s.running {
		s.mu.Lock()
		defer s.mu.Unlock()
		if err := s.scheduleTask(task); err != nil {
			// 删除已保存的任务
			s.taskRepo.Delete(task.ID)
			return fmt.Errorf("failed to schedule task: %w", err)
		}
	}

	logger.Info("Task created:", task.Name, "ID:", task.ID)
	return nil
}

// UpdateTask 更新任务
func (s *SchedulerService) UpdateTask(task *entities.ScheduledTask) error {
	// 验证cron表达式
	if _, err := cron.ParseStandard(task.Cron); err != nil {
		return fmt.Errorf("invalid cron expression: %w", err)
	}

	// 更新任务
	if err := s.taskRepo.Update(task); err != nil {
		return fmt.Errorf("failed to update task: %w", err)
	}

	// 重新调度任务
	s.mu.Lock()
	defer s.mu.Unlock()

	// 移除旧的调度
	if entryID, exists := s.jobs[task.ID]; exists {
		s.cron.Remove(entryID)
		delete(s.jobs, task.ID)
	}

	// 如果任务启用且调度器正在运行，重新调度
	if task.Enabled && s.running {
		if err := s.scheduleTask(task); err != nil {
			return fmt.Errorf("failed to reschedule task: %w", err)
		}
	}

	logger.Info("Task updated:", task.Name, "ID:", task.ID)
	return nil
}

// DeleteTask 删除任务
func (s *SchedulerService) DeleteTask(taskID string) error {
	// 删除任务
	if err := s.taskRepo.Delete(taskID); err != nil {
		return fmt.Errorf("failed to delete task: %w", err)
	}

	// 移除调度
	s.mu.Lock()
	defer s.mu.Unlock()

	if entryID, exists := s.jobs[taskID]; exists {
		s.cron.Remove(entryID)
		delete(s.jobs, taskID)
	}

	logger.Info("Task deleted:", taskID)
	return nil
}

// GetTask 获取任务
func (s *SchedulerService) GetTask(taskID string) (*entities.ScheduledTask, error) {
	return s.taskRepo.GetByID(taskID)
}

// GetAllTasks 获取所有任务
func (s *SchedulerService) GetAllTasks() ([]*entities.ScheduledTask, error) {
	return s.taskRepo.GetAll()
}

// GetUserTasks 获取用户创建的任务
func (s *SchedulerService) GetUserTasks(userID int64) ([]*entities.ScheduledTask, error) {
	return s.taskRepo.GetByUserID(userID)
}

// scheduleTask 调度单个任务（内部方法，需要加锁）
func (s *SchedulerService) scheduleTask(task *entities.ScheduledTask) error {
	// 创建任务执行函数
	jobFunc := func() {
		s.executeTask(task)
	}

	// 添加到cron
	entryID, err := s.cron.AddFunc(task.Cron, jobFunc)
	if err != nil {
		return err
	}

	s.jobs[task.ID] = entryID

	// 更新下次运行时间
	entry := s.cron.Entry(entryID)
	if entry.ID != 0 {
		nextTime := entry.Next
		s.taskRepo.UpdateNextRunTime(task.ID, nextTime)
	}

	return nil
}

// executeTask 执行任务
func (s *SchedulerService) executeTask(task *entities.ScheduledTask) {
	logger.Info("Executing scheduled task:", task.Name)

	// 创建context
	ctx := context.Background()

	// 更新最后运行时间
	now := time.Now()
	s.taskRepo.UpdateLastRunTime(task.ID, now)

	// 计算时间范围
	startTime := now.Add(-time.Duration(task.HoursAgo) * time.Hour)

	// 使用新的contracts接口获取文件列表
	req := contracts.TimeRangeFileRequest{
		Path:      task.Path,
		StartTime: startTime,
		EndTime:   now,
		VideoOnly: task.VideoOnly,
		HoursAgo:  task.HoursAgo,
	}

	resp, err := s.fileService.GetFilesByTimeRange(ctx, req)
	if err != nil {
		logger.Error("Failed to fetch files for scheduled task:", task.Name, "error:", err)

		// 发送失败通知
		failReq := contracts.TaskNotificationRequest{
			TaskID:       task.ID,
			TaskName:     task.Name,
			TaskType:     "scheduled",
			Status:       "failed",
			ErrorMessage: err.Error(),
		}
		s.notificationSvc.NotifyTaskFailed(ctx, failReq)
		return
	}

	files := resp.Files

	if len(files) == 0 {
		logger.Info("No files found for scheduled task:", task.Name)
		// 也发送无文件的通知（可选，避免用户疑惑）
		if task.AutoPreview {
			completeReq := contracts.TaskNotificationRequest{
				TaskID:     task.ID,
				TaskName:   task.Name,
				TaskType:   "scheduled",
				Status:     "completed",
				FilesCount: 0,
				Extra: map[string]interface{}{
					"path":      task.Path,
					"hours_ago": task.HoursAgo,
					"message":   "没有找到新文件",
				},
			}
			s.notificationSvc.NotifyTaskComplete(ctx, completeReq)
		}
		return
	}

	// 记录执行开始时间
	executionStart := time.Now()

	// 计算总大小
	var totalSize int64
	for _, file := range files {
		totalSize += file.Size
	}

	if task.AutoPreview {
		// 预览模式 - 不实际下载,只发送通知
		completeReq := contracts.TaskNotificationRequest{
			TaskID:     task.ID,
			TaskName:   task.Name,
			TaskType:   "scheduled",
			Status:     "completed",
			FilesCount: len(files),
			TotalSize:  totalSize,
			Duration:   time.Since(executionStart),
			Extra: map[string]interface{}{
				"path":      task.Path,
				"hours_ago": task.HoursAgo,
				"preview":   true,
				"files":     files[:min(10, len(files))], // 只传递前10个文件
			},
		}
		s.notificationSvc.NotifyTaskComplete(ctx, completeReq)
	} else {
		// 实际执行下载
		downloadCount := 0
		var downloadedFiles []string
		var downloadedSize int64

		for _, file := range files {
			// 视频过滤（如果需要）- files 已经按需过滤
			if task.VideoOnly && !s.fileService.IsVideoFile(file.Name) {
				continue
			}

			// 构建下载请求
			downloadReq := contracts.DownloadRequest{
				URL:       file.InternalURL,
				Filename:  file.Name,
				Directory: file.DownloadPath,
				FileSize:  file.Size,
				Options: map[string]interface{}{
					"dir": file.DownloadPath,
					"out": file.Name,
				},
			}

			// 创建下载任务
			if _, err := s.downloadService.CreateDownload(ctx, downloadReq); err != nil {
				logger.Error("Failed to create download for file:", file.Name, "error:", err)
			} else {
				downloadCount++
				downloadedSize += file.Size
				// 记录前5个文件名
				if len(downloadedFiles) < 5 {
					downloadedFiles = append(downloadedFiles, file.Name)
				}
			}
		}

		// 发送完成通知
		if downloadCount > 0 {
			completeReq := contracts.TaskNotificationRequest{
				TaskID:     task.ID,
				TaskName:   task.Name,
				TaskType:   "scheduled",
				Status:     "completed",
				FilesCount: downloadCount,
				TotalSize:  downloadedSize,
				Duration:   time.Since(executionStart),
				Extra: map[string]interface{}{
					"path":            task.Path,
					"hours_ago":       task.HoursAgo,
					"downloaded_files": downloadedFiles,
					"total_files":     len(files),
				},
			}
			s.notificationSvc.NotifyTaskComplete(ctx, completeReq)
		} else {
			// 没有文件需要下载
			completeReq := contracts.TaskNotificationRequest{
				TaskID:     task.ID,
				TaskName:   task.Name,
				TaskType:   "scheduled",
				Status:     "completed",
				FilesCount: 0,
				Duration:   time.Since(executionStart),
				Extra: map[string]interface{}{
					"path":      task.Path,
					"hours_ago": task.HoursAgo,
					"message":   "没有符合条件的文件需要下载",
				},
			}
			s.notificationSvc.NotifyTaskComplete(ctx, completeReq)
		}
	}

	// 更新下次运行时间
	s.mu.RLock()
	if entryID, exists := s.jobs[task.ID]; exists {
		entry := s.cron.Entry(entryID)
		if entry.ID != 0 {
			s.taskRepo.UpdateNextRunTime(task.ID, entry.Next)
		}
	}
	s.mu.RUnlock()
}

// RunTaskNow 立即运行任务
func (s *SchedulerService) RunTaskNow(taskID string) error {
	task, err := s.taskRepo.GetByID(taskID)
	if err != nil {
		return fmt.Errorf("failed to get task: %w", err)
	}

	// 在新的goroutine中执行，避免阻塞
	go s.executeTask(task)

	return nil
}

// ToggleTask 启用/禁用任务
func (s *SchedulerService) ToggleTask(taskID string, enabled bool) error {
	task, err := s.taskRepo.GetByID(taskID)
	if err != nil {
		return fmt.Errorf("failed to get task: %w", err)
	}

	task.Enabled = enabled
	return s.UpdateTask(task)
}

// min 返回两个整数中的最小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
