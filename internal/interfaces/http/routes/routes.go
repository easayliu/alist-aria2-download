package routes

import (
	"github.com/easayliu/alist-aria2-download/internal/application/services"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/config"
	telegramInfra "github.com/easayliu/alist-aria2-download/internal/infrastructure/telegram"
	"github.com/easayliu/alist-aria2-download/internal/interfaces/http/handlers"
	"github.com/easayliu/alist-aria2-download/internal/interfaces/http/middleware"
	"github.com/easayliu/alist-aria2-download/internal/interfaces/telegram"
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

func (rc *RoutesConfig) SetupRoutes(router *gin.Engine) {
	downloadHandler := handlers.NewDownloadHandler(rc.container)
	fileHandler := handlers.NewFileHandler(rc.container)
	taskHandler := handlers.NewTaskHandler(rc.container)
	alistHandler := handlers.NewAlistHandler(rc.container)
	llmHandler := handlers.NewLLMHandler(rc.container)

	router.GET("/health", handlers.HealthCheck)

	downloads := router.Group("/downloads")
	{
		downloads.POST("/", downloadHandler.CreateDownload)
		downloads.GET("/", downloadHandler.ListDownloads)
		downloads.GET("/:id", downloadHandler.GetDownload)
		downloads.DELETE("/:id", downloadHandler.DeleteDownload)
		downloads.POST("/:id/pause", downloadHandler.PauseDownload)
		downloads.POST("/:id/resume", downloadHandler.ResumeDownload)
		downloads.POST("/batch", downloadHandler.CreateBatchDownload)
		downloads.POST("/pause-all", downloadHandler.PauseAllDownloads)
		downloads.POST("/resume-all", downloadHandler.ResumeAllDownloads)
		downloads.GET("/statistics", downloadHandler.GetDownloadStatistics)
		downloads.GET("/system-status", downloadHandler.GetSystemStatus)
	}

	alist := router.Group("/alist")
	{
		alist.GET("/files", alistHandler.ListFiles)
		alist.GET("/file", alistHandler.GetFileInfo)
		alist.POST("/login", alistHandler.Login)
	}

	files := router.Group("/files")
	{
		files.GET("/yesterday", fileHandler.GetYesterdayFiles)
		files.POST("/yesterday/download", fileHandler.DownloadYesterdayFiles)
		files.POST("/download", fileHandler.DownloadFilesFromPath)
		files.POST("/list", fileHandler.ListFilesHandler)
		files.POST("/manual-download", fileHandler.ManualDownloadFiles)
		files.POST("/search", fileHandler.SearchFiles)
		files.POST("/time-range", fileHandler.GetFilesByTimeRange)
		files.GET("/recent", fileHandler.GetRecentFiles)
		files.POST("/classify", fileHandler.ClassifyFiles)
		files.GET("/category/:category", fileHandler.GetFilesByCategory)
		files.POST("/single-download", fileHandler.DownloadSingleFile)
		// LLM增强的文件重命名路由
		files.POST("/rename-with-llm", llmHandler.RenameWithLLM)
		files.POST("/batch-rename-with-llm", llmHandler.BatchRenameWithLLM)
		files.POST("/rename-stream", llmHandler.StreamRename)
	}

	tasks := router.Group("/tasks")
	{
		tasks.POST("/", taskHandler.CreateTask)
		tasks.GET("/", taskHandler.ListTasks)
		tasks.GET("/:id", taskHandler.GetTask)
		tasks.PUT("/:id", taskHandler.UpdateTask)
		tasks.DELETE("/:id", taskHandler.DeleteTask)
		tasks.POST("/:id/run", taskHandler.RunTaskNow)
		tasks.GET("/:id/preview", taskHandler.PreviewTask)
		tasks.POST("/:id/toggle", taskHandler.EnableTask)
		tasks.POST("/:id/disable", taskHandler.DisableTask)
		tasks.POST("/:id/stop", taskHandler.StopTask)
		tasks.GET("/user/:user-id", taskHandler.GetUserTasks)
	}

	notifications := router.Group("/notifications")
	{
		notificationHandler := handlers.NewNotificationHandler(rc.container)
		notifications.POST("/send", notificationHandler.SendNotification)
		notifications.POST("/batch", notificationHandler.SendBatchNotifications)
		notifications.GET("/history", notificationHandler.GetNotificationHistory)
		notifications.GET("/stats", notificationHandler.GetNotificationStats)
		notifications.POST("/download-complete", notificationHandler.NotifyDownloadComplete)
		notifications.POST("/download-failed", notificationHandler.NotifyDownloadFailed)
		notifications.POST("/task-complete", notificationHandler.NotifyTaskComplete)
		notifications.POST("/task-failed", notificationHandler.NotifyTaskFailed)
		notifications.POST("/system-event", notificationHandler.NotifySystemEvent)
		notifications.GET("/config", notificationHandler.GetNotificationConfig)
	}

	// LLM路由组
	llm := router.Group("/llm")
	{
		llm.POST("/generate", llmHandler.Generate)
		llm.GET("/stream", llmHandler.Stream)
	}
}

// SetupRoutesWithContainer 使用ServiceContainer设置路由 - 新架构
func SetupRoutesWithContainer(cfg *config.Config, container *services.ServiceContainer) (*gin.Engine, *telegram.TelegramHandler, *telegramInfra.Client) {
	router := gin.Default()

	// 全局中间件
	router.Use(middleware.CORSMiddleware())
	router.Use(middleware.LoggerMiddleware())
	router.Use(middleware.ContainerMiddleware(container))

	// Swagger文档路由
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// 初始化Telegram Handler
	var telegramHandler *telegram.TelegramHandler
	var telegramClient *telegramInfra.Client
	if cfg.Telegram.Enabled {
		// 创建单例 Telegram Client
		telegramClient = telegramInfra.NewClient(&cfg.Telegram)
		container.SetTelegramClient(telegramClient)

		// 从容器获取服务
		notificationSvc := container.GetNotificationService()
		fileService := container.GetFileService()
		schedulerService := container.GetSchedulerService()

		// 类型断言为具体类型(因为TelegramHandler构造函数需要具体类型)
		notificationAppSvc, ok1 := notificationSvc.(*services.NotificationService)
		fileAppSvc, ok2 := fileService.(*services.FileService)

		if ok1 && ok2 {
			notificationAppSvc.SetTelegramClient(telegramClient)

			telegramHandler = telegram.NewTelegramHandler(
				cfg,
				notificationAppSvc,
				fileAppSvc,
				schedulerService,
				container,
				telegramClient,
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

	return router, telegramHandler, telegramClient
}
