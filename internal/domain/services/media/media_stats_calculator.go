package media

import (
	"github.com/easayliu/alist-aria2-download/internal/domain/entities"
	"github.com/easayliu/alist-aria2-download/internal/domain/valueobjects"
)

// MediaStatsCalculator 媒体统计计算器 - 领域服务
// 负责计算文件集合的统计信息(视频数量、电影数量、总大小等)
type MediaStatsCalculator struct{}

// NewMediaStatsCalculator 创建媒体统计计算器
func NewMediaStatsCalculator() *MediaStatsCalculator {
	return &MediaStatsCalculator{}
}

// MediaStats 媒体统计结果
type MediaStats struct {
	TotalFiles  int                      `json:"total_files"`
	TotalSize   valueobjects.FileSize    `json:"total_size"`
	VideoFiles  int                      `json:"video_files"`
	MovieFiles  int                      `json:"movie_files"`
	TVFiles     int                      `json:"tv_files"`
	VarietyFiles int                     `json:"variety_files"`
	OtherFiles  int                      `json:"other_files"`
}

// Calculate 计算文件统计信息
func (c *MediaStatsCalculator) Calculate(files []*entities.File) MediaStats {
	stats := MediaStats{
		TotalFiles: len(files),
	}

	for _, file := range files {
		if file.IsDir {
			continue
		}

		// 累计总大小
		stats.TotalSize = stats.TotalSize.Add(file.Size)

		// 按类型统计
		if file.IsVideo() {
			stats.VideoFiles++

			switch file.MediaType {
			case valueobjects.MediaTypeMovie:
				stats.MovieFiles++
			case valueobjects.MediaTypeTV:
				stats.TVFiles++
			case valueobjects.MediaTypeVariety:
				stats.VarietyFiles++
			}
		} else {
			stats.OtherFiles++
		}
	}

	return stats
}

// CalculateVideoOnly 只计算视频文件统计
func (c *MediaStatsCalculator) CalculateVideoOnly(files []*entities.File) MediaStats {
	var videoFiles []*entities.File
	for _, file := range files {
		if file.IsVideo() {
			videoFiles = append(videoFiles, file)
		}
	}
	return c.Calculate(videoFiles)
}

// GetCategoryDistribution 获取分类分布
func (c *MediaStatsCalculator) GetCategoryDistribution(files []*entities.File) map[string]int {
	distribution := make(map[string]int)

	for _, file := range files {
		if file.IsDir {
			continue
		}
		category := file.GetCategory()
		distribution[category]++
	}

	return distribution
}

// GetSizeDistribution 获取大小分布(按MB范围)
func (c *MediaStatsCalculator) GetSizeDistribution(files []*entities.File) map[string]int {
	distribution := map[string]int{
		"< 10MB":      0,
		"10-100MB":    0,
		"100MB-1GB":   0,
		"1-5GB":       0,
		"5-10GB":      0,
		"> 10GB":      0,
	}

	for _, file := range files {
		if file.IsDir {
			continue
		}

		bytes := file.Size.Bytes()
		mb := bytes / (1024 * 1024)
		gb := mb / 1024

		switch {
		case mb < 10:
			distribution["< 10MB"]++
		case mb < 100:
			distribution["10-100MB"]++
		case mb < 1024:
			distribution["100MB-1GB"]++
		case gb < 5:
			distribution["1-5GB"]++
		case gb < 10:
			distribution["5-10GB"]++
		default:
			distribution["> 10GB"]++
		}
	}

	return distribution
}

// GetAverageFileSize 获取平均文件大小
func (c *MediaStatsCalculator) GetAverageFileSize(files []*entities.File) valueobjects.FileSize {
	if len(files) == 0 {
		return valueobjects.FileSize(0)
	}

	stats := c.Calculate(files)
	return valueobjects.FileSize(stats.TotalSize.Bytes() / int64(stats.TotalFiles))
}

// GetLargestFile 获取最大的文件
func (c *MediaStatsCalculator) GetLargestFile(files []*entities.File) *entities.File {
	var largest *entities.File
	var maxSize valueobjects.FileSize

	for _, file := range files {
		if file.IsDir {
			continue
		}
		if file.Size.IsLargerThan(maxSize) {
			maxSize = file.Size
			largest = file
		}
	}

	return largest
}
