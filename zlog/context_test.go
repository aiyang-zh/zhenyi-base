package zlog

import (
	"context"
	"testing"

	"go.uber.org/zap"
)

func TestTraceFields_RoundTrip(t *testing.T) {
	ctx := context.Background()

	fields := TraceFields{
		TraceIdHi: 0xAABBCCDD11223344,
		TraceIdLo: 0x5566778899AABBCC,
		SpanId:    0x1122334455667788,
		MsgId:     1001,
		ActorId:   42,
	}

	ctx = NewTraceContext(ctx, fields)

	got, ok := TraceFieldsFromContext(ctx)
	if !ok {
		t.Fatal("expected TraceFields in context")
	}

	if got.TraceIdHi != fields.TraceIdHi {
		t.Errorf("TraceIdHi: want %x, got %x", fields.TraceIdHi, got.TraceIdHi)
	}
	if got.TraceIdLo != fields.TraceIdLo {
		t.Errorf("TraceIdLo: want %x, got %x", fields.TraceIdLo, got.TraceIdLo)
	}
	if got.SpanId != fields.SpanId {
		t.Errorf("SpanId: want %x, got %x", fields.SpanId, got.SpanId)
	}
	if got.MsgId != fields.MsgId {
		t.Errorf("MsgId: want %d, got %d", fields.MsgId, got.MsgId)
	}
	if got.ActorId != fields.ActorId {
		t.Errorf("ActorId: want %d, got %d", fields.ActorId, got.ActorId)
	}
}

func TestTraceFieldsFromContext_Missing(t *testing.T) {
	ctx := context.Background()

	_, ok := TraceFieldsFromContext(ctx)
	if ok {
		t.Error("expected no TraceFields in empty context")
	}
}

func TestTraceFieldsFromContext_ZeroValues(t *testing.T) {
	ctx := NewTraceContext(context.Background(), TraceFields{})

	got, ok := TraceFieldsFromContext(ctx)
	if !ok {
		t.Fatal("expected TraceFields even with zero values")
	}
	if got.TraceIdHi != 0 || got.MsgId != 0 || got.ActorId != 0 {
		t.Error("zero-value TraceFields should remain zero")
	}
}

func TestAppendTraceFields_WithTrace(t *testing.T) {
	ctx := NewTraceContext(context.Background(), TraceFields{
		TraceIdHi: 123,
		TraceIdLo: 456,
		MsgId:     1001,
	})

	fields := AppendTraceFields(ctx, nil)
	if len(fields) != 3 {
		t.Fatalf("expected 3 fields appended, got %d", len(fields))
	}

	if fields[0].Key != "traceIdHi" {
		t.Errorf("field[0] key: want traceIdHi, got %s", fields[0].Key)
	}
	if fields[1].Key != "traceIdLo" {
		t.Errorf("field[1] key: want traceIdLo, got %s", fields[1].Key)
	}
	if fields[2].Key != "msgId" {
		t.Errorf("field[2] key: want msgId, got %s", fields[2].Key)
	}
}

func TestAppendTraceFields_WithoutTrace(t *testing.T) {
	ctx := context.Background()

	existing := make([]zap.Field, 0, 2)
	result := AppendTraceFields(ctx, existing)
	if len(result) != 0 {
		t.Errorf("expected no fields appended without trace, got %d", len(result))
	}
}

func TestAppendTraceFields_PreservesExisting(t *testing.T) {
	ctx := NewTraceContext(context.Background(), TraceFields{
		TraceIdHi: 999,
		TraceIdLo: 888,
		MsgId:     2002,
	})

	var fa FieldArray8
	fa[0] = FastInt("custom", 42)
	fields := AppendTraceFields(ctx, fa[:1])

	if len(fields) != 4 {
		t.Fatalf("expected 4 fields (1 existing + 3 trace), got %d", len(fields))
	}
	if fields[0].Key != "custom" {
		t.Errorf("first field should be preserved, got key=%s", fields[0].Key)
	}
}

func TestTraceZapFields_WithTrace(t *testing.T) {
	ctx := NewTraceContext(context.Background(), TraceFields{
		TraceIdHi: 111,
		TraceIdLo: 222,
		MsgId:     3003,
	})

	fields := TraceZapFields(ctx)
	if len(fields) != 3 {
		t.Fatalf("expected 3 fields, got %d", len(fields))
	}
}

func TestTraceZapFields_WithoutTrace(t *testing.T) {
	ctx := context.Background()
	fields := TraceZapFields(ctx)
	if fields != nil {
		t.Errorf("expected nil fields without trace, got %v", fields)
	}
}
