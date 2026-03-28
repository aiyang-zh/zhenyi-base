package gmtls

import (
	"strconv"
	"testing"
)

func TestAlert_StringAndError(t *testing.T) {
	if s := alert(alertCloseNotify).String(); s == "" {
		t.Fatal("expected non-empty alert string")
	}
	if err := alert(alertCloseNotify).Error(); err == "" {
		t.Fatal("expected non-empty Error()")
	}
	unknown := alert(0xff)
	if want := "tls: alert(" + strconv.Itoa(0xff) + ")"; unknown.String() != want {
		t.Fatalf("unknown alert: got %q want %q", unknown.String(), want)
	}
}
