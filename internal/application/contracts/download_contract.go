package contracts

import (
	"context"
	"time"

	"github.com/easayliu/alist-aria2-download/internal/domain/entities"
)

// DownloadRequest 下载请求统一参数
type DownloadRequest struct {
	URL         string                 `json:"url" validate:"required,url"`
	Filename    string                 `json:"filename,omitempty"`
	Directory   string                 `json:"directory,omitempty"`
	Options     map[string]interface{} `json:"options,omitempty"`
	VideoOnly   bool                   `json:"video_only,omitempty"`
	AutoClassify bool                  `json:"auto_classify,omitempty"`
	FileSize    int64                  `json:"file_size,omitempty"` // 文件大小，用于磁盘空间检查
}

// DownloadResponse 下载响应统一格式
type DownloadResponse struct {
	ID           string                 `json:"id"`
	URL          string                 `json:"url"`
	Filename     string                 `json:"filename"`
	Directory    string                 `json:"directory"`
	Status       entities.DownloadStatus `json:"status"`
	Progress     float64                `json:"progress"`
	Speed        int64                  `json:"speed"`
	TotalSize    int64                  `json:"total_size"`
	CompletedSize int64                 `json:"completed_size"`
	ErrorMessage string                 `json:"error_message,omitempty"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
}

// DownloadListRequest 下载列表查询参数
type DownloadListRequest struct {
	Status    entities.DownloadStatus `json:"status,omitempty"`
	Limit     int                    `json:"limit,omitempty"`
	Offset    int                    `json:"offset,omitempty"`
	SortBy    string                 `json:"sort_by,omitempty"`
	SortOrder string                 `json:"sort_order,omitempty"`
}

// DownloadListResponse 下载列表响应
type DownloadListResponse struct {
	Downloads   []DownloadResponse     `json:"downloads"`
	TotalCount  int                   `json:"total_count"`
	ActiveCount int                   `json:"active_count"`
	GlobalStats map[string]interface{} `json:"global_stats"`
}

// BatchDownloadRequest 批量下载请求
type BatchDownloadRequest struct {
	Items       []DownloadRequest `json:"items" validate:"required,dive"`
	Directory   string           `json:"directory,omitempty"`
	VideoOnly   bool             `json:"video_only,omitempty"`
	AutoClassify bool            `json:"auto_classify,omitempty"`
}

// BatchDownloadResponse 批量下载响应
type BatchDownloadResponse struct {
	SuccessCount int                `json:"success_count"`
	FailureCount int                `json:"failure_count"`
	Results      []DownloadResult   `json:"results"`
	Summary      DownloadSummary    `json:"summary"`
}

// DownloadResult 单个下载结果
type DownloadResult struct {
	Request DownloadRequest   `json:"request"`
	Success bool             `json:"success"`
	Download *DownloadResponse `json:"download,omitempty"`
	Error   string           `json:"error,omitempty"`
}

// DownloadSummary 下载摘要信息
type DownloadSummary struct {
	TotalFiles  int   `json:"total_files"`
	TotalSize   int64 `json:"total_size"`
	VideoFiles  int   `json:"video_files"`
	MovieFiles  int   `json:"movie_files"`
	TVFiles     int   `json:"tv_files"`
	OtherFiles  int   `json:"other_files"`
}

// DownloadService 下载服务业务契约
type DownloadService interface {
	// 基础下载操作
	CreateDownload(ctx context.Context, req DownloadRequest) (*DownloadResponse, error)
	GetDownload(ctx context.Context, id string) (*DownloadResponse, error)
	ListDownloads(ctx context.Context, req DownloadListRequest) (*DownloadListResponse, error)
	
	// 下载控制
	PauseDownload(ctx context.Context, id string) error
	ResumeDownload(ctx context.Context, id string) error
	CancelDownload(ctx context.Context, id string) error
	RetryDownload(ctx context.Context, id string) (*DownloadResponse, error)
	
	// 批量操作
	CreateBatchDownload(ctx context.Context, req BatchDownloadRequest) (*BatchDownloadResponse, error)
	PauseAllDownloads(ctx context.Context) error
	ResumeAllDownloads(ctx context.Context) error
	
	// 系统状态
	GetSystemStatus(ctx context.Context) (map[string]interface{}, error)
	GetDownloadStatistics(ctx context.Context) (map[string]interface{}, error)
}