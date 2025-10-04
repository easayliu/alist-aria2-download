package middleware

import (
	"github.com/easayliu/alist-aria2-download/internal/application/services"
	"github.com/gin-gonic/gin"
)

// ContainerMiddleware 服务容器中间件
// 将ServiceContainer注入到gin.Context中,供handlers使用
// 这样避免了在每个handler中重复LoadConfig和创建各种Client
func ContainerMiddleware(container *services.ServiceContainer) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 将container注入到context
		c.Set("container", container)
		c.Next()
	}
}
