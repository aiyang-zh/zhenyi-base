package zid

import (
	"math/rand"
	"os"
	"sync"
	"time"

	"github.com/sony/sonyflake"
)

var (
	sf   *sonyflake.Sonyflake
	once sync.Once
)

func Init(machineId uint16) {
	once.Do(func() {
		var st sonyflake.Settings

		// 如果传入了有效的 machineId，使用传入的
		if machineId > 0 {
			st.MachineID = func() (uint16, error) {
				return machineId, nil
			}
		}
		sf = sonyflake.NewSonyflake(st)
		if sf == nil {
			panic("sonyflake not created")
		}
	})
}

func Next() uint64 {
	if sf == nil {
		// 懒加载：如果忘记调 Init，给个默认初始化
		Init(0)
	}
	id, err := sf.NextID()
	if err == nil {
		return id
	}
	time.Sleep(5 * time.Millisecond)
	id, err = sf.NextID()
	if err == nil {
		return id
	}
	return uint64(time.Now().UnixNano())<<16 | uint64(os.Getpid()&0xFFFF) ^ rand.Uint64()
}
