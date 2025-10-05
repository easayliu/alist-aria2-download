package path

import (
	"fmt"
	"strings"

	"github.com/easayliu/alist-aria2-download/internal/application/contracts"
	mediaservices "github.com/easayliu/alist-aria2-download/internal/domain/services/media"
	domainpathservices "github.com/easayliu/alist-aria2-download/internal/domain/services/path"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/config"
	"github.com/easayliu/alist-aria2-download/pkg/logger"
	pathutil "github.com/easayliu/alist-aria2-download/pkg/utils/path"
	strutil "github.com/easayliu/alist-aria2-download/pkg/utils/string"
)

// PathGenerationService 路径生成服务 - 专注于下载路径的生成逻辑
type PathGenerationService struct {
	config           *config.Config
	pathStrategy     *PathStrategyService
	pathCategory     *domainpathservices.PathCategoryService
	mediaClassifier  *mediaservices.MediaClassificationService
}

// NewPathGenerationService 创建路径生成服务
func NewPathGenerationService(
	cfg *config.Config,
	pathStrategy *PathStrategyService,
	pathCategory *domainpathservices.PathCategoryService,
	mediaClassifier *mediaservices.MediaClassificationService,
) *PathGenerationService {
	return &PathGenerationService{
		config:          cfg,
		pathStrategy:    pathStrategy,
		pathCategory:    pathCategory,
		mediaClassifier: mediaClassifier,
	}
}

// GenerateDownloadPath 生成下载路径
func (s *PathGenerationService) GenerateDownloadPath(file contracts.FileResponse) string {
	// 如果启用了路径策略服务，使用新的统一路径生成
	if s.pathStrategy != nil {
		baseDir := s.config.Aria2.DownloadDir
		if baseDir == "" {
			baseDir = "/downloads"
		}

		generatedPath, err := s.pathStrategy.GenerateDownloadPath(file, baseDir)
		if err != nil {
			return s.generateDownloadPathLegacy(file)
		}

		return generatedPath
	}

	// 未启用路径策略服务时，使用旧逻辑
	return s.generateDownloadPathLegacy(file)
}

// generateDownloadPathLegacy 旧的路径生成逻辑（保留作为回退）
func (s *PathGenerationService) generateDownloadPathLegacy(file contracts.FileResponse) string {
	baseDir := s.config.Aria2.DownloadDir
	if baseDir == "" {
		baseDir = "/downloads"
	}

	// 首先检查路径中的类型指示器（优先级最高）
	pathCategory := s.pathCategory.GetCategoryFromPath(file.Path)

	if pathCategory != "" {
		// 对于电视剧，使用智能路径解析和重组
		if pathCategory == "tv" {
			smartPath := s.generateSmartTVPath(file.Path, baseDir)
			if smartPath != "" {
				return smartPath
			}
		}

		// 提取并保留原始路径结构
		targetDir := s.extractPathStructure(file.Path, pathCategory, baseDir)
		if targetDir != "" {
			return targetDir
		}
	}

	// 如果路径分类失败，直接使用默认目录
	defaultDir := pathutil.JoinPath(baseDir, "others")
	return defaultDir
}

// extractPathStructure 从原始路径中提取并保留目录结构
func (s *PathGenerationService) extractPathStructure(filePath, pathCategory, baseDir string) string {
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
		varietyKeywords := []string{"variety", "show", "综艺", "娱乐"}
		for _, keyword := range varietyKeywords {
			if strings.Contains(pathLower, keyword) {
				keywordFound = keyword
				break
			}
		}
	case "video":
		targetCategoryDir = "videos"
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

	// 在原始路径中找到关键词的位置
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


	// 获取文件的父目录（去掉文件名）
	parentDir := pathutil.GetParentPath(afterKeyword)

	// 过滤掉路径中的其他分类关键词
	if parentDir != "" && parentDir != "/" {
		parentDir = s.filterCategoryKeywords(parentDir, allCategoryKeywords)
	}

	// 构建最终路径
	if parentDir == "" || parentDir == "/" {
		targetDir := pathutil.JoinPath(baseDir, targetCategoryDir)
		return targetDir
	}

	// 清理节目名
	pathParts := strings.Split(strings.Trim(parentDir, "/"), "/")
	if len(pathParts) > 0 {
		cleanedShowName := strutil.CleanShowName(pathParts[0])
		pathParts[0] = cleanedShowName
		parentDir = strings.Join(pathParts, "/")
	}

	targetDir := pathutil.JoinPath(baseDir, targetCategoryDir, parentDir)
	return targetDir
}

// filterCategoryKeywords 过滤路径中的分类关键词目录
func (s *PathGenerationService) filterCategoryKeywords(path string, keywords []string) string {
	if path == "" || path == "/" {
		return path
	}


	parts := strings.Split(strings.Trim(path, "/"), "/")
	var filteredParts []string

	for _, part := range parts {
		if part == "" {
			continue
		}

		partLower := strings.ToLower(part)
		isKeyword := false

		for _, keyword := range keywords {
			if partLower == keyword {
				isKeyword = true
				break
			}
		}

		if !isKeyword {
			filteredParts = append(filteredParts, part)
		}
	}

	if len(filteredParts) == 0 {
		return ""
	}

	result := strings.Join(filteredParts, "/")
	return result
}

// generateSmartTVPath 智能生成电视剧路径
func (s *PathGenerationService) generateSmartTVPath(filePath, baseDir string) string {

	pathLower := strings.ToLower(filePath)
	tvsIndex := strings.Index(pathLower, "tvs")
	if tvsIndex == -1 {
		logger.Warn("路径中未找到tvs关键词", "filePath", filePath)
		return ""
	}

	// 提取tvs之后的路径部分
	afterTvs := filePath[tvsIndex+3:]
	if strings.HasPrefix(afterTvs, "/") {
		afterTvs = afterTvs[1:]
	}

	pathParts := strings.Split(afterTvs, "/")
	if len(pathParts) < 2 {
		logger.Warn("TV路径结构不完整", "afterTvs", afterTvs, "parts", pathParts)
		return ""
	}


	// 寻找包含季度信息的目录
	var smartPath string
	lastIndex := len(pathParts) - 1

	if strings.Contains(pathParts[lastIndex], ".") {
		lastIndex--
	}

	for i := lastIndex; i >= 0; i-- {
		currentDir := pathParts[i]

		// 检查完整节目名
		extractedShowName := s.extractFullShowName(currentDir)
		if extractedShowName != "" {
			if strings.Contains(extractedShowName, "宝藏行") || strings.Contains(extractedShowName, "公益季") {
				smartPath = pathutil.JoinPath(baseDir, "tvs", extractedShowName)
				return smartPath
			}
		}

		// 提取季度信息
		seasonNumber := strutil.ExtractSeasonNumber(currentDir)
		if seasonNumber > 0 {
			baseShowName := strutil.CleanShowName(pathParts[0])
			seasonCode := fmt.Sprintf("S%02d", seasonNumber)
			smartPath = pathutil.JoinPath(baseDir, "tvs", baseShowName, seasonCode)

			logger.Info("从目录生成季度路径",
				"原路径", filePath,
				"基础节目名", baseShowName,
				"季度目录", currentDir,
				"季度", seasonNumber,
				"季度代码", seasonCode,
				"智能路径", smartPath)

			return smartPath
		}

		if extractedShowName != "" {
			smartPath = pathutil.JoinPath(baseDir, "tvs", extractedShowName)
			logger.Info("使用完整节目名生成路径",
				"原路径", filePath,
				"目标目录", currentDir,
				"提取节目名", extractedShowName,
				"智能路径", smartPath)
			return smartPath
		}
	}

	// 回退到传统解析
	showName := strutil.CleanShowName(pathParts[0])
	seasonDir := pathParts[1]

	logger.Info("回退到传统解析", "showName", showName, "seasonDir", seasonDir)

	seasonNumber := strutil.ExtractSeasonNumber(seasonDir)
	if seasonNumber > 0 {
		seasonCode := fmt.Sprintf("S%02d", seasonNumber)
		smartPath = pathutil.JoinPath(baseDir, "tvs", showName, seasonCode)

		logger.Info("传统方法生成路径",
			"原路径", filePath,
			"节目名", showName,
			"季度", seasonNumber,
			"季度代码", seasonCode,
			"智能路径", smartPath)

		return smartPath
	}

	logger.Info("未能解析季度信息，使用原始逻辑", "seasonDir", seasonDir)
	return ""
}

// extractFullShowName 提取完整的节目名
func (s *PathGenerationService) extractFullShowName(dirName string) string {
	if dirName == "" {
		return ""
	}

	seasonKeywords := []string{"第", "季", "season", "宝藏行", "公益季"}
	dirLower := strings.ToLower(dirName)

	for _, keyword := range seasonKeywords {
		if strings.Contains(dirLower, strings.ToLower(keyword)) {
			cleanName := strutil.CleanShowName(dirName)
			if cleanName != "" {
				return cleanName
			}
		}
	}

	return ""
}
