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

// GetParseData 从池获取
func GetParseData() *ParseData {
	p := parseDataPool.Get()
	p.Message = GetNetMessage()
	return p
}

// PutParseData 归还（自动清理所有持有的 buffer 和消息）
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
