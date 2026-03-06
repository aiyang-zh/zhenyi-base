package zlog

import (
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"
)

// TestFastFields 测试快速 Field 构造函数
func TestFastFields(t *testing.T) {
	tests := []struct {
		name     string
		fast     zap.Field
		standard zap.Field
	}{
		{
			name:     "Int64",
			fast:     FastInt64("key", 123),
			standard: zap.Int64("key", 123),
		},
		{
			name:     "Int",
			fast:     FastInt("key", 456),
			standard: zap.Int("key", 456),
		},
		{
			name:     "String",
			fast:     FastString("key", "value"),
			standard: zap.String("key", "value"),
		},
		{
			name:     "Bool-true",
			fast:     FastBool("key", true),
			standard: zap.Bool("key", true),
		},
		{
			name:     "Bool-false",
			fast:     FastBool("key", false),
			standard: zap.Bool("key", false),
		},
		{
			name:     "Duration",
			fast:     FastDuration("key", int64(time.Second)),
			standard: zap.Duration("key", time.Second),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.fast.Key != tt.standard.Key {
				t.Errorf("Key mismatch: %s != %s", tt.fast.Key, tt.standard.Key)
			}
			if tt.fast.Type != tt.standard.Type {
				t.Errorf("Type mismatch: %v != %v", tt.fast.Type, tt.standard.Type)
			}
			// 对于 String 类型，检查 String 字段
			if tt.fast.Type == tt.standard.Type {
				if tt.fast.Type == 15 { // StringType = 15
					if tt.fast.String != tt.standard.String {
						t.Errorf("String value mismatch: %s != %s", tt.fast.String, tt.standard.String)
					}
				} else {
					// 对于数值类型，检查 Integer 字段
					if tt.fast.Integer != tt.standard.Integer {
						t.Errorf("Integer value mismatch: %d != %d", tt.fast.Integer, tt.standard.Integer)
					}
				}
			}
		})
	}
}

// TestAppendCommonFields 测试批量追加字段
func TestAppendCommonFields(t *testing.T) {
	fields := make([]zap.Field, 0, 8)
	fields = AppendCommonFields(fields, 100, 200, 300)

	if len(fields) != 3 {
		t.Fatalf("Expected 3 fields, got %d", len(fields))
	}

	expected := []struct {
		key   string
		value int64
	}{
		{"sid", 100},
		{"uid", 200},
		{"msgId", 300},
	}

	for i, exp := range expected {
		if fields[i].Key != exp.key {
			t.Errorf("Field %d: expected key '%s', got '%s'", i, exp.key, fields[i].Key)
		}
		if fields[i].Integer != exp.value {
			t.Errorf("Field %d: expected value %d, got %d", i, exp.value, fields[i].Integer)
		}
	}
}

// TestAppendSessionFields 测试 session 字段追加
func TestAppendSessionFields(t *testing.T) {
	fields := make([]zap.Field, 0, 4)
	fields = AppendSessionFields(fields, 123, 456)

	if len(fields) != 2 {
		t.Fatalf("Expected 2 fields, got %d", len(fields))
	}

	if fields[0].Key != "sid" || fields[0].Integer != 123 {
		t.Errorf("First field mismatch")
	}
	if fields[1].Key != "authId" || fields[1].Integer != 456 {
		t.Errorf("Second field mismatch")
	}
}

// TestAppendErrorFields 测试错误字段追加
func TestAppendErrorFields(t *testing.T) {
	testErr := errors.New("test error")
	fields := make([]zap.Field, 0, 4)
	fields = AppendErrorFields(fields, testErr, 500)

	if len(fields) != 2 {
		t.Fatalf("Expected 2 fields, got %d", len(fields))
	}

	// 第一个应该是 error field
	if fields[0].Type != 26 { // ErrorType = 26
		t.Errorf("Expected error type, got %d", fields[0].Type)
	}

	// 第二个应该是 code field
	if fields[1].Key != "code" || fields[1].Integer != 500 {
		t.Errorf("Code field mismatch")
	}
}

// TestFieldArray 测试预分配数组
func TestFieldArray(t *testing.T) {
	var fa FieldArray4
	fa[0] = FastInt64("sid", 100)
	fa[1] = FastInt64("uid", 200)

	slice := fa.ToSlice(2)
	if len(slice) != 2 {
		t.Fatalf("Expected 2 elements, got %d", len(slice))
	}

	if slice[0].Integer != 100 || slice[1].Integer != 200 {
		t.Errorf("Field values mismatch")
	}
}

// 测试便捷函数 Fields2/3/4/5/6
func TestFieldsHelper(t *testing.T) {
	tests := []struct {
		name     string
		fields   []zap.Field
		expected int
	}{
		{
			name:     "Fields2",
			fields:   Fields2(FastInt64("a", 1), FastInt64("b", 2)),
			expected: 2,
		},
		{
			name:     "Fields3",
			fields:   Fields3(FastInt64("a", 1), FastInt64("b", 2), FastInt64("c", 3)),
			expected: 3,
		},
		{
			name:     "Fields4",
			fields:   Fields4(FastInt64("a", 1), FastInt64("b", 2), FastInt64("c", 3), FastInt64("d", 4)),
			expected: 4,
		},
		{
			name:     "Fields5",
			fields:   Fields5(FastInt64("a", 1), FastInt64("b", 2), FastInt64("c", 3), FastInt64("d", 4), FastInt64("e", 5)),
			expected: 5,
		},
		{
			name:     "Fields6",
			fields:   Fields6(FastInt64("a", 1), FastInt64("b", 2), FastInt64("c", 3), FastInt64("d", 4), FastInt64("e", 5), FastInt64("f", 6)),
			expected: 6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if len(tt.fields) != tt.expected {
				t.Errorf("expected %d fields, got %d", tt.expected, len(tt.fields))
			}
			// 验证字段值
			for i := 0; i < tt.expected; i++ {
				if tt.fields[i].Integer != int64(i+1) {
					t.Errorf("field[%d] expected value %d, got %d", i, i+1, tt.fields[i].Integer)
				}
			}
		})
	}
}

// BenchmarkFastInt64_vs_ZapInt64 对比快速和标准版本
func BenchmarkFastInt64_vs_ZapInt64(b *testing.B) {
	b.Run("FastInt64", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = FastInt64("key", 123)
		}
	})

	b.Run("zap.Int64", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = zap.Int64("key", 123)
		}
	})
}

// BenchmarkAppendFields 对比批量追加和单个追加
func BenchmarkAppendFields(b *testing.B) {
	b.Run("AppendCommonFields", func(b *testing.B) {
		fields := make([]zap.Field, 0, 8)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			fields = AppendCommonFields(fields[:0], 100, 200, 300)
		}
	})

	b.Run("IndividualAppend", func(b *testing.B) {
		fields := make([]zap.Field, 0, 8)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			fields = fields[:0]
			fields = append(fields,
				zap.Int64("sid", 100),
				zap.Int64("uid", 200),
				zap.Int("msgId", 300),
			)
		}
	})
}

// BenchmarkFieldArray 对比栈分配和堆分配
func BenchmarkFieldArray(b *testing.B) {
	b.Run("StackArray", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			var fa FieldArray4
			fa[0] = FastInt64("sid", 100)
			fa[1] = FastInt64("uid", 200)
			_ = fa.ToSlice(2)
		}
	})

	b.Run("HeapSlice", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			fields := make([]zap.Field, 2)
			fields[0] = FastInt64("sid", 100)
			fields[1] = FastInt64("uid", 200)
			_ = fields
		}
	})
}

// Benchmark 对比便捷函数性能
func BenchmarkFieldsHelper(b *testing.B) {
	b.Run("原始make方式", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			fields := make([]zap.Field, 2)
			fields[0] = FastInt64("sid", 100)
			fields[1] = FastInt64("uid", 200)
			_ = fields
		}
	})

	b.Run("手动FieldArray4", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			var fa FieldArray4
			fa[0] = FastInt64("sid", 100)
			fa[1] = FastInt64("uid", 200)
			fields := fa[0:2]
			_ = fields
		}
	})

	b.Run("Fields2便捷函数", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			fields := Fields2(FastInt64("sid", 100), FastInt64("uid", 200))
			_ = fields
		}
	})

	b.Run("Fields4便捷函数", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			fields := Fields4(
				FastInt64("sid", 100),
				FastInt64("uid", 200),
				FastInt64("msgId", 1001),
				FastString("action", "login"),
			)
			_ = fields
		}
	})
}
