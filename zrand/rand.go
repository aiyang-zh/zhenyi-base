package zrand

import (
	"math/rand"
)

func RandomList[T any](infos []T) T {
	if len(infos) == 0 {
		var zero T
		return zero
	}
	i := rand.Int63n(int64(len(infos)))
	return infos[i]
}

type Number interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 |
		~float32 | ~float64
}

// Range 泛型随机函数
func Range[T Number](min, max T) T {
	if min == max {
		return min
	}
	var zero T
	switch any(zero).(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		// 整数逻辑 [min, max)
		iMin := int64(min)
		iMax := int64(max)
		if iMin > iMax {
			iMin, iMax = iMax, iMin
		}
		return T(rand.Int63n(iMax-iMin) + iMin)

	default:
		// 浮点数逻辑 [min, max)
		fMin := float64(min)
		fMax := float64(max)
		if fMin > fMax {
			fMin, fMax = fMax, fMin
		}
		return T(fMin + rand.Float64()*(fMax-fMin))
	}
}
