package llm

import (
	"bytes"
	"fmt"
	"text/template"
)

// PromptBuilder Prompt构建器
// 提供模板化的Prompt构建能力，支持预定义模板和自定义模板
type PromptBuilder struct {
	templates map[string]*template.Template // 模板映射
}

// NewPromptBuilder 创建Prompt构建器
func NewPromptBuilder() *PromptBuilder {
	builder := &PromptBuilder{
		templates: make(map[string]*template.Template),
	}

	// 注册预定义模板
	builder.registerDefaultTemplates()

	return builder
}

// RegisterTemplate 注册自定义模板
// 参数:
//   - name: 模板名称
//   - tmpl: 模板字符串（使用Go template语法）
// 返回:
//   - error: 模板解析错误
func (b *PromptBuilder) RegisterTemplate(name string, tmpl string) error {
	t, err := template.New(name).Parse(tmpl)
	if err != nil {
		return fmt.Errorf("解析模板失败: %w", err)
	}

	b.templates[name] = t
	return nil
}

// Build 构建Prompt
// 参数:
//   - templateName: 模板名称
//   - data: 模板数据
// 返回:
//   - string: 构建的Prompt
//   - error: 构建错误
func (b *PromptBuilder) Build(templateName string, data interface{}) (string, error) {
	t, ok := b.templates[templateName]
	if !ok {
		return "", fmt.Errorf("模板不存在: %s", templateName)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("执行模板失败: %w", err)
	}

	return buf.String(), nil
}

// BuildSimple 构建简单Prompt（无模板）
// 使用fmt.Sprintf格式化字符串
func (b *PromptBuilder) BuildSimple(format string, args ...interface{}) string {
	return fmt.Sprintf(format, args...)
}

// GetTemplate 获取模板
// 返回模板实例，用于高级用法
func (b *PromptBuilder) GetTemplate(name string) (*template.Template, bool) {
	t, ok := b.templates[name]
	return t, ok
}

// HasTemplate 检查模板是否存在
func (b *PromptBuilder) HasTemplate(name string) bool {
	_, ok := b.templates[name]
	return ok
}

// registerDefaultTemplates 注册预定义模板
func (b *PromptBuilder) registerDefaultTemplates() {
	// 文件命名模板
	b.RegisterTemplate("file_naming", FileNamingTemplate)

	// 内容分类模板（预留）
	b.RegisterTemplate("content_classification", ContentClassificationTemplate)

	// 媒体信息提取模板
	b.RegisterTemplate("media_extraction", MediaExtractionTemplate)
}

// FileNamingData 文件命名模板数据
type FileNamingData struct {
	FileName string // 原始文件名
	FilePath string // 文件路径（可选）
}

// MediaExtractionData 媒体信息提取模板数据
type MediaExtractionData struct {
	FileName string // 文件名
	Context  string // 上下文信息（可选）
}

// 预定义模板常量
const (
	// FileNamingTemplate 文件命名模板
	// 用于分析文件名并提取影视信息
	FileNamingTemplate = `你是一个专业的影视文件命名助手。请分析以下文件名并推断其正确的影视信息。

原始文件名: {{.FileName}}
{{if .FilePath}}路径信息: {{.FilePath}}{{end}}

请输出JSON格式：
{
  "media_type": "tv或movie",
  "title": "标题（英文优先）",
  "title_cn": "中文标题",
  "year": 年份,
  "season": 季度（仅剧集，电影为null）,
  "episode": 集数（仅剧集，电影为null）,
  "confidence": 0.0-1.0的置信度分数
}

规则：
1. 优先识别英文名称，如果只有中文则保留
2. 准确提取年份、季度、集数
3. 剔除视频质量标记（1080p, WEB-DL, BluRay等）
4. 如果信息不明确，将confidence设为较低值（<0.5）
5. 返回的JSON必须是有效的JSON格式`

	// ContentClassificationTemplate 内容分类模板（预留）
	ContentClassificationTemplate = `你是一个内容分类专家。请对以下内容进行分类。

内容: {{.Content}}

请分析内容的类型、主题、情感等，并输出JSON格式：
{
  "category": "类别",
  "subcategory": "子类别",
  "tags": ["标签1", "标签2"],
  "sentiment": "positive/negative/neutral",
  "confidence": 0.0-1.0
}`

	// MediaExtractionTemplate 媒体信息提取模板
	// 用于提取影视媒体的详细信息
	MediaExtractionTemplate = `你是一个影视信息提取专家。请从以下信息中提取影视媒体的详细信息。

文件名: {{.FileName}}
{{if .Context}}上下文: {{.Context}}{{end}}

请提取以下信息并输出JSON格式：
{
  "title": "标题",
  "title_original": "原始标题",
  "year": 年份,
  "media_type": "movie/tv/documentary/anime",
  "season": 季数（如果是剧集）,
  "episode": 集数（如果是剧集）,
  "quality": "视频质量（如1080p, 4K等）",
  "source": "来源（如BluRay, WEB-DL等）",
  "codec": "编码格式（如x264, HEVC等）",
  "audio": "音频信息",
  "language": "语言",
  "subtitles": ["字幕语言"],
  "release_group": "发布组",
  "confidence": 0.0-1.0
}

规则：
1. 尽可能提取完整信息
2. 对于不确定的字段，返回null
3. confidence反映提取信息的可靠性
4. 返回的JSON必须是有效的JSON格式`
)
