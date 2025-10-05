package valueobjects

// DownloadStatus 下载状态值对象
// 不可变的值对象,表示下载任务的状态
type DownloadStatus string

const (
	DownloadStatusPending  DownloadStatus = "pending"  // 等待中
	DownloadStatusActive   DownloadStatus = "active"   // 下载中
	DownloadStatusPaused   DownloadStatus = "paused"   // 已暂停
	DownloadStatusComplete DownloadStatus = "complete" // 已完成
	DownloadStatusError    DownloadStatus = "error"    // 错误
	DownloadStatusRemoved  DownloadStatus = "removed"  // 已删除
)

// String 返回状态的字符串表示
func (s DownloadStatus) String() string {
	return string(s)
}

// IsValid 检查下载状态是否有效
func (s DownloadStatus) IsValid() bool {
	switch s {
	case DownloadStatusPending, DownloadStatusActive, DownloadStatusPaused,
		DownloadStatusComplete, DownloadStatusError, DownloadStatusRemoved:
		return true
	default:
		return false
	}
}

// IsActive 判断是否为活动状态
func (s DownloadStatus) IsActive() bool {
	return s == DownloadStatusActive
}

// IsCompleted 判断是否已完成
func (s DownloadStatus) IsCompleted() bool {
	return s == DownloadStatusComplete
}

// IsFailed 判断是否失败
func (s DownloadStatus) IsFailed() bool {
	return s == DownloadStatusError
}

// CanPause 判断是否可以暂停
func (s DownloadStatus) CanPause() bool {
	return s == DownloadStatusActive || s == DownloadStatusPending
}

// CanResume 判断是否可以恢复
func (s DownloadStatus) CanResume() bool {
	return s == DownloadStatusPaused
}

// CanRetry 判断是否可以重试
func (s DownloadStatus) CanRetry() bool {
	return s == DownloadStatusError
}

// NewDownloadStatus 创建下载状态值对象
func NewDownloadStatus(value string) DownloadStatus {
	status := DownloadStatus(value)
	if status.IsValid() {
		return status
	}
	return DownloadStatusPending // 默认为等待状态
}

// ChineseName 返回状态的中文名称
func (s DownloadStatus) ChineseName() string {
	switch s {
	case DownloadStatusPending:
		return "等待中"
	case DownloadStatusActive:
		return "下载中"
	case DownloadStatusPaused:
		return "已暂停"
	case DownloadStatusComplete:
		return "已完成"
	case DownloadStatusError:
		return "错误"
	case DownloadStatusRemoved:
		return "已删除"
	default:
		return "未知"
	}
}
