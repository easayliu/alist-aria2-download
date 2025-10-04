package utils

import (
	"strings"
)

// ========== HTML 相关工具函数 ==========

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

// ========== 说明 ==========
// FormatFileSize - 已存在于 pkg/utils/response.go
// IsVideoFile - 已存在于 pkg/utils/file_type.go (支持自定义扩展名列表)
