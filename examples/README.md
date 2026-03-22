# 示例代码

本目录包含 zhenyi-base 的示例项目，用于快速上手和压测验证。

## 示例列表

| 示例 | 说明 | 运行方式 |
|------|------|----------|
| [echodemo](./echodemo/) | 极简 Echo：服务端收到什么回什么，客户端支持交互输入 | `go run ./examples/echodemo/server` + `go run ./examples/echodemo/client` |
| [echobench](./echobench/) | Echo 压测：交互模式 + 批量压测，含 QPS 参考数据 | `go run ./examples/echobench/server` + `go run ./examples/echobench/client [-bench -n 100000 -c 10]` |

## 快速体验

```bash
# 1. 交互式 Echo（双终端）
# 终端 1
go run ./examples/echodemo/server

# 终端 2
go run ./examples/echodemo/client

# 2. 压测（根目录运行脚本）
make bench
```

## 依赖说明

- **echodemo**：服务端 zserver、ztcp、znet；客户端 ztcp、znet、zbrand
- **echobench**：服务端 zserver、ztcp、zws、zkcp、znet；客户端 ztcp、zws、zkcp、znet、ziface、zbrand（交互模式）

详见各子目录 README。
