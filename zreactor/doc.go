// Package zreactor 提供 reactor 模式 TCP 服务循环（Linux: epoll；macOS(darwin): kqueue）。
// listener 须为 *net.TCPListener。
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
// # 读路径与关闭
//
// syscall.Read 结果经 ingestConnReadAndDispatch 写入 channel 读缓冲并 ParseAndDispatch；
// 缓冲满时 writeConnBytes 最多 Parse 一次腾挪后重试，仍失败则 RecordReadIngestError（若 channel 实现 ReactorReadMetrics）并断链。
//
// 实现 ReactorChannelLifecycle（如 ServerReactor 共享写下的 *znet.BaseChannel）时：
//   - 读处理前后须成对 BeginReactorRead/EndReactorRead；
//   - closeConn、心跳超时等路径调用 CloseFromReactor，不阻塞事件循环。
//
// # 心跳
//
// ServeConfig.HeartbeatPollMs 控制 epoll/kqueue 等待超时；超时后 checkHeartbeats 扫描实现 Check() bool 的 channel，
// 返回 false 时 closeConn。默认 1000ms；-1 禁用周期性扫描。
//
// # 优雅退出与 FD 防护
//
// Serve/ServeWithConfig 在 ctx 取消时通过唤醒机制唤醒事件循环并 return，退出前通过 defer 释放：
// - Linux：eventfd
// - macOS(darwin)：pipe
// 以及 poller 实例。每个连接在 closeConn 中依次 poller.Remove(fd)、
// CloseFromReactor（实现 ReactorChannelLifecycle 时）或 Close、归还 reactor 读缓冲、从 fdMap 删除。
package zreactor
