package services

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// MediaType 媒体类型
type MediaType string

const (
	MediaTypeTV    MediaType = "tv"
	MediaTypeMovie MediaType = "movie"
	MediaTypeOther MediaType = "other"
)

// YesterdayFileInfo 昨天文件信息
type YesterdayFileInfo struct {
	Name         string    `json:"name"`
	Path         string    `json:"path"`
	Size         int64     `json:"size"`
	Modified     time.Time `json:"modified"`
	OriginalURL  string    `json:"original_url"`
	InternalURL  string    `json:"internal_url"`
	MediaType    MediaType `json:"media_type"`
	DownloadPath string    `json:"download_path"`
}

// FileInfo 文件信息
type FileInfo struct {
	Name         string    `json:"name"`
	Path         string    `json:"path"`
	Size         int64     `json:"size"`
	Modified     time.Time `json:"modified"`
	OriginalURL  string    `json:"original_url"`
	InternalURL  string    `json:"internal_url"`
	MediaType    MediaType `json:"media_type"`
	DownloadPath string    `json:"download_path"`
}

// FileMediaService 媒体文件服务
type FileMediaService struct {
	filterSvc *FileFilterService
	pathSvc   *FilePathService
}

// NewFileMediaService 创建媒体文件服务
func NewFileMediaService(filterSvc *FileFilterService, pathSvc *FilePathService) *FileMediaService {
	return &FileMediaService{
		filterSvc: filterSvc,
		pathSvc:   pathSvc,
	}
}

// DetermineMediaTypeAndPath 根据文件路径判断媒体类型并生成下载路径（公开方法）
func (s *FileMediaService) DetermineMediaTypeAndPath(fullPath, fileName string) (MediaType, string) {
	return s.determineMediaTypeAndPath(fullPath, fileName)
}

// GetMediaType 获取文件的媒体类型（用于统计）
func (s *FileMediaService) GetMediaType(filePath string) string {
	mediaType, _ := s.determineMediaTypeAndPath(filePath, filePath)
	switch mediaType {
	case MediaTypeMovie:
		return "movie"
	case MediaTypeTV:
		return "tv"
	default:
		return "other"
	}
}

// determineMediaTypeAndPath 根据文件路径判断媒体类型并生成下载路径
func (s *FileMediaService) determineMediaTypeAndPath(fullPath, fileName string) (MediaType, string) {
	// 需要同时检查原始路径和小写路径
	lowerPath := strings.ToLower(fullPath)

	// 检查是否是单文件目录（通过文件名包含的扩展名判断）
	if s.filterSvc.IsVideoFile(fileName) {
		// 首先检查是否为电影系列 - 电影系列优先级最高
		if s.filterSvc.IsMovieSeries(fullPath) {
			movieName := s.extractMovieName(fullPath)
			if movieName != "" {
				downloadPath := "/downloads/movies/" + movieName
				return MediaTypeMovie, downloadPath
			}
		}

		// 然后检查是否为TV剧集
		if s.filterSvc.IsTVShow(fullPath) || s.filterSvc.HasStrongTVIndicators(fullPath) || s.filterSvc.HasStrongTVIndicators(lowerPath) {
			// 特殊处理：如果文件名包含S##EP##格式，使用特殊的路径提取逻辑
			if s.filterSvc.HasSeasonEpisodePattern(fileName) {
				showName, versionPath := s.extractTVShowWithVersion(fullPath)
				if showName != "" {
					if versionPath != "" {
						downloadPath := "/downloads/tvs/" + showName + "/" + versionPath
						return MediaTypeTV, downloadPath
					}
					downloadPath := "/downloads/tvs/" + showName
					return MediaTypeTV, downloadPath
				}
			}
			
			// 提取剧集信息
			showName, seasonInfo := s.extractTVShowInfo(fullPath)
			if showName != "" && seasonInfo != "" {
				downloadPath := "/downloads/tvs/" + showName + "/" + seasonInfo
				return MediaTypeTV, downloadPath
			}
			if showName != "" {
				downloadPath := "/downloads/tvs/" + showName
				return MediaTypeTV, downloadPath
			}
			downloadPath := "/downloads/tvs/" + s.pathSvc.ExtractFolderName(fullPath)
			return MediaTypeTV, s.pathSvc.ApplyPathMapping(fullPath, downloadPath)
		}

		// 单个视频文件，默认判定为电影
		movieName := s.extractMovieName(fullPath)
		if movieName != "" {
			downloadPath := "/downloads/movies/" + movieName
			return MediaTypeMovie, downloadPath
		}
		downloadPath := "/downloads/movies"
		return MediaTypeMovie, s.pathSvc.ApplyPathMapping(fullPath, downloadPath)
	}

	// 判断是否为电影
	if s.filterSvc.IsMovie(lowerPath) || s.filterSvc.IsMovie(fullPath) {
		// 提取电影名称或系列名称
		movieName := s.extractMovieName(fullPath)
		if movieName != "" {
			downloadPath := "/downloads/movies/" + movieName
			return MediaTypeMovie, downloadPath
		}
		downloadPath := "/downloads/movies"
		return MediaTypeMovie, s.pathSvc.ApplyPathMapping(fullPath, downloadPath)
	}

	// 判断是否为TV剧集
	if s.filterSvc.IsTVShow(fullPath) || s.filterSvc.HasStrongTVIndicators(fullPath) || s.filterSvc.HasStrongTVIndicators(lowerPath) {
		// 提取剧集信息
		showName, seasonInfo := s.extractTVShowInfo(fullPath)
		if showName != "" && seasonInfo != "" {
			downloadPath := "/downloads/tvs/" + showName + "/" + seasonInfo
			return MediaTypeTV, downloadPath
		}
		if showName != "" {
			downloadPath := "/downloads/tvs/" + showName
			return MediaTypeTV, downloadPath
		}
		downloadPath := "/downloads/tvs/" + s.pathSvc.ExtractFolderName(fullPath)
		return MediaTypeTV, s.pathSvc.ApplyPathMapping(fullPath, downloadPath)
	}

	// 默认其他类型
	mediaType := MediaTypeOther
	downloadPath := "/downloads"
	
	// 应用源路径到下载路径的映射
	return mediaType, s.pathSvc.ApplyPathMapping(fullPath, downloadPath)
}

// extractMovieName 提取电影名称或系列名称
func (s *FileMediaService) extractMovieName(fullPath string) string {
	parts := strings.Split(fullPath, "/")

	var seriesName string
	var movieName string

	// 遍历路径部分，识别系列和具体电影
	for _, part := range parts {
		// 跳过系统目录和通用目录名
		if part == "data" || part == "来自：分享" || part == "/" || part == "" ||
			part == "movies" || part == "films" || part == "movie" {
			continue
		}

		// 查找系列/合集目录（优先级高）
		if strings.Contains(part, "系列") || strings.Contains(part, "合集") ||
			strings.Contains(part, "trilogy") || strings.Contains(part, "collection") {
			// 提取系列名称
			seriesName = s.extractSeriesName(part)
		}

		// 查找包含年份的部分（通常是具体电影）
		if s.filterSvc.HasYear(part) && movieName == "" {
			// 提取具体电影名称
			movieName = s.extractCleanMovieName(part)
		}
	}

	// 如果找到系列名称，优先使用系列名称作为目录
	if seriesName != "" {
		return seriesName
	}

	// 如果找到具体电影名称，使用电影名称
	if movieName != "" {
		return movieName
	}

	// 如果都没找到，尝试从第一个有意义的目录提取
	// 对于电影，如果是单个文件，尝试从文件名提取
	fileName := filepath.Base(fullPath)
	if s.filterSvc.IsVideoFile(fileName) {
		// 从文件名提取电影名
		cleanName := s.extractCleanMovieName(fileName)
		if cleanName != "" {
			return cleanName
		}
	}

	// 从目录名提取
	for _, part := range parts {
		if part != "" && part != "data" && part != "来自：分享" && part != "/" &&
			part != "movies" && part != "films" && part != "movie" {
			cleanName := s.extractMainShowName(part)
			if cleanName != "" {
				return cleanName
			}
		}
	}

	return ""
}

// extractCleanMovieName 提取干净的电影名称
func (s *FileMediaService) extractCleanMovieName(name string) string {
	// 去除文件扩展名
	cleanName := name
	if strings.Contains(cleanName, ".") {
		ext := filepath.Ext(cleanName)
		cleanName = strings.TrimSuffix(cleanName, ext)
	}

	// 去除年份 (如 (2014) 或 [2014] 或 .2014.)
	if idx := strings.Index(cleanName, "("); idx > 0 {
		yearPart := cleanName[idx:]
		if s.filterSvc.HasYear(yearPart) {
			cleanName = cleanName[:idx]
		}
	}

	// 去除方括号内容
	if idx := strings.Index(cleanName, "["); idx > 0 {
		cleanName = cleanName[:idx]
	}

	// 去除点分隔的年份格式 (如 Avatar.2022.4K)
	parts := strings.Split(cleanName, ".")
	var cleanParts []string
	for _, part := range parts {
		// 如果这个部分是年份，停止收集
		if s.filterSvc.HasYear(part) || len(part) == 4 && s.isYear(part) {
			break
		}
		cleanParts = append(cleanParts, part)
	}
	if len(cleanParts) > 0 {
		cleanName = strings.Join(cleanParts, ".")
	}

	// 去除格式信息
	patterns := []string{
		" 4K", " 1080P", " 1080p", " 720P", " 720p",
		" BluRay", " REMUX", " BDRip", " WEBRip", " HDTV",
		" 蓝光原盘", " 中文字幕", " 国英双语",
		".4K", ".1080P", ".1080p", ".720P", ".720p",
		".BluRay", ".REMUX", ".BDRip", ".WEBRip", ".HDTV",
	}

	for _, pattern := range patterns {
		cleanName = strings.ReplaceAll(cleanName, pattern, "")
	}

	// 将点替换为空格（电影名通常用点分隔）
	cleanName = strings.ReplaceAll(cleanName, ".", " ")
	cleanName = strings.TrimSpace(cleanName)

	// 清理文件系统不友好的字符
	return s.pathSvc.CleanFolderName(cleanName)
}

// extractSeriesName 提取系列名称
func (s *FileMediaService) extractSeriesName(name string) string {
	// 提取系列名称的主要部分
	cleanName := name

	// 处理 "XXX系列" 格式 - 保留"系列"前面的内容
	if idx := strings.Index(cleanName, "系列"); idx > 0 {
		// 提取"系列"前面的内容作为系列名
		cleanName = strings.TrimSpace(cleanName[:idx])
		// 如果提取出的名称有效，直接返回
		if cleanName != "" {
			return s.pathSvc.CleanFolderName(cleanName)
		}
	}

	// 处理其他格式标记
	markers := []string{
		"合集", "trilogy", "collection",
		" (", " [", " -", " +",
	}

	minIndex := len(cleanName)
	for _, marker := range markers {
		if idx := strings.Index(cleanName, marker); idx > 0 && idx < minIndex {
			minIndex = idx
		}
	}

	if minIndex < len(cleanName) {
		cleanName = cleanName[:minIndex]
	}

	cleanName = strings.TrimSpace(cleanName)

	// 如果清理后的名称太短或为空，返回原始名称的简化版本
	if len(cleanName) < 2 {
		// 尝试提取第一个有意义的词
		parts := strings.Fields(name)
		if len(parts) > 0 {
			cleanName = parts[0]
		}
	}

	// 清理文件系统不友好的字符
	return s.pathSvc.CleanFolderName(cleanName)
}

// isYear 检查字符串是否为年份
func (s *FileMediaService) isYear(str string) bool {
	if year, err := strconv.Atoi(str); err == nil {
		return year >= 1900 && year <= 2099
	}
	return false
}

// extractTVShowInfo 提取电视剧信息
func (s *FileMediaService) extractTVShowInfo(fullPath string) (showName, seasonInfo string) {
	parts := strings.Split(fullPath, "/")
	
	// 首先检查文件名是否包含S##E##格式，如果有，优先使用
	fileName := filepath.Base(fullPath)
	if seasonFromFile := s.extractSeasonFromFileName(fileName); seasonFromFile != "" {
		seasonInfo = seasonFromFile
	}

	// 收集所有包含季度信息的部分，按距离文件的远近排序（近的优先）
	var seasonParts []struct {
		index      int
		part       string
		seasonNum  int
		seasonStr  string
	}

	// 从后往前遍历，离文件越近的季度信息优先级越高
	for i := len(parts) - 1; i >= 0; i-- {
		part := parts[i]
		
		// 检查中文季度格式 "第 X 季"
		if strings.Contains(part, "第") && strings.Contains(part, "季") {
			if extractedSeason := s.extractSeasonFromChinese(part); extractedSeason != "" {
				// 提取季度数字用于比较
				seasonNum := s.parseSeasonNumber(extractedSeason)
				seasonParts = append(seasonParts, struct {
					index      int
					part       string
					seasonNum  int
					seasonStr  string
				}{i, part, seasonNum, extractedSeason})
			}
		}

		// 检查英文格式 Season X 或 S## 或 s1 等格式
		if s.isSeasonDirectory(part) {
			if extractedSeason := s.extractSeasonNumber(part); extractedSeason != "" {
				seasonNum := s.parseSeasonNumber(extractedSeason)
				seasonParts = append(seasonParts, struct {
					index      int
					part       string
					seasonNum  int
					seasonStr  string
				}{i, part, seasonNum, extractedSeason})
			}
		}
	}

	// 如果找到多个季度信息，优先使用距离文件最近且数字较大的
	if len(seasonParts) > 0 {
		// 选择最优的季度信息（距离文件最近的，如果距离相同则选择数字较大的）
		bestSeason := seasonParts[0]
		for _, sp := range seasonParts[1:] {
			// 距离文件更近的优先
			if sp.index > bestSeason.index {
				bestSeason = sp
			} else if sp.index == bestSeason.index && sp.seasonNum > bestSeason.seasonNum {
				// 距离相同时，选择数字较大的
				bestSeason = sp
			}
		}
		
		if seasonInfo == "" {
			seasonInfo = bestSeason.seasonStr
		}
		// 获取剧集名称
		showName = s.extractShowNameFromPath(parts, bestSeason.index)
		if showName != "" {
			return
		}
	}

	// 如果没有找到明确的季度信息，尝试从路径提取剧名
	showName = s.extractShowNameFromFullPath(fullPath)
	if seasonInfo == "" {
		// 检查是否为综艺节目，综艺节目不添加默认季度
		if !s.filterSvc.IsVarietyShow(fullPath) {
			seasonInfo = "S1" // 默认第一季
		}
	}

	return
}

// extractSeasonFromFileName 从文件名提取季度信息（S##E##格式）
func (s *FileMediaService) extractSeasonFromFileName(fileName string) string {
	// 匹配 S01E01, S##E## 格式
	seasonEpRegex := regexp.MustCompile(`(?i)S(\d{1,2})E\d{1,3}`)
	matches := seasonEpRegex.FindStringSubmatch(fileName)
	
	if len(matches) > 1 {
		if seasonNum, err := strconv.Atoi(matches[1]); err == nil {
			if seasonNum < 10 {
				return fmt.Sprintf("S0%d", seasonNum)
			}
			return fmt.Sprintf("S%d", seasonNum)
		}
	}
	
	return ""
}

// isSeasonDirectory 检查是否为季度目录
func (s *FileMediaService) isSeasonDirectory(dir string) bool {
	lowerDir := strings.ToLower(dir)
	
	// 检查是否为纯季度目录名（s1, s01, season1, season 1 等）
	// 匹配模式：s1, s01, season1, season 1, 第1季, 第一季 等
	patterns := []string{
		`^s\d{1,2}$`,           // s1, s01
		`^season\s*\d{1,2}$`,   // season1, season 1
		`^第.{1,2}季$`,         // 第1季, 第一季
	}
	
	for _, pattern := range patterns {
		if matched, _ := regexp.MatchString(pattern, lowerDir); matched {
			return true
		}
	}
	
	return false
}

// extractShowNameFromPath 从路径部分提取剧集名称
func (s *FileMediaService) extractShowNameFromPath(parts []string, seasonIndex int) string {
	// 优先查找包含剧名的上级目录
	skipDirs := map[string]bool{
		"": true, ".": true, "..": true, "/": true,
		"data": true, "来自：分享": true,
		"tvs": true, "series": true, "movies": true, "films": true,
		"tv": true, "movie": true, "video": true, "videos": true,
		"anime": true, "动画": true, "长篇剧": true, "drama": true,
		"download": true, "downloads": true, "media": true,
		"variety": true, "shows": true, "综艺": true,
	}

	// 从季度目录向前查找，优先选择有意义的剧名目录
	var candidateNames []string
	
	for i := seasonIndex - 1; i >= 0; i-- {
		part := parts[i]
		// 跳过系统目录及通用分类目录
		if skipDirs[part] || skipDirs[strings.ToLower(part)] {
			continue
		}
		
		// 检查是否是版本/质量目录（通常不是剧名）
		if s.filterSvc.IsVersionDirectory(part) {
			continue
		}
		
		// 提取候选剧名
		cleanName := s.extractMainShowName(part)
		if cleanName != "" {
			candidateNames = append(candidateNames, cleanName)
		}
	}
	
	// 如果有多个候选剧名，选择最合适的
	if len(candidateNames) > 0 {
		// 优先选择不包含"全"、"合集"等集合标识的剧名
		for _, name := range candidateNames {
			if !strings.Contains(name, "全") && !strings.Contains(name, "合集") && 
			   !strings.Contains(name, "1-") && !strings.Contains(name, "1~") {
				return name
			}
		}
		
		// 其次选择不包含季度信息的剧名
		for _, name := range candidateNames {
			if !strings.Contains(name, "第") || !strings.Contains(name, "季") {
				return name
			}
		}
		
		// 最后返回第一个
		return candidateNames[0]
	}
	
	return ""
}

// extractShowNameFromFullPath 从完整路径提取剧名
func (s *FileMediaService) extractShowNameFromFullPath(fullPath string) string {
	parts := strings.Split(fullPath, "/")

	// 从路径中找到最可能是剧名的部分，跳过系统目录和通用目录名
	skipDirs := map[string]bool{
		"data": true, "来自：分享": true, "/": true, "": true,
		"tvs": true, "series": true, "movies": true, "films": true,
		"tv": true, "movie": true, "video": true, "videos": true,
		"anime": true, "动画": true, "长篇剧": true, "drama": true,
		"download": true, "downloads": true, "media": true,
		"variety": true, "shows": true, "综艺": true,  // 跳过variety/shows这类通用类别目录
	}

	for _, part := range parts {
		// 跳过系统目录、空目录和通用目录名
		if skipDirs[part] || skipDirs[strings.ToLower(part)] {
			continue
		}

		// 提取主要剧名（移除合集、版本等后缀信息）
		cleanName := s.extractMainShowName(part)
		if cleanName != "" {
			return cleanName
		}
	}

	return "unknown"
}

// extractMainShowName 提取主要剧名（移除版本信息等）
func (s *FileMediaService) extractMainShowName(name string) string {
	// 移除常见的版本和格式信息  
	patterns := []string{
		" 三季合集",
		" 合集",
		" 全1-3季",
		" 全1~3季", 
		" 全集",
		" 1080P",
		" 1080p",
		" 720P",
		" 720p",
		" BluRay",
		" REMUX",
		" BDRip",
		" WEBRip",
		" HDTV",
		"[",
		"(",
	}

	cleanName := name
	for _, pattern := range patterns {
		if idx := strings.Index(cleanName, pattern); idx > 0 {
			cleanName = cleanName[:idx]
		}
	}

	cleanName = strings.TrimSpace(cleanName)

	// 去除类似"第八季"、"第二季"的季度后缀，保留纯剧名
	seasonSuffixRegex := regexp.MustCompile(`(?i)\s*第[\p{Han}\d]{1,4}季.*$`)
	if seasonSuffixRegex.MatchString(cleanName) {
		cleanName = seasonSuffixRegex.ReplaceAllString(cleanName, "")
		cleanName = strings.TrimSpace(cleanName)
	}
	
	// 处理括号内的年份等信息（如"毛骗 第二季 (2011)"）
	if idx := strings.Index(cleanName, "("); idx > 0 {
		cleanName = strings.TrimSpace(cleanName[:idx])
	}

	// 特殊处理：标准化节目名称
	cleanName = s.standardizeShowName(cleanName)

	// 如果清理后的名称太短，返回原始名称
	if len(cleanName) < 2 {
		return s.pathSvc.CleanFolderName(name)
	}

	return s.pathSvc.CleanFolderName(cleanName)
}

// standardizeShowName 标准化节目名称，处理同一节目的不同命名方式
func (s *FileMediaService) standardizeShowName(name string) string {
	// 标准化常见节目名称
	showNameMap := map[string]string{
		"大侦探": "明星大侦探",
		"明Xd侦探": "明星大侦探",
		"明星大侦探": "明星大侦探",
	}
	
	// 检查是否需要标准化
	for variant, standard := range showNameMap {
		if strings.Contains(name, variant) {
			return standard
		}
	}
	
	return name
}

// extractSeasonFromChinese 从中文格式提取季度
func (s *FileMediaService) extractSeasonFromChinese(part string) string {
	// 处理 "第 0 季", "第 1 季", "第一季" 等格式
	if strings.Contains(part, "第") && strings.Contains(part, "季") {
		// 提取数字
		start := strings.Index(part, "第") + len("第")
		end := strings.Index(part, "季")
		if start > 0 && end > start {
			seasonStr := strings.TrimSpace(part[start:end])

			// 尝试解析数字
			seasonNum := s.parseChineseNumber(seasonStr)
			if seasonNum >= 0 {
				if seasonNum < 10 {
					return "S0" + strconv.Itoa(seasonNum)
				}
				return "S" + strconv.Itoa(seasonNum)
			}
		}
	}
	return "S1"
}

// parseChineseNumber 解析中文数字或阿拉伯数字
func (s *FileMediaService) parseChineseNumber(str string) int {
	str = strings.TrimSpace(str)

	// 先尝试直接解析阿拉伯数字
	if num, err := strconv.Atoi(str); err == nil {
		return num
	}

	// 解析中文数字
	chineseNumbers := map[string]int{
		"零": 0, "一": 1, "二": 2, "三": 3, "四": 4,
		"五": 5, "六": 6, "七": 7, "八": 8, "九": 9,
		"十": 10, "十一": 11, "十二": 12, "十三": 13, "十四": 14,
		"十五": 15, "十六": 16, "十七": 17, "十八": 18, "十九": 19,
		"二十": 20,
	}

	if num, ok := chineseNumbers[str]; ok {
		return num
	}

	return -1
}

// parseSeasonNumber 从季度字符串中解析出数字（如 "S08" -> 8, "S1" -> 1）
func (s *FileMediaService) parseSeasonNumber(seasonStr string) int {
	// 移除 S 前缀
	if strings.HasPrefix(seasonStr, "S") {
		numStr := strings.TrimPrefix(seasonStr, "S")
		if num, err := strconv.Atoi(numStr); err == nil {
			return num
		}
	}
	return 1 // 默认第一季
}

// extractSeasonNumber 提取季度编号
func (s *FileMediaService) extractSeasonNumber(part string) string {
	lowerPart := strings.ToLower(part)

	// 只从简单的季度目录中提取，避免从复杂格式中提取
	// 匹配 s1, s01, season1, season 1 等简单格式
	seasonRegex := regexp.MustCompile(`^(?:s|season\s*)(\d{1,2})$`)
	matches := seasonRegex.FindStringSubmatch(lowerPart)
	
	if len(matches) > 1 {
		if seasonNum, err := strconv.Atoi(matches[1]); err == nil {
			if seasonNum < 10 {
				return fmt.Sprintf("S0%d", seasonNum)
			}
			return fmt.Sprintf("S%d", seasonNum)
		}
	}
	
	// 如果没有找到，返回S1
	return "S1"
}

// extractTVShowWithVersion 从路径提取剧名和版本/质量路径
func (s *FileMediaService) extractTVShowWithVersion(fullPath string) (showName, versionPath string) {
	parts := strings.Split(fullPath, "/")
	
	// 查找包含版本/质量信息的目录（通常是文件的直接父目录）
	if len(parts) >= 2 {
		// 获取文件的直接父目录
		parentDir := parts[len(parts)-2]
		
		// 检查是否是版本/质量目录（包含[]或特定关键词）
		if s.filterSvc.IsVersionDirectory(parentDir) {
			// 对于混合季度和质量信息的目录，提取季度信息作为版本路径
			// 例如："第 1 季 - 2160p WEB-DL H265 AAC" -> 提取季度信息
			if strings.Contains(parentDir, "第") && strings.Contains(parentDir, "季") {
				// 提取季度信息并标准化
				seasonInfo := s.extractSeasonFromChinese(parentDir)
				if seasonInfo != "" && seasonInfo != "S1" {
					versionPath = seasonInfo
				}
			} else {
				// 纯质量目录（如 "4K[DV][60帧][高码率]"）才保留完整路径
				if strings.Contains(parentDir, "[") || 
				   (!strings.Contains(parentDir, "第") && !strings.Contains(parentDir, "季")) {
					versionPath = parentDir
				}
			}
			
			// 继续向上查找剧名
			if len(parts) >= 3 {
				// 获取上上级目录，可能是剧名
				possibleShowName := parts[len(parts)-3]
				// 清理剧名（去除版本信息）
				showName = s.extractCleanShowName(possibleShowName)
				if showName != "" {
					return showName, versionPath
				}
			}
		}
	}
	
	// 如果没找到版本目录，使用标准提取逻辑
	showName = s.extractShowNameFromFullPath(fullPath)
	return showName, ""
}

// extractCleanShowName 提取干净的剧名（去除版本信息）
func (s *FileMediaService) extractCleanShowName(name string) string {
	// 移除常见的版本后缀
	cleanName := name
	
	// 移除版本标识
	versionSuffixes := []string{
		"4K收藏版", "4K版", "高清版", "蓝光版",
		"完整版", "未删减版", "导演剪辑版",
		"收藏版", "珍藏版", "典藏版",
	}
	
	for _, suffix := range versionSuffixes {
		if strings.HasSuffix(cleanName, suffix) {
			cleanName = strings.TrimSuffix(cleanName, suffix)
			break
		}
	}
	
	// 如果名称太短，返回原始名称
	if len(cleanName) < 2 {
		cleanName = name
	}
	
	return s.pathSvc.CleanFolderName(cleanName)
}

// ProcessMovieDirectoryGrouping 处理电影类型的同目录下载逻辑
// 当目录中有电影文件时，将该目录下的所有其他文件也归类为电影并使用相同的下载路径
func (s *FileMediaService) ProcessMovieDirectoryGrouping(files *[]FileInfo) {
	if files == nil || len(*files) == 0 {
		return
	}

	// 按目录分组文件
	directoryGroups := make(map[string][]int) // 目录路径 -> 文件索引列表

	for i, file := range *files {
		// 获取文件的目录路径
		dir := filepath.Dir(file.Path)
		if directoryGroups[dir] == nil {
			directoryGroups[dir] = make([]int, 0)
		}
		directoryGroups[dir] = append(directoryGroups[dir], i)
	}

	// 处理每个目录
	for _, fileIndices := range directoryGroups {
		// 检查该目录是否包含电影类型的文件
		var moviePath string
		var hasMovie bool

		for _, idx := range fileIndices {
			if (*files)[idx].MediaType == MediaTypeMovie {
				hasMovie = true
				moviePath = (*files)[idx].DownloadPath
				break
			}
		}

		// 如果该目录包含电影文件，将该目录下的所有其他文件也设置为相同的电影下载路径
		if hasMovie && moviePath != "" {
			for _, idx := range fileIndices {
				file := &(*files)[idx]
				// 只修改非电影类型的文件，电影文件保持原样
				if file.MediaType != MediaTypeMovie {
					file.MediaType = MediaTypeMovie
					file.DownloadPath = moviePath
				}
			}
		}
	}
}

// ProcessYesterdayMovieDirectoryGrouping 处理昨天文件的电影类型同目录下载逻辑
// 当目录中有电影文件时，将该目录下的所有其他文件也归类为电影并使用相同的下载路径
func (s *FileMediaService) ProcessYesterdayMovieDirectoryGrouping(files *[]YesterdayFileInfo) {
	if files == nil || len(*files) == 0 {
		return
	}

	// 按目录分组文件
	directoryGroups := make(map[string][]int) // 目录路径 -> 文件索引列表

	for i, file := range *files {
		// 获取文件的目录路径
		dir := filepath.Dir(file.Path)
		if directoryGroups[dir] == nil {
			directoryGroups[dir] = make([]int, 0)
		}
		directoryGroups[dir] = append(directoryGroups[dir], i)
	}

	// 处理每个目录
	for _, fileIndices := range directoryGroups {
		// 检查该目录是否包含电影类型的文件
		var moviePath string
		var hasMovie bool

		for _, idx := range fileIndices {
			if (*files)[idx].MediaType == MediaTypeMovie {
				hasMovie = true
				moviePath = (*files)[idx].DownloadPath
				break
			}
		}

		// 如果该目录包含电影文件，将该目录下的所有其他文件也设置为相同的电影下载路径
		if hasMovie && moviePath != "" {
			for _, idx := range fileIndices {
				file := &(*files)[idx]
				// 只修改非电影类型的文件，电影文件保持原样
				if file.MediaType != MediaTypeMovie {
					file.MediaType = MediaTypeMovie
					file.DownloadPath = moviePath
				}
			}
		}
	}
}