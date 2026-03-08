# Echo Demo

最简单的 zhenyi-base 示例：服务端收到什么回什么。

## 启动服务端

```bash
go run ./examples/echobench/server
```

输出：
```
[echo] starting server on :9001
[zhenyi] server listening on :9001 (TCP)
```

## 交互模式

```bash
go run ./examples/echobench/client
```

输入文字后回车发送，收到服务端回显：
```
connected to 127.0.0.1:9001
hello
> sent: hello
< recv msgId=1: hello
```

## 压测模式

```bash
go run ./examples/echobench/client -bench -n 100000 -c 10
```

参数：
- `-n` 总消息数（默认 10000）
- `-c` 并发客户端数（默认 1）
- `-addr` 服务端地址（默认 127.0.0.1:9001）

输出示例：
```
benchmark: 10 clients x 10000 msgs = 100000 total, payload=21 bytes

--- benchmark result ---
elapsed:  1.234s
sent:     100000
recv:     100000
qps:      81037 msg/s
latency:  0.01 ms/msg (avg)
```

## 核心代码（3 步）

```go
// 1. 创建
s := server.New(server.WithAddr(":9001"))

// 2. 路由
s.Handle(1, func(req *server.Request) {
    req.Reply(1, req.Data())
})

// 3. 启动
s.Run()
```

## 参考测试结果（Apple M3 · Go 1.24+）

运行 `./run_echo_bench.sh` 在本机得到的一组示例数据（单位：msg/s，仅供参考，具体数值会随机器和环境变化）：

```text
  [tcp] 23B/20c   QPS: 631612 msg/s
  [tcp] 1KB/20c   QPS: 608256 msg/s
  [tcp] 23B/100c  QPS: 734168 msg/s
  [tcp] 23B/1000c QPS: 746883 msg/s
  [tcp] 1KB/1000c QPS: 189504 msg/s

  [ws]  23B/20c   QPS: 523084 msg/s
  [ws]  1KB/20c   QPS: 541585 msg/s
  [ws]  23B/100c  QPS: 670934 msg/s
  [ws]  23B/1000c QPS: 679756 msg/s
  [ws]  1KB/1000c QPS: 160856 msg/s

  [kcp] 23B/20c   QPS: 35361 msg/s
  [kcp] 1KB/20c   QPS: 32522 msg/s
  [kcp] 23B/100c  QPS: 49434 msg/s
  [kcp] 23B/1000c QPS: 32488 msg/s
  [kcp] 1KB/1000c QPS: 12992 msg/s
```
