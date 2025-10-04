package dto

import "time"

// FileInfo 文件信息DTO - 应用层数据传输对象
type FileInfo struct {
	Name         string
	Path         string
	Size         int64
	Modified     time.Time
	MediaType    string // "tv", "movie", "other"
	DownloadPath string
	InternalURL  string
}
