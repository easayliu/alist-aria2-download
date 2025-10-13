package strutil

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

	// 1. 移除网站水印和发布信息（最高优先级）
	// 匹配模式：【网站信息】 或 [网站信息]
	websitePatterns := []*regexp.Regexp{
		regexp.MustCompile(`【[^】]*(?:www\.|\.com|\.cn|\.org|发布|高清|影视|字幕组|下载)[^】]*】`),
		regexp.MustCompile(`\[[^\]]*(?:www\.|\.com|\.cn|\.org|发布|高清|影视|字幕组|下载)[^\]]*\]`),
		regexp.MustCompile(`【[^】]+】`), // 移除所有【】括号内容作为备选
		regexp.MustCompile(`\[[^\]]+\]`), // 移除所有[]括号内容作为备选
	}

	for _, pattern := range websitePatterns {
		cleaned = pattern.ReplaceAllString(cleaned, "")
	}

	// 2. 移除视频质量和编码信息
	qualityPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)\d{3,4}p`),                                    // 1080p, 2160p
		regexp.MustCompile(`(?i)WEB-DL|WEB-RIP|BluRay|BDRip|HDTV|DVDRip`),   // 来源
		regexp.MustCompile(`(?i)H\.?264|H\.?265|x264|x265|HEVC|AVC`),         // 编码
		regexp.MustCompile(`(?i)HDR|SDR|DTS|DD5\.1|AAC|AC3|DDP\d+\.\d+`),    // 音视频格式
		regexp.MustCompile(`(?i)-[A-Z][a-zA-Z0-9]+$`),                        // 发布组名 -QuickIO
	}

	for _, pattern := range qualityPatterns {
		cleaned = pattern.ReplaceAllString(cleaned, "")
	}

	// 3. 优先提取中文部分（如果存在混合的英文和中文）
	// 匹配中文名称，移除英文部分
	if containsChinese(cleaned) {
		// 如果包含中文，尝试提取纯中文部分或中文为主的部分
		// 匹配模式：移除前面的纯英文单词和点号
		chinesePattern := regexp.MustCompile(`^[A-Za-z0-9.\s-]+(.*)$`)
		if matches := chinesePattern.FindStringSubmatch(cleaned); len(matches) > 1 && containsChinese(matches[1]) {
			cleaned = matches[1]
		}

		// 移除尾部的纯英文单词（用点号分隔）
		parts := strings.Split(cleaned, ".")
		var chineseParts []string
		for _, part := range parts {
			// 只保留包含中文或数字的部分
			if containsChinese(part) || (len(part) > 0 && part[0] >= '0' && part[0] <= '9') {
				chineseParts = append(chineseParts, part)
			} else if len(part) > 0 && !isAllEnglish(part) {
				chineseParts = append(chineseParts, part)
			}
		}
		if len(chineseParts) > 0 {
			cleaned = strings.Join(chineseParts, ".")
		}
	}

	// 4. 移除季度后缀信息（第X季、Season X等）
	seasonSuffixPattern := regexp.MustCompile(`[.\s]*(?:第[零一二三四五六七八九十百\d]+季|[Ss]eason[\s_-]?\d+|[Ss]\d{1,2}).*$`)
	cleaned = seasonSuffixPattern.ReplaceAllString(cleaned, "")

	// 5. 移除常见的后缀信息
	suffixesToRemove := []string{
		"（", "(", "[", "【",
		"2021", "2022", "2023", "2024", "2025", "2026", "2027", "2028",
		"全", "期全", "完结", "更新", "集全", "全集", "合集", "完整版", "系列",
		"国语配音", "中文字幕", "英文字幕", "双语字幕",
	}

	for _, suffix := range suffixesToRemove {
		if idx := strings.Index(cleaned, suffix); idx != -1 {
			cleaned = cleaned[:idx]
		}
	}

	// 6. 移除多余的点号、冒号和其他特殊字符
	cleaned = strings.ReplaceAll(cleaned, ".", "")
	cleaned = strings.ReplaceAll(cleaned, ":", "")   // 英文冒号
	cleaned = strings.ReplaceAll(cleaned, "：", "")  // 中文冒号
	cleaned = strings.ReplaceAll(cleaned, "·", "")   // 中文间隔号

	// 7. 去除前后空白
	cleaned = strings.TrimSpace(cleaned)

	// 8. 如果清理后为空或太短，返回原名
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

// isAllEnglish 检查字符串是否全部是英文字母
func isAllEnglish(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')) {
			return false
		}
	}
	return true
}
