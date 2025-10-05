package entities

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/easayliu/alist-aria2-download/internal/domain/valueobjects"
)

// Download 下载任务实体
type Download struct {
	ID            string                        `json:"id"`
	URL           string                        `json:"url"`
	Filename      string                        `json:"filename"`
	Status        valueobjects.DownloadStatus   `json:"status"` // 使用值对象
	Progress      float64                       `json:"progress"`
	Speed         int64                         `json:"speed"`
	TotalSize     int64                         `json:"total_size"`
	CompletedSize int64                         `json:"completed_size"`
	ErrorMessage  string                        `json:"error_message,omitempty"`
	CreatedAt     time.Time                     `json:"created_at"`
	UpdatedAt     time.Time                     `json:"updated_at"`
}

// File Alist文件信息实体 - 领域层核心实体
type File struct {
	Name      string                     `json:"name"`
	Size      valueobjects.FileSize      `json:"size"`       // 使用值对象
	IsDir     bool                       `json:"is_dir"`
	Modified  time.Time                  `json:"modified"`
	Path      string                     `json:"path"`       // 暂时保持string,避免breaking change
	URL       string                     `json:"url,omitempty"`
	MediaType valueobjects.MediaType     `json:"media_type,omitempty"` // 新增:媒体类型
}

// IsVideo 判断是否为视频文件(领域方法)
func (f *File) IsVideo() bool {
	if f.IsDir {
		return false
	}

	// 通过扩展名判断
	ext := strings.ToLower(filepath.Ext(f.Name))
	videoExts := []string{".mp4", ".mkv", ".avi", ".mov", ".wmv", ".flv", ".webm", ".m4v", ".ts", ".m2ts"}

	for _, ve := range videoExts {
		if ext == ve {
			return true
		}
	}

	// 也可以通过MediaType判断
	return f.MediaType.IsVideo()
}

// GetCategory 获取文件分类(领域方法)
func (f *File) GetCategory() string {
	if f.IsDir {
		return "directory"
	}

	if f.IsVideo() {
		return f.MediaType.String()
	}

	return "other"
}

// GenerateDownloadPath 生成下载路径(领域方法)
func (f *File) GenerateDownloadPath(baseDir string) string {
	if f.IsDir {
		return baseDir + "/others"
	}

	category := f.GetCategory()
	switch category {
	case "movie":
		return baseDir + "/movies"
	case "tv":
		return baseDir + "/tvs"
	case "variety":
		return baseDir + "/variety"
	default:
		return baseDir + "/others"
	}
}

// ShouldDownload 判断是否应该下载(领域规则)
func (f *File) ShouldDownload(videoOnly bool, minSize valueobjects.FileSize) bool {
	// 目录不下载
	if f.IsDir {
		return false
	}

	// 如果启用仅视频模式,只下载视频文件
	if videoOnly && !f.IsVideo() {
		return false
	}

	// 文件大小过小不下载
	if !f.Size.IsZero() {
		if !f.Size.IsLargerThan(minSize) {
			return false
		}
	}

	return true
}

// GetFormattedSize 获取格式化的文件大小
func (f *File) GetFormattedSize() string {
	return f.Size.Format()
}

// IsModifiedAfter 判断文件是否在指定时间之后修改
func (f *File) IsModifiedAfter(t time.Time) bool {
	return f.Modified.After(t)
}

// IsModifiedBefore 判断文件是否在指定时间之前修改
func (f *File) IsModifiedBefore(t time.Time) bool {
	return f.Modified.Before(t)
}

// IsModifiedInTimeRange 判断文件是否在时间范围内修改
func (f *File) IsModifiedInTimeRange(tr valueobjects.TimeRange) bool {
	return tr.Contains(f.Modified)
}

// GetExtension 获取文件扩展名(小写)
func (f *File) GetExtension() string {
	return strings.ToLower(filepath.Ext(f.Name))
}

// MatchesPattern 判断文件名是否匹配指定模式
func (f *File) MatchesPattern(pattern string) bool {
	matched, _ := filepath.Match(pattern, f.Name)
	return matched
}
