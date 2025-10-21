package platform

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/easayliu/alist-aria2-download/internal/infrastructure/filesystem"
	"github.com/easayliu/alist-aria2-download/pkg/logger"
)

// PathAdapter 跨平台路径适配器 - 处理不同操作系统的路径差异
type PathAdapter struct {
	platform         string
	reservedNames    map[string]bool
	invalidCharsWin  []string
	pathSeparator    string
	volumeNamePrefix string
}

// NewPathAdapter 创建路径适配器
func NewPathAdapter() *PathAdapter {
	platform := runtime.GOOS

	adapter := &PathAdapter{
		platform:      platform,
		reservedNames: filesystem.BuildReservedNamesMap(),
		invalidCharsWin: []string{
			"<", ">", ":", "\"", "|", "?", "*",
		},
	}

	// 设置平台特定属性
	switch platform {
	case "windows":
		adapter.pathSeparator = "\\"
		adapter.volumeNamePrefix = "C:"
	default:
		adapter.pathSeparator = "/"
		adapter.volumeNamePrefix = ""
	}

	return adapter
}

// NormalizePath 规范化路径（跨平台处理）
func (a *PathAdapter) NormalizePath(path string) string {
	logger.Debug("Normalizing path", "original", path, "platform", a.platform)

	// 1. 统一分隔符为当前平台
	path = filepath.FromSlash(path)

	// 2. 清理路径（移除 . 和 ..）
	path = filepath.Clean(path)

	// 3. Windows特殊处理
	if a.platform == "windows" {
		path = a.ensureWindowsDrive(path)
	}

	logger.Debug("Path normalization completed", "normalized", path)
	return path
}

// ToURLPath 转换为URL路径（用于Alist等Web接口）
func (a *PathAdapter) ToURLPath(path string) string {
	// 始终使用正斜杠
	urlPath := filepath.ToSlash(path)

	// 确保以 / 开头
	if !strings.HasPrefix(urlPath, "/") {
		urlPath = "/" + urlPath
	}

	return urlPath
}

// FromURLPath 从URL路径转换（Alist返回的路径）
func (a *PathAdapter) FromURLPath(urlPath string) string {
	return a.NormalizePath(urlPath)
}

// ValidatePath 验证路径在当前平台是否有效
func (a *PathAdapter) ValidatePath(path string) error {
	switch a.platform {
	case "windows":
		return a.validateWindowsPath(path)
	case "linux", "darwin":
		return a.validateUnixPath(path)
	default:
		return nil
	}
}

// validateWindowsPath 验证Windows路径
func (a *PathAdapter) validateWindowsPath(path string) error {
	// 1. 检查保留名称
	base := filepath.Base(path)
	nameWithoutExt := strings.TrimSuffix(base, filepath.Ext(base))
	nameUpper := strings.ToUpper(nameWithoutExt)

	if a.reservedNames[nameUpper] {
		return fmt.Errorf("Windows保留名称: %s", nameUpper)
	}

	// 2. 检查不允许的字符
	for _, char := range a.invalidCharsWin {
		if strings.Contains(path, char) {
			return fmt.Errorf("路径包含Windows不允许的字符: %s", char)
		}
	}

	// 3. 检查路径是否以空格或点结尾
	if strings.HasSuffix(base, " ") || strings.HasSuffix(base, ".") {
		return fmt.Errorf("Windows路径组件不能以空格或点结尾")
	}

	// 4. 检查路径长度
	if len(path) > 260 {
		logger.Warn("Windows path may be too long", "length", len(path), "max", 260)
	}

	return nil
}

// validateUnixPath 验证Unix路径
func (a *PathAdapter) validateUnixPath(path string) error {
	// Unix系统相对宽松，主要检查空字符
	if strings.Contains(path, "\x00") {
		return fmt.Errorf("路径包含空字符")
	}

	return nil
}

// ensureWindowsDrive 确保Windows路径有驱动器号
func (a *PathAdapter) ensureWindowsDrive(path string) string {
	// 如果是相对路径或者没有驱动器号
	if !filepath.IsAbs(path) || filepath.VolumeName(path) == "" {
		// 检查是否是Unix风格的绝对路径（以/开头）
		if strings.HasPrefix(path, "/") || strings.HasPrefix(path, "\\") {
			// 添加默认驱动器
			if a.volumeNamePrefix != "" {
				return a.volumeNamePrefix + path
			}
		}
	}
	return path
}

// ConvertSeparators 转换路径分隔符
func (a *PathAdapter) ConvertSeparators(path string, targetPlatform string) string {
	var targetSep string
	switch targetPlatform {
	case "windows":
		targetSep = "\\"
	default:
		targetSep = "/"
	}

	// 替换分隔符
	path = strings.ReplaceAll(path, "\\", targetSep)
	path = strings.ReplaceAll(path, "/", targetSep)

	return path
}

// IsAbsolute 检查是否为绝对路径
func (a *PathAdapter) IsAbsolute(path string) bool {
	if a.platform == "windows" {
		// Windows: 检查驱动器号或UNC路径
		return filepath.IsAbs(path) || strings.HasPrefix(path, "\\\\")
	}

	// Unix: 以 / 开头
	return filepath.IsAbs(path)
}

// Join 连接路径（跨平台）
func (a *PathAdapter) Join(paths ...string) string {
	return filepath.Join(paths...)
}

// Split 分割路径
func (a *PathAdapter) Split(path string) (dir, file string) {
	return filepath.Split(path)
}

// Dir 获取目录部分
func (a *PathAdapter) Dir(path string) string {
	return filepath.Dir(path)
}

// Base 获取文件名部分
func (a *PathAdapter) Base(path string) string {
	return filepath.Base(path)
}

// Ext 获取扩展名
func (a *PathAdapter) Ext(path string) string {
	return filepath.Ext(path)
}

// GetPlatform 获取当前平台
func (a *PathAdapter) GetPlatform() string {
	return a.platform
}

// IsCaseSensitive 检查当前平台文件系统是否大小写敏感
func (a *PathAdapter) IsCaseSensitive() bool {
	switch a.platform {
	case "windows", "darwin":
		return false // Windows和macOS默认不区分大小写
	default:
		return true // Linux默认区分大小写
	}
}

// NormalizeCasing 规范化大小写（用于比较）
func (a *PathAdapter) NormalizeCasing(path string) string {
	if !a.IsCaseSensitive() {
		return strings.ToLower(path)
	}
	return path
}

// ComparePaths 跨平台路径比较
func (a *PathAdapter) ComparePaths(path1, path2 string) bool {
	// 规范化后比较
	norm1 := a.NormalizePath(a.NormalizeCasing(path1))
	norm2 := a.NormalizePath(a.NormalizeCasing(path2))
	return norm1 == norm2
}

// MakeRelative 将绝对路径转换为相对路径
func (a *PathAdapter) MakeRelative(basePath, targetPath string) (string, error) {
	// 规范化路径
	basePath = a.NormalizePath(basePath)
	targetPath = a.NormalizePath(targetPath)

	// 使用标准库计算相对路径
	relPath, err := filepath.Rel(basePath, targetPath)
	if err != nil {
		return "", fmt.Errorf("无法计算相对路径: %w", err)
	}

	return relPath, nil
}

// MakeAbsolute 将相对路径转换为绝对路径
func (a *PathAdapter) MakeAbsolute(basePath, relativePath string) string {
	if a.IsAbsolute(relativePath) {
		return relativePath
	}

	return a.Join(basePath, relativePath)
}
