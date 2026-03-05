package ztime

import (
	"sync/atomic"
	"time"
)

type Time struct {
	offset int64
}

var TimeInst = NewTime()

func NewTime() *Time {
	return &Time{
		offset: 0,
	}
}

// SetOffset 设置时间偏移（毫秒），替换当前值
func (t *Time) SetOffset(offset int64) {
	atomic.StoreInt64(&t.offset, offset)
}

// AddOffset 累加时间偏移（毫秒）
func (t *Time) AddOffset(delta int64) {
	atomic.AddInt64(&t.offset, delta)
}

func (t *Time) ResetOffset() {
	atomic.StoreInt64(&t.offset, 0)
}

func (t *Time) Now() time.Time {
	return time.Now().Add(time.Duration(atomic.LoadInt64(&t.offset)) * time.Millisecond)
}

// ServerNowUnixMilli 获取时间戳(毫秒)
func ServerNowUnixMilli() int64 {
	return ServerNow().UnixMilli()
}

func ServerNow() time.Time {
	return TimeInst.Now()
}

// TimeStampToStr 时间戳转字符串
func TimeStampToStr(t int64, format string) string {
	if format == "" {
		format = "2006-01-02 15:04:05"
	}
	return time.Unix(t, 0).Format(format)
}

func TimeNowToStr(format string) string {
	if format == "" {
		format = "2006-01-02 15:04:05"
	}
	return ServerNow().Format(format)
}

// CurrentHour 当前小时
func CurrentHour() time.Time {
	now := ServerNow()
	return time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())
}

// NextHour 下个小时开始时间
func NextHour() time.Time {
	now := ServerNow().Add(time.Hour)
	return time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())
}

// CurrentDay 当前天
func CurrentDay() time.Time {
	now := ServerNow()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
}

// NextDay 下一天
func NextDay() time.Time {
	now := ServerNow().AddDate(0, 0, 1)
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
}

// CurrentWeek 当前周
func CurrentWeek() time.Time {
	now := ServerNow()
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	newDate := now.AddDate(0, 0, -weekday+1)
	return time.Date(newDate.Year(), newDate.Month(), newDate.Day(), 0, 0, 0, 0, newDate.Location())
}

// NextWeek 下周一 00:00:00（周一到周日均返回下一个周一）
func NextWeek() time.Time {
	now := ServerNow()
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7 // Sunday = 7
	}
	daysUntilNextMonday := 8 - weekday // Monday=1→7, Tue=2→6, ..., Sun=7→1
	nextMonday := now.AddDate(0, 0, daysUntilNextMonday)
	return time.Date(nextMonday.Year(), nextMonday.Month(), nextMonday.Day(), 0, 0, 0, 0, nextMonday.Location())
}

// CurrentMonth 当前月
func CurrentMonth() time.Time {
	now := ServerNow()
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
}

// NextMonth 下个月开始时间
func NextMonth() time.Time {
	now := ServerNow().AddDate(0, 1, 0)
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
}
