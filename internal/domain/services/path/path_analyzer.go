package path

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/easayliu/alist-aria2-download/internal/domain/valueobjects"
)

// PathAnalyzer 路径分析器 - 领域服务
// 负责分析路径结构、提取路径信息、判断路径类型等
type PathAnalyzer struct{}

// NewPathAnalyzer 创建路径分析器
func NewPathAnalyzer() *PathAnalyzer {
	return &PathAnalyzer{}
}

// PathInfo 路径信息
type PathInfo struct {
	Path       valueobjects.FilePath
	Dir        valueobjects.FilePath
	Filename   string
	Extension  string
	IsAbsolute bool
	Depth      int      // 路径深度
	Components []string // 路径组件
	MediaType  valueobjects.MediaType
}

// Analyze 分析路径
func (a *PathAnalyzer) Analyze(path string) (*PathInfo, error) {
	fp, err := valueobjects.NewFilePath(path)
	if err != nil {
		return nil, err
	}

	components := strings.Split(filepath.Clean(path), string(filepath.Separator))

	info := &PathInfo{
		Path:       fp,
		Dir:        fp.Dir(),
		Filename:   fp.Base(),
		Extension:  strings.ToLower(fp.Ext()),
		IsAbsolute: fp.IsAbsolute(),
		Depth:      len(components),
		Components: components,
		MediaType:  a.detectMediaTypeFromPath(path),
	}

	return info, nil
}

// detectMediaTypeFromPath 从路径检测媒体类型
func (a *PathAnalyzer) detectMediaTypeFromPath(path string) valueobjects.MediaType {
	lowerPath := strings.ToLower(path)

	// 路径包含特定关键词
	if strings.Contains(lowerPath, "/movies/") || strings.Contains(lowerPath, "/电影/") {
		return valueobjects.MediaTypeMovie
	}
	if strings.Contains(lowerPath, "/tvs/") || strings.Contains(lowerPath, "/tv shows/") ||
		strings.Contains(lowerPath, "/剧集/") || strings.Contains(lowerPath, "/电视剧/") {
		return valueobjects.MediaTypeTV
	}
	if strings.Contains(lowerPath, "/variety/") || strings.Contains(lowerPath, "/综艺/") {
		return valueobjects.MediaTypeVariety
	}

	return valueobjects.MediaTypeUnknown
}

// ExtractSeasonAndEpisode 从路径提取季和集信息
func (a *PathAnalyzer) ExtractSeasonAndEpisode(path string) (season, episode int, found bool) {
	// 匹配 S01E01, s01e01 等格式
	seRe := regexp.MustCompile(`[Ss](\d+)[Ee](\d+)`)
	matches := seRe.FindStringSubmatch(path)
	if len(matches) == 3 {
		s := parseInt(matches[1])
		e := parseInt(matches[2])
		return s, e, true
	}

	// 匹配中文格式: 第1季第2集
	cnRe := regexp.MustCompile(`第(\d+)季.*第(\d+)集`)
	matches = cnRe.FindStringSubmatch(path)
	if len(matches) == 3 {
		s := parseInt(matches[1])
		e := parseInt(matches[2])
		return s, e, true
	}

	return 0, 0, false
}

// parseInt 解析整数
func parseInt(s string) int {
	var result int
	for _, c := range s {
		if c >= '0' && c <= '9' {
			result = result*10 + int(c-'0')
		}
	}
	return result
}

// ExtractYear 从路径提取年份
func (a *PathAnalyzer) ExtractYear(path string) (year int, found bool) {
	// 匹配 (2024), [2024], 2024 等格式
	yearRe := regexp.MustCompile(`[\(\[]?(19\d{2}|20\d{2})[\)\]]?`)
	matches := yearRe.FindStringSubmatch(path)
	if len(matches) >= 2 {
		return parseInt(matches[1]), true
	}
	return 0, false
}

// IsSample 判断是否为样片
func (a *PathAnalyzer) IsSample(path string) bool {
	lowerPath := strings.ToLower(path)
	sampleKeywords := []string{"sample", "样片", "预览"}

	for _, keyword := range sampleKeywords {
		if strings.Contains(lowerPath, keyword) {
			return true
		}
	}
	return false
}

// IsSubtitle 判断是否为字幕文件
func (a *PathAnalyzer) IsSubtitle(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	subtitleExts := []string{".srt", ".ass", ".ssa", ".sub", ".vtt", ".sup"}

	for _, subtitleExt := range subtitleExts {
		if ext == subtitleExt {
			return true
		}
	}
	return false
}

// IsExtraContent 判断是否为额外内容(花絮、幕后等)
func (a *PathAnalyzer) IsExtraContent(path string) bool {
	lowerPath := strings.ToLower(path)
	extraKeywords := []string{
		"extras", "特典", "花絮", "幕后", "behind", "making", "featurette",
	}

	for _, keyword := range extraKeywords {
		if strings.Contains(lowerPath, keyword) {
			return true
		}
	}
	return false
}

// GetRelativePath 获取相对路径
func (a *PathAnalyzer) GetRelativePath(fullPath, basePath string) (string, error) {
	return filepath.Rel(basePath, fullPath)
}

// NormalizePath 规范化路径(统一分隔符、去除冗余)
func (a *PathAnalyzer) NormalizePath(path string) valueobjects.FilePath {
	cleaned := filepath.Clean(path)
	// 将所有路径分隔符统一为正斜杠
	normalized := filepath.ToSlash(cleaned)
	return valueobjects.NewFilePathUnchecked(normalized)
}

// GetPathDepth 获取路径深度
func (a *PathAnalyzer) GetPathDepth(path string) int {
	cleaned := filepath.Clean(path)
	if cleaned == "." || cleaned == "/" {
		return 0
	}
	components := strings.Split(cleaned, string(filepath.Separator))
	return len(components)
}

// IsVideoFile 判断是否为视频文件(通过扩展名)
func (a *PathAnalyzer) IsVideoFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	videoExts := []string{
		".mp4", ".mkv", ".avi", ".mov", ".wmv", ".flv",
		".webm", ".m4v", ".ts", ".m2ts", ".rmvb", ".rm",
	}

	for _, videoExt := range videoExts {
		if ext == videoExt {
			return true
		}
	}
	return false
}
