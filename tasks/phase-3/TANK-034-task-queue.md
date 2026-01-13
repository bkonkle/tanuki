---
id: TANK-034
title: Task Queue
status: todo
priority: high
estimate: M
depends_on: [TANK-030]
workstream: B
phase: 3
---

# Task Queue

## Summary

Implement a priority-based task queue that supports role-aware dequeuing. The queue maintains pending tasks sorted by priority and allows agents to request tasks matching their role.

## Acceptance Criteria

- [ ] Priority queue with critical > high > medium > low ordering
- [ ] Role-aware dequeue (get next task for specific role)
- [ ] Peek without removing
- [ ] Size queries (total and by role)
- [ ] Thread-safe operations
- [ ] Re-enqueue support (for blocked tasks)
- [ ] Unit tests with 80%+ coverage

## Technical Details

### Queue Interface

```go
// internal/task/queue.go
package task

import (
    "container/heap"
    "fmt"
    "sync"
)

// Queue is a priority queue for tasks, organized by role
type Queue struct {
    mu     sync.RWMutex
    queues map[string]*priorityQueue // role -> queue
}

// NewQueue creates a new task queue
func NewQueue() *Queue {
    return &Queue{
        queues: make(map[string]*priorityQueue),
    }
}

// Enqueue adds a task to the queue
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

// Dequeue removes and returns the highest priority task for a role
func (q *Queue) Dequeue(role string) (*Task, error) {
    q.mu.Lock()
    defer q.mu.Unlock()

    pq, ok := q.queues[role]
    if !ok || pq.Len() == 0 {
        return nil, fmt.Errorf("no tasks for role %q", role)
    }

    item := heap.Pop(pq).(*queueItem)
    return item.task, nil
}

// Peek returns the highest priority task without removing it
func (q *Queue) Peek(role string) (*Task, error) {
    q.mu.RLock()
    defer q.mu.RUnlock()

    pq, ok := q.queues[role]
    if !ok || pq.Len() == 0 {
        return nil, fmt.Errorf("no tasks for role %q", role)
    }

    return (*pq)[0].task, nil
}

// Size returns total number of tasks in queue
func (q *Queue) Size() int {
    q.mu.RLock()
    defer q.mu.RUnlock()

    total := 0
    for _, pq := range q.queues {
        total += pq.Len()
    }
    return total
}

// SizeByRole returns number of tasks for a specific role
func (q *Queue) SizeByRole(role string) int {
    q.mu.RLock()
    defer q.mu.RUnlock()

    pq, ok := q.queues[role]
    if !ok {
        return 0
    }
    return pq.Len()
}

// Roles returns all roles with pending tasks
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

// Clear empties the queue
func (q *Queue) Clear() {
    q.mu.Lock()
    defer q.mu.Unlock()

    q.queues = make(map[string]*priorityQueue)
}
```

### Priority Queue Implementation

```go
// internal/task/priority_queue.go
package task

// queueItem wraps a task with its priority for heap operations
type queueItem struct {
    task     *Task
    priority int // Lower = higher priority (0 = critical)
    index    int // Index in heap, maintained by heap.Interface
}

// priorityQueue implements heap.Interface
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
    item := x.(*queueItem)
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
    switch p {
    case PriorityCritical:
        return 0
    case PriorityHigh:
        return 1
    case PriorityMedium:
        return 2
    case PriorityLow:
        return 3
    default:
        return 2 // Default to medium
    }
}
```

### Batch Operations

```go
// EnqueueAll adds multiple tasks to the queue
func (q *Queue) EnqueueAll(tasks []*Task) error {
    for _, t := range tasks {
        if err := q.Enqueue(t); err != nil {
            return fmt.Errorf("enqueue %s: %w", t.ID, err)
        }
    }
    return nil
}

// DequeueAll removes and returns all tasks for a role
func (q *Queue) DequeueAll(role string) []*Task {
    q.mu.Lock()
    defer q.mu.Unlock()

    pq, ok := q.queues[role]
    if !ok {
        return nil
    }

    tasks := make([]*Task, 0, pq.Len())
    for pq.Len() > 0 {
        item := heap.Pop(pq).(*queueItem)
        tasks = append(tasks, item.task)
    }

    return tasks
}

// Remove removes a specific task from the queue by ID
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

// Contains checks if a task is in the queue
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
```

### Statistics

```go
// Stats returns queue statistics
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

type QueueStats struct {
    Total      int
    ByRole     map[string]int
    ByPriority map[Priority]int
}
```

## Testing

### Unit Tests

```go
func TestQueue_Enqueue(t *testing.T) {
    q := NewQueue()

    task := &Task{ID: "T1", Role: "backend", Priority: PriorityHigh}
    err := q.Enqueue(task)

    if err != nil {
        t.Fatalf("Enqueue() error: %v", err)
    }

    if q.Size() != 1 {
        t.Errorf("Size() = %d, want 1", q.Size())
    }

    if q.SizeByRole("backend") != 1 {
        t.Errorf("SizeByRole(backend) = %d, want 1", q.SizeByRole("backend"))
    }
}

func TestQueue_PriorityOrder(t *testing.T) {
    q := NewQueue()

    // Add in random priority order
    q.Enqueue(&Task{ID: "low", Role: "backend", Priority: PriorityLow})
    q.Enqueue(&Task{ID: "critical", Role: "backend", Priority: PriorityCritical})
    q.Enqueue(&Task{ID: "medium", Role: "backend", Priority: PriorityMedium})
    q.Enqueue(&Task{ID: "high", Role: "backend", Priority: PriorityHigh})

    // Should come out in priority order
    expected := []string{"critical", "high", "medium", "low"}
    for _, expectedID := range expected {
        task, err := q.Dequeue("backend")
        if err != nil {
            t.Fatalf("Dequeue() error: %v", err)
        }
        if task.ID != expectedID {
            t.Errorf("Dequeue() = %s, want %s", task.ID, expectedID)
        }
    }
}

func TestQueue_RoleIsolation(t *testing.T) {
    q := NewQueue()

    q.Enqueue(&Task{ID: "be1", Role: "backend", Priority: PriorityHigh})
    q.Enqueue(&Task{ID: "fe1", Role: "frontend", Priority: PriorityHigh})
    q.Enqueue(&Task{ID: "be2", Role: "backend", Priority: PriorityMedium})

    // Backend should only see backend tasks
    if q.SizeByRole("backend") != 2 {
        t.Errorf("SizeByRole(backend) = %d, want 2", q.SizeByRole("backend"))
    }

    task, _ := q.Dequeue("backend")
    if task.ID != "be1" {
        t.Errorf("Dequeue(backend) = %s, want be1", task.ID)
    }

    // Frontend unaffected
    if q.SizeByRole("frontend") != 1 {
        t.Errorf("SizeByRole(frontend) = %d, want 1", q.SizeByRole("frontend"))
    }
}

func TestQueue_Peek(t *testing.T) {
    q := NewQueue()

    q.Enqueue(&Task{ID: "T1", Role: "backend", Priority: PriorityHigh})

    // Peek should not remove
    task1, _ := q.Peek("backend")
    task2, _ := q.Peek("backend")

    if task1.ID != task2.ID {
        t.Error("Peek() modified queue")
    }

    if q.Size() != 1 {
        t.Error("Peek() removed item from queue")
    }
}

func TestQueue_EmptyRole(t *testing.T) {
    q := NewQueue()

    _, err := q.Dequeue("nonexistent")
    if err == nil {
        t.Error("Dequeue() should error for empty role")
    }
}

func TestQueue_ConcurrentAccess(t *testing.T) {
    q := NewQueue()

    // Pre-populate
    for i := 0; i < 100; i++ {
        q.Enqueue(&Task{
            ID:       fmt.Sprintf("T%d", i),
            Role:     "backend",
            Priority: PriorityMedium,
        })
    }

    // Concurrent reads and writes
    var wg sync.WaitGroup
    for i := 0; i < 10; i++ {
        wg.Add(2)

        // Reader
        go func() {
            defer wg.Done()
            for j := 0; j < 100; j++ {
                q.Size()
                q.SizeByRole("backend")
                q.Peek("backend")
            }
        }()

        // Writer
        go func(id int) {
            defer wg.Done()
            for j := 0; j < 10; j++ {
                q.Enqueue(&Task{
                    ID:       fmt.Sprintf("new-%d-%d", id, j),
                    Role:     "backend",
                    Priority: PriorityHigh,
                })
                q.Dequeue("backend")
            }
        }(i)
    }

    wg.Wait()
}

func TestQueue_Remove(t *testing.T) {
    q := NewQueue()

    q.Enqueue(&Task{ID: "T1", Role: "backend", Priority: PriorityHigh})
    q.Enqueue(&Task{ID: "T2", Role: "backend", Priority: PriorityMedium})
    q.Enqueue(&Task{ID: "T3", Role: "backend", Priority: PriorityLow})

    // Remove middle priority
    removed := q.Remove("T2")
    if !removed {
        t.Error("Remove() should return true")
    }

    if q.Size() != 2 {
        t.Errorf("Size() = %d, want 2", q.Size())
    }

    if q.Contains("T2") {
        t.Error("Contains(T2) should be false after remove")
    }
}
```

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Nil task | Return error |
| Task without role | Return error |
| Dequeue empty role | Return error with role name |
| Peek empty role | Return error with role name |

## Out of Scope

- Persistent queue (survives restart)
- Queue size limits
- Task expiration/TTL
- Priority boosting over time

## Notes

The queue is purely in-memory and rebuilt from task files on startup. This keeps it simple and avoids synchronization issues with the filesystem.

The role-based organization allows efficient dequeue when an agent asks "what's next for me?" without scanning all tasks.
