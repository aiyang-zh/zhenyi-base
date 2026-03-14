# Go 无锁队列 zqueue 单点深挖：设计、选型与性能

**标签**：Go、高性能、无锁队列、并发、开源、后端

---

上一篇介绍了 [zhenyi-base](https://juejin.cn/post/7614110147108913192) 整体能力，其中提到无锁队列 **16.7 ns/op**、0 分配。很多同学问：和 channel 比有什么不同？在 MPSC 场景下该怎么选？这篇单点深挖 **zqueue**，从设计、选型到 96 组合基准测试，把结论说清楚。

---

## 一、为什么需要无锁队列（和 channel 的差异）

用 Go 做「多 goroutine 写、一个 goroutine 读」时，channel 是常用方案：

```go
ch := make(chan T, 1024)
go func() { ch <- x1 }()
go func() { ch <- x2 }()
for v := range ch { ... }
```

在 MPSC 场景下，channel 有一些固有特点：有界时 buffer 满会阻塞发送；多生产者需要协调谁来 close；没有内置的「批量投 / 批量取」语义；多 goroutine 共享同一 channel 时，竞争会随并发增加。若你需要**无界、不阻塞发送方、批量入队/出队、或更细的延迟控制**，无锁队列可以**补充**这些能力。

所以：在 **多生产单消费（MPSC）** 场景下，zqueue 不是要取代 channel，而是在 channel 不擅长的需求上做**补充**。

---

## 二、zqueue 类型总览


| 类型                | 生产者/消费者 | 有界/无界 | 典型场景            |
| ----------------- | ------- | ----- | --------------- |
| **MPSCQueue**     | 多 / 单   | 有界    | 固定容量、延迟敏感       |
| **UnboundedMPSC** | 多 / 单   | 无界    | 任务队列、消息 mailbox |
| **SPSCQueue**     | 单 / 单   | 有界    | 单写单读、极致延迟       |
| **Queue**（有锁）     | 多 / 多   | 有界可扩容 | 需要 Count/Front  |


本篇重点说 **MPSCQueue**（有界）和 **UnboundedMPSC**（无界）。

---

## 三、设计要点

### 3.1 伪共享与 cache line

`head` 和 `tail` 之间垫 128 字节，保证生产者和消费者不抢同一条 cache line。高并发下可用 **NewMPSCQueuePadded** 消除 slot 间伪共享。

### 3.2 有界 MPSC：环形 + sequence，批量 CAS

- **环形数组**：capacity 取 2 的幂，`head & mask` 下标无分支。
- **EnqueueBatch**：一次 CAS 占 N 个连续 slot，摊薄竞争。
- **CAS 失败退避**：`runtime.Gosched()` 减少自旋。

### 3.3 无界 UnboundedMPSC：链表 + 双端对象池

- **入队**：对象池取节点，`atomic.SwapPointer` 挂到 head，wait-free。
- **出队**：消费者只读 tail，单消费者无需 CAS。
- **Shrink**：空闲时调用，控制常驻内存。

---

## 四、对比测试设计

我们设计了 **96 种组合** 的基准测试，维度如下：


| 维度       | 取值                                               |
| -------- | ------------------------------------------------ |
| **类型**   | MPSC 有界、MPSC 无界、Channel 有缓冲、Channel 无缓冲          |
| **数据大小** | Small(256)、Medium(4096)、Large(65536) — 有界/有缓冲的容量 |
| **生产者**  | 1、4、16、64                                        |
| **消费**   | Single（单条）、Batch（批量）                             |


**有界 MPSC 与 Channel 有缓冲的容量保持一致**，保证公平对比。无缓冲 Channel（cap=0）作为同步场景对照。

**测试环境**：Go 1.24，darwin/arm64，Apple M3。以下数据为本地单次或少量运行结果（未取多次平均值），存在机器负载、调度等误差，仅供参考；不同 Go 版本、操作系统、硬件可能导致性能差异，ns/op 会有波动，建议在目标环境自行复现。

**复现命令**：

```bash
go test -bench=BenchmarkMatrix -benchmem ./zqueue/
```

---

## 五、结果分析

### 5.1 生产者数量对延迟的影响（固定 Medium 4096）


| 生产者 | MPSC 有界 Single | MPSC 有界 Batch | Chan 有缓冲 Single | Chan 无缓冲 Single |
| --- | -------------- | ------------- | --------------- | --------------- |
| 1   | 25.5 ns        | **17.8 ns**   | 52.8 ns         | 215 ns          |
| 4   | 31.7 ns        | **23.1 ns**   | 81.9 ns         | 471 ns          |
| 16  | 34.9 ns        | **23.9 ns**   | 120.5 ns        | 463 ns          |
| 64  | 37.3 ns        | **31.2 ns**   | 214.5 ns        | 552 ns          |


**解读**：有界 MPSC 在所有生产者规模下保持 20–40 ns 极低延迟，且随并发增加性能衰减远小于 channel。当生产者数达到 64 时，有界 MPSC 批量消费仅需 **31 ns**，而有缓冲 channel 单条消费高达 **215 ns**。无缓冲 channel 因同步握手，延迟在 200–600 ns。

### 5.2 数据大小对延迟的影响（固定 P16）


| 数据大小         | MPSC 有界 Single | MPSC 有界 Batch | Chan 有缓冲 Single |
| ------------ | -------------- | ------------- | --------------- |
| Small(256)   | 43.7 ns        | 52.9 ns       | 184.1 ns        |
| Medium(4096) | 34.9 ns        | **23.9 ns**   | 120.5 ns        |
| Large(65536) | 43.9 ns        | **29.6 ns**   | 119.4 ns        |


**解读**：数据大小对有界队列影响不大；Medium/Large 下 Batch 优势更明显。Channel 在 Medium 以上容量时略有改善，但仍远高于 MPSC。

### 5.3 批量消费 vs 单条消费（MPSC 有界）


| 场景         | Single  | Batch       | Batch 收益 |
| ---------- | ------- | ----------- | -------- |
| P1/Medium  | 25.5 ns | **17.8 ns** | 约 30% 更快 |
| P16/Medium | 34.9 ns | **23.9 ns** | 约 31% 更快 |
| P64/Large  | 41.5 ns | **29.2 ns** | 约 30% 更快 |


**解读**：批量消费在多数情况下优于单条消费，尤其在高并发和大容量时优势显著。

### 5.4 内存分配对比（P16，Medium）


| 类型       | ns/op     | B/op  | allocs/op |
| -------- | --------- | ----- | --------- |
| MPSC 有界  | 23.9–34.9 | **0** | **0**     |
| MPSC 无界  | 239–290   | 15–16 | 0（池化）     |
| Chan 有缓冲 | 98–125    | **0** | **0**     |
| Chan 无缓冲 | 463–564   | **0** | **0**     |


**解读**：有界 MPSC 与 channel 均为零分配。无界 MPSC 有少量 B/op（节点池），池化后 allocs 为 0，GC 可控。

### 5.5 核心结论

1. **有界无锁队列（MPSCBounded）**：所有场景下 20–50 ns、零分配，高并发下性能衰减远小于 channel。
2. **批量消费**：多数情况下优于单条，高并发和大数据时优势更明显。
3. **无界无锁队列（MPSCUnbounded）**：100–300 ns，少量 B/op，仍远优于无缓冲 channel。
4. **有缓冲 channel**：单生产者约 50 ns，高并发下飙升至 200–300 ns，且无原生批量语义。
5. **无缓冲 channel**：200–600 ns，仅适用于同步场景。

---

## 六、选型指南


| 场景                   | 推荐                                         |
| -------------------- | ------------------------------------------ |
| 多生产单消费 + 固定容量 + 延迟敏感 | **MPSCQueue**（有界），满时 `TryEnqueue` 返回 false |
| 多生产单消费 + 突发、容量不可控    | **UnboundedMPSC**，定期 `Shrink` 控内存          |
| 简单 MPSC + 能接受阻塞与关闭语义 | **Channel** 有缓冲                            |
| 单生产单消费 + 极致延迟        | **SPSCQueue**                              |
| 同步握手、必须阻塞            | **Channel** 无缓冲                            |


**快速判断**：多生产单消费 + 要批量/无界/主动背压 → zqueue；简单 MPSC + 不想写退避逻辑 → channel。

---

## 七、典型用法

### 无界：任务队列

```go
q := zqueue.NewUnboundedMPSC[Task]()
for i := 0; i < 10; i++ {
    go func(id int) { q.Enqueue(Task{ID: id}) }(i)
}
buf := make([]Task, 128)
for {
    n := q.DequeueBatch(buf)
    for i := 0; i < n; i++ { process(buf[i]) }
    if n == 0 && q.Empty() { time.Sleep(time.Millisecond) }
}
```

### 有界：满则丢弃

```go
q := zqueue.NewMPSCQueue[int](1024)
if !q.TryEnqueue(42) { /* 丢弃或重试 */ }
if v, ok := q.Dequeue(); ok { use(v) }
```

---

## 八、容易踩的坑

- **消费者侧**：`Dequeue`/`DequeueBatch`/`Shrink` 仅允许**单 goroutine** 调用。
- **无界内存**：定期调用 `Shrink()` 控制常驻内存。
- **容量取整**：`NewMPSCQueue(1000)` 实际为 1024，需事先按 2 的幂取值。

---

## 九、总结

- **channel** 胜在原生语义、开发成本低，适合简单并发。
- **zqueue** 胜在无锁、批量、主动背压，适合高吞吐、低延迟的 MPSC 场景。

选型：有界固定容量 → MPSCQueue；无界突发 → UnboundedMPSC；单写单读 → SPSC 系。

---

**即时试用**：

```bash
go get github.com/aiyang-zh/zhenyi-base
```

示例与性能图表见 [官网](https://zhenyi-site.pages.dev/)。代码与单测在 [GitHub · zhenyi-base/zqueue](https://github.com/aiyang-zh/zhenyi-base/tree/main/zqueue)。

---

## 附录：完整基准数据

**96 种组合的完整原始结果**见：

- [docs/benchmark/zqueue_matrix_results.txt](https://github.com/aiyang-zh/zhenyi-base/blob/main/docs/benchmark/zqueue_matrix_results.txt)
- [基准测试说明](https://github.com/aiyang-zh/zhenyi-base/blob/main/docs/benchmark/README.md)

欢迎试用、反馈。多生产单消费场景下，你更倾向 channel 还是无锁队列？评论区见。