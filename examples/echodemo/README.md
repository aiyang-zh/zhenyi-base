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
s := zserver.New(
	zserver.WithAddr(":9001"),
	zserver.WithName("echodemo/server"),
)
s.Handle(1, func(req *zserver.Request) {
	req.Reply(1, req.Data())
})
s.Run()
```

客户端：`fmt.Print(zbrand.Banner)`；stdin 行输入回显，Ctrl+D 退出。

## 客户端代码（默认 Request 模式）

```go
client, _ := ztcp.NewClient("127.0.0.1:9001")
scanner := bufio.NewScanner(os.Stdin)
for scanner.Scan() {
    line := scanner.Bytes()
    if len(line) == 0 {
        continue
    }
    resp, err := client.Request(&znet.NetMessage{MsgId: 1, Data: bytes.Clone(line)})
    if err != nil {
        break
    }
    fmt.Printf("echo: %s\n", string(resp.GetMessageData()))
}
```
