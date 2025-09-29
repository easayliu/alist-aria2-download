package routes

import (
	"github.com/easayliu/alist-aria2-download/internal/api/handlers"
	"github.com/easayliu/alist-aria2-download/internal/api/handlers/telegram"
	"github.com/easayliu/alist-aria2-download/internal/api/middleware"
	"github.com/easayliu/alist-aria2-download/internal/application/services"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/config"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/repository"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// RoutesConfig 路由配置 - API First架构
type RoutesConfig struct {
	container *services.ServiceContainer
}

// NewRoutesConfig 创建路由配置
func NewRoutesConfig(container *services.ServiceContainer) *RoutesConfig {
	return &RoutesConfig{
		container: container,
	}
}

// SetupRoutes 设置路由 - 统一业务服务架构
func (rc *RoutesConfig) SetupRoutes(router *gin.Engine) {
	// 创建处理器实例
	downloadHandler := handlers.NewDownloadHandler(rc.container)
	taskHandler := handlers.NewTaskHandler(rc.container)
	
	// API 路由组
	api := router.Group("/api")
	{
		// 下载管理路由
		downloads := api.Group("/downloads")
		{
			// 基础下载操作
			downloads.POST("", downloadHandler.CreateDownload)
			downloads.GET("", downloadHandler.ListDownloads)
			downloads.GET("/:id", downloadHandler.GetDownload)
			downloads.DELETE("/:id", downloadHandler.DeleteDownload)
			
			// 下载控制
			downloads.POST("/:id/pause", downloadHandler.PauseDownload)
			downloads.POST("/:id/resume", downloadHandler.ResumeDownload)
			
			// 批量操作
			downloads.POST("/batch", downloadHandler.CreateBatchDownload)
			
			// 系统状态
			downloads.GET("/status", downloadHandler.GetSystemStatus)
			downloads.GET("/statistics", downloadHandler.GetStatistics)
		}

		// 任务管理路由
		tasks := api.Group("/tasks")
		{
			// 基础任务操作
			tasks.POST("", taskHandler.CreateTask)
			tasks.GET("", taskHandler.ListTasks)
			tasks.GET("/:id", taskHandler.GetTask)
			tasks.PUT("/:id", taskHandler.UpdateTask)
			tasks.DELETE("/:id", taskHandler.DeleteTask)
			
			// 任务控制
			tasks.POST("/:id/run", taskHandler.RunTaskNow)
			tasks.GET("/:id/preview", taskHandler.PreviewTask)
			tasks.POST("/:id/enable", taskHandler.EnableTask)
			tasks.POST("/:id/disable", taskHandler.DisableTask)
			
			// 快捷任务
			tasks.POST("/quick", taskHandler.CreateQuickTask)
			
			// 系统状态
			tasks.GET("/statistics", taskHandler.GetTaskStatistics)
			tasks.GET("/scheduler/status", taskHandler.GetSchedulerStatus)
		}

		// 文件管理路由（如果需要）
		files := api.Group("/files")
		{
			// TODO: 实现文件管理相关路由
			_ = files // 避免未使用变量警告
		}

		// 通知管理路由（如果需要）
		notifications := api.Group("/notifications")
		{
			// TODO: 实现通知管理相关路由
			_ = notifications // 避免未使用变量警告
		}

		// 系统健康检查
		api.GET("/health", rc.handleHealthCheck)
	}
}

// handleHealthCheck 处理健康检查请求
func (rc *RoutesConfig) handleHealthCheck(c *gin.Context) {
	health := rc.container.GetHealthStatus()
	
	// 根据健康状态设置HTTP状态码
	var statusCode int
	switch health.Status {
	case "healthy":
		statusCode = 200
	case "degraded":
		statusCode = 200 // 降级但仍可用
	case "unhealthy":
		statusCode = 503 // 服务不可用
	default:
		statusCode = 500
	}
	
	c.JSON(statusCode, health)
}

// SetupMiddlewares 设置中间件
func (rc *RoutesConfig) SetupMiddlewares(router *gin.Engine) {
	// 依赖注入中间件 - 将服务容器注入到上下文
	router.Use(func(c *gin.Context) {
		c.Set("serviceContainer", rc.container)
		c.Set("downloadService", rc.container.GetDownloadService())
		c.Set("fileService", rc.container.GetFileService())
		c.Set("taskService", rc.container.GetTaskService())
		c.Set("notificationService", rc.container.GetNotificationService())
		c.Next()
	})

	// 错误处理中间件
	router.Use(rc.errorHandlingMiddleware())

	// 请求日志中间件
	router.Use(rc.requestLoggingMiddleware())
}

// errorHandlingMiddleware 错误处理中间件
func (rc *RoutesConfig) errorHandlingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// 记录panic错误
				c.JSON(500, gin.H{
					"error": "Internal server error",
					"code":  "INTERNAL_ERROR",
				})
				c.Abort()
			}
		}()
		c.Next()
	}
}

// requestLoggingMiddleware 请求日志中间件
func (rc *RoutesConfig) requestLoggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 记录请求信息
		// TODO: 实现详细的请求日志记录
		c.Next()
	}
}

// GetContainer 获取服务容器（用于其他组件）
func (rc *RoutesConfig) GetContainer() *services.ServiceContainer {
	return rc.container
}

// ========== 向后兼容性支持 ==========

// SetupRoutes 向后兼容函数 - 保持与现有main.go的兼容性
func SetupRoutes(cfg *config.Config, notificationService *services.NotificationService, fileService *services.FileService) (*gin.Engine, *telegram.TelegramHandler, *services.SchedulerService) {
	router := gin.Default()

	// 初始化任务仓库和调度服务
	taskRepo, _ := repository.NewTaskRepository("./data")
	downloadService := services.NewDownloadService(cfg)
	schedulerService := services.NewSchedulerService(taskRepo, fileService, notificationService, downloadService)

	// 设置中间件
	router.Use(func(c *gin.Context) {
		c.Set("schedulerService", schedulerService)
		c.Set("taskRepo", taskRepo)
		c.Set("fileService", fileService)
		c.Next()
	})

	// 初始化Telegram处理器
	telegramHandler := telegram.NewTelegramHandler(cfg, notificationService, fileService, schedulerService)

	// 全局中间件
	router.Use(middleware.CORSMiddleware())
	router.Use(middleware.LoggerMiddleware())

	// Swagger文档路由
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Telegram Webhook路由
	if cfg.Telegram.Enabled && cfg.Telegram.Webhook.Enabled {
		router.POST("/telegram/webhook", telegramHandler.Webhook)
	}

	// 创建服务容器和新路由配置
	container, _ := services.NewServiceContainer(cfg)
	routesConfig := NewRoutesConfig(container)
	
	// 设置新架构的路由
	routesConfig.SetupRoutes(router)
	routesConfig.SetupMiddlewares(router)

	return router, telegramHandler, schedulerService
}