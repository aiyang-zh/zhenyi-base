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

## 6. 选择传输协议

| 场景       | 推荐     | 说明                     |
|------------|----------|--------------------------|
| 内网/低延迟 | ztcp     | 原生 TCP，性能最高       |
| 浏览器/跨域 | zws      | WebSocket                |
| 弱网/丢包  | zkcp     | KCP 可靠 UDP             |

底层均基于 znet，接口一致，切换协议只需改 `NewServer` 的包名。

---

## 下一步

- [API 文档](API.md)
- [架构设计](ARCHITECTURE.md)
- [示例代码](../examples/)
