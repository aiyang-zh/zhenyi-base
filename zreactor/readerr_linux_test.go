//go:build linux

package zreactor

import (
	"syscall"
	"testing"

	"github.com/aiyang-zh/zhenyi-base/zerrs"
)

func TestClassifyReadErr(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want ReadErrKind
	}{
		{"nil", nil, ReadErrKindUnknown},
		{"ECONNRESET", syscall.ECONNRESET, ReadErrKindReset},
		{"ECONNREFUSED", syscall.ECONNREFUSED, ReadErrKindReset},
		{"ECONNABORTED", syscall.ECONNABORTED, ReadErrKindReset},
		{"ETIMEDOUT", syscall.ETIMEDOUT, ReadErrKindTimeout},
		{"EAGAIN", syscall.EAGAIN, ReadErrKindTimeout},
		{"EPIPE", syscall.EPIPE, ReadErrKindClosed},
		{"ENOTCONN", syscall.ENOTCONN, ReadErrKindClosed},
		{"ENETUNREACH", syscall.ENETUNREACH, ReadErrKindOther},
		{"EHOSTUNREACH", syscall.EHOSTUNREACH, ReadErrKindOther},
		{"wrapped", zerrs.Wrap(syscall.ECONNRESET, zerrs.ErrTypeConnection, "wrap"), ReadErrKindReset},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyReadErr(tt.err)
			if got != tt.want {
				t.Errorf("classifyReadErr() = %v, want %v", got, tt.want)
			}
		})
	}
}
