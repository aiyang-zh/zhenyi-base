# Changelog

## [1.0.3] - 2026-03-14

### Added
- **zreactor**：Linux 下基于 epoll 的 reactor 模式 TCP 服务循环（`Serve`/`ServeWithConfig`）；优雅退出与 FD 释放说明（doc）；核心流程日志（accept/close/read error）；`Metrics` 回调接口由调用方实现并注入。`ParseAndDispatch` 调用处增加 panic 恢复（`parseAndDispatchSafe`），单连接 handler panic 时仅关闭该连接并打日志，不拖垮进程。
- **ztcp**：`ServerReactor(ctx)`（仅 Linux）使用 zreactor 驱动读；`SetReactorMetrics(*zreactor.Metrics)` 注入 reactor 监控回调；非 Linux 构建使用 `server_reactor_stub.go`，调用 `ServerReactor` 时 panic 提示仅 Linux 可用。
- **ziface**：`IChannelMetricsSetter`，用于向 Channel 注入单连接指标。
- **znet.BaseServer**：`SetChannelMetrics(IChannelMetrics)`，AddChannel 时自动注入到实现 `IChannelMetricsSetter` 的 channel。
- **znet.BaseChannel**：`SetChannelMetrics(m)` 实现 `IChannelMetricsSetter`；`WriteToReadBuffer`/`ParseAndDispatch` 供 zreactor 驱动。
- **zserver**：`WithContext(ctx, cancel)` Option，注入生命周期 context 与 cancel（`Stop()` 会调用 cancel）；不设置时 `New` 内部创建 `context.WithCancel(context.Background())`。
- **docs**：新手学习方案 `BEGINNER_GUIDE.md`（含 Go 前置与阶段 0～4）；文档索引增加该入口。

### Changed
- **znet.BaseServer.SetMetrics**：注释明确为服务级指标（ConnInc/ConnDec/ConnRejectedInc）。

---

## [1.0.2] - 2026-03-13

### Fixed
- **znet.BaseChannel.isNormalCloseError**：去掉内部 `c.Close()` 调用，改为纯判断、无副作用；关闭统一由 `Start()` 的 defer 执行
- **znet.BaseChannel.SendBatchMsg**：入口增加 `IsOpen()` 检查，避免连接已关闭时仍做加密/写
- **zserver.Server.handleRead**：未知 msgId 时打 `zlog.Warn` 再 return；`pool.Invoke` 失败时打 `zlog.Warn` 并记录错误，不再静默丢弃
- **znet.BaseChannel.Send**：sync 模式下由 panic 改为打 `zlog.Error` 并 Release 消息后 return，避免误用打崩进程
- **zserver.Request.Reply**：sync 模式下 `ReplyImmediate` 失败时打 `zlog.Error`，不再静默丢弃错误

### Changed
- **znet.BaseChannel.read()**：返回值由 `int` 改为 `bool`（true 表示应退出读循环），语义更清晰
- **zqueue.SPSCQueue**：`closed` 字段由 `int32` 改为 `atomic.Bool`，与 MPSCQueue 一致
- **zqueue/spsc.go**：删除自定义 `min`，改用 Go 1.21+ 内置 `min`
- **znet.RingBuffer**：注释明确池默认 4KB 与 `DefaultRingBufferConfig` 64KB 的刻意区分（池省内存、直接 New 默认 64KB）

### Added
- **Makefile**：`install-hooks` 目标，用于启用 `.githooks` 提交前跑单测

---

## [1.0.1]

### Added
- **znet.NetMessage.SetDataCopy**：将 data 拷贝到池化 buffer，发送路径 0 分配；Release 时自动归还
- **zserver.Conn.Send**：内部改用 `msg.SetDataCopy(data)`，实现 0 GC
- **ziface.ConnMode**：新增 `ModeSync` / `ModeAsync`，Client 与 Server 统一使用
- **ISession.SetHeartbeatTimeout**：设置心跳超时
- **ITransport.WriteImmediate**：读协程内同步直写，sync/RPC 场景使用
- **zserver**：`WithDirectDispatchRef`、`WithSyncMode`、`WithAsyncMode`；`ReplyImmediate` 同步直写回复；`WithHeartbeatTimeout` 配置/禁用心跳
- **run_echo_bench.sh**：支持协议参数 `[tcp|ws|kcp|all]`；`wait_for_port` 等待服务就绪后再压测

### Changed
- **znet.BaseChannel**：sync 模式不创建 mailBoxQueue，Reply 直写；async 模式保留队列
- **zserver.Request.Reply**：sync 模式下自动调用 ReplyImmediate，async 模式下入队
- **examples/echobench**：文档示例 `server` → `zserver`；服务端增加 WithDirectDispatch、WithAsyncMode、WithDirectDispatchRef

### Fixed
- **zid**：`Init(0)` 时 machineId 为 0，sonyflake 默认读网卡在无网络/沙盒环境失败，改为 fallback `os.Getpid() & 0xFFFF`
- **zcoll**：移除 testify 依赖，改用标准库 `testing` 断言（避免 goproxy 拒绝）
- **1k1k 压测 recv 不足**：服务端默认 30s 心跳在高并发下会因调度延迟误断部分连接；echobench 使用 `WithHeartbeatTimeout(0)` 禁用心跳，TCP/WS 1k1k 可收齐
- **znet.read()**：`ErrBufferFull` 时尝试 `Grow(65536)` 并 drain，避免连接卡死
- **no buffer space available (ENOBUFS)**：服务端/客户端 TCP 每连接读写缓冲为 64KB；run_echo_bench.sh 注释提示遇 ENOBUFS 可 `ulimit -n 4096`

### Removed
- **go.mod**：移除 `github.com/stretchr/testify` 依赖

---

## [1.0.0] - 初始版本

### 包含内容
- 网络层：znet、ztcp、zws、zkcp、zserver、ziface
- 高性能数据结构：zqueue、zpool、zbatch、zcoll
- 基础工具：zerrs、zlog、zencrypt、zserialize、zbackoff、zlimiter、zpub、zid、zgrace、zrand、ztime、ztimer、zfile
- Echo 示例（交互模式 + 压测模式）
