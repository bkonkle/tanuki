package task

import (
	"container/heap"
	"fmt"
	"sync"
)

// Queue is a priority queue for tasks, organized by role.
// It supports role-aware dequeuing, allowing agents to request tasks
// matching their specific role while maintaining priority ordering.
type Queue struct {
	mu     sync.RWMutex
	queues map[string]*priorityQueue // role -> queue
}

// NewQueue creates a new task queue.
func NewQueue() *Queue {
	return &Queue{
		queues: make(map[string]*priorityQueue),
	}
}

// Enqueue adds a task to the queue.
// Returns an error if the task is nil or has no role assigned.
func (q *Queue) Enqueue(t *Task) error {
	if t == nil {
		return fmt.Errorf("task is nil")
	}
	if t.Role == "" {
		return fmt.Errorf("task has no role")
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	pq, ok := q.queues[t.Role]
	if !ok {
		pq = &priorityQueue{}
		heap.Init(pq)
		q.queues[t.Role] = pq
	}

	heap.Push(pq, &queueItem{task: t, priority: priorityValue(t.Priority)})
	return nil
}

// Dequeue removes and returns the highest priority task for a role.
// Returns an error if there are no tasks for the specified role.
func (q *Queue) Dequeue(role string) (*Task, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	pq, ok := q.queues[role]
	if !ok || pq.Len() == 0 {
		return nil, fmt.Errorf("no tasks for role %q", role)
	}

	item, _ := heap.Pop(pq).(*queueItem)
	return item.task, nil
}

// Peek returns the highest priority task without removing it.
// Returns an error if there are no tasks for the specified role.
func (q *Queue) Peek(role string) (*Task, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	pq, ok := q.queues[role]
	if !ok || pq.Len() == 0 {
		return nil, fmt.Errorf("no tasks for role %q", role)
	}

	return (*pq)[0].task, nil
}

// Size returns total number of tasks in queue across all roles.
func (q *Queue) Size() int {
	q.mu.RLock()
	defer q.mu.RUnlock()

	total := 0
	for _, pq := range q.queues {
		total += pq.Len()
	}
	return total
}

// SizeByRole returns number of tasks for a specific role.
func (q *Queue) SizeByRole(role string) int {
	q.mu.RLock()
	defer q.mu.RUnlock()

	pq, ok := q.queues[role]
	if !ok {
		return 0
	}
	return pq.Len()
}

// Roles returns all roles with pending tasks.
func (q *Queue) Roles() []string {
	q.mu.RLock()
	defer q.mu.RUnlock()

	roles := make([]string, 0, len(q.queues))
	for role, pq := range q.queues {
		if pq.Len() > 0 {
			roles = append(roles, role)
		}
	}
	return roles
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

// DequeueAll removes and returns all tasks for a role.
// Returns nil if there are no tasks for the role.
func (q *Queue) DequeueAll(role string) []*Task {
	q.mu.Lock()
	defer q.mu.Unlock()

	pq, ok := q.queues[role]
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
		ByRole:     make(map[string]int),
		ByPriority: make(map[Priority]int),
	}

	for role, pq := range q.queues {
		stats.ByRole[role] = pq.Len()
		stats.Total += pq.Len()

		for _, item := range *pq {
			stats.ByPriority[item.task.Priority]++
		}
	}

	return stats
}

// QueueStats contains statistics about the queue.
type QueueStats struct {
	Total      int
	ByRole     map[string]int
	ByPriority map[Priority]int
}
