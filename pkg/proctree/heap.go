package proctree

import (
	"container/heap"
	"sync"
	"time"
)

// heapItem represents an item in the min-heap
type heapItem struct {
	pid       uint32
	timestamp time.Time
	index     int // index in heap
}

// minHeap implements heap.Interface for time-based ordering
type minHeap []*heapItem

func (h minHeap) Len() int           { return len(h) }
func (h minHeap) Less(i, j int) bool { return h[i].timestamp.Before(h[j].timestamp) }
func (h minHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *minHeap) Push(x interface{}) {
	n := len(*h)
	item := x.(*heapItem)
	item.index = n
	*h = append(*h, item)
}

func (h *minHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // avoid memory leak
	item.index = -1 // for safety
	*h = old[0 : n-1]
	return item
}

// timeIndex manages time-ordered access to process entries
type timeIndex struct {
	mu        sync.Mutex
	heap      minHeap
	pidToItem map[uint32]*heapItem
}

func newTimeIndex() *timeIndex {
	return &timeIndex{
		heap:      make(minHeap, 0),
		pidToItem: make(map[uint32]*heapItem),
	}
	// No need to call heap.Init on empty heap
}

func (ti *timeIndex) Add(pid uint32, timestamp time.Time) {
	ti.mu.Lock()
	defer ti.mu.Unlock()

	// If already exists, remove old entry
	if oldItem, exists := ti.pidToItem[pid]; exists {
		heap.Remove(&ti.heap, oldItem.index)
	}

	// Add new entry
	item := &heapItem{
		pid:       pid,
		timestamp: timestamp,
	}
	heap.Push(&ti.heap, item)
	ti.pidToItem[pid] = item
}

func (ti *timeIndex) Remove(pid uint32) {
	ti.mu.Lock()
	defer ti.mu.Unlock()

	if item, exists := ti.pidToItem[pid]; exists {
		heap.Remove(&ti.heap, item.index)
		delete(ti.pidToItem, pid)
	}
}

// PopOldest returns and removes the oldest PID
func (ti *timeIndex) PopOldest() (uint32, bool) {
	ti.mu.Lock()
	defer ti.mu.Unlock()

	if ti.heap.Len() == 0 {
		return 0, false
	}

	item := heap.Pop(&ti.heap).(*heapItem)
	delete(ti.pidToItem, item.pid)
	return item.pid, true
}

// GetOldest returns the oldest PID without removing it
func (ti *timeIndex) GetOldest() (uint32, time.Time, bool) {
	ti.mu.Lock()
	defer ti.mu.Unlock()

	if ti.heap.Len() == 0 {
		return 0, time.Time{}, false
	}

	item := ti.heap[0]
	return item.pid, item.timestamp, true
}

func (ti *timeIndex) Len() int {
	ti.mu.Lock()
	defer ti.mu.Unlock()
	return ti.heap.Len()
}
