package file

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/easayliu/alist-aria2-download/internal/domain/entities"
	"github.com/easayliu/alist-aria2-download/internal/domain/valueobjects"
)

// FileClassifier 文件分类器 - 领域服务
// 负责根据文件名、路径、元数据等判断文件类型(电影/电视剧/综艺)
type FileClassifier struct{}

// NewFileClassifier 创建文件分类器
func NewFileClassifier() *FileClassifier {
	return &FileClassifier{}
}

// Classify 分类文件
func (c *FileClassifier) Classify(file *entities.File) valueobjects.MediaType {
	if file.IsDir {
		return valueobjects.MediaTypeOther
	}

	if !file.IsVideo() {
		return valueobjects.MediaTypeOther
	}

	// 1. 优先使用已有的MediaType
	if file.MediaType != valueobjects.MediaTypeUnknown && file.MediaType != "" {
		return file.MediaType
	}

	// 2. 从路径分析
	pathType := c.classifyByPath(file.Path)
	if pathType != valueobjects.MediaTypeUnknown {
		return pathType
	}

	// 3. 从文件名分析
	nameType := c.classifyByName(file.Name)
	if nameType != valueobjects.MediaTypeUnknown {
		return nameType
	}

	// 4. 默认为其他
	return valueobjects.MediaTypeOther
}

// classifyByPath 根据路径分类
func (c *FileClassifier) classifyByPath(path string) valueobjects.MediaType {
	lowerPath := strings.ToLower(path)

	// 电影路径特征
	if strings.Contains(lowerPath, "/movies/") ||
	   strings.Contains(lowerPath, "/电影/") ||
	   strings.Contains(lowerPath, "/films/") {
		return valueobjects.MediaTypeMovie
	}

	// 电视剧路径特征
	if strings.Contains(lowerPath, "/tvs/") ||
	   strings.Contains(lowerPath, "/tv shows/") ||
	   strings.Contains(lowerPath, "/剧集/") ||
	   strings.Contains(lowerPath, "/电视剧/") ||
	   strings.Contains(lowerPath, "/series/") {
		return valueobjects.MediaTypeTV
	}

	// 综艺路径特征
	if strings.Contains(lowerPath, "/variety/") ||
	   strings.Contains(lowerPath, "/综艺/") ||
	   strings.Contains(lowerPath, "/shows/") {
		return valueobjects.MediaTypeVariety
	}

	return valueobjects.MediaTypeUnknown
}

// classifyByName 根据文件名分类
func (c *FileClassifier) classifyByName(name string) valueobjects.MediaType {
	lowerName := strings.ToLower(name)

	// 电视剧特征:包含季集信息
	if c.hasSeasonEpisodeInfo(lowerName) {
		return valueobjects.MediaTypeTV
	}

	// 综艺特征:包含期数信息
	if c.hasIssueInfo(lowerName) {
		return valueobjects.MediaTypeVariety
	}

	// 电影特征:包含年份但不包含季集信息
	if c.hasYearInfo(lowerName) && !c.hasSeasonEpisodeInfo(lowerName) {
		return valueobjects.MediaTypeMovie
	}

	return valueobjects.MediaTypeUnknown
}

// hasSeasonEpisodeInfo 检查是否包含季集信息
func (c *FileClassifier) hasSeasonEpisodeInfo(name string) bool {
	// S01E01, s01e01 格式
	sePattern := regexp.MustCompile(`[Ss]\d+[Ee]\d+`)
	if sePattern.MatchString(name) {
		return true
	}

	// 第1季第2集 格式
	cnPattern := regexp.MustCompile(`第\d+季.*第\d+集`)
	if cnPattern.MatchString(name) {
		return true
	}

	// EP01, ep01 格式
	epPattern := regexp.MustCompile(`[Ee][Pp]\s*\d+`)
	if epPattern.MatchString(name) {
		return true
	}

	return false
}

// hasIssueInfo 检查是否包含期数信息
func (c *FileClassifier) hasIssueInfo(name string) bool {
	// 第1期, 第01期 格式
	issuePattern := regexp.MustCompile(`第\d+期`)
	if issuePattern.MatchString(name) {
		return true
	}

	// Issue 01 格式
	issueEnPattern := regexp.MustCompile(`[Ii]ssue\s*\d+`)
	if issueEnPattern.MatchString(name) {
		return true
	}

	return false
}

// hasYearInfo 检查是否包含年份信息
func (c *FileClassifier) hasYearInfo(name string) bool {
	yearPattern := regexp.MustCompile(`[\(\[]?(19\d{2}|20\d{2})[\)\]]?`)
	return yearPattern.MatchString(name)
}

// ClassifyBatch 批量分类
func (c *FileClassifier) ClassifyBatch(files []*entities.File) map[valueobjects.MediaType][]*entities.File {
	result := make(map[valueobjects.MediaType][]*entities.File)

	for _, file := range files {
		mediaType := c.Classify(file)
		result[mediaType] = append(result[mediaType], file)
	}

	return result
}

// GetMovies 获取所有电影文件
func (c *FileClassifier) GetMovies(files []*entities.File) []*entities.File {
	var movies []*entities.File
	for _, file := range files {
		if c.Classify(file) == valueobjects.MediaTypeMovie {
			movies = append(movies, file)
		}
	}
	return movies
}

// GetTVShows 获取所有电视剧文件
func (c *FileClassifier) GetTVShows(files []*entities.File) []*entities.File {
	var tvShows []*entities.File
	for _, file := range files {
		if c.Classify(file) == valueobjects.MediaTypeTV {
			tvShows = append(tvShows, file)
		}
	}
	return tvShows
}

// GetVarietyShows 获取所有综艺文件
func (c *FileClassifier) GetVarietyShows(files []*entities.File) []*entities.File {
	var variety []*entities.File
	for _, file := range files {
		if c.Classify(file) == valueobjects.MediaTypeVariety {
			variety = append(variety, file)
		}
	}
	return variety
}

// IsMovie 判断是否为电影
func (c *FileClassifier) IsMovie(file *entities.File) bool {
	return c.Classify(file) == valueobjects.MediaTypeMovie
}

// IsTVShow 判断是否为电视剧
func (c *FileClassifier) IsTVShow(file *entities.File) bool {
	return c.Classify(file) == valueobjects.MediaTypeTV
}

// IsVarietyShow 判断是否为综艺
func (c *FileClassifier) IsVarietyShow(file *entities.File) bool {
	return c.Classify(file) == valueobjects.MediaTypeVariety
}

// GetFileExtension 获取文件扩展名(小写)
func (c *FileClassifier) GetFileExtension(filename string) string {
	return strings.ToLower(filepath.Ext(filename))
}

// GetBasename 获取不含扩展名的文件名
func (c *FileClassifier) GetBasename(filename string) string {
	ext := filepath.Ext(filename)
	return strings.TrimSuffix(filename, ext)
}
