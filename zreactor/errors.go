package zreactor

import "github.com/aiyang-zh/zhenyi-base/zerrs"

// ErrNotTCPListener 表示 listener 不是 *net.TCPListener，仅 Linux 下 Serve 可能返回。
// 调用方可用 errors.Is(err, zreactor.ErrNotTCPListener) 或 zerrs.IsValidation(err) 判断。
var ErrNotTCPListener = zerrs.New(zerrs.ErrTypeValidation, "zreactor: listener must be *net.TCPListener")
