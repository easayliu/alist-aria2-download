package contracts

import (
	"context"
	"time"

	"github.com/easayliu/alist-aria2-download/internal/domain/models/rename"
)

// FileListRequest 文件列表请求参数
type FileListRequest struct {
	Path      string `json:"path" validate:"required"`
	Page      int    `json:"page,omitempty" validate:"min=1"`
	PageSize  int    `json:"page_size,omitempty" validate:"min=1,max=1000"`
	Recursive bool   `json:"recursive,omitempty"`
	VideoOnly bool   `json:"video_only,omitempty"`
	SortBy    string `json:"sort_by,omitempty" validate:"omitempty,oneof=name size modified"`
	SortOrder string `json:"sort_order,omitempty" validate:"omitempty,oneof=asc desc"`
}

// FileResponse 文件响应信息
type FileResponse struct {
	Name          string    `json:"name"`
	Path          string    `json:"path"`
	Size          int64     `json:"size"`
	SizeFormatted string    `json:"size_formatted"`
	Modified      time.Time `json:"modified"`
	IsDir         bool      `json:"is_dir"`
	MediaType     string    `json:"media_type,omitempty"`
	Category      string    `json:"category,omitempty"`
	DownloadPath  string    `json:"download_path,omitempty"`
	InternalURL   string    `json:"internal_url,omitempty"`
	ExternalURL   string    `json:"external_url,omitempty"`
	Thumbnail     string    `json:"thumbnail,omitempty"`
}

// FileListResponse 文件列表响应
type FileListResponse struct {
	Files       []FileResponse `json:"files"`
	Directories []FileResponse `json:"directories"`
	CurrentPath string         `json:"current_path"`
	ParentPath  string         `json:"parent_path,omitempty"`
	TotalCount  int            `json:"total_count"`
	Summary     FileSummary    `json:"summary"`
	Pagination  Pagination     `json:"pagination"`
}

// FileSummary 文件摘要信息
type FileSummary struct {
	TotalFiles         int    `json:"total_files"`
	TotalDirs          int    `json:"total_dirs"`
	TotalSize          int64  `json:"total_size"`
	TotalSizeFormatted string `json:"total_size_formatted"`
	VideoFiles         int    `json:"video_files"`
	MovieFiles         int    `json:"movie_files"`
	TVFiles            int    `json:"tv_files"`
	OtherFiles         int    `json:"other_files"`
}

// Pagination 分页信息
type Pagination struct {
	Page     int  `json:"page"`
	PageSize int  `json:"page_size"`
	Total    int  `json:"total"`
	HasNext  bool `json:"has_next"`
	HasPrev  bool `json:"has_prev"`
}

// TimeRangeFileRequest 时间范围文件请求
type TimeRangeFileRequest struct {
	Path      string    `json:"path" validate:"required"`
	StartTime time.Time `json:"start_time" validate:"required"`
	EndTime   time.Time `json:"end_time" validate:"required"`
	VideoOnly bool      `json:"video_only,omitempty"`
	HoursAgo  int       `json:"hours_ago,omitempty" validate:"min=1,max=8760"`
}

// TimeRangeFileResponse 时间范围文件响应
type TimeRangeFileResponse struct {
	Files     []FileResponse `json:"files"`
	TimeRange TimeRange      `json:"time_range"`
	Summary   FileSummary    `json:"summary"`
}

// RecentFilesRequest 最近文件请求
type RecentFilesRequest struct {
	Path      string `json:"path" validate:"required"`
	HoursAgo  int    `json:"hours_ago" validate:"required,min=1,max=8760"`
	VideoOnly bool   `json:"video_only,omitempty"`
	Limit     int    `json:"limit,omitempty" validate:"min=1,max=1000"`
}

// FileDownloadRequest 文件下载请求
type FileDownloadRequest struct {
	FilePath     string                 `json:"file_path" validate:"required"`
	TargetDir    string                 `json:"target_dir,omitempty"`
	AutoClassify bool                   `json:"auto_classify,omitempty"`
	Options      map[string]interface{} `json:"options,omitempty"`
}

// BatchFileDownloadRequest 批量文件下载请求
type BatchFileDownloadRequest struct {
	Files        []FileDownloadRequest `json:"files" validate:"required,dive"`
	TargetDir    string                `json:"target_dir,omitempty"`
	VideoOnly    bool                  `json:"video_only,omitempty"`
	AutoClassify bool                  `json:"auto_classify,omitempty"`
}

// DirectoryDownloadRequest 目录下载请求
type DirectoryDownloadRequest struct {
	DirectoryPath string `json:"directory_path" validate:"required"`
	Recursive     bool   `json:"recursive,omitempty"`
	VideoOnly     bool   `json:"video_only,omitempty"`
	AutoClassify  bool   `json:"auto_classify,omitempty"`
	TargetDir     string `json:"target_dir,omitempty"`
}

// FileClassificationRequest 文件分类请求
type FileClassificationRequest struct {
	Files []FileResponse `json:"files" validate:"required,dive"`
}

// FileClassificationResponse 文件分类响应
type FileClassificationResponse struct {
	ClassifiedFiles map[string][]FileResponse `json:"classified_files"`
	Summary         ClassificationSummary     `json:"summary"`
}

// ClassificationSummary 分类摘要
type ClassificationSummary struct {
	MovieCount int            `json:"movie_count"`
	TVCount    int            `json:"tv_count"`
	OtherCount int            `json:"other_count"`
	Categories map[string]int `json:"categories"`
}

// FileSearchRequest 文件搜索请求
type FileSearchRequest struct {
	Query          string     `json:"query" validate:"required"`
	Path           string     `json:"path,omitempty"`
	FileType       string     `json:"file_type,omitempty"`
	MinSize        int64      `json:"min_size,omitempty"`
	MaxSize        int64      `json:"max_size,omitempty"`
	ModifiedAfter  *time.Time `json:"modified_after,omitempty"`
	ModifiedBefore *time.Time `json:"modified_before,omitempty"`
	Limit          int        `json:"limit,omitempty" validate:"min=1,max=1000"`
}

// RenameSuggestion 重命名建议（领域模型的别名）
// 直接使用 rename.Suggestion，无需转换
type RenameSuggestion = rename.Suggestion

// FileRenameRequest 文件重命名请求（新增）
type FileRenameRequest struct {
	OriginalPath string `json:"original_path"` // 原始文件路径
	UserHint     string `json:"user_hint"`     // 用户额外提示（可选）
}

// FileRenameResponse 文件重命名响应（新增）
type FileRenameResponse struct {
	OriginalName  string     `json:"original_name"`
	SuggestedName string     `json:"suggested_name"`
	Confidence    float32    `json:"confidence"`
	Source        string     `json:"source"` // "tmdb", "llm", "hybrid"
	MediaInfo     *MediaInfo `json:"media_info"`
}

// MediaInfo 媒体信息（新增）
type MediaInfo struct {
	Type    string `json:"type"`     // tv, movie
	Title   string `json:"title"`    // 英文标题
	TitleCN string `json:"title_cn"` // 中文标题
	Year    int    `json:"year"`
	Season  *int   `json:"season"`  // 季度（仅剧集）
	Episode *int   `json:"episode"` // 集数（仅剧集）
}

// HybridStrategy 混合策略类型（新增）
type HybridStrategy int

const (
	// TMDBFirst TMDB优先，失败时使用LLM
	TMDBFirst HybridStrategy = iota
	// LLMFirst LLM优先，失败时使用TMDB
	LLMFirst
	// TMDBOnly 仅使用TMDB
	TMDBOnly
	// LLMOnly 仅使用LLM
	LLMOnly
	// Compare 同时使用并比较结果
	Compare
)

// RenameTask 重命名任务
type RenameTask struct {
	OldPath string
	NewPath string
}

// RenameResult 重命名结果
type RenameResult struct {
	OldPath string
	NewPath string
	Success bool
	Error   error
}

// FileService 文件服务业务契约
type FileService interface {
	// 基础文件操作
	ListFiles(ctx context.Context, req FileListRequest) (*FileListResponse, error)
	GetFileInfo(ctx context.Context, path string) (*FileResponse, error)
	SearchFiles(ctx context.Context, req FileSearchRequest) (*FileListResponse, error)

	// 时间范围文件查询
	GetFilesByTimeRange(ctx context.Context, req TimeRangeFileRequest) (*TimeRangeFileResponse, error)
	GetRecentFiles(ctx context.Context, req RecentFilesRequest) (*FileListResponse, error)
	GetYesterdayFiles(ctx context.Context, path string) (*FileListResponse, error)

	// 文件分类
	ClassifyFiles(ctx context.Context, req FileClassificationRequest) (*FileClassificationResponse, error)
	GetFilesByCategory(ctx context.Context, path string, category string) (*FileListResponse, error)

	// 下载相关
	DownloadFile(ctx context.Context, req FileDownloadRequest) (*DownloadResponse, error)
	DownloadFiles(ctx context.Context, req BatchFileDownloadRequest) (*BatchDownloadResponse, error)
	DownloadDirectory(ctx context.Context, req DirectoryDownloadRequest) (*BatchDownloadResponse, error)

	// 文件工具
	IsVideoFile(filename string) bool
	GetFileCategory(filename string) string
	GetMediaType(filePath string) string
	FormatFileSize(size int64) string
	GenerateDownloadPath(file FileResponse) string

	// 系统功能
	GetStorageInfo(ctx context.Context, path string) (map[string]interface{}, error)

	// 文件重命名
	RenameFile(ctx context.Context, path, newName string) error
	RenameAndMoveFile(ctx context.Context, oldPath, newPath string) error
	BatchRenameAndMoveFiles(ctx context.Context, tasks []RenameTask) []RenameResult
	GetRenameSuggestions(ctx context.Context, path string) ([]RenameSuggestion, error)
	GetBatchRenameSuggestions(ctx context.Context, paths []string) (map[string][]RenameSuggestion, error)

	// 批量重命名(统一使用TMDB批量模式,单文件也通过批量接口处理)
	// 返回: suggestionsMap[文件路径] = 建议列表, usedLLM(已废弃,始终为false), error
	GetBatchRenameSuggestionsWithLLM(ctx context.Context, paths []string) (map[string][]RenameSuggestion, bool, error)

	// 文件删除
	DeleteFile(ctx context.Context, path string) error
	DeleteFiles(ctx context.Context, paths []string) error
}
