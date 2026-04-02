package gmtls

import "testing"

func safeUnmarshalCall(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panic: %v", r)
		}
	}()
	fn()
}

func clampBytes(b []byte, max int) []byte {
	if len(b) <= max {
		return b
	}
	return b[:max]
}

func FuzzClientHelloMsg_unmarshal(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte("seed"))

	f.Fuzz(func(t *testing.T, data []byte) {
		data = clampBytes(data, 16*1024)
		m := &clientHelloMsg{}
		safeUnmarshalCall(t, func() { _ = m.unmarshal(data) })
	})
}

func FuzzServerHelloMsg_unmarshal(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte("seed"))

	f.Fuzz(func(t *testing.T, data []byte) {
		data = clampBytes(data, 16*1024)
		m := &serverHelloMsg{}
		safeUnmarshalCall(t, func() { _ = m.unmarshal(data) })
	})
}

func FuzzCertificateMsg_unmarshal(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte("seed"))

	f.Fuzz(func(t *testing.T, data []byte) {
		data = clampBytes(data, 16*1024)
		m := &certificateMsg{}
		safeUnmarshalCall(t, func() { _ = m.unmarshal(data) })
	})
}

func FuzzCertificateRequestMsgGM_unmarshal(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte("seed"))

	f.Fuzz(func(t *testing.T, data []byte) {
		data = clampBytes(data, 16*1024)
		m := &certificateRequestMsgGM{}
		safeUnmarshalCall(t, func() { _ = m.unmarshal(data) })
	})
}

func FuzzSessionState_unmarshal(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte("seed"))

	f.Fuzz(func(t *testing.T, data []byte) {
		data = clampBytes(data, 16*1024)
		s := &sessionState{}
		safeUnmarshalCall(t, func() { _ = s.unmarshal(data) })
	})
}

func FuzzX509KeyPair_noPanic(f *testing.F) {
	f.Add([]byte{}, []byte{})
	f.Add([]byte("not a pem"), []byte("not a pem"))

	f.Fuzz(func(t *testing.T, certPEMBlock []byte, keyPEMBlock []byte) {
		certPEMBlock = clampBytes(certPEMBlock, 64*1024)
		keyPEMBlock = clampBytes(keyPEMBlock, 64*1024)
		safeUnmarshalCall(t, func() {
			_, _ = X509KeyPair(certPEMBlock, keyPEMBlock)
		})
	})
}

func FuzzGMX509KeyPairsSingle_noPanic(f *testing.F) {
	f.Add([]byte{}, []byte{})
	f.Add([]byte("not a pem"), []byte("not a pem"))

	f.Fuzz(func(t *testing.T, certPEMBlock []byte, keyPEMBlock []byte) {
		certPEMBlock = clampBytes(certPEMBlock, 64*1024)
		keyPEMBlock = clampBytes(keyPEMBlock, 64*1024)
		safeUnmarshalCall(t, func() {
			_, _ = GMX509KeyPairsSingle(certPEMBlock, keyPEMBlock)
		})
	})
}
