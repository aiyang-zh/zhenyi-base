# 架构设计

## 模块划分

```
zhenyi-base
├── 网络层      ziface, znet, ztcp, zws, zkcp, zserver
├── 数据结构    zqueue, zpool, zbatch, zcoll
└── 基础工具    zerrs, zlog, zencrypt, zserialize, zbackoff, zlimiter, zpub, zid, zgrace, zrand, ztime, ztimer, zfile
```

## 依赖关系

- **ziface**：纯接口，零依赖
- **znet**：依赖 ziface、zpool、zqueue、zbatch、zerrs、zlog、ztime
- **ztcp/zws/zkcp**：依赖 znet，实现不同传输协议
- **zserver**：依赖 znet、ziface，提供业务侧 API
- **zqueue/zpool/zbatch**：零外部依赖（仅标准库）
- **zlog**：依赖 zap、zpool
- **zerrs**：零外部依赖

按需引入，只编译用到的包。

## 设计原则

1. **零分配**：无锁队列、对象池、Ring Buffer 零拷贝
2. **接口抽象**：ziface 定义核心接口，ztcp/zws/zkcp 实现可替换
3. **按需依赖**：用 zqueue 不拉入 zap，用 zpool 不拉入 websocket
4. **生产可用**：连接管理、优雅关闭、指标、TLS/GM-TLS、panic 恢复

## 网络层数据流

```
Accept → HandleAccept → AddChannel → StartSend + Read 循环
         ↓
     OnRead 回调 → 业务处理 → channel.Send
         ↓
     runSend 批量 writev 发送
```

## 协议格式 (znet)

- Header: `msgId(4) + seqId(4) + dataLen(4)` 共 12 字节
- 可选 v1: `version(1) + msgId(4) + seqId(4) + dataLen(4)` 共 13 字节
