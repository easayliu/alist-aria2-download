package utils

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	strutil "github.com/easayliu/alist-aria2-download/pkg/utils/string"
	fileutil "github.com/easayliu/alist-aria2-download/pkg/utils/file"
)

// FileFilterService 文件过滤服务
type FileFilterService struct{}

// NewFileFilterService 创建文件过滤服务
func NewFileFilterService() *FileFilterService {
	return &FileFilterService{}
}

// IsVideoFile 检查文件名是否是视频文件（使用公共工具函数）
func (s *FileFilterService) IsVideoFile(fileName string) bool {
	return fileutil.IsVideoFile(fileName)
}

// IsTVShow 判断是否为电视剧
func (s *FileFilterService) IsTVShow(path string) bool {
	lowerPath := strings.ToLower(path)

	// 最明确的TV特征：S##格式（如S01, S02等）
	if s.hasSeasonPattern(lowerPath) {
		return true
	}

	// 检查中文季度标识
	if strings.Contains(lowerPath, "第") && strings.Contains(lowerPath, "季") {
		return true
	}

	// TV剧集的常见特征
	tvKeywords := []string{
		"tvs", "tv", "series", "season", "episode",
		"剧集", "集", "话", "动画", "番剧", "连续剧", "电视剧",
	}

	for _, keyword := range tvKeywords {
		if strings.Contains(lowerPath, keyword) {
			return true
		}
	}

	// 检查是否匹配S##E##格式
	seasonEpisodePatterns := []string{
		"s01e", "s02e", "s03e", "s04e", "s05e", "s06e", "s07e", "s08e", "s09e", "s10e",
		"s11e", "s12e", "s13e", "s14e", "s15e", "s16e", "s17e", "s18e", "s19e", "s20e",
	}
	for _, pattern := range seasonEpisodePatterns {
		if strings.Contains(lowerPath, pattern) {
			return true
		}
	}

	// 检查是否包含多集特征（如 EP01, E01等）- 使用更灵活的检测
	if s.hasEpisodePattern(path) {
		return true
	}

	// 检查文件名是否为纯数字集数格式（如 01.mp4, 02.mp4, 08.mp4）
	fileName := filepath.Base(path)
	fileNameNoExt := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	if s.isEpisodeNumber(fileNameNoExt) {
		return true
	}

	return false
}

// IsMovie 判断是否为电影 - 基于单个视频文件判断
func (s *FileFilterService) IsMovie(path string) bool {
	// 提取文件名
	fileName := filepath.Base(path)

	// 首先检查是否为视频文件
	if !s.IsVideoFile(fileName) {
		return false
	}

	// 如果是视频文件，且不包含强TV特征，则认为是电影
	return !s.hasStrongTVIndicators(path)
}

// IsMovieSeries 检查是否为电影系列
func (s *FileFilterService) IsMovieSeries(path string) bool {
	// 检查路径中是否包含明确的电影系列标识
	movieSeriesKeywords := []string{
		"系列", "三部曲", "四部曲", "合集", "trilogy", "collection",
		"saga", "franchise", "series",
	}

	lowerPath := strings.ToLower(path)
	for _, keyword := range movieSeriesKeywords {
		if strings.Contains(path, keyword) || strings.Contains(lowerPath, keyword) {
			// 进一步检查是否真的是电影系列而不是TV剧集
			// 如果路径中包含年份，更可能是电影系列
			if s.hasYear(path) {
				return true
			}
			// 如果路径中不包含强TV特征，也认为是电影系列
			if !s.hasExplicitTVFeatures(path) {
				return true
			}
		}
	}

	return false
}

// HasStrongTVIndicators 检查是否有强烈的TV剧集特征
func (s *FileFilterService) HasStrongTVIndicators(path string) bool {
	return s.hasStrongTVIndicators(path)
}

// hasStrongTVIndicators 检查是否有强烈的TV剧集特征
func (s *FileFilterService) hasStrongTVIndicators(path string) bool {
	lowerPath := strings.ToLower(path)

	// 最强TV特征：S##格式（如S01, S02等）
	if s.hasSeasonPattern(lowerPath) {
		return true
	}

	// S##E##格式是明确的TV剧集标识
	seasonEpisodePatterns := []string{
		"s01e", "s02e", "s03e", "s04e", "s05e", "s06e", "s07e", "s08e", "s09e", "s10e",
		"s11e", "s12e", "s13e", "s14e", "s15e", "s16e", "s17e", "s18e", "s19e", "s20e",
	}
	for _, pattern := range seasonEpisodePatterns {
		if strings.Contains(lowerPath, pattern) {
			return true
		}
	}

	// 中文季度格式
	if strings.Contains(lowerPath, "第") && strings.Contains(lowerPath, "季") {
		return true
	}

	// 明确的季度关键词
	if strings.Contains(lowerPath, "season") {
		return true
	}

	// 检查路径中是否明确包含 tvs 或 series 目录
	if strings.Contains(lowerPath, "/tvs/") || strings.Contains(lowerPath, "/series/") {
		return true
	}

	// 检查文件名是否为纯数字集数格式（如 01.mp4, 02.mp4, 08.mp4）
	// 这是剧集的常见命名模式
	fileName := filepath.Base(path)
	fileNameNoExt := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	if s.isEpisodeNumber(fileNameNoExt) {
		return true
	}

	// 检查是否包含明确的集数标识（E##或EP##格式）- 使用更灵活的检测
	// 匹配 E01-E999, EP01-EP999 格式
	if s.hasEpisodePattern(path) {
		return true
	}
	
	// 检查是否是已知的TV节目/综艺节目
	if s.isKnownTVShow(path) {
		return true
	}

	// 其他强TV特征需要多个指示符组合
	strongIndicators := []string{
		"集", "话", "episode", "ep01", "ep02", "ep03", "e01", "e02", "e03",
	}

	matchCount := 0
	for _, indicator := range strongIndicators {
		if strings.Contains(lowerPath, indicator) {
			matchCount++
			if matchCount >= 2 {
				return true
			}
		}
	}

	return false
}

// hasExplicitTVFeatures 检查是否有明确的TV剧集特征（不包括"系列"）
func (s *FileFilterService) hasExplicitTVFeatures(path string) bool {
	lowerPath := strings.ToLower(path)

	// 检查S##E##格式
	seasonEpisodePatterns := []string{
		"s01e", "s02e", "s03e", "s04e", "s05e", "s06e", "s07e", "s08e", "s09e", "s10e",
		"s11e", "s12e", "s13e", "s14e", "s15e", "s16e", "s17e", "s18e", "s19e", "s20e",
	}
	for _, pattern := range seasonEpisodePatterns {
		if strings.Contains(lowerPath, pattern) {
			return true
		}
	}

	// 检查中文季度格式
	if strings.Contains(lowerPath, "第") && strings.Contains(lowerPath, "季") {
		return true
	}

	// 检查明确的季度关键词
	if strings.Contains(lowerPath, "season") {
		return true
	}

	// 检查明确的剧集关键词
	explicitTVKeywords := []string{
		"集", "话", "episode", "ep01", "ep02", "ep03", "e01", "e02", "e03",
		"/tvs/", "/series/", "剧集", "连续剧", "电视剧", "番剧",
	}

	for _, keyword := range explicitTVKeywords {
		if strings.Contains(lowerPath, keyword) {
			return true
		}
	}

	return false
}

// hasSeasonPattern 检查是否包含季度模式（使用预编译正则）
func (s *FileFilterService) hasSeasonPattern(str string) bool {
	// 使用预编译正则匹配季度格式
	matches := strutil.SeasonPatternCI.FindStringSubmatch(str)
	if len(matches) > 2 {
		// 提取季度数字
		if seasonNum, err := strconv.Atoi(matches[2]); err == nil {
			// 季度在合理范围内（1-99）
			return seasonNum >= 1 && seasonNum <= 99
		}
	}

	return false
}

// hasEpisodePattern 检查是否包含集数模式（E01, EP01, E74等）（使用预编译正则）
func (s *FileFilterService) hasEpisodePattern(path string) bool {
	// 使用预编译正则匹配集数格式
	matches := strutil.EpisodePatternCI.FindStringSubmatch(path)
	if len(matches) > 3 {
		// 提取集数（第3个捕获组是数字）
		if episodeNum, err := strconv.Atoi(matches[3]); err == nil {
			// 集数在合理范围内（1-999）
			return episodeNum >= 1 && episodeNum <= 999
		}
	}

	return false
}

// isEpisodeNumber 检查是否为纯数字的集数格式
func (s *FileFilterService) isEpisodeNumber(name string) bool {
	// 去除可能的前导零
	name = strings.TrimSpace(name)

	// 检查是否为纯数字（可能有前导零）
	if len(name) == 0 || len(name) > 4 {
		return false
	}

	// 检查是否全部为数字
	for _, ch := range name {
		if ch < '0' || ch > '9' {
			return false
		}
	}

	// 转换为数字检查范围
	if num, err := strconv.Atoi(name); err == nil {
		// 集数通常在 1-999 范围内
		return num >= 1 && num <= 999
	}

	return false
}

// HasSeasonEpisodePattern 检查文件名是否包含S##EP##格式
func (s *FileFilterService) HasSeasonEpisodePattern(fileName string) bool {
	return s.hasSeasonEpisodePattern(fileName)
}

// hasSeasonEpisodePattern 检查文件名是否包含S##EP##格式
func (s *FileFilterService) hasSeasonEpisodePattern(fileName string) bool {
	// 匹配 S01EP01, S01EP76 等格式
	matched, _ := regexp.MatchString(`(?i)S\d{1,2}EP\d{1,3}`, fileName)
	return matched
}

// isKnownTVShow 检查是否是已知的TV节目或综艺节目
func (s *FileFilterService) isKnownTVShow(path string) bool {
	// 已知的TV节目/综艺节目名称列表
	knownTVShows := []string{
		"喜人奇妙夜",
		"快乐大本营",
		"天天向上",
		"向往的生活",
		"奔跑吧",
		"极限挑战",
		"王牌对王牌",
		"明星大侦探",
		"乘风破浪",
		"爸爸去哪儿",
		"中国好声音",
		"我是歌手",
		"蒙面歌王",
		"这就是街舞",
		"创造营",
		"青春有你",
		"脱口秀大会",
		"吐槽大会",
	}
	
	for _, show := range knownTVShows {
		if strings.Contains(path, show) {
			return true
		}
	}
	
	// 检查是否包含综艺节目的常见模式
	varietyPatterns := []string{
		"先导",       // 先导片
		"纯享版",     // 纯享版
		"精华版",     // 精华版
		"加长版",     // 加长版
		"花絮",      // 花絮
		"彩蛋",      // 彩蛋
		"幕后",      // 幕后
	}
	
	for _, pattern := range varietyPatterns {
		if strings.Contains(path, pattern) {
			// 如果包含综艺特征词，很可能是综艺节目
			return true
		}
	}
	
	// 检查日期格式的节目（如 20240628, 20250919）（使用预编译正则）
	// 这种格式通常是综艺节目
	fileName := filepath.Base(path)
	if strutil.DatePattern.MatchString(fileName) {
		// 如果文件名包含8位日期格式（YYYYMMDD），很可能是综艺节目
		return true
	}
	
	return false
}

// IsVarietyShow 检查是否为综艺节目
func (s *FileFilterService) IsVarietyShow(path string) bool {
	// 已知的综艺节目名称列表
	knownVarietyShows := []string{
		"喜人奇妙夜",
		"快乐大本营",
		"天天向上",
		"向往的生活",
		"奔跑吧",
		"极限挑战",
		"王牌对王牌",
		"明星大侦探",
		"乘风破浪",
		"爸爸去哪儿",
		"中国好声音",
		"我是歌手",
		"蒙面歌王",
		"这就是街舞",
		"创造营",
		"青春有你",
		"脱口秀大会",
		"吐槽大会",
	}
	
	// 检查是否包含已知综艺节目名称
	for _, show := range knownVarietyShows {
		if strings.Contains(path, show) {
			return true
		}
	}
	
	// 检查综艺特征词
	varietyPatterns := []string{
		"先导",       // 先导片
		"纯享版",     // 纯享版
		"精华版",     // 精华版
		"加长版",     // 加长版
		"花絮",      // 花絮
		"彩蛋",      // 彩蛋
		"幕后",      // 幕后
		"复盘",      // 复盘
	}
	
	for _, pattern := range varietyPatterns {
		if strings.Contains(path, pattern) {
			return true
		}
	}
	
	// 检查日期格式的节目（如 20240628, 20250919）（使用预编译正则）
	fileName := filepath.Base(path)
	if strutil.DatePattern.MatchString(fileName) {
		return true
	}
	
	// 检查路径中是否包含综艺相关目录
	lowerPath := strings.ToLower(path)
	varietyDirs := []string{"/variety/", "/show/", "/综艺/", "/娱乐/"}
	for _, dir := range varietyDirs {
		if strings.Contains(lowerPath, dir) {
			return true
		}
	}
	
	return false
}

// HasYear 检查路径是否包含年份
func (s *FileFilterService) HasYear(path string) bool {
	return s.hasYear(path)
}

// hasYear 检查路径是否包含年份
func (s *FileFilterService) hasYear(path string) bool {
	// 简单检查是否包含19xx或20xx格式的年份
	for i := 1900; i <= 2099; i++ {
		year := strconv.Itoa(i)
		if strings.Contains(path, "("+year+")") ||
			strings.Contains(path, "["+year+"]") ||
			strings.Contains(path, "."+year+".") ||
			strings.Contains(path, " "+year+" ") ||
			strings.Contains(path, year) {
			return true
		}
	}
	return false
}

// IsVersionDirectory 检查是否为版本/质量目录
func (s *FileFilterService) IsVersionDirectory(dir string) bool {
	// 包含方括号通常表示版本/质量信息
	if strings.Contains(dir, "[") && strings.Contains(dir, "]") {
		return true
	}
	
	// 检查常见的版本/质量关键词
	versionKeywords := []string{
		"4K", "1080P", "1080p", "720P", "720p",
		"BluRay", "BDRip", "WEBRip", "HDTV", "WEB-DL",
		"60帧", "高码率", "DV", "HDR", "H265", "H264",
		"AAC", "DTS", "REMUX", "2160p",
	}
	
	for _, keyword := range versionKeywords {
		if strings.Contains(dir, keyword) {
			return true
		}
	}
	
	// 检查复杂的编码格式目录（包含季度信息但主要是技术格式）
	// 如：S08.2025.2160p.WEB-DL.H265.AAC
	if strings.Contains(dir, ".") && (
		strings.Contains(dir, "p.") || // 分辨率格式
		strings.Contains(dir, "WEB") || 
		strings.Contains(dir, "BluRay") ||
		strings.Contains(dir, "H26")) {
		return true
	}
	
	return false
}