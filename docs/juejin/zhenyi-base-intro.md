# zhenyi-base 开源 | Go 高性能基础库：TCP 77万 QPS，无锁队列 16ns/op，按需引入

**标签**：Go、高性能、网络编程、开源、后端、实时应用

---

大家好，最近我开源了一套 **Go 高性能基础库 zhenyi-base**，定位是轻量、无依赖、可独立引入的网络与基础组件库，今天正式发出来给大家试用、交流。

---

## 一、项目背景 & 设计思路

做过长连接、网关、高吞吐服务的同学应该都有同感：

- 想用一个无锁队列，结果拉进来一堆无关依赖  
- 想用一个网络库，配置复杂、侵入性强  
- 多个库混用，导致依赖爆炸、编译体积变大、GC 不可控  

所以我做了 **zhenyi-base**，核心思路非常简单：

> **按包拆分、最小依赖、按需引入、零冗余。**

- 你用 `zqueue` 就只引队列，不会带上 websocket  
- 你用 `ztcp` 就只引 TCP，不会带上日志库  
- **只编译你真正用到的代码**，干净、轻量、高性能  

---

## 二、核心能力一览

zhenyi-base 不是单一组件，而是一整套高性能底层工具集，**所有包均可独立使用**。

### 1. 网络层（高性能 IO）

- **ztcp**：高性能 TCP 实现  
- **zws**：WebSocket 支持  
- **zkcp**：KCP 支持（弱网场景）  
- Ring Buffer 零拷贝、writev 批量发送  
- 高并发、低延迟、低 GC  

### 2. 数据结构（无锁 / 零分配 / 泛型）

- SPSC / MPSC 有界 / 无界队列  
- 泛型对象池  
- 自适应批处理  
- 分片 Map  

### 3. 工具集（生产级）

- 结构化错误、异步日志  
- 国密支持（SM2 / SM3 / SM4）  
- 序列化、限流、事件总线  

---

## 三、适用场景

- **长连接网关**：消息转发、会话保持 → TCP / WebSocket
- **游戏 / 实时应用**：游戏服务器、实时对战 → TCP / KCP / WebSocket
- **实时推送**：推送、直播、协同编辑 → WebSocket
- **高吞吐 API**：内网服务、低延迟 → TCP
- **弱网环境**：丢包、移动网络 → KCP
- **队列 / 池化**：任务队列、连接池、对象复用 → 无锁队列 / 对象池

同一套业务，`zserver.WithProtocol()` 即可切换协议，零修改。

---

## 四、压测数据

测试环境：Go 1.24.0，darwin/arm64 (M3)。本地基准测试，仅供参考。

- TCP Echo 23B/1000 连接：**77.8 万 msg/s**
- WebSocket Echo 23B/1000 连接：**71.6 万 msg/s**
- MPSC 队列 Dequeue：**16.7 ns/op**，0 allocs
- 对象池 Get/Put：**7.9 ns/op**，0 allocs

项目目前已完成单元测试，核心包覆盖率 85%+，但未经过大规模线上流量验证，欢迎大家在测试环境充分压测、反馈。

---

## 五、快速上手

### 安装

```bash
go get github.com/aiyang-zh/zhenyi-base
```

按包单独引入也可以：

```bash
# 只使用无锁队列（零外部依赖）
go get github.com/aiyang-zh/zhenyi-base/zqueue

# 只使用 TCP 网络
go get github.com/aiyang-zh/zhenyi-base/ztcp
```

---

## 六、上手实例

下面是三个可直接复制运行的示例，分别对应：Echo 服务、无锁队列、对象池。

### 示例 1：TCP Echo 服务器（3 步启动）

以下为 TCP 协议，默认监听 `:9001`。若需 WebSocket 或 KCP，可用 `zserver.WithProtocol()` 切换。

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

`go run main.go` 启动服务端后，另开终端执行 `go run ./examples/echodemo/client/main.go` 即可验证（输入内容会原样回显）。

---

### 示例 2：无锁队列（零外部依赖）

```go
package main

import "github.com/aiyang-zh/zhenyi-base/zqueue"

func main() {
	q := zqueue.NewMPSCQueue[int](1024)
	q.Enqueue(42)
	q.Enqueue(100)

	val, ok := q.Dequeue()
	// val == 42, ok == true

	val2, ok2 := q.Dequeue()
	// val2 == 100, ok2 == true
}
```

`zqueue` 不拉入任何第三方依赖，MPSC/SPSC/Priority 多种队列按需选择。

---

### 示例 3：泛型对象池（零外部依赖）

```go
package main

import "github.com/aiyang-zh/zhenyi-base/zpool"

type MyObject struct {
	Data []byte
}

func main() {
	pool := zpool.NewPool(func() *MyObject {
		return &MyObject{Data: make([]byte, 0, 1024)}
	})

	obj := pool.Get()
	defer pool.Put(obj)
	obj.Data = append(obj.Data, 'H', 'i')
	// 使用完毕 Put 回池，避免 GC
}
```

更多示例见 [官网](https://zhenyi-site.pages.dev/)。

---

## 七、项目特点（一句话总结）

- **MIT 协议**，完全开源商用  
- **协议无损切换**，支持 TCP / KCP / WebSocket 无缝切换，业务代码零修改  
- **模块化设计**，按需引入，不拖冗余依赖  
- **零分配 / 低分配**，高吞吐、低延迟、低 GC  
- **无锁队列、高性能网络、泛型池** 一站式配齐  
- **不捆绑** 任何中间件、日志库、配置框架  

---

## 八、社区与反馈

目前项目处于开源初期，非常需要大家的 **实测数据、使用反馈、bug 报告与优化建议**。

所有实测压测、使用场景、落地案例，我会在官网上长期收集并公开署名，欢迎大家一起参与共建。

---

## 官网

**[https://zhenyi-site.pages.dev/](https://zhenyi-site.pages.dev/)**

快速上手示例、性能图表、一键安装、生态介绍，都在这里。欢迎访问、试用、反馈，一起把这套库做得更稳、更强。
