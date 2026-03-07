package zencrypt

import (
	"testing"
)

// ============================================================
// Linear Encryption (linear.go)
// ============================================================

func TestLinear_EncryptDecrypt(t *testing.T) {
	// a=7, b=3, mod=100 (7 和 100 互质)
	l := NewLinear(7, 3, 100)

	for id := int64(0); id < 100; id++ {
		encrypted := l.Encrypt(id)
		decrypted := l.Decrypt(encrypted)
		if decrypted != id {
			t.Errorf("round trip failed for id=%d: encrypted=%d, decrypted=%d", id, encrypted, decrypted)
		}
	}
}

func TestLinear_EncryptDifferent(t *testing.T) {
	l := NewLinear(7, 3, 1000)
	e1 := l.Encrypt(1)
	e2 := l.Encrypt(2)
	if e1 == e2 {
		t.Error("different IDs should produce different encrypted values")
	}
}

func TestLinear_EncryptRange(t *testing.T) {
	l := NewLinear(7, 3, 1000)
	for id := int64(0); id < 100; id++ {
		enc := l.Encrypt(id)
		if enc < 0 || enc >= 1000 {
			t.Errorf("encrypted value %d out of range [0, 1000) for id=%d", enc, id)
		}
	}
}

func TestLinear_CalculateA(t *testing.T) {
	l := NewLinear(0, 0, 1000)
	a := l.CalculateA()
	if a <= 0 || a >= 1000 {
		t.Errorf("CalculateA returned %d, should be in (0, 1000)", a)
	}
	// 验证 a 和 mod 互质 (gcd == 1)
	if l.gcd(a, 1000) != 1 {
		t.Errorf("CalculateA returned %d, which is not coprime with 1000", a)
	}
}

func TestLinear_Bijection(t *testing.T) {
	// 当 a 和 mod 互质时，映射是双射的（不同输入不同输出）
	l := NewLinear(7, 3, 100)
	seen := make(map[int64]bool, 100)
	for id := int64(0); id < 100; id++ {
		enc := l.Encrypt(id)
		if seen[enc] {
			t.Fatalf("collision at id=%d, encrypted=%d", id, enc)
		}
		seen[enc] = true
	}
	if len(seen) != 100 {
		t.Errorf("expected 100 unique values, got %d", len(seen))
	}
}

func BenchmarkLinear_Encrypt(b *testing.B) {
	l := NewLinear(7, 3, 1000000)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		l.Encrypt(int64(i))
	}
}

func BenchmarkLinear_Decrypt(b *testing.B) {
	l := NewLinear(7, 3, 1000000)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		l.Decrypt(int64(i))
	}
}

func BenchmarkLinear_RoundTrip(b *testing.B) {
	l := NewLinear(7, 3, 1000000)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		l.Decrypt(l.Encrypt(int64(i)))
	}
}
