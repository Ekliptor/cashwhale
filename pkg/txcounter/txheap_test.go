package txcounter

import (
	"container/heap"
	"testing"
	"time"
)

func TestTransactionsHeap(t *testing.T) {
	h := &TxHeap{
		&TxCounterTransaction{
			SizeBch: 123.4,
			When:    time.Now(),
		},
		&TxCounterTransaction{
			SizeBch: 0.4,
			When:    time.Now(),
		},
		&TxCounterTransaction{
			SizeBch: 444.4,
			When:    time.Now(),
		},
	}
	heap.Init(h)
	heap.Push(h, &TxCounterTransaction{
		SizeBch: 5555.4,
		When:    time.Now(),
	})
	//t.Logf("minimum: %v", (*h)[0])
	t.Logf("maximum: %v", (*h)[0])
	for h.Len() > 0 {
		t.Logf("popped: %v ", heap.Pop(h))
	}
}
