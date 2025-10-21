package utils

import (
	"strings"

	"github.com/easayliu/alist-aria2-download/internal/infrastructure/config"
	"github.com/easayliu/alist-aria2-download/pkg/logger"
)

// TemplateRenderer 模板渲染引擎 - 将模板和变量渲染成路径
type TemplateRenderer struct {
	templates config.PathTemplates
}

// NewTemplateRenderer 创建模板渲染器
func NewTemplateRenderer(templates config.PathTemplates) *TemplateRenderer {
	return &TemplateRenderer{
		templates: templates,
	}
}

// Render 渲染模板
func (r *TemplateRenderer) Render(template string, vars map[string]string) string {
	logger.Debug("Rendering template", "template", template, "vars", vars)

	result := template

	// 替换所有变量
	for key, value := range vars {
		placeholder := "{" + key + "}"
		result = strings.ReplaceAll(result, placeholder, value)
	}

	// 清理未替换的变量（保持原样或移除）
	result = r.cleanUnusedPlaceholders(result)

	logger.Debug("Template rendering completed", "result", result)

	return result
}

// RenderByCategory 根据分类渲染模板
func (r *TemplateRenderer) RenderByCategory(category string, vars map[string]string) string {
	template := r.selectTemplate(category)
	return r.Render(template, vars)
}

// selectTemplate 选择模板
func (r *TemplateRenderer) selectTemplate(category string) string {
	switch strings.ToLower(category) {
	case "tv":
		return r.templates.TV
	case "movie":
		return r.templates.Movie
	case "variety":
		return r.templates.Variety
	default:
		return r.templates.Default
	}
}

// cleanUnusedPlaceholders 清理未使用的占位符
func (r *TemplateRenderer) cleanUnusedPlaceholders(path string) string {
	// 移除未替换的变量占位符 {xxx}
	// 简单实现：替换为空字符串
	for {
		start := strings.Index(path, "{")
		if start == -1 {
			break
		}

		end := strings.Index(path[start:], "}")
		if end == -1 {
			break
		}

		// 移除这个占位符
		placeholder := path[start : start+end+1]
		logger.Warn("Unreplaced variable placeholder", "placeholder", placeholder)
		path = strings.Replace(path, placeholder, "", 1)
	}

	// 清理多余的斜杠
	for strings.Contains(path, "//") {
		path = strings.ReplaceAll(path, "//", "/")
	}

	return path
}

// ValidateTemplate 验证模板格式
func (r *TemplateRenderer) ValidateTemplate(template string) error {
	// 检查模板中的占位符是否配对
	openCount := strings.Count(template, "{")
	closeCount := strings.Count(template, "}")

	if openCount != closeCount {
		logger.Warn("Template format error: mismatched braces",
			"template", template,
			"open", openCount,
			"close", closeCount)
	}

	return nil
}
