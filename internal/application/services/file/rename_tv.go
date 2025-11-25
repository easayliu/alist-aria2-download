package file

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/easayliu/alist-aria2-download/internal/domain/models/rename"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/tmdb"
	"github.com/easayliu/alist-aria2-download/pkg/logger"
)

// suggestTVName 为TV剧集生成重命名建议
func (rs *RenameSuggester) suggestTVName(ctx context.Context, fullPath string, info *MediaInfo) ([]rename.Suggestion, error) {
	searchQuery := info.Title
	if info.Version != "" {
		searchQuery = fmt.Sprintf("%s %s", info.Title, info.Version)
		logger.Info("Version detected, using full name for search", "originalTitle", info.Title, "version", info.Version, "searchQuery", searchQuery)
	}

	return rs.searchTVByQuery(ctx, fullPath, info, searchQuery)
}

// searchTVByQuery 通过查询搜索TV剧集
func (rs *RenameSuggester) searchTVByQuery(ctx context.Context, fullPath string, info *MediaInfo, query string) ([]rename.Suggestion, error) {
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
		// 检查 name 或 original_name 是否匹配（处理简繁体差异）
		nameMatch := rs.matchOriginalName(query, result.Name)
		originalNameMatch := rs.matchOriginalName(query, result.OriginalName)

		if !nameMatch && !originalNameMatch {
			logger.Debug("Skipping result: neither name nor original_name matches",
				"query", query,
				"name", result.Name,
				"originalName", result.OriginalName)
			continue
		}

		year := rs.extractYear(result.FirstAirDate)
		confidence := rs.calculateConfidence(i, info.Year, year)

		seasonDetails, err := rs.tmdbClient.GetSeasonDetails(ctx, result.ID, info.Season)
		if err != nil {
			logger.Warn("Failed to get season details", "tvID", result.ID, "name", result.Name, "season", info.Season, "error", err)
			continue
		}

		logger.Info("Found matching season", "name", result.Name, "season", seasonDetails.SeasonNumber, "episodeCount", seasonDetails.EpisodeCount)

		matchedEpisode, _ := rs.matchEpisodeByAirDate(info, seasonDetails.Episodes, "")
		if matchedEpisode > seasonDetails.EpisodeCount {
			logger.Warn("Episode number out of range", "name", result.Name, "season", info.Season, "requestedEpisode", matchedEpisode, "maxEpisode", seasonDetails.EpisodeCount)
			continue
		}

		sug := rs.buildTVSuggestion(fullPath, query, info, result.ID, year, matchedEpisode, seasonDetails.Episodes, confidence)
		suggestions = append(suggestions, sug)
	}

	if len(suggestions) == 0 {
		return nil, fmt.Errorf("未找到包含第 %d 季的剧集 '%s'", info.Season, query)
	}

	return suggestions, nil
}

// BatchSuggestTVNames 批量生成TV剧集重命名建议
func (rs *RenameSuggester) BatchSuggestTVNames(ctx context.Context, paths []string) (map[string][]rename.Suggestion, error) {
	if len(paths) == 0 {
		return make(map[string][]rename.Suggestion), nil
	}

	// 预解析所有文件
	pathInfoMap := rs.parseAllPaths(paths)

	// 提取剧集名
	showName := rs.extractShowNameFromPaths(paths, pathInfoMap)
	if showName == "" {
		return nil, fmt.Errorf("无法从路径中提取节目名称")
	}

	logger.Info("Batch rename: extracted show name", "showName", showName, "referencePath", paths[0])

	// 按版本分组
	pathsByVersion := rs.groupPathsByVersion(paths, pathInfoMap)

	result := make(map[string][]rename.Suggestion)

	for version, versionPaths := range pathsByVersion {
		searchQuery := showName
		if version != "" {
			searchQuery = fmt.Sprintf("%s %s", showName, version)
			logger.Info("Batch rename: processing version files", "version", version, "searchQuery", searchQuery, "fileCount", len(versionPaths))
		} else {
			logger.Info("Batch rename: processing regular files", "searchQuery", searchQuery, "fileCount", len(versionPaths))
		}

		seasonMap := rs.groupPathsBySeason(versionPaths, pathInfoMap)
		rs.logSeasonDistribution(searchQuery, seasonMap)

		versionResults, err := rs.batchSearchTVByQuery(ctx, searchQuery, seasonMap, pathInfoMap)
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

// batchSearchTVByQuery 批量搜索TV剧集
func (rs *RenameSuggester) batchSearchTVByQuery(ctx context.Context, query string, seasonMap map[int][]string, pathInfoMap map[string]*MediaInfo) (map[string][]rename.Suggestion, error) {
	totalFiles := 0
	for _, paths := range seasonMap {
		totalFiles += len(paths)
	}

	logger.Info("Batch searching TMDB TV series", "query", query, "seasonCount", len(seasonMap), "fileCount", totalFiles)

	resp, err := rs.tmdbClient.SearchTV(ctx, query, 0)
	if err != nil {
		return nil, fmt.Errorf("TMDB搜索失败: %w", err)
	}

	// 如果没有结果，尝试移除年份重新搜索
	if len(resp.Results) == 0 {
		resp, err = rs.retrySearchWithoutYear(ctx, query)
		if err != nil {
			return nil, err
		}
	}

	logger.Info("TMDB search returned results", "query", query, "resultCount", len(resp.Results))

	result := make(map[string][]rename.Suggestion)

	for _, tvResult := range resp.Results {
		// 检查 name 或 original_name 是否匹配（处理简繁体差异）
		nameMatch := rs.matchOriginalName(query, tvResult.Name)
		originalNameMatch := rs.matchOriginalName(query, tvResult.OriginalName)

		if !nameMatch && !originalNameMatch {
			logger.Debug("Skipping result: neither name nor original_name matches",
				"query", query,
				"name", tvResult.Name,
				"originalName", tvResult.OriginalName)
			continue
		}

		logger.Info("Matched TV show", "query", query, "tvID", tvResult.ID, "name", tvResult.Name, "originalName", tvResult.OriginalName, "nameMatch", nameMatch, "originalNameMatch", originalNameMatch)

		year := rs.extractYear(tvResult.FirstAirDate)
		successCount := 0

		for season, seasonPaths := range seasonMap {
			seasonDetails, err := rs.tmdbClient.GetSeasonDetails(ctx, tvResult.ID, season)
			if err != nil {
				logger.Warn("Failed to get season details", "tvID", tvResult.ID, "query", query, "season", season, "error", err)
				continue
			}

			episodeMap := rs.buildEpisodeMap(seasonDetails.Episodes)
			logger.Info("Got season details", "query", query, "season", season, "episodeCount", len(episodeMap))

			for _, path := range seasonPaths {
				info := pathInfoMap[path]
				matchedEpisode, _ := rs.matchEpisodeByAirDate(info, seasonDetails.Episodes, "Batch rename: ")

				if episode, exists := episodeMap[matchedEpisode]; exists {
					sug := rs.buildBatchTVSuggestion(path, query, info, tvResult.ID, year, season, matchedEpisode, episode.Name)
					result[path] = append(result[path], sug)
					successCount++
				} else {
					logger.Warn("Episode not found in episodeMap", "path", path, "matchedEpisode", matchedEpisode, "season", season)
				}
			}
		}

		if successCount > 0 {
			break
		}
	}

	logger.Info("Batch search completed", "query", query, "matchedFiles", len(result), "totalInputFiles", totalFiles)
	return result, nil
}

// ============ 辅助方法 ============

// matchOriginalName 检查原始名称是否匹配
func (rs *RenameSuggester) matchOriginalName(query, originalName string) bool {
	queryLower := strings.ToLower(strings.TrimSpace(query))
	originalNameLower := strings.ToLower(strings.TrimSpace(originalName))
	return originalNameLower == queryLower
}

// extractYear 从日期字符串提取年份
func (rs *RenameSuggester) extractYear(dateStr string) int {
	if dateStr != "" && len(dateStr) >= 4 {
		if year, err := strconv.Atoi(dateStr[:4]); err == nil {
			return year
		}
	}
	return 0
}

// calculateConfidence 计算置信度
func (rs *RenameSuggester) calculateConfidence(index, infoYear, resultYear int) float64 {
	confidence := 1.0 - (float64(index) * 0.1)
	if infoYear > 0 && resultYear == infoYear {
		confidence += 0.2
	}
	return confidence
}

// parseAllPaths 预解析所有路径
func (rs *RenameSuggester) parseAllPaths(paths []string) map[string]*MediaInfo {
	pathInfoMap := make(map[string]*MediaInfo, len(paths))
	for _, path := range paths {
		pathInfoMap[path] = rs.ParseFileName(path)
	}
	return pathInfoMap
}

// extractShowNameFromPaths 从路径中提取剧集名
func (rs *RenameSuggester) extractShowNameFromPaths(paths []string, pathInfoMap map[string]*MediaInfo) string {
	// 找到第一个TV路径
	firstTVPath := ""
	for _, path := range paths {
		if pathInfoMap[path].MediaType == tmdb.MediaTypeTV {
			firstTVPath = path
			break
		}
	}

	if firstTVPath == "" {
		logger.Info("No TV path detected, trying to extract show name from path", "firstPath", paths[0])
		firstTVPath = paths[0]
	}

	showName, _ := rs.getPathInfo(pathInfoMap[firstTVPath], firstTVPath)
	if showName == "" {
		showName = pathInfoMap[firstTVPath].Title
	}

	return showName
}

// groupPathsByVersion 按版本分组
func (rs *RenameSuggester) groupPathsByVersion(paths []string, pathInfoMap map[string]*MediaInfo) map[string][]string {
	pathsByVersion := make(map[string][]string)
	for _, path := range paths {
		version := pathInfoMap[path].Version
		pathsByVersion[version] = append(pathsByVersion[version], path)
	}
	return pathsByVersion
}

// groupPathsBySeason 按季度分组
func (rs *RenameSuggester) groupPathsBySeason(paths []string, pathInfoMap map[string]*MediaInfo) map[int][]string {
	seasonMap := make(map[int][]string)
	for _, path := range paths {
		info := pathInfoMap[path]
		_, pathSeason := rs.getPathInfo(info, path)

		detectedSeason := pathSeason
		if info.Season > 0 {
			detectedSeason = info.Season
		}

		if detectedSeason > 0 {
			seasonMap[detectedSeason] = append(seasonMap[detectedSeason], path)
		} else {
			seasonMap[1] = append(seasonMap[1], path)
		}
	}
	return seasonMap
}

// logSeasonDistribution 记录季度分布
func (rs *RenameSuggester) logSeasonDistribution(searchQuery string, seasonMap map[int][]string) {
	var parts []string
	for s, paths := range seasonMap {
		parts = append(parts, fmt.Sprintf("S%02d: %d files", s, len(paths)))
	}
	sort.Strings(parts)
	logger.Info("Season distribution summary", "searchQuery", searchQuery, "seasonMap", strings.Join(parts, ", "))
}

// retrySearchWithoutYear 移除年份后重试搜索
func (rs *RenameSuggester) retrySearchWithoutYear(ctx context.Context, query string) (*tmdb.SearchTVResponse, error) {
	yearRegex := regexp.MustCompile(`\s+\d{4}$`)
	if !yearRegex.MatchString(query) {
		return nil, fmt.Errorf("TMDB数据库中未找到剧集 '%s'", query)
	}

	showNameWithoutYear := yearRegex.ReplaceAllString(query, "")
	logger.Info("Retry search without year", "originalQuery", query, "newQuery", showNameWithoutYear)

	resp, err := rs.tmdbClient.SearchTV(ctx, showNameWithoutYear, 0)
	if err != nil {
		return nil, fmt.Errorf("TMDB搜索失败: %w", err)
	}

	if len(resp.Results) == 0 {
		return nil, fmt.Errorf("TMDB数据库中未找到剧集 '%s'", showNameWithoutYear)
	}

	return resp, nil
}

// buildEpisodeMap 构建集数映射
func (rs *RenameSuggester) buildEpisodeMap(episodes []tmdb.Episode) map[int]*tmdb.Episode {
	episodeMap := make(map[int]*tmdb.Episode)
	for i := range episodes {
		ep := &episodes[i]
		episodeMap[ep.EpisodeNumber] = ep
	}
	return episodeMap
}

// buildTVSuggestion 构建TV建议
func (rs *RenameSuggester) buildTVSuggestion(fullPath, query string, info *MediaInfo, tmdbID, year, matchedEpisode int, episodes []tmdb.Episode, confidence float64) rename.Suggestion {
	var episodeName string
	if len(episodes) > 0 && matchedEpisode > 0 && matchedEpisode <= len(episodes) {
		episodeName = episodes[matchedEpisode-1].Name
	}

	newName := fmt.Sprintf("%s - S%02dE%02d", query, info.Season, matchedEpisode)
	if episodeName != "" {
		newName += fmt.Sprintf(" - %s", episodeName)
	}
	newName += info.Extension

	newPath := rs.buildEmbyPath(fullPath, query, year, info.Season, newName)

	logger.Info("Generated rename suggestion", "originalPath", fullPath, "newName", newName, "newPath", newPath, "tmdbID", tmdbID, "season", info.Season, "episode", matchedEpisode)

	sug := rename.Suggestion{
		NewName:    newName,
		NewPath:    newPath,
		MediaType:  rename.FromTMDBMediaType(tmdb.MediaTypeTV),
		TMDBID:     tmdbID,
		Title:      query,
		Year:       year,
		Confidence: confidence,
		Source:     rename.SourceTMDB,
	}
	sug.SetSeason(info.Season)
	sug.SetEpisode(matchedEpisode)
	return sug
}

// buildBatchTVSuggestion 构建批量TV建议
func (rs *RenameSuggester) buildBatchTVSuggestion(path, query string, info *MediaInfo, tmdbID, year, season, matchedEpisode int, episodeName string) rename.Suggestion {
	newName := fmt.Sprintf("%s - S%02dE%02d", query, season, matchedEpisode)
	if episodeName != "" {
		newName += fmt.Sprintf(" - %s", episodeName)
	}
	newName += info.Extension

	newPath := rs.buildEmbyPath(path, query, year, season, newName)

	logger.Info("Batch rename: generated rename suggestion", "originalPath", path, "newName", newName, "newPath", newPath, "tmdbID", tmdbID, "season", season, "episode", matchedEpisode)

	sug := rename.Suggestion{
		NewName:    newName,
		NewPath:    newPath,
		MediaType:  rename.FromTMDBMediaType(tmdb.MediaTypeTV),
		TMDBID:     tmdbID,
		Title:      query,
		Year:       year,
		Confidence: 1.0,
		Source:     rename.SourceTMDB,
	}
	sug.SetSeason(season)
	sug.SetEpisode(matchedEpisode)
	return sug
}

// matchEpisodeByAirDate 根据播出日期匹配集数
func (rs *RenameSuggester) matchEpisodeByAirDate(info *MediaInfo, episodes []tmdb.Episode, logPrefix string) (int, string) {
	if info.AirDate == "" {
		return info.Episode, ""
	}

	var sameDateEpisodes []tmdb.Episode
	for _, ep := range episodes {
		if ep.AirDate == info.AirDate {
			sameDateEpisodes = append(sameDateEpisodes, ep)
		}
	}

	if len(sameDateEpisodes) == 0 {
		return info.Episode, ""
	}

	selectedEpisode := sameDateEpisodes[0]

	if info.Part != "" && len(sameDateEpisodes) > 1 {
		partIndex := rs.getPartIndex(info.Part, len(sameDateEpisodes))
		if partIndex < len(sameDateEpisodes) {
			selectedEpisode = sameDateEpisodes[partIndex]
			logger.Info(logPrefix+"matched episode by air date and part",
				"airDate", info.AirDate, "part", info.Part,
				"totalEpisodes", len(sameDateEpisodes), "partIndex", partIndex,
				"episode", selectedEpisode.EpisodeNumber, "episodeName", selectedEpisode.Name)
		}
	} else {
		if len(sameDateEpisodes) > 1 && info.Part == "" {
			logger.Warn(logPrefix+"multiple episodes on same air date without part specified, selecting first episode",
				"airDate", info.AirDate, "episodeCount", len(sameDateEpisodes), "selectedEpisode", selectedEpisode.EpisodeNumber)
		} else {
			logger.Info(logPrefix+"matched episode by air date",
				"airDate", info.AirDate, "episode", selectedEpisode.EpisodeNumber, "episodeName", selectedEpisode.Name)
		}
	}

	return selectedEpisode.EpisodeNumber, selectedEpisode.Name
}

// getPartIndex 获取分集索引
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
