package services

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/easayliu/alist-aria2-download/internal/infrastructure/config"
	"github.com/easayliu/alist-aria2-download/pkg/logger"
)

// ConflictPolicy 冲突策略类型
type ConflictPolicy string

const (
	ConflictPolicySkip      ConflictPolicy = "skip"      // 跳过
	ConflictPolicyRename    ConflictPolicy = "rename"    // 重命名
	ConflictPolicyOverwrite ConflictPolicy = "overwrite" // 覆盖
)

// ConflictDetector 冲突检测器 - 检测路径冲突和重复下载
type ConflictDetector struct {
	config          *config.Config
	downloadHistory map[string]DownloadRecord // key: 文件路径
	pathRegistry    map[string]string         // key: 路径, value: 媒体类型
	mutex           sync.RWMutex
}

// DownloadRecord 下载记录
type DownloadRecord struct {
	FilePath     string
	FileName     string
	DownloadPath string
	MediaType    string
	DownloadedAt time.Time
	FileSize     int64
}

// NewConflictDetector 创建冲突检测器
func NewConflictDetector(cfg *config.Config) *ConflictDetector {
	return &ConflictDetector{
		config:          cfg,
		downloadHistory: make(map[string]DownloadRecord),
		pathRegistry:    make(map[string]string),
	}
}

// CheckPathConflict 检测路径冲突
func (d *ConflictDetector) CheckPathConflict(
	targetPath string,
	mediaType string,
) (bool, error) {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	// 检查路径是否已被其他类型占用
	existingType, exists := d.pathRegistry[targetPath]
	if exists && existingType != mediaType {
		return true, fmt.Errorf(
			"路径冲突：%s 已被 %s 类型占用，当前类型为 %s",
			targetPath,
			existingType,
			mediaType,
		)
	}

	return false, nil
}

// CheckDuplicateDownload 检测重复下载
func (d *ConflictDetector) CheckDuplicateDownload(
	filePath string,
) (*DownloadRecord, error) {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	if record, exists := d.downloadHistory[filePath]; exists {
		logger.Info("检测到重复下载",
			"file", filePath,
			"downloaded_at", record.DownloadedAt)

		return &record, fmt.Errorf("文件已下载: %s", filePath)
	}

	return nil, nil
}

// ResolveConflict 解决冲突
func (d *ConflictDetector) ResolveConflict(
	targetPath string,
	policy ConflictPolicy,
) (string, error) {
	logger.Info("解决路径冲突",
		"path", targetPath,
		"policy", policy)

	switch policy {
	case ConflictPolicySkip:
		return "", fmt.Errorf("跳过下载（冲突策略）")

	case ConflictPolicyOverwrite:
		logger.Warn("将覆盖现有文件", "path", targetPath)
		return targetPath, nil

	case ConflictPolicyRename:
		// 生成新路径：添加序号或时间戳
		newPath := d.generateUniquePath(targetPath)
		logger.Info("重命名路径", "original", targetPath, "new", newPath)
		return newPath, nil

	default:
		return "", fmt.Errorf("未知的冲突策略: %s", policy)
	}
}

// generateUniquePath 生成唯一路径
func (d *ConflictDetector) generateUniquePath(path string) string {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	nameWithoutExt := strings.TrimSuffix(base, ext)

	// 策略1: 添加序号 (filename_1.ext, filename_2.ext, ...)
	for i := 1; i < 100; i++ {
		newName := fmt.Sprintf("%s_%d%s", nameWithoutExt, i, ext)
		newPath := filepath.Join(dir, newName)

		d.mutex.RLock()
		_, exists := d.downloadHistory[newPath]
		d.mutex.RUnlock()

		if !exists {
			return newPath
		}
	}

	// 策略2: 回退使用时间戳
	timestamp := time.Now().Format("20060102_150405")
	newName := fmt.Sprintf("%s_%s%s", nameWithoutExt, timestamp, ext)
	return filepath.Join(dir, newName)
}

// RegisterDownload 注册下载记录
func (d *ConflictDetector) RegisterDownload(record DownloadRecord) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// 记录下载历史
	d.downloadHistory[record.FilePath] = record

	// 注册路径和媒体类型
	d.pathRegistry[record.DownloadPath] = record.MediaType

	logger.Debug("注册下载记录",
		"file", record.FilePath,
		"download_path", record.DownloadPath,
		"media_type", record.MediaType)
}

// GetDownloadRecord 获取下载记录
func (d *ConflictDetector) GetDownloadRecord(filePath string) (*DownloadRecord, bool) {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	record, exists := d.downloadHistory[filePath]
	if exists {
		return &record, true
	}

	return nil, false
}

// ClearHistory 清空历史记录
func (d *ConflictDetector) ClearHistory() {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.downloadHistory = make(map[string]DownloadRecord)
	d.pathRegistry = make(map[string]string)

	logger.Info("已清空下载历史记录")
}

// GetHistoryCount 获取历史记录数量
func (d *ConflictDetector) GetHistoryCount() int {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	return len(d.downloadHistory)
}

// GetConflictPolicy 获取冲突策略
func (d *ConflictDetector) GetConflictPolicy() ConflictPolicy {
	// 默认策略：重命名
	return ConflictPolicyRename
}

// ShouldSkipDuplicate 是否跳过重复下载
func (d *ConflictDetector) ShouldSkipDuplicate() bool {
	// 默认不跳过重复下载
	return false
}
