# API 文档

> 完整 API 见 [pkg.go.dev](https://pkg.go.dev/github.com/aiyang-zh/zhenyi-base)。本文档为各包简要索引。

## 网络层

### ziface
接口定义：`IServer`、`IClient`、`IChannel`、`IMessage`、`IWireMessage`、`IEncrypt`、`ILimit`、`IMetrics`、`IChannelMetrics`、`IChannelMetricsSetter` 等。
- **IClient**：`Request(msg)` 同步发收（默认 ziface.ModeSync）；`Read()` 需 `WithAsyncMode()`（ziface.ModeAsync）。
- **IMetrics**：服务级连接指标（ConnInc/ConnDec/ConnRejectedInc），通过 `BaseServer.SetMetrics(impl)` 注入。
- **IChannelMetrics**：单连接维度指标（BytesRecAdd/BytesSentAdd/ConnErrorsInc/ConnHeartbeatTimeoutInc），通过 `BaseServer.SetChannelMetrics(impl)` 注入，AddChannel 时自动设到实现 `IChannelMetricsSetter` 的 channel。

### znet
网络核心：`BaseServer`、`BaseClient`、`BaseChannel`、`BaseSocket`、`RingBuffer`、`NetMessage`、`ParseData`。协议解析、零拷贝读写、TLS/GM-TLS 配置。
- **AdaptiveWriter**（`buff.go`）：按写频率在多档缓冲间自适应；**默认偏保守**（升档较快、降档需缓冲已空且超过 `idleTimeout` 空闲后再逐级缩小）。档位评估依赖 `Write` 路径（与 `ztime` 取时）；**长时间无写不会触发 tryAdapt**，`Flush`/`Close` 也不评估档位，静默连接可能长期保持较高档位直至下次写入或 `Close`/`Reset`。频率阈值、检查间隔、`idleTimeout` 等为**包内常量**，未通过 Option 暴露，调参需改源码或 fork；详见类型注释。
- **BaseChannel.Send / Close（async）**：`Close` 在 `runSend` 退出前对发送队列 **StopEnqueue**，与 `Send` 的 **TryEnqueue** 配合，避免「已停写仍永久挂起在队列里」。与关闭并发时 **TryEnqueue 可能失败**，此时 `Send` 会立即对 `IMessage` 做 **Release**，**不保证消息已发出**；成功入队的消息由 `runSend` 在写出后 **Release**（每条一次）。若业务需要「必达或显式错误码」，须在协议/业务层处理。**Sync 模式**无发送队列，应使用 **ReplyImmediate**，勿用 `Send`。详见 `znet` 包内 `Send`/`Close` 注释。
- **RingBuffer**：**单协程**使用；**Peek\*** 返回底层 **slice 视图**（零拷贝），在 **Discard / 后续读写 / Reset** 前不得长期持有该切片引用。池化归还前会 **Reset（含 clear）**。
- **BaseServer**：`SetMetrics(IMetrics)` 服务级指标；`SetChannelMetrics(IChannelMetrics)` 单连接指标，AddChannel 时注入到实现 `IChannelMetricsSetter` 的 channel。
- **BaseChannel**：实现 `IChannelMetricsSetter`，`SetChannelMetrics(m)` 由 AddChannel 自动调用；供 reactor 驱动时实现 `WriteToReadBuffer`、`ParseAndDispatch`、`GetChannelId`、`Close`。
- **客户端**：`NewBaseClient(opts...)`、`WithAsyncMode()`；默认 sync（Request），可选 async（Read），与 ziface.ModeSync/ModeAsync 对应。
- **直写**：`WriteImmediate`、`PreparePacketFromWire`（读协程内同步直写，sync/RPC 场景原生支持）。

### ztcp / zws / zkcp
传输实现：`Server`、`Client`、`Channel`，`NewServer`、`NewClient(addr, opts...)`、`NewChannel`。
- **zws**：业务读写经 **WebSocket 二进制帧**（`zws` 内部 `wsConn` 适配 `net.Conn`），与浏览器 `WebSocket` 发送的二进制负载互通；线协议仍为 `msgId+seqId+len+data`（见 `znet.BaseSocket`）。
- **ztcp.Server**：`SetReactorMetrics(*zreactor.Metrics)` 仅在使用 `ServerReactor`（Linux）时生效，传 nil 表示不埋点。
- `NewClient(addr)` 默认 sync；`NewClient(addr, znet.WithAsyncMode())` 启用 async（Read），与 server WithAsyncMode 共用 ziface.ModeAsync。

### zreactor（仅 Linux）
基于 epoll 的 reactor 模式 TCP 服务循环：`Serve`、`ServeWithConfig`，listener 须为 `*net.TCPListener`。`Metrics` 为可选监控回调（OnAccept/OnClose/OnReadErr/OnReadBytes 等），由调用方实现并传入；ztcp 使用 reactor 时通过 `SetReactorMetrics(m)` 注入。包内 doc 说明优雅退出与 FD 释放；核心流程（accept/close/read error）打 zlog。单连接 `ParseAndDispatch` panic 时自动恢复并关闭该连接，不拖垮进程。

### zserver
轻量服务器：`New`、`Handle`、`Run`、`Request`、`Conn`，3 步启动。
- **Request/Conn**：`Request` 为服务端请求封装；`Reply` 在 sync 下直写、async 下入队；`ReplyImmediate` 读协程内直写（配合 `WithDirectDispatch`）。
- **Option**：`WithDirectDispatch`、`WithDirectDispatchRef`；默认 sync；`WithAsyncMode`；`WithContext`；`WithReactorMode`（Linux + TCP）；**`WithBanner(show bool)`**（默认 true）；**`WithName(name string)`**（横幅第二行）。ASCII：**`zbrand.Banner`**。

### zbrand
- **`Banner`**：`const`；`zserver.printBanner`、示例客户端使用。

---

## 高性能数据结构

### zqueue
- `MPSCQueue`、`SPSCQueue` - 无锁队列
- `UnboundedMPSC`、`UnboundedSPSC` - 无界队列
- `PriorityQueue`、`SmartDoubleQueue` - 优先队列、双端队列
- `Queue` - 通用队列（支持 Resize）

### zpool
- `Pool[T]` - 泛型对象池；仅当 **T 为指针类型** 时 `Put` 会丢弃 typed nil（`any` 相等比较，见包 `doc`）；非指针 `T`（如切片、标量）不走该分支
- `Buffer`、`GetBytesBuffer`、`PutBytesBuffer` - 字节缓冲池
- `GetNetBuffer`、`PutNetBuffer` - 网络缓冲池

### zbatch
- `AdaptiveBatcher`、`FastAdaptiveBatcher` - 自适应批处理

### zcoll
- `Set[T]` - 泛型集合
- `ShardMap` - 分片并发 Map

---

## 基础工具

### zerrs
- `TypedError`、`New`、`Newf`、`Wrap`、`Wrapf`
- `IsType`、`Is`、`As`、`IsTimeout`、`IsNetwork` 等

### zlog
- `Logger`、`NewLogger`、`NewDefaultLogger`
- `Info`、`Debug`、`Warn`、`Error`、`Recover`
- `WithLevel`、`WithAsync`、`WithBuffer` 等 Option

### zencrypt
- AES-GCM、RSA、XTEA、国密 SM2/SM3/SM4
- `bcrypt`、`argon2` 密码哈希

### zserialize
- Protobuf、JSON(sonic)、**MsgPack**（**`github.com/vmihailenco/msgpack/v5`**，`MarshalMsgPack` / `UnmarshalMsgPack`）

### zbackoff / zlimiter / zgrace / zpub / zid / zrand / ztime / ztimer / zfile
各包独立，详见 pkg.go.dev 或源码 godoc。

**zgrace**：优雅退出与停机回调；`Register(func(context.Context))`、`SetContext`、单次 `Wait` 等约定见包注释 `zgrace/doc.go`（`go doc github.com/aiyang-zh/zhenyi-base/zgrace`）。
