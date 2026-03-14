//go:build linux

package zreactor

import (
	"syscall"
	"testing"

	"golang.org/x/sys/unix"
)

func TestPoller_NewClose(t *testing.T) {
	p, err := NewPoller()
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	if err := p.Close(); err != nil {
		t.Fatal("second Close should be no-op")
	}
}

func TestPoller_AddRemoveWait(t *testing.T) {
	p, err := NewPoller()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	var fds [2]int
	if err := unix.Pipe(fds[:]); err != nil {
		t.Fatal(err)
	}
	defer unix.Close(fds[0])
	defer unix.Close(fds[1])
	if err := unix.SetNonblock(fds[0], true); err != nil {
		t.Fatal(err)
	}
	if err := p.Add(fds[0]); err != nil {
		t.Fatal(err)
	}
	_, _ = unix.Write(fds[1], []byte{1})
	ready, err := p.Wait(100)
	if err != nil {
		t.Fatal(err)
	}
	if len(ready) == 0 {
		t.Fatal("expected at least one ready fd")
	}
	if err := p.Remove(fds[0]); err != nil {
		t.Fatal(err)
	}
	if err := p.Remove(fds[0]); err != nil {
		t.Fatal("Remove already removed fd should ignore EBADF/ENOENT")
	}
}

func TestPoller_WaitWithEvents(t *testing.T) {
	p, err := NewPoller()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	var fds [2]int
	if err := unix.Pipe(fds[:]); err != nil {
		t.Fatal(err)
	}
	defer unix.Close(fds[0])
	defer unix.Close(fds[1])
	if err := unix.SetNonblock(fds[0], true); err != nil {
		t.Fatal(err)
	}
	if err := p.Add(fds[0]); err != nil {
		t.Fatal(err)
	}
	_, _ = unix.Write(fds[1], []byte{1})
	ready, err := p.WaitWithEvents(100)
	if err != nil {
		t.Fatal(err)
	}
	if len(ready) == 0 {
		t.Fatal("expected at least one ready event")
	}
	if ready[0].Fd != fds[0] {
		t.Errorf("ready fd = %d, want %d", ready[0].Fd, fds[0])
	}
	if ready[0].Events&unix.EPOLLIN == 0 {
		t.Errorf("expected EPOLLIN in events, got %x", ready[0].Events)
	}
}

func TestPoller_Remove_InvalidFd(t *testing.T) {
	p, err := NewPoller()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if err := p.Remove(-1); err != nil {
		t.Fatal("Remove(-1) should not return error (EBADF ignored)")
	}
	if err := p.Remove(int(syscall.Stderr) + 99999); err != nil {
		t.Fatal("Remove invalid fd should not return error")
	}
}

func BenchmarkPoller_Wait(b *testing.B) {
	p, err := NewPoller()
	if err != nil {
		b.Fatal(err)
	}
	defer p.Close()
	efd, err := unix.Eventfd(0, unix.EFD_NONBLOCK|unix.EFD_CLOEXEC)
	if err != nil {
		b.Fatal(err)
	}
	defer unix.Close(efd)
	if err := p.Add(efd); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = p.Wait(0)
	}
}
