package valueobjects

import (
	"errors"
	"path/filepath"
	"strings"
)

// FilePath 文件路径值对象
// 不可变的值对象,提供自动验证和规范化功能
type FilePath struct {
	value string
}

// String 返回路径字符串
func (p FilePath) String() string {
	return p.value
}

// IsEmpty 判断路径是否为空
func (p FilePath) IsEmpty() bool {
	return p.value == ""
}

// Join 连接路径,返回新的FilePath
func (p FilePath) Join(elem string) FilePath {
	return FilePath{value: filepath.Join(p.value, elem)}
}

// Dir 返回路径的目录部分
func (p FilePath) Dir() FilePath {
	return FilePath{value: filepath.Dir(p.value)}
}

// Base 返回路径的文件名部分
func (p FilePath) Base() string {
	return filepath.Base(p.value)
}

// Ext 返回文件扩展名
func (p FilePath) Ext() string {
	return filepath.Ext(p.value)
}

// IsAbsolute 判断是否为绝对路径
func (p FilePath) IsAbsolute() bool {
	return filepath.IsAbs(p.value)
}

// NewFilePath 创建文件路径值对象,进行验证和规范化
func NewFilePath(path string) (FilePath, error) {
	// 去除首尾空格
	path = strings.TrimSpace(path)

	// 空路径检查
	if path == "" {
		return FilePath{}, errors.New("path cannot be empty")
	}

	// 路径遍历攻击检查
	if strings.Contains(path, "..") {
		return FilePath{}, errors.New("path contains illegal '..' sequence")
	}

	// 路径长度检查(最大1024字节)
	if len(path) > 1024 {
		return FilePath{}, errors.New("path exceeds maximum length of 1024 bytes")
	}

	// 清理路径(统一分隔符)
	cleaned := filepath.Clean(path)

	return FilePath{value: cleaned}, nil
}

// MustNewFilePath 创建文件路径值对象,如果失败则panic(仅用于测试或确定安全的场景)
func MustNewFilePath(path string) FilePath {
	fp, err := NewFilePath(path)
	if err != nil {
		panic(err)
	}
	return fp
}

// NewFilePathUnchecked 创建文件路径值对象,不进行验证(内部使用)
func NewFilePathUnchecked(path string) FilePath {
	return FilePath{value: path}
}
