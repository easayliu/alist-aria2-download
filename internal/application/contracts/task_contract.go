package contracts

import (
	"context"
	"time"

	"github.com/easayliu/alist-aria2-download/internal/domain/entities"
)

// TaskRequest 任务请求统一参数
type TaskRequest struct {
	Name        string `json:"name" validate:"required,min=1,max=100"`
	Path        string `json:"path" validate:"required"`
	CronExpr    string `json:"cron_expr" validate:"required"`
	HoursAgo    int    `json:"hours_ago" validate:"required,min=1,max=8760"` // 最多1年
	VideoOnly   bool   `json:"video_only"`
	AutoPreview bool   `json:"auto_preview"`
	Enabled     bool   `json:"enabled"`
	CreatedBy   int64  `json:"created_by"`
}

// TaskUpdateRequest 任务更新请求
type TaskUpdateRequest struct {
	Name        *string `json:"name,omitempty" validate:"omitempty,min=1,max=100"`
	Path        *string `json:"path,omitempty"`
	CronExpr    *string `json:"cron_expr,omitempty"`
	HoursAgo    *int    `json:"hours_ago,omitempty" validate:"omitempty,min=1,max=8760"`
	VideoOnly   *bool   `json:"video_only,omitempty"`
	AutoPreview *bool   `json:"auto_preview,omitempty"`
	Enabled     *bool   `json:"enabled,omitempty"`
}

// TaskResponse 任务响应统一格式
type TaskResponse struct {
	ID          string                `json:"id"`
	Name        string                `json:"name"`
	Path        string                `json:"path"`
	CronExpr    string                `json:"cron_expr"`
	HoursAgo    int                   `json:"hours_ago"`
	VideoOnly   bool                  `json:"video_only"`
	AutoPreview bool                  `json:"auto_preview"`
	Enabled     bool                  `json:"enabled"`
	CreatedBy   int64                 `json:"created_by"`
	Status      entities.TaskStatus   `json:"status"`
	LastRunAt   *time.Time            `json:"last_run_at,omitempty"`
	NextRunAt   *time.Time            `json:"next_run_at,omitempty"`
	RunCount    int                   `json:"run_count"`
	SuccessCount int                  `json:"success_count"`
	FailureCount int                  `json:"failure_count"`
	CreatedAt   time.Time             `json:"created_at"`
	UpdatedAt   time.Time             `json:"updated_at"`
}

// TaskListRequest 任务列表查询参数
type TaskListRequest struct {
	CreatedBy int64  `json:"created_by,omitempty"`
	Enabled   *bool  `json:"enabled,omitempty"`
	Status    string `json:"status,omitempty"`
	Limit     int    `json:"limit,omitempty"`
	Offset    int    `json:"offset,omitempty"`
	SortBy    string `json:"sort_by,omitempty"`
	SortOrder string `json:"sort_order,omitempty"`
}

// TaskListResponse 任务列表响应
type TaskListResponse struct {
	Tasks      []TaskResponse `json:"tasks"`
	TotalCount int           `json:"total_count"`
	Summary    TaskSummary   `json:"summary"`
}

// TaskSummary 任务摘要信息
type TaskSummary struct {
	EnabledCount  int `json:"enabled_count"`
	DisabledCount int `json:"disabled_count"`
	RunningCount  int `json:"running_count"`
	ErrorCount    int `json:"error_count"`
}

// TaskPreviewRequest 任务预览请求
type TaskPreviewRequest struct {
	TaskID   string     `json:"task_id" validate:"required"`
	StartTime *time.Time `json:"start_time,omitempty"`
	EndTime   *time.Time `json:"end_time,omitempty"`
}

// TaskPreviewResponse 任务预览响应
type TaskPreviewResponse struct {
	Task        TaskResponse    `json:"task"`
	Files       []FilePreview   `json:"files"`
	Summary     PreviewSummary  `json:"summary"`
	TimeRange   TimeRange       `json:"time_range"`
}

// FilePreview 文件预览信息
type FilePreview struct {
	Name         string    `json:"name"`
	Path         string    `json:"path"`
	Size         int64     `json:"size"`
	Modified     time.Time `json:"modified"`
	MediaType    string    `json:"media_type"`
	DownloadPath string    `json:"download_path"`
	InternalURL  string    `json:"internal_url"`
}

// PreviewSummary 预览摘要
type PreviewSummary struct {
	TotalFiles  int    `json:"total_files"`
	TotalSize   string `json:"total_size"`
	VideoFiles  int    `json:"video_files"`
	MovieFiles  int    `json:"movie_files"`
	TVFiles     int    `json:"tv_files"`
	OtherFiles  int    `json:"other_files"`
}


// TaskRunRequest 任务执行请求
type TaskRunRequest struct {
	TaskID     string `json:"task_id" validate:"required"`
	Preview    bool   `json:"preview"`
	ForceRun   bool   `json:"force_run"`
	NotifyUser bool   `json:"notify_user"`
}

// TaskRunResponse 任务执行响应
type TaskRunResponse struct {
	TaskID       string           `json:"task_id"`
	RunID        string           `json:"run_id"`
	StartedAt    time.Time        `json:"started_at"`
	Status       string           `json:"status"`
	Preview      *TaskPreviewResponse `json:"preview,omitempty"`
	DownloadIDs  []string         `json:"download_ids,omitempty"`
}

// QuickTaskRequest 快捷任务请求
type QuickTaskRequest struct {
	Type      string `json:"type" validate:"required,oneof=daily recent weekly realtime"`
	Path      string `json:"path,omitempty"`
	CreatedBy int64  `json:"created_by" validate:"required"`
}

// TaskService 任务服务业务契约
type TaskService interface {
	// 基础任务操作
	CreateTask(ctx context.Context, req TaskRequest) (*TaskResponse, error)
	GetTask(ctx context.Context, id string) (*TaskResponse, error)
	UpdateTask(ctx context.Context, id string, req TaskUpdateRequest) (*TaskResponse, error)
	DeleteTask(ctx context.Context, id string) error
	ListTasks(ctx context.Context, req TaskListRequest) (*TaskListResponse, error)
	
	// 任务控制
	EnableTask(ctx context.Context, id string) error
	DisableTask(ctx context.Context, id string) error
	RunTaskNow(ctx context.Context, req TaskRunRequest) (*TaskRunResponse, error)
	StopTask(ctx context.Context, id string) error
	
	// 任务预览
	PreviewTask(ctx context.Context, req TaskPreviewRequest) (*TaskPreviewResponse, error)
	
	// 快捷任务
	CreateQuickTask(ctx context.Context, req QuickTaskRequest) (*TaskResponse, error)
	
	// 用户任务管理
	GetUserTasks(ctx context.Context, userID int64) (*TaskListResponse, error)
	
	// 系统管理
	GetTaskStatistics(ctx context.Context) (map[string]interface{}, error)
	GetSchedulerStatus(ctx context.Context) (map[string]interface{}, error)
}