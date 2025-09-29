package utils

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// TimeParser 时间解析器 - 统一时间解析逻辑
type TimeParser struct {
	formats []string
}

var (
	// 默认时间格式，按使用频率排序
	defaultTimeFormats = []string{
		time.RFC3339,                          // 标准RFC3339: 2006-01-02T15:04:05Z07:00
		time.RFC3339Nano,                      // 标准RFC3339Nano: 2006-01-02T15:04:05.999999999Z07:00
		"2006-01-02T15:04:05.999-07:00",       // 毫秒+时区: 2025-09-27T20:16:27.132+08:00
		"2006-01-02T15:04:05-07:00",           // 秒+时区
		"2006-01-02T15:04:05.999Z",            // 毫秒+UTC
		"2006-01-02T15:04:05Z",                // 秒+UTC
		"2006-01-02T15:04:05.99-07:00",        // 2位毫秒+时区
		"2006-01-02T15:04:05.9-07:00",         // 1位毫秒+时区
		"2006-01-02T15:04:05.999999-07:00",    // 微秒+时区
		"2006-01-02T15:04:05.999999Z",         // 微秒+UTC
		"2006-01-02T15:04:05",                 // ISO格式无时区
		"2006-01-02 15:04:05",                 // 标准格式
		"2006-01-02",                          // 日期格式
	}

	// 全局时间解析器实例
	DefaultParser = NewTimeParser(defaultTimeFormats...)

	// 日期格式正则
	dateRegex = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
)

// NewTimeParser 创建时间解析器
func NewTimeParser(formats ...string) *TimeParser {
	if len(formats) == 0 {
		formats = defaultTimeFormats
	}
	return &TimeParser{formats: formats}
}

// ParseTime 解析时间字符串 - 支持多种格式
func (tp *TimeParser) ParseTime(timeStr string) (time.Time, error) {
	if timeStr == "" {
		return time.Time{}, fmt.Errorf("empty time string")
	}

	timeStr = strings.TrimSpace(timeStr)

	for _, format := range tp.formats {
		if t, err := time.Parse(format, timeStr); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse time string: %s", timeStr)
}

// ParseTimeWithDefault 解析时间字符串，失败时返回默认值
func (tp *TimeParser) ParseTimeWithDefault(timeStr string, defaultTime time.Time) time.Time {
	if t, err := tp.ParseTime(timeStr); err == nil {
		return t
	}
	return defaultTime
}

// ParseTimeOrNow 解析时间字符串，失败时返回当前时间
func (tp *TimeParser) ParseTimeOrNow(timeStr string) time.Time {
	return tp.ParseTimeWithDefault(timeStr, time.Now())
}

// ParseTimeOrZero 解析时间字符串，失败时返回零值时间
func (tp *TimeParser) ParseTimeOrZero(timeStr string) time.Time {
	return tp.ParseTimeWithDefault(timeStr, time.Time{})
}

// TimeRange 时间范围类型
type TimeRange struct {
	Start time.Time
	End   time.Time
}

// IsValid 检查时间范围是否有效
func (tr TimeRange) IsValid() bool {
	return !tr.Start.IsZero() && !tr.End.IsZero() && !tr.Start.After(tr.End)
}

// Contains 检查时间是否在范围内（包含边界）
func (tr TimeRange) Contains(t time.Time) bool {
	return !t.Before(tr.Start) && !t.After(tr.End)
}

// Duration 获取时间范围的持续时间
func (tr TimeRange) Duration() time.Duration {
	if !tr.IsValid() {
		return 0
	}
	return tr.End.Sub(tr.Start)
}

// String 时间范围的字符串表示
func (tr TimeRange) String() string {
	return fmt.Sprintf("%s ~ %s", tr.Start.Format(time.RFC3339), tr.End.Format(time.RFC3339))
}

// TimeComparator 时间比较器 - 统一时间比较逻辑
type TimeComparator struct{}

var DefaultComparator = &TimeComparator{}

// IsInRange 检查时间是否在指定范围内
func (tc *TimeComparator) IsInRange(t, start, end time.Time) bool {
	return !t.Before(start) && !t.After(end)
}

// IsRecentlyModified 检查文件是否在指定小时内修改过
func (tc *TimeComparator) IsRecentlyModified(modTime time.Time, hoursAgo int) bool {
	if modTime.IsZero() {
		return false
	}
	cutoff := time.Now().Add(-time.Duration(hoursAgo) * time.Hour)
	return modTime.After(cutoff)
}

// ParseTimeRange 解析时间范围参数
func ParseTimeRange(startStr, endStr string) (TimeRange, error) {
	if startStr == "" && endStr == "" {
		return TimeRange{}, fmt.Errorf("both start and end time are empty")
	}

	// 尝试解析为完整时间戳
	startTime, err1 := DefaultParser.ParseTime(startStr)
	endTime, err2 := DefaultParser.ParseTime(endStr)

	if err1 == nil && err2 == nil {
		if startTime.After(endTime) {
			return TimeRange{}, fmt.Errorf("start time cannot be after end time")
		}
		return TimeRange{Start: startTime, End: endTime}, nil
	}

	// 尝试解析为日期格式
	if dateRegex.MatchString(startStr) && dateRegex.MatchString(endStr) {
		startTime, err1 := time.Parse("2006-01-02", startStr)
		endTime, err2 := time.Parse("2006-01-02", endStr)

		if err1 == nil && err2 == nil {
			if startTime.After(endTime) {
				return TimeRange{}, fmt.Errorf("start date cannot be after end date")
			}
			// 将结束日期设置为当天的23:59:59
			endTime = endTime.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
			return TimeRange{Start: startTime, End: endTime}, nil
		}
	}

	// 如果都解析失败，返回错误
	if err1 != nil && err2 != nil {
		return TimeRange{}, fmt.Errorf("unable to parse time range: start=%s, end=%s", startStr, endStr)
	}
	if err1 != nil {
		return TimeRange{}, fmt.Errorf("unable to parse start time: %s", startStr)
	}
	return TimeRange{}, fmt.Errorf("unable to parse end time: %s", endStr)
}

// CreateTimeRangeFromHours 根据小时数创建时间范围
func CreateTimeRangeFromHours(hoursAgo int) TimeRange {
	now := time.Now()
	start := now.Add(-time.Duration(hoursAgo) * time.Hour)
	return TimeRange{Start: start, End: now}
}

// CreateYesterdayRange 创建昨天的时间范围
func CreateYesterdayRange() TimeRange {
	now := time.Now()
	startOfYesterday := time.Date(now.Year(), now.Month(), now.Day()-1, 0, 0, 0, 0, now.Location())
	endOfYesterday := startOfYesterday.Add(24 * time.Hour)
	return TimeRange{Start: startOfYesterday, End: endOfYesterday}
}

// CreateTodayRange 创建今天的时间范围
func CreateTodayRange() TimeRange {
	now := time.Now()
	startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return TimeRange{Start: startOfToday, End: now}
}

// CreateWeekRange 创建本周的时间范围
func CreateWeekRange() TimeRange {
	now := time.Now()
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7 // 将周日调整为7
	}
	startOfWeek := now.AddDate(0, 0, -weekday+1).Truncate(24 * time.Hour)
	return TimeRange{Start: startOfWeek, End: now}
}

// FormatDuration 格式化持续时间为可读字符串
func FormatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0f秒", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.0f分钟", d.Minutes())
	} else if d < 24*time.Hour {
		return fmt.Sprintf("%.1f小时", d.Hours())
	} else {
		days := int(d.Hours() / 24)
		hours := d.Hours() - float64(days*24)
		if hours > 0 {
			return fmt.Sprintf("%d天%.1f小时", days, hours)
		}
		return fmt.Sprintf("%d天", days)
	}
}

// FormatTimeAgo 格式化时间为"多久之前"的格式
func FormatTimeAgo(t time.Time) string {
	if t.IsZero() {
		return "未知"
	}

	duration := time.Since(t)
	if duration < 0 {
		return "未来时间"
	}

	if duration < time.Minute {
		return "刚刚"
	} else if duration < time.Hour {
		minutes := int(duration.Minutes())
		return fmt.Sprintf("%d分钟前", minutes)
	} else if duration < 24*time.Hour {
		hours := int(duration.Hours())
		return fmt.Sprintf("%d小时前", hours)
	} else if duration < 7*24*time.Hour {
		days := int(duration.Hours() / 24)
		return fmt.Sprintf("%d天前", days)
	} else if duration < 30*24*time.Hour {
		weeks := int(duration.Hours() / (24 * 7))
		return fmt.Sprintf("%d周前", weeks)
	} else if duration < 365*24*time.Hour {
		months := int(duration.Hours() / (24 * 30))
		return fmt.Sprintf("%d个月前", months)
	} else {
		years := int(duration.Hours() / (24 * 365))
		return fmt.Sprintf("%d年前", years)
	}
}

// 便利函数 - 使用默认解析器
func ParseTime(timeStr string) (time.Time, error) {
	return DefaultParser.ParseTime(timeStr)
}

func ParseTimeWithDefault(timeStr string, defaultTime time.Time) time.Time {
	return DefaultParser.ParseTimeWithDefault(timeStr, defaultTime)
}

func ParseTimeOrNow(timeStr string) time.Time {
	return DefaultParser.ParseTimeOrNow(timeStr)
}

func ParseTimeOrZero(timeStr string) time.Time {
	return DefaultParser.ParseTimeOrZero(timeStr)
}

func IsInRange(t, start, end time.Time) bool {
	return DefaultComparator.IsInRange(t, start, end)
}

func IsRecentlyModified(modTime time.Time, hoursAgo int) bool {
	return DefaultComparator.IsRecentlyModified(modTime, hoursAgo)
}