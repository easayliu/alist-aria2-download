package pathutil

import "strings"

// CommonSkipDirs 常见的需要跳过的目录名（系统目录、通用分类目录）
// 这些目录通常不包含有意义的节目名信息
var CommonSkipDirs = map[string]bool{
	// 空值和特殊目录
	"":   true,
	".":  true,
	"..": true,
	"/":  true,

	// 特殊标记
	"data":     true,
	"来自：分享":   true,
	"来自分享":    true,
	"分享":      true,

	// 媒体分类目录（英文）
	"tvs":       true,
	"tv":        true,
	"series":    true,
	"movies":    true,
	"movie":     true,
	"films":     true,
	"film":      true,
	"video":     true,
	"videos":    true,
	"anime":     true,
	"variety":   true,
	"shows":     true,
	"show":      true,
	"download":  true,
	"downloads": true,
	"media":     true,

	// 媒体分类目录（中文）
	"电视剧":  true,
	"电影":   true,
	"动画":   true,
	"动漫":   true,
	"长篇剧":  true,
	"综艺":   true,
	"娱乐":   true,
	"视频":   true,
}

// ShouldSkipDirectory 判断是否应该跳过该目录
// 会同时检查原始名称和小写版本
func ShouldSkipDirectory(dirName string) bool {
	if dirName == "" {
		return true
	}

	// 检查原始名称
	if CommonSkipDirs[dirName] {
		return true
	}

	// 检查小写版本
	if CommonSkipDirs[strings.ToLower(dirName)] {
		return true
	}

	return false
}

// IsCommonCategoryDir 判断是否为常见的分类目录（不包含具体节目信息）
func IsCommonCategoryDir(dirName string) bool {
	return ShouldSkipDirectory(dirName)
}

// FilterSkipDirs 从目录列表中过滤掉应该跳过的目录
func FilterSkipDirs(dirs []string) []string {
	if len(dirs) == 0 {
		return dirs
	}

	filtered := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		if !ShouldSkipDirectory(dir) {
			filtered = append(filtered, dir)
		}
	}
	return filtered
}

// AddCustomSkipDir 添加自定义需要跳过的目录（用于扩展）
func AddCustomSkipDir(dirName string) {
	if dirName != "" {
		CommonSkipDirs[dirName] = true
		CommonSkipDirs[strings.ToLower(dirName)] = true
	}
}
