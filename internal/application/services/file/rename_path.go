package file

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/easayliu/alist-aria2-download/pkg/logger"
	strutil "github.com/easayliu/alist-aria2-download/pkg/utils/string"
)

// isTVPath 判断路径是否为TV剧集路径
func (rs *RenameSuggester) isTVPath(fullPath string) bool {
	lowerPath := strings.ToLower(fullPath)
	for rootDir := range tvRootDirs {
		if strings.Contains(lowerPath, "/"+rootDir+"/") {
			return true
		}
	}
	return false
}

// cachePathInfo 解析并缓存路径信息，避免重复解析
func (rs *RenameSuggester) cachePathInfo(info *MediaInfo, fullPath string) {
	if info.pathInfoParsed {
		return
	}
	info.pathShowName, info.pathSeason = rs.extractTVInfoFromPath(fullPath)
	info.pathInfoParsed = true
}

// getPathInfo 获取缓存的路径信息，如果未缓存则解析
func (rs *RenameSuggester) getPathInfo(info *MediaInfo, fullPath string) (showName string, season int) {
	if !info.pathInfoParsed {
		rs.cachePathInfo(info, fullPath)
	}
	return info.pathShowName, info.pathSeason
}

// extractTVInfoFromPath 从路径中提取剧集名和季度
func (rs *RenameSuggester) extractTVInfoFromPath(fullPath string) (showName string, season int) {
	parts := strings.Split(fullPath, "/")

	// 优先尝试使用 tvs 根目录后的第一个路径作为剧名
	showName = rs.extractShowNameAfterTVRoot(parts)

	var candidates []string
	var seasonCandidates []struct {
		name   string
		season int
	}

	seasonDirIndex := -1
	logger.Debug("Extracting TV info from path", "fullPath", fullPath, "parts", parts)

	// 第一遍:找到季度目录的位置
	for i := len(parts) - 1; i >= 0; i-- {
		part := parts[i]
		if rs.shouldSkipPathPart(part) {
			continue
		}

		if strutil.IsSeasonDirectory(part) {
			seasonDirIndex = i
			season = rs.extractSeasonFromDirectory(part)
			if season > 0 {
				break
			}
		}
	}

	// 第二遍:处理所有目录部分
	for i := len(parts) - 1; i >= 0; i-- {
		part := parts[i]
		if rs.shouldSkipPathPart(part) {
			continue
		}

		// 跳过文件名本身
		if strings.Contains(part, ".") && i == len(parts)-1 {
			continue
		}

		// 从季度目录的上一级提取剧集名
		if seasonDirIndex > 0 && i == seasonDirIndex-1 && showName == "" {
			if !rs.isQualityOrFormatDir(part) {
				cleaned := strutil.CleanShowName(part)
				if cleaned != "" && len(cleaned) > 1 {
					showName = cleaned
					logger.Info("Found show name from parent of season directory",
						"seasonDir", parts[seasonDirIndex],
						"showDir", part,
						"cleanedShowName", showName)
					return showName, rs.defaultSeason(season)
				}
			}
		}

		// 处理"剧集名+季度"组合格式
		if showName == "" {
			if name, s := rs.extractFromCombinedDir(part, season); name != "" {
				showName = name
				if s > 0 {
					season = s
				}
				return showName, rs.defaultSeason(season)
			}
		}

		// 处理中文季度格式
		if name, s := rs.extractFromChineseFormat(part, showName); name != "" {
			showName = name
			season = s
			return showName, season
		}

		// 处理合集格式
		if name := rs.extractFromCollectionFormat(part, showName); name != "" {
			showName = name
			continue
		}

		// 提取季度
		if season == 0 {
			season = rs.extractSeasonFromDirectory(part)
		}

		// 跳过质量/格式目录
		if rs.isQualityOrFormatDir(part) {
			continue
		}

		// 收集候选
		rs.collectCandidates(part, &candidates, &seasonCandidates)
	}

	// 从候选列表中选择
	if showName == "" && len(candidates) > 0 {
		showName = candidates[len(candidates)-1]
		logger.Debug("Selected show name from candidates", "showName", showName, "totalCandidates", len(candidates))
	}

	if len(seasonCandidates) > 0 && season == 0 {
		season = seasonCandidates[0].season
	}

	return showName, rs.defaultSeason(season)
}

// shouldSkipPathPart 判断是否应该跳过该路径部分
func (rs *RenameSuggester) shouldSkipPathPart(part string) bool {
	skipParts := map[string]bool{
		"":         true,
		"data":     true,
		"来自：分享":   true,
		"tvs":      true,
		"剧集":       true,
		"电视剧":      true,
	}
	return skipParts[part]
}

// defaultSeason 返回默认季度（如果为0则返回1）
func (rs *RenameSuggester) defaultSeason(season int) int {
	if season == 0 {
		return 1
	}
	return season
}

// extractSeasonFromDirectory 从目录名提取季度
func (rs *RenameSuggester) extractSeasonFromDirectory(part string) int {
	lowerPart := strings.ToLower(part)

	// S01 格式
	if match := strutil.SeasonPattern.FindStringSubmatch(lowerPart); len(match) > 1 {
		if num, err := strconv.Atoi(match[1]); err == nil {
			return num
		}
	}

	// Season 1 格式
	if match := strutil.SeasonEnglishPattern.FindStringSubmatch(lowerPart); len(match) > 1 {
		if num, err := strconv.Atoi(match[1]); err == nil {
			return num
		}
	}

	return 0
}

// extractFromCombinedDir 从组合目录名提取（如 "新闻女王 S2"）
func (rs *RenameSuggester) extractFromCombinedDir(part string, currentSeason int) (string, int) {
	seasonPattern := strutil.SeasonPattern
	if match := seasonPattern.FindStringSubmatch(strings.ToLower(part)); len(match) > 1 {
		season := currentSeason
		if num, err := strconv.Atoi(match[1]); err == nil && season == 0 {
			season = num
		}

		cleaned := strutil.CleanShowName(part)
		if cleaned != "" && len(cleaned) > 1 {
			logger.Info("Found show name from combined directory (show+season)",
				"originalDir", part,
				"cleanedShowName", cleaned,
				"season", season)
			return cleaned, season
		}
	}
	return "", 0
}

// extractFromChineseFormat 从中文格式提取（如 "重影第一季"）
func (rs *RenameSuggester) extractFromChineseFormat(part, currentShowName string) (string, int) {
	if !strings.Contains(part, "第") || (!strings.Contains(part, "季") && !strings.Contains(part, "部")) {
		return "", 0
	}

	seasonRegex := regexp.MustCompile(`第([一二三四五六七八九十\d]+)季`)
	if match := seasonRegex.FindStringSubmatch(part); len(match) > 1 {
		seasonStr := match[1]
		var season int
		if num, ok := chineseNumMap[seasonStr]; ok {
			season = num
		} else if num, err := strconv.Atoi(seasonStr); err == nil {
			season = num
		}

		nameBeforeSeason := strings.Split(part, "第")[0]
		trimmedName := strings.TrimSpace(nameBeforeSeason)
		if trimmedName != "" && currentShowName == "" {
			logger.Debug("Found season in Chinese format", "part", part, "showName", trimmedName, "season", season)
			return trimmedName, season
		}
	}
	return "", 0
}

// ExtractSeasonRange 提取季度范围信息
// 支持格式: "第1-3季"、"第1~3季"、"Season 1-3"、"S01-S03"
// 返回: 剧名、起始季度、结束季度
func (rs *RenameSuggester) ExtractSeasonRange(path string) (showName string, startSeason, endSeason int) {
	parts := strings.Split(path, "/")

	// 尝试各种季度范围格式
	patterns := []struct {
		regex   *regexp.Regexp
		desc    string
		isChNum bool // 是否使用中文数字
	}{
		{regexp.MustCompile(`第(\d+)-(\d+)季`), "第X-Y季", false},
		{regexp.MustCompile(`第(\d+)~(\d+)季`), "第X~Y季", false},
		{regexp.MustCompile(`第(\d+)至(\d+)季`), "第X至Y季", false},
		{regexp.MustCompile(`(?i)season\s*(\d+)-(\d+)`), "Season X-Y", false},
		{regexp.MustCompile(`(?i)s(\d{1,2})-s(\d{1,2})`), "SX-SY", false},
	}

	for i := len(parts) - 1; i >= 0; i-- {
		part := parts[i]

		for _, pattern := range patterns {
			match := pattern.regex.FindStringSubmatch(part)
			if len(match) > 2 {
				start, err1 := strconv.Atoi(match[1])
				end, err2 := strconv.Atoi(match[2])

				if err1 == nil && err2 == nil && start > 0 && end >= start && end <= 20 {
					// 提取剧名(季度信息之前的部分)
					name := pattern.regex.ReplaceAllString(part, "")
					name = strings.TrimSpace(name)

					logger.Info("检测到季度范围",
						"pathPart", part,
						"pattern", pattern.desc,
						"showName", name,
						"startSeason", start,
						"endSeason", end)

					return name, start, end
				}
			}
		}
	}

	return "", 0, 0
}

// extractFromCollectionFormat 从合集格式提取（如 "重影全3季"）
func (rs *RenameSuggester) extractFromCollectionFormat(part, currentShowName string) string {
	if !strings.Contains(part, "全") || !strings.Contains(part, "季") {
		return ""
	}

	collectionRegex := regexp.MustCompile(`^(.+?)\s*全\d+`)
	if match := collectionRegex.FindStringSubmatch(part); len(match) > 1 {
		if currentShowName == "" {
			name := strings.TrimSpace(match[1])
			logger.Info("Detected collection directory", "showName", name, "pathPart", part)
			return name
		}
	}
	return ""
}

// collectCandidates 收集候选剧集名
func (rs *RenameSuggester) collectCandidates(part string, candidates *[]string, seasonCandidates *[]struct{ name string; season int }) {
	if strutil.IsSeasonDirectory(part) || strings.Contains(part, "全") || rs.isQualityOrFormatDir(part) {
		return
	}

	cleaned := strutil.CleanShowName(part)
	if cleaned == "" || len(cleaned) <= 1 {
		return
	}

	seasonNum := rs.extractSeasonFromDirName(part)
	if seasonNum > 0 {
		*seasonCandidates = append(*seasonCandidates, struct{ name string; season int }{cleaned, seasonNum})
		logger.Debug("Found season candidate", "part", part, "cleaned", cleaned, "season", seasonNum)
	} else {
		*candidates = append(*candidates, cleaned)
		logger.Debug("Found show name candidate", "part", part, "cleaned", cleaned)
	}
}

// extractShowNameAfterTVRoot 从TV根目录后提取剧集名
func (rs *RenameSuggester) extractShowNameAfterTVRoot(parts []string) string {
	for i, part := range parts {
		lowerPart := strings.ToLower(part)
		if _, ok := tvRootDirs[lowerPart]; !ok {
			continue
		}

		if i+1 >= len(parts) {
			return ""
		}

		candidate := parts[i+1]
		if candidate == "" || candidate == "." || candidate == ".." {
			return ""
		}

		if rs.isQualityOrFormatDir(candidate) {
			return ""
		}

		cleaned := strutil.CleanShowName(candidate)
		if cleaned != "" && len(cleaned) > 1 {
			logger.Info("Found show name from TV root directory",
				"rootDir", part,
				"showDir", candidate,
				"cleanedShowName", cleaned)
			return cleaned
		}

		return ""
	}

	return ""
}

// extractSeasonFromDirName 从目录名提取季度号
func (rs *RenameSuggester) extractSeasonFromDirName(dirName string) int {
	dirNameLower := strings.ToLower(dirName)

	seasonPatterns := []struct {
		regex *regexp.Regexp
		group int
	}{
		{regexp.MustCompile(`(?i)^season[\s\-_]*(\d+)$`), 1},
		{regexp.MustCompile(`(?i)^s(\d{1,2})$`), 1},
		{regexp.MustCompile(`^(.+?)[\s\-_]*(\d{1,2})$`), 2},
	}

	for i, pattern := range seasonPatterns {
		var match []string
		if i < 2 {
			match = pattern.regex.FindStringSubmatch(dirNameLower)
		} else {
			match = pattern.regex.FindStringSubmatch(dirName)
		}

		if len(match) > pattern.group {
			seasonStr := match[pattern.group]
			if num, err := strconv.Atoi(seasonStr); err == nil && num > 0 && num < 100 {
				return num
			}
		}
	}

	return 0
}

// isQualityOrFormatDir 判断是否为质量/格式目录
func (rs *RenameSuggester) isQualityOrFormatDir(dir string) bool {
	for _, pattern := range rs.qualityDirPatterns {
		matched, _ := regexp.MatchString(pattern, dir)
		if matched {
			return true
		}
	}
	return false
}

// findTVRootDir 查找TV根目录路径
func (rs *RenameSuggester) findTVRootDir(fullPath string) string {
	parts := strings.Split(fullPath, "/")
	for i, part := range parts {
		lowerPart := strings.ToLower(part)
		if _, ok := tvRootDirs[lowerPart]; ok {
			return "/" + filepath.Join(parts[1:i+1]...)
		}
	}
	return ""
}

// buildEmbyPath 构建Emby标准路径
func (rs *RenameSuggester) buildEmbyPath(originalPath, seriesName string, _, season int, fileName string) string {
	tvRootDir := rs.findTVRootDir(originalPath)
	if tvRootDir == "" {
		dir := filepath.Dir(originalPath)
		return filepath.Join(dir, fileName)
	}

	seasonDir := fmt.Sprintf("Season %02d", season)
	return filepath.Join(tvRootDir, seriesName, seasonDir, fileName)
}

// buildMoviePath 构建电影路径（保留原目录）
func (rs *RenameSuggester) buildMoviePath(fullPath, _ string, _ int, fileName string) string {
	dir := filepath.Dir(fullPath)
	return filepath.Join(dir, fileName)
}
