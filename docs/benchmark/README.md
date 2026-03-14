# 基准测试

本目录存放 zqueue 及相关组件的基准测试原始数据。

## 文件说明

| 文件 | 说明 |
|------|------|
| [zqueue_matrix_results.txt](zqueue_matrix_results.txt) | zqueue 96 组合基准测试完整结果（类型×数据大小×生产者×消费方式） |

## 复现

```bash
go test -bench=BenchmarkMatrix -benchmem ./zqueue/
```

## 测试维度

- **类型**：MPSC 有界、MPSC 无界、Channel 有缓冲、Channel 无缓冲
- **数据大小**：Small(256)、Medium(4096)、Large(65536)
- **生产者**：1、4、16、64
- **消费**：Single（单条）、Batch（批量）
