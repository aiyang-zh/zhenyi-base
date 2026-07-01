# API 文档

> 完整 API 见 [pkg.go.dev](https://pkg.go.dev/github.com/aiyang-zh/zhenyi-base)。本文档为各包简要索引。

## 网络层

### ziface
接口定义：`IServer`、`IClient`、`IChannel`、`IMessage`、`IWireMessage`、`IEncrypt`、`ILimit`、`IMetrics`、`IChannelMetrics`、`IChannelMetricsSetter` 等。
- **IClient**：`Request(msg)` 同步发收（默认 ziface.ModeSync）；`Read()` 需 `WithAsyncMode()`（ziface.ModeAsync）；`SendMsgAsync(msg)` 仅 async 模式异步入队。
- **IMetrics**：服务级连接指标（ConnInc/ConnDec/ConnRejectedInc），通过 `BaseServer.SetMetrics(impl)` 注入。
- **IChannelMetrics**：单连接维度指标（BytesRecAdd/BytesSentAdd/ConnErrorsInc/ConnHeartbeatTimeoutInc），通过 `BaseServer.SetChannelMetrics(impl)` 注入，AddChannel 时自动设到实现 `IChannelMetricsSetter` 的 channel。

### znet
网络核心：`BaseServer`、`BaseClient`、`BaseChannel`、`BaseSocket`、`RingBuffer`、`NetMessage`、`ParseData`。协议解析、零拷贝读写、TLS/GM-TLS 配置。
- **AdaptiveWriter**（`buff.go`）：按写频率在多档缓冲间自适应；**默认偏保守**（升档较快、降档需缓冲已空且超过 `idleTimeout` 空闲后再逐级缩小）。档位评估依赖 `Write` 路径（与 `ztime` 取时）；**长时间无写不会触发 tryAdapt**，`Flush`/`Close` 也不评估档位，静默连接可能长期保持较高档位直至下次写入或 `Close`/`Reset`。频率阈值、检查间隔、`idleTimeout` 等为**包内常量**，未通过 Option 暴露，调参需改源码或 fork；详见类型注释。
- **BaseChannel.Send / Close（async）**：`Close` 在 `runSend` 退出前对发送队列 **StopEnqueue**，与 `Send` 的 **TryEnqueue** 配合，避免「已停写仍永久挂起在队列里」。与关闭并发时 **TryEnqueue 可能失败**，此时 `Send` 会立即对 `IMessage` 做 **Release**，**不保证消息已发出**；成功入队的消息由 `runSend` 在写出后 **Release**（每条一次）。若业务需要「必达或显式错误码」，须在协议/业务层处理。**Sync 模式**无发送队列，应使用 **ReplyImmediate**，勿用 `Send`。详见 `znet` 包内 `Send`/`Close` 注释。
- **RingBuffer**：**单协程**使用；**Peek\*** 返回底层 **slice 视图**（零拷贝），在 **Discard / 后续读写 / Reset** 前不得长期持有该切片引用。池化归还前会 **Reset（含 clear）**。**扩容上限**（非写死）：默认按 **`RingBufferMaxSizeForSocket(SocketConfig)`**（`MaxDataLength` + 线协议头，取 2 的幂）；**`SetRingBufferPoolMaxSize`** 改全局池默认（**须在 accept 前调用**）；**`RingBuffer.SetMaxSize`** / **`GetRingBufferForSocket(cfg)`** 改单实例；客户端 **`WithSocketConfig`**、服务端 **`BaseServer.SetSocketConfig`** 自动同步读环与解析器。未定制服务端读环上限时亦可用 **`SetRingBufferPoolMaxSize`**。`RingBufferConfig.MaxSize==0` 为不限制（仅测试/自建）。
- **BaseServer**：`SetMetrics`/`SetChannelMetrics`；**`SetSharedSendWorkerMode`**、`BindSharedSendHook`、`RunChannel`；**`SetChannelAuth`**；**`SetSocketConfig`**（accept 前；`NewBaseChannel` 读环与 `BaseSocket` 对齐）；**`maxConn>0` 时 `HandleAccept` Load 快拒（所有 `IChannel`）**，`*BaseChannel` 通过后 `MarkAcceptConnSlotReserved`，**`connCount` 仅在 `AddChannel` 递增**（`HandleAccept` true 未 `AddChannel` 不泄漏计数）。**`SetDisconnectOnDecryptError(false)`** 恢复 1.1.4 解密失败仅丢包（默认 true 断链）。**`BaseClose`** drain 后 **`stopSharedSendWorkers`** 取消 worker ctx 排空队列（超时亦主动 stop）。**`SendLoopTuning`** 含 `ReactorMaxQueuedMsgs`、`ReactorFlushBatchesPerTurn`、`SharedSendCloseTimeout`（`BaseClose` 并行 drain，全局 ctx 2×本值）。
- **BaseChannel**：`acceptConnSlotMarker`（HandleAccept 闸门标记）；reactor 实现 `ReactorChannelLifecycle`；`CloseFromSharedSendPath`、`RecordReadIngestError`；**服务端解密失败默认断链**（`SetDisconnectOnDecryptError` 控制）；**`BaseClient` async `Read()` 解密失败仍跳过**。**`OnRead` 不得阻塞**；**`SetSendQueueOverflowHook`**。
- **客户端**：`NewBaseClient(opts...)`、`WithAsyncMode()`、`WithSocketConfig(cfg)`、`WithTLSConfig(cfg)`、`WithDialTimeout(d)`、`WithWebSocketPath(path)`、`WithWebSocketHeaders(h)`；默认 sync（Request），可选 async（Read）。`DialTLSWithTimeout` 供传输层建连使用（标准 TLS 与 GM-TLS 均尊重 `timeout`）。
- **直写**：`WriteImmediate`、`PreparePacketFromWire`（读协程内同步直写，sync/RPC 场景原生支持）。
- **日志级别（Warn / Error）**：网络层统一约定如下（`zlog.Debug` 用于可关闭的诊断；`zlog.Info` 用于正常生命周期如 listener 已关闭）：
  - **`Error`**：服务/进程级失败或重试耗尽——**监听/拨号启动失败**（如 `Failed to start WebSocket server`、`Failed to dial WebSocket server`）、**listener 关闭失败**、`shared send worker panic`、**shared-send close ack 重试仍超时**（mailbox 可能残留）、**单包超过缓冲必须断链**、**服务端 `SendBatchMsg` 加密失败**（配置/密钥类严重错误）、sync 模式误用 `Send`。
  - **`Warn`**：单连接或可恢复退化——**`Close`/`BaseClose` 路径上 `conn.Close()` 的非预期错误**（已过滤 `use of closed network connection` 等预期文案）、对端断开/读写出错、心跳超时、解析错误、**解密失败仅丢包**（`SetDisconnectOnDecryptError(false)`）、WebSocket **upgrade 失败**、**`OnAccept` 拒绝**、发送队列 overflow、shared-send **首次** ack 超时（会 re-hook）、**`BaseClose` drain 全局超时**（已 `stopSharedSendWorkers`）、HTTP **Shutdown 非致命失败**。
  - **客户端发送路径**（`writeImmediate` / `sendBatchMsg`）加密或写失败记 **`Warn`**：单请求失败由业务重试，默认避免 Error 刷屏；与服务端批量出站 **`Error`** 区分。

### ztcp / zws / zkcp
传输实现：`Server`、`Client`、`Channel`，`NewServer`、`NewClient(addr, opts...)`、`NewChannel`。
- **zws**：业务读写经 **WebSocket 二进制帧**（`zws` 内部 `wsConn` 适配 `net.Conn`），与浏览器 `WebSocket` 发送的二进制负载互通；线协议仍为 `msgId+seqId+len+data`（见 `znet.BaseSocket`）。客户端 `WithTLSConfig` 时使用 `wss://`；`WithWebSocketPath`/`WithWebSocketHeaders` 可配置握手路径与头；服务端 **`SetWebSocketPath(path)`** 配置升级路径（默认 `/`）。
- **ztcp.Server**：**`ServerReactor(ctx)`** 在 **Linux / macOS** 使用 **`zreactor`** 单循环驱动读（**epoll / kqueue**），并默认 **`SetSharedSendWorkerMode(true)`**；**`SetReactorMetrics(*zreactor.Metrics)`** 在使用 **`ServerReactor`** 时生效，传 nil 表示不埋点。**其它 GOOS** 调用 **`ServerReactor`** 会 panic，请使用普通 **`Server(ctx)`**。
- `NewClient(addr)` 默认 sync；`NewClient(addr, znet.WithAsyncMode())` 启用 async（Read），与 server WithAsyncMode 共用 ziface.ModeAsync。

### zreactor（Linux / macOS）
**Linux**：epoll；**macOS（darwin）**：kqueue。TCP 服务循环：**`Serve`**、**`ServeWithConfig`**，listener 须为 **`*net.TCPListener`**。
- **`ReactorChannel`** / **`ReactorChannelLifecycle`**：读缓冲写入、解析分发、异步 `CloseFromReactor`；`*znet.BaseChannel` 在 ServerReactor 共享写下实现后者。
- **`ingestConnReadAndDispatch`**（内部）：syscall.Read 后写缓冲并 Parse；缓冲满最多 Parse 一次腾挪；失败可经 **`ReactorReadMetrics.RecordReadIngestError`** 递增 ConnErrors。
- **`ServeConfig.HeartbeatPollMs`**（默认 1s，**-1** 禁用）：I/O 等待超时时 **`checkHeartbeats`** 扫描 **`Check() bool`** 并关闭空闲超时连接。
- **`Metrics`**：可选监控回调；**ztcp** 通过 **`SetReactorMetrics(m)`** 注入。**`!linux && !darwin`** 构建为 stub。

### zserver
轻量服务器：`New`、`Handle`、`Run`、`Request`、`Conn`，3 步启动。
- **Request/Conn**：`Request` 为服务端请求封装；`Reply` 在 sync 下直写、async 下入队；`ReplyImmediate` 读协程内直写（配合 `WithDirectDispatch`）。
- **Option**：`WithDirectDispatch`、`WithDirectDispatchRef`；默认 sync；`WithAsyncMode`；`WithContext`；**`WithReactorMode`**（**Linux / macOS**、**TCP**、**无 TLS** 时走 **`ztcp.ServerReactor`**，否则忽略）；**`WithBanner(show bool)`**（默认 true）；**`WithName(name string)`**（横幅第二行）。ASCII：**`zbrand.Banner`**。

### zbrand
- **`Banner`**：`const`；`zserver.printBanner`、示例客户端使用。

### 模糊测试（可选）

若干包提供 **`Fuzz*`** 用例，可对序列化、缓冲、国密、`zgmtls` 握手解析、队列语义等做 **`go test -fuzz`**。示例（任选包与用例名，详见各包 `fuzz_test.go`）：

```bash
go test ./zserialize -fuzz=FuzzMarshalJsonRoundtrip -fuzztime=30s
go test ./zqueue -fuzz=FuzzQueueOps -fuzztime=30s
```

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
- `ShardMap`（`Map[K,V]`）- 分片并发 Map，支持 `SetExpire`/`Expire` 带 TTL 条目
- **`ClearTimer`** - 增量清理过期项（非一次删光）；单次最多删 **`LimitDelete`（100）** 条、扫描 **`LimitCheck`（1000）** 条，适合后台 tick 周期调用；未清完的过期项靠后续 tick 或 `Load` 惰性删除

---

## 基础工具

### zerrs
- `TypedError`、`New`、`Newf`、`Wrap`、`Wrapf`
- `IsType`、`Is`、`As`、`IsTimeout`、`IsNetwork` 等

### zlog
- `Logger`、`NewLogger`、`NewDefaultLogger`
- `Info`、`Debug`、`Warn`、`Error`、`Recover`
- `WithLevel`、`WithAsync`、`WithBuffer` 等 Option
- **与 znet 联用**：Warn/Error 场景见上文 **「网络层 → 日志级别（Warn / Error）」**；原则为 **Error = 服务级或重试耗尽**，**Warn = 单连接 I/O 或已降级的关服/清理路径**。

### zencrypt
- AES-GCM、RSA、XTEA、国密 SM2/SM3/SM4
- `bcrypt`、`argon2` 密码哈希

### zgmtls（国密 GM-TLS）
- 包名 **`gmtls`**：`Listen` / `Dial` / `Client` / `Server`、**`Config`**（**`GMSupport`**、**`CipherSuites`**、证书链等）。
- **套件**：**`GMTLS_ECDHE_SM2_WITH_SM4_SM3`**（ECDHE 临时密钥）、**`GMTLS_SM2_WITH_SM4_SM3`**（静态 ECC 密钥封装）；未配置 **`CipherSuites`** 时默认 **ECDHE 优先**，再协商 ECC。
- **服务端**：默认构建下需 **双证书**（签名 + 加密）；ECDHE 的 **ServerKeyExchange** 使用 **SM2 签名证书**。
- 业务侧优先经 **`ziface.GMTLSConfig`** / **`znet.NewGMTLSConfig*`** 使用；直接使用 `gmtls` 的细节见 **[zgmtls/README.md](../zgmtls/README.md)**。
- **静态分析**：遗留 TLS 路径含 RFC 规定的 MD5/SHA-1；**VersionGMSSL** 为 SM3。CodeQL 等工具若报弱哈希，见根目录 **[SECURITY.md](../SECURITY.md)**。

### zserialize
- Protobuf、JSON(sonic)、**MsgPack**（**`github.com/vmihailenco/msgpack/v5`**，`MarshalMsgPack` / `UnmarshalMsgPack`）

### zbackoff / zlimiter / zgrace / zpub / zid / zrand / ztime / ztimer / zfile
各包独立，详见 pkg.go.dev 或源码 godoc。

**zgrace**：优雅退出与停机回调；`Register(func(context.Context))`、`SetContext`、单次 `Wait` 等约定见包注释 `zgrace/doc.go`（`go doc github.com/aiyang-zh/zhenyi-base/zgrace`）。
