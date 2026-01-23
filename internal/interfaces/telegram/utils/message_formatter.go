package utils

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// MessageFormatter message formatting utility - follows Telegram Bot API HTML best practices
//
// Telegram supported HTML tags:
//   - <b>, <strong> - bold
//   - <i>, <em> - italic
//   - <u>, <ins> - underline
//   - <s>, <strike>, <del> - strikethrough
//   - <code> - inline code
//   - <pre> - code block
//   - <pre><code class="language-xxx"> - code block with language identifier
//   - <a href="url"> - link
//   - <tg-spoiler> - spoiler tag
//
// Best practices:
//   - Tag nesting is supported
//   - Only need to escape 4 characters: & < > "
//   - Emoji and Chinese do not need escaping
//   - Let Telegram client render naturally, do not force uniform message width
//
// Reference: https://core.telegram.org/bots/api#html-style
type MessageFormatter struct {
	maxWidth int // 最大宽度(字符数) - 用于内容智能换行参考(不强制)
}

// NewMessageFormatter creates message formatter
func NewMessageFormatter() *MessageFormatter {
	return &MessageFormatter{
		maxWidth: 50, // 内容智能换行的参考宽度（不强制）
	}
}

// FormatTitle formats title - follows Telegram best practices
func (mf *MessageFormatter) FormatTitle(emoji, title string) string {
	// Telegram 官方推荐：简洁清晰的标题格式
	// 使用 emoji 提升可读性和用户体验
	return fmt.Sprintf("<b>%s %s</b>", emoji, title)
}

// FormatSection formats section title
func (mf *MessageFormatter) FormatSection(title string) string {
	return fmt.Sprintf("\n<b>%s</b>", title)
}

// FormatSeparator formats separator line
func (mf *MessageFormatter) FormatSeparator() string {
	return strings.Repeat("─", 30)
}

// FormatField 格式化字段 - 标签:值格式,确保宽度一致
func (mf *MessageFormatter) FormatField(label, value string) string {
	return fmt.Sprintf("<b>%s:</b> %s", label, value)
}

// FormatFieldCode 格式化代码字段
func (mf *MessageFormatter) FormatFieldCode(label, value string) string {
	return fmt.Sprintf("<b>%s:</b> <code>%s</code>", label, value)
}

// FormatListItem 格式化列表项
func (mf *MessageFormatter) FormatListItem(bullet, text string) string {
	return fmt.Sprintf("%s %s", bullet, text)
}

// ========== Telegram HTML 标签格式化方法 ==========

// FormatBold 格式化粗体文本
func (mf *MessageFormatter) FormatBold(text string) string {
	return fmt.Sprintf("<b>%s</b>", text)
}

// FormatItalic 格式化斜体文本
func (mf *MessageFormatter) FormatItalic(text string) string {
	return fmt.Sprintf("<i>%s</i>", text)
}

// FormatUnderline 格式化下划线文本
func (mf *MessageFormatter) FormatUnderline(text string) string {
	return fmt.Sprintf("<u>%s</u>", text)
}

// FormatStrikethrough 格式化删除线文本
func (mf *MessageFormatter) FormatStrikethrough(text string) string {
	return fmt.Sprintf("<s>%s</s>", text)
}

// FormatCode 格式化行内代码
func (mf *MessageFormatter) FormatCode(text string) string {
	return fmt.Sprintf("<code>%s</code>", text)
}

// FormatPre 格式化代码块
func (mf *MessageFormatter) FormatPre(code string) string {
	return fmt.Sprintf("<pre>%s</pre>", code)
}

// FormatPreWithLanguage 格式化带语言标识的代码块
func (mf *MessageFormatter) FormatPreWithLanguage(code, language string) string {
	return fmt.Sprintf("<pre><code class=\"language-%s\">%s</code></pre>", language, code)
}

// FormatLink 格式化链接
func (mf *MessageFormatter) FormatLink(text, url string) string {
	return fmt.Sprintf("<a href=\"%s\">%s</a>", url, text)
}

// FormatSpoiler 格式化剧透文本
func (mf *MessageFormatter) FormatSpoiler(text string) string {
	return fmt.Sprintf("<tg-spoiler>%s</tg-spoiler>", text)
}

// FormatProgressBar 格式化进度条 - 固定宽度
func (mf *MessageFormatter) FormatProgressBar(progress float64, width int) string {
	if width <= 0 {
		width = 20 // 默认宽度
	}

	filled := int(progress / 100.0 * float64(width))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	return fmt.Sprintf("%s %.1f%%", bar, progress)
}

// FormatKeyValue 格式化键值对 - 使用等宽对齐
func (mf *MessageFormatter) FormatKeyValue(key, value string, keyWidth int) string {
	// 计算 key 的显示宽度(中文占2个字符宽度)
	keyDisplayWidth := mf.getDisplayWidth(key)
	padding := keyWidth - keyDisplayWidth
	if padding < 0 {
		padding = 0
	}

	// 使用空格填充以保持对齐
	return fmt.Sprintf("%s%s: %s", key, strings.Repeat(" ", padding), value)
}

// getDisplayWidth 获取字符串的显示宽度(中文算2个字符)
func (mf *MessageFormatter) getDisplayWidth(s string) int {
	width := 0
	for _, r := range s {
		if r > 127 { // 简单判断:ASCII以外的字符算2个宽度
			width += 2
		} else {
			width += 1
		}
	}
	return width
}

// wrapLongText 智能换行处理长文本
func (mf *MessageFormatter) wrapLongText(text string, maxWidth int) string {
	width := mf.getDisplayWidth(text)
	if width <= maxWidth {
		return text
	}

	// 超长则截断并添加省略号
	runes := []rune(text)
	currentWidth := 0
	cutPos := 0

	for i, r := range runes {
		charWidth := 1
		if r > 127 {
			charWidth = 2
		}
		if currentWidth+charWidth > maxWidth-3 { // 预留省略号空间
			cutPos = i
			break
		}
		currentWidth += charWidth
	}

	if cutPos > 0 {
		return string(runes[:cutPos]) + "..."
	}
	return text
}

// TruncateButtonText 截断按钮文本到指定显示宽度（考虑中英文）
// 这是一个公共方法，供其他模块使用
func (mf *MessageFormatter) TruncateButtonText(text string, maxWidth int) string {
	return mf.wrapLongText(text, maxWidth)
}

// formatLongPath 格式化长路径 - 使用换行和缩进
func (mf *MessageFormatter) formatLongPath(path string) string {
	// 如果路径不长，直接返回
	if mf.getDisplayWidth(path) <= mf.maxWidth {
		return path
	}

	// 分割路径组件
	parts := strings.Split(path, "/")
	if len(parts) <= 2 {
		// 路径太短无法拆分，直接截断
		return mf.wrapLongText(path, mf.maxWidth)
	}

	// 尝试智能换行：显示开头和结尾
	first := parts[0]
	if first == "" && len(parts) > 1 {
		first = "/" + parts[1]
	}
	last := parts[len(parts)-1]

	// 构建缩略形式：开头.../结尾
	abbreviated := first + "/.../" + last
	if mf.getDisplayWidth(abbreviated) <= mf.maxWidth {
		return abbreviated
	}

	// 仍然太长，截断结尾
	return mf.wrapLongText(abbreviated, mf.maxWidth)
}

// FormatFieldWithWrap 格式化字段 - 支持自动换行
// 遵循 Telegram HTML 最佳实践,使用嵌套标签增强可读性
func (mf *MessageFormatter) FormatFieldWithWrap(label, value string) string {
	// 计算标签宽度
	labelWidth := mf.getDisplayWidth(label)
	valueMaxWidth := mf.maxWidth - labelWidth - 3 // 3 = ": " + 空格

	// 如果值太长，换行显示
	if mf.getDisplayWidth(value) > valueMaxWidth {
		wrappedValue := mf.wrapLongText(value, mf.maxWidth-3)
		return fmt.Sprintf("<b>%s:</b>\n   %s", label, wrappedValue)
	}

	return mf.FormatField(label, value)
}

// FormatFieldCodeWithWrap 格式化代码字段 - 支持自动换行
// 遵循 Telegram HTML 最佳实践,code 标签用于显示路径、ID 等
func (mf *MessageFormatter) FormatFieldCodeWithWrap(label, value string) string {
	// 计算标签宽度
	labelWidth := mf.getDisplayWidth(label)
	valueMaxWidth := mf.maxWidth - labelWidth - 3

	// 如果值太长，换行显示
	if mf.getDisplayWidth(value) > valueMaxWidth {
		wrappedValue := mf.wrapLongText(value, mf.maxWidth-3)
		return fmt.Sprintf("<b>%s:</b>\n   <code>%s</code>", label, wrappedValue)
	}

	return mf.FormatFieldCode(label, value)
}

// FormatDownloadStatus 格式化下载状态 - 统一格式
type DownloadStatusData struct {
	StatusEmoji    string
	StatusText     string
	ID             string
	Filename       string
	Progress       float64
	CompletedSize  int64
	TotalSize      int64
	Speed          int64
	ErrorMessage   string
	FormatFileSize func(int64) string
}

// FormatDownloadStatus 格式化下载状态消息 - 固定宽度布局
func (mf *MessageFormatter) FormatDownloadStatus(data DownloadStatusData) string {
	var lines []string

	// 标题
	lines = append(lines, mf.FormatTitle(data.StatusEmoji, "下载状态"))
	lines = append(lines, "")

	// 基本信息 - 使用智能换行
	lines = append(lines, mf.FormatFieldCode("任务ID", mf.truncateID(data.ID)))

	wrappedFilename := mf.wrapLongText(data.Filename, mf.maxWidth)
	lines = append(lines, mf.FormatFieldCodeWithWrap("文件名", wrappedFilename))

	lines = append(lines, mf.FormatField("状态", fmt.Sprintf("%s %s", data.StatusEmoji, data.StatusText)))

	// 进度信息
	lines = append(lines, "")
	lines = append(lines, mf.FormatField("进度", mf.FormatProgressBar(data.Progress, 20)))

	// 大小信息
	if data.TotalSize > 0 {
		sizeText := fmt.Sprintf("%s / %s",
			data.FormatFileSize(data.CompletedSize),
			data.FormatFileSize(data.TotalSize))
		lines = append(lines, mf.FormatField("大小", sizeText))
	}

	// 速度信息
	if data.Speed > 0 {
		speedText := fmt.Sprintf("%s/s", data.FormatFileSize(data.Speed))
		lines = append(lines, mf.FormatField("速度", speedText))
	}

	// 错误信息
	if data.ErrorMessage != "" {
		lines = append(lines, "")
		wrappedError := mf.wrapLongText(data.ErrorMessage, mf.maxWidth)
		lines = append(lines, mf.FormatFieldCodeWithWrap("错误", wrappedError))
	}

	message := strings.Join(lines, "\n")
	return message
}

// truncateID 截断ID显示
func (mf *MessageFormatter) truncateID(id string) string {
	if utf8.RuneCountInString(id) <= 8 {
		return id
	}
	return id[:8] + "..."
}

// FormatDownloadList 格式化下载列表 - 固定宽度布局
type DownloadListData struct {
	TotalCount  int
	ActiveCount int
	Downloads   []DownloadItemData
}

type DownloadItemData struct {
	StatusEmoji string
	ID          string
	Filename    string
	Progress    float64
}

func (mf *MessageFormatter) FormatDownloadList(data DownloadListData) string {
	var lines []string

	// 标题
	lines = append(lines, mf.FormatTitle("📥", fmt.Sprintf("下载任务列表 (%d个)", data.TotalCount)))
	lines = append(lines, "")

	// 统计信息
	if data.ActiveCount > 0 {
		lines = append(lines, mf.FormatField("活动任务", fmt.Sprintf("%d 个", data.ActiveCount)))
		lines = append(lines, "")
	}

	// 任务列表 - 固定格式
	displayCount := len(data.Downloads)
	if displayCount > 10 {
		displayCount = 10
	}

	for i := 0; i < displayCount; i++ {
		item := data.Downloads[i]

		// 序号和状态
		prefix := fmt.Sprintf("%d. %s", i+1, item.StatusEmoji)

		// ID (截断)
		shortID := mf.truncateID(item.ID)

		// 文件名和进度 - 使用智能换行
		wrappedFilename := mf.wrapLongText(item.Filename, mf.maxWidth-10)
		taskInfo := fmt.Sprintf("<code>%s</code>\n   %s (%.1f%%)",
			shortID,
			wrappedFilename,
			item.Progress)

		lines = append(lines, fmt.Sprintf("%s %s", prefix, taskInfo))

		if i < displayCount-1 {
			lines = append(lines, "")
		}
	}

	// 显示剩余数量
	if len(data.Downloads) > 10 {
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("... 还有 %d 个任务", len(data.Downloads)-10))
	}

	message := strings.Join(lines, "\n")
	return message
}

// FormatSystemStatus 格式化系统状态 - 固定宽度布局
type SystemStatusData struct {
	ServiceStatus  string
	Port           string
	Mode           string
	AlistURL       string
	AlistPath      string
	Aria2RPC       string
	Aria2Dir       string
	TelegramStatus string
	TelegramUsers  int
	TelegramAdmins int
	OS             string
	Arch           string
}

func (mf *MessageFormatter) FormatSystemStatus(data SystemStatusData) string {
	var lines []string

	// 标题
	lines = append(lines, mf.FormatTitle("🏥", "系统健康检查"))
	lines = append(lines, "")

	// 服务状态
	lines = append(lines, mf.FormatSection("📊 服务状态"))
	lines = append(lines, mf.FormatField("状态", data.ServiceStatus))
	lines = append(lines, mf.FormatFieldCode("端口", data.Port))
	lines = append(lines, mf.FormatFieldCode("模式", data.Mode))

	// Alist配置 - 使用智能换行
	lines = append(lines, mf.FormatSection("📂 Alist配置"))
	wrappedURL := mf.wrapLongText(data.AlistURL, mf.maxWidth-10)
	lines = append(lines, mf.FormatListItem("•", fmt.Sprintf("地址: <code>%s</code>", wrappedURL)))

	wrappedPath := mf.formatLongPath(data.AlistPath)
	lines = append(lines, mf.FormatListItem("•", fmt.Sprintf("默认路径: <code>%s</code>", wrappedPath)))

	// Aria2配置 - 使用智能换行
	lines = append(lines, mf.FormatSection("⬇️ Aria2配置"))
	wrappedRPC := mf.wrapLongText(data.Aria2RPC, mf.maxWidth-10)
	lines = append(lines, mf.FormatListItem("•", fmt.Sprintf("RPC地址: <code>%s</code>", wrappedRPC)))

	wrappedDir := mf.formatLongPath(data.Aria2Dir)
	lines = append(lines, mf.FormatListItem("•", fmt.Sprintf("下载目录: <code>%s</code>", wrappedDir)))

	// Telegram配置
	lines = append(lines, mf.FormatSection("📱 Telegram配置"))
	lines = append(lines, mf.FormatListItem("•", fmt.Sprintf("状态: %s", data.TelegramStatus)))
	if data.TelegramUsers > 0 {
		lines = append(lines, mf.FormatListItem("•", fmt.Sprintf("授权用户数: %d", data.TelegramUsers)))
		lines = append(lines, mf.FormatListItem("•", fmt.Sprintf("管理员数: %d", data.TelegramAdmins)))
	}

	// 系统信息
	lines = append(lines, mf.FormatSection("💻 系统信息"))
	lines = append(lines, mf.FormatListItem("•", fmt.Sprintf("操作系统: <code>%s</code>", data.OS)))
	lines = append(lines, mf.FormatListItem("•", fmt.Sprintf("系统架构: <code>%s</code>", data.Arch)))

	message := strings.Join(lines, "\n")
	return message
}

// FormatBatchResult 格式化批量操作结果 - 固定宽度布局
type BatchResultData struct {
	Title        string
	TotalFiles   int
	VideoFiles   int
	SuccessCount int
	FailureCount int
	MovieCount   int
	TVCount      int
	OtherCount   int
	TotalSize    string
}

func (mf *MessageFormatter) FormatBatchResult(data BatchResultData) string {
	var lines []string

	// 标题
	lines = append(lines, mf.FormatTitle("📊", data.Title))
	lines = append(lines, "")

	// 文件统计
	if data.VideoFiles > 0 {
		lines = append(lines, mf.FormatSection("文件统计"))
		lines = append(lines, mf.FormatListItem("•", fmt.Sprintf("视频文件: %d 个", data.VideoFiles)))
		if data.TotalSize != "" {
			lines = append(lines, mf.FormatListItem("•", fmt.Sprintf("总大小: %s", data.TotalSize)))
		}
		if data.MovieCount > 0 {
			lines = append(lines, mf.FormatListItem("•", fmt.Sprintf("电影: %d 个", data.MovieCount)))
		}
		if data.TVCount > 0 {
			lines = append(lines, mf.FormatListItem("•", fmt.Sprintf("剧集: %d 个", data.TVCount)))
		}
		if data.OtherCount > 0 {
			lines = append(lines, mf.FormatListItem("•", fmt.Sprintf("其他: %d 个", data.OtherCount)))
		}
		lines = append(lines, "")
	}

	// 下载结果
	lines = append(lines, mf.FormatSection("下载结果"))
	lines = append(lines, mf.FormatListItem("•", fmt.Sprintf("成功: %d", data.SuccessCount)))
	if data.FailureCount > 0 {
		lines = append(lines, mf.FormatListItem("•", fmt.Sprintf("失败: %d", data.FailureCount)))
	}

	// 成功提示
	if data.SuccessCount > 0 {
		lines = append(lines, "")
		lines = append(lines, "✅ 所有任务已使用自动路径分类功能")
		lines = append(lines, "📥 可通过「下载管理」查看任务状态")
	}

	// 失败警告
	if data.FailureCount > 0 {
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("⚠️ 有 %d 个文件下载失败", data.FailureCount))
	}

	message := strings.Join(lines, "\n")
	return message
}

// FormatFileInfo 格式化文件信息 - 固定宽度布局
type FileInfoData struct {
	Icon       string
	Name       string
	Path       string
	Type       string
	Size       string
	Modified   string
	IsDir      bool
	EscapeHTML func(string) string
}

func (mf *MessageFormatter) FormatFileInfo(data FileInfoData) string {
	var lines []string

	// 标题
	lines = append(lines, mf.FormatTitle(data.Icon, "文件信息"))
	lines = append(lines, "")

	// 基本信息 - 使用智能换行
	wrappedName := mf.wrapLongText(data.Name, mf.maxWidth)
	lines = append(lines, mf.FormatFieldCodeWithWrap("名称", data.EscapeHTML(wrappedName)))

	formattedPath := mf.formatLongPath(data.Path)
	lines = append(lines, mf.FormatFieldCodeWithWrap("路径", data.EscapeHTML(formattedPath)))

	if data.Type != "" {
		lines = append(lines, mf.FormatFieldCode("类型", data.Type))
	}

	if !data.IsDir && data.Size != "" {
		lines = append(lines, mf.FormatField("大小", data.Size))
	}

	if data.Modified != "" {
		lines = append(lines, mf.FormatField("修改时间", data.Modified))
	}

	message := strings.Join(lines, "\n")
	return message
}

// FormatFileOperation 格式化文件操作 - 固定宽度布局
type FileOperationData struct {
	Icon       string
	FileName   string
	FilePath   string
	FileType   string
	Prompt     string
	EscapeHTML func(string) string
}

func (mf *MessageFormatter) FormatFileOperation(data FileOperationData) string {
	var lines []string

	// 标题
	lines = append(lines, mf.FormatTitle(data.Icon, "文件操作"))
	lines = append(lines, "")

	// 文件信息 - 使用智能换行
	wrappedName := mf.wrapLongText(data.FileName, mf.maxWidth)
	lines = append(lines, mf.FormatFieldCodeWithWrap("文件", data.EscapeHTML(wrappedName)))

	formattedPath := mf.formatLongPath(data.FilePath)
	lines = append(lines, mf.FormatFieldCodeWithWrap("路径", data.EscapeHTML(formattedPath)))

	if data.FileType != "" {
		lines = append(lines, mf.FormatFieldCode("类型", data.FileType))
	}

	// 提示信息
	if data.Prompt != "" {
		lines = append(lines, "")
		lines = append(lines, data.Prompt)
	}

	message := strings.Join(lines, "\n")
	return message
}

// FormatTaskList 格式化任务列表 - 固定宽度布局
type TaskListData struct {
	TotalCount int
	Tasks      []TaskItemData
}

type TaskItemData struct {
	ID          string
	Name        string
	Schedule    string
	Status      string
	StatusEmoji string
	LastRun     string
	NextRun     string
}

func (mf *MessageFormatter) FormatTaskList(data TaskListData) string {
	var lines []string

	// 标题
	lines = append(lines, mf.FormatTitle("⏰", fmt.Sprintf("定时任务 (%d个)", data.TotalCount)))
	lines = append(lines, "")

	if data.TotalCount == 0 {
		lines = append(lines, "暂无定时任务")
		message := strings.Join(lines, "\n")
		return message
	}

	// 任务列表
	for i, task := range data.Tasks {
		// 任务标题 - 使用智能换行
		wrappedName := mf.wrapLongText(task.Name, mf.maxWidth-10)
		taskTitle := fmt.Sprintf("%d. %s %s", i+1, task.StatusEmoji, wrappedName)
		lines = append(lines, fmt.Sprintf("<b>%s</b>", taskTitle))

		// 任务详情
		lines = append(lines, fmt.Sprintf("   ID: <code>%s</code>", task.ID))
		lines = append(lines, fmt.Sprintf("   计划: %s", task.Schedule))

		if task.LastRun != "" {
			lines = append(lines, fmt.Sprintf("   上次: %s", task.LastRun))
		}

		if task.NextRun != "" {
			lines = append(lines, fmt.Sprintf("   下次: %s", task.NextRun))
		}

		if i < len(data.Tasks)-1 {
			lines = append(lines, "")
		}
	}

	message := strings.Join(lines, "\n")
	return message
}

// FormatFileBrowser 格式化文件浏览器 - 固定宽度布局
type FileBrowserData struct {
	Path       string
	Page       int
	TotalPages int
	TotalFiles int
	DirCount   int
	FileCount  int
	VideoCount int
	EscapeHTML func(string) string
}

func (mf *MessageFormatter) FormatFileBrowser(data FileBrowserData) string {
	var lines []string

	// 标题
	lines = append(lines, mf.FormatTitle("📁", "文件浏览器"))
	lines = append(lines, "")

	// 路径信息 - 使用智能换行
	formattedPath := mf.formatLongPath(data.Path)
	lines = append(lines, mf.FormatFieldCodeWithWrap("当前路径", data.EscapeHTML(formattedPath)))

	// 统计信息（如果有）
	if data.TotalFiles > 0 {
		lines = append(lines, mf.FormatField("文件总数", fmt.Sprintf("%d 个", data.TotalFiles)))

		if data.DirCount > 0 || data.FileCount > 0 {
			stats := []string{}
			if data.DirCount > 0 {
				stats = append(stats, fmt.Sprintf("目录 %d", data.DirCount))
			}
			if data.FileCount > 0 {
				stats = append(stats, fmt.Sprintf("文件 %d", data.FileCount))
			}
			if data.VideoCount > 0 {
				stats = append(stats, fmt.Sprintf("视频 %d", data.VideoCount))
			}
			if len(stats) > 0 {
				lines = append(lines, mf.FormatField("分类", strings.Join(stats, " • ")))
			}
		}
	}

	// 页码信息
	if data.TotalPages > 1 {
		lines = append(lines, mf.FormatField("页码", fmt.Sprintf("第 %d/%d 页", data.Page, data.TotalPages)))
	} else if data.Page > 0 {
		lines = append(lines, mf.FormatField("页码", fmt.Sprintf("第 %d 页", data.Page)))
	}

	message := strings.Join(lines, "\n")
	return message
}

// FormatTimeRangeHelp 格式化时间范围帮助信息
func (mf *MessageFormatter) FormatTimeRangeHelp(errorMsg string) string {
	var lines []string

	// 标题
	if errorMsg != "" {
		lines = append(lines, mf.FormatTitle("❌", "时间参数错误"))
		lines = append(lines, "")
		lines = append(lines, errorMsg)
	} else {
		lines = append(lines, mf.FormatTitle("ℹ️", "时间参数帮助"))
	}

	lines = append(lines, "")
	lines = append(lines, mf.FormatSection("支持的格式"))
	lines = append(lines, mf.FormatListItem("•", "<code>/download</code> - 预览最近24小时"))
	lines = append(lines, mf.FormatListItem("•", "<code>/download 48</code> - 预览最近48小时"))
	lines = append(lines, mf.FormatListItem("•", "<code>/download 2025-09-01 2025-09-26</code> - 预览日期范围"))
	lines = append(lines, mf.FormatListItem("•", "<code>/download 2025-09-01T00:00:00Z ...</code> - 精确时间"))

	lines = append(lines, "")
	lines = append(lines, mf.FormatField("提示", "在命令后添加 <code>confirm</code> 可直接开始下载"))

	message := strings.Join(lines, "\n")
	return message
}

// FormatDownloadControl 格式化下载控制中心
type DownloadControlData struct {
	ActiveCount  int
	WaitingCount int
	PausedCount  int
	TotalCount   int
}

func (mf *MessageFormatter) FormatDownloadControl(data DownloadControlData) string {
	var lines []string

	// 标题
	lines = append(lines, mf.FormatTitle("🎛️", "下载控制中心"))
	lines = append(lines, "")

	// 状态统计
	lines = append(lines, mf.FormatSection("当前状态"))
	lines = append(lines, mf.FormatListItem("•", fmt.Sprintf("活动任务: %d 个", data.ActiveCount)))
	lines = append(lines, mf.FormatListItem("•", fmt.Sprintf("等待任务: %d 个", data.WaitingCount)))
	lines = append(lines, mf.FormatListItem("•", fmt.Sprintf("暂停任务: %d 个", data.PausedCount)))
	lines = append(lines, mf.FormatListItem("•", fmt.Sprintf("总任务数: %d 个", data.TotalCount)))

	message := strings.Join(lines, "\n")
	return message
}

// FormatFileBrowseCenter 格式化文件浏览中心
func (mf *MessageFormatter) FormatFileBrowseCenter() string {
	var lines []string

	// 标题
	lines = append(lines, mf.FormatTitle("📁", "文件浏览中心"))
	lines = append(lines, "")

	// 功能列表
	lines = append(lines, mf.FormatSection("可用功能"))
	lines = append(lines, mf.FormatListItem("•", "文件浏览 - 浏览目录、查看文件信息"))
	lines = append(lines, mf.FormatListItem("•", "文件搜索 - 快速查找目标文件"))
	lines = append(lines, mf.FormatListItem("•", "查看详情 - 文件大小、修改时间"))
	lines = append(lines, mf.FormatListItem("•", "文件下载 - 从指定路径下载文件"))
	lines = append(lines, mf.FormatListItem("•", "批量下载 - 多个文件同时下载"))
	lines = append(lines, "")
	lines = append(lines, "选择操作：")

	message := strings.Join(lines, "\n")
	return message
}

// FormatWelcome 格式化欢迎消息
func (mf *MessageFormatter) FormatWelcome() string {
	var lines []string

	// 标题
	lines = append(lines, mf.FormatTitle("👋", "欢迎使用 Alist-Aria2 下载管理器"))
	lines = append(lines, "")

	// 功能模块
	lines = append(lines, mf.FormatSection("功能模块"))
	lines = append(lines, mf.FormatListItem("•", "下载管理 - 创建、监控、控制下载任务"))
	lines = append(lines, mf.FormatListItem("•", "文件浏览 - 浏览目录、查看文件信息"))
	lines = append(lines, mf.FormatListItem("•", "定时任务 - 自动化下载管理"))
	lines = append(lines, mf.FormatListItem("•", "系统管理 - 系统状态、健康检查"))
	lines = append(lines, "")
	lines = append(lines, "选择一个功能开始使用：")

	message := strings.Join(lines, "\n")
	return message
}

// FormatHelp 格式化帮助消息
func (mf *MessageFormatter) FormatHelp() string {
	var lines []string

	// 标题
	lines = append(lines, mf.FormatTitle("❓", "使用帮助"))
	lines = append(lines, "")

	// 快捷按钮
	lines = append(lines, mf.FormatSection("快捷按钮"))
	lines = append(lines, "使用下方键盘按钮进行常用操作")
	lines = append(lines, "")

	// 常用命令
	lines = append(lines, mf.FormatSection("常用命令"))
	lines = append(lines, mf.FormatListItem("•", "<code>/download</code> - 开始下载"))
	lines = append(lines, mf.FormatListItem("•", "<code>/status</code> - 查看下载状态"))
	lines = append(lines, mf.FormatListItem("•", "<code>/cancel &lt;ID&gt;</code> - 取消下载"))
	lines = append(lines, mf.FormatListItem("•", "<code>/list</code> - 浏览文件"))
	lines = append(lines, "")

	// 下载命令
	lines = append(lines, mf.FormatSection("下载命令"))
	lines = append(lines, mf.FormatListItem("•", "<code>/download</code> - 预览最近24小时文件"))
	lines = append(lines, mf.FormatListItem("•", "<code>/download 48</code> - 预览最近48小时"))
	lines = append(lines, mf.FormatListItem("•", "<code>/download confirm</code> - 直接下载"))
	lines = append(lines, mf.FormatListItem("•", "<code>/download URL</code> - 从URL下载"))
	lines = append(lines, "")

	// 提示信息
	lines = append(lines, mf.FormatField("提示", "点击命令可直接复制使用"))

	message := strings.Join(lines, "\n")
	return message
}

// FormatManagePanel 格式化管理面板
func (mf *MessageFormatter) FormatManagePanel() string {
	var lines []string

	// 标题
	lines = append(lines, mf.FormatTitle("⚙️", "管理面板"))
	lines = append(lines, "")

	// 功能说明
	lines = append(lines, mf.FormatSection("管理功能"))
	lines = append(lines, mf.FormatListItem("•", "系统状态 - 查看系统运行状态信息"))
	lines = append(lines, mf.FormatListItem("•", "下载管理 - 管理所有下载任务列表"))
	lines = append(lines, mf.FormatListItem("•", "定时任务 - 配置自动化下载计划"))
	lines = append(lines, mf.FormatListItem("•", "健康检查 - 检查服务运行健康度"))
	lines = append(lines, "")
	lines = append(lines, "选择管理功能：")

	message := strings.Join(lines, "\n")
	return message
}

// FormatTimeRangeDownloadPreview 格式化时间范围下载预览
type TimeRangeDownloadPreviewData struct {
	TimeDescription string
	Path            string
	TotalFiles      int
	TotalSize       string
	MovieCount      int
	TVCount         int
	OtherCount      int
	ExampleFiles    []ExampleFileData
	ConfirmCommand  string
	EscapeHTML      func(string) string
}

type ExampleFileData struct {
	Name         string
	DownloadPath string
}

func (mf *MessageFormatter) FormatTimeRangeDownloadPreview(data TimeRangeDownloadPreviewData) string {
	var lines []string

	// 标题
	lines = append(lines, mf.FormatTitle("👁️", "手动下载预览"))
	lines = append(lines, "")

	// 时间和路径信息 - 使用智能换行
	lines = append(lines, mf.FormatField("时间范围", data.TimeDescription))

	formattedPath := mf.formatLongPath(data.Path)
	lines = append(lines, mf.FormatFieldCodeWithWrap("路径", data.EscapeHTML(formattedPath)))
	lines = append(lines, "")

	// 文件统计
	lines = append(lines, mf.FormatSection("文件统计"))
	lines = append(lines, mf.FormatListItem("•", fmt.Sprintf("总文件: %d 个", data.TotalFiles)))
	lines = append(lines, mf.FormatListItem("•", fmt.Sprintf("总大小: %s", data.TotalSize)))
	lines = append(lines, mf.FormatListItem("•", fmt.Sprintf("电影: %d 个", data.MovieCount)))
	lines = append(lines, mf.FormatListItem("•", fmt.Sprintf("剧集: %d 个", data.TVCount)))
	lines = append(lines, mf.FormatListItem("•", fmt.Sprintf("其他: %d 个", data.OtherCount)))

	// 示例文件 - 使用智能换行
	if len(data.ExampleFiles) > 0 {
		lines = append(lines, "")
		lines = append(lines, mf.FormatSection("示例文件"))
		for _, file := range data.ExampleFiles {
			wrappedName := mf.wrapLongText(file.Name, mf.maxWidth-10)
			wrappedPath := mf.wrapLongText(file.DownloadPath, mf.maxWidth-10)
			lines = append(lines, mf.FormatListItem("•", fmt.Sprintf("%s → <code>%s</code>",
				data.EscapeHTML(wrappedName),
				data.EscapeHTML(wrappedPath))))
		}
	}

	// 确认命令提示
	if data.ConfirmCommand != "" {
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("⚠️ 预览有效期 10 分钟。也可以发送 <code>%s</code> 开始下载。", data.ConfirmCommand))
	}

	message := strings.Join(lines, "\n")
	return message
}

// FormatTimeRangeDownloadResult 格式化时间范围下载结果
type TimeRangeDownloadResultData struct {
	Title           string
	TimeDescription string
	Path            string
	TotalFiles      int
	TotalSize       string
	MovieCount      int
	TVCount         int
	OtherCount      int
	SuccessCount    int
	FailCount       int
	EscapeHTML      func(string) string
}

func (mf *MessageFormatter) FormatTimeRangeDownloadResult(data TimeRangeDownloadResultData) string {
	var lines []string

	// 标题
	emoji := "✅"
	if data.FailCount > 0 {
		emoji = "⚠️"
	}
	title := data.Title
	if title == "" {
		title = "手动下载任务已创建"
	}
	lines = append(lines, mf.FormatTitle(emoji, title))
	lines = append(lines, "")

	// 时间和路径信息 - 使用智能换行
	lines = append(lines, mf.FormatField("时间范围", data.TimeDescription))

	formattedPath := mf.formatLongPath(data.Path)
	lines = append(lines, mf.FormatFieldCodeWithWrap("路径", data.EscapeHTML(formattedPath)))
	lines = append(lines, "")

	// 文件统计
	lines = append(lines, mf.FormatSection("文件统计"))
	lines = append(lines, mf.FormatListItem("•", fmt.Sprintf("总文件: %d 个", data.TotalFiles)))
	lines = append(lines, mf.FormatListItem("•", fmt.Sprintf("总大小: %s", data.TotalSize)))
	lines = append(lines, mf.FormatListItem("•", fmt.Sprintf("电影: %d 个", data.MovieCount)))
	lines = append(lines, mf.FormatListItem("•", fmt.Sprintf("剧集: %d 个", data.TVCount)))
	lines = append(lines, mf.FormatListItem("•", fmt.Sprintf("其他: %d 个", data.OtherCount)))
	lines = append(lines, "")

	// 下载结果
	lines = append(lines, mf.FormatSection("下载结果"))
	lines = append(lines, mf.FormatListItem("•", fmt.Sprintf("成功: %d", data.SuccessCount)))
	lines = append(lines, mf.FormatListItem("•", fmt.Sprintf("失败: %d", data.FailCount)))

	// 失败警告
	if data.FailCount > 0 {
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("⚠️ 有 %d 个文件下载失败，请检查日志获取详细信息", data.FailCount))
	}

	message := strings.Join(lines, "\n")
	return message
}

// FormatNoFilesFound 格式化未找到文件消息
func (mf *MessageFormatter) FormatNoFilesFound(title, timeDescription string) string {
	var lines []string

	emoji := "ℹ️"
	lines = append(lines, mf.FormatTitle(emoji, title))
	lines = append(lines, "")
	lines = append(lines, mf.FormatField("时间范围", timeDescription))
	lines = append(lines, "")
	lines = append(lines, mf.FormatField("结果", "未找到符合条件的文件"))

	message := strings.Join(lines, "\n")
	return message
}

// FormatYesterdayFiles 格式化昨日文件列表
type YesterdayFilesData struct {
	TotalCount     int
	DisplayFiles   []YesterdayFileItem
	TotalSize      string
	TVCount        int
	MovieCount     int
	OtherCount     int
	RemainingCount int
	EscapeHTML     func(string) string
}

type YesterdayFileItem struct {
	MediaType     string
	Name          string
	SizeFormatted string
}

func (mf *MessageFormatter) FormatYesterdayFiles(data YesterdayFilesData) string {
	var lines []string

	// 标题
	lines = append(lines, mf.FormatTitle("📅", fmt.Sprintf("昨天的文件 (%d个)", data.TotalCount)))
	lines = append(lines, "")

	// 文件列表 - 使用智能换行
	for _, file := range data.DisplayFiles {
		wrappedName := mf.wrapLongText(file.Name, mf.maxWidth-15)
		lines = append(lines, mf.FormatListItem("•", fmt.Sprintf("[%s] %s (%s)",
			file.MediaType,
			data.EscapeHTML(wrappedName),
			file.SizeFormatted)))
	}

	// 剩余文件提示
	if data.RemainingCount > 0 {
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("... 还有 %d 个文件未显示", data.RemainingCount))
	}

	// 统计信息
	lines = append(lines, "")
	lines = append(lines, mf.FormatSection("统计信息"))
	lines = append(lines, mf.FormatListItem("•", fmt.Sprintf("总大小: %s", data.TotalSize)))
	if data.TVCount > 0 {
		lines = append(lines, mf.FormatListItem("•", fmt.Sprintf("电视剧: %d", data.TVCount)))
	}
	if data.MovieCount > 0 {
		lines = append(lines, mf.FormatListItem("•", fmt.Sprintf("电影: %d", data.MovieCount)))
	}
	if data.OtherCount > 0 {
		lines = append(lines, mf.FormatListItem("•", fmt.Sprintf("其他: %d", data.OtherCount)))
	}

	message := strings.Join(lines, "\n")
	return message
}

// FormatBatchDownloadResult2 格式化批量下载结果（简化版）
type BatchDownloadResult2Data struct {
	SuccessCount int
	FailCount    int
	TotalCount   int
}

func (mf *MessageFormatter) FormatBatchDownloadResult2(data BatchDownloadResult2Data) string {
	var lines []string

	// 标题
	emoji := "✅"
	if data.FailCount > 0 {
		emoji = "⚠️"
	}
	lines = append(lines, mf.FormatTitle(emoji, "下载任务创建完成"))
	lines = append(lines, "")

	// 结果统计
	lines = append(lines, mf.FormatListItem("•", fmt.Sprintf("成功: %d", data.SuccessCount)))
	if data.FailCount > 0 {
		lines = append(lines, mf.FormatListItem("•", fmt.Sprintf("失败: %d", data.FailCount)))
	}
	lines = append(lines, mf.FormatListItem("•", fmt.Sprintf("总计: %d", data.TotalCount)))

	message := strings.Join(lines, "\n")
	return message
}

// FormatSimpleSystemStatus 格式化简单系统状态
type SimpleSystemStatusData struct {
	TelegramStatus string
	Aria2Status    string
	Aria2Version   string
	ServerPort     string
	ServerMode     string
}

func (mf *MessageFormatter) FormatSimpleSystemStatus(data SimpleSystemStatusData) string {
	var lines []string

	lines = append(lines, mf.FormatTitle("ℹ️", "系统状态"))
	lines = append(lines, "")
	lines = append(lines, mf.FormatField("Telegram Bot", data.TelegramStatus))
	lines = append(lines, mf.FormatField("Aria2", fmt.Sprintf("%s (版本: %s)", data.Aria2Status, data.Aria2Version)))
	lines = append(lines, mf.FormatField("服务器", fmt.Sprintf("运行中 (端口: %s, 模式: %s)", data.ServerPort, data.ServerMode)))

	message := strings.Join(lines, "\n")
	return message
}

// FormatRuntimeInfo 格式化运行时信息
type RuntimeInfoData struct {
	GoVersion    string
	CPUCores     int
	MemoryUsage  float64
	SystemMemory float64
	Goroutines   int
	CheckTime    string
}

func (mf *MessageFormatter) FormatRuntimeInfo(data RuntimeInfoData) string {
	var lines []string

	lines = append(lines, "")
	lines = append(lines, mf.FormatFieldCode("Go版本", data.GoVersion))
	lines = append(lines, mf.FormatFieldCode("CPU核心数", fmt.Sprintf("%d", data.CPUCores)))
	lines = append(lines, mf.FormatFieldCode("内存使用", fmt.Sprintf("%.2f MB", data.MemoryUsage)))
	lines = append(lines, mf.FormatFieldCode("系统内存", fmt.Sprintf("%.2f MB", data.SystemMemory)))
	lines = append(lines, mf.FormatFieldCode("Goroutine数", fmt.Sprintf("%d", data.Goroutines)))
	lines = append(lines, "")
	lines = append(lines, mf.FormatField("🕐 检查时间", data.CheckTime))

	return strings.Join(lines, "\n")
}

// FormatAlistConnectionResult 格式化 Alist 连接结果
type AlistConnectionData struct {
	Success  bool
	URL      string
	Username string
	Error    string
}

func (mf *MessageFormatter) FormatAlistConnectionResult(data AlistConnectionData) string {
	var lines []string

	if data.Success {
		lines = append(lines, mf.FormatTitle("✅", "Alist连接成功！"))
		lines = append(lines, "")
		// 使用智能换行处理长URL
		wrappedURL := mf.wrapLongText(data.URL, mf.maxWidth)
		lines = append(lines, mf.FormatFieldCodeWithWrap("地址", wrappedURL))
		lines = append(lines, mf.FormatFieldCode("用户", data.Username))
		lines = append(lines, "")
		lines = append(lines, "现在可以开始使用下载功能了")
	} else {
		lines = append(lines, mf.FormatTitle("❌", "Alist连接失败"))
		lines = append(lines, "")
		// 使用智能换行处理长URL
		wrappedURL := mf.wrapLongText(data.URL, mf.maxWidth)
		lines = append(lines, mf.FormatFieldCodeWithWrap("地址", wrappedURL))
		if data.Error != "" {
			lines = append(lines, mf.FormatField("错误", data.Error))
		}
		lines = append(lines, "")
		lines = append(lines, "请检查配置并重试")
	}

	message := strings.Join(lines, "\n")
	return message
}

// FormatDownloadCreated 格式化下载创建成功消息
type DownloadCreatedData struct {
	URL      string
	GID      string
	Filename string
}

func (mf *MessageFormatter) FormatDownloadCreated(data DownloadCreatedData) string {
	var lines []string

	lines = append(lines, mf.FormatTitle("✅", "下载任务已创建"))
	lines = append(lines, "")

	// 使用智能换行处理长URL
	wrappedURL := mf.wrapLongText(data.URL, mf.maxWidth)
	lines = append(lines, mf.FormatFieldCodeWithWrap("URL", wrappedURL))

	lines = append(lines, mf.FormatFieldCode("GID", data.GID))

	// 使用智能换行处理长文件名
	wrappedFilename := mf.wrapLongText(data.Filename, mf.maxWidth)
	lines = append(lines, mf.FormatFieldCodeWithWrap("文件名", wrappedFilename))

	message := strings.Join(lines, "\n")
	return message
}

// FormatDownloadCancelled 格式化下载取消消息
func (mf *MessageFormatter) FormatDownloadCancelled(gid string) string {
	var lines []string

	lines = append(lines, mf.FormatTitle("🚫", "下载已取消"))
	lines = append(lines, "")
	lines = append(lines, mf.FormatFieldCode("下载GID", gid))

	message := strings.Join(lines, "\n")
	return message
}

// FileDownloadSuccessData 文件下载成功数据
type FileDownloadSuccessData struct {
	Filename     string
	FilePath     string
	DownloadPath string
	TaskID       string
	Size         string
	EscapeHTML   func(string) string
}

// FormatFileDownloadSuccess 格式化文件下载成功消息
func (mf *MessageFormatter) FormatFileDownloadSuccess(data FileDownloadSuccessData) string {
	var lines []string

	lines = append(lines, mf.FormatTitle("✅", "文件下载任务已创建"))
	lines = append(lines, "")
	lines = append(lines, mf.FormatFieldCode("文件", data.EscapeHTML(data.Filename)))
	lines = append(lines, mf.FormatFieldCode("路径", data.EscapeHTML(data.FilePath)))
	lines = append(lines, mf.FormatFieldCode("下载路径", data.EscapeHTML(data.DownloadPath)))
	lines = append(lines, mf.FormatFieldCode("任务ID", data.EscapeHTML(data.TaskID)))
	lines = append(lines, mf.FormatField("大小", data.Size))

	return strings.Join(lines, "\n")
}

// FormatError 格式化错误消息
func (mf *MessageFormatter) FormatError(action string, err error) string {
	return fmt.Sprintf("❌ %s失败: %v", action, err)
}

// FormatSimpleError 格式化简单错误消息（只有错误描述）
func (mf *MessageFormatter) FormatSimpleError(message string) string {
	return fmt.Sprintf("❌ %s", message)
}

// RenameResultData 重命名结果数据
type RenameResultData struct {
	UseLLM             bool
	TotalCount         int
	SuccessCount       int
	FailedCount        int
	SkippedCount       int // 已标准化
	ConflictCount      int // 冲突跳过
	UnprocessableCount int // 无法处理
	SuccessItems       []RenameResultItem
	FailedItems        []RenameResultItem
	MaxDisplayItems    int
}

// RenameResultItem 单个重命名结果项
type RenameResultItem struct {
	Index      int
	OldPath    string
	NewPath    string
	Error      string
	IsConflict bool
}

// FormatRenameResult 格式化重命名结果
func (mf *MessageFormatter) FormatRenameResult(data RenameResultData) string {
	var lines []string

	// 标题
	emoji := "✅"
	title := "批量重命名完成"
	if data.FailedCount > 0 {
		if data.SuccessCount > 0 {
			emoji = "⚠️"
			title = "批量重命名部分完成"
		} else {
			emoji = "❌"
			title = "批量重命名失败"
		}
	}
	lines = append(lines, mf.FormatTitle(emoji, title))
	lines = append(lines, "")

	// 重命名方式
	if data.UseLLM {
		lines = append(lines, "🤖 使用LLM智能重命名")
	} else {
		lines = append(lines, "🎬 使用TMDB重命名")
	}
	lines = append(lines, "")

	// 详细结果
	displayCount := 0
	maxDisplay := data.MaxDisplayItems
	if maxDisplay <= 0 {
		maxDisplay = 10
	}

	// 成功项
	for _, item := range data.SuccessItems {
		if displayCount >= maxDisplay {
			break
		}
		oldName := mf.wrapLongText(item.OldPath, mf.maxWidth-5)
		newName := mf.wrapLongText(item.NewPath, mf.maxWidth-5)
		lines = append(lines, fmt.Sprintf("%d. ✅ <code>%s</code>", item.Index, oldName))
		lines = append(lines, fmt.Sprintf("   → <code>%s</code>", newName))
		lines = append(lines, "")
		displayCount++
	}

	// 失败项
	for _, item := range data.FailedItems {
		if displayCount >= maxDisplay {
			break
		}
		oldName := mf.wrapLongText(item.OldPath, mf.maxWidth-5)
		if item.IsConflict {
			lines = append(lines, fmt.Sprintf("%d. 🔀 <code>%s</code>", item.Index, oldName))
			lines = append(lines, "   目标文件冲突，已跳过")
		} else {
			lines = append(lines, fmt.Sprintf("%d. ❌ <code>%s</code>", item.Index, oldName))
			if item.Error != "" {
				errMsg := mf.wrapLongText(item.Error, mf.maxWidth-8)
				lines = append(lines, fmt.Sprintf("   失败: %s", errMsg))
			}
		}
		lines = append(lines, "")
		displayCount++
	}

	// 省略提示
	totalItems := len(data.SuccessItems) + len(data.FailedItems)
	if totalItems > maxDisplay {
		lines = append(lines, fmt.Sprintf("... 还有 %d 个文件未显示", totalItems-maxDisplay))
		lines = append(lines, "")
	}

	// 统计信息
	lines = append(lines, mf.FormatSection("统计"))
	lines = append(lines, mf.FormatListItem("✅", fmt.Sprintf("成功: %d", data.SuccessCount)))
	if data.SkippedCount > 0 {
		lines = append(lines, mf.FormatListItem("⏭️", fmt.Sprintf("已标准化: %d", data.SkippedCount)))
	}
	if data.ConflictCount > 0 {
		lines = append(lines, mf.FormatListItem("🔀", fmt.Sprintf("冲突跳过: %d", data.ConflictCount)))
	}
	if data.UnprocessableCount > 0 {
		lines = append(lines, mf.FormatListItem("⚠️", fmt.Sprintf("无法处理: %d", data.UnprocessableCount)))
	}
	if data.FailedCount > 0 {
		lines = append(lines, mf.FormatListItem("❌", fmt.Sprintf("失败: %d", data.FailedCount)))
	}
	lines = append(lines, mf.FormatListItem("📊", fmt.Sprintf("总计: %d", data.TotalCount)))

	return strings.Join(lines, "\n")
}

// FormatRenamePreview 格式化重命名预览
type RenamePreviewData struct {
	UseLLM             bool
	TotalCount         int
	SuccessCount       int // 需要重命名
	SkippedCount       int // 已标准化
	UnprocessableCount int // 无法处理
	ConflictCount      int // 冲突
	PreviewItems       []RenamePreviewItem
	MaxDisplayItems    int
}

// RenamePreviewItem 单个预览项
type RenamePreviewItem struct {
	Index      int
	OldPath    string
	NewPath    string
	Status     string // "rename", "skip", "conflict", "error"
	SkipReason string
}

// FormatRenamePreview 格式化重命名预览
func (mf *MessageFormatter) FormatRenamePreview(data RenamePreviewData) string {
	var lines []string

	// 标题
	lines = append(lines, mf.FormatTitle("📝", "批量重命名预览"))
	lines = append(lines, "")

	// 重命名方式
	if data.UseLLM {
		lines = append(lines, "🤖 使用LLM智能重命名")
	} else {
		lines = append(lines, "🎬 使用TMDB重命名")
	}
	lines = append(lines, "")

	// 统计概要
	statsLine := fmt.Sprintf("✅ 需重命名: %d", data.SuccessCount)
	if data.SkippedCount > 0 {
		statsLine += fmt.Sprintf(" | ⏭️ 已标准化: %d", data.SkippedCount)
	}
	if data.UnprocessableCount > 0 {
		statsLine += fmt.Sprintf(" | ⚠️ 无法处理: %d", data.UnprocessableCount)
	}
	if data.ConflictCount > 0 {
		statsLine += fmt.Sprintf(" | 🔀 冲突: %d", data.ConflictCount)
	}
	statsLine += fmt.Sprintf(" | 📊 总计: %d", data.TotalCount)
	lines = append(lines, statsLine)
	lines = append(lines, "")

	// 预览列表
	maxDisplay := data.MaxDisplayItems
	if maxDisplay <= 0 {
		maxDisplay = 10
	}

	for i, item := range data.PreviewItems {
		if i >= maxDisplay {
			break
		}
		switch item.Status {
		case "rename":
			oldName := mf.wrapLongText(item.OldPath, mf.maxWidth-5)
			newName := mf.wrapLongText(item.NewPath, mf.maxWidth-5)
			lines = append(lines, fmt.Sprintf("%d. <code>%s</code>", item.Index, oldName))
			lines = append(lines, fmt.Sprintf("   → <code>%s</code>", newName))
		case "conflict":
			oldName := mf.wrapLongText(item.OldPath, mf.maxWidth-5)
			lines = append(lines, fmt.Sprintf("%d. ⚠️ <code>%s</code>", item.Index, oldName))
			lines = append(lines, "   目标文件冲突，将跳过")
		case "error":
			oldName := mf.wrapLongText(item.OldPath, mf.maxWidth-5)
			lines = append(lines, fmt.Sprintf("%d. ⚠️ <code>%s</code>", item.Index, oldName))
			if item.SkipReason != "" {
				lines = append(lines, fmt.Sprintf("   %s", item.SkipReason))
			}
		}
		lines = append(lines, "")
	}

	// 省略提示
	if len(data.PreviewItems) > maxDisplay {
		lines = append(lines, fmt.Sprintf("... 还有 %d 个文件未显示", len(data.PreviewItems)-maxDisplay))
		lines = append(lines, "")
	}

	lines = append(lines, "是否确认批量重命名？")

	return strings.Join(lines, "\n")
}
