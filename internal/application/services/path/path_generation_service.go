package path

import (
	"strings"

	"github.com/easayliu/alist-aria2-download/internal/application/contracts"
	mediaservices "github.com/easayliu/alist-aria2-download/internal/domain/services/media"
	domainpathservices "github.com/easayliu/alist-aria2-download/internal/domain/services/path"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/config"
	"github.com/easayliu/alist-aria2-download/pkg/logger"
	pathutil "github.com/easayliu/alist-aria2-download/pkg/utils/path"
)

// PathGenerationService 路径生成服务 - 专注于下载路径的生成逻辑
type PathGenerationService struct {
	config          *config.Config
	pathStrategy    *PathStrategyService
	pathCategory    *domainpathservices.PathCategoryService
	mediaClassifier *mediaservices.MediaClassificationService
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

	pathCategory := s.pathCategory.GetCategoryFromPath(file.Path)
	if pathCategory != "" {
		targetDir := s.extractPathStructure(file.Path, pathCategory, baseDir)
		if targetDir != "" {
			return targetDir
		}
	}

	return pathutil.JoinPath(baseDir, "others")
}

// extractPathStructure 从原始路径中提取并保留目录结构
func (s *PathGenerationService) extractPathStructure(filePath, pathCategory, baseDir string) string {
	pathLower := strings.ToLower(filePath)

	var keywordFound string
	switch pathCategory {
	case "tv":
		keywordFound = "/tvs/"
	case "movie":
		keywordFound = "/movies/"
	case "variety":
		keywordFound = "/variety/"
	case "video":
		keywordFound = "/videos/"
	default:
		return ""
	}

	keywordIndex := strings.Index(pathLower, keywordFound)
	if keywordIndex == -1 {
		logger.Warn("Unable to find keyword in path", "filePath", filePath, "keyword", keywordFound)
		return ""
	}

	pathFromKeyword := filePath[keywordIndex:]

	parentDir := pathutil.GetParentPath(pathFromKeyword)
	if parentDir == "." || parentDir == "" {
		return pathutil.JoinPath(baseDir, strings.Trim(keywordFound, "/"))
	}

	parentDir = strings.TrimPrefix(parentDir, "/")
	targetDir := pathutil.JoinPath(baseDir, parentDir)
	return targetDir
}
