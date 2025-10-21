package filesystem

import (
	"fmt"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"unicode"

	"github.com/easayliu/alist-aria2-download/internal/infrastructure/config"
	"github.com/easayliu/alist-aria2-download/pkg/logger"
)

// PathValidatorService 路径验证服务 - 负责路径安全性和有效性验证
type PathValidatorService struct {
	maxPathLength int
	platform      string
	reservedNames map[string]bool
}

// PathValidationError 路径验证错误
type PathValidationError struct {
	Path   string
	Reason string
}

func (e *PathValidationError) Error() string {
	return fmt.Sprintf("路径验证失败: %s - %s", e.Path, e.Reason)
}

// NewPathValidatorService 创建路径验证服务
func NewPathValidatorService(cfg *config.Config) *PathValidatorService {
	// 默认最大路径长度1024（保守值，兼容大多数系统）
	maxLength := 1024

	return &PathValidatorService{
		maxPathLength: maxLength,
		platform:      runtime.GOOS,
		reservedNames: BuildReservedNamesMap(),
	}
}

// BuildReservedNamesMap 构建Windows保留名称映射表（导出供platform包使用）
func BuildReservedNamesMap() map[string]bool {
	reserved := []string{
		"CON", "PRN", "AUX", "NUL",
		"COM1", "COM2", "COM3", "COM4", "COM5",
		"COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5",
		"LPT6", "LPT7", "LPT8", "LPT9",
	}

	reservedMap := make(map[string]bool)
	for _, name := range reserved {
		reservedMap[name] = true
	}

	return reservedMap
}

// Validate 验证路径有效性
func (s *PathValidatorService) Validate(path string) error {
	if path == "" {
		return &PathValidationError{Path: path, Reason: "路径为空"}
	}

	// 1. 检查路径长度
	if err := s.validateLength(path); err != nil {
		return err
	}

	// 2. 检查路径遍历攻击
	if err := s.validateTraversal(path); err != nil {
		return err
	}

	// 3. 检查特殊字符
	if err := s.validateCharacters(path); err != nil {
		return err
	}

	// 4. Windows平台特殊检查
	if s.platform == "windows" {
		if err := s.validateWindowsPath(path); err != nil {
			return err
		}
	}

	return nil
}

// validateLength 验证路径长度
func (s *PathValidatorService) validateLength(path string) error {
	if len(path) > s.maxPathLength {
		return &PathValidationError{
			Path:   path,
			Reason: fmt.Sprintf("路径长度超过限制 (%d > %d)", len(path), s.maxPathLength),
		}
	}
	return nil
}

// validateTraversal 验证路径遍历攻击
func (s *PathValidatorService) validateTraversal(path string) error {
	// 先检查原始路径是否包含 ..
	if strings.Contains(path, "..") {
		return &PathValidationError{
			Path:   path,
			Reason: "路径包含潜在的目录遍历攻击 (..)",
		}
	}
	return nil
}

// validateCharacters 验证特殊字符
func (s *PathValidatorService) validateCharacters(path string) error {
	// 检查零宽字符和控制字符
	for _, r := range path {
		if unicode.Is(unicode.Cc, r) && r != '\t' && r != '\n' && r != '\r' {
			return &PathValidationError{
				Path:   path,
				Reason: fmt.Sprintf("路径包含控制字符: U+%04X", r),
			}
		}

		// 检查零宽字符
		if isZeroWidthChar(r) {
			return &PathValidationError{
				Path:   path,
				Reason: fmt.Sprintf("路径包含零宽字符: U+%04X", r),
			}
		}
	}

	return nil
}

// validateWindowsPath 验证Windows路径特殊规则
func (s *PathValidatorService) validateWindowsPath(path string) error {
	// 1. 检查保留名称
	base := filepath.Base(path)
	nameWithoutExt := strings.TrimSuffix(base, filepath.Ext(base))
	nameUpper := strings.ToUpper(nameWithoutExt)

	if s.reservedNames[nameUpper] {
		return &PathValidationError{
			Path:   path,
			Reason: fmt.Sprintf("包含Windows保留名称: %s", nameUpper),
		}
	}

	// 2. 检查Windows不允许的字符
	invalidChars := []string{"<", ">", ":", "\"", "|", "?", "*"}
	for _, char := range invalidChars {
		if strings.Contains(path, char) {
			return &PathValidationError{
				Path:   path,
				Reason: fmt.Sprintf("路径包含Windows不允许的字符: %s", char),
			}
		}
	}

	// 3. 检查路径是否以空格或点结尾（Windows不允许）
	if strings.HasSuffix(base, " ") || strings.HasSuffix(base, ".") {
		return &PathValidationError{
			Path:   path,
			Reason: "Windows路径组件不能以空格或点结尾",
		}
	}

	return nil
}

// CleanPath 清理路径（移除不安全字符）
func (s *PathValidatorService) CleanPath(path string) string {
	logger.Debug("Cleaning path", "original", path)

	// 1. 移除零宽字符
	path = removeZeroWidthChars(path)

	// 2. 标准化空格
	path = normalizeWhitespace(path)

	// 3. 替换不安全字符
	path = s.replaceSafeChars(path)

	// 4. 清理路径（移除多余的分隔符等）
	path = filepath.Clean(path)

	// 5. 限制长度
	if len(path) > s.maxPathLength {
		path = s.truncatePath(path)
	}

	logger.Debug("Path cleaning completed", "cleaned", path)
	return path
}

// replaceSafeChars 替换不安全字符为安全字符
func (s *PathValidatorService) replaceSafeChars(path string) string {
	// 根据平台选择替换策略
	if s.platform == "windows" {
		// Windows: 替换不允许的字符
		replacer := strings.NewReplacer(
			":", "-",  // 冒号替换为破折号
			"?", "",   // 问号移除
			"*", "",   // 星号移除
			"<", "",   // 移除
			">", "",   // 移除
			"|", "-",  // 管道符替换为破折号
			"\"", "'", // 双引号替换为单引号
		)
		return replacer.Replace(path)
	}

	// Linux/macOS: 只替换可能有问题的字符
	replacer := strings.NewReplacer(
		":", "-", // 冒号替换为破折号（虽然Linux允许，但可能造成混淆）
	)
	return replacer.Replace(path)
}

// truncatePath 截断过长的路径
func (s *PathValidatorService) truncatePath(path string) string {
	// 策略：保留目录结构，截断文件名
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	nameWithoutExt := strings.TrimSuffix(base, ext)

	// 计算可用长度
	availableLength := s.maxPathLength - len(dir) - len(ext) - 10 // 预留10字节缓冲

	if availableLength < 20 {
		// 如果目录路径太长，只保留基础目录
		logger.Warn("Path too long, using simplified path", "original", path)
		return filepath.Join(filepath.Dir(dir), base[:20]+ext)
	}

	// 截断文件名
	if len(nameWithoutExt) > availableLength {
		nameWithoutExt = nameWithoutExt[:availableLength]
	}

	return filepath.Join(dir, nameWithoutExt+ext)
}

// ValidateAndClean 验证并清理路径（组合方法）
func (s *PathValidatorService) ValidateAndClean(path string) (string, error) {
	// 1. 先清理
	cleanPath := s.CleanPath(path)

	// 2. 再验证
	if err := s.Validate(cleanPath); err != nil {
		return "", err
	}

	return cleanPath, nil
}

// NormalizePath 规范化路径（跨平台处理）
func (s *PathValidatorService) NormalizePath(path string) string {
	// 1. 统一分隔符为当前平台
	path = filepath.FromSlash(path)

	// 2. 清理路径
	path = filepath.Clean(path)

	// 3. Windows特殊处理
	if s.platform == "windows" {
		path = s.ensureWindowsDrive(path)
	}

	return path
}

// ensureWindowsDrive 确保Windows路径有驱动器号
func (s *PathValidatorService) ensureWindowsDrive(path string) string {
	// 如果是相对路径或者没有驱动器号
	if !filepath.IsAbs(path) || filepath.VolumeName(path) == "" {
		// 检查是否是Unix风格的绝对路径（以/开头）
		if strings.HasPrefix(path, "/") || strings.HasPrefix(path, "\\") {
			// 添加默认驱动器（通常是C:）
			return "C:" + path
		}
	}
	return path
}

// ========== 辅助函数 ==========

// isZeroWidthChar 检查是否为零宽字符
func isZeroWidthChar(r rune) bool {
	zeroWidthChars := []rune{
		'\u200B', // 零宽空格
		'\u200C', // 零宽非连接符
		'\u200D', // 零宽连接符
		'\u200E', // 左到右标记
		'\u200F', // 右到左标记
		'\uFEFF', // 零宽非断空格（BOM）
	}

	for _, zw := range zeroWidthChars {
		if r == zw {
			return true
		}
	}

	return false
}

// removeZeroWidthChars 移除零宽字符
func removeZeroWidthChars(s string) string {
	var result strings.Builder
	result.Grow(len(s))

	for _, r := range s {
		if !isZeroWidthChar(r) {
			result.WriteRune(r)
		}
	}

	return result.String()
}

// normalizeWhitespace 标准化空格
func normalizeWhitespace(s string) string {
	// 将多个连续空格替换为单个空格
	re := regexp.MustCompile(`\s+`)
	s = re.ReplaceAllString(s, " ")

	// 移除首尾空格
	s = strings.TrimSpace(s)

	return s
}
