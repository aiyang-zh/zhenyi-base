package zcoll

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSet_NewSet 测试创建空集合
func TestSet_NewSet(t *testing.T) {
	s := NewSet[int]()
	assert.NotNil(t, s)
	assert.Equal(t, 0, s.Count())
}

// TestSet_NewSetByList 测试通过列表创建集合
func TestSet_NewSetByList(t *testing.T) {
	values := []int{1, 2, 3, 4, 5}
	s := NewSetByList(values)
	assert.Equal(t, 5, s.Count())
	assert.True(t, s.Contains(1))
	assert.True(t, s.Contains(5))
	assert.False(t, s.Contains(6))
}

// TestSet_Add 测试添加元素
func TestSet_Add(t *testing.T) {
	s := NewSet[string]()
	s.Add("apple")
	s.Add("banana")
	s.Add("apple") // 重复添加

	assert.Equal(t, 2, s.Count())
	assert.True(t, s.Contains("apple"))
	assert.True(t, s.Contains("banana"))
}

// TestSet_Remove 测试删除元素
func TestSet_Remove(t *testing.T) {
	s := NewSet[int]()
	s.Add(1)
	s.Add(2)
	s.Add(3)

	s.Remove(2)
	assert.Equal(t, 2, s.Count())
	assert.False(t, s.Contains(2))
	assert.True(t, s.Contains(1))
	assert.True(t, s.Contains(3))
}

// TestSet_RemoveMany 测试批量删除
func TestSet_RemoveMany(t *testing.T) {
	s := NewSet[int]()
	for i := 1; i <= 10; i++ {
		s.Add(i)
	}

	s.RemoveMany([]int{2, 4, 6, 8, 10})
	assert.Equal(t, 5, s.Count())
	assert.False(t, s.Contains(2))
	assert.True(t, s.Contains(1))
	assert.True(t, s.Contains(3))
}

// TestSet_Clear 测试清空集合
func TestSet_Clear(t *testing.T) {
	s := NewSet[int]()
	s.Add(1)
	s.Add(2)
	s.Add(3)

	s.Clear()
	assert.Equal(t, 0, s.Count())
	assert.False(t, s.Contains(1))
}

// TestSet_Contains 测试包含检查
func TestSet_Contains(t *testing.T) {
	s := NewSet[int]()
	s.Add(1)
	s.Add(2)

	assert.True(t, s.Contains(1))
	assert.True(t, s.Contains(2))
	assert.False(t, s.Contains(3))
}

// TestSet_List 测试获取元素列表
func TestSet_List(t *testing.T) {
	s := NewSet[int]()
	s.Add(1)
	s.Add(2)
	s.Add(3)

	list := s.List()
	assert.Equal(t, 3, len(list))

	// 验证所有元素都在列表中（顺序不保证）
	contains := func(list []int, val int) bool {
		for _, v := range list {
			if v == val {
				return true
			}
		}
		return false
	}
	assert.True(t, contains(list, 1))
	assert.True(t, contains(list, 2))
	assert.True(t, contains(list, 3))
}

// TestSet_Merge 测试合并集合
func TestSet_Merge(t *testing.T) {
	s1 := NewSet[int]()
	s1.Add(1)
	s1.Add(2)

	s2 := NewSet[int]()
	s2.Add(2)
	s2.Add(3)

	s1.Merge(s2)
	assert.Equal(t, 3, s1.Count())
	assert.True(t, s1.Contains(1))
	assert.True(t, s1.Contains(2))
	assert.True(t, s1.Contains(3))
}

// TestSet_MergeByList 测试通过列表合并
func TestSet_MergeByList(t *testing.T) {
	s := NewSet[int]()
	s.Add(1)
	s.Add(2)

	s.MergeByList([]int{2, 3, 4})
	assert.Equal(t, 4, s.Count())
	assert.True(t, s.Contains(1))
	assert.True(t, s.Contains(4))
}

// TestSet_Subtract 测试差集（原地修改）
func TestSet_Subtract(t *testing.T) {
	s1 := NewSet[int]()
	s1.Add(1)
	s1.Add(2)
	s1.Add(3)

	s2 := NewSet[int]()
	s2.Add(2)
	s2.Add(3)

	s1.Subtract(s2)
	assert.Equal(t, 1, s1.Count())
	assert.True(t, s1.Contains(1))
	assert.False(t, s1.Contains(2))
	assert.False(t, s1.Contains(3))
}

// TestSet_Union 测试并集
func TestSet_Union(t *testing.T) {
	s1 := NewSet[int]()
	s1.Add(1)
	s1.Add(2)

	s2 := NewSet[int]()
	s2.Add(2)
	s2.Add(3)

	result := s1.Union(s2)
	assert.Equal(t, 3, result.Count())
	assert.True(t, result.Contains(1))
	assert.True(t, result.Contains(2))
	assert.True(t, result.Contains(3))

	// 验证原集合未被修改
	assert.Equal(t, 2, s1.Count())
	assert.Equal(t, 2, s2.Count())
}

// TestSet_Intersect 测试交集
func TestSet_Intersect(t *testing.T) {
	s1 := NewSet[int]()
	s1.Add(1)
	s1.Add(2)
	s1.Add(3)

	s2 := NewSet[int]()
	s2.Add(2)
	s2.Add(3)
	s2.Add(4)

	result := s1.Intersect(s2)
	assert.Equal(t, 2, result.Count())
	assert.True(t, result.Contains(2))
	assert.True(t, result.Contains(3))
	assert.False(t, result.Contains(1))
	assert.False(t, result.Contains(4))
}

// TestSet_Difference 测试差集（返回新集合）
func TestSet_Difference(t *testing.T) {
	s1 := NewSet[int]()
	s1.Add(1)
	s1.Add(2)
	s1.Add(3)

	s2 := NewSet[int]()
	s2.Add(2)
	s2.Add(3)

	result := s1.Difference(s2)
	assert.Equal(t, 1, result.Count())
	assert.True(t, result.Contains(1))
	assert.False(t, result.Contains(2))
	assert.False(t, result.Contains(3))

	// 验证原集合未被修改
	assert.Equal(t, 3, s1.Count())
}

// TestSet_SymmetricDifference 测试对称差集
func TestSet_SymmetricDifference(t *testing.T) {
	s1 := NewSet[int]()
	s1.Add(1)
	s1.Add(2)
	s1.Add(3)

	s2 := NewSet[int]()
	s2.Add(2)
	s2.Add(3)
	s2.Add(4)

	result := s1.SymmetricDifference(s2)
	assert.Equal(t, 2, result.Count())
	assert.True(t, result.Contains(1))
	assert.True(t, result.Contains(4))
	assert.False(t, result.Contains(2))
	assert.False(t, result.Contains(3))
}

// TestSet_String 测试字符串表示
func TestSet_String(t *testing.T) {
	s := NewSet[int]()
	s.Add(1)
	s.Add(2)

	str := s.String()
	assert.NotEmpty(t, str)
	// 顺序不保证，只验证包含元素
	assert.Contains(t, str, "1")
	assert.Contains(t, str, "2")
}

// TestSet_ConcurrentAccess 测试并发访问安全性
func TestSet_ConcurrentAccess(t *testing.T) {
	s := NewSet[int]()
	var wg sync.WaitGroup

	// 并发添加
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(val int) {
			defer wg.Done()
			s.Add(val)
		}(i)
	}

	// 并发读取
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(val int) {
			defer wg.Done()
			s.Contains(val)
		}(i)
	}

	// 并发删除
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(val int) {
			defer wg.Done()
			s.Remove(val)
		}(i + 50)
	}

	wg.Wait()

	// 验证至少有一些元素（具体数量取决于竞态）
	count := s.Count()
	assert.GreaterOrEqual(t, count, 0)
	assert.LessOrEqual(t, count, 100)
}

// TestSet_GenericTypes 测试不同类型的泛型
func TestSet_GenericTypes(t *testing.T) {
	// String类型
	strSet := NewSet[string]()
	strSet.Add("hello")
	assert.True(t, strSet.Contains("hello"))

	// Int类型
	intSet := NewSet[int]()
	intSet.Add(42)
	assert.True(t, intSet.Contains(42))

	// Float类型
	floatSet := NewSet[float64]()
	floatSet.Add(3.14)
	assert.True(t, floatSet.Contains(3.14))
}

// BenchmarkSet_Add 基准测试：添加元素
func BenchmarkSet_Add(b *testing.B) {
	s := NewSet[int]()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		s.Add(i)
	}
}

// BenchmarkSet_Contains 基准测试：查询元素
func BenchmarkSet_Contains(b *testing.B) {
	s := NewSet[int]()
	for i := 0; i < 10000; i++ {
		s.Add(i)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		s.Contains(i % 10000)
	}
}

// BenchmarkSet_Union 基准测试：并集操作
func BenchmarkSet_Union(b *testing.B) {
	s1 := NewSet[int]()
	s2 := NewSet[int]()
	for i := 0; i < 1000; i++ {
		s1.Add(i)
		s2.Add(i + 500)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = s1.Union(s2)
	}
}

// BenchmarkSet_Intersect 基准测试：交集操作
func BenchmarkSet_Intersect(b *testing.B) {
	s1 := NewSet[int]()
	s2 := NewSet[int]()
	for i := 0; i < 1000; i++ {
		s1.Add(i)
		s2.Add(i + 500)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = s1.Intersect(s2)
	}
}

func BenchmarkSet_List(b *testing.B) {
	s := NewSet[int]()
	for i := 0; i < 1000; i++ {
		s.Add(i)
	}

	b.Run("List()", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			list := s.List()
			_ = list
		}
	})

	b.Run("ForEach()", func(b *testing.B) {
		b.ReportAllocs()
		var sum int
		for i := 0; i < b.N; i++ {
			sum = 0
			s.ForEach(func(v int) bool {
				sum += v
				return true
			})
		}
		_ = sum
	})

	b.Run("Range()", func(b *testing.B) {
		b.ReportAllocs()
		var sum int
		for i := 0; i < b.N; i++ {
			sum = 0
			s.Range(func(v int) {
				sum += v
			})
		}
		_ = sum
	})
}

func BenchmarkSet_Merge(b *testing.B) {
	s1 := NewSet[int]()
	s2 := NewSet[int]()
	for i := 0; i < 1000; i++ {
		s1.Add(i)
		s2.Add(i + 500)
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		s := NewSet[int]()
		for j := 0; j < 100; j++ {
			s.Add(j)
		}
		s.Merge(s2)
	}
}
