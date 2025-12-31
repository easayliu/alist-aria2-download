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
)

// 跳过原因常量
const (
	skipReasonEmbyFormat      = "已符合 Emby 标准格式"
	skipReasonSpecialContent  = "特殊内容（先导片/加更/花絮等），无法匹配标准剧集"
	skipReasonEpisodeNotFound = "无法从文件名中识别剧集编号"
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
		nameMatch := rs.matchContains(query, result.Name)
		originalNameMatch := rs.matchContains(query, result.OriginalName)

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

	result := make(map[string][]rename.Suggestion)

	// 预过滤：跳过已符合 Emby 标准格式的文件和特殊内容
	var pathsToProcess []string
	for _, path := range paths {
		filename := filepath.Base(path)
		filenameWithoutExt := strings.TrimSuffix(filename, filepath.Ext(filename))

		if rs.IsAlreadyEmbyTVFormat(filename) {
			logger.Info("文件已符合 Emby 标准格式，跳过",
				"path", path,
				"filename", filename)
			result[path] = []rename.Suggestion{rs.BuildSkippedSuggestion(path, skipReasonEmbyFormat)}
		} else if media.IsSpecialContent(filenameWithoutExt) {
			logger.Info("特殊内容文件，跳过重命名",
				"path", path,
				"filename", filename)
			result[path] = []rename.Suggestion{rs.BuildSkippedSuggestion(path, skipReasonSpecialContent)}
		} else {
			pathsToProcess = append(pathsToProcess, path)
		}
	}

	// 如果所有文件都已被跳过（符合标准或特殊内容），直接返回
	if len(pathsToProcess) == 0 {
		logger.Info("所有文件已被跳过，无需进一步处理", "totalFiles", len(paths))
		return result, nil
	}

	logger.Info("批量重命名预过滤完成",
		"totalFiles", len(paths),
		"skipped", len(paths)-len(pathsToProcess),
		"toProcess", len(pathsToProcess))

	// 预解析需要处理的文件
	pathInfoMap := rs.parseAllPaths(pathsToProcess)

	// 检测分集模式并重新计算集数
	partsPerEpisode := rs.detectPartsPerEpisode(pathInfoMap)
	if partsPerEpisode > 0 {
		rs.recalculateEpisodesWithPartMode(pathInfoMap, partsPerEpisode)
	}

	// 提取剧集名
	showName := rs.extractShowNameFromPaths(pathsToProcess, pathInfoMap)
	if showName == "" {
		return nil, fmt.Errorf("无法从路径中提取节目名称")
	}

	logger.Info("Batch rename: extracted show name", "showName", showName, "referencePath", pathsToProcess[0])

	// 按版本分组（仅处理未标准化的文件）
	pathsByVersion := rs.groupPathsByVersion(pathsToProcess, pathInfoMap)

	for version, versionPaths := range pathsByVersion {
		searchQuery := showName
		if version != "" {
			searchQuery = fmt.Sprintf("%s %s", showName, version)
			logger.Info("Batch rename: processing version files", "version", version, "searchQuery", searchQuery, "fileCount", len(versionPaths))
		} else {
			logger.Info("Batch rename: processing regular files", "searchQuery", searchQuery, "fileCount", len(versionPaths))
		}

		// 按父目录分组(解决混合单季和多季目录的问题)
		dirGroups := rs.groupPathsByParentDir(versionPaths)
		logger.Info("Grouping paths by parent directory", "groupCount", len(dirGroups))

		for parentDir, dirPaths := range dirGroups {
			logger.Info("Processing directory group", "parentDir", parentDir, "fileCount", len(dirPaths))

			// 检测季度范围(针对当前目录组)
			var seasonRangeDetected bool
			var startSeason, endSeason int
			if len(dirPaths) > 0 {
				_, startSeason, endSeason = rs.ExtractSeasonRange(dirPaths[0])
				if startSeason > 0 && endSeason > 0 {
					seasonRangeDetected = true
					logger.Info("Detected season range directory",
						"parentDir", parentDir,
						"startSeason", startSeason,
						"endSeason", endSeason,
						"fileCount", len(dirPaths))
				}
			}

			var seasonMap map[int][]string
			if !seasonRangeDetected {
				// 常规分组:按路径中的季度信息分组
				seasonMap = rs.groupPathsBySeason(dirPaths, pathInfoMap)
				rs.logSeasonDistribution(searchQuery, seasonMap)
			} else {
				// 季度范围模式:所有文件放在一个虚拟分组中
				seasonMap = map[int][]string{
					0: dirPaths, // 使用0作为标记,表示需要智能分配
				}
				logger.Info("Season range mode, files pending smart assignment", "fileCount", len(dirPaths))
			}

			versionResults, err := rs.batchSearchTVByQuery(ctx, searchQuery, seasonMap, pathInfoMap, seasonRangeDetected, startSeason, endSeason)
			if err != nil {
				logger.Warn("Batch rename: search failed", "query", searchQuery, "parentDir", parentDir, "error", err)
				continue
			}

			for path, suggestions := range versionResults {
				result[path] = append(result[path], suggestions...)
			}
		}
	}

	// 检查是否有任何非跳过的结果
	hasNonSkippedResult := false
	for _, suggestions := range result {
		for _, sug := range suggestions {
			if !sug.Skipped {
				hasNonSkippedResult = true
				break
			}
		}
		if hasNonSkippedResult {
			break
		}
	}

	// 如果没有非跳过的结果，且原始请求中有需要处理的文件，则返回错误
	if !hasNonSkippedResult && len(pathsToProcess) > 0 {
		return nil, fmt.Errorf("TV series '%s' not found in TMDB database", showName)
	}

	return result, nil
}

// batchSearchTVByQuery 批量搜索TV剧集
func (rs *RenameSuggester) batchSearchTVByQuery(
	ctx context.Context,
	query string,
	seasonMap map[int][]string,
	pathInfoMap map[string]*MediaInfo,
	seasonRangeDetected bool,
	startSeason, endSeason int,
) (map[string][]rename.Suggestion, error) {
	totalFiles := 0
	for _, paths := range seasonMap {
		totalFiles += len(paths)
	}

	logger.Info("Batch searching TMDB TV series",
		"query", query,
		"seasonCount", len(seasonMap),
		"fileCount", totalFiles,
		"seasonRangeDetected", seasonRangeDetected,
		"seasonRange", fmt.Sprintf("%d-%d", startSeason, endSeason))

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

	// 尝试从文件名提取英文名称作为备选搜索词
	var alternativeQuery string
	for _, paths := range seasonMap {
		if len(paths) > 0 {
			if info, exists := pathInfoMap[paths[0]]; exists {
				alternativeQuery = rs.extractEnglishTitleFromFileName(info.OriginalName)
				if alternativeQuery != "" && alternativeQuery != query {
					logger.Info("Extracted alternative query from filename",
						"originalQuery", query,
						"alternativeQuery", alternativeQuery)
				}
			}
			break
		}
	}

	// 如果原搜索无结果且有备选英文名，用英文名重新搜索
	if len(resp.Results) == 0 && alternativeQuery != "" && alternativeQuery != query {
		logger.Info("Retrying search with alternative query", "alternativeQuery", alternativeQuery)
		altResp, altErr := rs.tmdbClient.SearchTV(ctx, alternativeQuery, 0)
		if altErr == nil && len(altResp.Results) > 0 {
			resp = altResp
			logger.Info("Alternative search succeeded", "alternativeQuery", alternativeQuery, "resultCount", len(resp.Results))
		}
	}

	logger.Info("TMDB search returned results", "query", query, "resultCount", len(resp.Results))

	result := make(map[string][]rename.Suggestion)

	// 单次遍历，按匹配分数排序选择最佳结果
	bestMatch := rs.findBestMatch(resp.Results, query, alternativeQuery)
	if bestMatch == nil {
		logger.Info("Batch search completed", "query", query, "matchedFiles", 0, "totalInputFiles", totalFiles)
		return result, nil
	}

	logger.Info("Matched TV show", "query", query, "tvID", bestMatch.ID, "name", bestMatch.Name,
		"originalName", bestMatch.OriginalName, "matchType", bestMatch.matchType)

	year := rs.extractYear(bestMatch.FirstAirDate)
	if seasonRangeDetected && startSeason > 0 && endSeason > 0 {
		rs.handleSeasonRange(ctx, bestMatch.ID, query, year, startSeason, endSeason, seasonMap, pathInfoMap, &result)
	} else {
		rs.handleRegularSeasons(ctx, bestMatch.ID, query, year, seasonMap, pathInfoMap, &result)
	}

	logger.Info("Batch search completed", "query", query, "matchedFiles", len(result), "totalInputFiles", totalFiles)
	return result, nil
}

// handleRegularSeasons 处理常规季度分组
func (rs *RenameSuggester) handleRegularSeasons(
	ctx context.Context,
	tvID int,
	query string,
	year int,
	seasonMap map[int][]string,
	pathInfoMap map[string]*MediaInfo,
	result *map[string][]rename.Suggestion,
) int {
	successCount := 0

	for season, seasonPaths := range seasonMap {
		seasonDetails, err := rs.tmdbClient.GetSeasonDetails(ctx, tvID, season)
		if err != nil {
			logger.Warn("Failed to get season details", "tvID", tvID, "query", query, "season", season, "error", err)
			continue
		}

		episodeMap := rs.buildEpisodeMap(seasonDetails.Episodes)
		logger.Info("Got season details", "query", query, "season", season, "episodeCount", len(episodeMap))

		for _, path := range seasonPaths {
			info := pathInfoMap[path]
			matchedEpisode, _ := rs.matchEpisodeByAirDate(info, seasonDetails.Episodes, "Batch rename: ")

			if episode, exists := episodeMap[matchedEpisode]; exists {
				sug := rs.buildBatchTVSuggestion(path, query, info, tvID, year, season, matchedEpisode, episode.Name)
				(*result)[path] = append((*result)[path], sug)
				successCount++
			} else {
				logger.Warn("Episode not found in episodeMap", "path", path, "matchedEpisode", matchedEpisode, "season", season)
				(*result)[path] = []rename.Suggestion{rs.BuildSkippedSuggestion(path, skipReasonEpisodeNotFound)}
			}
		}
	}

	return successCount
}

// handleSeasonRange 处理季度范围情况(如"第1-3季"目录包含多季内容)
func (rs *RenameSuggester) handleSeasonRange(
	ctx context.Context,
	tvID int,
	query string,
	year int,
	startSeason, endSeason int,
	seasonMap map[int][]string,
	pathInfoMap map[string]*MediaInfo,
	result *map[string][]rename.Suggestion,
) int {
	// 收集所有文件并按集数排序
	var allPaths []string
	for _, paths := range seasonMap {
		allPaths = append(allPaths, paths...)
	}

	logger.Info("Season range processing started",
		"startSeason", startSeason,
		"endSeason", endSeason,
		"totalFiles", len(allPaths))

	// 按文件名中的集数排序
	sort.Slice(allPaths, func(i, j int) bool {
		ei := pathInfoMap[allPaths[i]].Episode
		ej := pathInfoMap[allPaths[j]].Episode
		return ei < ej
	})

	logger.Debug("Files sorted by episode number",
		"firstFile", allPaths[0],
		"firstEpisode", pathInfoMap[allPaths[0]].Episode,
		"lastFile", allPaths[len(allPaths)-1],
		"lastEpisode", pathInfoMap[allPaths[len(allPaths)-1]].Episode)

	// 获取所有季度的数据
	type seasonInfo struct {
		season       int
		episodeCount int
		episodes     []tmdb.Episode
	}

	var seasons []seasonInfo
	totalEpisodes := 0

	for s := startSeason; s <= endSeason; s++ {
		seasonDetails, err := rs.tmdbClient.GetSeasonDetails(ctx, tvID, s)
		if err != nil {
			logger.Warn("Failed to get season details", "tvID", tvID, "season", s, "error", err)
			continue
		}

		info := seasonInfo{
			season:       s,
			episodeCount: len(seasonDetails.Episodes),
			episodes:     seasonDetails.Episodes,
		}
		seasons = append(seasons, info)
		totalEpisodes += len(seasonDetails.Episodes)

		logger.Info("Got season data",
			"season", s,
			"episodeCount", len(seasonDetails.Episodes))
	}

	if len(seasons) == 0 {
		logger.Warn("Failed to get any season data", "startSeason", startSeason, "endSeason", endSeason)
		return 0
	}

	logger.Info("Multi-season data retrieval completed",
		"seasonCount", len(seasons),
		"totalEpisodes", totalEpisodes,
		"totalFiles", len(allPaths))

	// 智能分配:根据集数累加确定每个文件属于哪一季
	successCount := 0
	episodeOffset := 0

	for _, si := range seasons {
		episodeMap := rs.buildEpisodeMap(si.episodes)
		seasonMatchCount := 0

		for _, path := range allPaths {
			info := pathInfoMap[path]
			fileEpisode := info.Episode

			// 判断此文件是否属于当前季度
			if fileEpisode > episodeOffset && fileEpisode <= episodeOffset+si.episodeCount {
				// 计算在当前季度中的集数
				seasonEpisode := fileEpisode - episodeOffset

				if episode, exists := episodeMap[seasonEpisode]; exists {
					sug := rs.buildBatchTVSuggestion(path, query, info, tvID, year, si.season, seasonEpisode, episode.Name)
					(*result)[path] = append((*result)[path], sug)
					successCount++
					seasonMatchCount++
					logger.Debug("Smart episode assignment",
						"path", path,
						"fileEpisode", fileEpisode,
						"assignedSeason", si.season,
						"seasonEpisode", seasonEpisode,
						"episodeName", episode.Name)
				} else {
					logger.Warn("Episode not in episodeMap",
						"path", path,
						"fileEpisode", fileEpisode,
						"season", si.season,
						"seasonEpisode", seasonEpisode,
						"episodeMapSize", len(episodeMap))
				}
			}
		}

		logger.Info("Season processing completed",
			"season", si.season,
			"matchCount", seasonMatchCount,
			"episodeRange", fmt.Sprintf("%d-%d", episodeOffset+1, episodeOffset+si.episodeCount))

		episodeOffset += si.episodeCount
	}

	logger.Info("Season range processing completed",
		"successCount", successCount,
		"totalFiles", len(allPaths),
		"failedCount", len(allPaths)-successCount)

	// 为未匹配的文件添加跳过建议
	for _, path := range allPaths {
		if _, exists := (*result)[path]; !exists {
			(*result)[path] = []rename.Suggestion{rs.BuildSkippedSuggestion(path, skipReasonEpisodeNotFound)}
		}
	}

	return successCount
}

// ============ 辅助方法 ============

// extractEnglishTitleFromFileName 从文件名中提取英文标题
// 例如: "Stranger.Things.S05E01.2025.2160p..." -> "Stranger Things"
func (rs *RenameSuggester) extractEnglishTitleFromFileName(fileName string) string {
	// 移除扩展名
	nameWithoutExt := strings.TrimSuffix(fileName, filepath.Ext(fileName))

	// 查找 SxxExx 的位置，取其之前的部分
	seasonEpisodeRegex := regexp.MustCompile(`[._\s][Ss]\d+[Ee]\d+`)
	loc := seasonEpisodeRegex.FindStringIndex(nameWithoutExt)
	if loc != nil {
		nameWithoutExt = nameWithoutExt[:loc[0]]
	}

	// 替换分隔符为空格
	cleanRegex := regexp.MustCompile(`[._\-]+`)
	cleaned := cleanRegex.ReplaceAllString(nameWithoutExt, " ")

	// 移除年份
	yearRegex := regexp.MustCompile(`\s+(19|20)\d{2}\s*$`)
	cleaned = yearRegex.ReplaceAllString(cleaned, "")

	// 移除常见的质量/格式标记
	removePatterns := []string{
		`(?i)\s+\d{3,4}p\s*$`,
		`(?i)\s+(UHD|FHD|4K|2K|HQ)\s*$`,
		`(?i)\s+(BluRay|WEB-?DL|WEBRip|HDRip)\s*$`,
	}
	for _, pattern := range removePatterns {
		re := regexp.MustCompile(pattern)
		cleaned = re.ReplaceAllString(cleaned, "")
	}

	result := strings.TrimSpace(cleaned)

	// 只返回看起来像英文名的结果（包含英文字母且长度合理）
	if len(result) >= 2 && regexp.MustCompile(`[A-Za-z]`).MatchString(result) {
		return result
	}

	return ""
}

// tvMatchResult 包装 TMDB 搜索结果和匹配类型
type tvMatchResult struct {
	tmdb.TVResult
	matchType string // "exact" 或 "contains"
}

// findBestMatch 从搜索结果中找到最佳匹配（优先精确匹配）
// 匹配优先级：精确匹配 > 包含匹配 > 唯一结果信任（TMDB已通过翻译名/别名关联）
func (rs *RenameSuggester) findBestMatch(results []tmdb.TVResult, query, altQuery string) *tvMatchResult {
	var exactMatch, containsMatch *tvMatchResult

	for i := range results {
		r := &results[i]
		// 检查精确匹配
		if rs.matchExact(query, r.Name) || rs.matchExact(query, r.OriginalName) ||
			(altQuery != "" && (rs.matchExact(altQuery, r.Name) || rs.matchExact(altQuery, r.OriginalName))) {
			exactMatch = &tvMatchResult{*r, "exact"}
			break // 精确匹配直接返回
		}
		// 记录第一个包含匹配
		if containsMatch == nil {
			if rs.matchContains(query, r.Name) || rs.matchContains(query, r.OriginalName) ||
				(altQuery != "" && (rs.matchContains(altQuery, r.Name) || rs.matchContains(altQuery, r.OriginalName))) {
				containsMatch = &tvMatchResult{*r, "contains"}
			}
		}
	}

	if exactMatch != nil {
		return exactMatch
	}
	if containsMatch != nil {
		return containsMatch
	}

	// 当搜索返回唯一结果时，信任TMDB的相关性匹配（处理翻译名/别名情况，如"同乐者"->Pluribus）
	if len(results) == 1 {
		logger.Info("Trusting TMDB single result", "query", query, "matchedName", results[0].Name,
			"originalName", results[0].OriginalName)
		return &tvMatchResult{results[0], "trusted"}
	}

	return nil
}

// matchExact 精确匹配（规范化后完全相等）
func (rs *RenameSuggester) matchExact(query, name string) bool {
	return rs.normalize(query) == rs.normalize(name)
}

// matchContains 包含匹配
func (rs *RenameSuggester) matchContains(query, name string) bool {
	q, n := rs.normalize(query), rs.normalize(name)
	if q == n {
		return true
	}
	return (len(n) >= 2 && strings.Contains(q, n)) || (len(q) >= 2 && strings.Contains(n, q))
}

// normalize 规范化名称（统一标点和空格）
func (rs *RenameSuggester) normalize(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	for _, ch := range []string{"·", "：", ":", "."} {
		s = strings.ReplaceAll(s, ch, " ")
	}
	return strings.Join(strings.Fields(s), " ")
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

// detectPartsPerEpisode 检测分集模式（2集：上/下，3集：上/中/下）
// 通过扫描所有文件的 Part 字段，如果存在"中"则为3集模式，否则为2集模式
func (rs *RenameSuggester) detectPartsPerEpisode(pathInfoMap map[string]*MediaInfo) int {
	for _, info := range pathInfoMap {
		if info.Part == "中" {
			logger.Info("Detected 3-part mode (上/中/下)", "file", info.OriginalName)
			return 3
		}
	}
	// 检查是否有分集标记
	hasPart := false
	for _, info := range pathInfoMap {
		if info.Part != "" {
			hasPart = true
			break
		}
	}
	if hasPart {
		logger.Info("Detected 2-part mode (上/下)")
		return 2
	}
	// 无分集标记，返回0表示不需要分集计算
	return 0
}

// recalculateEpisodesWithPartMode 根据检测到的分集模式重新计算集数
func (rs *RenameSuggester) recalculateEpisodesWithPartMode(pathInfoMap map[string]*MediaInfo, partsPerEpisode int) {
	if partsPerEpisode <= 0 {
		return
	}
	for path, info := range pathInfoMap {
		if info.BaseEpisode > 0 && info.Part != "" {
			oldEpisode := info.Episode
			info.Episode = rs.calculateEpisodeNumber(info.BaseEpisode, info.Part, partsPerEpisode)
			if oldEpisode != info.Episode {
				logger.Info("Recalculated episode number",
					"path", path,
					"baseEpisode", info.BaseEpisode,
					"part", info.Part,
					"partsPerEpisode", partsPerEpisode,
					"oldEpisode", oldEpisode,
					"newEpisode", info.Episode)
			}
		}
	}
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

// groupPathsByParentDir 按父目录分组
// 用于区分不同的季度目录(如"第1-3季"、"第6季"等)
func (rs *RenameSuggester) groupPathsByParentDir(paths []string) map[string][]string {
	dirGroups := make(map[string][]string)
	for _, path := range paths {
		parentDir := filepath.Dir(path)
		dirGroups[parentDir] = append(dirGroups[parentDir], path)
	}
	return dirGroups
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

		logger.Debug("Grouping path by season",
			"path", path,
			"pathSeason", pathSeason,
			"infoSeason", info.Season,
			"detectedSeason", detectedSeason)

		if detectedSeason > 0 {
			seasonMap[detectedSeason] = append(seasonMap[detectedSeason], path)
		} else {
			logger.Warn("Season not detected, defaulting to 1", "path", path)
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
	// 衍生节目加后缀区分
	if info.SpinOff != "" {
		newName += fmt.Sprintf(" [%s]", info.SpinOff)
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

// matchEpisode 智能匹配集数
// 匹配优先级：1. 期数+分集直接匹配 2. 播出日期匹配 3. 回退到解析的集数
func (rs *RenameSuggester) matchEpisode(info *MediaInfo, episodes []tmdb.Episode, logPrefix string) (int, string) {
	// 策略1：如果有期数和分集信息，直接在集名中查找匹配
	if info.BaseEpisode > 0 && info.Part != "" {
		if ep := rs.findEpisodeByBaseAndPart(episodes, info.BaseEpisode, info.Part); ep != nil {
			logger.Info(logPrefix+"matched by base episode and part",
				"baseEpisode", info.BaseEpisode, "part", info.Part,
				"episode", ep.EpisodeNumber, "episodeName", ep.Name)
			return ep.EpisodeNumber, ep.Name
		}
	}

	// 策略2：使用播出日期匹配
	if info.AirDate != "" {
		if ep, name := rs.matchByAirDate(info, episodes, logPrefix); ep > 0 {
			return ep, name
		}
	}

	// 策略3：回退到解析的集数
	if info.Episode > 0 && info.Episode <= len(episodes) {
		ep := episodes[info.Episode-1]
		logger.Info(logPrefix+"fallback to parsed episode",
			"episode", ep.EpisodeNumber, "episodeName", ep.Name)
		return ep.EpisodeNumber, ep.Name
	}

	return info.Episode, ""
}

// findEpisodeByBaseAndPart 根据期数和分集标记直接查找集数
func (rs *RenameSuggester) findEpisodeByBaseAndPart(episodes []tmdb.Episode, baseEpisode int, part string) *tmdb.Episode {
	for i := range episodes {
		epNum, epPart := rs.extractEpisodeInfo(episodes[i].Name)
		if epNum == baseEpisode && epPart == part {
			return &episodes[i]
		}
	}
	return nil
}

// extractEpisodeInfo 从集名中提取期数和分集标记
func (rs *RenameSuggester) extractEpisodeInfo(episodeName string) (int, string) {
	// 匹配 "第X期上/中/下" 格式
	partRegex := regexp.MustCompile(`第(\d+)期([上中下])`)
	if match := partRegex.FindStringSubmatch(episodeName); len(match) > 2 {
		if epNum, err := strconv.Atoi(match[1]); err == nil {
			return epNum, match[2]
		}
	}
	return 0, ""
}

// matchByAirDate 根据播出日期匹配
func (rs *RenameSuggester) matchByAirDate(info *MediaInfo, episodes []tmdb.Episode, logPrefix string) (int, string) {
	var sameDateEpisodes []tmdb.Episode
	for _, ep := range episodes {
		if ep.AirDate == info.AirDate {
			sameDateEpisodes = append(sameDateEpisodes, ep)
		}
	}

	if len(sameDateEpisodes) == 0 {
		return 0, ""
	}

	// 单集匹配
	if len(sameDateEpisodes) == 1 {
		ep := sameDateEpisodes[0]
		logger.Info(logPrefix+"matched by air date",
			"airDate", info.AirDate, "episode", ep.EpisodeNumber, "episodeName", ep.Name)
		return ep.EpisodeNumber, ep.Name
	}

	// 多集同天播出，使用 Part 选择
	if info.Part != "" {
		partIndex := rs.getPartIndex(info.Part, len(sameDateEpisodes))
		if partIndex < len(sameDateEpisodes) {
			ep := sameDateEpisodes[partIndex]
			logger.Info(logPrefix+"matched by air date and part",
				"airDate", info.AirDate, "part", info.Part,
				"episode", ep.EpisodeNumber, "episodeName", ep.Name)
			return ep.EpisodeNumber, ep.Name
		}
	}

	// 无法确定，返回第一集并警告
	ep := sameDateEpisodes[0]
	logger.Warn(logPrefix+"multiple episodes on same date, selecting first",
		"airDate", info.AirDate, "count", len(sameDateEpisodes), "episode", ep.EpisodeNumber)
	return ep.EpisodeNumber, ep.Name
}

// matchEpisodeByAirDate 保留旧接口，调用新的 matchEpisode
func (rs *RenameSuggester) matchEpisodeByAirDate(info *MediaInfo, episodes []tmdb.Episode, logPrefix string) (int, string) {
	return rs.matchEpisode(info, episodes, logPrefix)
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
