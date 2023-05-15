package txcounter

import (
	"container/heap"
)

// ensure we implement heap.Interface (compile error otherwise)
var _ heap.Interface = (*TxHeap)(nil)

// An TxHeap is a min-heap of float32 TX size values.
type TxHeap []*TxCounterTransaction

func (h TxHeap) Len() int { return len(h) }

// func (h TxHeap) Less(i, j int) bool { return h[i].SizeBch < h[j].SizeBch }
func (h TxHeap) Less(i, j int) bool { return h[i].SizeBch >= h[j].SizeBch } // we want a heap with maximum on top
func (h TxHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *TxHeap) Push(x interface{}) {
	// Push and Pop use pointer receivers because they modify the slice's length,
	// not just its contents.
	*h = append(*h, x.(*TxCounterTransaction))
}

func (h *TxHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]     // get the last element of our slice (top element in heap)
	*h = old[0 : n-1] // update the heap pointer to a new slice without the last element
	return x
}
