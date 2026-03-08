# API 文档

> 完整 API 见 [pkg.go.dev](https://pkg.go.dev/github.com/aiyang-zh/zhenyi-base)。本文档为各包简要索引。

## 网络层

### ziface
接口定义：`IServer`、`IClient`、`IChannel`、`IMessage`、`IWireMessage`、`IEncrypt`、`ILimit`、`IMetrics` 等。

### znet
网络核心：`BaseServer`、`BaseClient`、`BaseChannel`、`BaseSocket`、`RingBuffer`、`NetMessage`、`ParseData`。协议解析、零拷贝读写、TLS/GM-TLS 配置。

### ztcp / zws / zkcp
传输实现：`Server`、`Client`、`Channel`，`NewServer`、`NewClient`、`NewChannel`。

### zserver
轻量服务器：`New`、`Handle`、`Run`、`Request`、`Conn`，3 步启动。

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
