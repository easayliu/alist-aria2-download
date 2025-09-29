package services

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/easayliu/alist-aria2-download/internal/application/contracts"
	"github.com/easayliu/alist-aria2-download/pkg/logger"
	"github.com/easayliu/alist-aria2-download/pkg/utils"
)

// IsVideoFile 检查是否为视频文件
func (s *AppFileService) IsVideoFile(filename string) bool {
	if filename == "" {
		return false
	}

	ext := strings.ToLower(filename)
	if idx := strings.LastIndex(ext, "."); idx != -1 {
		ext = ext[idx+1:]
	}

	for _, videoExt := range s.config.Download.VideoExts {
		if ext == strings.ToLower(videoExt) {
			return true
		}
	}

	return false
}

// GetFileCategory 获取文件分类
func (s *AppFileService) GetFileCategory(filename string) string {
	if !s.IsVideoFile(filename) {
		return "other"
	}

	filename = strings.ToLower(filename)

	// 电影关键词
	movieKeywords := []string{"movie", "film", "电影", "蓝光", "bluray", "bd", "4k", "1080p", "720p"}
	for _, keyword := range movieKeywords {
		if strings.Contains(filename, keyword) {
			return "movie"
		}
	}

	// 电视剧关键词
	tvKeywords := []string{"tv", "series", "episode", "ep", "s01", "s02", "s03", "season", "电视剧", "连续剧"}
	for _, keyword := range tvKeywords {
		if strings.Contains(filename, keyword) {
			return "tv"
		}
	}

	// 综艺关键词
	varietyKeywords := []string{"variety", "show", "综艺", "娱乐"}
	for _, keyword := range varietyKeywords {
		if strings.Contains(filename, keyword) {
			return "variety"
		}
	}

	return "video"
}

// GetMediaType 获取媒体类型（用于统计）
func (s *AppFileService) GetMediaType(filePath string) string {
	// 首先检查路径中的类型指示器（优先级）
	pathCategory := s.GetCategoryFromPath(filePath)
	if pathCategory != "" {
		switch pathCategory {
		case "movie":
			return "movie"
		case "tv":
			return "tv"
		case "variety":
			return "tv" // 综艺节目也算作TV类型
		default:
			return "other"
		}
	}

	// 回退到基于文件名的分类
	filename := utils.GetFileName(filePath)
	category := s.GetFileCategory(filename)
	switch category {
	case "movie":
		return "movie"
	case "tv":
		return "tv"
	case "variety":
		return "tv" // 综艺节目也算作TV类型
	default:
		return "other"
	}
}

// FormatFileSize 格式化文件大小
func (s *AppFileService) FormatFileSize(size int64) string {
	return utils.FormatFileSize(size)
}

// GenerateDownloadPath 生成下载路径
func (s *AppFileService) GenerateDownloadPath(file contracts.FileResponse) string {
	baseDir := s.config.Aria2.DownloadDir
	if baseDir == "" {
		baseDir = "/downloads"
	}

	// 首先检查路径中的类型指示器（优先级最高）
	pathCategory := s.GetCategoryFromPath(file.Path)
	logger.Info("🏷️  路径分类分析", "path", file.Path, "pathCategory", pathCategory)
	
	if pathCategory != "" {
		// 对于电视剧，使用智能路径解析和重组
		if pathCategory == "tv" {
			smartPath := s.generateSmartTVPath(file.Path, baseDir)
			if smartPath != "" {
				logger.Info("🎯 使用智能电视剧路径", "file", file.Name, "path", file.Path, "smartPath", smartPath)
				return smartPath
			}
		}
		
		// 提取并保留原始路径结构
		targetDir := s.extractPathStructure(file.Path, pathCategory, baseDir)
		if targetDir != "" {
			logger.Info("✅ 使用路径分类结果（保留目录结构）", "file", file.Name, "path", file.Path, "pathCategory", pathCategory, "targetDir", targetDir)
			return targetDir
		}
	}

	// 如果路径分类失败，直接使用默认目录
	defaultDir := utils.JoinPath(baseDir, "others")
	logger.Info("⚠️  路径分类失败，使用默认目录", "file", file.Name, "path", file.Path, "defaultDir", defaultDir)
	return defaultDir
}

// GetCategoryFromPath 从路径中分析文件类型（优先级高于文件名分析）
func (s *AppFileService) GetCategoryFromPath(path string) string {
	if path == "" {
		return ""
	}

	// 将路径转为小写以便匹配
	pathLower := strings.ToLower(path)
	
	// 检查 TVs 和 Movies 的位置，选择最早出现的
	tvsIndex := strings.Index(pathLower, "tvs")
	moviesIndex := strings.Index(pathLower, "movies")
	
	// 如果两个都存在，选择最早出现的（路径层级更高的）
	if tvsIndex != -1 && moviesIndex != -1 {
		if tvsIndex < moviesIndex {
			logger.Info("🔍 路径同时包含 tvs 和 movies，选择更早出现的 tvs", "path", path, "tvsIndex", tvsIndex, "moviesIndex", moviesIndex)
			return "tv"
		} else {
			logger.Info("🔍 路径同时包含 tvs 和 movies，选择更早出现的 movies", "path", path, "tvsIndex", tvsIndex, "moviesIndex", moviesIndex)
			return "movie"
		}
	}
	
	// 简化的 TVs 判断：只要路径包含 tvs 就判断为 tv
	if tvsIndex != -1 {
		return "tv"
	}

	// 简化的 Movies 判断：只要路径包含 movies 就判断为 movie  
	if moviesIndex != -1 {
		return "movie"
	}

	// 综艺类型指示器
	varietyPathKeywords := []string{"/variety/", "/show/", "/综艺/", "/娱乐/"}
	for _, keyword := range varietyPathKeywords {
		if strings.Contains(pathLower, keyword) {
			return "variety"
		}
	}

	// 一般视频类型指示器
	videoPathKeywords := []string{"/videos/", "/video/", "/视频/"}
	for _, keyword := range videoPathKeywords {
		if strings.Contains(pathLower, keyword) {
			return "video"
		}
	}

	// 如果路径中没有明确的类型指示器，返回空字符串
	return ""
}

// updateMediaStats 更新媒体统计
func (s *AppFileService) updateMediaStats(summary *contracts.FileSummary, filePath, filename string) {
	if !s.IsVideoFile(filename) {
		summary.OtherFiles++
		return
	}

	summary.VideoFiles++
	
	// 使用 GetMediaType 方法，它会优先使用路径分类，然后回退到文件名分类
	mediaType := s.GetMediaType(filePath)
	logger.Info("📊 文件统计分类", "filePath", filePath, "filename", filename, "mediaType", mediaType)
	
	switch mediaType {
	case "movie":
		summary.MovieFiles++
	case "tv":
		summary.TVFiles++
	default:
		summary.OtherFiles++
	}
}

// extractPathStructure 从原始路径中提取并保留目录结构（过滤其他分类关键词）
func (s *AppFileService) extractPathStructure(filePath, pathCategory, baseDir string) string {
	// 将路径转为小写用于匹配
	pathLower := strings.ToLower(filePath)
	
	// 定义所有分类关键词
	allCategoryKeywords := []string{"tvs", "movies", "variety", "show", "综艺", "娱乐", "videos", "video", "视频"}
	
	// 根据分类找到对应的关键词和目标目录
	var keywordFound string
	var targetCategoryDir string
	
	switch pathCategory {
	case "tv":
		targetCategoryDir = "tvs"
		keywordFound = "tvs"
	case "movie":
		targetCategoryDir = "movies"
		keywordFound = "movies"
	case "variety":
		targetCategoryDir = "variety"
		// 对于 variety，选择第一个匹配的关键词
		varietyKeywords := []string{"variety", "show", "综艺", "娱乐"}
		for _, keyword := range varietyKeywords {
			if strings.Contains(pathLower, keyword) {
				keywordFound = keyword
				break
			}
		}
	case "video":
		targetCategoryDir = "videos"
		// 对于 video，选择第一个匹配的关键词
		videoKeywords := []string{"videos", "video", "视频"}
		for _, keyword := range videoKeywords {
			if strings.Contains(pathLower, keyword) {
				keywordFound = keyword
				break
			}
		}
	}
	
	if keywordFound == "" {
		logger.Warn("未找到匹配的关键词", "filePath", filePath, "pathCategory", pathCategory)
		return ""
	}
	
	// 在原始路径中找到关键词的位置（保持原始大小写）
	keywordIndex := strings.Index(pathLower, keywordFound)
	if keywordIndex == -1 {
		logger.Warn("无法在原始路径中找到关键词位置", "filePath", filePath, "keywordFound", keywordFound)
		return ""
	}
	
	// 提取关键词之后的路径部分
	afterKeywordStart := keywordIndex + len(keywordFound)
	if afterKeywordStart < len(filePath) && filePath[afterKeywordStart] == '/' {
		afterKeywordStart++ // 跳过关键词后的 /
	}
	
	afterKeyword := ""
	if afterKeywordStart < len(filePath) {
		afterKeyword = filePath[afterKeywordStart:]
	}
	
	logger.Info("🔍 提取路径片段", "keywordFound", keywordFound, "afterKeyword", afterKeyword)
	
	// 获取文件的父目录（去掉文件名）
	parentDir := utils.GetParentPath(afterKeyword)
	
	// 关键步骤：过滤掉路径中的其他分类关键词
	if parentDir != "" && parentDir != "/" {
		parentDir = s.filterCategoryKeywords(parentDir, allCategoryKeywords)
		logger.Info("🧹 过滤分类关键词后", "originalParentDir", utils.GetParentPath(afterKeyword), "filteredParentDir", parentDir)
	}
	
	// 构建最终路径：baseDir + 分类目录 + 过滤后的目录结构
	if parentDir == "" || parentDir == "/" {
		// 如果没有子目录，直接使用分类目录
		targetDir := utils.JoinPath(baseDir, targetCategoryDir)
		logger.Info("📁 无子目录，使用分类根目录", "targetDir", targetDir)
		return targetDir
	} else {
		// 保留过滤后的子目录结构
		targetDir := utils.JoinPath(baseDir, targetCategoryDir, parentDir)
		logger.Info("✅ 最终下载路径", "targetDir", targetDir)
		return targetDir
	}
}

// filterCategoryKeywords 过滤路径中的分类关键词目录
func (s *AppFileService) filterCategoryKeywords(path string, keywords []string) string {
	if path == "" || path == "/" {
		return path
	}
	
	logger.Info("🧹 开始过滤分类关键词", "originalPath", path, "keywords", keywords)
	
	// 分割路径为目录片段
	parts := strings.Split(strings.Trim(path, "/"), "/")
	var filteredParts []string
	
	for _, part := range parts {
		if part == "" {
			continue
		}
		
		partLower := strings.ToLower(part)
		isKeyword := false
		
		// 检查是否是完全匹配的分类关键词
		for _, keyword := range keywords {
			if partLower == keyword {
				logger.Info("🚫 过滤掉分类关键词目录（完全匹配）", "part", part, "keyword", keyword)
				isKeyword = true
				break
			}
		}
		
		// 如果不是关键词，保留这个目录
		if !isKeyword {
			logger.Info("✅ 保留目录", "part", part)
			filteredParts = append(filteredParts, part)
		}
	}
	
	// 重新组装路径
	if len(filteredParts) == 0 {
		logger.Info("⚠️  所有目录都被过滤，返回空路径")
		return ""
	}
	
	result := strings.Join(filteredParts, "/")
	logger.Info("🔧 路径过滤结果", "original", path, "filtered", result, "removedParts", len(parts)-len(filteredParts))
	return result
}

// generateSmartTVPath 智能生成电视剧路径，将季度信息规范化
func (s *AppFileService) generateSmartTVPath(filePath, baseDir string) string {
	logger.Info("🎬 开始智能电视剧路径解析", "filePath", filePath)
	
	// 从路径中提取tvs之后的部分
	pathLower := strings.ToLower(filePath)
	tvsIndex := strings.Index(pathLower, "tvs")
	if tvsIndex == -1 {
		logger.Warn("⚠️  路径中未找到tvs关键词", "filePath", filePath)
		return ""
	}
	
	// 提取tvs之后的路径部分
	afterTvs := filePath[tvsIndex+3:] // 跳过"tvs"
	if strings.HasPrefix(afterTvs, "/") {
		afterTvs = afterTvs[1:] // 去掉开头的/
	}
	
	// 分割路径为各个部分
	pathParts := strings.Split(afterTvs, "/")
	if len(pathParts) < 2 {
		logger.Warn("⚠️  电视剧路径结构不完整", "afterTvs", afterTvs, "parts", pathParts)
		return ""
	}
	
	logger.Info("🔍 路径组件分析", "pathParts", pathParts)
	
	// 寻找包含季度信息的目录（从最深层开始检查）
	var smartPath string
	lastIndex := len(pathParts) - 1
	
	// 如果最后一个部分是文件（包含文件扩展名），则排除它
	if strings.Contains(pathParts[lastIndex], ".") {
		lastIndex-- 
	}
	
	for i := lastIndex; i >= 0; i-- {
		currentDir := pathParts[i]
		logger.Info("🔍 检查目录", "index", i, "dir", currentDir)
		
		// 先检查是否包含完整的节目名信息
		extractedShowName := s.extractFullShowName(currentDir)
		if extractedShowName != "" {
			// 检查是否是"宝藏行"或其他特殊系列（包含更多信息）
			if strings.Contains(extractedShowName, "宝藏行") || strings.Contains(extractedShowName, "公益季") {
				// 对于特殊系列，直接使用完整节目名
				smartPath = utils.JoinPath(baseDir, "tvs", extractedShowName)
				logger.Info("✅ 使用完整特殊节目名", 
					"原路径", filePath,
					"完整节目名", extractedShowName,
					"智能路径", smartPath)
				return smartPath
			}
		}
		
		// 尝试从当前目录提取季度信息并生成规范化路径
		seasonNumber := s.extractSeasonNumber(currentDir)
		if seasonNumber > 0 {
			// 使用第一层目录作为基础节目名，生成 节目名/S##
			baseShowName := pathParts[0]
			seasonCode := fmt.Sprintf("S%02d", seasonNumber)
			smartPath = utils.JoinPath(baseDir, "tvs", baseShowName, seasonCode)
			
			logger.Info("✅ 从目录生成季度路径", 
				"原路径", filePath,
				"基础节目名", baseShowName,
				"季度目录", currentDir,
				"季度", seasonNumber,
				"季度代码", seasonCode,
				"智能路径", smartPath)
			
			return smartPath
		}
		
		// 最后检查其他完整节目名
		if extractedShowName != "" {
			// 直接使用提取的完整节目名作为最终目录
			smartPath = utils.JoinPath(baseDir, "tvs", extractedShowName)
			
			logger.Info("✅ 使用完整节目名生成路径", 
				"原路径", filePath,
				"目标目录", currentDir,
				"提取节目名", extractedShowName,
				"智能路径", smartPath)
			
			return smartPath
		}
	}
	
	// 如果上述方法失败，尝试传统的季度解析方法
	showName := pathParts[0]
	seasonDir := pathParts[1]
	
	logger.Info("🔄 回退到传统解析", "showName", showName, "seasonDir", seasonDir)
	
	// 解析季度信息
	seasonNumber := s.extractSeasonNumber(seasonDir)
	if seasonNumber > 0 {
		// 构建规范化路径：/downloads/tvs/节目名/S##
		seasonCode := fmt.Sprintf("S%02d", seasonNumber)
		smartPath = utils.JoinPath(baseDir, "tvs", showName, seasonCode)
		
		logger.Info("✅ 传统方法生成路径", 
			"原路径", filePath,
			"节目名", showName, 
			"季度", seasonNumber,
			"季度代码", seasonCode,
			"智能路径", smartPath)
		
		return smartPath
	}
	
	logger.Info("⚠️  未能解析季度信息，使用原始逻辑", "seasonDir", seasonDir)
	return ""
}

// extractSeasonNumber 从目录名中提取季度编号
func (s *AppFileService) extractSeasonNumber(dirName string) int {
	if dirName == "" {
		return 0
	}
	
	dirLower := strings.ToLower(dirName)
	
	// 匹配各种季度格式
	patterns := []struct {
		pattern string
		extract func(string) int
	}{
		// 第X季 格式
		{"第", func(s string) int {
			if idx := strings.Index(s, "第"); idx != -1 {
				after := s[idx+len("第"):]
				if seasonIdx := strings.Index(after, "季"); seasonIdx != -1 {
					seasonStr := after[:seasonIdx]
					// 转换中文数字或阿拉伯数字
					return chineseOrArabicToNumber(seasonStr)
				}
			}
			return 0
		}},
		// Season X 格式
		{"season", func(s string) int {
			if idx := strings.Index(s, "season"); idx != -1 {
				after := strings.TrimSpace(s[idx+6:])
				// 提取数字部分
				var numStr string
				for _, char := range after {
					if char >= '0' && char <= '9' {
						numStr += string(char)
					} else {
						break
					}
				}
				if num, err := strconv.Atoi(numStr); err == nil && num > 0 {
					return num
				}
			}
			return 0
		}},
		// SXX 格式
		{"s", func(s string) int {
			if len(s) >= 2 && s[0] == 's' {
				numStr := ""
				for i := 1; i < len(s) && i < 4; i++ { // 最多取3位数字
					if s[i] >= '0' && s[i] <= '9' {
						numStr += string(s[i])
					} else {
						break
					}
				}
				if num, err := strconv.Atoi(numStr); err == nil && num > 0 {
					return num
				}
			}
			return 0
		}},
		// 直接包含年份+季度信息，如"极限挑战第9季2023"
		{"", func(s string) int {
			// 查找"第X季"模式
			for i := 0; i < len(s)-1; i++ {
				if s[i:i+1] == "第" && i+2 < len(s) && s[i+2:i+3] == "季" {
					seasonChar := s[i+1 : i+2]
					return chineseOrArabicToNumber(seasonChar)
				}
			}
			return 0
		}},
	}
	
	// 尝试各种模式
	for _, pattern := range patterns {
		if pattern.pattern == "" || strings.Contains(dirLower, pattern.pattern) {
			if num := pattern.extract(dirLower); num > 0 {
				logger.Info("🎯 成功提取季度编号", "dirName", dirName, "pattern", pattern.pattern, "seasonNumber", num)
				return num
			}
		}
	}
	
	logger.Info("⚠️  无法从目录名提取季度编号", "dirName", dirName)
	return 0
}

// extractFullShowName 提取完整的节目名（包含季度信息）
func (s *AppFileService) extractFullShowName(dirName string) string {
	if dirName == "" {
		return ""
	}
	
	logger.Info("🔍 分析节目名", "dirName", dirName)
	
	// 检查是否包含季度关键词，如果包含则认为这是完整的节目名
	seasonKeywords := []string{"第", "季", "season", "宝藏行", "公益季"}
	hasSeasonInfo := false
	
	dirLower := strings.ToLower(dirName)
	for _, keyword := range seasonKeywords {
		if strings.Contains(dirLower, strings.ToLower(keyword)) {
			hasSeasonInfo = true
			logger.Info("🎯 发现季度关键词", "dirName", dirName, "keyword", keyword)
			break
		}
	}
	
	if hasSeasonInfo {
		// 清理目录名，移除不必要的后缀信息
		cleanName := s.cleanShowName(dirName)
		if cleanName != "" {
			logger.Info("✅ 提取完整节目名", "原目录名", dirName, "清理后", cleanName)
			return cleanName
		}
	}
	
	logger.Info("⚠️  目录不包含季度信息", "dirName", dirName)
	return ""
}

// cleanShowName 清理节目名，移除不必要的后缀信息
func (s *AppFileService) cleanShowName(showName string) string {
	if showName == "" {
		return ""
	}
	
	// 移除常见的后缀信息
	suffixesToRemove := []string{
		"（", "(", // 移除括号及之后的内容
		"2021", "2022", "2023", "2024", "2025", // 移除年份
		"全", "期全", "完结", "[", "【", // 移除完结标记
	}
	
	cleaned := showName
	for _, suffix := range suffixesToRemove {
		if idx := strings.Index(cleaned, suffix); idx != -1 {
			cleaned = cleaned[:idx]
			logger.Info("🧹 移除后缀", "原名", showName, "后缀", suffix, "清理后", cleaned)
		}
	}
	
	// 去除前后空白
	cleaned = strings.TrimSpace(cleaned)
	
	// 如果清理后为空或太短，返回原名
	if len(cleaned) < 3 {
		logger.Info("⚠️  清理后名称太短，使用原名", "cleaned", cleaned, "original", showName)
		return showName
	}
	
	logger.Info("✅ 节目名清理完成", "原名", showName, "清理后", cleaned)
	return cleaned
}

// chineseOrArabicToNumber 转换中文数字或阿拉伯数字为整数
func chineseOrArabicToNumber(str string) int {
	if str == "" {
		return 0
	}
	
	// 先尝试直接转换阿拉伯数字
	if num, err := strconv.Atoi(str); err == nil {
		return num
	}
	
	// 转换中文数字
	chineseNumbers := map[string]int{
		"一": 1, "二": 2, "三": 3, "四": 4, "五": 5,
		"六": 6, "七": 7, "八": 8, "九": 9, "十": 10,
		"1": 1, "2": 2, "3": 3, "4": 4, "5": 5,
		"6": 6, "7": 7, "8": 8, "9": 9,
	}
	
	if num, exists := chineseNumbers[str]; exists {
		return num
	}
	
	return 0
}