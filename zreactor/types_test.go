package zreactor

import "testing"

func TestMetrics_NilSafe(t *testing.T) {
	var m *Metrics
	if m != nil {
		t.Fatal("test assumes nil Metrics")
	}
	// 调用方传 nil 时不应触发任何回调；此处仅保证类型可编译、nil 可传。
	_ = m
}

func TestReadErrKind_String(t *testing.T) {
	tests := []struct {
		k    ReadErrKind
		want string
	}{
		{ReadErrKindUnknown, "unknown"},
		{ReadErrKindTimeout, "timeout"},
		{ReadErrKindReset, "reset"},
		{ReadErrKindClosed, "closed"},
		{ReadErrKindOther, "other"},
	}
	for _, tt := range tests {
		if got := tt.k.String(); got != tt.want {
			t.Errorf("ReadErrKind(%d).String() = %q, want %q", tt.k, got, tt.want)
		}
	}
}
