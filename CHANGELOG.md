# Changelog

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
