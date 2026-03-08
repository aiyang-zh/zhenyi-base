package znet

import (
	"github.com/aiyang-zh/zhenyi-base/zpool"
)

var parseDataPool = zpool.NewPool(func() *ParseData {
	return &ParseData{
		Message:      GetNetMessage(),
		OwnedBuffers: make([]*zpool.Buffer, 0, 2),
	}
})

// GetParseData 从池中获取 ParseData，用于协议解析时复用，避免每次分配。
func GetParseData() *ParseData {
	p := parseDataPool.Get()
	p.Message = GetNetMessage()
	return p
}

// PutParseData 将 ParseData 归还到池，并释放其持有的 Buffer 与 Message。
func PutParseData(p *ParseData) {
	if p == nil {
		return
	}

	for _, buf := range p.OwnedBuffers {
		buf.Release()
	}
	p.OwnedBuffers = p.OwnedBuffers[:0]

	if p.Message != nil {
		p.Message.Release()
		p.Message = nil
	}
	p.Error = nil

	parseDataPool.Put(p)
}
