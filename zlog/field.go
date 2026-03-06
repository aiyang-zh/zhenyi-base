package zlog

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// FastInt64 快速创建 int64 Field（内联优化）
// 相比 zap.Int64，这个版本避免了类型转换
func FastInt64(key string, val int64) zap.Field {
	return zap.Field{Key: key, Type: zapcore.Int64Type, Integer: val}
}

// FastUInt64 快速创建 int Field（内联优化）
func FastUInt64(key string, val uint64) zap.Field {
	return zap.Field{Key: key, Type: zapcore.Uint64Type, Integer: int64(val)}
}

// FastInt 快速创建 int Field（内联优化）
func FastInt(key string, val int) zap.Field {
	return zap.Field{Key: key, Type: zapcore.Int64Type, Integer: int64(val)}
}

// FastInt32 快速创建 int Field（内联优化）
func FastInt32(key string, val int32) zap.Field {
	return zap.Field{Key: key, Type: zapcore.Int32Type, Integer: int64(val)}
}

// FastString 快速创建 string Field（内联优化）
func FastString(key string, val string) zap.Field {
	return zap.Field{Key: key, Type: zapcore.StringType, String: val}
}

// FastBool 快速创建 bool Field（内联优化）
func FastBool(key string, val bool) zap.Field {
	var i int64
	if val {
		i = 1
	}
	return zap.Field{Key: key, Type: zapcore.BoolType, Integer: i}
}

// FastDuration 快速创建 duration Field（内联优化）
func FastDuration(key string, val int64) zap.Field {
	return zap.Field{Key: key, Type: zapcore.DurationType, Integer: val}
}

// AppendCommonFields 批量追加常用字段（减少函数调用）
// 使用示例：
//
//	fields := AppendCommonFields(fields[:0], sessionId, userId, msgId)
func AppendCommonFields(fields []zap.Field, sessionId, userId int64, msgId int32) []zap.Field {
	fields = append(fields,
		FastInt64("sid", sessionId),
		FastInt64("uid", userId),
		FastInt("msgId", int(msgId)),
	)
	return fields
}

// AppendSessionFields 追加 session 相关字段
func AppendSessionFields(fields []zap.Field, sessionId, authId int64) []zap.Field {
	fields = append(fields,
		FastInt64("sid", sessionId),
		FastInt64("authId", authId),
	)
	return fields
}

// AppendErrorFields 追加错误相关字段
func AppendErrorFields(fields []zap.Field, err error, code int32) []zap.Field {
	fields = append(fields,
		zap.Error(err),
		FastInt("code", int(code)),
	)
	return fields
}

// FieldArray 预分配的 Field 数组结构（栈上分配）
// 使用示例：
//
//	var fa FieldArray4
//	fa[0] = FastInt64("sid", sessionId)
//	fa[1] = FastInt64("uid", userId)
//	logger.Info("message", fa[0:2]...)
type FieldArray4 [4]zap.Field
type FieldArray8 [8]zap.Field
type FieldArray16 [16]zap.Field

// ToSlice 转换为 slice（零分配）
func (fa *FieldArray4) ToSlice(n int) []zap.Field {
	return fa[:n]
}

func (fa *FieldArray8) ToSlice(n int) []zap.Field {
	return fa[:n]
}

func (fa *FieldArray16) ToSlice(n int) []zap.Field {
	return fa[:n]
}

// ============================================================
// 便捷函数：直接返回 []zap.Field，无需手动 var 数组
// 使用示例：logger.Info("msg", Fields2(FastInt64("sid", 123), FastString("name", "test"))...)
// ============================================================

// Fields2 返回包含 2 个字段的 slice（栈分配，零堆分配）
func Fields2(f1, f2 zap.Field) []zap.Field {
	var arr [2]zap.Field
	arr[0] = f1
	arr[1] = f2
	return arr[:]
}

// Fields3 返回包含 3 个字段的 slice（栈分配，零堆分配）
func Fields3(f1, f2, f3 zap.Field) []zap.Field {
	var arr [3]zap.Field
	arr[0] = f1
	arr[1] = f2
	arr[2] = f3
	return arr[:]
}

// Fields4 返回包含 4 个字段的 slice（栈分配，零堆分配）
func Fields4(f1, f2, f3, f4 zap.Field) []zap.Field {
	var arr [4]zap.Field
	arr[0] = f1
	arr[1] = f2
	arr[2] = f3
	arr[3] = f4
	return arr[:]
}

// Fields5 返回包含 5 个字段的 slice（栈分配，零堆分配）
func Fields5(f1, f2, f3, f4, f5 zap.Field) []zap.Field {
	var arr [5]zap.Field
	arr[0] = f1
	arr[1] = f2
	arr[2] = f3
	arr[3] = f4
	arr[4] = f5
	return arr[:]
}

// Fields6 返回包含 6 个字段的 slice（栈分配，零堆分配）
func Fields6(f1, f2, f3, f4, f5, f6 zap.Field) []zap.Field {
	var arr [6]zap.Field
	arr[0] = f1
	arr[1] = f2
	arr[2] = f3
	arr[3] = f4
	arr[4] = f5
	arr[5] = f6
	return arr[:]
}
