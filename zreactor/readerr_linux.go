//go:build linux

package zreactor

import (
	"errors"
	"syscall"
)

// classifyReadErr 将读错误归类，便于监控与问题定位。
// 覆盖 ECONNRESET、ECONNREFUSED、ETIMEDOUT、EAGAIN、EPIPE、ECONNABORTED、ENETUNREACH、ENOTCONN 等常见错误。
func classifyReadErr(err error) ReadErrKind {
	if err == nil {
		return ReadErrKindUnknown
	}
	switch {
	case errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, syscall.ECONNABORTED):
		return ReadErrKindReset
	case errors.Is(err, syscall.ETIMEDOUT) || errors.Is(err, syscall.EAGAIN):
		return ReadErrKindTimeout
	case errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.ENOTCONN):
		return ReadErrKindClosed
	case errors.Is(err, syscall.ENETUNREACH) || errors.Is(err, syscall.EHOSTUNREACH):
		return ReadErrKindOther
	default:
		return ReadErrKindOther
	}
}
