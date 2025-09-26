package entities

import "time"

// DownloadStatus 下载状态枚举
type DownloadStatus string

const (
	StatusPending  DownloadStatus = "pending"
	StatusActive   DownloadStatus = "active"
	StatusPaused   DownloadStatus = "paused"
	StatusComplete DownloadStatus = "complete"
	StatusError    DownloadStatus = "error"
	StatusRemoved  DownloadStatus = "removed"
)

// Download 下载任务实体
type Download struct {
	ID            string         `json:"id"`
	URL           string         `json:"url"`
	Filename      string         `json:"filename"`
	Status        DownloadStatus `json:"status"`
	Progress      float64        `json:"progress"`
	Speed         int64          `json:"speed"`
	TotalSize     int64          `json:"total_size"`
	CompletedSize int64          `json:"completed_size"`
	ErrorMessage  string         `json:"error_message,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

// File Alist文件信息实体
type File struct {
	Name     string    `json:"name"`
	Size     int64     `json:"size"`
	IsDir    bool      `json:"is_dir"`
	Modified time.Time `json:"modified"`
	Path     string    `json:"path"`
	URL      string    `json:"url,omitempty"`
}
