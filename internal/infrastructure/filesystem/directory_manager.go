package filesystem

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/easayliu/alist-aria2-download/internal/infrastructure/config"
	"github.com/easayliu/alist-aria2-download/pkg/logger"
)

// DirectoryManager 目录管理服务 - 负责目录创建、权限验证和空间检查
type DirectoryManager struct {
	config         *config.Config
	dirCache       map[string]bool // 目录存在性缓存
	cacheMutex     sync.RWMutex
	autoCreate     bool
	validatePerms  bool
	checkDiskSpace bool
}

// DirectoryError 目录错误
type DirectoryError struct {
	Path   string
	Reason string
}

func (e *DirectoryError) Error() string {
	return fmt.Sprintf("目录错误: %s - %s", e.Path, e.Reason)
}

// NewDirectoryManager 创建目录管理服务
func NewDirectoryManager(cfg *config.Config) *DirectoryManager {
	// 所有功能已禁用，保留结构以兼容现有代码
	return &DirectoryManager{
		config:         cfg,
		dirCache:       make(map[string]bool),
		autoCreate:     false, // 禁用自动创建
		validatePerms:  false, // 禁用权限验证
		checkDiskSpace: false, // 禁用磁盘空间检查
	}
}

// EnsureDirectory 确保目录存在且可用
func (m *DirectoryManager) EnsureDirectory(path string) error {
	logger.Debug("Checking directory", "path", path)

	// 1. 检查缓存
	if m.isInCache(path) {
		logger.Debug("Directory found in cache", "path", path)
		return nil
	}

	// 2. 检查目录是否存在
	info, err := os.Stat(path)
	if err == nil {
		// 目录存在，验证是否为目录
		if !info.IsDir() {
			return &DirectoryError{
				Path:   path,
				Reason: "路径存在但不是目录",
			}
		}

		// 更新缓存
		m.updateCache(path, true)

		// 验证可写性（可选）
		if m.validatePerms {
			if err := m.checkWritable(path); err != nil {
				logger.Warn("Directory permission validation failed, but directory exists, continuing", "path", path, "error", err)
				// 不返回错误，允许继续使用已存在的目录
			}
		}

		logger.Debug("Directory exists", "path", path)
		return nil
	}

	// 3. 目录不存在
	if !os.IsNotExist(err) {
		return &DirectoryError{
			Path:   path,
			Reason: fmt.Sprintf("检查目录失败: %v", err),
		}
	}

	// 4. 自动创建目录（仅当配置启用时）
	if !m.autoCreate {
		logger.Warn("Directory does not exist and auto-create is disabled, delegating to download tool", "path", path)
		// 不返回错误，让 Aria2 自己尝试创建
		return nil
	}

	// 5. 尝试创建目录
	logger.Debug("Attempting to create directory", "path", path)
	if err := os.MkdirAll(path, 0755); err != nil {
		// 创建失败时，检查是否是权限问题
		if os.IsPermission(err) {
			logger.Warn("No permission to create directory, delegating to download tool", "path", path, "error", err)
			// 不返回错误，让 Aria2 自己尝试
			return nil
		}

		// 其他错误（如只读文件系统）也不阻止下载
		logger.Warn("Failed to create directory, delegating to download tool", "path", path, "error", err)
		return nil
	}

	// 6. 验证可写性（新创建的目录）
	if m.validatePerms {
		if err := m.checkWritable(path); err != nil {
			logger.Warn("Permission validation failed for newly created directory", "path", path, "error", err)
			// 不返回错误，不清理目录
		}
	}

	// 7. 更新缓存
	m.updateCache(path, true)

	logger.Debug("Directory created successfully", "path", path)
	return nil
}

// CheckDiskSpace 检查磁盘空间
func (m *DirectoryManager) CheckDiskSpace(path string, requiredBytes int64) error {
	if !m.checkDiskSpace {
		return nil
	}

	logger.Debug("Checking disk space", "path", path, "required", formatSize(requiredBytes))

	availableBytes, err := m.getAvailableSpace(path)
	if err != nil {
		logger.Warn("Unable to check disk space", "path", path, "error", err)
		return nil // 不阻止下载，只是警告
	}

	// 预留20%缓冲空间
	requiredWithBuffer := requiredBytes * 120 / 100

	if availableBytes < requiredWithBuffer {
		return &DirectoryError{
			Path: path,
			Reason: fmt.Sprintf(
				"Insufficient disk space: required %s (with buffer), available %s",
				formatSize(requiredWithBuffer),
				formatSize(availableBytes),
			),
		}
	}

	logger.Debug("Sufficient disk space",
		"available", formatSize(availableBytes),
		"required", formatSize(requiredWithBuffer))

	return nil
}

// CheckBatchDiskSpace 批量检查磁盘空间（用于批量下载）
func (m *DirectoryManager) CheckBatchDiskSpace(path string, totalBytes int64) error {
	if !m.checkDiskSpace || totalBytes == 0 {
		return nil
	}

	logger.Debug("Checking batch disk space",
		"path", path,
		"totalSize", formatSize(totalBytes))

	return m.CheckDiskSpace(path, totalBytes)
}

// checkWritable 检查目录可写性
func (m *DirectoryManager) checkWritable(path string) error {
	// 创建测试文件
	testFile := filepath.Join(path, ".write_test")

	// 尝试写入
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		return &DirectoryError{
			Path:   path,
			Reason: fmt.Sprintf("目录不可写: %v", err),
		}
	}

	// 清理测试文件
	if err := os.Remove(testFile); err != nil {
		logger.Warn("Failed to clean up test file", "file", testFile, "error", err)
	}

	return nil
}

// getAvailableSpace 获取可用磁盘空间
func (m *DirectoryManager) getAvailableSpace(path string) (int64, error) {
	var stat syscall.Statfs_t

	// 确保路径存在，否则使用父目录
	checkPath := path
	if _, err := os.Stat(path); os.IsNotExist(err) {
		checkPath = filepath.Dir(path)
	}

	err := syscall.Statfs(checkPath, &stat)
	if err != nil {
		return 0, fmt.Errorf("获取文件系统信息失败: %w", err)
	}

	// 可用空间 = 可用块数 * 块大小
	availableBytes := int64(stat.Bavail) * int64(stat.Bsize)

	return availableBytes, nil
}

// isInCache 检查目录是否在缓存中
func (m *DirectoryManager) isInCache(path string) bool {
	m.cacheMutex.RLock()
	defer m.cacheMutex.RUnlock()

	exists, ok := m.dirCache[path]
	return ok && exists
}

// updateCache 更新缓存
func (m *DirectoryManager) updateCache(path string, exists bool) {
	m.cacheMutex.Lock()
	defer m.cacheMutex.Unlock()

	m.dirCache[path] = exists
}

// ClearCache 清空缓存（用于测试或重置）
func (m *DirectoryManager) ClearCache() {
	m.cacheMutex.Lock()
	defer m.cacheMutex.Unlock()

	m.dirCache = make(map[string]bool)
	logger.Debug("Directory cache cleared")
}

// GetCacheSize 获取缓存大小（用于监控）
func (m *DirectoryManager) GetCacheSize() int {
	m.cacheMutex.RLock()
	defer m.cacheMutex.RUnlock()

	return len(m.dirCache)
}

// EnsureParentDirectory 确保父目录存在
func (m *DirectoryManager) EnsureParentDirectory(filePath string) error {
	parentDir := filepath.Dir(filePath)
	return m.EnsureDirectory(parentDir)
}

// ValidateDirectory 仅验证目录，不创建
func (m *DirectoryManager) ValidateDirectory(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &DirectoryError{
				Path:   path,
				Reason: "目录不存在",
			}
		}
		return &DirectoryError{
			Path:   path,
			Reason: fmt.Sprintf("检查目录失败: %v", err),
		}
	}

	if !info.IsDir() {
		return &DirectoryError{
			Path:   path,
			Reason: "路径存在但不是目录",
		}
	}

	// 验证可写性
	if m.validatePerms {
		if err := m.checkWritable(path); err != nil {
			return err
		}
	}

	return nil
}

// ========== 辅助函数 ==========

// formatSize 格式化文件大小
func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %cB",
		float64(bytes)/float64(div),
		"KMGTPE"[exp])
}
