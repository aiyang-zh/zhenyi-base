// Package zreactor 提供基于 Linux epoll 的 reactor 模式 TCP 服务循环（仅 Linux，listener 须为 *net.TCPListener）。
//
// # 使用方式
//
// 方式一（推荐）：通过 ztcp 使用。创建 ztcp.Server 后调用 ServerReactor(ctx)，由 ztcp 内部创建 listener 并调用本包的 Serve。
//
//	ser := ztcp.NewServer(addr, handlers)
//	ser.SetReactorMetrics(&zreactor.Metrics{OnAccept: fn, OnClose: fn, ...}) // 可选
//	ser.ServerReactor(ctx) // 阻塞直到 ctx 取消
//
// 方式二：直接调用本包。需自行 net.Listen("tcp", addr) 得到 *net.TCPListener，实现 AcceptFunc（返回实现 ReactorChannel 的 channel，如 *znet.BaseChannel），然后 Serve(ctx, listener, acceptFn, metrics) 或 ServeWithConfig(..., config)。Serve 返回后调用方应 listener.Close() 释放监听 fd。
//
// # 优雅退出与 FD 防护
//
// Serve/ServeWithConfig 在 ctx 取消时通过 eventfd 唤醒 epoll 并 return，退出前通过 defer 释放：
// listener 的 File() 句柄、wakeFd、poller（epfd）。每个连接在 closeConn 中依次 poller.Remove(fd)、
// ch.Close()、file.Close()、conn.Close()、归还读缓冲、从 fdMap 删除，确保无 FD 泄漏。
package zreactor
