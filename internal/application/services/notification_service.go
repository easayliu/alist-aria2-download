package services

import (
	"fmt"
	"time"

	"github.com/easayliu/alist-aria2-download/internal/domain/entities"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/config"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/telegram"
	"github.com/easayliu/alist-aria2-download/pkg/logger"
)

type NotificationService struct {
	telegramClient *telegram.Client
	config         *config.Config
}

func NewNotificationService(cfg *config.Config) *NotificationService {
	var telegramClient *telegram.Client
	if cfg.Telegram.Enabled {
		telegramClient = telegram.NewClient(&cfg.Telegram)
	}

	return &NotificationService{
		telegramClient: telegramClient,
		config:         cfg,
	}
}

func (s *NotificationService) NotifyDownloadStarted(download *entities.Download) {
	if s.telegramClient == nil {
		return
	}

	msg := &telegram.NotificationMessage{
		Type:      "download_started",
		Title:     download.Filename,
		Content:   fmt.Sprintf("URL: %s", download.URL),
		Timestamp: time.Now(),
		Extra: map[string]interface{}{
			"download_id": download.ID,
			"url":         download.URL,
		},
	}

	if err := s.telegramClient.SendNotification(msg); err != nil {
		logger.Error("Failed to send download started notification", "error", err, "downloadID", download.ID)
	}
}

func (s *NotificationService) NotifyDownloadCompleted(download *entities.Download) {
	if s.telegramClient == nil {
		return
	}

	sizeInMB := float64(download.TotalSize) / (1024 * 1024)
	content := fmt.Sprintf("大小: %.2f MB\n下载ID: `%s`", sizeInMB, download.ID)

	msg := &telegram.NotificationMessage{
		Type:      "download_completed",
		Title:     download.Filename,
		Content:   content,
		Timestamp: time.Now(),
		Extra: map[string]interface{}{
			"download_id":  download.ID,
			"total_size":   download.TotalSize,
			"completed_at": download.UpdatedAt,
		},
	}

	if err := s.telegramClient.SendNotification(msg); err != nil {
		logger.Error("Failed to send download completed notification", "error", err, "downloadID", download.ID)
	}
}

func (s *NotificationService) NotifyDownloadError(download *entities.Download) {
	if s.telegramClient == nil {
		return
	}

	msg := &telegram.NotificationMessage{
		Type:      "download_error",
		Title:     download.Filename,
		Content:   download.ErrorMessage,
		Timestamp: time.Now(),
		Extra: map[string]interface{}{
			"download_id": download.ID,
			"error":       download.ErrorMessage,
		},
	}

	if err := s.telegramClient.SendNotification(msg); err != nil {
		logger.Error("Failed to send download error notification", "error", err, "downloadID", download.ID)
	}
}

func (s *NotificationService) NotifyDownloadProgress(download *entities.Download) {
	if s.telegramClient == nil {
		return
	}

	completedMB := float64(download.CompletedSize) / (1024 * 1024)
	totalMB := float64(download.TotalSize) / (1024 * 1024)
	speedKBps := float64(download.Speed) / 1024

	content := fmt.Sprintf(
		"进度: %.1f%%\n已下载: %.2f MB / %.2f MB\n速度: %.2f KB/s",
		download.Progress,
		completedMB,
		totalMB,
		speedKBps,
	)

	msg := &telegram.NotificationMessage{
		Type:      "download_progress",
		Title:     download.Filename,
		Content:   content,
		Timestamp: time.Now(),
		Extra: map[string]interface{}{
			"download_id":    download.ID,
			"progress":       download.Progress,
			"completed_size": download.CompletedSize,
			"total_size":     download.TotalSize,
			"speed":          download.Speed,
		},
	}

	if err := s.telegramClient.SendNotification(msg); err != nil {
		logger.Error("Failed to send download progress notification", "error", err, "downloadID", download.ID)
	}
}

func (s *NotificationService) SendCustomMessage(title, content string) error {
	if s.telegramClient == nil {
		return fmt.Errorf("telegram client not configured")
	}

	msg := &telegram.NotificationMessage{
		Type:      "custom",
		Title:     title,
		Content:   content,
		Timestamp: time.Now(),
	}

	return s.telegramClient.SendNotification(msg)
}

func (s *NotificationService) IsEnabled() bool {
	return s.telegramClient != nil && s.config.Telegram.Enabled
}

// SendMessage 发送消息给指定用户
func (s *NotificationService) SendMessage(userID int64, message string) error {
	if s.telegramClient == nil {
		return fmt.Errorf("telegram client not configured")
	}

	return s.telegramClient.SendMessage(userID, message)
}
