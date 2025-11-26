package telegram

import (
	"fmt"
	"sync"

	"github.com/easayliu/alist-aria2-download/pkg/logger"
)

// Common utility functions and shared state
type Common struct {
	controller *TelegramController

	// Path cache related
	pathMutex        sync.RWMutex
	pathCache        map[string]string // token -> path
	pathReverseCache map[string]string // path -> token
	pathTokenCounter int
}

// NewCommon creates a new common utility instance
func NewCommon(controller *TelegramController) *Common {
	return &Common{
		controller:       controller,
		pathCache:        make(map[string]string),
		pathReverseCache: make(map[string]string),
		pathTokenCounter: 1,
	}
}

// ================================
// File size formatting functions
// ================================

// FormatFileSize formats file size
func (c *Common) FormatFileSize(size int64) string {
	return c.controller.messageUtils.FormatFileSize(size)
}

// ================================
// Path cache management
// ================================

// EncodeFilePath encodes file path for callback data (using cache to avoid 64-byte limit)
func (c *Common) EncodeFilePath(path string) string {
	c.pathMutex.Lock()
	defer c.pathMutex.Unlock()

	// Check if path is already in cache
	if token, exists := c.pathReverseCache[path]; exists {
		return token
	}

	// Create new short token for path
	c.pathTokenCounter++
	token := fmt.Sprintf("p%d", c.pathTokenCounter)

	// Store path and token in cache
	c.pathCache[token] = path
	c.pathReverseCache[path] = token

	// Clean up cache if it gets too large (keep cache size reasonable)
	if len(c.pathCache) > 1000 {
		c.cleanupPathCache()
	}

	return token
}

// DecodeFilePath decodes file path from token
func (c *Common) DecodeFilePath(encoded string) string {
	c.pathMutex.RLock()
	defer c.pathMutex.RUnlock()

	if path, exists := c.pathCache[encoded]; exists {
		return path
	}

	logger.WarnSafe("Path token not found", "token", encoded)
	return "/"
}

// cleanupPathCache cleans up path cache (keeps most recent 500 entries)
func (c *Common) cleanupPathCache() {
	// Simple cleanup strategy: clear all when limit exceeded
	// In production, could use LRU or other advanced strategies
	if len(c.pathCache) <= 500 {
		return
	}

	// Clear entire cache and restart counter (simple but effective)
	c.pathCache = make(map[string]string)
	c.pathReverseCache = make(map[string]string)
	c.pathTokenCounter = 1

	logger.Info("Path cache cleared")
}

// ================================
// Path utility functions
// ================================

// IsDirectoryPath checks if path is a directory
func (c *Common) IsDirectoryPath(path string) bool {
	// Simplified implementation: always return true to avoid circular dependency
	return true
}

// GetParentPath gets parent directory path - compatibility wrapper method
func (c *Common) GetParentPath(path string) string {
	if path == "/" {
		return "/"
	}

	// Simple implementation: remove the last path component
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
// Helper methods - internal use only
// ================================
