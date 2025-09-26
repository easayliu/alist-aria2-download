package utils

import (
	"github.com/gin-gonic/gin"
)

// Success 成功响应
func Success(c *gin.Context, data interface{}) {
	c.JSON(200, gin.H{
		"code": 200,
		"data": data,
	})
}

// ErrorWithStatus 错误响应
func ErrorWithStatus(c *gin.Context, httpStatus int, code int, message string) {
	c.JSON(httpStatus, gin.H{
		"code":    code,
		"message": message,
	})
}

// Error 一般错误响应
func Error(c *gin.Context, message string) {
	ErrorWithStatus(c, 500, 500, message)
}
