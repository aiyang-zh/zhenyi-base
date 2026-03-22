<div align="center">

<img src="assets/logo.png" alt="Zhenyi" width="120" />

# zhenyi-base

**高性能 Go 网络与基础工具库**

*MIT 协议 · 零分配 · 无锁队列 · 开箱即用*

[![Tests](https://github.com/aiyang-zh/zhenyi-base/actions/workflows/test.yml/badge.svg)](https://github.com/aiyang-zh/zhenyi-base/actions)
[![Go Reference](https://pkg.go.dev/badge/github.com/aiyang-zh/zhenyi-base.svg)](https://pkg.go.dev/github.com/aiyang-zh/zhenyi-base)
[![Go Report Card](https://goreportcard.com/badge/github.com/aiyang-zh/zhenyi-base?style=flat-square)](https://goreportcard.com/report/github.com/aiyang-zh/zhenyi-base)
[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat-square&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg?style=flat-square)](LICENSE)
[![GitHub Stars](https://img.shields.io/github/stars/aiyang-zh/zhenyi-base?style=flat-square)](https://github.com/aiyang-zh/zhenyi-base/stargazers)

![Zero Allocation](https://img.shields.io/badge/Zero%20Allocation-yes-brightgreen?style=flat-square)
![Lock-Free Queue](https://img.shields.io/badge/Lock--Free-Queue-orange?style=flat-square)
![QPS](https://img.shields.io/badge/QPS-770k+-brightgreen?style=flat-square)
![Latency](https://img.shields.io/badge/Latency-16ns-orange?style=flat-square)

**[官网](https://zhenyi-site.pages.dev/)** · [GitHub](https://github.com/aiyang-zh/zhenyi-base)

</div>

---

## 简介

zhenyi-base 是 Zhenyi 生态的 **MIT 协议** 基础层，提供生产级的网络通信、无锁队列、对象池、自适应批处理、加密、日志、序列化等核心组件。

按需引入，只编译你用到的包——用 `zqueue` 不会拉入 zap，用 `zpool` 不会拉入 websocket。

> 基于 zhenyi-base，更多高阶能力正在路上 →

**文档**：[API 文档](docs/API.md) · [架构设计](docs/ARCHITECTURE.md) · [教程](docs/TUTORIAL.md) · [文档总览](docs/README.md)

---

## 核心模块

### 网络与服务

| 包 | 说明 | 亮点 |
|------|------|------|
| **znet** | 网络核心层 | Ring Buffer 零拷贝、writev 批量发送 |
| **ztcp** | TCP 传输 | 高性能 TCP 连接管理 |
| **zws** | WebSocket 传输 | 基于 gorilla/websocket |
| **zkcp** | KCP 传输 | 可靠 UDP，适合弱网环境 |
| **zserver** | 轻量服务器 | 3 步启动，内置连接管理与优雅关闭 |
| **ziface** | 接口抽象 | IServer / IChannel / IClient / IMessage |

### 高性能数据结构

| 包 | 说明 | 亮点 |
|------|------|------|
| **zqueue** | 无锁泛型队列 | MPSC / SPSC / Priority / Swap，节点池化 + Shrink 缩容 |
| **zpool** | 泛型对象池 | `Pool[T]`、Buffer 字节池、网络缓冲池 |
| **zbatch** | 自适应批处理 | 延迟反馈、动态步长 |
| **zcoll** | 并发集合 | 泛型 Set、ShardMap（分片并发 Map） |

### 基础工具

| 包 | 说明 | 亮点 |
|------|------|------|
| **zerrs** | 结构化错误 | 带堆栈，`pkg/errors` 兼容 |
| **zlog** | 高性能日志 | 基于 Zap，异步 Writer + 自动轮转 |
| **zencrypt** | 加密工具库 | AES-GCM / RSA / XTEA / 国密 SM2/SM3/SM4，零拷贝加密 |
| **zserialize** | 多格式序列化 | Protobuf / JSON(sonic) / MsgPack（vmihailenco/msgpack/v5） |
| **zbrand** | 展示常量 | `Banner`（ASCII） |
| **zbackoff** | 退避策略 | 三级退避，CPU PAUSE 指令优化，适用于无锁场景 |
| **zlimiter** | 令牌桶限流 | 通用限流组件 |
| **zpub** | 进程内事件总线 | 发布-订阅模式 |
| **zid** | ID 生成器 | Snowflake / UUID / FastID（轻量零依赖） |
| **zgrace** | 优雅关闭 | 信号监听 + 关闭回调管理 |
| **zrand** | 泛型随机数 | 轻量随机数工具 |
| **ztime** | 时间工具 | 时间格式化与管理 |
| **ztimer** | 定时器 | Ticker 封装 |
| **zfile** | 文件工具 | 文件操作辅助 |

---

## 性能指标

> **测试环境**: darwin/arm64 · Apple M3 · Go 1.24+
>
> **测试覆盖率**: 核心包 85%+，详见下表（统计不包含 `examples/`、`ziface/`）

### Network Echo 压测

> 运行 `make bench`（或 `./scripts/run_echo_bench.sh`），测试环境 darwin/arm64 · Apple M3 · Go 1.24+

| 协议 | 消息/连接 | QPS (msg/s) |
|------|:---:|:---:|
| **TCP** | 23B/20c | **715,147** |
| **TCP** | 1KB/20c | 401,744 |
| **TCP** | 23B/100c | 696,507 |
| **TCP** | 23B/1000c | **778,365** |
| **TCP** | 1KB/1000c | 257,721 |
| **WebSocket** | 23B/20c | 657,616 |
| **WebSocket** | 1KB/20c | **675,725** |
| **WebSocket** | 23B/100c | 546,951 |
| **WebSocket** | 23B/1000c | **715,972** |
| **WebSocket** | 1KB/1000c | 160,539 |
| **KCP** | 23B/20c | 33,332 |
| **KCP** | 1KB/20c | 31,604 |
| **KCP** | 23B/100c | **50,940** |
| **KCP** | 23B/1000c | 27,419 |
| **KCP** | 1KB/1000c | 11,715 |

### 核心组件基准

| 组件 | 指标 | 结果 |
|:---:|:---:|:---|
| **MPSC 无锁队列** | Dequeue 单次 | **16.7 ns/op**，0 allocs |
| **SPSC 无锁队列** | Dequeue 单次 | **15.2 ns/op**，0 allocs |
| **对象池 Get/Put** | 单线程 | **7.9 ns/op**，0 allocs |
| **自适应批处理** | FastPath 单次 | **1.4 ns/op**，0 allocs |
| **错误哨兵匹配** | Sentinel 匹配 | **0.41 ns/op**，0 allocs |
| **日志并发吞吐** | AsyncWriter | **821 MB/s** |

### 测试覆盖率

| 包 | 覆盖率 | 包 | 覆盖率 |
|:---:|:---:|:---:|:---:|
| zbackoff | **100%** | zpool | **100%** |
| zgrace | **100%** | zlimiter | **100%** |
| zerrs | **97.8%** | ztime | **97.4%** |
| zbatch | **96.1%** | zqueue | **95.1%** |
| zfile | **94.4%** | zpub | **93.5%** |
| ztcp | **91.2%** | ztimer | **92.9%** |
| zencrypt | **90.9%** | zkcp | **90.0%** |
| zrand | **89.5%** | zws | **85.9%** |
| znet | **77.8%** | zserialize | **76.0%** |
| zcoll | **72.8%** | zid | **66.7%** |
| zlog | **62.6%** | zserver | **60.5%** |

---

## 快速开始

### 安装

```bash
go get github.com/aiyang-zh/zhenyi-base
```

### Echo 服务器（3 步启动）

```go
package main

import (
    "fmt"
    "github.com/aiyang-zh/zhenyi-base/zserver"
)

func main() {
    s := zserver.New(zserver.WithAddr(":9001"))

    s.Handle(1, func(req *zserver.Request) {
        fmt.Printf("收到: %s\n", string(req.Data()))
        req.Reply(1, req.Data())
    })

    s.Run()
}
```

### Echo 客户端（默认 Request 模式）

```go
client, _ := ztcp.NewClient("127.0.0.1:9001")
resp, err := client.Request(&znet.NetMessage{MsgId: 1, Data: []byte("hello")})
if err == nil {
    fmt.Printf("echo: %s\n", string(resp.GetMessageData()))
}
// 流式收包需用 ztcp.NewClient(addr, znet.WithAsyncMode())，见 examples/echobench
```

### 单独使用无锁队列（零外部依赖）

```go
package main

import "github.com/aiyang-zh/zhenyi-base/zqueue"

func main() {
    q := zqueue.NewMPSCQueue[int](1024)
    q.Enqueue(42)
    val, ok := q.Dequeue()
    // val == 42, ok == true
}
```

### 单独使用对象池（零外部依赖）

```go
package main

import "github.com/aiyang-zh/zhenyi-base/zpool"

func main() {
    pool := zpool.NewPool(func() *MyObject {
        return &MyObject{}
    })
    obj := pool.Get()
    defer pool.Put(obj)
}
```

**示例**：极简 Echo 见 [examples/echodemo](examples/echodemo)，压测见 [examples/echobench](examples/echobench)（运行 `make bench`）。

---

## 依赖说明

zhenyi-base 的每个包独立设计，按需引入。Go 的构建系统只编译你实际 import 的包：

| 你 import 的包 | 实际拉入的外部依赖 |
|---------------|------------------|
| `zqueue`、`zpool`、`zbatch`、`zbackoff`、`zerrs`、`zrand` | **无**（零外部依赖） |
| `zlog` | zap |
| `zencrypt` | crypto、gmsm |
| `zserialize` | sonic、protobuf、msgpack |
| `znet` / `ztcp` | 以上 + 内部工具包 |
| `zws` | + gorilla/websocket |
| `zkcp` | + kcp-go |

---

## 项目结构

```
zhenyi-base/
├── znet/          # 网络核心（Ring Buffer + 零拷贝）
├── ztcp/          # TCP 传输（含 ServerReactor，仅 Linux）
├── zws/           # WebSocket 传输
├── zkcp/          # KCP 传输
├── zreactor/      # Linux epoll reactor 循环（可选，供 ztcp.ServerReactor）
├── zserver/       # 轻量服务器
├── ziface/        # 核心接口定义
├── zqueue/        # 无锁泛型队列（MPSC/SPSC/Priority/Swap）
├── zpool/         # 泛型对象池
├── zbatch/        # 自适应批处理引擎
├── zcoll/         # 并发集合（Set/ShardMap）
├── zerrs/         # 结构化错误处理
├── zlog/          # 高性能异步日志
├── zencrypt/      # 加密工具库（AES/RSA/XTEA/国密）
├── zserialize/    # 多格式序列化
├── zbackoff/      # 退避策略
├── zlimiter/      # 令牌桶限流
├── zpub/          # 进程内事件总线
├── zid/           # ID 生成器
├── zgrace/        # 优雅关闭管理
├── zrand/         # 泛型随机数
├── ztime/         # 时间工具
├── ztimer/        # 定时器
├── zfile/         # 文件工具
├── zbrand/        # 展示常量（如启动 Banner）
└── examples/      # 示例代码（含 groupchat 内存群聊 + 网页）
```

---

## 协议

本项目采用 [MIT License](LICENSE)，可自由使用、修改、分发、商用，无需公开源代码。

---

## 贡献

欢迎参与！请查看 [CONTRIBUTING.md](CONTRIBUTING.md)。

**维护者**：1093993119@qq.com

---

<div align="center">

**如果 zhenyi-base 对你有帮助，请给我们一颗 Star！**

</div>
