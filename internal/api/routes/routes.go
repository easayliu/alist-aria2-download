package routes

import (
	"github.com/easayliu/alist-aria2-download/internal/api/handlers"
	"github.com/easayliu/alist-aria2-download/internal/api/middleware"
	"github.com/gin-gonic/gin"
	ginSwagger "github.com/swaggo/gin-swagger"
	swaggerFiles "github.com/swaggo/files"
)

func SetupRoutes() *gin.Engine {
	router := gin.Default()

	// 全局中间件
	router.Use(middleware.CORSMiddleware())
	router.Use(middleware.LoggerMiddleware())

	// Swagger文档路由
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

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
		}
	}

	return router
}