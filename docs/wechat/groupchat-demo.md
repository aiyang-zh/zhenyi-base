# 用 zhenyi-base 做一个带网页的群聊 Demo

---

**zhenyi-base** 是一套 Go 网络与基础组件库（TCP / WebSocket / 无锁队列等），轻量、按包引入。这篇用它的 **groupchat** 示例：一条命令起服务，浏览器打开就能群聊。

官网：https://zhenyi-site.pages.dev/  
GitHub：https://github.com/aiyang-zh/zhenyi-base

---

## 一条命令跑起来

```bash
git clone https://github.com/aiyang-zh/zhenyi-base.git
cd zhenyi-base
go run ./examples/groupchat/server
```

终端会提示 `open http://127.0.0.1:8080`，浏览器打开，填昵称点「连接」，多开几个标签页即可群聊。

---

## 架构简述

- **:8080**：静态 HTML 页面（`embed` 打包进二进制）
- **:9001**：WebSocket 群聊（`zserver` + `znet.WebSocket`）

单房间、纯内存，刷新即清空。网页和 WS 都在一个进程里，无需单独部署前端。

---

## 协议约定

线协议 v0：`msgId(4) + seqId(4) + dataLen(4) + data`，大端。

- msgId **1**：加入，body 昵称（≤24）
- msgId **2**：发言，body 文本（≤512）
- msgId **10**：广播事件，body 为 JSON `{"type":"join|leave|say","user":"…","text":"…"}`
- msgId **99**：错误提示

---

## 服务端要点

- `OnConnect` / `OnDisconnect`：维护 `conns` 和 `nicks` map
- `Handle(msgJoin)`：校验昵称，登记后 broadcast 进入事件
- `Handle(msgChat)`：校验已加入，broadcast 发言
- **必须**加 `zserver.WithAsyncMode()`，否则 `Send` 广播会丢包

---

## 前端要点

- 原生 WebSocket，`binaryType = "arraybuffer"`
- 按 12 字节头组包：`msgId + seqId + dataLen + data`
- 收包时按头部解析，MSG_EVT 的 payload 是 JSON，解析后渲染

完整前后端代码见 `examples/groupchat/server/` 和 `web/index.html`。

---

## 扩展

- 改端口：`-http :3000 -addr :9002`，网页里改 WS 端口
- 单独部署前端：拷 `web/` 用 nginx 挂，WS 地址填服务端 `-addr`
- 加房间、持久化：在 `conns` / `nicks` 上分桶或接数据库即可

---

## 一句话总结

一个二进制，HTTP 静态页 + WebSocket 群聊，协议简单、前后端都能改。群聊要加 `WithAsyncMode()`，否则广播丢包。

---

**示例代码**：https://github.com/aiyang-zh/zhenyi-base/tree/main/examples/groupchat  
**更多文档**：https://zhenyi-site.pages.dev/

---

*发布时建议把「阅读原文」设为示例目录：https://github.com/aiyang-zh/zhenyi-base/tree/main/examples/groupchat*
