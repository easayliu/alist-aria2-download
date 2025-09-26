package entities

import (
	"time"
)

// ScheduledTask 定时任务实体
type ScheduledTask struct {
	ID          string     `json:"id"`           // 任务ID
	Name        string     `json:"name"`         // 任务名称
	Enabled     bool       `json:"enabled"`      // 是否启用
	Cron        string     `json:"cron"`         // cron表达式
	Path        string     `json:"path"`         // 下载路径
	HoursAgo    int        `json:"hours_ago"`    // 下载多少小时内的文件
	VideoOnly   bool       `json:"video_only"`   // 是否只下载视频
	AutoPreview bool       `json:"auto_preview"` // 是否预览模式
	CreatedBy   int64      `json:"created_by"`   // 创建者Telegram ID
	CreatedAt   time.Time  `json:"created_at"`   // 创建时间
	UpdatedAt   time.Time  `json:"updated_at"`   // 更新时间
	LastRunAt   *time.Time `json:"last_run_at"`  // 最后运行时间
	NextRunAt   *time.Time `json:"next_run_at"`  // 下次运行时间
}
