package task

import (
	"container/heap"
	"testing"
)

func TestPriorityQueue_Basic(t *testing.T) {
	pq := &priorityQueue{}
	heap.Init(pq)

	// Push items
	heap.Push(pq, &queueItem{task: &Task{ID: "T1"}, priority: 2})
	heap.Push(pq, &queueItem{task: &Task{ID: "T2"}, priority: 0})
	heap.Push(pq, &queueItem{task: &Task{ID: "T3"}, priority: 1})

	if pq.Len() != 3 {
		t.Errorf("Len() = %d, want 3", pq.Len())
	}

	// Pop in priority order
	item1 := heap.Pop(pq).(*queueItem)
	if item1.task.ID != "T2" {
		t.Errorf("First pop = %s, want T2 (priority 0)", item1.task.ID)
	}

	item2 := heap.Pop(pq).(*queueItem)
	if item2.task.ID != "T3" {
		t.Errorf("Second pop = %s, want T3 (priority 1)", item2.task.ID)
	}

	item3 := heap.Pop(pq).(*queueItem)
	if item3.task.ID != "T1" {
		t.Errorf("Third pop = %s, want T1 (priority 2)", item3.task.ID)
	}
}

func TestPriorityQueue_Swap(t *testing.T) {
	pq := &priorityQueue{
		{task: &Task{ID: "T1"}, priority: 0, index: 0},
		{task: &Task{ID: "T2"}, priority: 1, index: 1},
	}

	pq.Swap(0, 1)

	if (*pq)[0].task.ID != "T2" {
		t.Errorf("After swap, [0] = %s, want T2", (*pq)[0].task.ID)
	}
	if (*pq)[1].task.ID != "T1" {
		t.Errorf("After swap, [1] = %s, want T1", (*pq)[1].task.ID)
	}
	if (*pq)[0].index != 0 {
		t.Errorf("After swap, [0].index = %d, want 0", (*pq)[0].index)
	}
	if (*pq)[1].index != 1 {
		t.Errorf("After swap, [1].index = %d, want 1", (*pq)[1].index)
	}
}

func TestPriorityQueue_Less(t *testing.T) {
	pq := &priorityQueue{
		{task: &Task{ID: "T1"}, priority: 0, index: 0},
		{task: &Task{ID: "T2"}, priority: 1, index: 1},
	}

	if !pq.Less(0, 1) {
		t.Error("Less(0, 1) should be true (0 < 1)")
	}
	if pq.Less(1, 0) {
		t.Error("Less(1, 0) should be false (1 > 0)")
	}
}

func TestPriorityQueue_PushPop(t *testing.T) {
	pq := &priorityQueue{}

	// Test Push
	item := &queueItem{task: &Task{ID: "T1"}, priority: 1}
	pq.Push(item)

	if pq.Len() != 1 {
		t.Errorf("After Push, Len() = %d, want 1", pq.Len())
	}
	if item.index != 0 {
		t.Errorf("After Push, item.index = %d, want 0", item.index)
	}

	// Test Pop
	popped := pq.Pop().(*queueItem)
	if popped.task.ID != "T1" {
		t.Errorf("Pop() = %s, want T1", popped.task.ID)
	}
	if popped.index != -1 {
		t.Errorf("After Pop, item.index = %d, want -1", popped.index)
	}
	if pq.Len() != 0 {
		t.Errorf("After Pop, Len() = %d, want 0", pq.Len())
	}
}

func TestPriorityValue(t *testing.T) {
	tests := []struct {
		priority Priority
		expected int
	}{
		{PriorityCritical, 0},
		{PriorityHigh, 1},
		{PriorityMedium, 2},
		{PriorityLow, 3},
		{"", 2},       // Empty defaults to medium
		{"invalid", 2}, // Invalid defaults to medium
	}

	for _, tt := range tests {
		t.Run(string(tt.priority), func(t *testing.T) {
			got := priorityValue(tt.priority)
			if got != tt.expected {
				t.Errorf("priorityValue(%q) = %d, want %d", tt.priority, got, tt.expected)
			}
		})
	}
}

func TestPriorityQueue_HeapOperations(t *testing.T) {
	pq := &priorityQueue{}
	heap.Init(pq)

	// Add items in wrong order
	items := []*queueItem{
		{task: &Task{ID: "low"}, priority: 3},
		{task: &Task{ID: "critical"}, priority: 0},
		{task: &Task{ID: "medium"}, priority: 2},
		{task: &Task{ID: "high"}, priority: 1},
	}

	for _, item := range items {
		heap.Push(pq, item)
	}

	// Verify heap property - min should be at top
	if (*pq)[0].priority != 0 {
		t.Errorf("Heap top priority = %d, want 0", (*pq)[0].priority)
	}

	// Remove using heap.Remove
	heap.Remove(pq, 2) // Remove some middle element

	if pq.Len() != 3 {
		t.Errorf("After Remove, Len() = %d, want 3", pq.Len())
	}

	// Should still maintain heap property
	prev := -1
	for pq.Len() > 0 {
		item := heap.Pop(pq).(*queueItem)
		if item.priority < prev {
			t.Errorf("Heap property violated: %d after %d", item.priority, prev)
		}
		prev = item.priority
	}
}
