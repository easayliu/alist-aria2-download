package services

import (
	"fmt"
	"path/filepath"
	"regexp"
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

// ListFilesSimple 简单列出文件（用于Telegram等场景）
func (s *FileService) ListFilesSimple(path string, page, perPage int) ([]alist.FileItem, error) {
	fileList, err := s.alistClient.ListFiles(path, page, perPage)
	if err != nil {
		return nil, err
	}
	return fileList.Data.Content, nil
}

// FetchFilesByTimeRange 获取指定时间范围内的文件
func (s *FileService) FetchFilesByTimeRange(path string, startTime, endTime time.Time, videoOnly bool) ([]alist.FileItem, error) {
	var allFiles []alist.FileItem

	// 递归获取所有文件
	if err := s.fetchFilesRecursiveByTime(path, startTime, endTime, videoOnly, &allFiles); err != nil {
		return nil, err
	}

	return allFiles, nil
}

// fetchFilesRecursiveByTime 递归获取时间范围内的文件
func (s *FileService) fetchFilesRecursiveByTime(path string, startTime, endTime time.Time, videoOnly bool, files *[]alist.FileItem) error {
	fileList, err := s.alistClient.ListFiles(path, 1, 1000)
	if err != nil {
		return fmt.Errorf("获取文件列表失败: %w", err)
	}

	for _, file := range fileList.Data.Content {
		fileTime, _ := time.Parse(time.RFC3339, file.Modified)

		if file.IsDir {
			// 递归处理子目录
			subPath := path + "/" + file.Name
			if path == "/" {
				subPath = "/" + file.Name
			}
			s.fetchFilesRecursiveByTime(subPath, startTime, endTime, videoOnly, files)
		} else {
			// 检查文件时间和类型
			if fileTime.After(startTime) && fileTime.Before(endTime) {
				if !videoOnly || (videoOnly && s.isSingleVideoFile(file.Name)) {
					*files = append(*files, file)
				}
			}
		}
	}

	return nil
}

// GetFileDownloadURL 获取文件下载URL
func (s *FileService) GetFileDownloadURL(path, fileName string) string {
	// 构建完整路径
	fullPath := path + "/" + fileName
	if path == "/" {
		fullPath = "/" + fileName
	}

	// 这里需要根据Alist的配置构建下载URL
	// 通常是 base_url + /d + path
	return s.alistClient.BaseURL + "/d" + fullPath
}

// CreateDownloadTask 创建下载任务（需要依赖下载服务）
func (s *FileService) CreateDownloadTask(url, fileName string) (string, error) {
	// 这里暂时返回一个模拟的任务ID
	// 实际应该调用下载服务
	return "task-" + time.Now().Format("20060102150405"), nil
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

	// 处理电影类型的同目录下载逻辑
	s.processYesterdayMovieDirectoryGrouping(&allYesterdayFiles)

	return allYesterdayFiles, nil
}

// GetFilesByTimeRange 获取指定时间范围内修改的文件（用于定时任务）
func (s *FileService) GetFilesByTimeRange(basePath string, startTime, endTime time.Time, videoOnly bool) ([]YesterdayFileInfo, error) {
	var allFiles []YesterdayFileInfo

	// 递归获取文件
	if err := s.fetchFilesRecursiveWithInfo(basePath, startTime, endTime, videoOnly, &allFiles); err != nil {
		return nil, err
	}

	// 处理电影类型的同目录下载逻辑
	s.processYesterdayMovieDirectoryGrouping(&allFiles)

	return allFiles, nil
}

// fetchFilesRecursiveWithInfo 递归获取指定时间范围的文件（通用方法）
func (s *FileService) fetchFilesRecursiveWithInfo(path string, startTime, endTime time.Time, videoOnly bool, result *[]YesterdayFileInfo) error {
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
				if err := s.fetchFilesRecursiveWithInfo(fullPath, startTime, endTime, videoOnly, result); err != nil {
					return err
				}
			} else {
				// 如果需要过滤视频文件
				if videoOnly && !s.isSingleVideoFile(file.Name) {
					continue
				}

				// 检查是否在时间范围内
				if modTime.After(startTime) && modTime.Before(endTime) {
					// 获取文件详细信息（包含下载链接）
					fileInfo, err := s.alistClient.GetFileInfo(fullPath)
					if err != nil {
						continue
					}

					// 替换URL（只在包含fcalist-public时替换）
					originalURL := fileInfo.Data.RawURL
					internalURL := originalURL
					if strings.Contains(originalURL, "fcalist-public") {
						internalURL = strings.ReplaceAll(originalURL, "fcalist-public", "fcalist-internal")
					}

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
				// 如果是文件，先检查是否为视频文件
				if !s.isSingleVideoFile(file.Name) {
					continue
				}

				// 检查是否是昨天修改的
				if modTime.After(yesterdayStart) && modTime.Before(yesterdayEnd) {
					// 获取文件详细信息（包含下载链接）
					fileInfo, err := s.alistClient.GetFileInfo(fullPath)
					if err != nil {
						continue
					}

					// 替换URL（只在包含fcalist-public时替换）
					originalURL := fileInfo.Data.RawURL
					internalURL := originalURL
					if strings.Contains(originalURL, "fcalist-public") {
						internalURL = strings.ReplaceAll(originalURL, "fcalist-public", "fcalist-internal")
					}

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
		// 首先检查是否为电影系列 - 电影系列优先级最高
		if s.isMovieSeries(fullPath) {
			movieName := s.extractMovieName(fullPath)
			if movieName != "" {
				return MediaTypeMovie, "/downloads/movies/" + movieName
			}
		}

		// 然后检查是否为TV剧集
		if s.isTVShow(fullPath) || s.hasStrongTVIndicators(fullPath) || s.hasStrongTVIndicators(lowerPath) {
			// 特殊处理：如果文件名包含S##EP##格式，使用特殊的路径提取逻辑
			if s.hasSeasonEpisodePattern(fileName) {
				showName, versionPath := s.extractTVShowWithVersion(fullPath)
				if showName != "" {
					if versionPath != "" {
						return MediaTypeTV, "/downloads/tvs/" + showName + "/" + versionPath
					}
					return MediaTypeTV, "/downloads/tvs/" + showName + "/S1"
				}
			}
			
			// 提取剧集信息
			showName, seasonInfo := s.extractTVShowInfo(fullPath)
			if showName != "" && seasonInfo != "" {
				return MediaTypeTV, "/downloads/tvs/" + showName + "/" + seasonInfo
			}
			return MediaTypeTV, "/downloads/tvs/" + s.extractFolderName(fullPath) + "/S1"
		}

		// 单个视频文件，默认判定为电影
		movieName := s.extractMovieName(fullPath)
		if movieName != "" {
			return MediaTypeMovie, "/downloads/movies/" + movieName
		}
		return MediaTypeMovie, "/downloads/movies"
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

	// 判断是否为TV剧集
	if s.isTVShow(fullPath) || s.hasStrongTVIndicators(fullPath) || s.hasStrongTVIndicators(lowerPath) {
		// 提取剧集信息
		showName, seasonInfo := s.extractTVShowInfo(fullPath)
		if showName != "" && seasonInfo != "" {
			return MediaTypeTV, "/downloads/tvs/" + showName + "/" + seasonInfo
		}
		return MediaTypeTV, "/downloads/tvs/" + s.extractFolderName(fullPath) + "/S1"
	}

	// 默认其他类型
	return MediaTypeOther, "/downloads"
}

// isMovieSeries 检查是否为电影系列
func (s *FileService) isMovieSeries(path string) bool {
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

// hasExplicitTVFeatures 检查是否有明确的TV剧集特征（不包括"系列"）
func (s *FileService) hasExplicitTVFeatures(path string) bool {
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

// IsVideoFile 检查文件名是否是视频文件（公开方法）
func (s *FileService) IsVideoFile(fileName string) bool {
	return s.isSingleVideoFile(fileName)
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

// isTVShow 判断是否为电视剧
func (s *FileService) isTVShow(path string) bool {
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

// isMovie 判断是否为电影 - 基于单个视频文件判断
func (s *FileService) isMovie(path string) bool {
	// 提取文件名
	fileName := filepath.Base(path)

	// 首先检查是否为视频文件
	if !s.isSingleVideoFile(fileName) {
		return false
	}

	// 如果是视频文件，且不包含强TV特征，则认为是电影
	return !s.hasStrongTVIndicators(path)
}

// extractMovieName 提取电影名称或系列名称
func (s *FileService) extractMovieName(fullPath string) string {
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
	// 对于电影，如果是单个文件，尝试从文件名提取
	fileName := filepath.Base(fullPath)
	if s.isSingleVideoFile(fileName) {
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
func (s *FileService) extractCleanMovieName(name string) string {
	// 去除文件扩展名
	cleanName := name
	if strings.Contains(cleanName, ".") {
		ext := filepath.Ext(cleanName)
		cleanName = strings.TrimSuffix(cleanName, ext)
	}

	// 去除年份 (如 (2014) 或 [2014] 或 .2014.)
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

	// 去除点分隔的年份格式 (如 Avatar.2022.4K)
	parts := strings.Split(cleanName, ".")
	var cleanParts []string
	for _, part := range parts {
		// 如果这个部分是年份，停止收集
		if s.hasYear(part) || len(part) == 4 && s.isYear(part) {
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
			strings.Contains(path, " "+year+" ") ||
			strings.Contains(path, year) {
			return true
		}
	}
	return false
}

// isYear 检查字符串是否为年份
func (s *FileService) isYear(str string) bool {
	if year, err := strconv.Atoi(str); err == nil {
		return year >= 1900 && year <= 2099
	}
	return false
}

// extractTVShowInfo 提取电视剧信息
func (s *FileService) extractTVShowInfo(fullPath string) (showName, seasonInfo string) {
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
		seasonInfo = "S1" // 默认第一季
	}

	return
}

// extractSeasonFromFileName 从文件名提取季度信息（S##E##格式）
func (s *FileService) extractSeasonFromFileName(fileName string) string {
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
func (s *FileService) isSeasonDirectory(dir string) bool {
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

// isKnownTVShow 检查是否是已知的TV节目或综艺节目
func (s *FileService) isKnownTVShow(path string) bool {
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
	
	// 检查日期格式的节目（如 20240628, 20250919）
	// 这种格式通常是综艺节目
	fileName := filepath.Base(path)
	datePattern := regexp.MustCompile(`\b20\d{6}\b`)
	if datePattern.MatchString(fileName) {
		// 如果文件名包含8位日期格式（YYYYMMDD），很可能是综艺节目
		return true
	}
	
	return false
}

// extractShowNameFromPath 从路径部分提取剧集名称
func (s *FileService) extractShowNameFromPath(parts []string, seasonIndex int) string {
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
		if s.isVersionDirectory(part) {
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
		// 优先选择不包含季度信息的剧名
		for _, name := range candidateNames {
			if !strings.Contains(name, "第") || !strings.Contains(name, "季") {
				return name
			}
		}
		// 如果都包含季度信息，返回第一个
		return candidateNames[0]
	}
	
	return ""
}

// extractShowNameFromFullPath 从完整路径提取剧名
func (s *FileService) extractShowNameFromFullPath(fullPath string) string {
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

	// 去除类似"第八季"的季度后缀，保留纯剧名
	seasonSuffixRegex := regexp.MustCompile(`(?i)\s*第[\p{Han}\d]{1,4}季$`)
	if seasonSuffixRegex.MatchString(cleanName) {
		cleanName = seasonSuffixRegex.ReplaceAllString(cleanName, "")
		cleanName = strings.TrimSpace(cleanName)
	}

	// 特殊处理：标准化节目名称
	cleanName = s.standardizeShowName(cleanName)

	// 如果清理后的名称太短，返回原始名称
	if len(cleanName) < 2 {
		return s.cleanFolderName(name)
	}

	return s.cleanFolderName(cleanName)
}

// standardizeShowName 标准化节目名称，处理同一节目的不同命名方式
func (s *FileService) standardizeShowName(name string) string {
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

// parseSeasonNumber 从季度字符串中解析出数字（如 "S08" -> 8, "S1" -> 1）
func (s *FileService) parseSeasonNumber(seasonStr string) int {
	// 移除 S 前缀
	if strings.HasPrefix(seasonStr, "S") {
		numStr := strings.TrimPrefix(seasonStr, "S")
		if num, err := strconv.Atoi(numStr); err == nil {
			return num
		}
	}
	return 1 // 默认第一季
}

// hasSeasonPattern 检查是否包含季度模式
func (s *FileService) hasSeasonPattern(str string) bool {
	// 使用正则表达式匹配更灵活的季度格式
	// 匹配 /s1/, /s01/, s1/, s01/ 等作为目录，但不匹配复杂的格式如 S08.2025.2160p
	// 避免将质量/版本信息误识别为季度
	seasonRegex := regexp.MustCompile(`(?i)(^|[/\s])s(\d{1,2})($|[/\s])`)
	
	matches := seasonRegex.FindStringSubmatch(str)
	if len(matches) > 2 {
		// 提取季度数字
		if seasonNum, err := strconv.Atoi(matches[2]); err == nil {
			// 季度在合理范围内（1-99）
			return seasonNum >= 1 && seasonNum <= 99
		}
	}
	
	return false
}

// extractSeasonNumber 提取季度编号
func (s *FileService) extractSeasonNumber(part string) string {
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

// isEpisodeNumber 检查是否为纯数字的集数格式
func (s *FileService) isEpisodeNumber(name string) bool {
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

// hasEpisodePattern 检查是否包含集数模式（E01, EP01, E74等）
func (s *FileService) hasEpisodePattern(path string) bool {
	// 正则表达式匹配常见的集数格式
	// 匹配 E01-E999, EP01-EP999, e01-e999, ep01-ep999 等格式
	// 也支持 S01E01 格式中的 E 部分
	episodeRegex := regexp.MustCompile(`(?i)(^|[^A-Za-z])(E|EP)(\d{1,3})($|[^0-9])`)
	
	// 检查是否匹配
	matches := episodeRegex.FindStringSubmatch(path)
	if len(matches) > 3 {
		// 提取集数（第3个捕获组是数字）
		if episodeNum, err := strconv.Atoi(matches[3]); err == nil {
			// 集数在合理范围内（1-999）
			return episodeNum >= 1 && episodeNum <= 999
		}
	}
	
	return false
}

// hasSeasonEpisodePattern 检查文件名是否包含S##EP##格式
func (s *FileService) hasSeasonEpisodePattern(fileName string) bool {
	// 匹配 S01EP01, S01EP76 等格式
	matched, _ := regexp.MatchString(`(?i)S\d{1,2}EP\d{1,3}`, fileName)
	return matched
}

// extractTVShowWithVersion 从路径提取剧名和版本/质量路径
func (s *FileService) extractTVShowWithVersion(fullPath string) (showName, versionPath string) {
	parts := strings.Split(fullPath, "/")
	
	// 查找包含版本/质量信息的目录（通常是文件的直接父目录）
	// 例如：4K[DV][60帧][高码率]
	if len(parts) >= 2 {
		// 获取文件的直接父目录
		parentDir := parts[len(parts)-2]
		
		// 检查是否是版本/质量目录（包含[]或特定关键词）
		if s.isVersionDirectory(parentDir) {
			versionPath = parentDir
			
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

// isVersionDirectory 检查是否为版本/质量目录
func (s *FileService) isVersionDirectory(dir string) bool {
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

// extractCleanShowName 提取干净的剧名（去除版本信息）
func (s *FileService) extractCleanShowName(name string) string {
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
	
	return s.cleanFolderName(cleanName)
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

	// 处理电影类型的同目录下载逻辑
	s.processMovieDirectoryGrouping(&allFiles)

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

			// 跳过非视频文件
			if !s.isSingleVideoFile(file.Name) {
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

			// 替换URL（只在包含fcalist-public时替换）
			originalURL := fileInfo.Data.RawURL
			internalURL := originalURL
			if strings.Contains(originalURL, "fcalist-public") {
				internalURL = strings.ReplaceAll(originalURL, "fcalist-public", "fcalist-internal")
			}

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
				// 如果是文件，先检查是否为视频文件
				if !s.isSingleVideoFile(file.Name) {
					continue
				}

				// 添加到结果
				fileInfo, err := s.alistClient.GetFileInfo(fullPath)
				if err != nil {
					continue
				}

				// 替换URL（只在包含fcalist-public时替换）
				originalURL := fileInfo.Data.RawURL
				internalURL := originalURL
				if strings.Contains(originalURL, "fcalist-public") {
					internalURL = strings.ReplaceAll(originalURL, "fcalist-public", "fcalist-internal")
				}

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

// processMovieDirectoryGrouping 处理电影类型的同目录下载逻辑
// 当目录中有电影文件时，将该目录下的所有其他文件也归类为电影并使用相同的下载路径
func (s *FileService) processMovieDirectoryGrouping(files *[]FileInfo) {
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

// processYesterdayMovieDirectoryGrouping 处理昨天文件的电影类型同目录下载逻辑
// 当目录中有电影文件时，将该目录下的所有其他文件也归类为电影并使用相同的下载路径
func (s *FileService) processYesterdayMovieDirectoryGrouping(files *[]YesterdayFileInfo) {
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
