//go:build darwin

package zreactor

import (
	"syscall"

	"github.com/aiyang-zh/zhenyi-base/zerrs"
)

// classifyReadErr 将读错误归类，便于监控与问题定位。
// darwin 下错误分类与 linux 保持同语义（尽量复用 errors.Is + syscall.Errno）。
func classifyReadErr(err error) ReadErrKind {
	if err == nil {
		return ReadErrKindUnknown
	}

	switch {
	case zerrs.Is(err, syscall.ECONNRESET) || zerrs.Is(err, syscall.ECONNREFUSED) || zerrs.Is(err, syscall.ECONNABORTED):
		return ReadErrKindReset
	case zerrs.Is(err, syscall.ETIMEDOUT) || zerrs.Is(err, syscall.EAGAIN):
		return ReadErrKindTimeout
	case zerrs.Is(err, syscall.EPIPE) || zerrs.Is(err, syscall.ENOTCONN):
		return ReadErrKindClosed
	case zerrs.Is(err, syscall.ENETUNREACH) || zerrs.Is(err, syscall.EHOSTUNREACH):
		return ReadErrKindOther
	default:
		return ReadErrKindOther
	}
}
