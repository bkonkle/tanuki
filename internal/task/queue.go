package task

import (
	"container/heap"
	"fmt"
	"sync"
)

// Queue is a priority queue for tasks, organized by workstream.
// It supports workstream-aware dequeuing, allowing agents to request tasks
// matching their specific workstream while maintaining priority ordering.
type Queue struct {
	mu     sync.RWMutex
	queues map[string]*priorityQueue // workstream -> queue
}

// NewQueue creates a new task queue.
func NewQueue() *Queue {
	return &Queue{
		queues: make(map[string]*priorityQueue),
	}
}

// Enqueue adds a task to the queue.
// Returns an error if the task is nil.
func (q *Queue) Enqueue(t *Task) error {
	if t == nil {
		return fmt.Errorf("task is nil")
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	ws := t.GetWorkstream()
	pq, ok := q.queues[ws]
	if !ok {
		pq = &priorityQueue{}
		heap.Init(pq)
		q.queues[ws] = pq
	}

	heap.Push(pq, &queueItem{task: t, priority: priorityValue(t.Priority)})
	return nil
}

// Dequeue removes and returns the highest priority task for a workstream.
// Returns an error if there are no tasks for the specified workstream.
func (q *Queue) Dequeue(workstream string) (*Task, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	pq, ok := q.queues[workstream]
	if !ok || pq.Len() == 0 {
		return nil, fmt.Errorf("no tasks for workstream %q", workstream)
	}

	item, _ := heap.Pop(pq).(*queueItem)
	return item.task, nil
}

// Peek returns the highest priority task without removing it.
// Returns an error if there are no tasks for the specified workstream.
func (q *Queue) Peek(workstream string) (*Task, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	pq, ok := q.queues[workstream]
	if !ok || pq.Len() == 0 {
		return nil, fmt.Errorf("no tasks for workstream %q", workstream)
	}

	return (*pq)[0].task, nil
}

// Size returns total number of tasks in queue across all workstreams.
func (q *Queue) Size() int {
	q.mu.RLock()
	defer q.mu.RUnlock()

	total := 0
	for _, pq := range q.queues {
		total += pq.Len()
	}
	return total
}

// SizeByWorkstream returns number of tasks for a specific workstream.
func (q *Queue) SizeByWorkstream(workstream string) int {
	q.mu.RLock()
	defer q.mu.RUnlock()

	pq, ok := q.queues[workstream]
	if !ok {
		return 0
	}
	return pq.Len()
}

// Workstreams returns all workstreams with pending tasks.
func (q *Queue) Workstreams() []string {
	q.mu.RLock()
	defer q.mu.RUnlock()

	workstreams := make([]string, 0, len(q.queues))
	for ws, pq := range q.queues {
		if pq.Len() > 0 {
			workstreams = append(workstreams, ws)
		}
	}
	return workstreams
}

// Clear empties the queue.
func (q *Queue) Clear() {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.queues = make(map[string]*priorityQueue)
}

// EnqueueAll adds multiple tasks to the queue.
// Returns an error if any task fails to enqueue.
func (q *Queue) EnqueueAll(tasks []*Task) error {
	for _, t := range tasks {
		if err := q.Enqueue(t); err != nil {
			return fmt.Errorf("enqueue %s: %w", t.ID, err)
		}
	}
	return nil
}

// DequeueAll removes and returns all tasks for a workstream.
// Returns nil if there are no tasks for the workstream.
func (q *Queue) DequeueAll(workstream string) []*Task {
	q.mu.Lock()
	defer q.mu.Unlock()

	pq, ok := q.queues[workstream]
	if !ok {
		return nil
	}

	tasks := make([]*Task, 0, pq.Len())
	for pq.Len() > 0 {
		item, _ := heap.Pop(pq).(*queueItem)
		tasks = append(tasks, item.task)
	}

	return tasks
}

// Remove removes a specific task from the queue by ID.
// Returns true if the task was found and removed.
func (q *Queue) Remove(taskID string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	for _, pq := range q.queues {
		for i, item := range *pq {
			if item.task.ID == taskID {
				heap.Remove(pq, i)
				return true
			}
		}
	}
	return false
}

// Contains checks if a task is in the queue.
func (q *Queue) Contains(taskID string) bool {
	q.mu.RLock()
	defer q.mu.RUnlock()

	for _, pq := range q.queues {
		for _, item := range *pq {
			if item.task.ID == taskID {
				return true
			}
		}
	}
	return false
}

// Stats returns queue statistics.
func (q *Queue) Stats() *QueueStats {
	q.mu.RLock()
	defer q.mu.RUnlock()

	stats := &QueueStats{
		ByWorkstream: make(map[string]int),
		ByPriority:   make(map[Priority]int),
	}

	for ws, pq := range q.queues {
		stats.ByWorkstream[ws] = pq.Len()
		stats.Total += pq.Len()

		for _, item := range *pq {
			stats.ByPriority[item.task.Priority]++
		}
	}

	return stats
}

// QueueStats contains statistics about the queue.
type QueueStats struct {
	Total        int
	ByWorkstream map[string]int
	ByPriority   map[Priority]int
}
