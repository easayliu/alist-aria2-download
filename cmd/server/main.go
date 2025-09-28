package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/easayliu/alist-aria2-download/docs"
	"github.com/easayliu/alist-aria2-download/internal/api/routes"
	"github.com/easayliu/alist-aria2-download/internal/application/services"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/alist"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/config"
	"github.com/easayliu/alist-aria2-download/pkg/logger"
	"github.com/gin-gonic/gin"
)

// @title Alist Aria2 Download API
// @version 1.0
// @description 基于Gin框架的Alist和Aria2下载管理服务
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.url http://www.swagger.io/support
// @contact.email support@swagger.io

// @license.name MIT
// @license.url https://github.com/easayliu/alist-aria2-download/blob/main/LICENSE

// @host localhost:8081
// @BasePath /api/v1
// @schemes http https
func main() {
	// 加载配置
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal("Failed to load config:", err)
	}

	// 设置Gin模式
	if cfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	// 初始化服务
	notificationService := services.NewNotificationService(cfg)

	// 初始化Alist客户端和文件服务
	alistClient := alist.NewClientWithQPS(cfg.Alist.BaseURL, cfg.Alist.Username, cfg.Alist.Password, cfg.Alist.QPS)
	fileService := services.NewFileService(alistClient)

	// 初始化路由
	router, telegramHandler, schedulerService := routes.SetupRoutes(cfg, notificationService, fileService)

	// 启动Telegram轮询（如果启用且未使用webhook）
	if cfg.Telegram.Enabled && !cfg.Telegram.Webhook.Enabled {
		telegramHandler.StartPolling()
		logger.Info("Telegram polling started")
	}

	// 启动调度服务
	if err := schedulerService.Start(); err != nil {
		logger.Error("Failed to start scheduler service:", err)
	} else {
		logger.Info("Scheduler service started")
	}

	// 设置信号处理
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// 启动服务器
	go func() {
		logger.Info("Starting server on port", cfg.Server.Port)
		if err := router.Run(":" + cfg.Server.Port); err != nil {
			log.Fatal("Failed to start server:", err)
		}
	}()

	// 等待退出信号
	<-quit
	logger.Info("Shutting down server...")

	// 停止Telegram轮询
	if telegramHandler != nil {
		telegramHandler.StopPolling()
	}

	// 停止调度服务
	if schedulerService != nil {
		schedulerService.Stop()
	}

	logger.Info("Server stopped")
}
