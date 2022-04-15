package utils

// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//就是把 container/heap 的代码改成了泛型实现, 并使用了数组来存储数据。
// 性能来说应该比原来包高一些，因为少了 interface打包和解包 的损耗。
// 之所以有这个转换，是打算优化一下smux中相关shaper。

// The Heap type describes the requirements
// for a type using the routines in this package.
// Any type that implements it may be used as a
// min-heap with the following invariants (established after
// Init has been called or if the data is empty or sorted):
//
//	!h.Less(j, i) for 0 <= i < h.Len() and 2*i+1 <= j <= 2*i+2 and j < h.Len()
//
// Note that Push and Pop in this interface are for package heap's
// implementation to call. To add and remove things from the heap,
// use heap.Push and heap.Pop.
//
// 实际上我们这个包装已经类似 优先队列了, 至于如何优先取决于 LessFunc
type Heap[T any] struct {
	LessFunc func(i, j int, a []T) bool

	Array []T
}

// Init establishes the heap invariants required by the other routines in this package.
// Init is idempotent with respect to the heap invariants
// and may be called whenever the heap invariants may have been invalidated.
// The complexity is O(n) where n = h.Len().
func (h *Heap[T]) Init() {
	// heapify
	n := h.Len()
	for i := n/2 - 1; i >= 0; i-- {
		h.down(i, n)
	}
}

func (h *Heap[T]) Len() int {
	return len(h.Array)
}

func (h *Heap[T]) rawPush(x T) {
	h.Array = append(h.Array, x)
}

func (h *Heap[T]) rawPop() T {
	old := h.Array
	n := len(old)
	x := old[n-1]
	h.Array = old[0 : n-1]
	return x
}

func (h *Heap[T]) swap(i, j int) {

	h.Array[i], h.Array[j] = h.Array[j], h.Array[i]
}

// Push pushes the element x onto the heap.
// The complexity is O(log n) where n = h.Len().
func (h *Heap[T]) Push(x T) {
	h.rawPush(x)
	h.up(h.Len() - 1)
}

// Pop removes and returns the minimum element (according to Less) from the heap.
// The complexity is O(log n) where n = h.Len().
// Pop is equivalent to Remove(h, 0).
func (h *Heap[T]) Pop() T {
	n := h.Len() - 1
	h.swap(0, n)
	h.down(0, n)
	return h.rawPop()
}

// Remove removes and returns the element at index i from the heap.
// The complexity is O(log n) where n = h.Len().
func (h *Heap[T]) Remove(i int) T {
	n := h.Len() - 1
	if n != i {
		h.swap(i, n)
		if !h.down(i, n) {
			h.up(i)
		}
	}
	return h.Pop()
}

// Fix re-establishes the heap ordering after the element at index i has changed its value.
// Changing the value of the element at index i and then calling Fix is equivalent to,
// but less expensive than, calling Remove(h, i) followed by a Push of the new value.
// The complexity is O(log n) where n = h.Len().
func (h *Heap[T]) Fix(i int) {
	if !h.down(i, h.Len()) {
		h.up(i)
	}
}

func (h *Heap[T]) up(j int) {
	for {
		i := (j - 1) / 2 // parent
		if i == j || !h.LessFunc(j, i, h.Array) {
			break
		}
		h.swap(i, j)
		j = i
	}
}

func (h *Heap[T]) down(i0, n int) bool {
	i := i0
	for {
		j1 := 2*i + 1
		if j1 >= n || j1 < 0 { // j1 < 0 after int overflow
			break
		}
		j := j1 // left child
		if j2 := j1 + 1; j2 < n && h.LessFunc(j2, j1, h.Array) {
			j = j2 // = 2*i + 2  // right child
		}
		if !h.LessFunc(j, i, h.Array) {
			break
		}
		h.swap(i, j)
		i = j
	}
	return i > i0
}
