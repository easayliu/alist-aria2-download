package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HealthCheck 健康检查
// @Summary 健康检查
// @Description 检查服务健康状态
// @Tags 健康检查
// @Produce json
// @Success 200 {object} map[string]string
// @Router /health [get]
func HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "Alist Aria2 Download service is running",
	})
}
