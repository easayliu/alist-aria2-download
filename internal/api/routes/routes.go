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

// SetupRoutes 设置路由 - 简单直接的处理方式
func (rc *RoutesConfig) SetupRoutes(router *gin.Engine) {
	// API 路由组
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

		// 文件管理相关路由
		files := api.Group("/files")
		{
			files.GET("/yesterday", handlers.GetYesterdayFiles)
			files.POST("/yesterday/download", handlers.DownloadYesterdayFiles)
			files.POST("/download", handlers.DownloadFilesFromPath)
			files.POST("/list", handlers.ListFilesHandler)
			files.POST("/manual-download", handlers.ManualDownloadFiles)
		}

		// 定时任务相关路由
		tasks := api.Group("/tasks")
		{
			tasks.POST("/", handlers.CreateTask)
			tasks.GET("/", handlers.ListTasks)
			tasks.GET("/:id", handlers.GetTask)
			tasks.PUT("/:id", handlers.UpdateTask)
			tasks.DELETE("/:id", handlers.DeleteTask)
			tasks.POST("/:id/run", handlers.RunTaskNow)
			tasks.GET("/:id/preview", handlers.PreviewTask)
			tasks.POST("/:id/toggle", handlers.ToggleTask)
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

		// 文件管理相关路由
		files := api.Group("/files")
		{
			files.GET("/yesterday", handlers.GetYesterdayFiles)
			files.POST("/yesterday/download", handlers.DownloadYesterdayFiles)
			files.POST("/download", handlers.DownloadFilesFromPath)
			files.POST("/list", handlers.ListFilesHandler)
			files.POST("/manual-download", handlers.ManualDownloadFiles)
		}

		// 定时任务相关路由
		tasks := api.Group("/tasks")
		{
			tasks.POST("/", handlers.CreateTask)
			tasks.GET("/", handlers.ListTasks)
			tasks.GET("/:id", handlers.GetTask)
			tasks.PUT("/:id", handlers.UpdateTask)
			tasks.DELETE("/:id", handlers.DeleteTask)
			tasks.POST("/:id/run", handlers.RunTaskNow)
			tasks.GET("/:id/preview", handlers.PreviewTask)
			tasks.POST("/:id/toggle", handlers.ToggleTask)
		}

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