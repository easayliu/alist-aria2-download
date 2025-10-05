package routes

import (
	"github.com/easayliu/alist-aria2-download/internal/interfaces/http/handlers"
	"github.com/easayliu/alist-aria2-download/internal/interfaces/telegram"
	"github.com/easayliu/alist-aria2-download/internal/interfaces/http/middleware"
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

// SetupRoutes 设置路由 - 简单直接的处理方式
func (rc *RoutesConfig) SetupRoutes(router *gin.Engine) {
	// 创建TaskHandler实例
	taskHandler := handlers.NewTaskHandler(rc.container)

	// API 路由组
	api := router.Group("/api/v1")
	{
		// 健康检查
		api.GET("/health", handlers.HealthCheck)

		// TODO: 下载相关路由 - 需要重构为使用新架构
		// downloads := api.Group("/downloads")
		// {
		// 	downloads.POST("/", handlers.CreateDownload)
		// 	downloads.GET("/", handlers.ListDownloads)
		// 	downloads.GET("/:id", handlers.GetDownload)
		// 	downloads.DELETE("/:id", handlers.DeleteDownload)
		// 	downloads.POST("/:id/pause", handlers.PauseDownload)
		// 	downloads.POST("/:id/resume", handlers.ResumeDownload)
		// }

		// TODO: Alist相关路由 - 需要重构为使用新架构
		// alist := api.Group("/alist")
		// {
		// 	alist.GET("/files", handlers.ListFiles)
		// 	alist.GET("/file", handlers.GetFileInfo)
		// 	alist.POST("/login", handlers.AlistLogin)
		// }

		// 文件管理相关路由
		fileHandler := handlers.NewFileHandler(rc.container)
		files := api.Group("/files")
		{
			files.GET("/yesterday", fileHandler.GetYesterdayFiles)
			files.POST("/yesterday/download", fileHandler.DownloadYesterdayFiles)
			files.POST("/download", fileHandler.DownloadFilesFromPath)
			files.POST("/list", fileHandler.ListFilesHandler)
			files.POST("/manual-download", fileHandler.ManualDownloadFiles)
		}

		// 定时任务相关路由 - 使用TaskHandler实例方法
		tasks := api.Group("/tasks")
		{
			tasks.POST("/", taskHandler.CreateTask)
			tasks.GET("/", taskHandler.ListTasks)
			tasks.GET("/:id", taskHandler.GetTask)
			tasks.PUT("/:id", taskHandler.UpdateTask)
			tasks.DELETE("/:id", taskHandler.DeleteTask)
			tasks.POST("/:id/run", taskHandler.RunTaskNow)
			tasks.GET("/:id/preview", taskHandler.PreviewTask)
			tasks.POST("/:id/toggle", taskHandler.EnableTask) // 注意: ToggleTask在task.go中是EnableTask
		}
	}
}



// SetupRoutes 设置路由 - 重构前的简单实现
func SetupRoutes(cfg *config.Config, notificationService *services.NotificationService, fileService *services.FileService) (*gin.Engine, *telegram.TelegramHandler, *services.SchedulerService) {
	router := gin.Default()

	// 初始化任务仓库和调度服务
	taskRepo, _ := repository.NewTaskRepository("./data")
	downloadService := services.NewDownloadService(cfg)
	schedulerService := services.NewSchedulerService(taskRepo, fileService, notificationService, downloadService)

	// 设置中间件，将服务实例添加到上下文
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

	// API路由组
	api := router.Group("/api/v1")
	{
		// 健康检查
		api.GET("/health", handlers.HealthCheck)

		// 下载相关路由
		downloads := api.Group("/downloads")
		{
			downloads.POST("/", handlers.CreateDownload)
			downloads.GET("/", handlers.ListDownloads)
			downloads.GET("/:id", handlers.GetDownload)
			downloads.DELETE("/:id", handlers.DeleteDownload)
			downloads.POST("/:id/pause", handlers.PauseDownload)
			downloads.POST("/:id/resume", handlers.ResumeDownload)
		}

		// Alist相关路由
		alist := api.Group("/alist")
		{
			alist.GET("/files", handlers.ListFiles)
			alist.GET("/file", handlers.GetFileInfo)
			alist.POST("/login", handlers.AlistLogin)
		}

		// 文件管理相关路由 - 已废弃,需要使用新的SetupRoutesWithContainer
		// files := api.Group("/files")
		// {
		// 	files.GET("/yesterday", handlers.GetYesterdayFiles)
		// 	files.POST("/yesterday/download", handlers.DownloadYesterdayFiles)
		// 	files.POST("/download", handlers.DownloadFilesFromPath)
		// 	files.POST("/list", handlers.ListFilesHandler)
		// 	files.POST("/manual-download", handlers.ManualDownloadFiles)
		// }

		// 定时任务相关路由 - 旧版本已废弃
		// 请使用新的RoutesConfig.SetupRoutes方法设置路由

		// Telegram管理路由
		if cfg.Telegram.Enabled {
			telegram := api.Group("/telegram")
			{
				telegram.POST("/notify", func(c *gin.Context) {
					// TODO: 手动发送通知接口
					c.JSON(200, gin.H{"message": "Notification sent"})
				})
			}
		}
	}

	return router, telegramHandler, schedulerService
}

// SetupRoutesWithContainer 使用ServiceContainer设置路由 - 新架构
func SetupRoutesWithContainer(cfg *config.Config, container *services.ServiceContainer) (*gin.Engine, *telegram.TelegramHandler) {
	router := gin.Default()

	// 全局中间件
	router.Use(middleware.CORSMiddleware())
	router.Use(middleware.LoggerMiddleware())
	router.Use(middleware.ContainerMiddleware(container))

	// Swagger文档路由
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// 初始化Telegram Handler
	var telegramHandler *telegram.TelegramHandler
	if cfg.Telegram.Enabled {
		// 从容器获取服务
		notificationSvc := container.GetNotificationService()
		fileService := container.GetFileService()
		schedulerService := container.GetSchedulerService()

		// 类型断言为具体类型(因为TelegramHandler构造函数需要具体类型)
		notificationAppSvc, ok1 := notificationSvc.(*services.NotificationService)
		fileAppSvc, ok2 := fileService.(*services.FileService)

		if ok1 && ok2 {
			telegramHandler = telegram.NewTelegramHandler(
				cfg,
				notificationAppSvc,
				fileAppSvc,
				schedulerService,
			)

			// 注册Webhook路由
			if cfg.Telegram.Webhook.Enabled {
				router.POST("/telegram/webhook", telegramHandler.Webhook)
			}
		}
	}

	// 创建路由配置并设置路由
	routesConfig := NewRoutesConfig(container)
	routesConfig.SetupRoutes(router)

	return router, telegramHandler
}