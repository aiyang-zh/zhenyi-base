# Go 无锁队列 zqueue：和 channel 比到底该选谁？

---

上一篇发了 zhenyi-base 整体能力后，很多人问：无锁队列 **16.7 ns/op**、0 分配，和 channel 有啥区别？MPSC 场景该用哪个？

这篇用 **96 组基准测试** 把 zqueue 和 channel 对比清楚，直接给你选型结论。

---

## 为什么需要无锁队列？

多 goroutine 写、一个读，channel 是首选。但 channel 有几点：buffer 满会阻塞、没有批量入队/出队、多 goroutine 抢一个 channel 竞争会上去。

如果你要**无界、不阻塞发送方、批量、或更细的延迟控制**，无锁队列可以补上。zqueue 不是取代 channel，是在 channel 不擅长的需求上做**补充**。

---

## zqueue 有哪些类型？

- **MPSCQueue**（有界）：多生产单消费，固定容量、延迟敏感  
- **UnboundedMPSC**（无界）：任务队列、消息 mailbox、突发流量  
- **SPSCQueue**（有界）：单写单读、极致延迟  
- **Queue**（有锁）：多对多，要 Count/Front 时用  

本篇重点是有界 MPSC 和无界 MPSC。

---

## 设计要点（简）

- **伪共享**：head/tail 之间垫 128 字节，避免抢同一条 cache line；高并发可用 NewMPSCQueuePadded。  
- **有界**：环形数组 + sequence + EnqueueBatch 一次占 N 个 slot，摊薄竞争；CAS 失败 Gosched 退避。  
- **无界**：链表 + 双端对象池，入队 wait-free；Shrink 控内存。

---

## 96 组测试说了啥？

我们做了 4 类型 × 3 数据大小 × 4 生产者数 × 2 消费方式 = 96 组合，有界 MPSC 和 Channel 有缓冲**容量一致**，无缓冲 channel 单独对比。

**测试环境**：Go 1.24，darwin/arm64，M3。数据为本地少量运行（未取多次平均），仅供参考；不同环境会有波动，建议自行复现。

**核心结论**：

1. **有界 MPSC**：20–50 ns、零分配，高并发下衰减远小于 channel；64 生产者时批量仅 **31 ns**，channel 单条 **215 ns**。  
2. **批量消费**：多数场景优于单条，高并发时更明显。  
3. **无界 MPSC**：100–300 ns，少量 B/op（池化），仍远优于无缓冲 channel。  
4. **有缓冲 channel**：单生产者约 50 ns，高并发飙到 200–300 ns，且无原生批量。  
5. **无缓冲 channel**：200–600 ns，只适合同步场景。

复现：`go test -bench=BenchmarkMatrix -benchmem ./zqueue/`

---

## 选型一句话

- 多生产单消费 + 固定容量 + 要延迟 → **MPSCQueue**（有界），满时 TryEnqueue 返回 false。  
- 多生产单消费 + 突发、容量不可控 → **UnboundedMPSC**，定期 Shrink。  
- 简单 MPSC + 能接受阻塞 → **Channel** 有缓冲。  
- 单生产单消费 + 极致延迟 → **SPSCQueue**。  
- 同步握手 → **Channel** 无缓冲。

**记**：要批量/无界/主动背压 → zqueue；不想写退避逻辑 → channel。

---

## 典型用法（缩略）

**无界任务队列**：NewUnboundedMPSC，多 goroutine Enqueue，单 goroutine DequeueBatch + 空时 Sleep；Dequeue/Shrink 只能同一 goroutine 调。

**有界满则丢**：NewMPSCQueue(1024)，TryEnqueue 失败就丢或重试。

**场景对号**：微服务任务不能丢 → UnboundedMPSC；网关日志可丢 → 有界 MPSCQueue；Kafka 消费缓冲 → SPSCQueue。

---

## 三个坑

1. **消费者**：Dequeue / DequeueBatch / Shrink 只能**单 goroutine** 调，多 goroutine 会乱序甚至丢数据。  
2. **无界内存**：定期 Shrink()，否则池会涨。  
3. **容量取整**：NewMPSCQueue(1000) 实际 1024，要严格上限就自己按 2 的幂传。

---

## 一句话总结

channel 胜在简单、省心；zqueue 胜在无锁、批量、主动背压，适合高吞吐低延迟的 MPSC。有界固定容量用 MPSCQueue，无界突发用 UnboundedMPSC，单写单读用 SPSC。

---

**安装**：`go get github.com/aiyang-zh/zhenyi-base`

**完整 96 组数据**：见 GitHub 上 zhenyi-base 仓库的 `docs/benchmark` 目录。

**更多示例与图表**：见项目官网 zhenyi-site。

---

*发布时建议把「阅读原文」设为 GitHub 仓库：https://github.com/aiyang-zh/zhenyi-base*
