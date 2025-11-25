package file

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/easayliu/alist-aria2-download/internal/infrastructure/tmdb"
	"github.com/easayliu/alist-aria2-download/pkg/logger"
	"github.com/easayliu/alist-aria2-download/pkg/utils/media"
)

// ParseFileName 解析文件名，提取媒体信息
func (rs *RenameSuggester) ParseFileName(fullPath string) *MediaInfo {
	fileName := filepath.Base(fullPath)
	info := &MediaInfo{
		OriginalName: fileName,
		Extension:    filepath.Ext(fileName),
	}

	nameWithoutExt := strings.TrimSuffix(fileName, info.Extension)
	isTVPath := rs.isTVPath(fullPath)

	info.AirDate = rs.extractAirDate(nameWithoutExt)
	info.Version = rs.extractVersion(nameWithoutExt)

	// 提取年份时，先移除分辨率标记避免误匹配（如2160p被识别为年份）
	nameForYear := regexp.MustCompile(`(?i)\d{3,4}[pP]`).ReplaceAllString(nameWithoutExt, "")
	// 限制年份范围为1900-2099
	yearRegex := regexp.MustCompile(`[\[\(]?(19\d{2}|20\d{2})[\]\)]?`)
	if match := yearRegex.FindStringSubmatch(nameForYear); len(match) > 1 {
		if year, err := strconv.Atoi(match[1]); err == nil {
			info.Year = year
		}
	}

	seasonEpisodeRegex := regexp.MustCompile(`[Ss](\d+)[Ee](\d+)`)
	if match := seasonEpisodeRegex.FindStringSubmatch(nameWithoutExt); len(match) > 2 {
		info.Season, _ = strconv.Atoi(match[1])
		info.Episode, _ = strconv.Atoi(match[2])
		info.MediaType = tmdb.MediaTypeTV
		rs.cachePathInfo(info, fullPath)
	} else if isTVPath {
		info.MediaType = tmdb.MediaTypeTV
		rs.cachePathInfo(info, fullPath)
		if info.pathShowName != "" {
			info.Title = info.pathShowName
		}
		if info.pathSeason > 0 {
			info.Season = info.pathSeason
		}

		// 尝试提取集数
		info.Episode = rs.extractEpisodeNumber(nameWithoutExt)

		if info.Episode == 0 {
			episode, part := rs.extractEpisodeAndPart(nameWithoutExt)
			if episode > 0 {
				info.Episode = episode
				info.Part = part
			} else {
				info.Episode = rs.extractNumericEpisode(nameWithoutExt)
			}
		}

		if info.Part == "" {
			info.Part = rs.extractPart(nameWithoutExt)
		}

		if info.Title != "" {
			return info
		}
	} else {
		info.MediaType = tmdb.MediaTypeMovie
	}

	// 清理文件名提取标题
	if info.Title == "" {
		info.Title = rs.cleanFileName(nameWithoutExt, info.Year)
	}

	return info
}

// extractEpisodeNumber 提取集数（E格式）
func (rs *RenameSuggester) extractEpisodeNumber(nameWithoutExt string) int {
	episodeOnlyRegex := regexp.MustCompile(`[Ee](\d+)`)
	if match := episodeOnlyRegex.FindStringSubmatch(nameWithoutExt); len(match) > 1 {
		if ep, err := strconv.Atoi(match[1]); err == nil && ep > 0 {
			logger.Debug("Extracted episode from E format", "episode", ep)
			return ep
		}
	}
	return 0
}

// extractNumericEpisode 提取纯数字集数
func (rs *RenameSuggester) extractNumericEpisode(fileName string) int {
	// 尝试匹配文件名开头的数字
	numericEpisodeRegex := regexp.MustCompile(`^(\d{1,3})(?:[._\-\s]|$)`)
	if match := numericEpisodeRegex.FindStringSubmatch(fileName); len(match) > 1 {
		if episode, err := strconv.Atoi(match[1]); err == nil && episode > 0 && episode < 1000 {
			return episode
		}
	}

	// 尝试匹配文件名末尾的数字（如：小猪佩奇第八季中文22）
	trailingEpisodeRegex := regexp.MustCompile(`(\d{1,3})$`)
	if match := trailingEpisodeRegex.FindStringSubmatch(fileName); len(match) > 1 {
		if episode, err := strconv.Atoi(match[1]); err == nil && episode > 0 && episode < 1000 {
			logger.Debug("Extracted episode from trailing number", "fileName", fileName, "episode", episode)
			return episode
		}
	}

	return 0
}

// extractEpisodeAndPart 提取中文格式的集数和分集
func (rs *RenameSuggester) extractEpisodeAndPart(fileName string) (int, string) {
	if media.IsSpecialContent(fileName) {
		logger.Info("Special content detected, skipping match", "fileName", fileName)
		return 0, ""
	}

	episodeRegex := regexp.MustCompile(`第\s*(\d+)\s*[期集话話]([上中下])?`)
	if match := episodeRegex.FindStringSubmatch(fileName); len(match) > 1 {
		if baseNum, err := strconv.Atoi(match[1]); err == nil {
			part := ""
			if len(match) > 2 && match[2] != "" {
				part = match[2]
			}
			episode := rs.calculateEpisodeNumber(baseNum, part)
			return episode, part
		}
	}

	episodeChineseRegex := regexp.MustCompile(`第\s*([一二三四五六七八九十]+)\s*[期集话話]([上中下])?`)
	if match := episodeChineseRegex.FindStringSubmatch(fileName); len(match) > 1 {
		if baseNum, ok := chineseNumMap[match[1]]; ok {
			part := ""
			if len(match) > 2 && match[2] != "" {
				part = match[2]
			}
			episode := rs.calculateEpisodeNumber(baseNum, part)
			return episode, part
		}
	}

	return 0, ""
}

// calculateEpisodeNumber 计算带分集的实际集数
func (rs *RenameSuggester) calculateEpisodeNumber(baseNum int, part string) int {
	if part == "" {
		return baseNum
	}

	partOffset := 0
	switch part {
	case "上":
		partOffset = 0
	case "中":
		partOffset = 1
	case "下":
		partOffset = 2
	}

	return (baseNum-1)*3 + partOffset + 1
}

// extractAirDate 提取播出日期
func (rs *RenameSuggester) extractAirDate(fileName string) string {
	dateRegex := regexp.MustCompile(`(\d{4})[\-\.]?(\d{2})[\-\.]?(\d{2})期?`)
	if match := dateRegex.FindStringSubmatch(fileName); len(match) > 3 {
		return fmt.Sprintf("%s-%s-%s", match[1], match[2], match[3])
	}
	return ""
}

// extractPart 提取分集标记（上/中/下）
func (rs *RenameSuggester) extractPart(fileName string) string {
	partRegex := regexp.MustCompile(`[.\-_\s\(（]([上中下])[.\-_\s\)）]?`)
	if match := partRegex.FindStringSubmatch(fileName); len(match) > 1 {
		return match[1]
	}
	return ""
}

// extractVersion 提取版本信息
func (rs *RenameSuggester) extractVersion(fileName string) string {
	versionPatterns := []string{
		"沉浸版", "加长版", "未删减版", "导演剪辑版", "特别版",
		"完整版", "修复版", "重制版", "精华版", "会员版",
	}

	for _, pattern := range versionPatterns {
		if strings.Contains(fileName, pattern) {
			return pattern
		}
	}
	return ""
}

// cleanFileName 清理文件名，提取标题
func (rs *RenameSuggester) cleanFileName(nameWithoutExt string, year int) string {
	seasonEpisodeRegex := regexp.MustCompile(`[Ss](\d+)[Ee](\d+)`)
	cleanedName := seasonEpisodeRegex.ReplaceAllString(nameWithoutExt, "")

	removePatterns := []string{
		`\d{3,4}[pP]`,
		`\d+fps`, `\d+帧`,
		`UHD`, `FHD`, `4K`, `2K`, `HQ`,
		`BluRay`, `Blu-?ray`, `WEB-?DL`, `WEBRip`, `HDRip`, `BDRip`, `DVDRip`, `BD\d*`,
		`x264`, `x265`, `H\.?264`, `H\.?265`, `HEVC`, `AVC`, `X\.?264`, `X\.?265`,
		`AAC[\d.]*`, `AC3`, `DTS-?[XMA]*[\d.]*`, `DD[P]?[\d.]*`, `Atmos`, `TrueHD[\d.]*`, `MA[\d.]*`,
		`10bit`, `8bit`, `HDR\d*`, `DoVi`, `DV`,
		`MultiAudio`, `Dual[\s-]?Audio`, `Multi`, `Mandarin`, `English`, `CHS`, `CHT`, `ENG`,
		`&[A-Za-z]+`,
		`-[A-Z][a-zA-Z0-9]+$`,
	}

	// 如果有中文，尝试提取英文名
	hasChinese := regexp.MustCompile(`[\p{Han}]`).MatchString(cleanedName)
	if hasChinese {
		englishParts := regexp.MustCompile(`[A-Za-z0-9]+(?:[\s.][A-Za-z0-9]+)*`).FindAllString(cleanedName, -1)
		var longestEnglish string
		for _, part := range englishParts {
			trimmed := strings.TrimSpace(part)
			if len(trimmed) > len(longestEnglish) {
				longestEnglish = trimmed
			}
		}
		if len(longestEnglish) > 5 {
			cleanedName = longestEnglish
		}
	}

	// 移除技术标记
	for _, pattern := range removePatterns {
		re := regexp.MustCompile(`(?i)` + pattern)
		cleanedName = re.ReplaceAllString(cleanedName, " ")
	}

	cleanedName = regexp.MustCompile(`\s+bit\b`).ReplaceAllString(cleanedName, "")

	// 移除年份
	if year > 0 {
		yearPatterns := []string{
			fmt.Sprintf(`[\.\s_-]*\(?%d\)?[\.\s_-]*`, year),
			fmt.Sprintf(`[\.\s_-]+%d[\.\s_-]*`, year),
		}
		for _, pattern := range yearPatterns {
			re := regexp.MustCompile(pattern)
			cleanedName = re.ReplaceAllString(cleanedName, " ")
		}
	}

	cleanRegex := regexp.MustCompile(`[._\-\s]+`)
	return strings.TrimSpace(cleanRegex.ReplaceAllString(cleanedName, " "))
}
