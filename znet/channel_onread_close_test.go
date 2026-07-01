package znet

import (
	"encoding/binary"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aiyang-zh/zhenyi-base/ziface"
)

// channel_onread_close_test.go：OnRead 回调内/外调用 Close 不得与读循环死锁（readWg 异步收尾回归）。

func writeTestPacketV0(conn net.Conn, msgId uint32, body []byte) error {
	hdr := make([]byte, headerSizeV0)
	binary.BigEndian.PutUint32(hdr[0:4], msgId)
	binary.BigEndian.PutUint32(hdr[8:12], uint32(len(body)))
	if _, err := conn.Write(hdr); err != nil {
		return err
	}
	if len(body) == 0 {
		return nil
	}
	_, err := conn.Write(body)
	return err
}

func TestOnRead_Close_FromCallback_NoDeadlock(t *testing.T) {
	closeReturned := make(chan struct{})

	bs := NewBaseServer("test:0", ServerHandlers{
		OnAccept: func(ziface.IChannel) bool { return true },
		OnRead: func(ch ziface.IChannel, _ ziface.IWireMessage) {
			ch.Close()
			close(closeReturned)
		},
	})
	ts := &testServer{BaseServer: bs}
	ch, serverConn := newTestChannelWithServer(t, ts)
	bs.AddChannel(ch)

	startDone := make(chan struct{})
	go func() {
		ch.Start()
		close(startDone)
	}()

	if err := writeTestPacketV0(serverConn, 1, []byte("x")); err != nil {
		t.Fatalf("write packet: %v", err)
	}

	select {
	case <-closeReturned:
	case <-time.After(2 * time.Second):
		t.Fatal("Close() called from OnRead deadlocked - readWg.Wait() blocks when called from read goroutine")
	}
	if !ch.isClose.Load() {
		t.Fatal("Close from OnRead must mark channel closed")
	}

	select {
	case <-startDone:
	case <-time.After(2 * time.Second):
		t.Fatal("Start() should exit after Close from OnRead")
	}
	if ch.readBuffer != nil {
		t.Fatal("readBuffer must be released after Start exits")
	}
}

func TestOnRead_Close_FromOtherGoroutine_NoDeadlock(t *testing.T) {
	readEntered := make(chan struct{})
	var readOnce atomic.Bool

	bs := NewBaseServer("test:0", ServerHandlers{
		OnAccept: func(ziface.IChannel) bool { return true },
		OnRead: func(_ ziface.IChannel, _ ziface.IWireMessage) {
			if readOnce.CompareAndSwap(false, true) {
				close(readEntered)
			}
		},
	})
	ts := &testServer{BaseServer: bs}
	ch, serverConn := newTestChannelWithServer(t, ts)
	bs.AddChannel(ch)

	startDone := make(chan struct{})
	go func() {
		ch.Start()
		close(startDone)
	}()

	if err := writeTestPacketV0(serverConn, 1, []byte("x")); err != nil {
		t.Fatalf("write packet: %v", err)
	}
	select {
	case <-readEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("OnRead not entered")
	}
	// OnRead 已返回；读 goroutine 应阻塞在下一轮 conn.Read，此时从其他 goroutine Close 不得自死锁。

	closeDone := make(chan struct{})
	go func() {
		ch.Close()
		close(closeDone)
	}()

	select {
	case <-closeDone:
	case <-time.After(2 * time.Second):
		t.Fatal("Close from other goroutine deadlocked while read loop active")
	}

	select {
	case <-startDone:
	case <-time.After(2 * time.Second):
		t.Fatal("Start() should exit after external Close")
	}
}
