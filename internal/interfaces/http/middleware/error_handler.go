package middleware

import (
	"net/http"

	"github.com/easayliu/alist-aria2-download/internal/application/contracts"
	"github.com/gin-gonic/gin"
)

// ErrorHandlerMiddleware 统一错误处理中间件
// 捕获handler中设置的错误,自动转换为合适的HTTP响应
func ErrorHandlerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// 检查是否有错误
		if len(c.Errors) > 0 {
			err := c.Errors.Last().Err

			// 根据错误类型返回不同的HTTP状态码
			if serviceErr, ok := err.(*contracts.ServiceError); ok {
				statusCode := mapErrorCodeToHTTPStatus(serviceErr.Code)
				c.JSON(statusCode, gin.H{
					"error":   serviceErr.Message,
					"code":    serviceErr.Code,
					"details": serviceErr.Details,
				})
			} else {
				// 未知错误,返回500
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": err.Error(),
					"code":  contracts.ErrorCodeInternalError,
				})
			}
		}
	}
}

// mapErrorCodeToHTTPStatus 将业务错误码映射到HTTP状态码
func mapErrorCodeToHTTPStatus(code contracts.ErrorCode) int {
	switch code {
	case contracts.ErrorCodeInvalidRequest:
		return http.StatusBadRequest
	case contracts.ErrorCodeNotFound:
		return http.StatusNotFound
	case contracts.ErrorCodeUnauthorized:
		return http.StatusUnauthorized
	case contracts.ErrorCodeForbidden:
		return http.StatusForbidden
	case contracts.ErrorCodeConflict:
		return http.StatusConflict
	case contracts.ErrorCodeServiceUnavailable:
		return http.StatusServiceUnavailable
	case contracts.ErrorCodeTimeout:
		return http.StatusRequestTimeout
	case contracts.ErrorCodeRateLimit:
		return http.StatusTooManyRequests
	case contracts.ErrorCodeQuotaExceeded:
		return http.StatusInsufficientStorage
	default:
		return http.StatusInternalServerError
	}
}

// RecoverMiddleware 恢复中间件 - 捕获panic并转换为500错误
func RecoverMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "Internal server error",
					"code":  contracts.ErrorCodeInternalError,
				})
				c.Abort()
			}
		}()
		c.Next()
	}
}
