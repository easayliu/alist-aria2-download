package handlers

import (
	"github.com/easayliu/alist-aria2-download/internal/application/services"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/config"
	"github.com/gin-gonic/gin"
)

// GetContainer 从gin.Context中获取ServiceContainer
// 这个方法假设Container已经通过中间件注入到Context中
func GetContainer(c *gin.Context) *services.ServiceContainer {
	container, exists := c.Get("container")
	if !exists {
		panic("ServiceContainer not found in context. Did you forget to use ContainerMiddleware?")
	}
	return container.(*services.ServiceContainer)
}

// GetConfig 从gin.Context中获取Config
func GetConfig(c *gin.Context) *config.Config {
	container := GetContainer(c)
	return container.GetConfig()
}
