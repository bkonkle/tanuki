package task

import (
	"strings"
	"testing"
)

func TestNewResolver(t *testing.T) {
	tasks := []*Task{
		{ID: "T1", Title: "Task 1"},
		{ID: "T2", Title: "Task 2"},
	}

	resolver := NewResolver(tasks)

	if resolver == nil {
		t.Fatal("NewResolver() returned nil")
	}
	if len(resolver.tasks) != 2 {
		t.Errorf("tasks count = %d, want 2", len(resolver.tasks))
	}
	if !resolver.HasTask("T1") {
		t.Error("resolver should have task T1")
	}
}

func TestResolver_GetReady(t *testing.T) {
	tasks := []*Task{
		{ID: "T1", Status: StatusPending, DependsOn: nil},
		{ID: "T2", Status: StatusPending, DependsOn: []string{"T1"}},
		{ID: "T3", Status: StatusPending, DependsOn: []string{"T1", "T2"}},
	}

	resolver := NewResolver(tasks)
	ready := resolver.GetReady()

	// Only T1 should be ready (no deps)
	if len(ready) != 1 {
		t.Errorf("GetReady() returned %d tasks, want 1", len(ready))
	}
	if ready[0].ID != "T1" {
		t.Errorf("GetReady() = [%s], want [T1]", ready[0].ID)
	}
}

func TestResolver_GetReadyAfterComplete(t *testing.T) {
	tasks := []*Task{
		{ID: "T1", Status: StatusComplete, DependsOn: nil},
		{ID: "T2", Status: StatusPending, DependsOn: []string{"T1"}},
		{ID: "T3", Status: StatusPending, DependsOn: []string{"T1", "T2"}},
	}

	resolver := NewResolver(tasks)
	ready := resolver.GetReady()

	// T2 should now be ready (T1 complete)
	if len(ready) != 1 {
		t.Errorf("GetReady() returned %d tasks, want 1", len(ready))
	}
	if ready[0].ID != "T2" {
		t.Errorf("GetReady() = [%s], want [T2]", ready[0].ID)
	}
}

func TestResolver_GetReady_NonPendingExcluded(t *testing.T) {
	tasks := []*Task{
		{ID: "T1", Status: StatusInProgress, DependsOn: nil},
		{ID: "T2", Status: StatusComplete, DependsOn: nil},
		{ID: "T3", Status: StatusFailed, DependsOn: nil},
	}

	resolver := NewResolver(tasks)
	ready := resolver.GetReady()

	// None should be ready (not pending)
	if len(ready) != 0 {
		t.Errorf("GetReady() returned %d tasks, want 0", len(ready))
	}
}

func TestResolver_GetBlocking(t *testing.T) {
	tasks := []*Task{
		{ID: "T1", Status: StatusComplete},
		{ID: "T2", Status: StatusPending},
		{ID: "T3", DependsOn: []string{"T1", "T2", "missing"}},
	}

	resolver := NewResolver(tasks)
	blocking, err := resolver.GetBlocking("T3")

	if err != nil {
		t.Errorf("GetBlocking() error = %v", err)
	}

	// T2 (not complete) and missing should be blocking
	if len(blocking) != 2 {
		t.Errorf("GetBlocking() returned %d, want 2", len(blocking))
	}

	// Check T2 is in blocking list
	hasT2 := false
	hasMissing := false
	for _, b := range blocking {
		if b == "T2" {
			hasT2 = true
		}
		if strings.Contains(b, "missing") {
			hasMissing = true
		}
	}
	if !hasT2 {
		t.Error("blocking should contain T2")
	}
	if !hasMissing {
		t.Error("blocking should contain 'missing (not found)'")
	}
}

func TestResolver_GetBlocking_NotFound(t *testing.T) {
	resolver := NewResolver(nil)
	_, err := resolver.GetBlocking("missing")

	if err == nil {
		t.Error("GetBlocking() expected error for missing task")
	}
}

func TestResolver_IsBlocked(t *testing.T) {
	tasks := []*Task{
		{ID: "T1", Status: StatusComplete},
		{ID: "T2", Status: StatusPending},
		{ID: "T3", DependsOn: []string{"T1"}},       // Not blocked
		{ID: "T4", DependsOn: []string{"T2"}},       // Blocked
		{ID: "T5", DependsOn: []string{"missing"}},  // Blocked (missing)
	}

	resolver := NewResolver(tasks)

	tests := []struct {
		id      string
		blocked bool
	}{
		{"T1", false},
		{"T2", false},
		{"T3", false}, // T1 is complete
		{"T4", true},  // T2 not complete
		{"T5", true},  // missing dependency
		{"missing", true}, // not found = blocked
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			blocked := resolver.IsBlocked(tt.id)
			if blocked != tt.blocked {
				t.Errorf("IsBlocked(%s) = %v, want %v", tt.id, blocked, tt.blocked)
			}
		})
	}
}

func TestResolver_TopologicalSort(t *testing.T) {
	tasks := []*Task{
		{ID: "T3", DependsOn: []string{"T1", "T2"}},
		{ID: "T1", DependsOn: nil},
		{ID: "T2", DependsOn: []string{"T1"}},
	}

	resolver := NewResolver(tasks)
	sorted, err := resolver.TopologicalSort()

	if err != nil {
		t.Fatalf("TopologicalSort() error: %v", err)
	}

	if len(sorted) != 3 {
		t.Fatalf("TopologicalSort() returned %d tasks, want 3", len(sorted))
	}

	// T1 must come before T2 and T3
	// T2 must come before T3
	ids := taskIDs(sorted)
	t1Idx := indexOf(ids, "T1")
	t2Idx := indexOf(ids, "T2")
	t3Idx := indexOf(ids, "T3")

	if t1Idx > t2Idx || t1Idx > t3Idx {
		t.Errorf("T1 should come first: %v", ids)
	}
	if t2Idx > t3Idx {
		t.Errorf("T2 should come before T3: %v", ids)
	}
}

func TestResolver_TopologicalSort_Empty(t *testing.T) {
	resolver := NewResolver(nil)
	sorted, err := resolver.TopologicalSort()

	if err != nil {
		t.Errorf("TopologicalSort() error = %v", err)
	}
	if len(sorted) != 0 {
		t.Errorf("TopologicalSort() returned %d tasks, want 0", len(sorted))
	}
}

func TestResolver_DetectCycle(t *testing.T) {
	// A → B → C → A (cycle)
	tasks := []*Task{
		{ID: "A", DependsOn: []string{"C"}},
		{ID: "B", DependsOn: []string{"A"}},
		{ID: "C", DependsOn: []string{"B"}},
	}

	resolver := NewResolver(tasks)
	cycle := resolver.DetectCycle()

	if cycle == nil {
		t.Fatal("DetectCycle() should find cycle")
	}

	// Cycle should contain all three
	cycleStr := strings.Join(cycle, " → ")
	if !strings.Contains(cycleStr, "A") ||
		!strings.Contains(cycleStr, "B") ||
		!strings.Contains(cycleStr, "C") {
		t.Errorf("Cycle should contain A, B, C: %s", cycleStr)
	}
}

func TestResolver_DetectCycle_NoCycle(t *testing.T) {
	tasks := []*Task{
		{ID: "T1", DependsOn: nil},
		{ID: "T2", DependsOn: []string{"T1"}},
		{ID: "T3", DependsOn: []string{"T1", "T2"}},
	}

	resolver := NewResolver(tasks)
	cycle := resolver.DetectCycle()

	if cycle != nil {
		t.Errorf("DetectCycle() found false cycle: %v", cycle)
	}
}

func TestResolver_DetectCycle_SelfReference(t *testing.T) {
	tasks := []*Task{
		{ID: "T1", DependsOn: []string{"T1"}}, // Self-reference
	}

	resolver := NewResolver(tasks)
	cycle := resolver.DetectCycle()

	if cycle == nil {
		t.Fatal("DetectCycle() should find self-reference cycle")
	}
}

func TestResolver_TopologicalSort_WithCycle(t *testing.T) {
	tasks := []*Task{
		{ID: "A", DependsOn: []string{"B"}},
		{ID: "B", DependsOn: []string{"A"}},
	}

	resolver := NewResolver(tasks)
	_, err := resolver.TopologicalSort()

	if err == nil {
		t.Error("TopologicalSort() should return error for cycle")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("Error should mention cycle: %v", err)
	}
}

func TestResolver_GetLevels(t *testing.T) {
	tasks := []*Task{
		{ID: "L0-A", DependsOn: nil},
		{ID: "L0-B", DependsOn: nil},
		{ID: "L1-A", DependsOn: []string{"L0-A"}},
		{ID: "L1-B", DependsOn: []string{"L0-B"}},
		{ID: "L2", DependsOn: []string{"L1-A", "L1-B"}},
	}

	resolver := NewResolver(tasks)
	levels, err := resolver.GetLevels()

	if err != nil {
		t.Fatalf("GetLevels() error: %v", err)
	}

	if len(levels) != 3 {
		t.Errorf("Expected 3 levels, got %d", len(levels))
	}

	// Level 0 should have 2 tasks
	if len(levels[0]) != 2 {
		t.Errorf("Level 0 should have 2 tasks: %v", taskIDs(levels[0]))
	}

	// Level 1 should have 2 tasks
	if len(levels[1]) != 2 {
		t.Errorf("Level 1 should have 2 tasks: %v", taskIDs(levels[1]))
	}

	// Level 2 should have 1 task
	if len(levels[2]) != 1 {
		t.Errorf("Level 2 should have 1 task: %v", taskIDs(levels[2]))
	}
}

func TestResolver_GetLevels_Empty(t *testing.T) {
	resolver := NewResolver(nil)
	levels, err := resolver.GetLevels()

	if err != nil {
		t.Errorf("GetLevels() error = %v", err)
	}
	if levels != nil {
		t.Errorf("GetLevels() = %v, want nil for empty", levels)
	}
}

func TestResolver_GetLevels_WithCycle(t *testing.T) {
	tasks := []*Task{
		{ID: "A", DependsOn: []string{"B"}},
		{ID: "B", DependsOn: []string{"A"}},
	}

	resolver := NewResolver(tasks)
	_, err := resolver.GetLevels()

	if err == nil {
		t.Error("GetLevels() should return error for cycle")
	}
}

func TestResolver_MissingDependency(t *testing.T) {
	tasks := []*Task{
		{ID: "T1", DependsOn: []string{"missing"}},
	}

	resolver := NewResolver(tasks)

	// Should be blocked
	if !resolver.IsBlocked("T1") {
		t.Error("T1 should be blocked (missing dep)")
	}

	// GetBlocking should report missing
	blocking, _ := resolver.GetBlocking("T1")
	if len(blocking) != 1 || !strings.Contains(blocking[0], "not found") {
		t.Errorf("Should report missing dependency: %v", blocking)
	}
}

func TestResolver_Graph(t *testing.T) {
	tasks := []*Task{
		{ID: "T1", Status: StatusComplete, DependsOn: nil},
		{ID: "T2", Status: StatusPending, DependsOn: []string{"T1"}},
	}

	resolver := NewResolver(tasks)
	graph := resolver.Graph()

	if !strings.Contains(graph, "T1") {
		t.Error("Graph should contain T1")
	}
	if !strings.Contains(graph, "T2") {
		t.Error("Graph should contain T2")
	}
	if !strings.Contains(graph, "complete") {
		t.Error("Graph should contain status")
	}
}

func TestResolver_Graph_WithCycle(t *testing.T) {
	tasks := []*Task{
		{ID: "A", DependsOn: []string{"B"}},
		{ID: "B", DependsOn: []string{"A"}},
	}

	resolver := NewResolver(tasks)
	graph := resolver.Graph()

	if !strings.Contains(graph, "Error") {
		t.Error("Graph should show error for cycle")
	}
}

func TestResolver_Mermaid(t *testing.T) {
	tasks := []*Task{
		{ID: "T1", Title: "Task 1", DependsOn: nil},
		{ID: "T2", Title: "Task 2", DependsOn: []string{"T1"}},
	}

	resolver := NewResolver(tasks)
	mermaid := resolver.Mermaid()

	if !strings.Contains(mermaid, "graph TD") {
		t.Error("Mermaid should start with 'graph TD'")
	}
	if !strings.Contains(mermaid, "T1") {
		t.Error("Mermaid should contain T1")
	}
	if !strings.Contains(mermaid, "T2") {
		t.Error("Mermaid should contain T2")
	}
	if !strings.Contains(mermaid, "T1 --> T2") {
		t.Error("Mermaid should contain edge T1 --> T2")
	}
}

func TestResolver_HasTask(t *testing.T) {
	tasks := []*Task{
		{ID: "T1"},
	}

	resolver := NewResolver(tasks)

	if !resolver.HasTask("T1") {
		t.Error("HasTask(T1) should be true")
	}
	if resolver.HasTask("T2") {
		t.Error("HasTask(T2) should be false")
	}
}

func TestResolver_TaskCount(t *testing.T) {
	tasks := []*Task{
		{ID: "T1"},
		{ID: "T2"},
		{ID: "T3"},
	}

	resolver := NewResolver(tasks)

	if resolver.TaskCount() != 3 {
		t.Errorf("TaskCount() = %d, want 3", resolver.TaskCount())
	}

	empty := NewResolver(nil)
	if empty.TaskCount() != 0 {
		t.Errorf("TaskCount() for empty = %d, want 0", empty.TaskCount())
	}
}

// Helper functions
func taskIDs(tasks []*Task) []string {
	ids := make([]string, len(tasks))
	for i, t := range tasks {
		ids[i] = t.ID
	}
	return ids
}

func indexOf(slice []string, item string) int {
	for i, s := range slice {
		if s == item {
			return i
		}
	}
	return -1
}
