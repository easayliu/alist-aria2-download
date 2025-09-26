package services

import (
	"strconv"
	"strings"
	"time"

	"github.com/easayliu/alist-aria2-download/internal/infrastructure/alist"
)

// FileService 文件服务
type FileService struct {
	alistClient *alist.Client
}

// NewFileService 创建文件服务
func NewFileService(alistClient *alist.Client) *FileService {
	return &FileService{
		alistClient: alistClient,
	}
}

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

// GetYesterdayFiles 获取昨天修改的文件
func (s *FileService) GetYesterdayFiles(basePath string) ([]YesterdayFileInfo, error) {
	var allYesterdayFiles []YesterdayFileInfo
	
	// 获取昨天的日期范围
	now := time.Now()
	yesterday := now.AddDate(0, 0, -1)
	yesterdayStart := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, yesterday.Location())
	yesterdayEnd := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 23, 59, 59, 999999999, yesterday.Location())
	
	// 递归获取文件
	if err := s.fetchYesterdayFilesRecursive(basePath, yesterdayStart, yesterdayEnd, &allYesterdayFiles); err != nil {
		return nil, err
	}
	
	return allYesterdayFiles, nil
}

// fetchYesterdayFilesRecursive 递归获取昨天的文件
func (s *FileService) fetchYesterdayFilesRecursive(path string, yesterdayStart, yesterdayEnd time.Time, result *[]YesterdayFileInfo) error {
	page := 1
	perPage := 100
	
	for {
		// 获取文件列表
		fileList, err := s.alistClient.ListFiles(path, page, perPage)
		if err != nil {
			return err
		}
		
		// 处理每个文件/目录
		for _, file := range fileList.Data.Content {
			// 解析修改时间
			modTime, err := time.Parse(time.RFC3339, file.Modified)
			if err != nil {
				continue
			}
			
			// 构建完整路径
			fullPath := file.Path
			if fullPath == "" {
				if path == "/" {
					fullPath = "/" + file.Name
				} else {
					fullPath = path + "/" + file.Name
				}
			}
			
			if file.IsDir {
				// 如果是目录，递归处理
				if err := s.fetchYesterdayFilesRecursive(fullPath, yesterdayStart, yesterdayEnd, result); err != nil {
					return err
				}
			} else {
				// 如果是文件，检查是否是昨天修改的
				if modTime.After(yesterdayStart) && modTime.Before(yesterdayEnd) {
					// 获取文件详细信息（包含下载链接）
					fileInfo, err := s.alistClient.GetFileInfo(fullPath)
					if err != nil {
						continue
					}
					
					// 替换URL
					originalURL := fileInfo.Data.RawURL
					internalURL := strings.Replace(originalURL, "fcalist-public", "fcalist-internal", -1)
					
					// 判断媒体类型并生成下载路径
					mediaType, downloadPath := s.determineMediaTypeAndPath(fullPath, file.Name)
					
					*result = append(*result, YesterdayFileInfo{
						Name:         file.Name,
						Path:         fullPath,
						Size:         file.Size,
						Modified:     modTime,
						OriginalURL:  originalURL,
						InternalURL:  internalURL,
						MediaType:    mediaType,
						DownloadPath: downloadPath,
					})
				}
			}
		}
		
		// 检查是否还有更多页
		if len(fileList.Data.Content) < perPage {
			break
		}
		page++
	}
	
	return nil
}

// determineMediaTypeAndPath 根据文件路径判断媒体类型并生成下载路径
func (s *FileService) determineMediaTypeAndPath(fullPath, fileName string) (MediaType, string) {
	// 需要同时检查原始路径和小写路径
	lowerPath := strings.ToLower(fullPath)
	
	// 检查是否是单文件目录（通过文件名包含的扩展名判断）
	if s.isSingleVideoFile(fileName) {
		// 单个视频文件通常是电影
		// 除非路径明确包含TV剧集特征
		if s.hasStrongTVIndicators(fullPath) || s.hasStrongTVIndicators(lowerPath) {
			// 有强TV特征，仍然判定为TV
			showName, seasonInfo := s.extractTVShowInfo(fullPath)
			if showName != "" && seasonInfo != "" {
				return MediaTypeTV, "/downloads/tvs/" + showName + "/" + seasonInfo
			}
			return MediaTypeTV, "/downloads/tvs/" + s.extractFolderName(fullPath) + "/S1"
		}
		// 单文件且无强TV特征，判定为电影
		movieName := s.extractMovieName(fullPath)
		if movieName != "" {
			return MediaTypeMovie, "/downloads/movies/" + movieName
		}
		return MediaTypeMovie, "/downloads/movies"
	}
	
	// 判断是否为TV剧集 - 使用原始路径以保留中文字符
	if s.isTVShow(fullPath) || s.isTVShow(lowerPath) {
		// 提取剧集信息
		showName, seasonInfo := s.extractTVShowInfo(fullPath)
		if showName != "" && seasonInfo != "" {
			return MediaTypeTV, "/downloads/tvs/" + showName + "/" + seasonInfo
		}
		return MediaTypeTV, "/downloads/tvs/" + s.extractFolderName(fullPath) + "/S1"
	}
	
	// 判断是否为电影
	if s.isMovie(lowerPath) || s.isMovie(fullPath) {
		// 提取电影名称或系列名称
		movieName := s.extractMovieName(fullPath)
		if movieName != "" {
			return MediaTypeMovie, "/downloads/movies/" + movieName
		}
		return MediaTypeMovie, "/downloads/movies"
	}
	
	// 默认其他类型
	return MediaTypeOther, "/downloads"
}

// isSingleVideoFile 检查文件名是否是视频文件
func (s *FileService) isSingleVideoFile(fileName string) bool {
	lowerName := strings.ToLower(fileName)
	videoExts := []string{
		".mp4", ".mkv", ".avi", ".mov", ".wmv", ".flv", ".webm",
		".m4v", ".mpg", ".mpeg", ".3gp", ".rmvb", ".ts", ".m2ts",
	}
	
	for _, ext := range videoExts {
		if strings.HasSuffix(lowerName, ext) {
			return true
		}
	}
	return false
}

// hasStrongTVIndicators 检查是否有强烈的TV剧集特征
func (s *FileService) hasStrongTVIndicators(path string) bool {
	lowerPath := strings.ToLower(path)
	
	// 强TV特征：明确的季度和集数标识
	strongIndicators := []string{
		"第", "季", "集", "话", "episode", "season",
		"s01e", "s02e", "s03e", // S##E## 格式
		"ep", "e01", "e02", // EP## 或 E## 格式
	}
	
	matchCount := 0
	for _, indicator := range strongIndicators {
		if strings.Contains(lowerPath, indicator) {
			matchCount++
			if matchCount >= 2 {
				// 至少匹配两个强特征才认为是TV
				return true
			}
		}
	}
	
	// 检查路径中是否明确包含 tvs 或 series 目录
	if strings.Contains(lowerPath, "/tvs/") || strings.Contains(lowerPath, "/series/") {
		return true
	}
	
	return false
}

// isTVShow 判断是否为电视剧
func (s *FileService) isTVShow(lowerPath string) bool {
	// 检查中文季度标识
	if strings.Contains(lowerPath, "第") && strings.Contains(lowerPath, "季") {
		return true
	}
	
	// TV剧集的常见特征
	tvKeywords := []string{
		"tvs", "tv", "series", "season", "s0", "s1", "s2", "s3", "s4", "s5",
		"episode", "e0", "e1", "ep", "剧集", "第", "季", "集", "话",
		"动画", "番剧", "连续剧", "电视剧",
	}
	
	for _, keyword := range tvKeywords {
		if strings.Contains(lowerPath, keyword) {
			return true
		}
	}
	
	// 检查是否匹配S##E##格式
	if strings.Contains(lowerPath, "s") && strings.Contains(lowerPath, "e") {
		return true
	}
	
	// 检查是否包含多集特征（如 EP01, E01等）
	if strings.Contains(lowerPath, "ep") || strings.Contains(lowerPath, " e") {
		return true
	}
	
	return false
}

// isMovie 判断是否为电影
func (s *FileService) isMovie(lowerPath string) bool {
	// 电影的常见特征
	movieKeywords := []string{
		"movies", "movie", "film", "电影", "影片", "系列",
		"trilogy", "三部曲", "合集", "collection", "蓝光原盘",
		"4k", "bluray", "bd", "dvd", "remux",
	}
	
	for _, keyword := range movieKeywords {
		if strings.Contains(lowerPath, keyword) {
			return true
		}
	}
	
	// 检查是否有年份（电影通常包含年份）
	if s.hasYear(lowerPath) && !s.isTVShow(lowerPath) {
		return true
	}
	
	return false
}

// extractMovieName 提取电影名称或系列名称
func (s *FileService) extractMovieName(fullPath string) string {
	parts := strings.Split(fullPath, "/")
	
	var seriesName string
	var movieName string
	
	// 遍历路径部分，识别系列和具体电影
	for _, part := range parts {
		// 跳过系统目录
		if part == "data" || part == "来自：分享" || part == "/" || part == "" {
			continue
		}
		
		// 查找系列/合集目录（优先级高）
		if strings.Contains(part, "系列") || strings.Contains(part, "合集") || 
		   strings.Contains(part, "trilogy") || strings.Contains(part, "collection") {
			// 提取系列名称
			seriesName = s.extractSeriesName(part)
		}
		
		// 查找包含年份的部分（通常是具体电影）
		if s.hasYear(part) && movieName == "" {
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
	for _, part := range parts {
		if part != "" && part != "data" && part != "来自：分享" && part != "/" {
			cleanName := s.extractMainShowName(part)
			if cleanName != "" {
				return cleanName
			}
		}
	}
	
	return ""
}

// extractCleanMovieName 提取干净的电影名称
func (s *FileService) extractCleanMovieName(name string) string {
	// 提取电影名称，去除年份和格式信息
	cleanName := name
	
	// 去除年份 (如 (2014) 或 [2014])
	if idx := strings.Index(cleanName, "("); idx > 0 {
		yearPart := cleanName[idx:]
		if s.hasYear(yearPart) {
			cleanName = cleanName[:idx]
		}
	}
	
	// 去除方括号内容
	if idx := strings.Index(cleanName, "["); idx > 0 {
		cleanName = cleanName[:idx]
	}
	
	// 去除格式信息
	patterns := []string{
		" 4K", " 1080P", " 1080p", " 720P", " 720p",
		" BluRay", " REMUX", " BDRip", " WEBRip", " HDTV",
		" 蓝光原盘", " 中文字幕", " 国英双语",
	}
	
	for _, pattern := range patterns {
		cleanName = strings.Replace(cleanName, pattern, "", -1)
	}
	
	cleanName = strings.TrimSpace(cleanName)
	
	// 清理文件系统不友好的字符
	return s.cleanFolderName(cleanName)
}

// extractSeriesName 提取系列名称
func (s *FileService) extractSeriesName(name string) string {
	// 提取系列名称的主要部分
	cleanName := name
	
	// 处理 "XXX系列" 格式 - 保留"系列"前面的内容
	if idx := strings.Index(cleanName, "系列"); idx > 0 {
		// 提取"系列"前面的内容作为系列名
		cleanName = strings.TrimSpace(cleanName[:idx])
		// 如果提取出的名称有效，直接返回
		if cleanName != "" {
			return s.cleanFolderName(cleanName)
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
	return s.cleanFolderName(cleanName)
}

// hasYear 检查路径是否包含年份
func (s *FileService) hasYear(path string) bool {
	// 简单检查是否包含19xx或20xx格式的年份
	for i := 1900; i <= 2099; i++ {
		year := strconv.Itoa(i)
		if strings.Contains(path, "("+year+")") ||
		   strings.Contains(path, "["+year+"]") ||
		   strings.Contains(path, "."+year+".") ||
		   strings.Contains(path, " "+year+" ") {
			return true
		}
	}
	return false
}

// extractTVShowInfo 提取电视剧信息
func (s *FileService) extractTVShowInfo(fullPath string) (showName, seasonInfo string) {
	parts := strings.Split(fullPath, "/")
	
	// 优先查找包含"第 X 季"格式的部分
	for i, part := range parts {
		// 检查中文季度格式 "第 X 季"
		if strings.Contains(part, "第") && strings.Contains(part, "季") {
			seasonInfo = s.extractSeasonFromChinese(part)
			// 获取剧集名称
			showName = s.extractShowNameFromPath(parts, i)
			if showName != "" {
				return
			}
		}
		
		// 检查英文格式 Season X 或 S##
		lowerPart := strings.ToLower(part)
		if strings.Contains(lowerPart, "season") || s.hasSeasonPattern(lowerPart) {
			seasonInfo = s.extractSeasonNumber(part)
			// 获取剧集名称
			showName = s.extractShowNameFromPath(parts, i)
			if showName != "" {
				return
			}
		}
	}
	
	// 如果没有找到明确的季度信息，尝试从路径提取剧名
	showName = s.extractShowNameFromFullPath(fullPath)
	if seasonInfo == "" {
		seasonInfo = "S1" // 默认第一季
	}
	
	return
}

// extractShowNameFromPath 从路径部分提取剧集名称
func (s *FileService) extractShowNameFromPath(parts []string, seasonIndex int) string {
	// 优先查找包含剧名的上级目录
	for i := seasonIndex - 1; i >= 0; i-- {
		part := parts[i]
		// 跳过系统目录
		if part == "data" || part == "来自：分享" || part == "/" || part == "" {
			continue
		}
		// 找到第一个有效的目录名作为剧名
		cleanName := s.extractMainShowName(part)
		if cleanName != "" {
			return cleanName
		}
	}
	return ""
}

// extractShowNameFromFullPath 从完整路径提取剧名
func (s *FileService) extractShowNameFromFullPath(fullPath string) string {
	parts := strings.Split(fullPath, "/")
	
	// 从路径中找到最可能是剧名的部分
	for _, part := range parts {
		// 跳过系统目录和空目录
		if part == "data" || part == "来自：分享" || part == "/" || part == "" {
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
func (s *FileService) extractMainShowName(name string) string {
	// 移除常见的版本和格式信息
	patterns := []string{
		" 三季合集",
		" 合集",
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
	
	// 如果清理后的名称太短，返回原始名称
	if len(cleanName) < 2 {
		return s.cleanFolderName(name)
	}
	
	return s.cleanFolderName(cleanName)
}

// extractSeasonFromChinese 从中文格式提取季度
func (s *FileService) extractSeasonFromChinese(part string) string {
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
func (s *FileService) parseChineseNumber(str string) int {
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

// hasSeasonPattern 检查是否包含季度模式
func (s *FileService) hasSeasonPattern(str string) bool {
	// 检查S01, S02等格式
	for i := 1; i <= 99; i++ {
		seasonNum := strconv.Itoa(i)
		if i < 10 {
			seasonNum = "0" + seasonNum
		}
		if strings.Contains(str, "s"+seasonNum) {
			return true
		}
	}
	return false
}

// extractSeasonNumber 提取季度编号
func (s *FileService) extractSeasonNumber(part string) string {
	lowerPart := strings.ToLower(part)
	
	// 尝试提取S##格式
	for i := 1; i <= 99; i++ {
		num := strconv.Itoa(i)
		patterns := []string{
			"s" + num,
			"s0" + num,
			"season" + num,
			"season " + num,
		}
		
		for _, pattern := range patterns {
			if strings.Contains(lowerPart, pattern) {
				if i < 10 {
					return "S0" + num
				}
				return "S" + num
			}
		}
	}
	
	// 如果没有找到，返回原始部分
	return s.cleanFolderName(part)
}

// extractFolderName 提取文件夹名称
func (s *FileService) extractFolderName(fullPath string) string {
	parts := strings.Split(fullPath, "/")
	if len(parts) > 1 {
		// 返回倒数第二个部分（通常是包含文件的文件夹）
		return s.cleanFolderName(parts[len(parts)-2])
	}
	return "unknown"
}

// cleanFolderName 清理文件夹名称
func (s *FileService) cleanFolderName(name string) string {
	// 移除特殊字符，保留字母数字和基本符号
	name = strings.TrimSpace(name)
	
	// 替换不适合作为文件夹名的字符
	replacer := strings.NewReplacer(
		":", "-",
		"?", "",
		"*", "",
		"<", "",
		">", "",
		"|", "",
		"\\", "",
		"/", "",
		"\"", "",
	)
	
	return replacer.Replace(name)
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

// GetFilesFromPath 从指定路径获取文件
func (s *FileService) GetFilesFromPath(basePath string, recursive bool) ([]FileInfo, error) {
	var allFiles []FileInfo
	
	if recursive {
		// 递归获取所有文件
		if err := s.fetchFilesRecursive(basePath, &allFiles); err != nil {
			return nil, err
		}
	} else {
		// 只获取当前目录的文件
		if err := s.fetchFilesFromDirectory(basePath, &allFiles); err != nil {
			return nil, err
		}
	}
	
	return allFiles, nil
}

// fetchFilesFromDirectory 获取目录中的文件（不递归）
func (s *FileService) fetchFilesFromDirectory(path string, result *[]FileInfo) error {
	page := 1
	perPage := 100
	
	for {
		// 获取文件列表
		fileList, err := s.alistClient.ListFiles(path, page, perPage)
		if err != nil {
			return err
		}
		
		// 处理每个文件
		for _, file := range fileList.Data.Content {
			// 跳过目录
			if file.IsDir {
				continue
			}
			
			// 解析修改时间
			modTime, err := time.Parse(time.RFC3339, file.Modified)
			if err != nil {
				modTime = time.Now()
			}
			
			// 构建完整路径
			fullPath := file.Path
			if fullPath == "" {
				if path == "/" {
					fullPath = "/" + file.Name
				} else {
					fullPath = path + "/" + file.Name
				}
			}
			
			// 获取文件详细信息（包含下载链接）
			fileInfo, err := s.alistClient.GetFileInfo(fullPath)
			if err != nil {
				continue
			}
			
			// 替换URL
			originalURL := fileInfo.Data.RawURL
			internalURL := strings.Replace(originalURL, "fcalist-public", "fcalist-internal", -1)
			
			// 判断媒体类型并生成下载路径
			mediaType, downloadPath := s.determineMediaTypeAndPath(fullPath, file.Name)
			
			*result = append(*result, FileInfo{
				Name:         file.Name,
				Path:         fullPath,
				Size:         file.Size,
				Modified:     modTime,
				OriginalURL:  originalURL,
				InternalURL:  internalURL,
				MediaType:    mediaType,
				DownloadPath: downloadPath,
			})
		}
		
		// 检查是否还有更多页
		if len(fileList.Data.Content) < perPage {
			break
		}
		page++
	}
	
	return nil
}

// fetchFilesRecursive 递归获取所有文件
func (s *FileService) fetchFilesRecursive(path string, result *[]FileInfo) error {
	page := 1
	perPage := 100
	
	for {
		// 获取文件列表
		fileList, err := s.alistClient.ListFiles(path, page, perPage)
		if err != nil {
			return err
		}
		
		// 处理每个文件/目录
		for _, file := range fileList.Data.Content {
			// 解析修改时间
			modTime, err := time.Parse(time.RFC3339, file.Modified)
			if err != nil {
				modTime = time.Now()
			}
			
			// 构建完整路径
			fullPath := file.Path
			if fullPath == "" {
				if path == "/" {
					fullPath = "/" + file.Name
				} else {
					fullPath = path + "/" + file.Name
				}
			}
			
			if file.IsDir {
				// 如果是目录，递归处理
				if err := s.fetchFilesRecursive(fullPath, result); err != nil {
					return err
				}
			} else {
				// 如果是文件，添加到结果
				fileInfo, err := s.alistClient.GetFileInfo(fullPath)
				if err != nil {
					continue
				}
				
				// 替换URL
				originalURL := fileInfo.Data.RawURL
				internalURL := strings.Replace(originalURL, "fcalist-public", "fcalist-internal", -1)
				
				// 判断媒体类型并生成下载路径
				mediaType, downloadPath := s.determineMediaTypeAndPath(fullPath, file.Name)
				
				*result = append(*result, FileInfo{
					Name:         file.Name,
					Path:         fullPath,
					Size:         file.Size,
					Modified:     modTime,
					OriginalURL:  originalURL,
					InternalURL:  internalURL,
					MediaType:    mediaType,
					DownloadPath: downloadPath,
				})
			}
		}
		
		// 检查是否还有更多页
		if len(fileList.Data.Content) < perPage {
			break
		}
		page++
	}
	
	return nil
}