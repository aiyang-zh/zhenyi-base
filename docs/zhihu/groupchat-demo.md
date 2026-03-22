# 用 zhenyi-base 做一个带网页的群聊 Demo：一条命令跑起来

*一条命令起服务，浏览器打开就能群聊。服务端 zserver + WebSocket，前端 embed 打包进二进制。*

---

## 写在前面的废话

**zhenyi-base** 是一套 Go 网络与基础组件库（TCP / WebSocket / 无锁队列等），轻量、按包引入。官网：https://zhenyi-site.pages.dev/ ，GitHub：https://github.com/aiyang-zh/zhenyi-base 。

之前发过整体介绍和 zqueue 深挖，这篇用它的 **groupchat** 示例：一个带网页的群聊 Demo，前后端都在一个二进制里，改得动、能扩展，适合做联调或二次开发。

---

## 一、跑起来看看

先克隆仓库，进入示例目录：

```bash
git clone https://github.com/aiyang-zh/zhenyi-base.git
cd zhenyi-base
go run ./examples/groupchat/server
```

终端会输出类似：

```
   #####  #   #  #####  ...
  [zhenyi-base] examples/groupchat | WebSocket | direct dispatch
[examples/groupchat] server listening on :9001 (WebSocket, direct dispatch)
[groupchat] open http://127.0.0.1:8080 (WS ws://127.0.0.1:9001)
```

浏览器打开 **http://127.0.0.1:8080**，填昵称点「连接」，多开几个标签页即可群聊。

---

## 二、整体架构

| 端口 | 用途 | 技术 |
|------|------|------|
| **:8080** | 静态 HTML 页面 | `net/http` + `embed` |
| **:9001** | WebSocket 群聊 | `zserver` + `znet.WebSocket` |

网页通过 `embed` 打进二进制，无需单独部署前端；WebSocket 走 znet v0 协议（12 字节头 + body）；单房间、纯内存，刷新即清空，适合 Demo 和联调。

---

## 三、协议约定

线协议为 znet **v0**：`msgId(4) + seqId(4) + dataLen(4) + data`，大端。

| msgId | 方向 | 说明 |
|:---:|------|------|
| 1 | 客户端 → 服务端 | 加入，body 为 UTF-8 昵称（≤24 字节） |
| 2 | 客户端 → 服务端 | 发言，body 为 UTF-8 文本（≤512 字节） |
| 10 | 服务端 → 客户端 | 广播事件，body 为 JSON：`{"type":"join|leave|say","user":"…","text":"…"}` |
| 99 | 服务端 → 客户端 | 错误提示，body 为 UTF-8 文本 |

---

## 四、服务端核心代码

### 4.1 启动与路由

```go
s := zserver.New(
    zserver.WithAddr(*wsAddr),
    zserver.WithProtocol(znet.WebSocket),
    zserver.WithName("examples/groupchat"),
    zserver.WithAsyncMode(), // 广播需 Send 入队，sync 模式下会丢弃
)

s.OnConnect(func(c *zserver.Conn) {
    mu.Lock()
    conns[c.Id()] = c
    mu.Unlock()
})

s.OnDisconnect(func(c *zserver.Conn) {
    mu.Lock()
    nick := nicks[c.Id()]
    delete(nicks, c.Id())
    delete(conns, c.Id())
    others := make([]*zserver.Conn, 0, len(conns))
    for _, x := range conns { others = append(others, x) }
    mu.Unlock()
    if nick != "" {
        b, _ := json.Marshal(chatEvent{Type: "leave", User: nick})
        for _, x := range others { x.Send(msgEvt, b) }
    }
})

s.Handle(msgJoin, func(req *zserver.Request) { /* 见下 */ })
s.Handle(msgChat, func(req *zserver.Request) { /* 见下 */ })
```

要点：`conns` map 保存所有连接，`nicks` 保存昵称；OnDisconnect 时向剩余用户广播 leave。群聊需要 `c.Send()` 广播，必须加 `WithAsyncMode()`，否则 sync 模式下无发送队列，`Send` 会直接丢弃。

### 4.2 加入与发言

```go
s.Handle(msgJoin, func(req *zserver.Request) {
    nick := strings.TrimSpace(string(req.Data()))
    if nick == "" || len(nick) > 24 {
        req.Reply(msgErr, []byte("invalid nick"))
        return
    }
    id := req.Conn().Id()
    mu.Lock()
    if nicks[id] != "" {
        mu.Unlock()
        req.Reply(msgErr, []byte("already joined"))
        return
    }
    nicks[id] = nick
    mu.Unlock()
    broadcast(conns, &mu, msgEvt, chatEvent{Type: "join", User: nick})
})

s.Handle(msgChat, func(req *zserver.Request) {
    text := strings.TrimSpace(string(req.Data()))
    if text == "" || len(text) > 512 {
        req.Reply(msgErr, []byte("invalid text"))
        return
    }
    mu.Lock()
    nick := nicks[req.Conn().Id()]
    mu.Unlock()
    if nick == "" {
        req.Reply(msgErr, []byte("join first"))
        return
    }
    broadcast(conns, &mu, msgEvt, chatEvent{Type: "say", User: nick, Text: text})
})
```

### 4.3 广播与静态页

```go
func broadcast(conns map[uint64]*zserver.Conn, mu *sync.Mutex, msgID int32, ev chatEvent) {
    b, _ := json.Marshal(ev)
    mu.Lock()
    out := make([]*zserver.Conn, 0, len(conns))
    for _, c := range conns { out = append(out, c) }
    mu.Unlock()
    for _, c := range out { c.Send(msgID, b) }
}

//go:embed web/*
var webFS embed.FS

sub, _ := fs.Sub(webFS, "web")
go func() {
    http.ListenAndServe(*httpAddr, http.FileServer(http.FS(sub)))
}()
```

---

## 五、前端：组包与解析

浏览器用原生 `WebSocket`，发送二进制帧，需按 znet v0 格式组包：

```javascript
function pack(msgId, bodyUtf8) {
  const body = new TextEncoder().encode(bodyUtf8);
  const buf = new ArrayBuffer(12 + body.length);
  const dv = new DataView(buf);
  dv.setInt32(0, msgId, false);   // 大端
  dv.setUint32(4, (++seq) >>> 0, false);
  dv.setUint32(8, body.length, false);
  new Uint8Array(buf, 12).set(body);
  return buf;
}
```

连接成功后先发 join，再发 chat；收包时按 12 字节头解析 msgId 和 body，MSG_EVT 的 payload 是 JSON，解析后渲染即可。完整代码见 `examples/groupchat/server/web/index.html`。

---

## 六、扩展与自定义

改端口：`go run ./examples/groupchat/server -http :3000 -addr :9002`，网页里把 WS 端口改成 9002 即可。

单独部署前端：把 `web/` 目录拷出去，用 nginx 或任意静态服务器挂载，浏览器连你自己的 HTTP 地址；WS 地址填服务端的 `-addr` 端口。

加房间、持久化：在现有 `conns` / `nicks` 上按 room 分桶，或接入数据库，逻辑类似，按需扩展即可。

---

## 七、总结

- 一个二进制，同时提供 HTTP 静态页和 WebSocket 服务
- 协议简单：4 个 MsgID，JSON 事件体
- 群聊需加 `WithAsyncMode()` 开启异步发送，否则广播会丢包

示例代码：https://github.com/aiyang-zh/zhenyi-base/tree/main/examples/groupchat ，更多文档与示例见 [官网](https://zhenyi-site.pages.dev/)。
