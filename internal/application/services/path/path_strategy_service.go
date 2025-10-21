package path

import (
	"fmt"
	"path/filepath"

	"github.com/easayliu/alist-aria2-download/internal/application/contracts"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/config"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/filesystem"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/platform"
	"github.com/easayliu/alist-aria2-download/internal/shared/utils"
	"github.com/easayliu/alist-aria2-download/pkg/logger"
	strutil "github.com/easayliu/alist-aria2-download/pkg/utils/string"
)

// PathStrategyService 路径策略服务 - 统一路径生成入口
type PathStrategyService struct {
	config           *config.Config
	fileService      contracts.FileService
	pathValidator    *filesystem.PathValidatorService
	directoryMgr     *filesystem.DirectoryManager
	varExtractor     *utils.VariableExtractor     // 变量提取器
	templateRenderer *utils.TemplateRenderer      // 模板渲染器
	conflictDetector *filesystem.ConflictDetector      // 冲突检测器
	mappingEngine    *PathMappingEngine     // 映射规则引擎（可选）
	pathAdapter      *platform.PathAdapter           // 跨平台路径适配器
	useTemplateMode  bool                   // 是否启用模板模式
	useMappingMode   bool                   // 是否启用映射规则模式
}

// NewPathStrategyService 创建路径策略服务
func NewPathStrategyService(
	cfg *config.Config,
	fileService contracts.FileService,
) *PathStrategyService {
	// 检查是否配置了自定义模板
	useTemplateMode := cfg.Download.PathConfig.Templates.TV != "" ||
		cfg.Download.PathConfig.Templates.Movie != "" ||
		cfg.Download.PathConfig.Templates.Variety != ""

	// 创建映射引擎（可选功能）
	var mappingEngine *PathMappingEngine
	useMappingMode := false
	// 映射引擎默认关闭，可以通过配置启用
	// if cfg.Download.PathConfig.EnableMappingEngine {
	// 	mappingEngine = NewPathMappingEngine(cfg)
	// 	useMappingMode = true
	// }

	return &PathStrategyService{
		config:           cfg,
		fileService:      fileService,
		pathValidator:    filesystem.NewPathValidatorService(cfg),
		directoryMgr:     filesystem.NewDirectoryManager(cfg),
		varExtractor:     utils.NewVariableExtractor(),
		templateRenderer: utils.NewTemplateRenderer(cfg.Download.PathConfig.Templates),
		conflictDetector: filesystem.NewConflictDetector(cfg),
		mappingEngine:    mappingEngine,
		pathAdapter:      platform.NewPathAdapter(),
		useTemplateMode:  useTemplateMode,
		useMappingMode:   useMappingMode,
	}
}

// GenerateDownloadPath 生成下载路径（主入口）
func (s *PathStrategyService) GenerateDownloadPath(
	file contracts.FileResponse,
	baseDir string,
) (string, error) {
	logger.Debug("Generating download path",
		"file", file.Name,
		"sourcePath", file.Path,
		"baseDir", baseDir,
		"templateMode", s.useTemplateMode)

	var downloadPath string

	// 1. 根据模式选择路径生成方式
	if s.useMappingMode && s.mappingEngine != nil {
		// 规则映射模式（最高优先级）
		path, err := s.mappingEngine.ApplyRules(file, baseDir)
		if err == nil {
			downloadPath = path
			logger.Debug("Path mapped by rules", "path", downloadPath)
		} else {
			logger.Debug("No mapping rule matched, using fallback", "error", err)
			// 回退到模板或智能模式
			s.useMappingMode = false
		}
	}

	if downloadPath == "" {
		if s.useTemplateMode {
			// 模板模式：使用变量和模板渲染
			vars := s.varExtractor.ExtractVariables(file, baseDir)
			category := vars["category"]
			downloadPath = s.templateRenderer.RenderByCategory(category, vars)

			logger.Debug("Path rendered from template",
				"category", category,
				"path", downloadPath)
		} else {
			// 非模板模式：PathStrategyService 不应被调用
			// 如果到这里，说明配置有问题，返回错误
			logger.Error("PathStrategyService called without template mode enabled")
			return "", fmt.Errorf("PathStrategyService requires template mode to be enabled")
		}
	}

	// 1.5 使用PathAdapter规范化路径（跨平台处理）
	downloadPath = s.pathAdapter.NormalizePath(downloadPath)

	// 2. 路径验证和清理
	cleanPath, err := s.pathValidator.ValidateAndClean(downloadPath)
	if err != nil {
		logger.Warn("Path validation failed, using fallback",
			"original", downloadPath,
			"error", err)

		// 回退到基础目录
		cleanPath = filepath.Join(baseDir, "others")
	}

	// 3. 冲突检测和处理
	if s.conflictDetector != nil {
		// 检查重复下载
		if s.conflictDetector.ShouldSkipDuplicate() {
			if record, err := s.conflictDetector.CheckDuplicateDownload(file.Path); err != nil {
				logger.Warn("Duplicate download detected",
					"file", file.Path,
					"previous_download", record.DownloadedAt)
				return "", err
			}
		}

		// 检查路径冲突
		mediaType := "other"
		if s.useTemplateMode {
			vars := s.varExtractor.ExtractVariables(file, baseDir)
			mediaType = vars["category"]
		}

		if conflict, err := s.conflictDetector.CheckPathConflict(cleanPath, mediaType); conflict {
			logger.Warn("Path conflict detected", "path", cleanPath, "error", err)

			// 根据策略解决冲突
			policy := s.conflictDetector.GetConflictPolicy()
			cleanPath, err = s.conflictDetector.ResolveConflict(cleanPath, policy)
			if err != nil {
				return "", err
			}
		}
	}

	// 4. 确保目录存在
	downloadDir := filepath.Dir(cleanPath)
	if err := s.directoryMgr.EnsureDirectory(downloadDir); err != nil {
		logger.Error("Failed to ensure directory",
			"dir", downloadDir,
			"error", err)
		return "", err
	}

	logger.Debug("Path generation completed",
		"file", file.Name,
		"finalPath", cleanPath)

	return cleanPath, nil
}

// PrepareDownloadDirectory 准备下载目录（用于批量下载前的预检）
func (s *PathStrategyService) PrepareDownloadDirectory(
	baseDir string,
	totalSize int64,
) error {
	logger.Info("Preparing download directory",
		"baseDir", baseDir,
		"totalSize", strutil.FormatFileSize(totalSize))

	// 1. 确保基础目录存在
	if err := s.directoryMgr.EnsureDirectory(baseDir); err != nil {
		return err
	}

	// 2. 检查磁盘空间
	if totalSize > 0 {
		if err := s.directoryMgr.CheckDiskSpace(baseDir, totalSize); err != nil {
			return err
		}
	}

	return nil
}

// ValidatePath 验证路径（不生成，只验证）
func (s *PathStrategyService) ValidatePath(path string) error {
	return s.pathValidator.Validate(path)
}

// CleanPath 清理路径
func (s *PathStrategyService) CleanPath(path string) string {
	return s.pathValidator.CleanPath(path)
}

// NormalizePath 规范化路径（跨平台处理）
func (s *PathStrategyService) NormalizePath(path string) string {
	return s.pathValidator.NormalizePath(path)
}
