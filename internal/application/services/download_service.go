package services

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/easayliu/alist-aria2-download/internal/domain/entities"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/aria2"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/config"
	"github.com/easayliu/alist-aria2-download/pkg/utils"
)

type DownloadService struct {
	config      *config.Config
	aria2Client *aria2.Client
}

func NewDownloadService(cfg *config.Config) *DownloadService {
	return &DownloadService{
		config:      cfg,
		aria2Client: aria2.NewClient(cfg.Aria2.RpcURL, cfg.Aria2.Token),
	}
}

// CreateDownload 创建下载任务
func (s *DownloadService) CreateDownload(url, filename, dir string, options map[string]interface{}) (*entities.Download, error) {
	if options == nil {
		options = make(map[string]interface{})
	}

	// 应用视频过滤配置
	if s.config.Download.VideoOnly {
		if !s.isVideoFile(filename) {
			return nil, fmt.Errorf("文件不是视频格式，已跳过下载")
		}
	}

	// 设置下载目录
	if dir != "" {
		options["dir"] = dir
	} else if s.config.Aria2.DownloadDir != "" {
		options["dir"] = s.config.Aria2.DownloadDir
	}

	// 设置文件名
	if filename != "" {
		options["out"] = filename
	}

	gid, err := s.aria2Client.AddURI(url, options)
	if err != nil {
		return nil, fmt.Errorf("failed to create download: %w", err)
	}

	return &entities.Download{
		ID:        gid,
		URL:       url,
		Filename:  s.extractFilename(filename, url),
		Status:    entities.StatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, nil
}

// GetDownloadStatus 获取下载状态
func (s *DownloadService) GetDownloadStatus(gid string) (*entities.Download, error) {
	status, err := s.aria2Client.GetStatus(gid)
	if err != nil {
		return nil, fmt.Errorf("failed to get download status: %w", err)
	}

	totalLength, _ := strconv.ParseInt(status.TotalLength, 10, 64)
	completedLength, _ := strconv.ParseInt(status.CompletedLength, 10, 64)
	downloadSpeed, _ := strconv.ParseInt(status.DownloadSpeed, 10, 64)

	var progress float64
	if totalLength > 0 {
		progress = float64(completedLength) / float64(totalLength) * 100
	}

	// 确定文件名
	filename := ""
	if len(status.Files) > 0 {
		filename = status.Files[0].Path
		if idx := strings.LastIndex(filename, "/"); idx != -1 {
			filename = filename[idx+1:]
		}
	}

	downloadStatus := entities.StatusPending
	switch status.Status {
	case "active":
		downloadStatus = entities.StatusActive
	case "complete":
		downloadStatus = entities.StatusComplete
	case "error":
		downloadStatus = entities.StatusError
	case "paused":
		downloadStatus = entities.StatusPaused
	case "removed":
		downloadStatus = entities.StatusRemoved
	}

	return &entities.Download{
		ID:            gid,
		Filename:      filename,
		Status:        downloadStatus,
		Progress:      progress,
		Speed:         downloadSpeed,
		TotalSize:     totalLength,
		CompletedSize: completedLength,
		ErrorMessage:  status.ErrorMessage,
		UpdatedAt:     time.Now(),
	}, nil
}

// ListDownloads 获取下载列表
func (s *DownloadService) ListDownloads() (map[string]interface{}, error) {
	// 获取活动下载
	active, err := s.aria2Client.GetActive()
	if err != nil {
		return nil, fmt.Errorf("failed to get active downloads: %w", err)
	}

	// 获取等待下载
	waiting, err := s.aria2Client.GetWaiting(0, 50)
	if err != nil {
		return nil, fmt.Errorf("failed to get waiting downloads: %w", err)
	}

	// 获取已停止下载
	stopped, err := s.aria2Client.GetStopped(0, 50)
	if err != nil {
		return nil, fmt.Errorf("failed to get stopped downloads: %w", err)
	}

	// 获取全局统计
	globalStat, err := s.aria2Client.GetGlobalStat()
	if err != nil {
		globalStat = make(map[string]interface{})
	}

	return map[string]interface{}{
		"active":      active,
		"waiting":     waiting,
		"stopped":     stopped,
		"global_stat": globalStat,
		"total_count": len(active) + len(waiting) + len(stopped),
	}, nil
}

// PauseDownload 暂停下载
func (s *DownloadService) PauseDownload(gid string) error {
	return s.aria2Client.Pause(gid)
}

// ResumeDownload 恢复下载
func (s *DownloadService) ResumeDownload(gid string) error {
	return s.aria2Client.Resume(gid)
}

// CancelDownload 取消下载
func (s *DownloadService) CancelDownload(gid string) error {
	return s.aria2Client.Remove(gid)
}

// GetSystemStatus 获取系统状态
func (s *DownloadService) GetSystemStatus() (map[string]interface{}, error) {
	// 检查Aria2连接
	globalStat, err := s.aria2Client.GetGlobalStat()
	aria2Status := "离线"
	if err == nil {
		aria2Status = "在线"
	}

	// 获取版本信息
	version, err := s.aria2Client.GetVersion()
	versionStr := "未知"
	if err == nil {
		versionStr = version.Version
	}

	return map[string]interface{}{
		"aria2": map[string]interface{}{
			"status":      aria2Status,
			"version":     versionStr,
			"global_stat": globalStat,
		},
		"telegram": map[string]interface{}{
			"enabled": s.config.Telegram.Enabled,
			"status":  "运行中",
		},
		"server": map[string]interface{}{
			"port": s.config.Server.Port,
			"mode": s.config.Server.Mode,
		},
	}, nil
}

// isVideoFile 检查是否为视频文件
func (s *DownloadService) isVideoFile(filename string) bool {
	return utils.IsVideoFile(filename, s.config.Download.VideoExts)
}

// extractFilename 提取文件名
func (s *DownloadService) extractFilename(filename, url string) string {
	if filename != "" {
		return filename
	}

	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		filename := parts[len(parts)-1]
		if filename != "" {
			return filename
		}
	}

	return "unknown_file"
}
