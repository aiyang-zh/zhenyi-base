package zcoll

import (
	"fmt"
	"sync"
	"unsafe"
)

type Set[T comparable] struct {
	info map[T]struct{}
	lock sync.RWMutex
}

func NewSet[T comparable]() *Set[T] {
	return &Set[T]{
		info: make(map[T]struct{}),
	}
}

func NewSetByList[T comparable](values []T) *Set[T] {
	s := &Set[T]{
		info: make(map[T]struct{}),
	}
	for _, value := range values {
		s.info[value] = struct{}{}
	}
	return s
}

func (s *Set[T]) String() string {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return fmt.Sprintf("%v", s.List())
}

func (s *Set[T]) Add(value T) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.info[value] = struct{}{}
}

func (s *Set[T]) Count() int {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return len(s.info)
}

func (s *Set[T]) Remove(value T) {
	s.lock.Lock()
	defer s.lock.Unlock()
	delete(s.info, value)
}
func (s *Set[T]) RemoveMany(values []T) {
	for _, value := range values {
		s.Remove(value)
	}
}
func (s *Set[T]) Clear() {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.info = make(map[T]struct{})
}

func (s *Set[T]) List() []T {
	s.lock.RLock()
	defer s.lock.RUnlock()
	items := make([]T, 0, len(s.info))
	for k := range s.info {
		items = append(items, k)
	}
	return items
}

// ✅ ForEach 零分配遍历（Iterator 模式）
// fn 返回 false 时停止遍历
func (s *Set[T]) ForEach(fn func(T) bool) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	for k := range s.info {
		if !fn(k) {
			break
		}
	}
}

// ✅ Range 零分配遍历（类似 sync.Map）
func (s *Set[T]) Range(fn func(T)) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	for k := range s.info {
		fn(k)
	}
}

func (s *Set[T]) Merge(s1 *Set[T]) {
	if s == s1 {
		return
	}
	// 按指针地址锁序，避免 AB-BA 死锁
	first, second := s, s1
	if uintptr(unsafe.Pointer(s)) > uintptr(unsafe.Pointer(s1)) {
		first, second = s1, s
	}
	first.lock.Lock()
	defer first.lock.Unlock()
	second.lock.Lock()
	defer second.lock.Unlock()

	for item := range s1.info {
		s.info[item] = struct{}{}
	}
}

func (s *Set[T]) MergeByList(values []T) {
	s.lock.Lock()
	defer s.lock.Unlock()
	for _, v := range values {
		s.info[v] = struct{}{}
	}
}

func (s *Set[T]) Contains(item T) bool {
	s.lock.RLock()
	defer s.lock.RUnlock()
	if _, ok := s.info[item]; ok {
		return true
	} else {
		return false
	}
}
func (s *Set[T]) Subtract(other *Set[T]) {
	if s == other {
		s.lock.Lock()
		s.info = make(map[T]struct{})
		s.lock.Unlock()
		return
	}
	first, second := s, other
	if uintptr(unsafe.Pointer(s)) > uintptr(unsafe.Pointer(other)) {
		first, second = other, s
	}
	first.lock.Lock()
	defer first.lock.Unlock()
	second.lock.Lock()
	defer second.lock.Unlock()

	for item := range other.info {
		delete(s.info, item)
	}
}

// Union (并集): 返回新集合 s + other
func (s *Set[T]) Union(other *Set[T]) *Set[T] {
	first, second := s, other
	if s != other && uintptr(unsafe.Pointer(s)) > uintptr(unsafe.Pointer(other)) {
		first, second = other, s
	}
	first.lock.RLock()
	if s != other {
		second.lock.RLock()
	}

	merged := make(map[T]struct{}, len(s.info)+len(other.info))
	for k := range s.info {
		merged[k] = struct{}{}
	}
	for k := range other.info {
		merged[k] = struct{}{}
	}

	if s != other {
		second.lock.RUnlock()
	}
	first.lock.RUnlock()

	return &Set[T]{
		info: merged,
	}
}

// Intersect (交集): 返回新集合，仅包含 s 和 other 都有的元素
func (s *Set[T]) Intersect(other *Set[T]) *Set[T] {
	first, second := s, other
	if s != other && uintptr(unsafe.Pointer(s)) > uintptr(unsafe.Pointer(other)) {
		first, second = other, s
	}
	first.lock.RLock()
	if s != other {
		second.lock.RLock()
	}
	defer func() {
		if s != other {
			second.lock.RUnlock()
		}
		first.lock.RUnlock()
	}()

	// 优化：遍历较小的集合，查找较大的集合
	// 这样 CPU 缓存命中率更高，循环次数更少
	lenA := len(s.info)
	lenB := len(other.info)

	var smaller, larger map[T]struct{}
	if lenA < lenB {
		smaller, larger = s.info, other.info
	} else {
		smaller, larger = other.info, s.info
	}

	// 预分配：容量顶多是较小集合的大小
	result := make(map[T]struct{}, len(smaller))

	for k := range smaller {
		if _, ok := larger[k]; ok {
			result[k] = struct{}{}
		}
	}

	return &Set[T]{
		info: result,
	}
}

// Difference (差集): 返回新集合，包含在 s 中但不在 other 中的元素
func (s *Set[T]) Difference(other *Set[T]) *Set[T] {
	first, second := s, other
	if s != other && uintptr(unsafe.Pointer(s)) > uintptr(unsafe.Pointer(other)) {
		first, second = other, s
	}
	first.lock.RLock()
	if s != other {
		second.lock.RLock()
	}
	defer func() {
		if s != other {
			second.lock.RUnlock()
		}
		first.lock.RUnlock()
	}()

	result := make(map[T]struct{}, len(s.info))

	for k := range s.info {
		if _, ok := other.info[k]; !ok {
			result[k] = struct{}{}
		}
	}

	return &Set[T]{
		info: result,
	}
}

// SymmetricDifference (对称差集): 返回新集合，包含只在 s 或只在 other 中的元素 (A ^ B)
func (s *Set[T]) SymmetricDifference(other *Set[T]) *Set[T] {
	first, second := s, other
	if s != other && uintptr(unsafe.Pointer(s)) > uintptr(unsafe.Pointer(other)) {
		first, second = other, s
	}
	first.lock.RLock()
	if s != other {
		second.lock.RLock()
	}
	defer func() {
		if s != other {
			second.lock.RUnlock()
		}
		first.lock.RUnlock()
	}()

	// 结果集大小未知，保守估计
	result := make(map[T]struct{}, len(s.info))

	for k := range s.info {
		if _, ok := other.info[k]; !ok {
			result[k] = struct{}{}
		}
	}
	for k := range other.info {
		if _, ok := s.info[k]; !ok {
			result[k] = struct{}{}
		}
	}

	return &Set[T]{
		info: result,
	}
}
