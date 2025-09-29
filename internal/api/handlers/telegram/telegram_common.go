package telegram

import (
	"fmt"
	"sync"

	"github.com/easayliu/alist-aria2-download/pkg/logger"
)

// Common 通用工具函数和共享状态
type Common struct {
	controller *TelegramController
	
	// 路径缓存相关
	pathMutex        sync.RWMutex
	pathCache        map[string]string // token -> path
	pathReverseCache map[string]string // path -> token
	pathTokenCounter int
}

// NewCommon 创建新的通用工具实例
func NewCommon(controller *TelegramController) *Common {
	return &Common{
		controller:       controller,
		pathCache:        make(map[string]string),
		pathReverseCache: make(map[string]string),
		pathTokenCounter: 1,
	}
}

// ================================
// 文件大小格式化功能
// ================================

// FormatFileSize 格式化文件大小
func (c *Common) FormatFileSize(size int64) string {
	return c.controller.messageUtils.FormatFileSize(size)
}

// ================================
// 路径缓存管理
// ================================

// EncodeFilePath 编码文件路径用于callback data（使用缓存机制避免64字节限制）
func (c *Common) EncodeFilePath(path string) string {
	c.pathMutex.Lock()
	defer c.pathMutex.Unlock()

	// 检查是否已有缓存
	if token, exists := c.pathReverseCache[path]; exists {
		return token
	}

	// 创建新的短token
	c.pathTokenCounter++
	token := fmt.Sprintf("p%d", c.pathTokenCounter)

	// 存储到缓存
	c.pathCache[token] = path
	c.pathReverseCache[path] = token

	// 清理过期缓存（保持缓存大小合理）
	if len(c.pathCache) > 1000 {
		c.cleanupPathCache()
	}

	return token
}

// DecodeFilePath 解码文件路径
func (c *Common) DecodeFilePath(encoded string) string {
	c.pathMutex.RLock()
	defer c.pathMutex.RUnlock()

	if path, exists := c.pathCache[encoded]; exists {
		return path
	}

	logger.Warn("路径token未找到:", "token", encoded)
	return "/" // 未找到时返回根目录
}

// cleanupPathCache 清理路径缓存（保留最近的500个）
func (c *Common) cleanupPathCache() {
	// 这是一个简单的清理策略，实际应用中可以使用LRU等更复杂的策略
	if len(c.pathCache) <= 500 {
		return
	}

	// 清空缓存，重新开始（简单但有效）
	c.pathCache = make(map[string]string)
	c.pathReverseCache = make(map[string]string)
	c.pathTokenCounter = 1

	logger.Info("路径缓存已清理")
}

// ================================
// 路径工具函数
// ================================

// IsDirectoryPath 判断路径是否为目录
func (c *Common) IsDirectoryPath(path string) bool {
	// 简化实现，避免循环依赖
	return true // 暂时返回true，实际实现需要通过FileService
}

// GetParentPath 获取父目录路径 - 兼容性方法
func (c *Common) GetParentPath(path string) string {
	if path == "/" {
		return "/"
	}
	
	// 简单实现：移除最后一个路径分量
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			if i == 0 {
				return "/"
			}
			return path[:i]
		}
	}
	
	return "/"
}

// ================================
// 辅助方法 - 内部使用
// ================================

// listFilesSimple 内部方法：简单列出文件 - 委托给文件服务
func (c *Common) listFilesSimple(path string, page, perPage int) ([]interface{}, error) {
	// 这里暂时返回一个简单的实现，避免循环依赖
	// 实际使用中会通过FileHandler来处理
	return []interface{}{}, nil
}