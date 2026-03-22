// Package zgrace 提供进程级优雅退出：监听 SIGINT/SIGTERM/SIGQUIT（及测试用 Stop），
// 按注册顺序串行调用关闭回调。
//
// # 推荐使用方式（全局一个 Grace）
//
// 进程内通常只创建一个 *Grace，在 main 末尾调用一次 Wait；各子系统在启动阶段 Register。
//
//	g := zgrace.New()
//	shutdownCtx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
//	defer cancel()
//	g.SetContext(shutdownCtx)
//	g.Register(func(ctx context.Context) { /* 关闭 HTTP、连接池等，内部应尊重 ctx */ })
//	g.Wait()
//
// # Context
//
// SetContext 设置在「收到退出信号之后」传给每个回调的 context；未调用 SetContext 或传入 nil
// 清除后，Wait 内会使用 context.Background()。注意：本包不会在 Wait 里监听 ctx 的取消/超时，
// 也不会打断正在执行的回调；超时控制需要各回调自行结合传入的 ctx（如 http.Server.Shutdown(ctx)）。
//
// # Panic 行为
//
// 单个回调 panic 时会被 recover，不会向上传播，后续已注册的回调仍会执行。
// panic 信息默认不会输出；若需记录或指标，请在回调内自行 defer recover 并处理。
//
// # 其他注意
//
//   - 同一 *Grace 上 Wait 只应使用一次：收到信号并执行完回调后，不会再次进入等待。
//   - Register 与 SetContext 可在运行时并发调用，与 Wait 并发时以 Wait 内拷贝到的快照为准。
package zgrace
