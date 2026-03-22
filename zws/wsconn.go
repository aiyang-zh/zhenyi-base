package zws

import (
	"net"
	"time"

	"github.com/gorilla/websocket"
)

// wsConn 将 *websocket.Conn 适配为 net.Conn：Read/Write 走二进制 WebSocket 帧，
// 与浏览器及标准 WebSocket 对端互通。勿再使用 conn.NetConn() 做裸 TCP 读写（会破坏帧同步）。
type wsConn struct {
	c       *websocket.Conn
	readBuf []byte
}

func newWSConn(c *websocket.Conn) net.Conn {
	return &wsConn{c: c}
}

func (w *wsConn) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if len(w.readBuf) > 0 {
		n := copy(p, w.readBuf)
		w.readBuf = w.readBuf[n:]
		return n, nil
	}
	for {
		mt, data, err := w.c.ReadMessage()
		if err != nil {
			return 0, err
		}
		switch mt {
		case websocket.BinaryMessage, websocket.TextMessage:
			if len(data) == 0 {
				continue
			}
			n := copy(p, data)
			if n < len(data) {
				w.readBuf = data[n:]
			}
			return n, nil
		default:
			continue
		}
	}
}

func (w *wsConn) Write(p []byte) (int, error) {
	if err := w.c.WriteMessage(websocket.BinaryMessage, p); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (w *wsConn) Close() error {
	return w.c.Close()
}

func (w *wsConn) LocalAddr() net.Addr {
	return w.c.LocalAddr()
}

func (w *wsConn) RemoteAddr() net.Addr {
	return w.c.RemoteAddr()
}

func (w *wsConn) SetDeadline(t time.Time) error {
	if err := w.c.SetReadDeadline(t); err != nil {
		return err
	}
	return w.c.SetWriteDeadline(t)
}

func (w *wsConn) SetReadDeadline(t time.Time) error {
	return w.c.SetReadDeadline(t)
}

func (w *wsConn) SetWriteDeadline(t time.Time) error {
	return w.c.SetWriteDeadline(t)
}
