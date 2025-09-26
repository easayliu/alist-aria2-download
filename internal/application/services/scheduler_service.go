package services

import (
	"fmt"
	"sync"
	"time"

	"github.com/easayliu/alist-aria2-download/internal/domain/entities"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/repository"
	"github.com/easayliu/alist-aria2-download/pkg/logger"
	"github.com/robfig/cron/v3"
)

type SchedulerService struct {
	cron            *cron.Cron
	taskRepo        *repository.TaskRepository
	fileService     *FileService
	notificationSvc *NotificationService
	downloadService *DownloadService
	jobs            map[string]cron.EntryID
	mu              sync.RWMutex
	running         bool
}

func NewSchedulerService(taskRepo *repository.TaskRepository, fileService *FileService, notificationSvc *NotificationService, downloadService *DownloadService) *SchedulerService {
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

	// 更新最后运行时间
	now := time.Now()
	s.taskRepo.UpdateLastRunTime(task.ID, now)

	// 计算时间范围
	startTime := now.Add(-time.Duration(task.HoursAgo) * time.Hour)

	// 使用新的方法获取文件列表（与 /api/v1/files/yesterday/download 一致）
	files, err := s.fileService.GetFilesByTimeRange(task.Path, startTime, now, task.VideoOnly)
	if err != nil {
		logger.Error("Failed to fetch files for scheduled task:", task.Name, "error:", err)
		s.notificationSvc.SendMessage(task.CreatedBy, fmt.Sprintf("定时任务执行失败: %s\n错误: %v", task.Name, err))
		return
	}

	if len(files) == 0 {
		logger.Info("No files found for scheduled task:", task.Name)
		// 也发送无文件的通知（可选，避免用户疑惑）
		if task.AutoPreview {
			s.notificationSvc.SendMessage(task.CreatedBy, fmt.Sprintf(
				"定时任务执行完成\n\n"+
					"任务: %s\n"+
					"路径: %s\n"+
					"时间范围: 最近%d小时\n"+
					"结果: 没有找到新文件",
				task.Name, task.Path, task.HoursAgo))
		}
		return
	}

	// 发送开始通知
	timeDesc := fmt.Sprintf("%d小时", task.HoursAgo)
	if task.HoursAgo == 24 {
		timeDesc = "24小时(1天)"
	} else if task.HoursAgo >= 168 {
		timeDesc = fmt.Sprintf("%d小时(%d天)", task.HoursAgo, task.HoursAgo/24)
	}

	message := fmt.Sprintf(
		"<b>定时任务开始执行</b>\n\n"+
			"任务名: %s\n"+
			"扫描路径: <code>%s</code>\n"+
			"时间范围: 最近%s内修改\n"+
			"找到文件: <b>%d</b> 个",
		task.Name, task.Path, timeDesc, len(files))

	if task.AutoPreview {
		message += "\n\n预览模式 - 不会实际下载"
		// 列出文件
		for i, file := range files {
			if i >= 10 {
				message += fmt.Sprintf("\n... 还有 %d 个文件", len(files)-10)
				break
			}
			message += fmt.Sprintf("\n• %s", file.Name)
		}
	} else {
		// 实际执行下载
		downloadCount := 0
		var downloadedFiles []string
		var totalSize int64

		for _, file := range files {
			// 视频过滤（如果需要）- files 已经按需过滤
			if task.VideoOnly && !s.fileService.IsVideoFile(file.Name) {
				continue
			}

			// 设置下载选项
			options := map[string]interface{}{
				"dir": file.DownloadPath,
				"out": file.Name,
			}

			// 创建下载任务（使用内部URL）
			if _, err := s.downloadService.CreateDownload(file.InternalURL, file.Name, file.DownloadPath, options); err != nil {
				logger.Error("Failed to create download for file:", file.Name, "error:", err)
			} else {
				downloadCount++
				totalSize += file.Size
				// 记录前5个文件名
				if len(downloadedFiles) < 5 {
					downloadedFiles = append(downloadedFiles, file.Name)
				}
			}
		}

		// 构建详细的通知消息
		if downloadCount > 0 {
			message += fmt.Sprintf("\n\n<b>已创建 %d 个下载任务</b>", downloadCount)

			// 格式化文件大小
			sizeStr := ""
			if totalSize < 1024*1024*1024 {
				sizeStr = fmt.Sprintf("%.2f MB", float64(totalSize)/(1024*1024))
			} else if totalSize < 1024*1024*1024*1024 {
				sizeStr = fmt.Sprintf("%.2f GB", float64(totalSize)/(1024*1024*1024))
			} else {
				sizeStr = fmt.Sprintf("%.2f TB", float64(totalSize)/(1024*1024*1024*1024))
			}
			message += fmt.Sprintf("\n总大小: %s", sizeStr)

			// 只显示前3个文件名，超过10个文件时显示摘要
			if downloadCount > 10 {
				// 大批量下载，只显示摘要信息
				message += fmt.Sprintf("\n\n<b>下载摘要:</b>\n")
				message += fmt.Sprintf("• 文件总数: %d 个\n", downloadCount)

				// 显示前3个文件名作为示例
				if len(downloadedFiles) > 0 {
					message += "\n<b>部分文件示例:</b>"
					maxShow := 3
					if len(downloadedFiles) < maxShow {
						maxShow = len(downloadedFiles)
					}
					for i := 0; i < maxShow; i++ {
						// 截断过长的文件名
						fileName := downloadedFiles[i]
						if len(fileName) > 50 {
							fileName = fileName[:47] + "..."
						}
						message += fmt.Sprintf("\n• %s", fileName)
					}
					message += fmt.Sprintf("\n... 等 %d 个文件", downloadCount)
				}
			} else if len(downloadedFiles) > 0 {
				// 少量文件，显示全部文件名
				message += "\n\n<b>下载文件:</b>"
				for _, fileName := range downloadedFiles {
					// 截断过长的文件名
					if len(fileName) > 60 {
						fileName = fileName[:57] + "..."
					}
					message += fmt.Sprintf("\n• %s", fileName)
				}
				if downloadCount > 5 {
					message += fmt.Sprintf("\n... 还有 %d 个文件", downloadCount-5)
				}
			}

			message += "\n\n提示: 使用 /status 查看下载进度"
		} else {
			message += "\n\n没有符合条件的文件需要下载"
		}
	}

	// 发送通知
	s.notificationSvc.SendMessage(task.CreatedBy, message)

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
