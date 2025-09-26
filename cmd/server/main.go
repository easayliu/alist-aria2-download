package main

import (
	"log"

	_ "github.com/easayliu/alist-aria2-download/docs"
	"github.com/easayliu/alist-aria2-download/internal/api/routes"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/config"
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

	// 初始化路由
	router := routes.SetupRoutes()

	// 启动服务器
	if err := router.Run(":" + cfg.Server.Port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}