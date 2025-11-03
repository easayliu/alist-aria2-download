package fileutil

import "strings"

// 默认支持的视频扩展名列表
var DefaultVideoExtensions = []string{
	"mp4", "mkv", "avi", "mov", "wmv", "flv", "webm",
	"m4v", "mpg", "mpeg", "3gp", "rmvb", "ts", "m2ts",
}

// IsVideoFile 检查文件是否为视频文件
// filename: 文件名或完整路径
// videoExts: 可选的视频扩展名列表，如果为空则使用默认列表
func IsVideoFile(filename string, videoExts ...[]string) bool {
	if filename == "" {
		return false
	}

	// 提取扩展名
	ext := ExtractExtension(filename)
	if ext == "" {
		return false
	}

	// 确定使用哪个扩展名列表
	var exts []string
	if len(videoExts) > 0 && len(videoExts[0]) > 0 {
		exts = videoExts[0]
	} else {
		exts = DefaultVideoExtensions
	}

	// 检查是否匹配
	for _, videoExt := range exts {
		if strings.EqualFold(ext, videoExt) {
			return true
		}
	}

	return false
}

// ExtractExtension 从文件名中提取扩展名（不带点号，小写）
// 例如：
//
//	"video.mp4" -> "mp4"
//	"movie.MKV" -> "mkv"
//	"/path/to/file.AVI" -> "avi"
func ExtractExtension(filename string) string {
	if filename == "" {
		return ""
	}

	// 查找最后一个点号
	idx := strings.LastIndex(filename, ".")
	if idx == -1 || idx == len(filename)-1 {
		return ""
	}

	// 提取并转为小写
	ext := filename[idx+1:]
	return strings.ToLower(ext)
}

// HasVideoExtension 检查文件名是否有视频扩展名（不检查列表，只检查格式）
func HasVideoExtension(filename string) bool {
	return ExtractExtension(filename) != ""
}
