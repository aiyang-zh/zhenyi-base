# Changelog

## [1.1.0] - 2026-03-28

### 升级风险说明

本版含 **编译级破坏性变更**（封装 SM2 / GM-TLS 对外类型）；暂不升级可继续固定 **`v1.0.5`**（或当前所用 tag；远端尚无 **`v1.0.6`** tag）。

| 风险类型 | 说明 |
|----------|------|
| **zencrypt / SM2** | 不再对外暴露 `*sm2.PrivateKey` / 裸 `*ecdsa.PublicKey`。改为 **`SM2PrivateKey` / `SM2PublicKey`**；`GenerateSM2Key` 返回 `*SM2PrivateKey`；公钥使用 **`priv.PublicKey()`**；`SM2Encrypt` / `SM2Decrypt` / `SM2Sign` / `SM2Verify` / PEM 编解码均改为上述类型。 |
| **ziface / GM-TLS** | **`TLSConfig.GMConfig`** 由 `*gmtls.Config` 改为 **`GMTLSConfig`**。勿直接持有 `*gmtls.Config`；通过 **`GMTLSConfig`** 的 **`Set*`** / **`Dial`** 使用；**`WrapListener`**、**`znet.DialTLS`** 已适配。 |
| **TLSModeGM 且 GMConfig 未设置** | **`WrapListener`** 在 **`Mode==TLSModeGM` 且 `GMConfig==nil`** 时 **panic**（与误用明文 listener 区分）；**`znet.DialTLS`** 在相同条件下 **返回错误**，不再退回明文 TCP。 |

### 依赖说明

- **国密实现两套并存（预期）**：**`zencrypt` / `zgmtls`** 使用 **`github.com/emmansun/gmsm`**；**`github.com/xtaci/kcp-go/v5`** 仍 **require** **`github.com/tjfoc/gmsm/sm4`**（多为 **indirect**）。构建中可能同时出现 **tjfoc** 与 **emmansun**；**并非**本版封装错误。若需 **完全去掉 tjfoc**，依赖 **kcp-go** 上游调整、**fork/replace**，或不用 **zkcp**。
- **`zgmtls/`**：改编自 **tjfoc/gmsm**，目录 **Apache-2.0**，与仓库 **MIT** 正文并存；见 **`zgmtls/NOTICE`**。发版前确保目录已入库，**`go test ./...`** 可通过。

### Added

- **zencrypt**：**`SM2PrivateKey`**、**`SM2PublicKey`**，封装国密密钥类型，便于替换底层实现。
- **ziface**：**`GMTLSConfig`**（封装 `*gmtls.Config`）；服务端 **`NewGMTLSServerTLSFromFiles`** / **`FromSingleFile`** / **`FromPEM`** / **`FromPEMSingle`**；客户端 **`NewClientGMTLSTLS`**；**`Dial`**、**`SetInsecureSkipVerify`**、**`SetRootCAsPEM`**、**`IsInsecureSkipVerify`**、**`SetCipherSuites`**（国密套件顺序）、**`SetServerName`**（客户端 SNI / 证书主机名校验名，对应 **`gmtls.Config.ServerName`**）。
- **zgmtls**：**`GMTLS_ECDHE_SM2_WITH_SM4_SM3`**（**`ecdheKeyAgreementGM`**）全链路实现（见 **Changed · zgmtls**）。
- **zgmtls / 测试**：在既有覆盖（双证书握手、**`GMX509KeyPairs*`**、**`Load*KeyPair`**、**`X509KeyPair`**、消息往返、`prf`、**`eccKeyAgreementGM`** 大端长度等）基础上补充：**`ecdhe_gm_test.go`**（**`hashForServerKeyExchange`** 的 GM+SM2 **SM3**、`ecdheKeyAgreementGM` 共享密钥往返、**`processServerKeyExchange`** 签名长度）；**`handshake_test`** 增加 **仅 ECDHE** / **仅 ECC** 握手用例；**`newTestGMServerCertificates`** 增加 **DNS SAN**（`zgmtls-test`），避免新版 **x509**「仅 CN、无 SAN」导致主机名校验失败。包级语句覆盖率约 **31%**（大量标准 TLS 遗留分支在纯 GM 场景不执行，属预期）。
- **CodeQL**：**不再全局**排除 **`go/weak-sensitive-data-hashing`**，**`zgmtls/prf.go`** 中对 RFC 路径使用 **`// codeql[go/weak-sensitive-data-hashing]`**（**上一行单独注释**，见 **`SECURITY.md`**），其余文件仍走该规则；在 **Code scanning** 使用 **Default setup** 即可（本仓库不附带 CodeQL Actions 工作流）。
- **开发**：**`make codeql-local`** / **`scripts/run_codeql_local.sh`** 在本地创建 Go CodeQL 数据库并默认只跑 **`go/weak-sensitive-data-hashing`**（需安装 **CodeQL CLI** 并 **`export CODEQL`**）；**`CODEQL_LOCAL_SUITE=1`** 可跑完整 **`go-code-scanning`**；产物在 **`.codeql/`**（已 **gitignore**）。

### Fixed

- **zgmtls**：**`eccKeyAgreementGM.processClientKeyExchange`** / **`processServerKeyExchange`** 对密文/签名长度使用 **`uint16(hi)<<8 | uint16(lo)`**，修正 **`byte<<8`** 丢高位问题，满足 **`go vet`** 与协议大端语义。
- **zgmtls**：**`eccKeyAgreementGM.generateClientKeyExchange`** 对密文分配增加长度上限，避免 **`len(encrypted)+2` 整数溢出**（CodeQL **`go/allocation-size-overflow`**）。
- **zgmtls / CodeQL**：**`prf.go`** 中 SSL3/TLS1.0 路径按 RFC 使用 MD5/SHA-1；**VersionGMSSL** 仅 **SM3**。对 **`go/weak-sensitive-data-hashing`** 采用源码 **`// codeql[...]`** 抑制误报；抑制须写在**各 sink 紧邻上一行**（path-problem 对每个 **`Write(secret)` / `Write(masterSecret)`** 等可能各需一条），说明见 **`SECURITY.md`**。

### Changed

- **zgmtls**：**ECDHE 国密套件**：服务端 SM2 临时密钥、**`ServerKeyExchange`**（SM2 签名）、客户端 **`ClientKeyExchange`** 完成 **`sm2.P256()` ECDH**；**`key_agreement.hashForServerKeyExchange`** 在 **`VersionGMSSL`** 且 **`signatureSM2`** 时对 **client_random、server_random、ECDH 参数** 的字节拼接结果做 **SM3**；**`getCipherSuites` 默认** **ECDHE 优先**；**`makeClientHelloGM`** 在含 ECDHE 套件时附带 **`supportedCurves`**（**`CurveP256`**）；**`ecdheKeyAgreementGM.processServerKeyExchange`** 修正 **`pickSignatureAlgorithm`** 参数顺序。
- **文档**：新增 **`zgmtls/README.md`**；**`README.md`**、**`docs/README.md`**、**`docs/API.md`** 引用 GM-TLS / ECDHE 说明；**`SECURITY.md`**、**`docs/API.md`** 补充 CodeQL / 弱哈希与 **`zgmtls`** 的说明；**`SECURITY.md`** 同步 CodeQL 与 **`prf.go`** 源码抑制说明。
- **Makefile / Git 钩子**：**`make test`** 前执行 **`go fmt` / `go vet` / `go mod tidy`**；**`pre-commit`** 跑完整 **`make test`**；**`install-hooks`** 文案同步。
- **CI / `make test`**：**`run_tests.sh`** 不再跑 **`go test -bench`**（基准仍用 **`go test -bench=...`** 或 **`make bench`**）。
- **ziface**：**`TLSConfig.WrapListener`** 在 **`TLSModeGM`** 下走 **`GMTLSConfig.wrapListener`**；单证书 GM 服务端对双 **`Certificate`** 槽使用 **`dupGMTLSCertificate`**，DER 副本独立。
- **znet**：**`NewGMTLSConfig*`**、**`DialTLS`**、**`NewClientTLSConfig`** 与 **`ziface`** 上述行为一致；**`DialTLS`** 在 **`TLSModeGM` 且 `GMConfig==nil`** 时返回错误。

## [1.0.6] - 2026-03-27

> **说明**：以下变更曾按「1.0.6」记录在案，但**未在仓库打 `v1.0.6` git tag**；当前远端**最新 tag 为 `v1.0.5`**。若需与已发布 tag 对齐的旧基线，请使用 **`v1.0.5`**。

### Added
- **znet**：新增 `SendLoopTuning`（`znet/send_tuning.go`），提供 `SetSendLoopTuning` / `GetSendLoopTuning`，可在启动期调整 `BaseChannel.runSend` 的 batch/backoff/shrink 参数。
- **ziface**：新增 `ISessionStatsSnapshot` 与 `IChannelSessionStatsSnapshot`，用于从连接级统计对象拉取快照并供服务端聚合。
- **zlog**：新增 `AppendPanicHook(fn)`，支持在已存在 panic hook 基础上 CAS 追加执行链。

### Changed
- **znet.BaseChannel**：
  - `NewBaseChannel` 中 `FastAdaptiveBatcher` 参数改为读取 `SendLoopTuning`（默认保持 `1/200/5ms` 不变）。
  - `runSend` 中的 `MaxBatchLimit`、`Backoff(10,30,1us)`、空闲缩容阈值改为可配置项（默认值与旧行为一致）。
  - 增加 `SessionStatsSnapshot()`，将挂接的 `ISessionStats` 导出为统一快照接口。
  - `WriteToReadBuffer` 在 reactor 路径下补充 `BytesRecAdd` 统计，保证与自旋读路径一致。
- **znet.BaseServer**：
  - 新增 `SetSessionStatsFactory` / `GetSessionStatsFactory`，用于为每条连接注入独立 `ISessionStats`。
  - 新增 `AggregateChannelSessionStats()`，按连接聚合 `send/recv count/bytes`。
- **zserialize**：
  - `json_sonic.go` 构建标签调整为 `sonic && (amd64 || arm64)`。
  - 默认 JSON 实现改为 `json_std.go`（`!sonic`），即不显式传 `-tags sonic` 时统一使用 `encoding/json`。

### Removed
- **zserialize**：删除 `json_generic.go`（由统一的 `json_std.go` 覆盖默认实现）。

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
