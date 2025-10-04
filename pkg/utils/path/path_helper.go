package pathutil

import "path/filepath"

// ResolveDefaultPath 解析默认路径
// 如果path为空,使用defaultPath;如果defaultPath也为空,使用"/"
func ResolveDefaultPath(path, defaultPath string) string {
	if path == "" {
		path = defaultPath
		if path == "" {
			path = "/"
		}
	}
	return path
}

// JoinPath 连接路径
func JoinPath(paths ...string) string {
	return filepath.Join(paths...)
}

// GetParentPath 获取父路径
func GetParentPath(path string) string {
	return filepath.Dir(path)
}

// GetFileName 获取文件名
func GetFileName(path string) string {
	return filepath.Base(path)
}
