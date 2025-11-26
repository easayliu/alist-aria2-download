package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/easayliu/alist-aria2-download/docs"
	"github.com/easayliu/alist-aria2-download/internal/application/services"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/config"
	"github.com/easayliu/alist-aria2-download/internal/interfaces/http/routes"
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

	// 初始化日志
	if err := logger.Init(logger.Options{
		Level:     cfg.Log.Level,
		Output:    cfg.Log.Output,
		Format:    cfg.Log.Format,
		FilePath:  cfg.Log.FilePath,
		Colorize:  cfg.Log.Colorize,
		AddSource: cfg.Log.AddSource,
	}); err != nil {
		log.Fatal("Failed to initialize logger:", err)
	}

	// 设置Gin模式
	if cfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	// 初始化服务容器
	container, err := services.NewServiceContainer(cfg)
	if err != nil {
		log.Fatal("Failed to initialize service container:", err)
	}

	// 初始化路由
	router, telegramHandler, telegramClient := routes.SetupRoutesWithContainer(cfg, container)

	// 配置 Telegram Webhook
	if cfg.Telegram.Enabled && telegramClient != nil {
		if cfg.Telegram.Webhook.Enabled {
			// Webhook 模式：自动设置 webhook
			if err := telegramClient.SetWebhook(cfg.Telegram.Webhook.URL); err != nil {
				logger.Error("Failed to set telegram webhook", "error", err)
			} else {
				logger.Info("Telegram webhook mode enabled")
			}
		} else {
			// Polling 模式：确保删除 webhook
			if err := telegramClient.DeleteWebhook(); err != nil {
				logger.Warn("Failed to delete telegram webhook", "error", err)
			}
			// 启动 Polling
			if telegramHandler != nil {
				telegramHandler.StartPolling()
				logger.Info("Telegram polling mode enabled")
			}
		}
	}

	// 设置信号处理
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// 启动服务器
	go func() {
		addr := cfg.Server.Host + ":" + cfg.Server.Port
		logger.Info("Starting server", "address", addr)
		if err := router.Run(addr); err != nil {
			log.Fatal("Failed to start server:", err)
		}
	}()

	// 等待退出信号
	<-quit
	logger.Info("Shutting down server...")

	// 停止Telegram轮询
	if telegramHandler != nil {
		telegramHandler.StopPolling()
		logger.Info("Telegram polling stopped")
	}

	logger.Info("Server stopped")
}
