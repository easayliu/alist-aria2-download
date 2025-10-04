package strutil

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// EscapeHTML 转义HTML特殊字符
// 遵循 Telegram Bot API HTML 格式规范,仅需转义 4 个字符: & < > "
// 其他字符(包括 emoji 和中文)无需转义
func EscapeHTML(text string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		"\"", "&quot;",
	)
	return replacer.Replace(text)
}

// FormatFileSize 格式化文件大小
func FormatFileSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return strconv.FormatInt(bytes, 10) + " B"
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return strconv.FormatFloat(float64(bytes)/float64(div), 'f', 1, 64) + " " + "KMGTPE"[exp:exp+1] + "B"
}

// ParseInt64 解析字符串为int64
func ParseInt64(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

// BuildMediaStats 构建媒体统计信息
func BuildMediaStats(tvCount, movieCount, otherCount int) gin.H {
	return gin.H{
		"tv":    tvCount,
		"movie": movieCount,
		"other": otherCount,
	}
}
