package utils

import (
	"regexp"
	"strings"
	"unicode"
)

// CleanShowName 清理节目名称 - 提取中文名，移除特殊字符和后缀
// 用于统一处理电视剧、电影、综艺等媒体名称
func CleanShowName(name string) string {
	if name == "" {
		return ""
	}

	cleaned := name

	// 1. 优先提取中文名（如果存在）
	// 匹配模式：英文名.中文名 或 英文名：中文名
	chineseNamePattern := regexp.MustCompile(`[a-zA-Z0-9.\s]+[.:](.+)`)
	if matches := chineseNamePattern.FindStringSubmatch(cleaned); len(matches) > 1 {
		chinesePart := matches[1]
		// 验证提取的是否包含中文
		if containsChinese(chinesePart) {
			cleaned = chinesePart
		}
	}

	// 2. 移除常见的后缀信息
	suffixesToRemove := []string{
		"（", "(", "[", "【",
		"2021", "2022", "2023", "2024", "2025", "2026", "2027",
		"全", "期全", "完结", "更新", "集全", "全集", "合集", "完整版", "系列",
	}

	for _, suffix := range suffixesToRemove {
		if idx := strings.Index(cleaned, suffix); idx != -1 {
			cleaned = cleaned[:idx]
		}
	}

	// 3. 移除多余的点号、冒号和其他特殊字符
	cleaned = strings.ReplaceAll(cleaned, ".", "")
	cleaned = strings.ReplaceAll(cleaned, ":", "")   // 英文冒号
	cleaned = strings.ReplaceAll(cleaned, "：", "")  // 中文冒号
	cleaned = strings.ReplaceAll(cleaned, "·", "")   // 中文间隔号

	// 4. 去除前后空白
	cleaned = strings.TrimSpace(cleaned)

	// 5. 如果清理后为空或太短，返回原名
	if len(cleaned) < 2 {
		return name
	}

	return cleaned
}

// containsChinese 检查字符串是否包含中文字符
func containsChinese(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}
