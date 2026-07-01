package zkcp

import (
	"net"

	"github.com/aiyang-zh/zhenyi-base/zerrs"
	"github.com/aiyang-zh/zhenyi-base/ziface"
	"github.com/aiyang-zh/zhenyi-base/zlog"
	"github.com/aiyang-zh/zhenyi-base/znet"
	"github.com/xtaci/kcp-go/v5"
	"go.uber.org/zap"
)

// Client 为 KCP 协议客户端，嵌入 BaseClient 并实现 Connect。
type Client struct {
	*znet.BaseClient
}

// NewClient 创建 KCP 客户端并连接 addr；失败返回错误。
// 默认 sync（Request）；可选 znet.WithAsyncMode() 启用 async（Read），与 ziface.ModeAsync 对应。
func NewClient(addr string, opts ...znet.ClientOption) (ziface.IClient, error) {
	client := &Client{
		BaseClient: znet.NewBaseClient(opts...),
	}
	err := client.Connect(addr)
	if err != nil {
		return nil, err
	}
	return client, nil
}

// Connect 使用 KCP 连接远端（FEC 关闭，与 Server 一致）。
// DialTimeout>0 时经 net.Dialer 建 UDP 再包装为 KCP；0 则使用 kcp.DialWithOptions。
func (n *Client) Connect(addr string) error {
	var (
		conn *kcp.UDPSession
		err  error
	)
	if timeout := n.DialTimeout(); timeout > 0 {
		d := net.Dialer{Timeout: timeout}
		udpConn, dialErr := d.Dial("udp", addr)
		if dialErr != nil {
			zlog.Error("Failed to dial KCP server",
				zap.String("addr", addr),
				zap.Error(dialErr))
			return zerrs.Wrap(dialErr, zerrs.ErrTypeNetwork, "failed to dial KCP server")
		}
		pc, ok := udpConn.(net.PacketConn)
		if !ok {
			if closeErr := udpConn.Close(); closeErr != nil {
				zlog.Warn("Failed to close non-packet UDP conn after dial",
					zap.String("addr", addr),
					zap.Error(closeErr))
			}
			return zerrs.New(zerrs.ErrTypeNetwork, "kcp dial: underlying conn is not PacketConn")
		}
		conn, err = kcp.NewConn2(udpConn.RemoteAddr(), nil, 0, 0, pc)
		if err != nil {
			if closeErr := pc.Close(); closeErr != nil {
				zlog.Warn("Failed to close UDP conn after KCP session setup",
					zap.String("addr", addr),
					zap.Error(closeErr))
			}
			zlog.Error("Failed to create KCP session",
				zap.String("addr", addr),
				zap.Error(err))
			return zerrs.Wrap(err, zerrs.ErrTypeNetwork, "failed to create KCP session")
		}
	} else {
		conn, err = kcp.DialWithOptions(addr, nil, 0, 0)
		if err != nil {
			zlog.Error("Failed to dial KCP server",
				zap.String("addr", addr),
				zap.Error(err))
			return zerrs.Wrap(err, zerrs.ErrTypeNetwork, "failed to dial KCP server")
		}
	}
	conn.SetNoDelay(1, 20, 2, 1)
	conn.SetWindowSize(128, 128)
	conn.SetACKNoDelay(true)
	conn.SetMtu(1200)

	// 5. 客户端特有：设置写入缓冲区
	// 压测时瞬间发送大量包，系统 UDP 缓冲区可能会满
	conn.SetWriteBuffer(4 * 1024 * 1024) // 4MB
	conn.SetReadBuffer(4 * 1024 * 1024)  // 4MB
	n.SetConn(conn)

	zlog.Info("KCP client connected successfully", zap.String("addr", addr))
	return nil
}
