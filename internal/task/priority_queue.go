package task

// queueItem wraps a task with its priority for heap operations
type queueItem struct {
	task     *Task
	priority int // Lower = higher priority (0 = critical)
	index    int // Index in heap, maintained by heap.Interface
}

// priorityQueue implements heap.Interface for priority-based task ordering
type priorityQueue []*queueItem

func (pq priorityQueue) Len() int { return len(pq) }

func (pq priorityQueue) Less(i, j int) bool {
	// Lower priority value = higher priority (pop first)
	return pq[i].priority < pq[j].priority
}

func (pq priorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *priorityQueue) Push(x interface{}) {
	n := len(*pq)
	item, _ := x.(*queueItem)
	item.index = n
	*pq = append(*pq, item)
}

func (pq *priorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // Avoid memory leak
	item.index = -1 // For safety
	*pq = old[0 : n-1]
	return item
}

// priorityValue converts Priority to numeric value for heap ordering
func priorityValue(p Priority) int {
	return p.Order()
}
