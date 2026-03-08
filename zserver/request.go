package zserver

import "github.com/aiyang-zh/zhenyi-base/zpool"

// Request 封装一次客户端请求的上下文，包含连接、消息 ID、序号与负载数据。
type Request struct {
	conn    *Conn
	msgId   int32
	seqId   uint32
	data    []byte
	handler HandlerFunc // pool dispatch 用，避免闭包分配
}

// MsgId 返回本次请求的消息 ID。
func (r *Request) MsgId() int32 { return r.msgId }

// SeqId 返回本次请求的序列号，用于在业务侧做请求跟踪。
func (r *Request) SeqId() uint32 { return r.seqId }

// Data 返回请求 payload 的引用。
// 注意：返回的 slice 仅在当前 handler 执行期间有效，handler 返回后底层内存会被复用。
// 如需异步使用，请自行拷贝：copy(dst, req.Data())
func (r *Request) Data() []byte { return r.data }

// Conn 返回本次请求所在的连接封装。
func (r *Request) Conn() *Conn { return r.conn }

// Reply 向发送方回复消息。
func (r *Request) Reply(msgId int32, data []byte) {
	r.conn.Send(msgId, data)
}

var requestPool = zpool.NewPool(func() *Request {
	return &Request{data: make([]byte, 0, 128)}
})

func getRequest() *Request {
	return requestPool.Get()
}

func putRequest(r *Request) {
	r.conn = nil
	r.msgId = 0
	r.seqId = 0
	r.data = r.data[:0]
	r.handler = nil
	requestPool.Put(r)
}
