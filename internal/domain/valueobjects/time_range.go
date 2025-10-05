package valueobjects

import (
	"errors"
	"time"
)

// TimeRange 时间范围值对象
// 不可变的值对象,表示一个时间区间
type TimeRange struct {
	Start time.Time
	End   time.Time
}

// Duration 返回时间范围的持续时间
func (tr TimeRange) Duration() time.Duration {
	return tr.End.Sub(tr.Start)
}

// Contains 判断指定时间是否在范围内
func (tr TimeRange) Contains(t time.Time) bool {
	return !t.Before(tr.Start) && !t.After(tr.End)
}

// Overlaps 判断与另一个时间范围是否有重叠
func (tr TimeRange) Overlaps(other TimeRange) bool {
	return tr.Start.Before(other.End) && tr.End.After(other.Start)
}

// IsValid 判断时间范围是否有效(结束时间必须晚于开始时间)
func (tr TimeRange) IsValid() bool {
	return tr.End.After(tr.Start)
}

// Format 格式化时间范围为字符串
func (tr TimeRange) Format(layout string) string {
	return tr.Start.Format(layout) + " - " + tr.End.Format(layout)
}

// NewTimeRange 创建时间范围值对象
func NewTimeRange(start, end time.Time) (TimeRange, error) {
	tr := TimeRange{Start: start, End: end}
	if !tr.IsValid() {
		return TimeRange{}, errors.New("invalid time range: end time must be after start time")
	}
	return tr, nil
}

// NewTimeRangeFromNow 从当前时间创建指定小时数之前到现在的时间范围
func NewTimeRangeFromNow(hoursAgo int) TimeRange {
	now := time.Now()
	start := now.Add(-time.Duration(hoursAgo) * time.Hour)
	return TimeRange{Start: start, End: now}
}

// NewYesterdayTimeRange 创建昨天的时间范围(昨天00:00 - 昨天23:59:59)
func NewYesterdayTimeRange() TimeRange {
	now := time.Now()
	yesterday := now.AddDate(0, 0, -1)
	start := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, yesterday.Location())
	end := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 23, 59, 59, 999999999, yesterday.Location())
	return TimeRange{Start: start, End: end}
}

// NewTodayTimeRange 创建今天的时间范围(今天00:00 - 当前时间)
func NewTodayTimeRange() TimeRange {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return TimeRange{Start: start, End: now}
}

// NewLastNDaysTimeRange 创建最近N天的时间范围
func NewLastNDaysTimeRange(days int) TimeRange {
	now := time.Now()
	start := now.AddDate(0, 0, -days)
	return TimeRange{Start: start, End: now}
}
