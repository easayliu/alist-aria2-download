package formatter

import (
	fileservices "github.com/easayliu/alist-aria2-download/internal/application/services/file"
	"github.com/gin-gonic/gin"
)

// PreviewFormatter 预览格式化器 - 统一预览数据格式
type PreviewFormatter struct{}

// NewPreviewFormatter 创建预览格式化器
func NewPreviewFormatter() *PreviewFormatter {
	return &PreviewFormatter{}
}

// PreviewResult 预览结果数据结构
type PreviewResult struct {
	File         string `json:"file"`
	SourcePath   string `json:"source_path"`
	Size         int64  `json:"size"`
	MediaType    string `json:"media_type"`
	DownloadPath string `json:"download_path"`
	DownloadFile string `json:"download_file"`
	InternalURL  string `json:"internal_url"`
	Modified     string `json:"modified,omitempty"`
}

// FormatPreviewResultsFromFileInfo 从FileInfo格式化预览结果列表
func (f *PreviewFormatter) FormatPreviewResultsFromFileInfo(files []fileservices.FileInfo) []PreviewResult {
	results := make([]PreviewResult, 0, len(files))
	for _, file := range files {
		results = append(results, PreviewResult{
			File:         file.Name,
			SourcePath:   file.Path,
			Size:         file.Size,
			MediaType:    string(file.MediaType),
			DownloadPath: file.DownloadPath,
			DownloadFile: file.DownloadPath + "/" + file.Name,
			InternalURL:  file.InternalURL,
			Modified:     file.Modified.Format("2006-01-02 15:04:05"),
		})
	}
	return results
}

// FormatPreviewResultsFromYesterdayFileInfo 从YesterdayFileInfo格式化预览结果列表
func (f *PreviewFormatter) FormatPreviewResultsFromYesterdayFileInfo(files []fileservices.YesterdayFileInfo) []PreviewResult {
	results := make([]PreviewResult, 0, len(files))
	for _, file := range files {
		results = append(results, PreviewResult{
			File:         file.Name,
			SourcePath:   file.Path,
			Size:         file.Size,
			MediaType:    string(file.MediaType),
			DownloadPath: file.DownloadPath,
			DownloadFile: file.DownloadPath + "/" + file.Name,
			InternalURL:  file.InternalURL,
			Modified:     file.Modified.Format("2006-01-02 15:04:05"),
		})
	}
	return results
}

// BuildDirectoryPreviewResponse 构建目录预览响应
func (f *PreviewFormatter) BuildDirectoryPreviewResponse(
	path string,
	files []fileservices.FileInfo,
	recursive bool,
	mediaStats gin.H,
) gin.H {
	return gin.H{
		"message":     "Preview mode - no downloads initiated",
		"mode":        "preview",
		"source_path": path,
		"recursive":   recursive,
		"total":       len(files),
		"media_stats": mediaStats,
		"files":       f.FormatPreviewResultsFromFileInfo(files),
	}
}

// BuildYesterdayPreviewResponse 构建昨日文件预览响应
func (f *PreviewFormatter) BuildYesterdayPreviewResponse(
	path string,
	files []fileservices.YesterdayFileInfo,
	mediaStats gin.H,
) gin.H {
	return gin.H{
		"message":     "Preview mode - no downloads initiated",
		"mode":        "preview",
		"search_path": path,
		"date":        "yesterday",
		"total":       len(files),
		"media_stats": mediaStats,
		"files":       f.FormatPreviewResultsFromYesterdayFileInfo(files),
	}
}

// BuildTimeRangePreviewResponse 构建时间范围预览响应
func (f *PreviewFormatter) BuildTimeRangePreviewResponse(
	path string,
	files []fileservices.YesterdayFileInfo,
	startTime, endTime string,
	mediaStats gin.H,
) gin.H {
	return gin.H{
		"message":     "Preview mode - no downloads initiated",
		"mode":        "preview",
		"path":        path,
		"start_time":  startTime,
		"end_time":    endTime,
		"date":        "custom_range",
		"total":       len(files),
		"media_stats": mediaStats,
		"files":       f.FormatPreviewResultsFromYesterdayFileInfo(files),
	}
}
