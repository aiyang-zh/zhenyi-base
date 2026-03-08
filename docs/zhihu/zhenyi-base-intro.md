# 开源一个 Go 高性能基础库：77万 QPS、16ns 无锁队列

---

## 写在前面的废话

做过长连接、网关、高吞吐服务的同学，多少有过这种经历：

想用一个无锁队列，结果拉进来一堆无关依赖。想用一个网络库，配置复杂、侵入性强。多个库混用，依赖爆炸、编译体积变大、GC 不可控。

所以我做了 **zhenyi-base**——一套按包拆分、最小依赖、按需引入的 Go 高性能基础库，今天正式发出来给大家试用、交流。

---

## 一、设计思路：按需引入、零冗余

核心思路就一句话：**只编译你真正用到的代码。**

- 你用 `zqueue` 就只引队列，不会带上 websocket  
- 你用 `ztcp` 就只引 TCP，不会带上日志库  
- 每个包独立设计，Go 的构建系统只编译你 import 的包  

干净、轻量、高性能。

---

## 二、核心能力一览

zhenyi-base 不是单一组件，而是一整套高性能底层工具集，所有包均可独立使用。

**网络层**：ztcp（TCP）、zws（WebSocket）、zkcp（KCP），Ring Buffer 零拷贝、writev 批量发送，高并发、低延迟、低 GC。

**数据结构**：SPSC/MPSC 无锁队列、泛型对象池、自适应批处理、分片 Map，零分配设计。

**工具集**：结构化错误、异步日志、国密 SM2/SM3/SM4、序列化、限流、事件总线。

---

## 三、适用场景

- **长连接网关**：消息转发、会话保持 → TCP / WebSocket
- **游戏 / 实时应用**：游戏服务器、实时对战、状态同步 → TCP / KCP / WebSocket
- **实时推送**：推送、直播、协同 → WebSocket
- **高吞吐 API**：内网服务、低延迟 → TCP
- **弱网环境**：丢包、抖动、移动网络 → KCP
- **队列/池化**：任务队列、连接池、对象复用 → 无锁队列 / 对象池

同一套业务，`zserver.WithProtocol()` 即可在 TCP / WebSocket / KCP 间切换，零修改。

---

## 四、压测数据

测试环境：Go 1.24.0，darwin/arm64 (M3)。

> 本地基准测试结果，仅供参考。完整性能图表见 [官网](https://zhenyi-site.pages.dev/) 性能基准板块。

- TCP Echo 23B/1000 连接：**77.8 万 msg/s**
- WebSocket Echo 23B/1000 连接：**71.6 万 msg/s**
- MPSC 队列 Dequeue：**16.7 ns/op**，0 allocs
- 对象池 Get/Put：**7.9 ns/op**，0 allocs

核心包测试覆盖率 85%+，欢迎在测试环境充分压测、反馈。

---

## 五、快速上手

```bash
go get github.com/aiyang-zh/zhenyi-base
```

按包单独引入：

```bash
go get github.com/aiyang-zh/zhenyi-base/zqueue   # 无锁队列，零依赖
go get github.com/aiyang-zh/zhenyi-base/ztcp     # TCP 网络
```

**TCP Echo 服务 3 步启动（默认 TCP，需 WebSocket/KCP 可用 `zserver.WithProtocol()` 切换）：**

```go
s := zserver.New(zserver.WithAddr(":9001"))
s.Handle(1, func(req *zserver.Request) { req.Reply(1, req.Data()) })
s.Run()
```

更多示例见 [官网](https://zhenyi-site.pages.dev/)。

---

## 六、项目特点

- MIT 协议，完全开源商用  
- 支持 TCP / KCP / WebSocket 无损切换，业务代码零修改  
- 模块化设计，按需引入，不拖冗余依赖  
- 零分配 / 低分配，高吞吐、低延迟、低 GC  
- 无锁队列、高性能网络、泛型池一站式配齐  
- 不捆绑任何中间件、日志库、配置框架  

---

## 七、社区与反馈

项目处于开源初期，非常需要实测数据、使用反馈、bug 报告与优化建议。所有落地案例会在官网长期收集并公开署名，欢迎共建。

如果对你有帮助，欢迎点赞、评论、分享！示例与性能图表见 [官网](https://zhenyi-site.pages.dev/)。
