package zlog

import (
	"context"

	"go.uber.org/zap"
)

// TraceFields 存储在 context 中的链路追踪元数据（值类型，零分配传递）
type TraceFields struct {
	TraceIdHi uint64
	TraceIdLo uint64
	SpanId    uint64
	MsgId     int32
	ActorId   int32
}

type traceCtxKey struct{}

// NewTraceContext 将 TraceFields 注入 context。
// context.WithValue 在 Go 1.22+ 下约 10ns，值类型不逃逸到堆。
func NewTraceContext(ctx context.Context, fields TraceFields) context.Context {
	return context.WithValue(ctx, traceCtxKey{}, fields)
}

// TraceFieldsFromContext 从 context 提取 TraceFields。
// 零分配：类型断言值类型不分配内存。
func TraceFieldsFromContext(ctx context.Context) (TraceFields, bool) {
	f, ok := ctx.Value(traceCtxKey{}).(TraceFields)
	return f, ok
}

// AppendTraceFields 将 context 中的 trace 元数据追加到 zap.Field 切片。
// 如果 context 无 trace 信息，原样返回 fields（零开销）。
//
// 用法（栈分配，零堆分配）：
//
//	var fa logger.FieldArray8
//	n := 0
//	fa[n] = zap.Int32("itemId", req.ItemId); n++
//	fields := logger.AppendTraceFields(ctx, fa[:n])
//	a.Logger.Info("buy item", fields...)
func AppendTraceFields(ctx context.Context, fields []zap.Field) []zap.Field {
	tf, ok := ctx.Value(traceCtxKey{}).(TraceFields)
	if !ok {
		return fields
	}
	fields = append(fields,
		FastUInt64("traceIdHi", tf.TraceIdHi),
		FastUInt64("traceIdLo", tf.TraceIdLo),
		FastInt("msgId", int(tf.MsgId)),
	)
	return fields
}

// TraceZapFields 从 context 提取 trace 字段并返回 zap.Field 数组（栈分配）。
// 返回值: fields 切片 + 有效字段数。如果无 trace 信息返回 nil。
//
// 用法：
//
//	a.Logger.Info("msg", logger.TraceZapFields(ctx)...)
func TraceZapFields(ctx context.Context) []zap.Field {
	tf, ok := ctx.Value(traceCtxKey{}).(TraceFields)
	if !ok {
		return nil
	}
	return Fields3(
		FastUInt64("traceIdHi", tf.TraceIdHi),
		FastUInt64("traceIdLo", tf.TraceIdLo),
		FastInt("msgId", int(tf.MsgId)),
	)
}
