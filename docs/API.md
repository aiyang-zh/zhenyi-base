# API 文档

> 完整 API 见 [pkg.go.dev](https://pkg.go.dev/github.com/aiyang-zh/zhenyi-base)。本文档为各包简要索引。

## 网络层

### ziface
接口定义：`IServer`、`IClient`、`IChannel`、`IMessage`、`IWireMessage`、`IEncrypt`、`ILimit`、`IMetrics` 等。
- **IClient**：`Request(msg)` 同步发收（默认 ziface.ModeSync）；`Read()` 需 `WithAsyncMode()`（ziface.ModeAsync）。

### znet
网络核心：`BaseServer`、`BaseClient`、`BaseChannel`、`BaseSocket`、`RingBuffer`、`NetMessage`、`ParseData`。协议解析、零拷贝读写、TLS/GM-TLS 配置。
- **客户端**：`NewBaseClient(opts...)`、`WithAsyncMode()`；默认 sync（Request），可选 async（Read），与 ziface.ModeSync/ModeAsync 对应。
- **直写**：`WriteImmediate`、`PreparePacketFromWire`（读协程内同步直写，sync/RPC 场景原生支持）。

### ztcp / zws / zkcp
传输实现：`Server`、`Client`、`Channel`，`NewServer`、`NewClient(addr, opts...)`、`NewChannel`。
- `NewClient(addr)` 默认 sync；`NewClient(addr, znet.WithAsyncMode())` 启用 async（Read），与 server WithAsyncMode 共用 ziface.ModeAsync。

### zserver
轻量服务器：`New`、`Handle`、`Run`、`Request`、`Conn`，3 步启动。
- **Request/Conn**：`Request` 为服务端请求封装；`Reply` 在 sync 下直写、async 下入队；`ReplyImmediate` 读协程内直写（配合 `WithDirectDispatch`）。
- **Option**：`WithDirectDispatch`、`WithDirectDispatchRef`；默认 sync 模式（与 client 默认 Request 一致）；`WithAsyncMode` 启用队列模式。

---

## 高性能数据结构

### zqueue
- `MPSCQueue`、`SPSCQueue` - 无锁队列
- `UnboundedMPSC`、`UnboundedSPSC` - 无界队列
- `PriorityQueue`、`SmartDoubleQueue` - 优先队列、双端队列
- `Queue` - 通用队列（支持 Resize）

### zpool
- `Pool[T]` - 泛型对象池
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
- Protobuf、JSON(sonic)、MsgPack 序列化

### zbackoff / zlimiter / zgrace / zpub / zid / zrand / ztime / ztimer / zfile
各包独立，详见 pkg.go.dev 或源码 godoc。
