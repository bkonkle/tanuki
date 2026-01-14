package task

import (
	"fmt"
	"sync"
	"testing"
)

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

func TestQueue_EnqueueNilTask(t *testing.T) {
	q := NewQueue()

	err := q.Enqueue(nil)
	if err == nil {
		t.Error("Enqueue(nil) should return error")
	}
}

func TestQueue_EnqueueNoRole(t *testing.T) {
	q := NewQueue()

	task := &Task{ID: "T1", Priority: PriorityHigh}
	err := q.Enqueue(task)
	if err == nil {
		t.Error("Enqueue() should return error for task with no role")
	}
}

func TestQueue_PriorityOrder(t *testing.T) {
	q := NewQueue()

	// Add in random priority order
	_ = q.Enqueue(&Task{ID: "low", Role: "backend", Priority: PriorityLow})
	_ = q.Enqueue(&Task{ID: "critical", Role: "backend", Priority: PriorityCritical})
	_ = q.Enqueue(&Task{ID: "medium", Role: "backend", Priority: PriorityMedium})
	_ = q.Enqueue(&Task{ID: "high", Role: "backend", Priority: PriorityHigh})

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

	_ = q.Enqueue(&Task{ID: "be1", Role: "backend", Priority: PriorityHigh})
	_ = q.Enqueue(&Task{ID: "fe1", Role: "frontend", Priority: PriorityHigh})
	_ = q.Enqueue(&Task{ID: "be2", Role: "backend", Priority: PriorityMedium})

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

	_ = q.Enqueue(&Task{ID: "T1", Role: "backend", Priority: PriorityHigh})

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

func TestQueue_PeekEmptyRole(t *testing.T) {
	q := NewQueue()

	_, err := q.Peek("nonexistent")
	if err == nil {
		t.Error("Peek() should error for empty role")
	}
}

func TestQueue_ConcurrentAccess(_ *testing.T) {
	q := NewQueue()

	// Pre-populate
	for i := 0; i < 100; i++ {
		_ = q.Enqueue(&Task{
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
				_, _ = q.Peek("backend")
				q.Contains("T50")
			}
		}()

		// Writer
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				_ = q.Enqueue(&Task{
					ID:       fmt.Sprintf("new-%d-%d", id, j),
					Role:     "backend",
					Priority: PriorityHigh,
				})
				_, _ = q.Dequeue("backend")
			}
		}(i)
	}

	wg.Wait()
}

func TestQueue_Remove(t *testing.T) {
	q := NewQueue()

	_ = q.Enqueue(&Task{ID: "T1", Role: "backend", Priority: PriorityHigh})
	_ = q.Enqueue(&Task{ID: "T2", Role: "backend", Priority: PriorityMedium})
	_ = q.Enqueue(&Task{ID: "T3", Role: "backend", Priority: PriorityLow})

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

func TestQueue_RemoveNonexistent(t *testing.T) {
	q := NewQueue()

	_ = q.Enqueue(&Task{ID: "T1", Role: "backend", Priority: PriorityHigh})

	removed := q.Remove("nonexistent")
	if removed {
		t.Error("Remove() should return false for nonexistent task")
	}
}

func TestQueue_Contains(t *testing.T) {
	q := NewQueue()

	_ = q.Enqueue(&Task{ID: "T1", Role: "backend", Priority: PriorityHigh})

	if !q.Contains("T1") {
		t.Error("Contains(T1) should be true")
	}

	if q.Contains("T2") {
		t.Error("Contains(T2) should be false")
	}
}

func TestQueue_Roles(t *testing.T) {
	q := NewQueue()

	_ = q.Enqueue(&Task{ID: "T1", Role: "backend", Priority: PriorityHigh})
	_ = q.Enqueue(&Task{ID: "T2", Role: "frontend", Priority: PriorityHigh})
	_ = q.Enqueue(&Task{ID: "T3", Role: "qa", Priority: PriorityHigh})

	roles := q.Roles()
	if len(roles) != 3 {
		t.Errorf("Roles() = %v, want 3 roles", roles)
	}

	// Verify all roles are present
	roleMap := make(map[string]bool)
	for _, r := range roles {
		roleMap[r] = true
	}

	for _, expected := range []string{"backend", "frontend", "qa"} {
		if !roleMap[expected] {
			t.Errorf("Roles() missing %s", expected)
		}
	}
}

func TestQueue_RolesEmpty(t *testing.T) {
	q := NewQueue()

	_ = q.Enqueue(&Task{ID: "T1", Role: "backend", Priority: PriorityHigh})
	_, _ = q.Dequeue("backend")

	roles := q.Roles()
	if len(roles) != 0 {
		t.Errorf("Roles() = %v, want empty", roles)
	}
}

func TestQueue_Clear(t *testing.T) {
	q := NewQueue()

	_ = q.Enqueue(&Task{ID: "T1", Role: "backend", Priority: PriorityHigh})
	_ = q.Enqueue(&Task{ID: "T2", Role: "frontend", Priority: PriorityHigh})

	q.Clear()

	if q.Size() != 0 {
		t.Errorf("Size() = %d, want 0 after Clear()", q.Size())
	}
}

func TestQueue_EnqueueAll(t *testing.T) {
	q := NewQueue()

	tasks := []*Task{
		{ID: "T1", Role: "backend", Priority: PriorityHigh},
		{ID: "T2", Role: "frontend", Priority: PriorityMedium},
		{ID: "T3", Role: "backend", Priority: PriorityLow},
	}

	err := q.EnqueueAll(tasks)
	if err != nil {
		t.Fatalf("EnqueueAll() error: %v", err)
	}

	if q.Size() != 3 {
		t.Errorf("Size() = %d, want 3", q.Size())
	}
}

func TestQueue_EnqueueAllWithError(t *testing.T) {
	q := NewQueue()

	tasks := []*Task{
		{ID: "T1", Role: "backend", Priority: PriorityHigh},
		{ID: "T2", Role: "", Priority: PriorityMedium}, // No role - should fail
	}

	err := q.EnqueueAll(tasks)
	if err == nil {
		t.Error("EnqueueAll() should return error for task with no role")
	}
}

func TestQueue_DequeueAll(t *testing.T) {
	q := NewQueue()

	_ = q.Enqueue(&Task{ID: "T1", Role: "backend", Priority: PriorityLow})
	_ = q.Enqueue(&Task{ID: "T2", Role: "backend", Priority: PriorityCritical})
	_ = q.Enqueue(&Task{ID: "T3", Role: "backend", Priority: PriorityMedium})

	tasks := q.DequeueAll("backend")

	// Should be in priority order
	if len(tasks) != 3 {
		t.Fatalf("DequeueAll() returned %d tasks, want 3", len(tasks))
	}

	expected := []string{"T2", "T3", "T1"} // critical, medium, low
	for i, task := range tasks {
		if task.ID != expected[i] {
			t.Errorf("DequeueAll()[%d] = %s, want %s", i, task.ID, expected[i])
		}
	}

	// Queue should be empty
	if q.SizeByRole("backend") != 0 {
		t.Error("Queue should be empty after DequeueAll")
	}
}

func TestQueue_DequeueAllEmpty(t *testing.T) {
	q := NewQueue()

	tasks := q.DequeueAll("nonexistent")
	if tasks != nil {
		t.Error("DequeueAll() should return nil for nonexistent role")
	}
}

func TestQueue_Stats(t *testing.T) {
	q := NewQueue()

	_ = q.Enqueue(&Task{ID: "T1", Role: "backend", Priority: PriorityHigh})
	_ = q.Enqueue(&Task{ID: "T2", Role: "backend", Priority: PriorityMedium})
	_ = q.Enqueue(&Task{ID: "T3", Role: "frontend", Priority: PriorityCritical})

	stats := q.Stats()

	if stats.Total != 3 {
		t.Errorf("Stats.Total = %d, want 3", stats.Total)
	}

	if stats.ByRole["backend"] != 2 {
		t.Errorf("Stats.ByRole[backend] = %d, want 2", stats.ByRole["backend"])
	}

	if stats.ByRole["frontend"] != 1 {
		t.Errorf("Stats.ByRole[frontend] = %d, want 1", stats.ByRole["frontend"])
	}

	if stats.ByPriority[PriorityHigh] != 1 {
		t.Errorf("Stats.ByPriority[high] = %d, want 1", stats.ByPriority[PriorityHigh])
	}

	if stats.ByPriority[PriorityMedium] != 1 {
		t.Errorf("Stats.ByPriority[medium] = %d, want 1", stats.ByPriority[PriorityMedium])
	}

	if stats.ByPriority[PriorityCritical] != 1 {
		t.Errorf("Stats.ByPriority[critical] = %d, want 1", stats.ByPriority[PriorityCritical])
	}
}

func TestQueue_DefaultPriority(t *testing.T) {
	q := NewQueue()

	// Task with empty priority should default to medium behavior
	_ = q.Enqueue(&Task{ID: "T1", Role: "backend", Priority: ""})
	_ = q.Enqueue(&Task{ID: "T2", Role: "backend", Priority: PriorityHigh})
	_ = q.Enqueue(&Task{ID: "T3", Role: "backend", Priority: PriorityLow})

	// High should come first, then empty (treated as medium), then low
	task1, _ := q.Dequeue("backend")
	task2, _ := q.Dequeue("backend")
	task3, _ := q.Dequeue("backend")

	if task1.ID != "T2" {
		t.Errorf("First task should be T2 (high), got %s", task1.ID)
	}
	if task2.ID != "T1" {
		t.Errorf("Second task should be T1 (empty/medium), got %s", task2.ID)
	}
	if task3.ID != "T3" {
		t.Errorf("Third task should be T3 (low), got %s", task3.ID)
	}
}

func TestQueue_SamePriorityFIFO(t *testing.T) {
	q := NewQueue()

	// Tasks with same priority should maintain order relative to heap behavior
	// Note: heap doesn't guarantee FIFO for same priority, but should be stable
	_ = q.Enqueue(&Task{ID: "T1", Role: "backend", Priority: PriorityHigh})
	_ = q.Enqueue(&Task{ID: "T2", Role: "backend", Priority: PriorityHigh})
	_ = q.Enqueue(&Task{ID: "T3", Role: "backend", Priority: PriorityHigh})

	// All should come out, we don't guarantee FIFO but should get all 3
	ids := make(map[string]bool)
	for i := 0; i < 3; i++ {
		task, err := q.Dequeue("backend")
		if err != nil {
			t.Fatalf("Dequeue() error: %v", err)
		}
		ids[task.ID] = true
	}

	if len(ids) != 3 {
		t.Errorf("Should have dequeued 3 unique tasks, got %d", len(ids))
	}
}
