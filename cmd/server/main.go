package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/easayliu/alist-aria2-download/docs"
	"github.com/easayliu/alist-aria2-download/internal/interfaces/http/routes"
	"github.com/easayliu/alist-aria2-download/internal/application/services"
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
	router, telegramHandler := routes.SetupRoutesWithContainer(cfg, container)

	// 启动Telegram轮询模式
	if cfg.Telegram.Enabled && !cfg.Telegram.Webhook.Enabled && telegramHandler != nil {
		telegramHandler.StartPolling()
		logger.Info("Telegram polling started successfully")
	}

	// 设置信号处理
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// 启动服务器
	go func() {
		logger.Info("Starting server on port", "port", cfg.Server.Port)
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
		logger.Info("Telegram polling stopped")
	}

	logger.Info("Server stopped")
}
