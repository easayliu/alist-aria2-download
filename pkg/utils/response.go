package utils

import (
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"
)

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Success 成功响应
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    data,
	})
}

// Error 错误响应
func Error(c *gin.Context, code int, message string) {
	c.JSON(http.StatusOK, Response{
		Code:    code,
		Message: message,
	})
}

// ErrorWithStatus 带HTTP状态码的错误响应
func ErrorWithStatus(c *gin.Context, httpStatus, code int, message string) {
	c.JSON(httpStatus, Response{
		Code:    code,
		Message: message,
	})
}

// JoinPath 连接路径
func JoinPath(paths ...string) string {
	return filepath.Join(paths...)
}

// ParseInt64 解析字符串为int64
func ParseInt64(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

// GetParentPath 获取父路径
func GetParentPath(path string) string {
	return filepath.Dir(path)
}

// GetFileName 获取文件名
func GetFileName(path string) string {
	return filepath.Base(path)
}

// FormatFileSize 格式化文件大小
func FormatFileSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return strconv.FormatInt(bytes, 10) + " B"
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return strconv.FormatFloat(float64(bytes)/float64(div), 'f', 1, 64) + " " + "KMGTPE"[exp:exp+1] + "B"
}
