package ztime

import (
	"testing"
	"time"
)

// ============================================================
// Time — 偏移量
// ============================================================

func TestTime_Now_NoOffset(t *testing.T) {
	ti := NewTime()
	before := time.Now()
	got := ti.Now()
	after := time.Now()

	if got.Before(before) || got.After(after) {
		t.Errorf("Now() should be between %v and %v, got %v", before, after, got)
	}
}

func TestTime_AddOffset(t *testing.T) {
	ti := NewTime()
	ti.AddOffset(3600000) // +1 小时

	now := time.Now()
	got := ti.Now()
	diff := got.Sub(now)

	// 应该在 59m ~ 61m 之间
	if diff < 59*time.Minute || diff > 61*time.Minute {
		t.Errorf("with +1h offset, diff=%v", diff)
	}
}

func TestTime_ResetOffset(t *testing.T) {
	ti := NewTime()
	ti.AddOffset(3600000)
	ti.ResetOffset()

	now := time.Now()
	got := ti.Now()
	diff := got.Sub(now)

	if diff < -time.Second || diff > time.Second {
		t.Errorf("after ResetOffset, diff should be ~0, got %v", diff)
	}
}

// ============================================================
// ServerNow 系列
// ============================================================

func TestServerNowMilli(t *testing.T) {
	before := time.Now().UnixMilli()
	got := ServerNowUnixMilli()
	after := time.Now().UnixMilli()
	if got < before || got > after {
		t.Errorf("ServerNowMilli: %d not in [%d, %d]", got, before, after)
	}
}

// ============================================================
// 时间格式化
// ============================================================

func TestTimeStampToStr(t *testing.T) {
	// 2025-01-01 00:00:00 UTC
	ts := int64(1735689600)
	got := TimeStampToStr(ts, "")
	if len(got) == 0 {
		t.Fatal("empty string")
	}
	// 验证包含日期部分
	if got[:4] != "2025" {
		t.Errorf("year should be 2025, got %q", got)
	}
}

func TestTimeStampToStr_CustomFormat(t *testing.T) {
	ts := int64(1735689600)
	got := TimeStampToStr(ts, "2006/01/02")
	if got != "2025/01/01" {
		t.Errorf("custom format: got %q, want '2025/01/01'", got)
	}
}

func TestTimeNowToStr(t *testing.T) {
	got := TimeNowToStr("")
	if len(got) == 0 {
		t.Fatal("empty string")
	}
	// 基本格式校验: "YYYY-MM-DD HH:MM:SS"
	if len(got) != 19 {
		t.Errorf("default format length should be 19, got %d: %q", len(got), got)
	}
}

// ============================================================
// 时间区间函数
// ============================================================

func TestCurrentHour(t *testing.T) {
	h := CurrentHour()
	if h.Minute() != 0 || h.Second() != 0 || h.Nanosecond() != 0 {
		t.Errorf("CurrentHour should be at minute 0: %v", h)
	}
}

func TestNextHour(t *testing.T) {
	cur := CurrentHour()
	next := NextHour()
	diff := next.Sub(cur)
	if diff != time.Hour {
		t.Errorf("NextHour - CurrentHour: got %v, want 1h", diff)
	}
}

func TestCurrentDay(t *testing.T) {
	d := CurrentDay()
	if d.Hour() != 0 || d.Minute() != 0 || d.Second() != 0 {
		t.Errorf("CurrentDay should be midnight: %v", d)
	}
}

func TestNextDay(t *testing.T) {
	cur := CurrentDay()
	next := NextDay()
	diff := next.Sub(cur)
	if diff != 24*time.Hour {
		t.Errorf("NextDay - CurrentDay: got %v, want 24h", diff)
	}
}

func TestCurrentWeek(t *testing.T) {
	w := CurrentWeek()
	if w.Weekday() != time.Monday {
		t.Errorf("CurrentWeek should be Monday, got %v", w.Weekday())
	}
	if w.Hour() != 0 || w.Minute() != 0 {
		t.Error("CurrentWeek should be at midnight")
	}
}

func TestNextWeek(t *testing.T) {
	cur := CurrentWeek()
	next := NextWeek()
	if next.Weekday() != time.Monday {
		t.Errorf("NextWeek should be Monday, got %v", next.Weekday())
	}
	diff := next.Sub(cur)
	if diff != 7*24*time.Hour {
		t.Errorf("NextWeek - CurrentWeek: got %v, want 168h", diff)
	}
}

func TestCurrentMonth(t *testing.T) {
	m := CurrentMonth()
	if m.Day() != 1 || m.Hour() != 0 {
		t.Errorf("CurrentMonth should be 1st at midnight: %v", m)
	}
}

func TestNextMonth(t *testing.T) {
	cur := CurrentMonth()
	next := NextMonth()
	if next.Day() != 1 {
		t.Errorf("NextMonth should be 1st, got day %d", next.Day())
	}
	if !next.After(cur) {
		t.Error("NextMonth should be after CurrentMonth")
	}
}

// ============================================================
// Benchmarks
// ============================================================

func BenchmarkServerNowMilli(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ServerNowUnixMilli()
	}
}

func BenchmarkServerNow(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ServerNow()
	}
}

func BenchmarkCurrentHour(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		CurrentHour()
	}
}

func BenchmarkTimeNowToStr(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		TimeNowToStr("")
	}
}
