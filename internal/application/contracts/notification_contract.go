package contracts

import (
	"context"
	"time"
)

// NotificationLevel 通知级别
type NotificationLevel string

const (
	NotificationLevelInfo    NotificationLevel = "info"
	NotificationLevelWarning NotificationLevel = "warning"
	NotificationLevelError   NotificationLevel = "error"
	NotificationLevelSuccess NotificationLevel = "success"
)

// NotificationChannel 通知渠道
type NotificationChannel string

const (
	ChannelTelegram NotificationChannel = "telegram"
	ChannelWebhook  NotificationChannel = "webhook"
	ChannelEmail    NotificationChannel = "email"
	ChannelSystem   NotificationChannel = "system"
)

// NotificationRequest 通知请求
type NotificationRequest struct {
	Channel   NotificationChannel `json:"channel" validate:"required"`
	Level     NotificationLevel   `json:"level" validate:"required"`
	Title     string             `json:"title" validate:"required"`
	Message   string             `json:"message" validate:"required"`
	Data      map[string]interface{} `json:"data,omitempty"`
	TargetID  string             `json:"target_id,omitempty"` // 如Telegram chat_id
	Template  string             `json:"template,omitempty"`
	Priority  int                `json:"priority,omitempty"`
}

// NotificationResponse 通知响应
type NotificationResponse struct {
	ID          string              `json:"id"`
	Channel     NotificationChannel `json:"channel"`
	Level       NotificationLevel   `json:"level"`
	Title       string              `json:"title"`
	Message     string              `json:"message"`
	Status      string              `json:"status"`
	SentAt      *time.Time          `json:"sent_at,omitempty"`
	ErrorReason string              `json:"error_reason,omitempty"`
	RetryCount  int                 `json:"retry_count"`
	CreatedAt   time.Time           `json:"created_at"`
}

// BatchNotificationRequest 批量通知请求
type BatchNotificationRequest struct {
	Notifications []NotificationRequest `json:"notifications" validate:"required,dive"`
	BatchMode     bool                  `json:"batch_mode,omitempty"`  // 是否批量发送
	FailFast      bool                  `json:"fail_fast,omitempty"`   // 遇到错误是否立即停止
}

// BatchNotificationResponse 批量通知响应
type BatchNotificationResponse struct {
	SuccessCount int                     `json:"success_count"`
	FailureCount int                     `json:"failure_count"`
	Results      []NotificationResult    `json:"results"`
	Summary      NotificationSummary     `json:"summary"`
}

// NotificationResult 单个通知结果
type NotificationResult struct {
	Request      NotificationRequest  `json:"request"`
	Success      bool                `json:"success"`
	Notification *NotificationResponse `json:"notification,omitempty"`
	Error        string              `json:"error,omitempty"`
}

// NotificationSummary 通知摘要
type NotificationSummary struct {
	TotalNotifications int                               `json:"total_notifications"`
	ByChannel          map[NotificationChannel]int       `json:"by_channel"`
	ByLevel            map[NotificationLevel]int         `json:"by_level"`
	ByStatus           map[string]int                    `json:"by_status"`
}

// DownloadNotificationRequest 下载完成通知请求
type DownloadNotificationRequest struct {
	DownloadID   string                 `json:"download_id" validate:"required"`
	Filename     string                 `json:"filename" validate:"required"`
	FileSize     int64                  `json:"file_size"`
	DownloadPath string                 `json:"download_path"`
	Duration     time.Duration          `json:"duration"`
	Success      bool                   `json:"success"`
	ErrorMessage string                 `json:"error_message,omitempty"`
	Extra        map[string]interface{} `json:"extra,omitempty"`
}

// TaskNotificationRequest 任务通知请求
type TaskNotificationRequest struct {
	TaskID       string                 `json:"task_id" validate:"required"`
	TaskName     string                 `json:"task_name" validate:"required"`
	TaskType     string                 `json:"task_type"` // scheduled, manual, etc.
	Status       string                 `json:"status"`    // started, completed, failed
	FilesCount   int                    `json:"files_count"`
	TotalSize    int64                  `json:"total_size"`
	Duration     time.Duration          `json:"duration"`
	ErrorMessage string                 `json:"error_message,omitempty"`
	Extra        map[string]interface{} `json:"extra,omitempty"`
}

// SystemNotificationRequest 系统通知请求
type SystemNotificationRequest struct {
	Component    string                 `json:"component" validate:"required"` // aria2, alist, scheduler, etc.
	Event        string                 `json:"event" validate:"required"`     // startup, shutdown, error, etc.
	Level        NotificationLevel      `json:"level" validate:"required"`
	Message      string                 `json:"message" validate:"required"`
	Details      map[string]interface{} `json:"details,omitempty"`
	Metrics      map[string]interface{} `json:"metrics,omitempty"`
}

// NotificationTemplate 通知模板
type NotificationTemplate struct {
	Name        string                 `json:"name"`
	Channel     NotificationChannel    `json:"channel"`
	Level       NotificationLevel      `json:"level"`
	Title       string                 `json:"title"`
	MessageHTML string                 `json:"message_html"`
	MessageText string                 `json:"message_text"`
	Variables   []string               `json:"variables"`
	Enabled     bool                   `json:"enabled"`
}

// NotificationConfig 通知配置
type NotificationConfig struct {
	Enabled         bool                          `json:"enabled"`
	DefaultChannel  NotificationChannel           `json:"default_channel"`
	MinLevel        NotificationLevel             `json:"min_level"`
	Channels        map[NotificationChannel]bool  `json:"channels"`
	Templates       []NotificationTemplate        `json:"templates"`
	RateLimit       int                          `json:"rate_limit"`       // 每分钟最大通知数
	RetryLimit      int                          `json:"retry_limit"`      // 重试次数
	RetryInterval   time.Duration                `json:"retry_interval"`   // 重试间隔
}

// NotificationService 通知服务业务契约
type NotificationService interface {
	// 基础通知操作
	SendNotification(ctx context.Context, req NotificationRequest) (*NotificationResponse, error)
	SendBatchNotifications(ctx context.Context, req BatchNotificationRequest) (*BatchNotificationResponse, error)
	
	// 业务通知
	NotifyDownloadComplete(ctx context.Context, req DownloadNotificationRequest) error
	NotifyDownloadFailed(ctx context.Context, req DownloadNotificationRequest) error
	NotifyTaskComplete(ctx context.Context, req TaskNotificationRequest) error
	NotifyTaskFailed(ctx context.Context, req TaskNotificationRequest) error
	NotifySystemEvent(ctx context.Context, req SystemNotificationRequest) error
	
	// 模板管理
	GetTemplate(ctx context.Context, name string, channel NotificationChannel) (*NotificationTemplate, error)
	RenderTemplate(ctx context.Context, template *NotificationTemplate, data map[string]interface{}) (string, error)
	
	// 通知历史
	GetNotificationHistory(ctx context.Context, limit int, offset int) ([]NotificationResponse, error)
	GetNotificationStats(ctx context.Context) (*NotificationSummary, error)
	
	// 配置管理
	GetConfig(ctx context.Context) (*NotificationConfig, error)
	UpdateConfig(ctx context.Context, config *NotificationConfig) error
	
	// 健康检查
	CheckChannelHealth(ctx context.Context, channel NotificationChannel) error
	TestNotification(ctx context.Context, channel NotificationChannel, targetID string) error
}