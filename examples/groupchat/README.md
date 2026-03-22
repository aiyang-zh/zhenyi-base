# 群聊示例（内存 + 自带网页）

单房间、无持久化；协议为 znet **v0**：`msgId(4) + seqId(4) + dataLen(4) + data`（大端）。

| msgId | 方向 | 说明 |
|:---:|:---:|------|
| 1 | 客户端→服务端 | 加入，body UTF-8 昵称（≤24） |
| 2 | 客户端→服务端 | 发言，body UTF-8 文本（≤512） |
| 10 | 服务端→客户端 | JSON：`{"type":"join|leave|say","user":"…","text":"…"}` |
| 99 | 服务端→客户端 | 错误提示 UTF-8 |

## 运行

```bash
go run ./examples/groupchat/server
```

默认 **HTTP** `:8080`（静态页）、**WebSocket** `:9001`。浏览器打开终端提示的 `http://127.0.0.1:8080`，按页面填写 WS 端口（默认 9001）与昵称。

```bash
go run ./examples/groupchat/server -http :8080 -addr :9001
```

## 依赖

`zserver`（WebSocket）、`znet`；静态页由 `embed` 打进二进制。
