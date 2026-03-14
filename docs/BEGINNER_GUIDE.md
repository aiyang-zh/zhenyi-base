# zhenyi-base 新手学习方案

本文档面向刚接触 zhenyi-base 的开发者，给出一套**从零到能写 Echo 服务、再选学高阶能力**的完整学习路径。建议按阶段顺序进行，每阶段完成「必做」再进入下一阶段。

**如果你对 Go 也不太熟**：请先完成下面的「前置：Go 基础」，再按阶段 0 → 1 → 2 → 3 进行；否则看示例代码时会吃力。

---

## 〇、前置：Go 语言基础（对 Go 不熟时必看）

在学 zhenyi-base 之前，需要能读懂和写简单 Go 程序。不必精通，下面这些**够用**即可。

### 必会概念（建议 2～5 天摸一遍）

| 内容 | 要会到什么程度 | 参考 |
|------|----------------|------|
| **安装与运行** | 本机装好 Go，能 `go run main.go`、`go build`、`go mod init` | [官方安装](https://go.dev/doc/install)、`go help` |
| **包与 import** | 知道 `package main`、`import "path/to/pkg"`，能 `go get` 拉依赖 | [A Tour of Go - 包](https://go.dev/tour/basics/1) |
| **函数** | 会写 `func name(a, b int) (int, error)`，知道多返回值、`error` | [Tour - 函数](https://go.dev/tour/basics/4) |
| **结构体与指针** | 会 `type S struct { F int }`、`s := &S{}`、`s.F`，知道 `nil` | [Tour - 结构体](https://go.dev/tour/moretypes/2) |
| **接口** | 能看懂 `type I interface { M() }`、某类型实现接口就「可赋值给 I」 | [Tour - 接口](https://go.dev/tour/methods/9) |
| **goroutine 与 channel** | 会 `go f()` 启动协程，会 `ch := make(chan int)`、`<-ch`、`ch <- 1`（本库阶段 1 会用到 goroutine） | [Tour - 并发](https://go.dev/tour/concurrency/1) |
| **错误处理** | 习惯 `if err != nil { return err }`，知道 `errors.New`、`fmt.Errorf` | [Go 错误处理](https://go.dev/blog/error-handling-and-go) |
| **defer** | 知道 `defer f()` 在函数返回前执行，常用于 `defer pool.Put(obj)` | [Tour - defer](https://go.dev/tour/flowcontrol/12) |

### 推荐学习方式（任选其一或组合）

1. **官方交互教程（最快）**  
   [A Tour of Go](https://go.dev/tour/)（约 2～4 小时）：按顺序做一遍，做到「并发」那一章即可。

2. **短篇文字**  
   [Go  by Example](https://gobyexample.com/)：按目录点进去看示例，重点看：Hello World、Values、Structs、Interfaces、Goroutines、Channels、Errors。

3. **中文入门**  
   搜「Go 语言入门」或「Golang 零基础」，选一个 5～10 小时的视频/专栏，看到「接口 + goroutine + channel」即可停，不必等全部看完再碰 zhenyi-base。

### 自测：能写出下面这段再进阶段 0

```go
package main

import "fmt"

func main() {
    ch := make(chan int, 1)
    go func() { ch <- 42 }()
    fmt.Println(<-ch)  // 输出 42
}
```

能看懂、能改、能 `go run` 跑通，就说明 goroutine/channel 的入门够了。然后直接进入下面的「阶段 0」。

---

## 一、从哪里开始：推荐入口

**建议起点**：从**零外部依赖**、**概念简单**的包开始，先建立「高性能基础组件」的直觉，再进入网络层。

| 推荐顺序 | 模块 | 理由 |
|---------|------|------|
| 1 | **zqueue** | 无锁队列，API 简单（Enqueue/Dequeue），零依赖，易验证 |
| 2 | **zpool** | 对象池，减少 GC，与 zqueue 一样零依赖，写法统一 |
| 3 | **zerrs** | 结构化错误，日常必用，零依赖 |
| 4 | **ziface** | 纯接口定义，理解「网络抽象」后再看 znet/ztcp 更清晰 |
| 5 | **zserver + ztcp** | 3 步启动 Echo，立刻看到「服务端 + 客户端」完整闭环 |

先学 **zqueue → zpool → zerrs**，再学 **ziface → zserver/ztcp**，可以避免一上来被网络、协议、连接管理分散注意力。

---

## 二、学习路线总览

```
前置（对 Go 不熟时）：Go 基础 ← 你在这里可多花 2～5 天
    ↓
阶段 0：环境与文档
    ↓
阶段 1：零依赖基础（zqueue / zpool / zerrs）
    ↓
阶段 2：接口与网络抽象（ziface → znet 概念）
    ↓
阶段 3：第一个服务（zserver + ztcp，Echo）
    ↓
阶段 4：按需选学（zlog / zws / zkcp / zbatch / zcoll / 其他工具）
```

---

## 三、阶段 0：环境与文档（约 15 分钟）

**目标**：能拉代码、跑测试、会查文档。

**必做**：

1. **克隆与依赖**
   ```bash
   go get github.com/aiyang-zh/zhenyi-base
   # 或在仓库根目录
   cd zhenyi-base && go mod tidy
   ```

2. **跑通测试**
   ```bash
   go test ./zqueue/... ./zpool/... ./zerrs/... -count=1
   ```

3. **知道文档在哪**
   - 主 README：模块列表、快速开始
   - [docs/README.md](README.md)：API / 架构 / 教程索引
   - [TUTORIAL.md](TUTORIAL.md)：分场景 step-by-step
   - [ARCHITECTURE.md](ARCHITECTURE.md)：模块划分与依赖关系
   - 本地 API：`go doc -all` 或 `godoc -http=:6060`

**可选**：浏览 [README 项目结构](https://github.com/aiyang-zh/zhenyi-base#项目结构)，对全仓库目录有个印象。

---

## 四、阶段 1：零依赖基础（约 1～2 天）

**目标**：会用无锁队列做生产者-消费者、会用对象池减 GC、会用 zerrs 做错误处理。

### 1.1 zqueue（无锁队列）

**必读**：

- 主 [README 里 zqueue 小节](https://github.com/aiyang-zh/zhenyi-base#zqueue) 和 [TUTORIAL 第 2 节](TUTORIAL.md#2-使用无锁队列做生产者-消费者)
- 源码：`zqueue/mpsc.go` 或 `zqueue/spsc.go` 的导出 API（Enqueue/Dequeue/Close）

**必做**：

1. 写一个「单生产者单消费者」小程序：生产者 Enqueue 100 个数，消费者 Dequeue 并打印，用 `Close()` 正确结束。
2. 再写一个「多生产者单消费者」：多个 goroutine Enqueue，一个 goroutine Dequeue 并计数，验证总数正确。

**自测**：能说出 MPSC 与 SPSC 的适用场景（多生产者 vs 单生产者）。

### 1.2 zpool（对象池）

**必读**：

- 主 README 的 zpool 小节与 [TUTORIAL 第 3 节](TUTORIAL.md#3-使用对象池减少-gc)
- 源码：`zpool/pool.go` 的 `NewPool`、`Get`、`Put`

**必做**：

1. 用 `zpool.NewPool` 池化一个 `struct { Data []byte }`，在循环里 Get → 写 Data → Put，对比「不用池、每次 new」的 alloc 差异（可用 `go test -bench=. -benchmem` 简单对比）。
2. 阅读并运行 README 里「单独使用对象池」的示例。

**自测**：知道为什么**禁止 Put(nil)**，以及 Get 到的对象要**用完再 Put**。

### 1.3 zerrs（结构化错误）

**必读**：

- [API 文档 - zerrs](API.md#zerrs)：`New`、`Wrap`、`Is`、`As`、`IsTimeout`、`IsNetwork` 等
- 源码：`zerrs/` 下核心类型与函数

**必做**：

1. 用 `zerrs.New` / `zerrs.Newf` 创建错误，用 `zerrs.Wrap` 包一层，用 `zerrs.Is` / `zerrs.As` 做判断。
2. 写一个简单 HTTP 或 TCP 调用，用 `zerrs.IsTimeout` / `zerrs.IsNetwork` 区分超时与网络错误。

**自测**：能说出「哨兵错误」和「错误链」在本库里的用法。

---

## 五、阶段 2：接口与网络抽象（约半天）

**目标**：理解 ziface 定义的「服务 / 连接 / 消息」抽象，知道 znet 在其中的位置，不要求写 znet 代码。

**必读**：

- [ARCHITECTURE.md](ARCHITECTURE.md)：模块划分、依赖关系、网络层数据流、协议格式
- [API 文档 - ziface / znet](API.md#网络层)：IServer、IClient、IChannel、IMessage、BaseServer、BaseClient、NetMessage

**必做**：

1. 打开 `ziface` 包，浏览 `IServer`、`IClient`、`IChannel`、`IMessage` 的 godoc，在笔记里写一句「各自负责什么」。
2. 看 README 或 ARCHITECTURE 的「协议格式 (znet)」：12 字节头（msgId + seqId + dataLen），知道业务数据在 Header 之后。

**可选**：看 `znet` 的 `BaseServer` / `BaseChannel` 的启动与读循环，和 ARCHITECTURE 里的「Accept → OnRead → runSend」对照。

---

## 六、阶段 3：第一个服务（约 1 天）

**目标**：用 zserver + ztcp 写出 Echo 服务端与客户端，能改消息 ID、能加 Handle、能跑 examples。

**必读**：

- [TUTORIAL 第 1 节](TUTORIAL.md#1-快速上手3-步启动-tcp-服务) 与主 README「快速开始」里的 Echo 示例
- [TUTORIAL 第 5 节](TUTORIAL.md#5-使用-zserver-做消息路由)：Handle、Conn、SetAuthId、GetConnByAuthId
- [examples/README](../examples/README.md)：echodemo / echobench 的用法

**必做**：

1. **最小 Echo**
   - 用 `zserver.New(zserver.WithAddr(":9001"))`、`Handle(1, ...)`、`req.Reply(1, req.Data())`、`Run()` 跑起服务端。
   - 用 `ztcp.NewClient("127.0.0.1:9001")` 和 `client.Request(&znet.NetMessage{MsgId: 1, Data: []byte("hello")})` 跑客户端，看到 echo 内容。

2. **跑官方示例**
   - 双终端：`go run ./examples/echodemo/server` 与 `go run ./examples/echodemo/client`，能交互输入并收到回显。
   - 可选：`./run_echo_bench.sh` 看压测结果（理解 QPS 表即可）。

3. **小扩展**
   - 增加一个 `Handle(2, ...)`，客户端发 msgId=2 时服务端返回固定字符串。
   - 阅读 [TUTORIAL 第 7 节](TUTORIAL.md#7-选择传输协议)，知道内网用 ztcp、浏览器用 zws、弱网用 zkcp 即可，暂不实现。

**自测**：能独立从零写一个「监听 :9001、两个 msgId、Echo + 固定回复」的 server，并用 ztcp client 请求成功。

---

## 七、阶段 4：按需选学（长期）

在完成阶段 1～3 后，按项目需要选学，不必全学。

| 方向 | 模块 | 建议阅读 |
|------|------|----------|
| 日志 | zlog | TUTORIAL 第 4 节、zlog godoc，注意 `Recover` 与异步 Writer |
| WebSocket | zws | 与 ztcp 接口一致，换 `zws.NewServer`/`NewClient`，examples 可选 echobench |
| 弱网 / UDP | zkcp | README/API，了解 KCP 适用场景 |
| 批处理 | zbatch | 高吞吐聚合场景，API 文档 + 源码注释 |
| 并发集合 | zcoll | Set、ShardMap，API 文档 |
| 错误与重试 | zbackoff | 退避策略，无锁场景适用 |
| 限流 | zlimiter | 令牌桶，API 文档 |
| 加密 | zencrypt | AES-GCM、国密等，按需查 API |
| 序列化 | zserialize | Protobuf/JSON/MsgPack，按业务选 |
| 优雅关闭 | zgrace | 信号与关闭回调，做生产部署时学 |
| 其他 | zpub/zid/zrand/ztime/ztimer/zfile | 需要时查 [API.md](API.md) 或 pkg.go.dev |

---

## 八、学习自检清单

在认为「入门完成」时，可以逐项打勾：

**Go 基础（对 Go 不熟时）**
- [ ] 能写 `go f()`、`chan`、`<-ch`、`if err != nil`，能看懂接口和结构体
- [ ] 能独立写并运行「goroutine 往 channel 发一个数、main 打印」的小程序

**zhenyi-base**
- [ ] 能解释 zqueue 的 MPSC/SPSC 区别，并写出一段生产者-消费者代码
- [ ] 能正确使用 zpool 的 Get/Put，并知道不能 Put(nil)
- [ ] 能用 zerrs 做 New/Wrap/Is/As，并能区分超时与网络错误
- [ ] 能说出 ziface 里 IServer、IClient、IChannel、IMessage 的角色
- [ ] 能独立用 zserver + ztcp 写 Echo 服务端与客户端，并增加一个 Handle
- [ ] 知道 ztcp / zws / zkcp 的适用场景（内网 / 浏览器 / 弱网）
- [ ] 会查 [docs/README](README.md)、[TUTORIAL](TUTORIAL.md)、[ARCHITECTURE](ARCHITECTURE.md)、[API](API.md) 和 pkg.go.dev

---

## 九、与 zhenyi 框架的关系

- **zhenyi-base**：基础层（MIT），网络、队列、池、日志、序列化等，**本学习方案只针对这一层**。
- **zhenyi**：应用框架层（AGPL + 双授权），基于 zhenyi-base，提供 Actor、网关、分布式、脚本引擎等。

建议：**先扎实学完 zhenyi-base 的上述路径**，再根据需求看 zhenyi 的 Exec/Entity/Group/Gate，这样网络与消息模型不会混淆。

---

## 十、文档与示例速查

| 需求 | 文档/位置 |
|------|-----------|
| 模块一览、快速开始 | 仓库根 [README.md](../README.md) |
| 分步教程 | [TUTORIAL.md](TUTORIAL.md) |
| 架构与依赖 | [ARCHITECTURE.md](ARCHITECTURE.md) |
| 各包 API 索引 | [API.md](API.md) |
| 示例列表与运行方式 | [examples/README.md](../examples/README.md) |
| Echo 交互示例 | `examples/echodemo/` |
| Echo 压测 | `examples/echobench/`、`./run_echo_bench.sh` |

---

**总结**：新手从 **zqueue → zpool → zerrs** 建立「零依赖高性能组件」直觉，再通过 **ziface → zserver/ztcp** 完成第一个 Echo 服务；之后按需选学 zlog、zws、zkcp、zbatch 等，并善用 TUTORIAL、ARCHITECTURE、API 与 examples 文档。
