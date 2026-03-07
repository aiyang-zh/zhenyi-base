# Echo Demo

最简单的 zhenyi-base 示例：服务端收到什么回什么。

## 启动

```bash
# 终端 1：启动服务端
go run ./examples/echodemo/server

# 终端 2：运行客户端
go run ./examples/echodemo/client
```

## 服务端代码（3 步）

```go
s := zserver.New(zserver.WithAddr(":9001"))
s.Handle(1, func(req *zserver.Request) {
    req.Reply(1, req.Data())
})
s.Run()
```

客户端支持**交互式输入**：输入任意内容，服务端原样回显，Ctrl+D 退出。

## 客户端代码

```go
client, _ := ztcp.NewClient("127.0.0.1:9001")
client.SetReadCall(func(msg ziface.IWireMessage) {
    fmt.Printf("echo: %s\n", string(msg.GetMessageData()))
})
go client.Read()

scanner := bufio.NewScanner(os.Stdin)
for scanner.Scan() {
    line := scanner.Bytes()
    if len(line) > 0 {
        client.SendMsg(&znet.NetMessage{MsgId: 1, Data: bytes.Clone(line)})
    }
}
```
