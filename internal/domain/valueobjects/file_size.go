package valueobjects

import "fmt"

// FileSize 文件大小值对象
// 不可变的值对象,自动提供格式化功能
type FileSize int64

// Bytes 返回字节数
func (f FileSize) Bytes() int64 {
	return int64(f)
}

// Format 格式化为人类可读的字符串
func (f FileSize) Format() string {
	size := float64(f)
	units := []string{"B", "KB", "MB", "GB", "TB", "PB"}

	unitIndex := 0
	for size >= 1024 && unitIndex < len(units)-1 {
		size /= 1024
		unitIndex++
	}

	if unitIndex == 0 {
		return fmt.Sprintf("%d %s", int(size), units[unitIndex])
	}
	return fmt.Sprintf("%.2f %s", size, units[unitIndex])
}

// IsZero 判断是否为0
func (f FileSize) IsZero() bool {
	return f == 0
}

// IsLargerThan 判断是否大于指定大小
func (f FileSize) IsLargerThan(other FileSize) bool {
	return f > other
}

// Add 加法运算,返回新的FileSize
func (f FileSize) Add(other FileSize) FileSize {
	return FileSize(int64(f) + int64(other))
}

// NewFileSize 创建文件大小值对象
func NewFileSize(bytes int64) FileSize {
	if bytes < 0 {
		return FileSize(0)
	}
	return FileSize(bytes)
}

// NewFileSizeFromMB 从MB创建文件大小
func NewFileSizeFromMB(mb float64) FileSize {
	return FileSize(int64(mb * 1024 * 1024))
}

// NewFileSizeFromGB 从GB创建文件大小
func NewFileSizeFromGB(gb float64) FileSize {
	return FileSize(int64(gb * 1024 * 1024 * 1024))
}
