package container

import (
	"fmt"
	"sync"

	"github.com/easayliu/alist-aria2-download/internal/application/contracts"
	"github.com/easayliu/alist-aria2-download/internal/application/services"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/config"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/repository"
	"github.com/easayliu/alist-aria2-download/pkg/logger"
)

// ServiceContainer 服务容器 - 实现依赖注入
type ServiceContainer struct {
	config              *config.Config
	taskRepo           *repository.TaskRepository
	schedulerService   *services.SchedulerService
	notificationService *services.NotificationService
	
	// 应用层服务实例
	downloadService contracts.DownloadService
	taskService     contracts.TaskService
	fileService     contracts.FileService
	
	// 单例模式锁
	once sync.Once
}

// NewServiceContainer 创建服务容器
func NewServiceContainer(cfg *config.Config) *ServiceContainer {
	return &ServiceContainer{
		config: cfg,
	}
}

// GetDownloadService 获取下载服务实例
func (c *ServiceContainer) GetDownloadService() contracts.DownloadService {
	c.once.Do(c.initServices)
	return c.downloadService
}

// GetTaskService 获取任务服务实例
func (c *ServiceContainer) GetTaskService() contracts.TaskService {
	c.once.Do(c.initServices)
	return c.taskService
}

// GetFileService 获取文件服务实例
func (c *ServiceContainer) GetFileService() contracts.FileService {
	c.once.Do(c.initServices)
	return c.fileService
}

// GetLegacySchedulerService 获取旧版调度服务（向后兼容）
func (c *ServiceContainer) GetLegacySchedulerService() *services.SchedulerService {
	c.once.Do(c.initServices)
	return c.schedulerService
}

// GetLegacyNotificationService 获取旧版通知服务（向后兼容）
func (c *ServiceContainer) GetLegacyNotificationService() *services.NotificationService {
	c.once.Do(c.initServices)
	return c.notificationService
}

// GetTaskRepository 获取任务仓库
func (c *ServiceContainer) GetTaskRepository() *repository.TaskRepository {
	c.once.Do(c.initServices)
	return c.taskRepo
}

// initServices 初始化所有服务（单例模式）
func (c *ServiceContainer) initServices() {
	logger.Info("Initializing service container")

	// 1. 初始化基础设施层
	var err error
	c.taskRepo, err = repository.NewTaskRepository("./data")
	if err != nil {
		logger.Error("Failed to initialize task repository:", err)
		return
	}
	c.notificationService = services.NewNotificationService(c.config)
	c.schedulerService = services.NewSchedulerService(c.taskRepo, nil, c.notificationService, nil)

	// 2. 初始化应用层服务 - 注意依赖关系
	// 首先初始化文件服务（最少依赖）
	c.downloadService = services.NewAppDownloadService(c.config, nil) // 临时传nil，稍后设置
	c.fileService = services.NewAppFileService(c.config, c.downloadService)
	
	// 重新创建下载服务，传入文件服务
	c.downloadService = services.NewAppDownloadService(c.config, c.fileService)
	
	// 最后初始化任务服务（依赖最多）
	c.taskService = services.NewAppTaskService(
		c.config,
		c.taskRepo,
		c.schedulerService,
		c.downloadService,
		c.fileService,
	)

	// 3. 更新旧版服务的依赖（向后兼容） - 方法不存在，跳过

	logger.Info("Service container initialized successfully")
}

// Shutdown 关闭服务容器
func (c *ServiceContainer) Shutdown() {
	logger.Info("Shutting down service container")
	
	if c.schedulerService != nil {
		c.schedulerService.Stop()
	}
	
	logger.Info("Service container shutdown completed")
}

// ValidateServices 验证服务配置
func (c *ServiceContainer) ValidateServices() error {
	c.once.Do(c.initServices)
	
	// 验证关键服务是否正确初始化
	if c.downloadService == nil {
		return fmt.Errorf("download service not initialized")
	}
	if c.fileService == nil {
		return fmt.Errorf("file service not initialized")
	}
	if c.taskService == nil {
		return fmt.Errorf("task service not initialized")
	}
	
	logger.Info("Service validation completed successfully")
	return nil
}

// GetServiceHealth 获取服务健康状态
func (c *ServiceContainer) GetServiceHealth() map[string]interface{} {
	c.once.Do(c.initServices)
	
	health := map[string]interface{}{
		"container": "healthy",
		"services": map[string]interface{}{
			"download_service": c.getServiceStatus(c.downloadService != nil),
			"file_service":     c.getServiceStatus(c.fileService != nil),
			"task_service":     c.getServiceStatus(c.taskService != nil),
			"scheduler_service": c.getServiceStatus(c.schedulerService != nil),
			"notification_service": c.getServiceStatus(c.notificationService != nil),
		},
	}
	
	return health
}

// getServiceStatus 获取服务状态
func (c *ServiceContainer) getServiceStatus(initialized bool) string {
	if initialized {
		return "healthy"
	}
	return "unhealthy"
}