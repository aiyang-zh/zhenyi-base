# 开源一个 Go 高性能基础库：77万 QPS、16ns 无锁队列，按需引入

---

做长连接、网关、高吞吐服务时，你有没有遇到过：想用个无锁队列，结果拉进来一堆无关依赖？想用个网络库，配置复杂、侵入性强？多个库混用，依赖爆炸、GC 不可控？

我做了 **zhenyi-base**，一套按需引入、零冗余的 Go 高性能基础库，今天正式开源。

---

## 核心思路：只编译你用到的

- 用 `zqueue` 就只引队列，不带你 websocket  
- 用 `ztcp` 就只引 TCP，不带你日志库  
- 按包拆分，Go 只编译你 import 的代码  

干净、轻量、高性能。

---

## 能做什么？

**网络**：TCP、WebSocket、KCP，零拷贝、批量发送，高并发低延迟。

**数据结构**：无锁队列（MPSC/SPSC）、泛型对象池、自适应批处理、分片 Map，零分配。

**工具**：错误、日志、国密、序列化、限流、事件总线。

---

## 适用场景

长连接网关、游戏/实时应用、实时推送、高吞吐 API → TCP / WebSocket；弱网环境 → KCP。一套业务，协议可切换，零修改。

---

## 压测数据

测试环境：Go 1.24.0，darwin/arm64 (M3)。

- TCP Echo 1000 连接：**77.8 万 msg/s**
- WebSocket 1000 连接：**71.6 万 msg/s**
- 无锁队列：**16.7 ns/op**，0 分配
- 对象池：**7.9 ns/op**，0 分配

核心包覆盖率 85%+，欢迎压测反馈。

---

## 一行安装

```bash
go get github.com/aiyang-zh/zhenyi-base
```

TCP Echo 服务 3 步启动：

```go
s := zserver.New(zserver.WithAddr(":9001"))
s.Handle(1, func(req *zserver.Request) { req.Reply(1, req.Data()) })
s.Run()
```

---

## 一句话总结

MIT 协议，按需引入，零分配无锁，不捆绑任何中间件。支持 TCP / KCP / WebSocket 无损切换，业务代码零修改。

示例与性能图表见 https://zhenyi-site.pages.dev/ ，欢迎试用、反馈。
