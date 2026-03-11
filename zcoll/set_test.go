package zcoll

import (
	"strings"
	"sync"
	"testing"
)

// TestSet_NewSet 测试创建空集合
func TestSet_NewSet(t *testing.T) {
	s := NewSet[int]()
	if s == nil {
		t.Fatal("expected non-nil set")
	}
	if n := s.Count(); n != 0 {
		t.Errorf("expected count 0, got %d", n)
	}
}

// TestSet_NewSetByList 测试通过列表创建集合
func TestSet_NewSetByList(t *testing.T) {
	values := []int{1, 2, 3, 4, 5}
	s := NewSetByList(values)
	if n := s.Count(); n != 5 {
		t.Errorf("expected count 5, got %d", n)
	}
	if !s.Contains(1) || !s.Contains(5) {
		t.Error("expected contains 1 and 5")
	}
	if s.Contains(6) {
		t.Error("expected not contains 6")
	}
}

// TestSet_Add 测试添加元素
func TestSet_Add(t *testing.T) {
	s := NewSet[string]()
	s.Add("apple")
	s.Add("banana")
	s.Add("apple") // 重复添加

	if n := s.Count(); n != 2 {
		t.Errorf("expected count 2, got %d", n)
	}
	if !s.Contains("apple") || !s.Contains("banana") {
		t.Error("expected contains apple and banana")
	}
}

// TestSet_Remove 测试删除元素
func TestSet_Remove(t *testing.T) {
	s := NewSet[int]()
	s.Add(1)
	s.Add(2)
	s.Add(3)

	s.Remove(2)
	if n := s.Count(); n != 2 {
		t.Errorf("expected count 2, got %d", n)
	}
	if s.Contains(2) {
		t.Error("expected not contains 2")
	}
	if !s.Contains(1) || !s.Contains(3) {
		t.Error("expected contains 1 and 3")
	}
}

// TestSet_RemoveMany 测试批量删除
func TestSet_RemoveMany(t *testing.T) {
	s := NewSet[int]()
	for i := 1; i <= 10; i++ {
		s.Add(i)
	}

	s.RemoveMany([]int{2, 4, 6, 8, 10})
	if n := s.Count(); n != 5 {
		t.Errorf("expected count 5, got %d", n)
	}
	if s.Contains(2) {
		t.Error("expected not contains 2")
	}
	if !s.Contains(1) || !s.Contains(3) {
		t.Error("expected contains 1 and 3")
	}
}

// TestSet_Clear 测试清空集合
func TestSet_Clear(t *testing.T) {
	s := NewSet[int]()
	s.Add(1)
	s.Add(2)
	s.Add(3)

	s.Clear()
	if n := s.Count(); n != 0 {
		t.Errorf("expected count 0, got %d", n)
	}
	if s.Contains(1) {
		t.Error("expected not contains 1")
	}
}

// TestSet_Contains 测试包含检查
func TestSet_Contains(t *testing.T) {
	s := NewSet[int]()
	s.Add(1)
	s.Add(2)

	if !s.Contains(1) || !s.Contains(2) {
		t.Error("expected contains 1 and 2")
	}
	if s.Contains(3) {
		t.Error("expected not contains 3")
	}
}

// TestSet_List 测试获取元素列表
func TestSet_List(t *testing.T) {
	s := NewSet[int]()
	s.Add(1)
	s.Add(2)
	s.Add(3)

	list := s.List()
	if len(list) != 3 {
		t.Errorf("expected len 3, got %d", len(list))
	}

	// 验证所有元素都在列表中（顺序不保证）
	contains := func(lst []int, val int) bool {
		for _, v := range lst {
			if v == val {
				return true
			}
		}
		return false
	}
	if !contains(list, 1) || !contains(list, 2) || !contains(list, 3) {
		t.Error("expected list to contain 1, 2, 3")
	}
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
	if n := s1.Count(); n != 3 {
		t.Errorf("expected count 3, got %d", n)
	}
	if !s1.Contains(1) || !s1.Contains(2) || !s1.Contains(3) {
		t.Error("expected contains 1, 2, 3")
	}
}

// TestSet_MergeByList 测试通过列表合并
func TestSet_MergeByList(t *testing.T) {
	s := NewSet[int]()
	s.Add(1)
	s.Add(2)

	s.MergeByList([]int{2, 3, 4})
	if n := s.Count(); n != 4 {
		t.Errorf("expected count 4, got %d", n)
	}
	if !s.Contains(1) || !s.Contains(4) {
		t.Error("expected contains 1 and 4")
	}
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
	if n := s1.Count(); n != 1 {
		t.Errorf("expected count 1, got %d", n)
	}
	if !s1.Contains(1) || s1.Contains(2) || s1.Contains(3) {
		t.Error("expected only contains 1")
	}
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
	if n := result.Count(); n != 3 {
		t.Errorf("expected result count 3, got %d", n)
	}
	if !result.Contains(1) || !result.Contains(2) || !result.Contains(3) {
		t.Error("expected result contains 1, 2, 3")
	}
	if s1.Count() != 2 || s2.Count() != 2 {
		t.Error("original sets should be unchanged")
	}
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
	if n := result.Count(); n != 2 {
		t.Errorf("expected result count 2, got %d", n)
	}
	if !result.Contains(2) || !result.Contains(3) || result.Contains(1) || result.Contains(4) {
		t.Error("expected result contains only 2, 3")
	}
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
	if n := result.Count(); n != 1 {
		t.Errorf("expected result count 1, got %d", n)
	}
	if !result.Contains(1) || result.Contains(2) || result.Contains(3) {
		t.Error("expected result contains only 1")
	}
	if s1.Count() != 3 {
		t.Error("s1 should be unchanged")
	}
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
	if n := result.Count(); n != 2 {
		t.Errorf("expected result count 2, got %d", n)
	}
	if !result.Contains(1) || !result.Contains(4) || result.Contains(2) || result.Contains(3) {
		t.Error("expected result contains only 1, 4")
	}
}

// TestSet_String 测试字符串表示
func TestSet_String(t *testing.T) {
	s := NewSet[int]()
	s.Add(1)
	s.Add(2)

	str := s.String()
	if str == "" {
		t.Error("expected non-empty string")
	}
	if !strings.Contains(str, "1") || !strings.Contains(str, "2") {
		t.Errorf("expected string to contain 1 and 2, got %q", str)
	}
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
	if count < 0 || count > 100 {
		t.Errorf("count out of range: %d", count)
	}
}

// TestSet_GenericTypes 测试不同类型的泛型
func TestSet_GenericTypes(t *testing.T) {
	strSet := NewSet[string]()
	strSet.Add("hello")
	if !strSet.Contains("hello") {
		t.Error("expected strSet contains hello")
	}

	intSet := NewSet[int]()
	intSet.Add(42)
	if !intSet.Contains(42) {
		t.Error("expected intSet contains 42")
	}

	floatSet := NewSet[float64]()
	floatSet.Add(3.14)
	if !floatSet.Contains(3.14) {
		t.Error("expected floatSet contains 3.14")
	}
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
