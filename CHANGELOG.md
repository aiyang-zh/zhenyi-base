# Changelog

## [1.0.5] - 2026-03-22

### 升级风险说明

本版含破坏性变更与默认行为变化；可继续固定 `v1.0.4`（或当前所用 tag）。

| 风险类型 | 说明 |
|----------|------|
| **编译级破坏** | `ziface` 会话/通道认证 ID：`int64` → **`uint64`**；`zgrace.Register`：`func(context.Context)`。 |
| **协议/校验行为** | `DefaultMaxMsgId`：`math.MaxInt32`；更严上界：`SocketConfig.MaxMsgId`。 |
| **网络通道语义** | `BaseChannel` / `UnboundedMPSC`：`StopEnqueue`、关闭与入队时序与旧版可能不同。 |
| **zgrace 语义** | 单回调 `panic`：`recover`，后续回调仍执行，不向 `Wait` 传播。 |
| **开发与 CI** | `make test-unit`：`-count=5`。 |
| **zbatch 默认窗口** | `AdaptiveBatcher` 默认 **`WindowSize`：64**（原 20）。 |
| **zbatch 配置字段重命名** | `batch.Config`：**`TargetP99` → `TargetMeanLatency`**；字面量赋值需改字段名。 |
| **zserver / zbrand** | **`zserver` 不再导出 `Banner`**，改用 **`zbrand.Banner`**。 |

### Added
- **zcoll**：`SyncMap[K,V]`，对 `sync.Map` 的泛型封装（`Load`/`Store`/`Range` 等）；`znet.BaseServer` 的 `channels` / `authChannels` 与 **`zserver.Server.connCache`** 改用该类型。
- **ztimer**：`TimerPool`（`timer.go`），基于 `zpool` 复用 `time.Timer`，Get/Put 时排空 `C` 并安全 `Reset`。
- **zgrace**：`doc.go`：`Register(context.Context)`、`SetContext`、`Wait`、单回调 `panic` 恢复等说明。
- **znet**：`channel_coverage_test.go` 等覆盖补充。
- **zbrand**：新包，常量 **`Banner`**（ASCII）；`zserver.printBanner`、示例客户端引用。
- **examples**：服务端 **`WithName`**（如 `echodemo/server`、`echobench/server`）；**echobench** 服务端 **`WithBanner(!*quiet)`**；交互客户端 **`fmt.Print(zbrand.Banner)`**。
- **examples/groupchat**：内存群聊示例；自带网页（`embed`）、WebSocket、单房间广播；MsgID 1 加入、2 发言、10 广播事件、99 错误；需 **`WithAsyncMode`** 使 `Send` 入队生效。

### Changed
- **ziface / znet / zserver**（**破坏性**）：会话与通道认证 ID：`int64` → **`uint64`**（`ISession.GetAuthId`/`SetAuthId`、`IServer.SetChannelAuth`/`GetChannelByAuthId`、`BaseServer`、`zserver.Conn.AuthId`/`SetAuthId`）。
- **zgrace**（**破坏性**）：`Register`：`func(context.Context)`；`SetContext`；单回调 `panic`：`recover`。
- **zserver**：`Run` 中 `zgrace.Register` 适配新签名；`connCache.Load` 使用泛型 `SyncMap` 返回值，去掉类型断言。
- **znet**：`DefaultSocketConfig.DefaultMaxMsgId`：`1_000_000_000` → **`math.MaxInt32`**（`common.go` 注释：合法区间、`MinInt32`）；更严上界用 `SocketConfig.MaxMsgId`。
- **znet.RingBuffer**：`Reset`：`clear(buf)`、读写统计归零；`Peek*` 切片生命周期见注释与 `docs/API.md`。
- **znet.BaseChannel / zqueue.UnboundedMPSC**：`StopEnqueue`、`TryEnqueue` 二次检查；`Close` 停入队、`sendDone.Wait`；停止/失败路径 **`Release`**（见注释）。
- **zbatch**（**破坏性**：`Config` 字段名）：**`TargetP99` → `TargetMeanLatency`**；`NewFastAdaptiveBatcher` 第三参 **`targetMeanLatency`**。`DefaultConfig` / 零值补全默认 **`WindowSize`：20→64**。
- **ztimer**：`tickerPool`：`NewPoolWithOptions`，**`WithName("ztimer.ticker")`**。
- **Makefile**：`test-unit`：`-count=1` → **`-count=5`**。
- **docs**：`docs/API.md`：zgrace。
- **examples/echobench**：客户端建连处 **`time.Sleep(1ms)`** 已注释。
- **zserver**：`printBanner` 使用 **`zbrand.Banner`**；不导出 **`Banner`**。
- **zserialize**：MsgPack **`github.com/vmihailenco/msgpack` v4 → `.../msgpack/v5`**；`Decoder` 池：`NewDecoder(bytes.NewReader(nil))`；**`Reset` 无 error 返回值**。
- **go.mod**：`go 1.24.0`；移除 **`google.golang.org/appengine`**、**`github.com/golang/protobuf`**；新增 **`github.com/vmihailenco/tagparser/v2`**。

### Documentation
- **znet**：`docs/API.md`、包注释：**async** 下 `BaseChannel.Send`/`Close`（`StopEnqueue`、`TryEnqueue`、`Release`）；**RingBuffer `Peek*`** 与 `Discard`/后续读写关系。
- **znet**：新增 `TestBaseChannel_Send_AfterClose_ReleasesMessage`、`TestBaseChannel_Send_Close_ConcurrentReleaseCount`（关闭并发下每条消息一次 `Release`）。
- **README.md**、**docs/API.md**、**examples/README.md**、**examples/echodemo/README.md**、**examples/echobench/README.md**、**examples/groupchat/README.md**：**zbrand**、**zserver** `WithBanner`/`WithName`、**zserialize** MsgPack v5、**groupchat** 协议说明、示例依赖 **zbrand**。

### Fixed
- **zws**：WebSocket 读写改为经 **`ReadMessage`/`WriteMessage` 二进制帧** 的 `net.Conn` 适配（`wsconn.go`），不再对 `NetConn()` 裸 TCP 读写；**浏览器等标准 WebSocket 客户端可互通**。
- **zpool**：`Put`：仅 **`T` 为指针**时用 `any(obj)==any(z)` 识别 typed nil，匹配则 `OnPutNil`、不入池；热路径无 `reflect.ValueOf`。
- **znet**：`TestRingBufferPool_GetPut`：池化 `Put` 后 **Stats** 与 `Reset` 一致。

---

## [1.0.4] - 2026-03-15

### Added
- **zpool**: 增加对象池观测与命名配置

### Fixed
- **znet.BaseClient**：修复 Close 与读协程之间的竞态与潜在死锁问题，引入 `connMu` 与 `readWg`，先关闭连接再等待读协程退出并统一清理缓冲区。
- **znet.NetMessage 测试**：修复 Reset 回归用例误报，确保 `SetDataCopy` + `Reset` + 复用场景下无泄漏、无 double-free。
- **znet TLS 客户端配置**：移除默认 `InsecureSkipVerify`，客户端标准 TLS/GM-TLS 默认启用证书校验，避免 CodeQL 报告禁用证书检查的风险。

### Changed
- **Makefile / scripts**：`test-unit`、`run_tests.sh`、基准与覆盖率脚本默认启用 `-race`，在 CI 中统一跑竞态检测。
- **GitHub Actions CI**：`test` workflow 增加最小化 `permissions: contents: read`，限制 `GITHUB_TOKEN` 权限以满足安全扫描要求。

---

## [1.0.3] - 2026-03-14

### Added
- **zreactor**：Linux 下基于 epoll 的 reactor 模式 TCP 服务循环（`Serve`/`ServeWithConfig`）；优雅退出与 FD 释放说明（doc）；核心流程日志（accept/close/read error）；`Metrics` 回调接口由调用方实现并注入。`ParseAndDispatch` 调用处增加 panic 恢复（`parseAndDispatchSafe`），单连接 handler panic 时仅关闭该连接并打日志，不拖垮进程。
- **ztcp**：`ServerReactor(ctx)`（仅 Linux）使用 zreactor 驱动读；`SetReactorMetrics(*zreactor.Metrics)` 注入 reactor 监控回调；非 Linux 构建使用 `server_reactor_stub.go`，调用 `ServerReactor` 时 panic 提示仅 Linux 可用。
- **ziface**：`IChannelMetricsSetter`，用于向 Channel 注入单连接指标。
- **znet.BaseServer**：`SetChannelMetrics(IChannelMetrics)`，AddChannel 时自动注入到实现 `IChannelMetricsSetter` 的 channel。
- **znet.BaseChannel**：`SetChannelMetrics(m)` 实现 `IChannelMetricsSetter`；`WriteToReadBuffer`/`ParseAndDispatch` 供 zreactor 驱动。
- **zserver**：`WithContext(ctx, cancel)` Option，注入生命周期 context 与 cancel（`Stop()` 会调用 cancel）；不设置时 `New` 内部创建 `context.WithCancel(context.Background())`。
- **docs**：新手学习方案 `BEGINNER_GUIDE.md`（含 Go 前置与阶段 0～4）；文档索引增加该入口。
- **scripts**：脚本统一放入 `scripts/`（`run_tests.sh`、`run_echo_bench.sh`、`check_xinchuang_compat.sh`、`run_tests_docker.sh`），Makefile 调用；新增 `scripts/README.md`。
- **信创检查**：支持单架构，`make check-xinchuang-amd64` / `check-xinchuang-arm64` / `check-xinchuang-loong64` 或 `make check-xinchuang PLATFORM=linux/amd64`；脚本可传参或使用环境变量 `PLATFORM`；Docker 内 `go test -v` 输出具体用例。

### Changed
- **znet.BaseServer.SetMetrics**：注释明确为服务级指标（ConnInc/ConnDec/ConnRejectedInc）。
- **测试范围**：`make test`、`make test-unit`、信创检查均排除 `examples/`、`ziface/`，仅测库代码（run_tests.sh、Makefile、check_xinchuang_compat.sh）。
- **Makefile**：`test`/`bench` 改为调用 `scripts/run_tests.sh`、`scripts/run_echo_bench.sh`；新增 `test-docker`、`check-xinchuang` 及单架构目标；`test-unit` 排除 examples/ziface。
- **文档**：README、CONTRIBUTING、examples 下 README 中脚本路径改为 `make test`/`make bench` 或 `./scripts/...`；README 与 CONTRIBUTING 注明测试/覆盖率不包含 examples、ziface。

### Fixed
- **zserialize**：`json_generic.go`（非 amd64/arm64 时使用）补全 `import "encoding/json"`，修复龙芯等架构下 `undefined: json` 编译错误。
- **zbackoff**：龙芯（LoongArch64）无 YIELD 指令，移除 `cpu_yield_loong64.s`，改由 `cpu_yield_other.go` 在 loong64 上提供实现（`runtime.Gosched()` 软回退）；`cpu_yield.go` 构建标签改为仅 `amd64 || arm64`。
- **ztcp**：移除 `server_reactor_linux.go` 未使用的 `znet` import，修复 CI 编译。
- **zqueue**：`TestSPSCQueue_NewCapacity_LargeAndMaxCap` 不再分配 1<<30（约 8GB），改为 1<<20+100 验证大容量，避免 CI/低内存环境被 OOM kill（signal: killed）。
- **zreactor**：单测 `TestServe_AcceptOneThenShutdown` 改为接受连接（返回 `fakeChannel`）而非拒绝，正确触发 OnAccept/OnClose，用例通过。

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
