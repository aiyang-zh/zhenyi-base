package gmtls

import (
	"bytes"
	"testing"
)

func TestCipherMarshalSM2Plain_RoundTrip(t *testing.T) {
	plain := make([]byte, 1+32+32+32+16)
	plain[0] = 0x04
	for i := range plain[1:] {
		plain[1+i] = byte(i + 1)
	}
	asn1der, err := cipherMarshalSM2Plain(plain)
	if err != nil {
		t.Fatalf("cipherMarshalSM2Plain: %v", err)
	}
	back, err := cipherUnmarshalSM2ToPlain(asn1der)
	if err != nil {
		t.Fatalf("cipherUnmarshalSM2ToPlain: %v", err)
	}
	if !bytes.Equal(back, plain) {
		t.Fatalf("round-trip mismatch")
	}
}

func TestCipherMarshalSM2Plain_Errors(t *testing.T) {
	if _, err := cipherMarshalSM2Plain(nil); err == nil {
		t.Fatal("expected error for nil/short input")
	}
	short := make([]byte, 32)
	if _, err := cipherMarshalSM2Plain(short); err == nil {
		t.Fatal("expected error for short input")
	}
	wrongPrefix := make([]byte, 1+32+32+32+4)
	wrongPrefix[0] = 0x03
	if _, err := cipherMarshalSM2Plain(wrongPrefix); err == nil {
		t.Fatal("expected error for wrong point prefix")
	}
}

func BenchmarkCipherMarshalSM2Plain_RoundTrip(b *testing.B) {
	plain := make([]byte, 1+32+32+32+64)
	plain[0] = 0x04
	for i := range plain[1:] {
		plain[1+i] = byte(i)
	}
	asn1der, err := cipherMarshalSM2Plain(plain)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cipherUnmarshalSM2ToPlain(asn1der)
		_, _ = cipherMarshalSM2Plain(plain)
	}
}
