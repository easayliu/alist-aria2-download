package services

import (
	"fmt"

	"github.com/easayliu/alist-aria2-download/internal/application/contracts"
	"github.com/easayliu/alist-aria2-download/internal/application/services/download"
	"github.com/easayliu/alist-aria2-download/internal/application/services/file"
	"github.com/easayliu/alist-aria2-download/internal/application/services/notification"
	"github.com/easayliu/alist-aria2-download/internal/application/services/task"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/config"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/repository"
)

// 向后兼容的类型别名 - 用于渐进式迁移
type (
	YesterdayFileInfo = file.YesterdayFileInfo
	SchedulerService  = task.SchedulerService
	NotificationService = notification.AppNotificationService
	FileService       = file.AppFileService
)

// 向后兼容的构造函数
func NewFileService(client interface{}) *file.AppFileService {
	// 这是一个临时的兼容函数,新代码应该使用ServiceContainer
	cfg, _ := config.LoadConfig()
	svc := file.NewAppFileService(cfg, nil)
	if appSvc, ok := svc.(*file.AppFileService); ok {
		return appSvc
	}
	// 理论上不会到这里,但保险起见返回一个新的实例
	return &file.AppFileService{}
}

func NewDownloadService(cfg *config.Config) contracts.DownloadService {
	return download.NewAppDownloadService(cfg, nil)
}

func NewNotificationService(cfg *config.Config) *notification.AppNotificationService {
	svc := notification.NewAppNotificationService(cfg)
	if appSvc, ok := svc.(*notification.AppNotificationService); ok {
		return appSvc
	}
	return &notification.AppNotificationService{}
}

func NewSchedulerService(taskRepo *repository.TaskRepository, fileService contracts.FileService, notificationService contracts.NotificationService, downloadService contracts.DownloadService) *task.SchedulerService {
	return task.NewSchedulerService(taskRepo, fileService, notificationService, downloadService)
}

// ServiceContainer 应用服务容器 - 实现依赖注入
type ServiceContainer struct {
	config   *config.Config

	// 应用服务缓存
	downloadService     contracts.DownloadService
	fileService        contracts.FileService
	taskService        contracts.TaskService
	notificationService contracts.NotificationService
	schedulerService    *task.SchedulerService  // 新增: 调度服务

	// 基础设施服务（非contracts）
	taskRepo        *repository.TaskRepository
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
	container.notificationService = notification.NewAppNotificationService(cfg)
	container.fileService = file.NewAppFileService(cfg, nil) // 暂时传nil，稍后会设置downloadService
	container.downloadService = download.NewAppDownloadService(cfg, container.fileService)

	// 更新fileService的downloadService依赖
	// 注意：由于字段私有，需要添加setter方法
	if appFileService, ok := container.fileService.(*file.AppFileService); ok {
		appFileService.SetDownloadService(container.downloadService)
	}

	// 3. 初始化TaskService和SchedulerService
	// 创建SchedulerService
	container.schedulerService = task.NewSchedulerService(
		container.taskRepo,
		container.fileService,
		container.notificationService,
		container.downloadService,
	)

	// 创建TaskService
	container.taskService = task.NewAppTaskService(
		cfg,
		container.taskRepo,
		container.schedulerService,
		container.downloadService,
		container.fileService,
	)

	// 启动调度器
	if err := container.schedulerService.Start(); err != nil {
		return nil, fmt.Errorf("failed to start scheduler: %w", err)
	}

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

// GetSchedulerService 获取调度服务
func (c *ServiceContainer) GetSchedulerService() *task.SchedulerService {
	return c.schedulerService
}