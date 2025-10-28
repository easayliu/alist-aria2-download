package notification

import (
	"context"
	"fmt"
	"time"

	"github.com/easayliu/alist-aria2-download/internal/application/contracts"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/config"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/telegram"
	"github.com/easayliu/alist-aria2-download/pkg/logger"
)

// AppNotificationService 应用层通知服务 - 实现contracts.NotificationService接口
type AppNotificationService struct {
	config         *config.Config
	telegramClient *telegram.Client
}

// NewAppNotificationService 创建应用通知服务
func NewAppNotificationService(cfg *config.Config) contracts.NotificationService {
	var telegramClient *telegram.Client
	if cfg.Telegram.Enabled {
		telegramClient = telegram.NewClient(&cfg.Telegram)
	}

	return &AppNotificationService{
		config:         cfg,
		telegramClient: telegramClient,
	}
}

// NewAppNotificationServiceWithClient 使用现有client创建应用通知服务
func NewAppNotificationServiceWithClient(cfg *config.Config, client *telegram.Client) contracts.NotificationService {
	return &AppNotificationService{
		config:         cfg,
		telegramClient: client,
	}
}

func (s *AppNotificationService) SetTelegramClient(client *telegram.Client) {
	s.telegramClient = client
}

// SendNotification 发送通知
func (s *AppNotificationService) SendNotification(ctx context.Context, req contracts.NotificationRequest) (*contracts.NotificationResponse, error) {
	if s.telegramClient == nil {
		return nil, fmt.Errorf("telegram client not available")
	}

	// 生成通知ID
	notificationID := fmt.Sprintf("notify_%d", time.Now().UnixNano())

	// 构建消息内容
	var message string
	if req.Template != "" {
		// 使用模板渲染（简化实现）
		template, err := s.GetTemplate(ctx, req.Template, req.Channel)
		if err == nil && template != nil {
			rendered, renderErr := s.RenderTemplate(ctx, template, req.Data)
			if renderErr == nil {
				message = rendered
			} else {
				message = req.Message
			}
		} else {
			message = req.Message
		}
	} else {
		// 构建标准消息格式
		message = fmt.Sprintf("<b>%s</b>\n\n%s", req.Title, req.Message)
	}

	// 发送通知
	var err error
	switch req.Channel {
	case contracts.ChannelTelegram:
		if req.TargetID != "" {
			// 发送给指定用户
			err = s.telegramClient.SendMessage(parseInt64(req.TargetID), message)
		} else {
			// 发送给所有授权用户
			err = s.sendToAllTelegramUsers(message)
		}
	default:
		err = fmt.Errorf("unsupported notification channel: %s", req.Channel)
	}

	// 构建响应
	response := &contracts.NotificationResponse{
		ID:        notificationID,
		Channel:   req.Channel,
		Level:     req.Level,
		Title:     req.Title,
		Message:   req.Message,
		CreatedAt: time.Now(),
	}

	if err != nil {
		response.Status = "failed"
		response.ErrorReason = err.Error()
	} else {
		response.Status = "sent"
		now := time.Now()
		response.SentAt = &now
	}

	return response, err
}

// SendBatchNotifications 批量发送通知
func (s *AppNotificationService) SendBatchNotifications(ctx context.Context, req contracts.BatchNotificationRequest) (*contracts.BatchNotificationResponse, error) {
	var results []contracts.NotificationResult
	var successCount, failureCount int
	summary := contracts.NotificationSummary{
		ByChannel: make(map[contracts.NotificationChannel]int),
		ByLevel:   make(map[contracts.NotificationLevel]int),
		ByStatus:  make(map[string]int),
	}

	for _, notificationReq := range req.Notifications {
		result := contracts.NotificationResult{
			Request: notificationReq,
		}

		notification, err := s.SendNotification(ctx, notificationReq)
		if err != nil {
			result.Success = false
			result.Error = err.Error()
			failureCount++
			summary.ByStatus["failed"]++
		} else {
			result.Success = true
			result.Notification = notification
			successCount++
			summary.ByStatus["sent"]++
		}

		// 更新统计
		summary.ByChannel[notificationReq.Channel]++
		summary.ByLevel[notificationReq.Level]++
		summary.TotalNotifications++

		results = append(results, result)

		// 如果设置了失败即停止，遇到错误就退出
		if req.FailFast && err != nil {
			break
		}
	}

	return &contracts.BatchNotificationResponse{
		SuccessCount: successCount,
		FailureCount: failureCount,
		Results:      results,
		Summary:      summary,
	}, nil
}

// NotifyDownloadComplete 下载完成通知
func (s *AppNotificationService) NotifyDownloadComplete(ctx context.Context, req contracts.DownloadNotificationRequest) error {
	if !s.config.Telegram.Enabled {
		return nil // 静默跳过
	}

	sizeStr := formatFileSize(req.FileSize)
	durationStr := req.Duration.String()

	message := fmt.Sprintf(
		"<b>✅ 下载完成</b>\n\n"+
			"<b>文件:</b> <code>%s</code>\n"+
			"<b>大小:</b> %s\n"+
			"<b>用时:</b> %s\n"+
			"<b>路径:</b> <code>%s</code>\n"+
			"<b>任务ID:</b> <code>%s</code>",
		escapeHTML(req.Filename),
		sizeStr,
		durationStr,
		escapeHTML(req.DownloadPath),
		req.DownloadID,
	)

	notificationReq := contracts.NotificationRequest{
		Channel: contracts.ChannelTelegram,
		Level:   contracts.NotificationLevelSuccess,
		Title:   "下载完成",
		Message: message,
	}

	_, err := s.SendNotification(ctx, notificationReq)
	return err
}

// NotifyDownloadFailed 下载失败通知
func (s *AppNotificationService) NotifyDownloadFailed(ctx context.Context, req contracts.DownloadNotificationRequest) error {
	if !s.config.Telegram.Enabled {
		return nil // 静默跳过
	}

	message := fmt.Sprintf(
		"<b>❌ 下载失败</b>\n\n"+
			"<b>文件:</b> <code>%s</code>\n"+
			"<b>任务ID:</b> <code>%s</code>\n"+
			"<b>错误:</b> <code>%s</code>",
		escapeHTML(req.Filename),
		req.DownloadID,
		escapeHTML(req.ErrorMessage),
	)

	notificationReq := contracts.NotificationRequest{
		Channel: contracts.ChannelTelegram,
		Level:   contracts.NotificationLevelError,
		Title:   "下载失败",
		Message: message,
	}

	_, err := s.SendNotification(ctx, notificationReq)
	return err
}

// NotifyTaskComplete 任务完成通知
func (s *AppNotificationService) NotifyTaskComplete(ctx context.Context, req contracts.TaskNotificationRequest) error {
	if !s.config.Telegram.Enabled {
		return nil // 静默跳过
	}

	sizeStr := formatFileSize(req.TotalSize)
	durationStr := req.Duration.String()

	message := fmt.Sprintf(
		"<b>✅ 定时任务完成</b>\n\n"+
			"<b>任务:</b> <code>%s</code>\n"+
			"<b>类型:</b> %s\n"+
			"<b>文件数:</b> %d 个\n"+
			"<b>总大小:</b> %s\n"+
			"<b>用时:</b> %s\n"+
			"<b>任务ID:</b> <code>%s</code>",
		escapeHTML(req.TaskName),
		req.TaskType,
		req.FilesCount,
		sizeStr,
		durationStr,
		req.TaskID,
	)

	notificationReq := contracts.NotificationRequest{
		Channel: contracts.ChannelTelegram,
		Level:   contracts.NotificationLevelSuccess,
		Title:   "任务完成",
		Message: message,
	}

	_, err := s.SendNotification(ctx, notificationReq)
	return err
}

// NotifyTaskFailed 任务失败通知
func (s *AppNotificationService) NotifyTaskFailed(ctx context.Context, req contracts.TaskNotificationRequest) error {
	if !s.config.Telegram.Enabled {
		return nil // 静默跳过
	}

	message := fmt.Sprintf(
		"<b>❌ 定时任务失败</b>\n\n"+
			"<b>任务:</b> <code>%s</code>\n"+
			"<b>类型:</b> %s\n"+
			"<b>任务ID:</b> <code>%s</code>\n"+
			"<b>错误:</b> <code>%s</code>",
		escapeHTML(req.TaskName),
		req.TaskType,
		req.TaskID,
		escapeHTML(req.ErrorMessage),
	)

	notificationReq := contracts.NotificationRequest{
		Channel: contracts.ChannelTelegram,
		Level:   contracts.NotificationLevelError,
		Title:   "任务失败",
		Message: message,
	}

	_, err := s.SendNotification(ctx, notificationReq)
	return err
}

// NotifySystemEvent 系统事件通知
func (s *AppNotificationService) NotifySystemEvent(ctx context.Context, req contracts.SystemNotificationRequest) error {
	if !s.config.Telegram.Enabled {
		return nil // 静默跳过
	}

	var icon string
	switch req.Level {
	case contracts.NotificationLevelError:
		icon = "🚨"
	case contracts.NotificationLevelWarning:
		icon = "⚠️"
	case contracts.NotificationLevelInfo:
		icon = "ℹ️"
	default:
		icon = "📋"
	}

	message := fmt.Sprintf(
		"<b>%s 系统事件</b>\n\n"+
			"<b>组件:</b> %s\n"+
			"<b>事件:</b> %s\n"+
			"<b>消息:</b> <code>%s</code>",
		icon,
		req.Component,
		req.Event,
		escapeHTML(req.Message),
	)

	notificationReq := contracts.NotificationRequest{
		Channel: contracts.ChannelTelegram,
		Level:   req.Level,
		Title:   "系统事件",
		Message: message,
	}

	_, err := s.SendNotification(ctx, notificationReq)
	return err
}

// GetTemplate 获取模板（简化实现）
func (s *AppNotificationService) GetTemplate(ctx context.Context, name string, channel contracts.NotificationChannel) (*contracts.NotificationTemplate, error) {
	// 简化实现：返回基础模板
	return &contracts.NotificationTemplate{
		Name:        name,
		Channel:     channel,
		Title:       "{{.title}}",
		MessageText: "{{.message}}",
		MessageHTML: "<b>{{.title}}</b>\n\n{{.message}}",
		Enabled:     true,
	}, nil
}

// RenderTemplate 渲染模板（简化实现）
func (s *AppNotificationService) RenderTemplate(ctx context.Context, template *contracts.NotificationTemplate, data map[string]interface{}) (string, error) {
	// 简化实现：直接返回HTML模板内容
	content := template.MessageHTML
	if title, ok := data["title"].(string); ok {
		content = fmt.Sprintf("<b>%s</b>\n\n", title)
	}
	if message, ok := data["message"].(string); ok {
		content += message
	}
	return content, nil
}

// GetNotificationHistory 获取通知历史（简化实现）
func (s *AppNotificationService) GetNotificationHistory(ctx context.Context, limit int, offset int) ([]contracts.NotificationResponse, error) {
	// 简化实现：返回空列表
	return []contracts.NotificationResponse{}, nil
}

// GetNotificationStats 获取通知统计（简化实现）
func (s *AppNotificationService) GetNotificationStats(ctx context.Context) (*contracts.NotificationSummary, error) {
	return &contracts.NotificationSummary{
		TotalNotifications: 0,
		ByChannel:          make(map[contracts.NotificationChannel]int),
		ByLevel:            make(map[contracts.NotificationLevel]int),
		ByStatus:           make(map[string]int),
	}, nil
}

// GetConfig 获取配置（简化实现）
func (s *AppNotificationService) GetConfig(ctx context.Context) (*contracts.NotificationConfig, error) {
	return &contracts.NotificationConfig{
		Enabled:        s.config.Telegram.Enabled,
		DefaultChannel: contracts.ChannelTelegram,
		MinLevel:       contracts.NotificationLevelInfo,
		Channels: map[contracts.NotificationChannel]bool{
			contracts.ChannelTelegram: s.config.Telegram.Enabled,
		},
		RateLimit:     60, // 每分钟60条
		RetryLimit:    3,
		RetryInterval: 5 * time.Second,
	}, nil
}

// UpdateConfig 更新配置（简化实现）
func (s *AppNotificationService) UpdateConfig(ctx context.Context, config *contracts.NotificationConfig) error {
	// 简化实现：不支持动态更新
	return fmt.Errorf("config update not supported")
}

// CheckChannelHealth 检查渠道健康状态
func (s *AppNotificationService) CheckChannelHealth(ctx context.Context, channel contracts.NotificationChannel) error {
	switch channel {
	case contracts.ChannelTelegram:
		if s.telegramClient == nil {
			return fmt.Errorf("telegram client not configured")
		}
		// 简化实现：假设健康
		return nil
	default:
		return fmt.Errorf("unsupported channel: %s", channel)
	}
}

// TestNotification 测试通知
func (s *AppNotificationService) TestNotification(ctx context.Context, channel contracts.NotificationChannel, targetID string) error {
	testReq := contracts.NotificationRequest{
		Channel: channel,
		Level:   contracts.NotificationLevelInfo,
		Title:   "测试通知",
		Message: fmt.Sprintf("这是一条测试通知，发送时间：%s", time.Now().Format("2006-01-02 15:04:05")),
		TargetID: targetID,
	}

	_, err := s.SendNotification(ctx, testReq)
	return err
}

// ========== 私有方法 ==========

// sendToAllTelegramUsers 发送消息给所有Telegram用户
func (s *AppNotificationService) sendToAllTelegramUsers(message string) error {
	if s.telegramClient == nil {
		return fmt.Errorf("telegram client not configured")
	}

	// 发送给所有配置的用户
	var lastErr error
	sent := false

	// 发送给普通用户
	for _, chatID := range s.config.Telegram.ChatIDs {
		if err := s.telegramClient.SendMessage(chatID, message); err != nil {
			logger.Warn("Failed to send telegram message", "chatID", chatID, "error", err)
			lastErr = err
		} else {
			sent = true
		}
	}

	// 发送给管理员
	for _, adminID := range s.config.Telegram.AdminIDs {
		if err := s.telegramClient.SendMessage(adminID, message); err != nil {
			logger.Warn("Failed to send telegram message", "adminID", adminID, "error", err)
			lastErr = err
		} else {
			sent = true
		}
	}

	if !sent && lastErr != nil {
		return lastErr
	}

	return nil
}

// parseInt64 解析int64
func parseInt64(s string) int64 {
	if s == "" {
		return 0
	}
	// 简化实现
	return 0
}

// formatFileSize 格式化文件大小
func formatFileSize(size int64) string {
	if size == 0 {
		return "0 B"
	}

	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}

	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	suffixes := []string{"KB", "MB", "GB", "TB"}
	return fmt.Sprintf("%.1f %s", float64(size)/float64(div), suffixes[exp])
}

// escapeHTML 转义HTML字符
func escapeHTML(s string) string {
	// 简化实现
	return s
}