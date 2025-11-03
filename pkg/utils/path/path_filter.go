package pathutil

import (
	"strconv"
	"strings"
)

// CommonSkipDirs 常见的需要跳过的目录名（系统目录、通用分类目录）
// 这些目录通常不包含有意义的节目名信息
var CommonSkipDirs = map[string]bool{
	// 空值和特殊目录
	"":   true,
	".":  true,
	"..": true,
	"/":  true,

	// 特殊标记
	"data":  true,
	"来自：分享": true,
	"来自分享":  true,
	"分享":    true,

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
	"电视剧": true,
	"电影":  true,
	"动画":  true,
	"动漫":  true,
	"长篇剧": true,
	"综艺":  true,
	"娱乐":  true,
	"视频":  true,

	// 地区分类目录
	"国产":    true,
	"华语":    true,
	"港台":    true,
	"欧美":    true,
	"日韩":    true,
	"日本":    true,
	"韩国":    true,
	"美国":    true,
	"英国":    true,
	"外语":    true,
	"其他":    true,
	"other": true,

	// 质量分类目录
	"4k":     true,
	"4K":     true,
	"1080p":  true,
	"1080P":  true,
	"720p":   true,
	"720P":   true,
	"高清":     true,
	"超清":     true,
	"蓝光":     true,
	"bluray": true,
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

// IsYearDirectory 判断目录名是否为年份（2000-2099）
func IsYearDirectory(dirName string) bool {
	// 检查是否为4位数字
	if len(dirName) != 4 {
		return false
	}

	// 尝试转换为数字
	year, err := strconv.Atoi(dirName)
	if err != nil {
		return false
	}

	// 检查范围（2000-2099）
	return year >= 2000 && year <= 2099
}

// ShouldSkipDirectoryAdvanced 判断是否应该跳过该目录（包含年份检测）
func ShouldSkipDirectoryAdvanced(dirName string) bool {
	// 首先检查基本的跳过条件
	if ShouldSkipDirectory(dirName) {
		return true
	}

	// 检查是否为年份目录
	if IsYearDirectory(dirName) {
		return true
	}

	return false
}
