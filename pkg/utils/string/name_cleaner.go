package strutil

import (
	"regexp"
	"strings"
	"unicode"
)

// 预编译正则表达式以提升性能
var (
	// 网站水印模式
	websitePattern1 = regexp.MustCompile(`【[^】]*(?:www\.|\.com|\.cn|\.org|发布|高清|影视|字幕组|下载)[^】]*】`)
	websitePattern2 = regexp.MustCompile(`\[[^\]]*(?:www\.|\.com|\.cn|\.org|发布|高清|影视|字幕组|下载)[^\]]*\]`)
	websitePattern3 = regexp.MustCompile(`【[^】]+】`)    // 移除所有【】括号内容
	websitePattern4 = regexp.MustCompile(`\[[^\]]+\]`) // 移除所有[]括号内容

	// 视频质量和编码信息（按从复杂到简单的顺序，避免部分匹配）
	qualityPattern1 = regexp.MustCompile(`(?i)\d{3,4}p`)                                                     // 1080p, 2160p, 4K, 8K
	qualityPattern2 = regexp.MustCompile(`(?i)WEB-DL|WEB-RIP|WEBRip|BluRay|Blu-ray|BDRip|HDTV|DVDRip|REMUX`) // 🔥 片源（增加REMUX）
	qualityPattern3 = regexp.MustCompile(`(?i)H\.?264|H\.?265|H\.?266|x264|x265|HEVC|AVC|AV1|VP9`)           // 🔥 编码（增加AV1, VP9, H266）

	// 🔥 版本标记（REPACK, PROPER, EXTENDED等）
	versionPattern = regexp.MustCompile(`(?i)\b(REPACK|PROPER|EXTENDED|UNRATED|DC|DIRECTORS?\.CUT|LIMITED|ANNIVERSARY\.EDITION|REMASTERED)\b`)

	// 🔥 复杂音频格式必须先匹配（避免被简单DTS规则部分清理）
	qualityPattern6 = regexp.MustCompile(`(?i)DTS-HD(MA)?[\d.]*|DTS:?-?X[\d.]*|Atmos|TrueHD[\d.]*|LPCM[\d.]*|FLAC[\d.]*|EAC3|E-AC3|DD\+[\d.]*|OPUS[\d.]*`) // 🔥 音频格式（修复DTS-X匹配）
	qualityPattern4 = regexp.MustCompile(`(?i)HDR\d*|SDR|DTS|DD[\d.]*|AAC|AC3|DDP\d+\.\d+|MP3|DV`)                                                         // 🔥 基础音视频格式（增加DV）

	// 🔥 声道信息（7.1, 5.1, 2.0等）
	channelPattern = regexp.MustCompile(`\.?[\d]+\.[\d]`)

	qualityPattern5     = regexp.MustCompile(`(?i)-[A-Z][a-zA-Z0-9]+$`) // 发布组名
	qualityPattern7     = regexp.MustCompile(`(?i)\d+bit`)              // 位深（10bit, 8bit）
	qualityPattern8     = regexp.MustCompile(`(?i)\d+Audio`)            // 多音轨（2Audio等）
	otherQualityPattern = regexp.MustCompile(`(?i)UHD|4K|8K`)           // 超高清标记

	// 🔥 多余的描述信息模式（多音轨、字幕等）
	descriptorPattern  = regexp.MustCompile(`(?i)[.\s]*(国台粤英?|国粤英?|国英|台英|粤英|多音轨|特效字幕|中[英日韩法]?字幕|内嵌?字幕|双语字幕|简[繁]?[中英日]?字幕|无字幕)[.\s]*`)
	qualityDescPattern = regexp.MustCompile(`(?i)[.\s]*(高清|超清|蓝光|原盘|修复版|导演剪辑版|加长版|未删减版|完整版)[.\s]*`)

	// 🔥 年份模式（独立的4位数年份：1900-2099）
	yearPattern = regexp.MustCompile(`\b(19\d{2}|20\d{2})\b`)
	// 🔥 年份范围模式（如1997-2012, 2002-2003）
	yearRangePattern = regexp.MustCompile(`\d{4}-\d{4}`)

	// 中文提取模式
	chineseExtractPattern = regexp.MustCompile(`^[A-Za-z0-9.\s-]+(.*)$`)

	// 季度后缀模式
	seasonSuffixPattern = regexp.MustCompile(`[.\s]*(?:第[零一二三四五六七八九十百\d]+季|[Ss]eason[\s_-]?\d+|[Ss]\d{1,2}).*$`)
)

// CleanShowName 清理节目名称 - 提取中文名，移除特殊字符和后缀
// 用于统一处理电视剧、电影、综艺等媒体名称
func CleanShowName(name string) string {
	if name == "" {
		return ""
	}

	cleaned := name

	// 🔥 0. 先移除常见视频文件扩展名（避免影响后续清理）
	videoExtensions := []string{".mkv", ".mp4", ".avi", ".mov", ".wmv", ".flv", ".webm", ".m4v", ".mpg", ".mpeg", ".ts", ".m2ts"}
	for _, ext := range videoExtensions {
		if strings.HasSuffix(strings.ToLower(cleaned), ext) {
			cleaned = cleaned[:len(cleaned)-len(ext)]
			break
		}
	}

	// 1. 移除网站水印和发布信息（使用预编译的正则）
	cleaned = websitePattern1.ReplaceAllString(cleaned, "")
	cleaned = websitePattern2.ReplaceAllString(cleaned, "")
	cleaned = websitePattern3.ReplaceAllString(cleaned, "")
	cleaned = websitePattern4.ReplaceAllString(cleaned, "")

	// 2. 移除视频质量和编码信息（按从复杂到简单的顺序）
	cleaned = yearRangePattern.ReplaceAllString(cleaned, "")    // 🔥 先移除年份范围（避免与单独年份冲突）
	cleaned = yearPattern.ReplaceAllString(cleaned, "")         // 🔥 移除年份
	cleaned = descriptorPattern.ReplaceAllString(cleaned, "")   // 🔥 移除多余描述信息（多音轨、字幕等）
	cleaned = qualityDescPattern.ReplaceAllString(cleaned, "")  // 🔥 移除质量描述（高清、蓝光等）
	cleaned = versionPattern.ReplaceAllString(cleaned, "")      // 🔥 版本标记（REPACK, PROPER等）
	cleaned = qualityPattern6.ReplaceAllString(cleaned, "")     // 🔥 先移除复杂音频格式（DTS-HDMA, TrueHD, DTS:X等）
	cleaned = channelPattern.ReplaceAllString(cleaned, "")      // 🔥 移除声道信息（7.1, 5.1等）
	cleaned = qualityPattern8.ReplaceAllString(cleaned, "")     // 🔥 移除多音轨标记（2Audio）
	cleaned = otherQualityPattern.ReplaceAllString(cleaned, "") // 🔥 移除UHD, 4K, 8K
	cleaned = qualityPattern1.ReplaceAllString(cleaned, "")     // 分辨率
	cleaned = qualityPattern2.ReplaceAllString(cleaned, "")     // 来源（REMUX, BluRay等）
	cleaned = qualityPattern3.ReplaceAllString(cleaned, "")     // 编码（AV1, VP9, HEVC等）
	cleaned = qualityPattern4.ReplaceAllString(cleaned, "")     // 基础音视频格式
	cleaned = qualityPattern7.ReplaceAllString(cleaned, "")     // 位深
	cleaned = qualityPattern5.ReplaceAllString(cleaned, "")     // 发布组名（最后清理）

	// 3. 优先提取中文部分（如果存在混合的英文和中文）
	// 匹配中文名称，移除英文部分
	if containsChinese(cleaned) {
		// 如果包含中文，尝试提取纯中文部分或中文为主的部分
		// 使用预编译的正则匹配模式
		if matches := chineseExtractPattern.FindStringSubmatch(cleaned); len(matches) > 1 && containsChinese(matches[1]) {
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

	// 4. 移除季度后缀信息（使用预编译的正则）
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

	// 6. 智能处理点号和特殊字符
	// 移除所有点号（无论是否包含中文）
	cleaned = strings.ReplaceAll(cleaned, ".", "")

	cleaned = strings.ReplaceAll(cleaned, ":", "") // 英文冒号
	cleaned = strings.ReplaceAll(cleaned, "：", "") // 中文冒号
	cleaned = strings.ReplaceAll(cleaned, "·", "") // 中文间隔号

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
