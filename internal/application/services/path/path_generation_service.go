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
		logger.Warn("No matching keyword found", "filePath", filePath, "pathCategory", pathCategory)
		return ""
	}

	// 在原始路径中找到关键词的位置
	keywordIndex := strings.Index(pathLower, keywordFound)
	if keywordIndex == -1 {
		logger.Warn("Unable to find keyword position in path", "filePath", filePath, "keywordFound", keywordFound)
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

	// 如果父目录是 "." 说明afterKeyword本身就是一个目录名，需要清理它
	if parentDir == "." || parentDir == "" {
		// 对于电视剧类型，特殊处理季度信息
		if pathCategory == "tv" {
			// 提取季度信息
			seasonNumber := strutil.ExtractSeasonNumber(afterKeyword)
			if seasonNumber > 0 {
				// 清理节目名
				cleanedShowName := strutil.CleanShowName(afterKeyword)
				seasonCode := strutil.FormatSeason(seasonNumber)
				targetDir := pathutil.JoinPath(baseDir, targetCategoryDir, cleanedShowName, seasonCode)
				logger.Info("Extracted season info from directory name",
					"original", afterKeyword,
					"showName", cleanedShowName,
					"season", seasonCode,
					"targetPath", targetDir)
				return targetDir
			}
		}

		// 非电视剧或无季度信息，常规清理
		cleanedDirName := strutil.CleanShowName(afterKeyword)
		if cleanedDirName != "" && cleanedDirName != afterKeyword {
			// 清理成功，使用清理后的名称
			targetDir := pathutil.JoinPath(baseDir, targetCategoryDir, cleanedDirName)
			logger.Info("Cleaned directory name",
				"original", afterKeyword,
				"cleaned", cleanedDirName,
				"targetPath", targetDir)
			return targetDir
		}
		// 如果清理失败，使用默认目录
		targetDir := pathutil.JoinPath(baseDir, targetCategoryDir)
		return targetDir
	}

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
		// 清理每一级目录名
		for i, part := range pathParts {
			cleanedShowName := strutil.CleanShowName(part)
			if cleanedShowName != "" {
				pathParts[i] = cleanedShowName
			}
		}
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
		logger.Warn("TVS keyword not found in path", "filePath", filePath)
		return ""
	}

	// 提取tvs之后的路径部分
	afterTvs := filePath[tvsIndex+3:]
	if strings.HasPrefix(afterTvs, "/") {
		afterTvs = afterTvs[1:]
	}

	pathParts := strings.Split(afterTvs, "/")
	if len(pathParts) < 1 {
		logger.Warn("Incomplete TV path structure", "afterTvs", afterTvs, "parts", pathParts)
		return ""
	}

	// 第一个目录总是节目名
	baseShowName := strutil.CleanShowName(pathParts[0])

	// 如果只有一个目录级别，直接返回
	if len(pathParts) == 1 {
		return pathutil.JoinPath(baseDir, "tvs", baseShowName)
	}

	// 从第二个目录开始寻找季度信息
	var smartPath string
	lastIndex := len(pathParts) - 1

	// 如果最后一个部分是文件（包含扩展名），跳过它
	if strings.Contains(pathParts[lastIndex], ".") {
		lastIndex--
	}

	// 从后往前遍历，寻找季度信息
	for i := 1; i <= lastIndex; i++ {
		currentDir := pathParts[i]

		// 提取季度信息
		seasonNumber := strutil.ExtractSeasonNumber(currentDir)
		if seasonNumber > 0 {
			seasonCode := fmt.Sprintf("S%02d", seasonNumber)
			smartPath = pathutil.JoinPath(baseDir, "tvs", baseShowName, seasonCode)

			logger.Info("Generated season path from directory",
				"originalPath", filePath,
				"baseShowName", baseShowName,
				"seasonDir", currentDir,
				"season", seasonNumber,
				"seasonCode", seasonCode,
				"smartPath", smartPath)

			return smartPath
		}
	}

	// 如果没有找到季度信息，检查是否是特殊节目（如宝藏行、公益季等）
	for i := 1; i <= lastIndex; i++ {
		currentDir := pathParts[i]

		// 检查是否包含特殊关键词
		if strings.Contains(currentDir, "宝藏行") || strings.Contains(currentDir, "公益季") {
			extractedShowName := strutil.CleanShowName(currentDir)
			if extractedShowName != "" {
				smartPath = pathutil.JoinPath(baseDir, "tvs", baseShowName, extractedShowName)
				logger.Info("Generated path using special show name",
					"originalPath", filePath,
					"baseShowName", baseShowName,
					"specialShow", extractedShowName,
					"smartPath", smartPath)
				return smartPath
			}
		}
	}

	// 没有找到季度信息，返回基础节目名路径
	smartPath = pathutil.JoinPath(baseDir, "tvs", baseShowName)
	logger.Info("Season info not found, using base show name",
		"originalPath", filePath,
		"showName", baseShowName,
		"smartPath", smartPath)

	return smartPath
}

