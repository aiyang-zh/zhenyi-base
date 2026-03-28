package gmtls

import (
	"errors"
	"net"
	"testing"
	"time"
)

func TestDialWithDialer_GM(t *testing.T) {
	certs := newTestGMServerCertificates(t)
	ln, err := Listen("tcp", "127.0.0.1:0", &Config{
		GMSupport:    NewGMSupport(),
		Certificates: certs,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	d := &net.Dialer{Timeout: 5 * time.Second}
	clientCfg := &Config{
		GMSupport:          NewGMSupport(),
		InsecureSkipVerify: true,
		ServerName:         "zgmtls-test",
	}

	errCh := make(chan error, 1)
	go func() {
		c, err := DialWithDialer(d, "tcp", ln.Addr().String(), clientCfg)
		if err != nil {
			errCh <- err
			return
		}
		defer c.Close()
		errCh <- c.Handshake()
	}()

	sc, err := ln.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer sc.(*Conn).Close()
	if err := sc.(*Conn).Handshake(); err != nil {
		t.Fatal(err)
	}
	if err := <-errCh; err != nil {
		t.Fatal(err)
	}
}

func TestListenDial_Handshake(t *testing.T) {
	certs := newTestGMServerCertificates(t)
	srvCfg := &Config{
		GMSupport:    NewGMSupport(),
		Certificates: certs,
	}
	ln, err := Listen("tcp", "127.0.0.1:0", srvCfg)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer ln.Close()

	clientCfg := &Config{
		GMSupport:          NewGMSupport(),
		InsecureSkipVerify: true,
		ServerName:         "zgmtls-test",
	}

	errCh := make(chan error, 1)
	go func() {
		c, err := Dial("tcp", ln.Addr().String(), clientCfg)
		if err != nil {
			errCh <- err
			return
		}
		defer c.Close()
		errCh <- c.Handshake()
	}()

	sc, err := ln.Accept()
	if err != nil {
		t.Fatalf("Accept: %v", err)
	}
	srvConn, ok := sc.(*Conn)
	if !ok {
		t.Fatalf("expected *Conn, got %T", sc)
	}
	defer srvConn.Close()

	if err := srvConn.Handshake(); err != nil {
		t.Fatalf("server Handshake: %v", err)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("client Handshake: %v", err)
	}
}

func TestListenDial_AppDataAndConnState(t *testing.T) {
	certs := newTestGMServerCertificates(t)
	srvCfg := &Config{
		GMSupport:    NewGMSupport(),
		Certificates: certs,
	}
	ln, err := Listen("tcp", "127.0.0.1:0", srvCfg)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	clientCfg := &Config{
		GMSupport:          NewGMSupport(),
		InsecureSkipVerify: true,
		ServerName:         "zgmtls-test",
	}

	errCh := make(chan error, 1)
	go func() {
		c, err := Dial("tcp", ln.Addr().String(), clientCfg)
		if err != nil {
			errCh <- err
			return
		}
		defer c.Close()
		if err := c.Handshake(); err != nil {
			errCh <- err
			return
		}
		st := c.ConnectionState()
		if !st.HandshakeComplete {
			errCh <- errors.New("client: handshake incomplete")
			return
		}
		if c.LocalAddr() == nil || c.RemoteAddr() == nil {
			errCh <- errors.New("client: nil addr")
			return
		}
		payload := []byte("ping!")
		if _, err := c.Write(payload); err != nil {
			errCh <- err
			return
		}
		buf := make([]byte, len(payload))
		if _, err := c.Read(buf); err != nil {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	sc, err := ln.Accept()
	if err != nil {
		t.Fatal(err)
	}
	srv := sc.(*Conn)
	defer srv.Close()

	if err := srv.Handshake(); err != nil {
		t.Fatalf("server handshake: %v", err)
	}
	st := srv.ConnectionState()
	if !st.HandshakeComplete {
		t.Fatal("server: handshake not complete")
	}
	buf := make([]byte, 32)
	n, err := srv.Read(buf)
	if err != nil {
		t.Fatalf("server read: %v", err)
	}
	if _, err := srv.Write(buf[:n]); err != nil {
		t.Fatalf("server write: %v", err)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("client side: %v", err)
	}
}
