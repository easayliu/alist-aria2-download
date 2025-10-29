package file

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/easayliu/alist-aria2-download/internal/infrastructure/tmdb"
	"github.com/easayliu/alist-aria2-download/pkg/logger"
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

type SuggestedName struct {
	NewName      string
	NewPath      string
	MediaType    tmdb.MediaType
	TMDBID       int
	Title        string
	Year         int
	Season       int
	Episode      int
	Confidence   float64
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

func (rs *RenameSuggester) SearchAndSuggest(ctx context.Context, fullPath string) ([]SuggestedName, error) {
	info := rs.ParseFileName(fullPath)

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

func (rs *RenameSuggester) suggestMovieName(ctx context.Context, fullPath string, info *MediaInfo) ([]SuggestedName, error) {
	resp, err := rs.tmdbClient.SearchMovie(ctx, info.Title, info.Year)
	if err != nil {
		return nil, fmt.Errorf("failed to search movie: %w", err)
	}

	if len(resp.Results) == 0 {
		return nil, fmt.Errorf("TMDB数据库中未找到电影 '%s'，可能是因为：\n1. 电影名称不准确\n2. TMDB未收录该影片\n3. 需要使用英文名称搜索", info.Title)
	}

	suggestions := make([]SuggestedName, 0, len(resp.Results))
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
			newPath := rs.buildMoviePath(result.Title, year, newName)

			suggestions = append(suggestions, SuggestedName{
				NewName:    newName,
				NewPath:    newPath,
				MediaType:  tmdb.MediaTypeMovie,
				TMDBID:     result.ID,
				Title:      result.Title,
				Year:       year,
				Confidence: confidence,
			})
			continue
		}

		title := details.Title
		if details.OriginalTitle != "" && details.OriginalLanguage != "en" {
			title = details.OriginalTitle
		}

		newName := fmt.Sprintf("%s (%d)%s", title, year, info.Extension)
		newPath := rs.buildMoviePath(title, year, newName)

		logger.Info("Generated movie rename suggestion",
			"originalPath", fullPath,
			"newName", newName,
			"newPath", newPath,
			"tmdbID", details.ID,
			"title", title,
			"originalTitle", details.OriginalTitle,
			"year", year,
			"runtime", details.Runtime)

		suggestions = append(suggestions, SuggestedName{
			NewName:    newName,
			NewPath:    newPath,
			MediaType:  tmdb.MediaTypeMovie,
			TMDBID:     details.ID,
			Title:      title,
			Year:       year,
			Confidence: confidence,
		})
	}

	return suggestions, nil
}

func (rs *RenameSuggester) suggestTVName(ctx context.Context, fullPath string, info *MediaInfo) ([]SuggestedName, error) {
	searchQuery := info.Title
	if info.Version != "" {
		searchQuery = fmt.Sprintf("%s %s", info.Title, info.Version)
		logger.Info("Version detected, using full name for search", "originalTitle", info.Title, "version", info.Version, "searchQuery", searchQuery)
	}

	return rs.searchTVByQuery(ctx, fullPath, info, searchQuery, info.Version != "")
}

func (rs *RenameSuggester) searchTVByQuery(ctx context.Context, fullPath string, info *MediaInfo, query string, isVersionSearch bool) ([]SuggestedName, error) {
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

	suggestions := make([]SuggestedName, 0, len(resp.Results))
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

		suggestions = append(suggestions, SuggestedName{
			NewName:    newName,
			NewPath:    newPath,
			MediaType:  tmdb.MediaTypeTV,
			TMDBID:     result.ID,
			Title:      displayName,
			Year:       year,
			Season:     info.Season,
			Episode:    matchedEpisode,
			Confidence: confidence,
		})
	}

	if len(suggestions) == 0 {
		return nil, fmt.Errorf("未找到包含第 %d 季的剧集 '%s'", info.Season, query)
	}

	return suggestions, nil
}

func (rs *RenameSuggester) extractTVInfoFromPath(fullPath string) (showName string, season int) {
	parts := strings.Split(fullPath, "/")

	for i := len(parts) - 1; i >= 0; i-- {
		part := parts[i]

		if part == "" || part == "data" || part == "来自：分享" || part == "tvs" || part == "剧集" || part == "电视剧" {
			continue
		}

		if strings.Contains(part, ".") && i == len(parts)-1 {
			continue
		}

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
					return
				}
				continue
			}
		}

		if strings.Contains(part, "全") && strings.Contains(part, "季") {
			collectionRegex := regexp.MustCompile(`^(.+?)\s*全\d+`)
			if match := collectionRegex.FindStringSubmatch(part); len(match) > 1 {
				showName = strings.TrimSpace(match[1])
				season = 0
				logger.Info("Detected collection directory", "showName", showName, "pathPart", part)
				continue
			}
		}

		if season == 0 {
			seasonPattern := strutil.SeasonPattern
			if match := seasonPattern.FindStringSubmatch(strings.ToLower(part)); len(match) > 1 {
				if num, err := strconv.Atoi(match[1]); err == nil {
					season = num
				}
			}
		}

		if rs.isQualityOrFormatDir(part) {
			continue
		}

		seasonPattern := strutil.SeasonStrictPattern
		if match := seasonPattern.FindStringSubmatch(strings.ToLower(part)); len(match) > 1 {
			if num, err := strconv.Atoi(match[1]); err == nil && season == 0 {
				season = num
			}
			continue
		}

		if showName == "" && !strutil.IsSeasonDirectory(part) && !strings.Contains(part, "全") && !rs.isQualityOrFormatDir(part) {
			cleaned := strutil.CleanShowName(part)
			if cleaned != "" && len(cleaned) > 1 {
				showName = cleaned
			}
		}
	}

	if season == 0 {
		season = 1
	}

	return
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
	if rs.isSpecialContent(fileName) {
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

func (rs *RenameSuggester) isSpecialContent(fileName string) bool {
	specialKeywords := []string{
		"加更", "花絮", "预告", "片花", "彩蛋", "幕后", "特辑",
		"番外", "访谈", "采访", "回顾", "精彩", "集锦", "合集",
		"trailer", "preview", "bonus", "extra", "special", "behind",
	}

	lowerFileName := strings.ToLower(fileName)
	for _, keyword := range specialKeywords {
		if strings.Contains(lowerFileName, keyword) {
			logger.Info("Special content detected, skipping match", "fileName", fileName, "keyword", keyword)
			return true
		}
	}
	return false
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

func (rs *RenameSuggester) buildEmbyPath(_ string, seriesName string, year, season int, fileName string) string {
	baseDir := "/data/tvs"

	var seriesDir string
	if year > 0 {
		seriesDir = fmt.Sprintf("%s (%d)", seriesName, year)
	} else {
		seriesDir = seriesName
	}

	seasonDir := fmt.Sprintf("Season %02d", season)

	return filepath.Join(baseDir, seriesDir, seasonDir, fileName)
}

func (rs *RenameSuggester) buildMoviePath(movieTitle string, year int, fileName string) string {
	baseDir := "/data/movies"

	var movieDir string
	if year > 0 {
		movieDir = fmt.Sprintf("%s (%d)", movieTitle, year)
	} else {
		movieDir = movieTitle
	}

	return filepath.Join(baseDir, movieDir, fileName)
}

func (rs *RenameSuggester) BatchSuggestTVNames(ctx context.Context, paths []string) (map[string][]SuggestedName, error) {
	if len(paths) == 0 {
		return make(map[string][]SuggestedName), nil
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

	result := make(map[string][]SuggestedName)

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
			info := rs.ParseFileName(path)
			if info.Season > 0 {
				seasonMap[info.Season] = append(seasonMap[info.Season], path)
			} else {
				seasonMap[1] = append(seasonMap[1], path)
			}
		}

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

func (rs *RenameSuggester) batchSearchTVByQuery(ctx context.Context, query string, seasonMap map[int][]string) (map[string][]SuggestedName, error) {
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

	result := make(map[string][]SuggestedName)

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

					result[path] = append(result[path], SuggestedName{
						NewName:    newName,
						NewPath:    newPath,
						MediaType:  tmdb.MediaTypeTV,
						TMDBID:     tvResult.ID,
						Title:      displayName,
						Year:       year,
						Season:     season,
						Episode:    matchedEpisode,
						Confidence: 1.0,
					})
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

func (rs *RenameSuggester) BatchSuggestMovieNames(ctx context.Context, paths []string) (map[string][]SuggestedName, error) {
	if len(paths) == 0 {
		return make(map[string][]SuggestedName), nil
	}

	result := make(map[string][]SuggestedName)

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
