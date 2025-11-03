package path

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/easayliu/alist-aria2-download/internal/application/contracts"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/config"
	"github.com/easayliu/alist-aria2-download/internal/shared/utils"
	"github.com/easayliu/alist-aria2-download/pkg/logger"
)

// PathMappingEngine 路径映射规则引擎 - 支持复杂的路径转换规则
type PathMappingEngine struct {
	config    *config.Config
	rules     []*PathMappingRule
	renderer  *utils.TemplateRenderer
	extractor *utils.VariableExtractor
}

// PathMappingRule 路径映射规则
type PathMappingRule struct {
	ID          string          // 规则ID
	Name        string          // 规则名称
	Enabled     bool            // 是否启用
	Priority    int             // 优先级（数字越大优先级越高）
	SourceMatch SourceMatchRule // 源匹配规则
	Transform   TransformRule   // 转换规则
}

// SourceMatchRule 源匹配规则
type SourceMatchRule struct {
	PathPattern     string     // 路径模式（支持通配符或正则）
	MediaType       string     // 媒体类型（tv/movie/variety）
	FileNamePattern string     // 文件名模式
	SizeMin         int64      // 最小文件大小（字节）
	SizeMax         int64      // 最大文件大小（字节）
	DateAfter       *time.Time // 修改日期晚于
	IsRegex         bool       // 路径模式是否为正则表达式
}

// TransformRule 转换规则
type TransformRule struct {
	TargetTemplate    string            // 目标路径模板
	Variables         map[string]string // 额外变量
	PreserveStructure bool              // 保留原始目录结构
}

// NewPathMappingEngine 创建路径映射引擎
func NewPathMappingEngine(cfg *config.Config) *PathMappingEngine {
	engine := &PathMappingEngine{
		config:    cfg,
		rules:     make([]*PathMappingRule, 0),
		renderer:  utils.NewTemplateRenderer(cfg.Download.PathConfig.Templates),
		extractor: utils.NewVariableExtractor(),
	}

	// 加载默认规则（可以从配置文件读取）
	engine.loadDefaultRules()

	return engine
}

// loadDefaultRules 加载默认规则
func (e *PathMappingEngine) loadDefaultRules() {
	// 示例规则：将综艺节目从/tvs/综艺/映射到/variety/
	defaultRules := []*PathMappingRule{
		{
			ID:       "rule_variety_to_variety",
			Name:     "综艺节目归类到variety目录",
			Enabled:  true,
			Priority: 100,
			SourceMatch: SourceMatchRule{
				PathPattern: "*/tvs/综艺/*",
				MediaType:   "variety",
			},
			Transform: TransformRule{
				TargetTemplate: "{base}/variety/{show}",
			},
		},
		{
			ID:       "rule_movie_series",
			Name:     "电影系列分组",
			Enabled:  true,
			Priority: 90,
			SourceMatch: SourceMatchRule{
				PathPattern: "*/movies/*系列/*",
				MediaType:   "movie",
			},
			Transform: TransformRule{
				TargetTemplate:    "{base}/movies/series/{title}",
				PreserveStructure: true,
			},
		},
	}

	e.rules = append(e.rules, defaultRules...)
	logger.Info("Default mapping rules loaded", "count", len(defaultRules))
}

// AddRule 添加规则
func (e *PathMappingEngine) AddRule(rule *PathMappingRule) {
	e.rules = append(e.rules, rule)
	e.sortRules()
	logger.Info("Mapping rule added", "id", rule.ID, "name", rule.Name)
}

// RemoveRule 移除规则
func (e *PathMappingEngine) RemoveRule(ruleID string) {
	for i, rule := range e.rules {
		if rule.ID == ruleID {
			e.rules = append(e.rules[:i], e.rules[i+1:]...)
			logger.Info("Mapping rule removed", "id", ruleID)
			return
		}
	}
}

// sortRules 按优先级排序规则
func (e *PathMappingEngine) sortRules() {
	sort.Slice(e.rules, func(i, j int) bool {
		return e.rules[i].Priority > e.rules[j].Priority
	})
}

// ApplyRules 应用规则
func (e *PathMappingEngine) ApplyRules(
	file contracts.FileResponse,
	baseDir string,
) (string, error) {
	logger.Debug("Applying mapping rules",
		"file", file.Name,
		"path", file.Path,
		"rulesCount", len(e.rules))

	// 遍历规则，找到第一个匹配的
	for _, rule := range e.rules {
		if !rule.Enabled {
			continue
		}

		if e.matchRule(file, rule.SourceMatch) {
			logger.Info("Rule matched",
				"rule", rule.Name,
				"file", file.Name)

			// 应用转换规则
			path, err := e.applyTransform(file, baseDir, rule.Transform)
			if err != nil {
				logger.Warn("Rule application failed",
					"rule", rule.Name,
					"error", err)
				continue
			}

			logger.Info("Rule applied successfully",
				"rule", rule.Name,
				"path", path)
			return path, nil
		}
	}

	// 无匹配规则
	return "", fmt.Errorf("no matching mapping rule found")
}

// matchRule 检查文件是否匹配规则
func (e *PathMappingEngine) matchRule(
	file contracts.FileResponse,
	match SourceMatchRule,
) bool {
	// 1. 检查路径模式
	if match.PathPattern != "" {
		if !e.matchPath(file.Path, match.PathPattern, match.IsRegex) {
			return false
		}
	}

	// 2. 检查媒体类型
	if match.MediaType != "" {
		vars := e.extractor.ExtractVariables(file, "")
		if vars["category"] != match.MediaType {
			return false
		}
	}

	// 3. 检查文件名模式
	if match.FileNamePattern != "" {
		if !e.matchPath(file.Name, match.FileNamePattern, match.IsRegex) {
			return false
		}
	}

	// 4. 检查文件大小
	if match.SizeMin > 0 && file.Size < match.SizeMin {
		return false
	}
	if match.SizeMax > 0 && file.Size > match.SizeMax {
		return false
	}

	// 5. 检查修改日期
	if match.DateAfter != nil && !file.Modified.IsZero() {
		if file.Modified.Before(*match.DateAfter) {
			return false
		}
	}

	return true
}

// matchPath 匹配路径模式
func (e *PathMappingEngine) matchPath(path, pattern string, isRegex bool) bool {
	if isRegex {
		// 正则表达式匹配
		matched, err := regexp.MatchString(pattern, path)
		if err != nil {
			logger.Warn("Regex pattern error", "pattern", pattern, "error", err)
			return false
		}
		return matched
	}

	// 通配符匹配（简单实现）
	pattern = strings.ReplaceAll(pattern, "*", ".*")
	pattern = "^" + pattern + "$"
	matched, _ := regexp.MatchString(pattern, path)
	return matched
}

// applyTransform 应用转换规则
func (e *PathMappingEngine) applyTransform(
	file contracts.FileResponse,
	baseDir string,
	transform TransformRule,
) (string, error) {
	// 1. 提取变量
	vars := e.extractor.ExtractVariables(file, baseDir)

	// 2. 合并额外变量
	for key, value := range transform.Variables {
		vars[key] = value
	}

	// 3. 渲染模板
	path := e.renderer.Render(transform.TargetTemplate, vars)

	// 4. 保留原始结构（如果需要）
	if transform.PreserveStructure {
		originalDir := filepath.Dir(file.Path)
		path = filepath.Join(path, filepath.Base(originalDir))
	}

	return path, nil
}

// GetRules 获取所有规则
func (e *PathMappingEngine) GetRules() []*PathMappingRule {
	return e.rules
}

// GetRule 获取指定规则
func (e *PathMappingEngine) GetRule(ruleID string) *PathMappingRule {
	for _, rule := range e.rules {
		if rule.ID == ruleID {
			return rule
		}
	}
	return nil
}

// EnableRule 启用规则
func (e *PathMappingEngine) EnableRule(ruleID string) {
	if rule := e.GetRule(ruleID); rule != nil {
		rule.Enabled = true
		logger.Info("Rule enabled", "id", ruleID)
	}
}

// DisableRule 禁用规则
func (e *PathMappingEngine) DisableRule(ruleID string) {
	if rule := e.GetRule(ruleID); rule != nil {
		rule.Enabled = false
		logger.Info("Rule disabled", "id", ruleID)
	}
}
