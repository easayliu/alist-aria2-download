package services

import (
	"context"
	"fmt"
	
	"github.com/easayliu/alist-aria2-download/internal/application/contracts"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/config"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/repository"
)

// ServiceContainer 应用服务容器 - 实现依赖注入
type ServiceContainer struct {
	config   *config.Config
	
	// 应用服务缓存
	downloadService     contracts.DownloadService
	fileService        contracts.FileService
	taskService        contracts.TaskService
	notificationService contracts.NotificationService
	
	// 基础设施服务（非contracts）
	taskRepo        *repository.TaskRepository
	schedulerService *SchedulerService
	oldNotificationSvc *NotificationService // 兼容旧版本的通知服务
}

// NewServiceContainer 创建服务容器
func NewServiceContainer(cfg *config.Config) (*ServiceContainer, error) {
	container := &ServiceContainer{
		config: cfg,
	}
	
	// 1. 初始化基础设施层
	dataDir := "./data" // 使用固定的数据目录
	taskRepo, err := repository.NewTaskRepository(dataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create task repository: %w", err)
	}
	container.taskRepo = taskRepo
	
	// 2. 初始化应用服务 - 注意依赖顺序
	// 先初始化不依赖其他服务的服务
	container.notificationService = NewAppNotificationService(cfg)
	container.fileService = NewAppFileService(cfg, nil) // 暂时传nil，稍后会设置downloadService
	container.downloadService = NewAppDownloadService(cfg, container.fileService)
	
	// 更新fileService的downloadService依赖
	// 注意：由于字段私有，需要添加setter方法
	if appFileService, ok := container.fileService.(*AppFileService); ok {
		appFileService.SetDownloadService(container.downloadService)
	}
	
	// 3. 初始化需要复杂依赖的服务
	// 创建兼容性的旧版本服务用于SchedulerService
	container.oldNotificationSvc = NewNotificationService(cfg)
	
	// 这里需要创建兼容的旧版本服务，暂时跳过SchedulerService和TaskService的完整初始化
	// container.schedulerService = NewSchedulerService(container.taskRepo, oldFileService, container.oldNotificationSvc, oldDownloadService)
	// container.taskService = NewAppTaskService(cfg, container.taskRepo, container.schedulerService, container.downloadService, container.fileService)
	
	return container, nil
}

// GetDownloadService 获取下载服务
func (c *ServiceContainer) GetDownloadService() contracts.DownloadService {
	return c.downloadService
}

// GetFileService 获取文件服务
func (c *ServiceContainer) GetFileService() contracts.FileService {
	return c.fileService
}

// GetTaskService 获取任务服务
func (c *ServiceContainer) GetTaskService() contracts.TaskService {
	// 如果TaskService还没有初始化，返回一个临时的空实现
	if c.taskService == nil {
		return &EmptyTaskService{}
	}
	return c.taskService
}

// GetNotificationService 获取通知服务
func (c *ServiceContainer) GetNotificationService() contracts.NotificationService {
	return c.notificationService
}

// GetHealthStatus 获取系统健康状态 - 临时实现
func (c *ServiceContainer) GetHealthStatus() *contracts.SystemHealth {
	return &contracts.SystemHealth{
		Status:    contracts.HealthStatusHealthy,
		Components: []contracts.ComponentHealth{},
	}
}

// GetConfig 获取配置
func (c *ServiceContainer) GetConfig() *config.Config {
	return c.config
}

// ========== 临时实现 ==========

// EmptyTaskService 空的任务服务实现 - 避免nil指针错误
type EmptyTaskService struct{}

func (e *EmptyTaskService) CreateTask(ctx context.Context, req contracts.TaskRequest) (*contracts.TaskResponse, error) {
	return nil, fmt.Errorf("task service not available")
}

func (e *EmptyTaskService) GetTask(ctx context.Context, id string) (*contracts.TaskResponse, error) {
	return nil, fmt.Errorf("task service not available")
}

func (e *EmptyTaskService) UpdateTask(ctx context.Context, id string, req contracts.TaskUpdateRequest) (*contracts.TaskResponse, error) {
	return nil, fmt.Errorf("task service not available")
}

func (e *EmptyTaskService) DeleteTask(ctx context.Context, id string) error {
	return fmt.Errorf("task service not available")
}

func (e *EmptyTaskService) ListTasks(ctx context.Context, req contracts.TaskListRequest) (*contracts.TaskListResponse, error) {
	return &contracts.TaskListResponse{
		Tasks:      []contracts.TaskResponse{},
		TotalCount: 0,
		Summary:    contracts.TaskSummary{},
	}, nil
}

func (e *EmptyTaskService) EnableTask(ctx context.Context, id string) error {
	return fmt.Errorf("task service not available")
}

func (e *EmptyTaskService) DisableTask(ctx context.Context, id string) error {
	return fmt.Errorf("task service not available")
}

func (e *EmptyTaskService) RunTaskNow(ctx context.Context, req contracts.TaskRunRequest) (*contracts.TaskRunResponse, error) {
	return nil, fmt.Errorf("task service not available")
}

func (e *EmptyTaskService) StopTask(ctx context.Context, id string) error {
	return fmt.Errorf("task service not available")
}

func (e *EmptyTaskService) PreviewTask(ctx context.Context, req contracts.TaskPreviewRequest) (*contracts.TaskPreviewResponse, error) {
	return nil, fmt.Errorf("task service not available")
}

func (e *EmptyTaskService) CreateQuickTask(ctx context.Context, req contracts.QuickTaskRequest) (*contracts.TaskResponse, error) {
	return nil, fmt.Errorf("task service not available")
}

func (e *EmptyTaskService) GetUserTasks(ctx context.Context, userID int64) (*contracts.TaskListResponse, error) {
	return &contracts.TaskListResponse{
		Tasks:      []contracts.TaskResponse{},
		TotalCount: 0,
		Summary:    contracts.TaskSummary{},
	}, nil
}

func (e *EmptyTaskService) GetTaskStatistics(ctx context.Context) (map[string]interface{}, error) {
	return map[string]interface{}{
		"message": "task service not available",
	}, nil
}

func (e *EmptyTaskService) GetSchedulerStatus(ctx context.Context) (map[string]interface{}, error) {
	return map[string]interface{}{
		"status":  "disabled",
		"message": "task service not available",
	}, nil
}