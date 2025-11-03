package file

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/easayliu/alist-aria2-download/internal/domain/entities"
	"github.com/easayliu/alist-aria2-download/internal/domain/valueobjects"
)

// FileFilter 文件过滤器 - 领域服务
// 负责根据各种条件过滤文件列表
type FileFilter struct{}

// NewFileFilter 创建文件过滤器
func NewFileFilter() *FileFilter {
	return &FileFilter{}
}

// FilterCriteria 过滤条件
type FilterCriteria struct {
	VideoOnly       bool                    // 仅视频文件
	MediaType       valueobjects.MediaType  // 媒体类型
	MinSize         valueobjects.FileSize   // 最小文件大小
	MaxSize         valueobjects.FileSize   // 最大文件大小
	Extensions      []string                // 允许的扩展名
	ExcludePatterns []string                // 排除的文件名模式
	TimeRange       *valueobjects.TimeRange // 修改时间范围
	ExcludeDirs     bool                    // 排除目录
}

// Filter 根据条件过滤文件
func (f *FileFilter) Filter(files []*entities.File, criteria FilterCriteria) []*entities.File {
	var filtered []*entities.File

	for _, file := range files {
		if f.matchesCriteria(file, criteria) {
			filtered = append(filtered, file)
		}
	}

	return filtered
}

// matchesCriteria 检查文件是否匹配过滤条件
func (f *FileFilter) matchesCriteria(file *entities.File, criteria FilterCriteria) bool {
	// 排除目录
	if criteria.ExcludeDirs && file.IsDir {
		return false
	}

	// 仅视频
	if criteria.VideoOnly && !file.IsVideo() {
		return false
	}

	// 媒体类型
	if criteria.MediaType != "" && criteria.MediaType != valueobjects.MediaTypeUnknown {
		if file.MediaType != criteria.MediaType {
			return false
		}
	}

	// 最小大小
	if !criteria.MinSize.IsZero() {
		if !file.Size.IsLargerThan(criteria.MinSize) && file.Size != criteria.MinSize {
			return false
		}
	}

	// 最大大小
	if !criteria.MaxSize.IsZero() {
		if file.Size.IsLargerThan(criteria.MaxSize) {
			return false
		}
	}

	// 扩展名
	if len(criteria.Extensions) > 0 {
		if !f.hasAllowedExtension(file.Name, criteria.Extensions) {
			return false
		}
	}

	// 排除模式
	if len(criteria.ExcludePatterns) > 0 {
		if f.matchesExcludePattern(file.Name, criteria.ExcludePatterns) {
			return false
		}
	}

	// 时间范围
	if criteria.TimeRange != nil {
		if !file.IsModifiedInTimeRange(*criteria.TimeRange) {
			return false
		}
	}

	return true
}

// hasAllowedExtension 检查是否有允许的扩展名
func (f *FileFilter) hasAllowedExtension(filename string, allowedExts []string) bool {
	ext := strings.ToLower(filepath.Ext(filename))

	for _, allowedExt := range allowedExts {
		if strings.ToLower(allowedExt) == ext {
			return true
		}
	}

	return false
}

// matchesExcludePattern 检查是否匹配排除模式
func (f *FileFilter) matchesExcludePattern(filename string, patterns []string) bool {
	lowerName := strings.ToLower(filename)

	for _, pattern := range patterns {
		lowerPattern := strings.ToLower(pattern)

		// 简单的通配符匹配
		if strings.Contains(lowerPattern, "*") {
			matched, _ := filepath.Match(lowerPattern, lowerName)
			if matched {
				return true
			}
		} else {
			// 普通字符串包含匹配
			if strings.Contains(lowerName, lowerPattern) {
				return true
			}
		}
	}

	return false
}

// FilterVideoOnly 只保留视频文件
func (f *FileFilter) FilterVideoOnly(files []*entities.File) []*entities.File {
	return f.Filter(files, FilterCriteria{VideoOnly: true, ExcludeDirs: true})
}

// FilterByMediaType 根据媒体类型过滤
func (f *FileFilter) FilterByMediaType(files []*entities.File, mediaType valueobjects.MediaType) []*entities.File {
	return f.Filter(files, FilterCriteria{MediaType: mediaType, ExcludeDirs: true})
}

// FilterByMinSize 根据最小大小过滤
func (f *FileFilter) FilterByMinSize(files []*entities.File, minSize valueobjects.FileSize) []*entities.File {
	return f.Filter(files, FilterCriteria{MinSize: minSize, ExcludeDirs: true})
}

// FilterBySizeRange 根据大小范围过滤
func (f *FileFilter) FilterBySizeRange(files []*entities.File, minSize, maxSize valueobjects.FileSize) []*entities.File {
	return f.Filter(files, FilterCriteria{
		MinSize:     minSize,
		MaxSize:     maxSize,
		ExcludeDirs: true,
	})
}

// FilterByTimeRange 根据时间范围过滤
func (f *FileFilter) FilterByTimeRange(files []*entities.File, timeRange valueobjects.TimeRange) []*entities.File {
	return f.Filter(files, FilterCriteria{
		TimeRange:   &timeRange,
		ExcludeDirs: true,
	})
}

// FilterModifiedAfter 过滤指定时间之后修改的文件
func (f *FileFilter) FilterModifiedAfter(files []*entities.File, after time.Time) []*entities.File {
	var filtered []*entities.File
	for _, file := range files {
		if file.IsModifiedAfter(after) {
			filtered = append(filtered, file)
		}
	}
	return filtered
}

// FilterModifiedBefore 过滤指定时间之前修改的文件
func (f *FileFilter) FilterModifiedBefore(files []*entities.File, before time.Time) []*entities.File {
	var filtered []*entities.File
	for _, file := range files {
		if file.IsModifiedBefore(before) {
			filtered = append(filtered, file)
		}
	}
	return filtered
}

// FilterByExtensions 根据扩展名过滤
func (f *FileFilter) FilterByExtensions(files []*entities.File, extensions []string) []*entities.File {
	return f.Filter(files, FilterCriteria{
		Extensions:  extensions,
		ExcludeDirs: true,
	})
}

// ExcludeByPatterns 排除匹配模式的文件
func (f *FileFilter) ExcludeByPatterns(files []*entities.File, patterns []string) []*entities.File {
	return f.Filter(files, FilterCriteria{
		ExcludePatterns: patterns,
		ExcludeDirs:     true,
	})
}

// ExcludeSamples 排除样片文件
func (f *FileFilter) ExcludeSamples(files []*entities.File) []*entities.File {
	samplePatterns := []string{"sample", "样片", "预览", "trailer"}
	return f.ExcludeByPatterns(files, samplePatterns)
}

// ExcludeSubtitles 排除字幕文件
func (f *FileFilter) ExcludeSubtitles(files []*entities.File) []*entities.File {
	subtitleExts := []string{".srt", ".ass", ".ssa", ".sub", ".vtt", ".sup"}
	var filtered []*entities.File

	for _, file := range files {
		ext := strings.ToLower(filepath.Ext(file.Name))
		isSubtitle := false

		for _, subtitleExt := range subtitleExts {
			if ext == subtitleExt {
				isSubtitle = true
				break
			}
		}

		if !isSubtitle {
			filtered = append(filtered, file)
		}
	}

	return filtered
}

// GetDownloadableFiles 获取可下载的文件(领域规则集成)
func (f *FileFilter) GetDownloadableFiles(files []*entities.File, videoOnly bool, minSize valueobjects.FileSize) []*entities.File {
	var downloadable []*entities.File

	for _, file := range files {
		if file.ShouldDownload(videoOnly, minSize) {
			downloadable = append(downloadable, file)
		}
	}

	return downloadable
}

// SortBySize 按大小排序(降序)
func (f *FileFilter) SortBySize(files []*entities.File, ascending bool) []*entities.File {
	sorted := make([]*entities.File, len(files))
	copy(sorted, files)

	// 简单的冒泡排序
	for i := 0; i < len(sorted)-1; i++ {
		for j := 0; j < len(sorted)-i-1; j++ {
			shouldSwap := false
			if ascending {
				shouldSwap = sorted[j].Size.IsLargerThan(sorted[j+1].Size)
			} else {
				shouldSwap = sorted[j+1].Size.IsLargerThan(sorted[j].Size)
			}

			if shouldSwap {
				sorted[j], sorted[j+1] = sorted[j+1], sorted[j]
			}
		}
	}

	return sorted
}

// SortByModified 按修改时间排序
func (f *FileFilter) SortByModified(files []*entities.File, ascending bool) []*entities.File {
	sorted := make([]*entities.File, len(files))
	copy(sorted, files)

	for i := 0; i < len(sorted)-1; i++ {
		for j := 0; j < len(sorted)-i-1; j++ {
			shouldSwap := false
			if ascending {
				shouldSwap = sorted[j].Modified.After(sorted[j+1].Modified)
			} else {
				shouldSwap = sorted[j+1].Modified.After(sorted[j].Modified)
			}

			if shouldSwap {
				sorted[j], sorted[j+1] = sorted[j+1], sorted[j]
			}
		}
	}

	return sorted
}
