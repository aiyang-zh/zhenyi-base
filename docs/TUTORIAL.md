# 教程

本文档提供常见场景的 step-by-step 教程，配合 [examples](../examples/) 食用更佳。

---

## 1. 快速上手：3 步启动 TCP 服务

```go
package main

import "github.com/aiyang-zh/zhenyi-base/zserver"

func main() {
    s := zserver.New(zserver.WithAddr(":9001"))
    s.Handle(1, func(req *zserver.Request) {
        req.Reply(1, req.Data())  // 收到什么回什么
    })
    s.Run()
}
```

- `WithAddr`：监听地址
- `Handle(msgId, handler)`：按消息 ID 路由
- `Run()`：阻塞直到收到退出信号

完整示例见 [examples/echodemo](../examples/echodemo/)。

---

## 2. 使用无锁队列做生产者-消费者

```go
package main

import "github.com/aiyang-zh/zhenyi-base/zqueue"

func main() {
    q := zqueue.NewMPSCQueue[int](1024)

    // 生产者
    go func() {
        for i := 0; i < 100; i++ {
            q.Enqueue(i)
        }
        q.Close()
    }()

    // 消费者
    for {
        v, ok := q.Dequeue()
        if !ok {
            break
        }
        println(v)
    }
}
```

- `NewMPSCQueue`：多生产者单消费者
- `NewSPSCQueue`：单生产者单消费者（更高性能）
- `Close()` 后 `Dequeue` 返回 `(zero, false)`，消费者可退出

---

## 3. 使用对象池减少 GC

```go
package main

import "github.com/aiyang-zh/zhenyi-base/zpool"

type MyStruct struct {
    Data []byte
}

func main() {
    pool := zpool.NewPool(func() *MyStruct {
        return &MyStruct{Data: make([]byte, 0, 1024)}
    })

    obj := pool.Get()
    defer pool.Put(obj)
    obj.Data = append(obj.Data[:0], "hello"...)
    // 使用 obj...
}
```

- `NewPool(f)`：`f` 在池空时创建新对象
- `Get`/`Put`：获取与归还，**禁止 Put(nil)**

---

## 4. 集成日志

```go
package main

import (
    "github.com/aiyang-zh/zhenyi-base/zlog"
    "go.uber.org/zap"
)

func main() {
    // 方式一：使用默认 logger（需先初始化）
    zlog.NewDefaultLogger(zlog.WithLevel(zlog.InfoLevel))
    zlog.Info("hello", zap.String("key", "value"))

    // 方式二：创建独立 logger
    logger := zlog.NewLogger(zlog.WithLevel(zlog.DebugLevel))
    logger.Info("debug mode")
}
```

- goroutine 中务必 `defer zlog.Recover("label")` 捕获 panic
- 详见 [zlog 包文档](https://pkg.go.dev/github.com/aiyang-zh/zhenyi-base/zlog)

---

## 5. 使用 zserver 做消息路由

```go
s := zserver.New(
    zserver.WithAddr(":9001"),
    zserver.WithWorkers(4),           // 工作协程数
    zserver.WithMaxConnections(1000), // 最大连接数
)

s.Handle(1, func(req *zserver.Request) {
    conn := req.Conn()
    conn.SetAuthId(12345)  // 绑定用户 ID
    req.Reply(1, req.Data())
})

s.Handle(2, func(req *zserver.Request) {
    // 通过 AuthId 查找其他连接
    other := s.GetConnByAuthId(99999)
    if other != nil {
        other.Send(2, []byte("push"))
    }
})

s.Run()
```

- `WithWorkers`：OnRead 回调的并发度
- `SetAuthId` / `GetConnByAuthId`：按业务 ID 管理连接

---

## 6. 使用 zreactor（仅 Linux，TCP 单循环）

zreactor 用 epoll 单循环驱动所有连接的读，不每连接起 goroutine，适合高连接数、Linux 部署。**仅 Linux 可用**，且当前仅 TCP（不支持 TLS）。

### 通过 zserver 使用（推荐，3 步与现有 API 一致）

在 zserver 上打开 reactor 模式：`WithReactorMode()`，仅 TCP、且未配 TLS 时在 Linux 下生效；非 Linux 自动回退为普通每连接一 goroutine。

```go
import "github.com/aiyang-zh/zhenyi-base/zserver"
import "github.com/aiyang-zh/zhenyi-base/zreactor"

s := zserver.New(
    zserver.WithAddr(":9001"),
    zserver.WithReactorMode(), // 仅 Linux + TCP 时用 epoll 单循环
)
s.Handle(1, func(req *zserver.Request) { req.Reply(1, req.Data()) })
s.SetReactorMetrics(&zreactor.Metrics{OnAccept: fn, OnClose: fn, ...}) // 可选
s.Run() // 阻塞直到 SIGINT/SIGTERM
```

### 通过 ztcp 使用

与普通 `Server(ctx)` 二选一：用 `ServerReactor(ctx)` 启动，阻塞直到 ctx 取消。

```go
package main

import (
    "context"
    "github.com/aiyang-zh/zhenyi-base/ziface"
    "github.com/aiyang-zh/zhenyi-base/znet"
    "github.com/aiyang-zh/zhenyi-base/zreactor"
    "github.com/aiyang-zh/zhenyi-base/ztcp"
)

func main() {
    handlers := znet.ServerHandlers{
        OnAccept: func(ch ziface.IChannel) bool { return true },
        OnRead:   func(ch ziface.IChannel, msg ziface.IWireMessage) { /* 处理 msg */ },
    }
    ser := ztcp.NewServer(":9001", handlers)

    // 可选：注入 reactor 监控回调（连接建立/关闭/读错误等）
    ser.SetReactorMetrics(&zreactor.Metrics{
        OnAccept: func() { /* 连接建立 */ },
        OnClose:  func() { /* 连接关闭 */ },
        OnReadErr: func(fd int, err error) { /* 读错误 */ },
    })

    ctx := context.Background() // 实际应用里用带取消的 ctx，便于优雅退出
    ser.ServerReactor(ctx)      // 阻塞，ctx 取消后返回
}
```

- `ServerReactor` 内部会 `net.Listen`、再调 `zreactor.Serve`，listener 与连接在退出时由 ztcp 关闭。
- 不调用 `SetReactorMetrics` 时传 `nil`，不埋点。

### 直接使用 zreactor 包

需要自己创建 `*net.TCPListener`，并实现 `zreactor.AcceptFunc`，返回实现 `zreactor.ReactorChannel` 的 channel（如 `*znet.BaseChannel` 通过 ztcp.NewChannel 得到）。示例见 [zreactor 包文档](https://pkg.go.dev/github.com/aiyang-zh/zhenyi-base/zreactor)：Serve 返回后需自行 `listener.Close()`。

---

## 7. 选择传输协议

| 场景       | 推荐     | 说明                     |
|------------|----------|--------------------------|
| 内网/低延迟 | ztcp     | 原生 TCP，性能最高       |
| 浏览器/跨域 | zws      | WebSocket                |
| 弱网/丢包  | zkcp     | KCP 可靠 UDP             |

底层均基于 znet，接口一致，切换协议只需改 `NewServer` 的包名。Linux 下 TCP 还可选 ztcp.ServerReactor（见上一节）。

---

## 下一步

- [API 文档](API.md)
- [架构设计](ARCHITECTURE.md)
- [示例代码](../examples/)
