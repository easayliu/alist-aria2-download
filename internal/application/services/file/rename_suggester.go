package file

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/easayliu/alist-aria2-download/internal/domain/models/rename"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/tmdb"
	"github.com/easayliu/alist-aria2-download/pkg/logger"
	"github.com/easayliu/alist-aria2-download/pkg/utils/media"
	strutil "github.com/easayliu/alist-aria2-download/pkg/utils/string"
)

type RenameSuggester struct {
	tmdbClient         *tmdb.Client
	qualityDirPatterns []string
}

func NewRenameSuggester(tmdbClient *tmdb.Client, qualityDirPatterns []string) *RenameSuggester {
	return &RenameSuggester{
		tmdbClient:         tmdbClient,
		qualityDirPatterns: qualityDirPatterns,
	}
}

type MediaInfo struct {
	OriginalName string
	MediaType    tmdb.MediaType
	Title        string
	Year         int
	Season       int
	Episode      int
	Part         string
	Extension    string
	AirDate      string
	Version      string
}

func (rs *RenameSuggester) ParseFileName(fullPath string) *MediaInfo {
	fileName := filepath.Base(fullPath)
	info := &MediaInfo{
		OriginalName: fileName,
		Extension:    filepath.Ext(fileName),
	}

	nameWithoutExt := strings.TrimSuffix(fileName, info.Extension)
	lowerPath := strings.ToLower(fullPath)

	isTVPath := strings.Contains(lowerPath, "/tvs/") ||
		strings.Contains(lowerPath, "/tv shows/") ||
		strings.Contains(lowerPath, "/剧集/") ||
		strings.Contains(lowerPath, "/电视剧/")

	airDate := rs.extractAirDate(nameWithoutExt)
	if airDate != "" {
		info.AirDate = airDate
	}

	version := rs.extractVersion(nameWithoutExt)
	if version != "" {
		info.Version = version
	}

	yearRegex := regexp.MustCompile(`[\[\(]?(\d{4})[\]\)]?`)
	if match := yearRegex.FindStringSubmatch(nameWithoutExt); len(match) > 1 {
		if year, err := strconv.Atoi(match[1]); err == nil {
			info.Year = year
		}
	}

	seasonEpisodeRegex := regexp.MustCompile(`[Ss](\d+)[Ee](\d+)`)
	if match := seasonEpisodeRegex.FindStringSubmatch(nameWithoutExt); len(match) > 2 {
		info.Season, _ = strconv.Atoi(match[1])
		info.Episode, _ = strconv.Atoi(match[2])
		info.MediaType = tmdb.MediaTypeTV
	} else if isTVPath {
		info.MediaType = tmdb.MediaTypeTV
		showName, season := rs.extractTVInfoFromPath(fullPath)
		if showName != "" {
			info.Title = showName
		}
		if season > 0 {
			info.Season = season
		}
		episode, part := rs.extractEpisodeAndPart(nameWithoutExt)
		if episode > 0 {
			info.Episode = episode
			info.Part = part
		} else {
			episode = rs.extractNumericEpisode(nameWithoutExt)
			if episode > 0 {
				info.Episode = episode
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

	cleanRegex := regexp.MustCompile(`[._\-\s]+`)
	if info.Title == "" {
		cleanedName := nameWithoutExt
		cleanedName = seasonEpisodeRegex.ReplaceAllString(cleanedName, "")

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

		for _, pattern := range removePatterns {
			re := regexp.MustCompile(`(?i)` + pattern)
			cleanedName = re.ReplaceAllString(cleanedName, " ")
		}

		cleanedName = regexp.MustCompile(`\s+bit\b`).ReplaceAllString(cleanedName, "")

		if info.Year > 0 {
			yearPatterns := []string{
				fmt.Sprintf(`[\.\s_-]*\(?%d\)?[\.\s_-]*`, info.Year),
				fmt.Sprintf(`[\.\s_-]+%d[\.\s_-]*`, info.Year),
			}
			for _, pattern := range yearPatterns {
				re := regexp.MustCompile(pattern)
				cleanedName = re.ReplaceAllString(cleanedName, " ")
			}
		}

		info.Title = strings.TrimSpace(cleanRegex.ReplaceAllString(cleanedName, " "))
	}

	return info
}

func (rs *RenameSuggester) SearchAndSuggest(ctx context.Context, fullPath string) ([]rename.Suggestion, error) {
	info := rs.ParseFileName(fullPath)

	// 对于TV剧集,优先从路径中提取剧集名和季度
	if info.MediaType == tmdb.MediaTypeTV {
		showName, pathSeason := rs.extractTVInfoFromPath(fullPath)

		// 如果从路径中成功提取了剧集名,使用它替代从文件名提取的标题
		if showName != "" {
			logger.Info("使用从路径提取的剧集名",
				"originalTitle", info.Title,
				"pathShowName", showName,
				"pathSeason", pathSeason,
				"fileNameSeason", info.Season)
			info.Title = showName
		}

		// 如果从路径中提取了季度,优先使用路径季度
		if pathSeason > 0 {
			info.Season = pathSeason
		}

		// 重置年份(避免从文件名中错误提取的年份,如2160p被识别为年份)
		// TV剧集的年份应该从TMDB查询结果中获取
		info.Year = 0
	}

	logger.Info("TMDB search started",
		"path", fullPath,
		"title", info.Title,
		"mediaType", info.MediaType,
		"season", info.Season,
		"episode", info.Episode,
		"part", info.Part,
		"airDate", info.AirDate,
		"version", info.Version,
		"year", info.Year)

	if info.MediaType == tmdb.MediaTypeTV {
		return rs.suggestTVName(ctx, fullPath, info)
	}
	return rs.suggestMovieName(ctx, fullPath, info)
}

func (rs *RenameSuggester) suggestMovieName(ctx context.Context, fullPath string, info *MediaInfo) ([]rename.Suggestion, error) {
	resp, err := rs.tmdbClient.SearchMovie(ctx, info.Title, info.Year)
	if err != nil {
		return nil, fmt.Errorf("failed to search movie: %w", err)
	}

	if len(resp.Results) == 0 {
		return nil, fmt.Errorf("TMDB数据库中未找到电影 '%s'，可能是因为：\n1. 电影名称不准确\n2. TMDB未收录该影片\n3. 需要使用英文名称搜索", info.Title)
	}

	suggestions := make([]rename.Suggestion, 0, len(resp.Results))
	for i, result := range resp.Results {
		year := 0
		if result.ReleaseDate != "" {
			if parsedYear, err := strconv.Atoi(result.ReleaseDate[:4]); err == nil {
				year = parsedYear
			}
		}

		confidence := 1.0 - (float64(i) * 0.1)
		if info.Year > 0 && year == info.Year {
			confidence += 0.2
		}

		details, err := rs.tmdbClient.GetMovieDetails(ctx, result.ID)
		if err != nil {
			logger.Warn("Failed to get movie details", "movieID", result.ID, "title", result.Title, "error", err)
			newName := fmt.Sprintf("%s (%d)%s", result.Title, year, info.Extension)
			newPath := rs.buildMoviePath(fullPath, result.Title, year, newName)

			suggestions = append(suggestions, rename.Suggestion{
				NewName:    newName,
				NewPath:    newPath,
				MediaType:  rename.FromTMDBMediaType(tmdb.MediaTypeMovie),
				TMDBID:     result.ID,
				Title:      result.Title,
				Year:       year,
				Confidence: confidence,
				Source:     rename.SourceTMDB,
			})
			continue
		}

		title := details.Title
		if details.OriginalTitle != "" && details.OriginalLanguage != "en" {
			title = details.OriginalTitle
		}

		newName := fmt.Sprintf("%s (%d)%s", title, year, info.Extension)
		newPath := rs.buildMoviePath(fullPath, title, year, newName)

		logger.Info("Generated movie rename suggestion",
			"originalPath", fullPath,
			"newName", newName,
			"newPath", newPath,
			"tmdbID", details.ID,
			"title", title,
			"originalTitle", details.OriginalTitle,
			"year", year,
			"runtime", details.Runtime)

		sug := rename.Suggestion{
			NewName:    newName,
			NewPath:    newPath,
			MediaType:  rename.FromTMDBMediaType(tmdb.MediaTypeMovie),
			TMDBID:     details.ID,
			Title:      title,
			Year:       year,
			Confidence: confidence,
			Source:     rename.SourceTMDB,
		}
		suggestions = append(suggestions, sug)
	}

	return suggestions, nil
}

func (rs *RenameSuggester) suggestTVName(ctx context.Context, fullPath string, info *MediaInfo) ([]rename.Suggestion, error) {
	searchQuery := info.Title
	if info.Version != "" {
		searchQuery = fmt.Sprintf("%s %s", info.Title, info.Version)
		logger.Info("Version detected, using full name for search", "originalTitle", info.Title, "version", info.Version, "searchQuery", searchQuery)
	}

	return rs.searchTVByQuery(ctx, fullPath, info, searchQuery, info.Version != "")
}

func (rs *RenameSuggester) searchTVByQuery(ctx context.Context, fullPath string, info *MediaInfo, query string, isVersionSearch bool) ([]rename.Suggestion, error) {
	logger.Info("Searching TMDB TV series", "query", query, "year", info.Year, "season", info.Season)

	resp, err := rs.tmdbClient.SearchTV(ctx, query, info.Year)
	if err != nil {
		logger.Error("TMDB API call failed", "query", query, "error", err)
		return nil, fmt.Errorf("TMDB搜索失败: %w", err)
	}

	logger.Info("TMDB search results", "query", query, "resultCount", len(resp.Results))

	if len(resp.Results) == 0 {
		return nil, fmt.Errorf("TMDB数据库中未找到剧集 '%s' (Season %d)，可能原因：\n1. 剧集名称提取不准确\n2. TMDB未收录该节目（如部分综艺、国产剧）\n3. 需要使用英文或原始名称\n\n建议：使用/rename命令时手动指定完整文件名", query, info.Season)
	}

	suggestions := make([]rename.Suggestion, 0, len(resp.Results))
	for i, result := range resp.Results {
		year := 0
		if result.FirstAirDate != "" {
			if parsedYear, err := strconv.Atoi(result.FirstAirDate[:4]); err == nil {
				year = parsedYear
			}
		}

		confidence := 1.0 - (float64(i) * 0.1)
		if info.Year > 0 && year == info.Year {
			confidence += 0.2
		}

		seasonDetails, err := rs.tmdbClient.GetSeasonDetails(ctx, result.ID, info.Season)
		if err != nil {
			logger.Warn("Failed to get season details", "tvID", result.ID, "name", result.Name, "season", info.Season, "error", err)
			continue
		}

		logger.Info("Found matching season",
			"name", result.Name,
			"season", seasonDetails.SeasonNumber,
			"episodeCount", seasonDetails.EpisodeCount)

		matchedEpisode := info.Episode
		if info.AirDate != "" {
			var sameDateEpisodes []tmdb.Episode
			for _, ep := range seasonDetails.Episodes {
				if ep.AirDate == info.AirDate {
					sameDateEpisodes = append(sameDateEpisodes, ep)
				}
			}

			if len(sameDateEpisodes) > 0 {
				selectedEpisode := sameDateEpisodes[0]

				if info.Part != "" && len(sameDateEpisodes) > 1 {
					partIndex := rs.getPartIndex(info.Part, len(sameDateEpisodes))

					if partIndex < len(sameDateEpisodes) {
						selectedEpisode = sameDateEpisodes[partIndex]
						logger.Info("Matched episode by air date and part",
							"airDate", info.AirDate,
							"part", info.Part,
							"totalEpisodes", len(sameDateEpisodes),
							"partIndex", partIndex,
							"episode", selectedEpisode.EpisodeNumber,
							"episodeName", selectedEpisode.Name)
					}
				} else {
					if len(sameDateEpisodes) > 1 && info.Part == "" {
						logger.Warn("Multiple episodes on same air date without part specified, selecting first episode",
							"airDate", info.AirDate,
							"episodeCount", len(sameDateEpisodes),
							"selectedEpisode", selectedEpisode.EpisodeNumber)
					} else {
						logger.Info("Matched episode by air date",
							"airDate", info.AirDate,
							"episode", selectedEpisode.EpisodeNumber,
							"episodeName", selectedEpisode.Name)
					}
				}

				matchedEpisode = selectedEpisode.EpisodeNumber
			}
		}

		if matchedEpisode > seasonDetails.EpisodeCount {
			logger.Warn("Episode number out of range",
				"name", result.Name,
				"season", info.Season,
				"requestedEpisode", matchedEpisode,
				"maxEpisode", seasonDetails.EpisodeCount)
			continue
		}

		var episodeName string
		if len(seasonDetails.Episodes) > 0 && matchedEpisode > 0 && matchedEpisode <= len(seasonDetails.Episodes) {
			episodeName = seasonDetails.Episodes[matchedEpisode-1].Name
		}

		displayName := query
		newName := fmt.Sprintf("%s - S%02dE%02d", displayName, info.Season, matchedEpisode)
		if episodeName != "" {
			newName += fmt.Sprintf(" - %s", episodeName)
		}
		newName += info.Extension

		newPath := rs.buildEmbyPath(fullPath, displayName, year, info.Season, newName)

		logger.Info("Generated rename suggestion",
			"originalPath", fullPath,
			"newName", newName,
			"newPath", newPath,
			"tmdbID", result.ID,
			"query", query,
			"isVersionSearch", isVersionSearch,
			"season", info.Season,
			"episode", matchedEpisode)

		sug := rename.Suggestion{
			NewName:    newName,
			NewPath:    newPath,
			MediaType:  rename.FromTMDBMediaType(tmdb.MediaTypeTV),
			TMDBID:     result.ID,
			Title:      displayName,
			Year:       year,
			Confidence: confidence,
			Source:     rename.SourceTMDB,
		}
		sug.SetSeason(info.Season)
		sug.SetEpisode(matchedEpisode)
		suggestions = append(suggestions, sug)
	}

	if len(suggestions) == 0 {
		return nil, fmt.Errorf("未找到包含第 %d 季的剧集 '%s'", info.Season, query)
	}

	return suggestions, nil
}

func (rs *RenameSuggester) extractTVInfoFromPath(fullPath string) (showName string, season int) {
	parts := strings.Split(fullPath, "/")

	var candidates []string
	var seasonCandidates []struct {
		name   string
		season int
	}

	// 用于记录季度目录的索引
	seasonDirIndex := -1

	logger.Debug("Extracting TV info from path", "fullPath", fullPath, "parts", parts)

	// 第一遍:找到季度目录的位置
	for i := len(parts) - 1; i >= 0; i-- {
		part := parts[i]

		if part == "" || part == "data" || part == "来自：分享" || part == "tvs" || part == "剧集" || part == "电视剧" {
			continue
		}

		// 检查是否是纯季度目录(如 S05, Season 5)
		if strutil.IsSeasonDirectory(part) {
			seasonDirIndex = i
			// 尝试提取季度数字
			seasonPattern := strutil.SeasonPattern
			if match := seasonPattern.FindStringSubmatch(strings.ToLower(part)); len(match) > 1 {
				if num, err := strconv.Atoi(match[1]); err == nil {
					season = num
					logger.Debug("Found season directory", "part", part, "season", season, "index", i)
					break
				}
			}
		}
	}

	// 第二遍:处理所有目录部分
	for i := len(parts) - 1; i >= 0; i-- {
		part := parts[i]

		if part == "" || part == "data" || part == "来自：分享" || part == "tvs" || part == "剧集" || part == "电视剧" {
			continue
		}

		// 跳过文件名本身
		if strings.Contains(part, ".") && i == len(parts)-1 {
			continue
		}

		// 如果找到了季度目录,优先从季度目录的上一级提取剧集名
		if seasonDirIndex > 0 && i == seasonDirIndex-1 {
			// 这个目录应该是剧集名目录
			if !rs.isQualityOrFormatDir(part) {
				// 直接使用这个目录名作为剧集名(清理后)
				cleaned := strutil.CleanShowName(part)
				if cleaned != "" && len(cleaned) > 1 {
					showName = cleaned
					logger.Info("Found show name from parent of season directory",
						"seasonDir", parts[seasonDirIndex],
						"showDir", part,
						"cleanedShowName", showName)
					// 找到了剧集名,可以直接返回
					if season == 0 {
						season = 1
					}
					return
				}
			}
		}

		// 处理中文季度格式(如 "重影第一季")
		if strings.Contains(part, "第") && (strings.Contains(part, "季") || strings.Contains(part, "部")) {
			chineseNumMap := map[string]int{
				"一": 1, "二": 2, "三": 3, "四": 4, "五": 5,
				"六": 6, "七": 7, "八": 8, "九": 9, "十": 10,
			}

			seasonRegex := regexp.MustCompile(`第([一二三四五六七八九十\d]+)季`)
			if match := seasonRegex.FindStringSubmatch(part); len(match) > 1 {
				seasonStr := match[1]
				if num, ok := chineseNumMap[seasonStr]; ok {
					season = num
				} else if num, err := strconv.Atoi(seasonStr); err == nil {
					season = num
				}

				nameBeforeSeason := strings.Split(part, "第")[0]
				if strings.TrimSpace(nameBeforeSeason) != "" {
					showName = strings.TrimSpace(nameBeforeSeason)
					logger.Debug("Found season in Chinese format", "part", part, "showName", showName, "season", season)
					return
				}
				continue
			}
		}

		// 处理合集格式(如 "重影全3季")
		if strings.Contains(part, "全") && strings.Contains(part, "季") {
			collectionRegex := regexp.MustCompile(`^(.+?)\s*全\d+`)
			if match := collectionRegex.FindStringSubmatch(part); len(match) > 1 {
				showName = strings.TrimSpace(match[1])
				season = 0
				logger.Info("Detected collection directory", "showName", showName, "pathPart", part)
				continue
			}
		}

		// 如果还没找到季度,继续从目录名中提取
		if season == 0 {
			seasonPattern := strutil.SeasonPattern
			if match := seasonPattern.FindStringSubmatch(strings.ToLower(part)); len(match) > 1 {
				if num, err := strconv.Atoi(match[1]); err == nil {
					season = num
					logger.Debug("Found season with SeasonPattern", "part", part, "season", season)
				}
			}
		}

		// 跳过质量/格式目录
		if rs.isQualityOrFormatDir(part) {
			continue
		}

		// 检查是否是季度目录(第二次检查,针对非纯季度目录)
		seasonPattern := strutil.SeasonStrictPattern
		if match := seasonPattern.FindStringSubmatch(strings.ToLower(part)); len(match) > 1 {
			if num, err := strconv.Atoi(match[1]); err == nil && season == 0 {
				season = num
				logger.Debug("Found season with SeasonStrictPattern", "part", part, "season", season)
			}
			continue
		}

		// 收集可能的剧集名候选
		if !strutil.IsSeasonDirectory(part) && !strings.Contains(part, "全") && !rs.isQualityOrFormatDir(part) {
			cleaned := strutil.CleanShowName(part)
			if cleaned != "" && len(cleaned) > 1 {
				seasonNum := rs.extractSeasonFromDirName(part)
				if seasonNum > 0 {
					seasonCandidates = append(seasonCandidates, struct {
						name   string
						season int
					}{cleaned, seasonNum})
					logger.Debug("Found season candidate", "part", part, "cleaned", cleaned, "season", seasonNum)
				} else {
					candidates = append(candidates, cleaned)
					logger.Debug("Found show name candidate", "part", part, "cleaned", cleaned)
				}
			}
		}
	}

	// 如果还没有找到剧集名,从候选列表中选择
	if showName == "" && len(candidates) > 0 {
		showName = candidates[len(candidates)-1]
		logger.Debug("Selected show name from candidates", "showName", showName, "totalCandidates", len(candidates))
	}

	// 如果有季度候选且还没确定季度,使用第一个季度候选
	if len(seasonCandidates) > 0 && season == 0 {
		bottomCandidate := seasonCandidates[0]
		season = bottomCandidate.season
		logger.Debug("Extracted season from directory",
			"showName", showName,
			"season", season,
			"seasonDirName", bottomCandidate.name)
	}

	// 默认季度为1
	if season == 0 {
		season = 1
		logger.Debug("Defaulting to season 1", "showName", showName)
	}

	logger.Debug("Final extraction result", "showName", showName, "season", season)
	return
}

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
				logger.Debug("Extracted season from directory name",
					"dirName", dirName,
					"pattern", i,
					"season", num)
				return num
			}
		}
	}

	return 0
}

func (rs *RenameSuggester) isQualityOrFormatDir(dir string) bool {
	for _, pattern := range rs.qualityDirPatterns {
		matched, _ := regexp.MatchString(pattern, dir)
		if matched {
			return true
		}
	}
	return false
}

func (rs *RenameSuggester) extractNumericEpisode(fileName string) int {
	numericEpisodeRegex := regexp.MustCompile(`^(\d{1,3})(?:[._\-\s]|$)`)
	if match := numericEpisodeRegex.FindStringSubmatch(fileName); len(match) > 1 {
		if episode, err := strconv.Atoi(match[1]); err == nil && episode > 0 && episode < 1000 {
			return episode
		}
	}
	return 0
}

func (rs *RenameSuggester) extractEpisodeAndPart(fileName string) (int, string) {
	if media.IsSpecialContent(fileName) {
		logger.Info("Special content detected, skipping match", "fileName", fileName)
		return 0, ""
	}

	chineseNumMap := map[string]int{
		"一": 1, "二": 2, "三": 3, "四": 4, "五": 5,
		"六": 6, "七": 7, "八": 8, "九": 9, "十": 10,
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

func (rs *RenameSuggester) extractAirDate(fileName string) string {
	dateRegex := regexp.MustCompile(`(\d{4})[\-\.]?(\d{2})[\-\.]?(\d{2})期?`)
	if match := dateRegex.FindStringSubmatch(fileName); len(match) > 3 {
		year := match[1]
		month := match[2]
		day := match[3]
		return fmt.Sprintf("%s-%s-%s", year, month, day)
	}
	return ""
}

func (rs *RenameSuggester) extractPart(fileName string) string {
	partRegex := regexp.MustCompile(`[.\-_\s\(（]([上中下])[.\-_\s\)）]?`)
	if match := partRegex.FindStringSubmatch(fileName); len(match) > 1 {
		return match[1]
	}
	return ""
}

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

func (rs *RenameSuggester) getPartIndex(part string, totalEpisodes int) int {
	switch totalEpisodes {
	case 2:
		switch part {
		case "上":
			return 0
		case "下":
			return 1
		default:
			return 0
		}
	case 3:
		switch part {
		case "上":
			return 0
		case "中":
			return 1
		case "下":
			return 2
		default:
			return 0
		}
	default:
		switch part {
		case "上":
			return 0
		case "中":
			if totalEpisodes > 2 {
				return 1
			}
			return 0
		case "下":
			if totalEpisodes > 1 {
				return totalEpisodes - 1
			}
			return 0
		default:
			return 0
		}
	}
}

func (rs *RenameSuggester) buildEmbyPath(originalPath string, seriesName string, year, season int, fileName string) string {
	// 保留原目录，只修改文件名
	dir := filepath.Dir(originalPath)
	return filepath.Join(dir, fileName)
}

func (rs *RenameSuggester) buildMoviePath(fullPath, movieTitle string, year int, fileName string) string {
	// 保留原目录，只修改文件名
	dir := filepath.Dir(fullPath)
	return filepath.Join(dir, fileName)
}

func (rs *RenameSuggester) BatchSuggestTVNames(ctx context.Context, paths []string) (map[string][]rename.Suggestion, error) {
	if len(paths) == 0 {
		return make(map[string][]rename.Suggestion), nil
	}

	firstTVPath := ""
	for _, path := range paths {
		info := rs.ParseFileName(path)
		if info.MediaType == tmdb.MediaTypeTV {
			firstTVPath = path
			break
		}
	}

	if firstTVPath == "" {
		logger.Info("No TV path detected, trying to extract show name from path", "firstPath", paths[0])
		firstTVPath = paths[0]
	}

	showName, _ := rs.extractTVInfoFromPath(firstTVPath)
	if showName == "" {
		info := rs.ParseFileName(firstTVPath)
		showName = info.Title
	}

	if showName == "" {
		return nil, fmt.Errorf("无法从路径中提取节目名称")
	}

	logger.Info("Batch rename: extracted show name", "showName", showName, "referencePath", firstTVPath)

	pathsByVersion := make(map[string][]string)
	for _, path := range paths {
		info := rs.ParseFileName(path)
		version := info.Version
		pathsByVersion[version] = append(pathsByVersion[version], path)
	}

	result := make(map[string][]rename.Suggestion)

	for version, versionPaths := range pathsByVersion {
		searchQuery := showName
		if version != "" {
			searchQuery = fmt.Sprintf("%s %s", showName, version)
			logger.Info("Batch rename: processing version files", "version", version, "searchQuery", searchQuery, "fileCount", len(versionPaths))
		} else {
			logger.Info("Batch rename: processing regular files", "searchQuery", searchQuery, "fileCount", len(versionPaths))
		}

		seasonMap := make(map[int][]string)
		for _, path := range versionPaths {
			_, pathSeason := rs.extractTVInfoFromPath(path)
			info := rs.ParseFileName(path)

			detectedSeason := pathSeason
			seasonSource := "path"
			if info.Season > 0 {
				detectedSeason = info.Season
				seasonSource = "filename"
			}

			logger.Info("Season detection detail",
				"path", path,
				"pathSeason", pathSeason,
				"fileNameSeason", info.Season,
				"finalSeason", detectedSeason,
				"seasonSource", seasonSource,
				"episode", info.Episode)

			if detectedSeason > 0 {
				seasonMap[detectedSeason] = append(seasonMap[detectedSeason], path)
			} else {
				seasonMap[1] = append(seasonMap[1], path)
			}
		}

		logger.Info("Season distribution summary", "searchQuery", searchQuery, "seasonMap", func() string {
			var parts []string
			for s, paths := range seasonMap {
				parts = append(parts, fmt.Sprintf("S%02d: %d files", s, len(paths)))
			}
			sort.Strings(parts)
			return strings.Join(parts, ", ")
		}())

		versionResults, err := rs.batchSearchTVByQuery(ctx, searchQuery, seasonMap)
		if err != nil {
			logger.Warn("Batch rename: search failed", "query", searchQuery, "version", version, "error", err)
			continue
		}

		for path, suggestions := range versionResults {
			result[path] = append(result[path], suggestions...)
		}
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("TV series '%s' not found in TMDB database", showName)
	}

	return result, nil
}

func (rs *RenameSuggester) batchSearchTVByQuery(ctx context.Context, query string, seasonMap map[int][]string) (map[string][]rename.Suggestion, error) {
	totalFiles := 0
	for _, paths := range seasonMap {
		totalFiles += len(paths)
	}

	logger.Info("Batch searching TMDB TV series", "query", query, "seasonCount", len(seasonMap), "fileCount", totalFiles)

	resp, err := rs.tmdbClient.SearchTV(ctx, query, 0)
	if err != nil {
		return nil, fmt.Errorf("TMDB搜索失败: %w", err)
	}

	if len(resp.Results) == 0 {
		yearRegex := regexp.MustCompile(`\s+\d{4}$`)
		if yearRegex.MatchString(query) {
			showNameWithoutYear := yearRegex.ReplaceAllString(query, "")
			logger.Info("Retry search without year", "originalQuery", query, "newQuery", showNameWithoutYear)

			resp, err = rs.tmdbClient.SearchTV(ctx, showNameWithoutYear, 0)
			if err != nil {
				return nil, fmt.Errorf("TMDB搜索失败: %w", err)
			}

			if len(resp.Results) == 0 {
				return nil, fmt.Errorf("TMDB数据库中未找到剧集 '%s'", showNameWithoutYear)
			}
		} else {
			return nil, fmt.Errorf("TMDB数据库中未找到剧集 '%s'", query)
		}
	}

	result := make(map[string][]rename.Suggestion)

	for _, tvResult := range resp.Results {
		year := 0
		if tvResult.FirstAirDate != "" && len(tvResult.FirstAirDate) >= 4 {
			if parsedYear, err := strconv.Atoi(tvResult.FirstAirDate[:4]); err == nil {
				year = parsedYear
			}
		}

		successCount := 0

		for season, seasonPaths := range seasonMap {
			seasonDetails, err := rs.tmdbClient.GetSeasonDetails(ctx, tvResult.ID, season)
			if err != nil {
				logger.Warn("Failed to get season details", "tvID", tvResult.ID, "query", query, "season", season, "error", err)
				continue
			}

			logger.Info("Got season details", "query", query, "season", season, "episodeCount", len(seasonDetails.Episodes))

			episodeMap := make(map[int]*tmdb.Episode)
			for i := range seasonDetails.Episodes {
				ep := &seasonDetails.Episodes[i]
				episodeMap[ep.EpisodeNumber] = ep
			}

			for _, path := range seasonPaths {
				info := rs.ParseFileName(path)

				matchedEpisode := info.Episode
				if info.AirDate != "" {
					var sameDateEpisodes []tmdb.Episode
					for _, ep := range seasonDetails.Episodes {
						if ep.AirDate == info.AirDate {
							sameDateEpisodes = append(sameDateEpisodes, ep)
						}
					}

					if len(sameDateEpisodes) > 0 {
						selectedEpisode := sameDateEpisodes[0]

						if info.Part != "" && len(sameDateEpisodes) > 1 {
							partIndex := rs.getPartIndex(info.Part, len(sameDateEpisodes))

							if partIndex < len(sameDateEpisodes) {
								selectedEpisode = sameDateEpisodes[partIndex]
								logger.Info("Batch rename: matched episode by air date and part",
									"path", path,
									"airDate", info.AirDate,
									"part", info.Part,
									"totalEpisodes", len(sameDateEpisodes),
									"partIndex", partIndex,
									"episode", selectedEpisode.EpisodeNumber,
									"episodeName", selectedEpisode.Name)
							}
						} else {
							if len(sameDateEpisodes) > 1 && info.Part == "" {
								logger.Warn("Batch rename: multiple episodes on same air date without part specified, selecting first episode",
									"path", path,
									"airDate", info.AirDate,
									"episodeCount", len(sameDateEpisodes),
									"selectedEpisode", selectedEpisode.EpisodeNumber)
							} else {
								logger.Info("Batch rename: matched episode by air date",
									"path", path,
									"airDate", info.AirDate,
									"episode", selectedEpisode.EpisodeNumber,
									"episodeName", selectedEpisode.Name)
							}
						}

						matchedEpisode = selectedEpisode.EpisodeNumber
					}
				}

				if episode, exists := episodeMap[matchedEpisode]; exists {
					displayName := query
					newName := fmt.Sprintf("%s - S%02dE%02d", displayName, season, matchedEpisode)
					if episode.Name != "" {
						newName += fmt.Sprintf(" - %s", episode.Name)
					}
					newName += info.Extension

					newPath := rs.buildEmbyPath(path, displayName, year, season, newName)

					logger.Info("Batch rename: generated rename suggestion",
						"originalPath", path,
						"newName", newName,
						"newPath", newPath,
						"tmdbID", tvResult.ID,
						"query", query,
						"season", season,
						"episode", matchedEpisode)

					sug := rename.Suggestion{
						NewName:    newName,
						NewPath:    newPath,
						MediaType:  rename.FromTMDBMediaType(tmdb.MediaTypeTV),
						TMDBID:     tvResult.ID,
						Title:      displayName,
						Year:       year,
						Confidence: 1.0,
						Source:     rename.SourceTMDB,
					}
					sug.SetSeason(season)
					sug.SetEpisode(matchedEpisode)
					result[path] = append(result[path], sug)
					successCount++
				}
			}
		}

		if successCount > 0 {
			break
		}
	}

	return result, nil
}

func (rs *RenameSuggester) BatchSuggestMovieNames(ctx context.Context, paths []string) (map[string][]rename.Suggestion, error) {
	if len(paths) == 0 {
		return make(map[string][]rename.Suggestion), nil
	}

	result := make(map[string][]rename.Suggestion)

	for _, path := range paths {
		info := rs.ParseFileName(path)

		if info.MediaType != tmdb.MediaTypeMovie {
			logger.Warn("Skipping non-movie file", "path", path, "detectedType", info.MediaType)
			continue
		}

		suggestions, err := rs.suggestMovieName(ctx, path, info)
		if err != nil {
			logger.Warn("Failed to suggest movie name", "path", path, "title", info.Title, "error", err)
			continue
		}

		result[path] = suggestions
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("未能为任何电影文件生成重命名建议")
	}

	return result, nil
}
