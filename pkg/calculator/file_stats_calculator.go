package calculator

import (
	"github.com/easayliu/alist-aria2-download/internal/application/services"
	strutil "github.com/easayliu/alist-aria2-download/pkg/utils/string"
	"github.com/gin-gonic/gin"
)

// FileStatsCalculator 文件统计计算器 - 统一文件统计逻辑
type FileStatsCalculator struct{}

// NewFileStatsCalculator 创建文件统计计算器
func NewFileStatsCalculator() *FileStatsCalculator {
	return &FileStatsCalculator{}
}

// FileStats 文件统计结果
type FileStats struct {
	TotalCount int
	TotalSize  int64
	TVCount    int
	MovieCount int
	OtherCount int
}

// CalculateFromFileInfo 从FileInfo计算统计信息
func (c *FileStatsCalculator) CalculateFromFileInfo(files []services.FileInfo) *FileStats {
	stats := &FileStats{
		TotalCount: len(files),
	}

	for _, file := range files {
		stats.TotalSize += file.Size

		switch file.MediaType {
		case "tv":
			stats.TVCount++
		case "movie":
			stats.MovieCount++
		default:
			stats.OtherCount++
		}
	}

	return stats
}

// CalculateFromYesterdayFileInfo 从YesterdayFileInfo计算统计信息
func (c *FileStatsCalculator) CalculateFromYesterdayFileInfo(files []services.YesterdayFileInfo) *FileStats {
	stats := &FileStats{
		TotalCount: len(files),
	}

	for _, file := range files {
		stats.TotalSize += file.Size

		switch file.MediaType {
		case "tv":
			stats.TVCount++
		case "movie":
			stats.MovieCount++
		default:
			stats.OtherCount++
		}
	}

	return stats
}

// BuildMediaStats 构建媒体统计信息(gin.H格式)
func (stats *FileStats) BuildMediaStats() gin.H {
	return strutil.BuildMediaStats(stats.TVCount, stats.MovieCount, stats.OtherCount)
}

// VideoCount 获取视频文件总数
func (stats *FileStats) VideoCount() int {
	return stats.TVCount + stats.MovieCount
}
