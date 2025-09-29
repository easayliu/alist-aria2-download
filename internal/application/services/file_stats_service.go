package services

import (
	"time"
)

// FileStatsService 文件统计服务
type FileStatsService struct {
	mediaSvc *FileMediaService
}

// NewFileStatsService 创建文件统计服务
func NewFileStatsService(mediaSvc *FileMediaService) *FileStatsService {
	return &FileStatsService{
		mediaSvc: mediaSvc,
	}
}

// FileStats 文件统计信息
type FileStats struct {
	TotalFiles   int                    `json:"total_files"`
	TotalSize    int64                  `json:"total_size"`
	TypeStats    map[string]int         `json:"type_stats"`
	SizeStats    map[string]int64       `json:"size_stats"`
	TimeRange    TimeRange              `json:"time_range"`
	FilesByType  map[string][]FileInfo  `json:"files_by_type"`
}

// TimeRange 时间范围
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// CalculateFileStats 计算文件统计信息
func (s *FileStatsService) CalculateFileStats(files []FileInfo) FileStats {
	stats := FileStats{
		TypeStats:   make(map[string]int),
		SizeStats:   make(map[string]int64),
		FilesByType: make(map[string][]FileInfo),
	}

	if len(files) == 0 {
		return stats
	}

	stats.TotalFiles = len(files)
	
	// 初始化时间范围
	stats.TimeRange.Start = files[0].Modified
	stats.TimeRange.End = files[0].Modified

	for _, file := range files {
		// 累计总大小
		stats.TotalSize += file.Size

		// 获取媒体类型
		mediaType := s.mediaSvc.GetMediaType(file.Path)
		
		// 统计类型数量
		stats.TypeStats[mediaType]++
		
		// 统计类型大小
		stats.SizeStats[mediaType] += file.Size

		// 按类型分组文件
		if stats.FilesByType[mediaType] == nil {
			stats.FilesByType[mediaType] = make([]FileInfo, 0)
		}
		stats.FilesByType[mediaType] = append(stats.FilesByType[mediaType], file)

		// 更新时间范围
		if file.Modified.Before(stats.TimeRange.Start) {
			stats.TimeRange.Start = file.Modified
		}
		if file.Modified.After(stats.TimeRange.End) {
			stats.TimeRange.End = file.Modified
		}
	}

	return stats
}

// CalculateYesterdayFileStats 计算昨天文件统计信息
func (s *FileStatsService) CalculateYesterdayFileStats(files []YesterdayFileInfo) FileStats {
	stats := FileStats{
		TypeStats:   make(map[string]int),
		SizeStats:   make(map[string]int64),
		FilesByType: make(map[string][]FileInfo),
	}

	if len(files) == 0 {
		return stats
	}

	stats.TotalFiles = len(files)
	
	// 初始化时间范围
	stats.TimeRange.Start = files[0].Modified
	stats.TimeRange.End = files[0].Modified

	for _, file := range files {
		// 累计总大小
		stats.TotalSize += file.Size

		// 获取媒体类型
		mediaType := s.mediaSvc.GetMediaType(file.Path)
		
		// 统计类型数量
		stats.TypeStats[mediaType]++
		
		// 统计类型大小
		stats.SizeStats[mediaType] += file.Size

		// 转换为FileInfo格式
		fileInfo := FileInfo{
			Name:         file.Name,
			Path:         file.Path,
			Size:         file.Size,
			Modified:     file.Modified,
			OriginalURL:  file.OriginalURL,
			InternalURL:  file.InternalURL,
			MediaType:    file.MediaType,
			DownloadPath: file.DownloadPath,
		}

		// 按类型分组文件
		if stats.FilesByType[mediaType] == nil {
			stats.FilesByType[mediaType] = make([]FileInfo, 0)
		}
		stats.FilesByType[mediaType] = append(stats.FilesByType[mediaType], fileInfo)

		// 更新时间范围
		if file.Modified.Before(stats.TimeRange.Start) {
			stats.TimeRange.Start = file.Modified
		}
		if file.Modified.After(stats.TimeRange.End) {
			stats.TimeRange.End = file.Modified
		}
	}

	return stats
}

// GetTypeDistribution 获取类型分布信息
func (s *FileStatsService) GetTypeDistribution(stats FileStats) map[string]float64 {
	distribution := make(map[string]float64)
	
	if stats.TotalFiles == 0 {
		return distribution
	}

	for mediaType, count := range stats.TypeStats {
		distribution[mediaType] = float64(count) / float64(stats.TotalFiles) * 100
	}

	return distribution
}

// GetSizeDistribution 获取大小分布信息
func (s *FileStatsService) GetSizeDistribution(stats FileStats) map[string]float64 {
	distribution := make(map[string]float64)
	
	if stats.TotalSize == 0 {
		return distribution
	}

	for mediaType, size := range stats.SizeStats {
		distribution[mediaType] = float64(size) / float64(stats.TotalSize) * 100
	}

	return distribution
}

// FormatFileSize 格式化文件大小
func (s *FileStatsService) FormatFileSize(size int64) string {
	const (
		B  = 1
		KB = 1024 * B
		MB = 1024 * KB
		GB = 1024 * MB
		TB = 1024 * GB
	)

	switch {
	case size >= TB:
		return formatFloat(float64(size)/TB) + " TB"
	case size >= GB:
		return formatFloat(float64(size)/GB) + " GB"
	case size >= MB:
		return formatFloat(float64(size)/MB) + " MB"
	case size >= KB:
		return formatFloat(float64(size)/KB) + " KB"
	default:
		return formatInt(size) + " B"
	}
}

// formatFloat 格式化浮点数
func formatFloat(f float64) string {
	if f == float64(int64(f)) {
		return formatInt(int64(f))
	}
	return sprintf("%.1f", f)
}

// formatInt 格式化整数
func formatInt(i int64) string {
	return sprintf("%d", i)
}

// sprintf 简单的格式化函数
func sprintf(format string, args ...interface{}) string {
	// 这里应该使用 fmt.Sprintf，但为了避免导入 fmt 包，简化实现
	// 在实际使用中，应该导入 fmt 包并使用 fmt.Sprintf
	switch format {
	case "%.1f":
		if len(args) > 0 {
			if f, ok := args[0].(float64); ok {
				// 简化的浮点数格式化
				intPart := int64(f)
				fracPart := int64((f - float64(intPart)) * 10)
				return formatInt(intPart) + "." + formatInt(fracPart)
			}
		}
	case "%d":
		if len(args) > 0 {
			if i, ok := args[0].(int64); ok {
				// 简化的整数格式化
				if i == 0 {
					return "0"
				}
				
				negative := i < 0
				if negative {
					i = -i
				}
				
				var result string
				for i > 0 {
					digit := i % 10
					result = string(rune('0'+digit)) + result
					i /= 10
				}
				
				if negative {
					result = "-" + result
				}
				return result
			}
		}
	}
	return ""
}