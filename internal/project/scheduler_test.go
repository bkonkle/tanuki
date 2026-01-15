package project

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bkonkle/tanuki/internal/task"
)

// createTestTask creates a task file in the test directory.
func createTestTask(t *testing.T, tasksDir string, taskDef *task.Task) {
	t.Helper()

	// Use the task's project directory if specified
	targetDir := tasksDir
	if taskDef.Project != "" {
		targetDir = filepath.Join(tasksDir, taskDef.Project)
		if err := os.MkdirAll(targetDir, 0750); err != nil {
			t.Fatalf("create project dir: %v", err)
		}
		// Create README.md to mark as project dir
		readmePath := filepath.Join(targetDir, "README.md")
		if _, err := os.Stat(readmePath); os.IsNotExist(err) {
			if err := os.WriteFile(readmePath, []byte("# "+taskDef.Project), 0600); err != nil {
				t.Fatalf("write readme: %v", err)
			}
		}
	}

	// Set the file path and write the task
	taskDef.FilePath = filepath.Join(targetDir, taskDef.ID+".md")
	if err := task.WriteFile(taskDef); err != nil {
		t.Fatalf("write task file: %v", err)
	}
}

// setupTestScheduler creates a scheduler with the given tasks.
func setupTestScheduler(t *testing.T, tasks []*task.Task) (*ReadinessAwareScheduler, string) {
	t.Helper()

	// Create temp directory
	tempDir := t.TempDir()
	tasksDir := filepath.Join(tempDir, "tasks")
	if err := os.MkdirAll(tasksDir, 0750); err != nil {
		t.Fatalf("create tasks dir: %v", err)
	}

	// Write task files
	for _, taskDef := range tasks {
		createTestTask(t, tasksDir, taskDef)
	}

	// Create task manager
	taskMgr := task.NewManager(&task.Config{ProjectRoot: tempDir})

	// Create scheduler
	scheduler := NewReadinessAwareScheduler(taskMgr)

	return scheduler, tempDir
}

func TestReadinessAwareScheduler_SameRoleDependency(t *testing.T) {
	// WS-B depends on WS-A (same role) - should spawn WS-A first
	tasks := []*task.Task{
		{
			ID:         "WS-A-001",
			Title:      "Task A1",
			Role:       "backend",
			Workstream: "A",
			Status:     task.StatusPending,
			FilePath:   "WS-A-001.md",
		},
		{
			ID:         "WS-B-001",
			Title:      "Task B1",
			Role:       "backend",
			Workstream: "B",
			Status:     task.StatusPending,
			DependsOn:  []string{"WS-A-001"},
			FilePath:   "WS-B-001.md",
		},
	}

	scheduler, _ := setupTestScheduler(t, tasks)
	scheduler.SetRoleConcurrency("backend", 1)

	if err := scheduler.Initialize(); err != nil {
		t.Fatalf("Initialize() error: %v", err)
	}

	// Should select WS-A first (it has a ready task)
	next := scheduler.GetNextWorkstream("backend")
	if next == nil {
		t.Fatal("GetNextWorkstream() returned nil, expected WS-A")
	}
	if next.Workstream != "A" {
		t.Errorf("GetNextWorkstream() = %s, want A", next.Workstream)
	}

	// WS-A has 1 ready task
	if next.ReadyTaskCount != 1 {
		t.Errorf("ReadyTaskCount = %d, want 1", next.ReadyTaskCount)
	}

	// Mark WS-A as active
	scheduler.ActivateWorkstream("backend", "A")

	// Should not return another workstream (at concurrency limit)
	next = scheduler.GetNextWorkstream("backend")
	if next != nil {
		t.Errorf("GetNextWorkstream() = %v, want nil (at limit)", next.Workstream)
	}

	// WS-B should be blocked
	blocked := scheduler.GetBlockedWorkstreams("backend")
	if len(blocked) != 1 {
		t.Errorf("GetBlockedWorkstreams() returned %d, want 1", len(blocked))
	}
	if len(blocked) > 0 && blocked[0].Workstream != "B" {
		t.Errorf("blocked workstream = %s, want B", blocked[0].Workstream)
	}
}

func TestReadinessAwareScheduler_CrossRoleDependency(t *testing.T) {
	// FE depends on BE - BE should be ready, FE blocked until BE completes
	tasks := []*task.Task{
		{
			ID:         "BE-001",
			Title:      "Backend Task",
			Role:       "backend",
			Workstream: "main",
			Status:     task.StatusPending,
			FilePath:   "BE-001.md",
		},
		{
			ID:         "FE-001",
			Title:      "Frontend Task",
			Role:       "frontend",
			Workstream: "main",
			Status:     task.StatusPending,
			DependsOn:  []string{"BE-001"},
			FilePath:   "FE-001.md",
		},
	}

	scheduler, _ := setupTestScheduler(t, tasks)
	scheduler.SetRoleConcurrency("backend", 1)
	scheduler.SetRoleConcurrency("frontend", 1)

	if err := scheduler.Initialize(); err != nil {
		t.Fatalf("Initialize() error: %v", err)
	}

	// Backend should be ready
	beNext := scheduler.GetNextWorkstream("backend")
	if beNext == nil {
		t.Fatal("Backend GetNextWorkstream() returned nil")
	}
	if beNext.Workstream != "main" {
		t.Errorf("Backend workstream = %s, want main", beNext.Workstream)
	}

	// Frontend should be blocked (depends on BE-001)
	feNext := scheduler.GetNextWorkstream("frontend")
	if feNext != nil {
		t.Errorf("Frontend GetNextWorkstream() = %v, want nil (blocked)", feNext.Workstream)
	}

	blocked := scheduler.GetBlockedWorkstreams("frontend")
	if len(blocked) != 1 {
		t.Errorf("Frontend blocked count = %d, want 1", len(blocked))
	}
}

func TestReadinessAwareScheduler_CircularDependency(t *testing.T) {
	// Circular dependency - should fail with cycle error
	tasks := []*task.Task{
		{
			ID:         "A-001",
			Title:      "Task A",
			Role:       "backend",
			Workstream: "A",
			Status:     task.StatusPending,
			DependsOn:  []string{"B-001"},
			FilePath:   "A-001.md",
		},
		{
			ID:         "B-001",
			Title:      "Task B",
			Role:       "backend",
			Workstream: "B",
			Status:     task.StatusPending,
			DependsOn:  []string{"A-001"},
			FilePath:   "B-001.md",
		},
	}

	scheduler, _ := setupTestScheduler(t, tasks)
	scheduler.SetRoleConcurrency("backend", 1)

	err := scheduler.Initialize()
	if err == nil {
		t.Error("Initialize() should return error for circular dependency")
	}
}

func TestReadinessAwareScheduler_DynamicRebalancing(t *testing.T) {
	// WS-B depends on WS-A task - after WS-A completes, WS-B should become ready
	tasks := []*task.Task{
		{
			ID:         "WS-A-001",
			Title:      "Task A1",
			Role:       "backend",
			Workstream: "A",
			Status:     task.StatusPending,
			FilePath:   "WS-A-001.md",
		},
		{
			ID:         "WS-B-001",
			Title:      "Task B1",
			Role:       "backend",
			Workstream: "B",
			Status:     task.StatusPending,
			DependsOn:  []string{"WS-A-001"},
			FilePath:   "WS-B-001.md",
		},
	}

	scheduler, tempDir := setupTestScheduler(t, tasks)
	scheduler.SetRoleConcurrency("backend", 1)

	if err := scheduler.Initialize(); err != nil {
		t.Fatalf("Initialize() error: %v", err)
	}

	// Initially, WS-B should be blocked
	blocked := scheduler.GetBlockedWorkstreams("backend")
	if len(blocked) != 1 {
		t.Fatalf("Initial blocked count = %d, want 1", len(blocked))
	}

	// Get and activate WS-A (this removes it from the ready queue)
	wsA := scheduler.GetNextWorkstream("backend")
	if wsA == nil || wsA.Workstream != "A" {
		t.Fatalf("Expected to get workstream A, got %v", wsA)
	}
	scheduler.ActivateWorkstream("backend", "A")

	// Simulate completing WS-A-001
	taskPath := filepath.Join(tempDir, "tasks", "WS-A-001.md")
	content, err := os.ReadFile(taskPath) //nolint:gosec // G304: Test file path
	if err != nil {
		t.Fatalf("read task file: %v", err)
	}

	// Update status to complete
	newContent := string(content)
	newContent = replaceStatus(newContent, "complete")
	if err := os.WriteFile(taskPath, []byte(newContent), 0600); err != nil {
		t.Fatalf("write task file: %v", err)
	}

	// Trigger rebalancing
	scheduler.OnTaskComplete("WS-A-001")

	// Now WS-B should be ready (moved from blocked to ready queue)
	blocked = scheduler.GetBlockedWorkstreams("backend")
	if len(blocked) != 0 {
		t.Errorf("After completion, blocked count = %d, want 0", len(blocked))
	}

	ready := scheduler.GetReadyWorkstreams("backend")
	if len(ready) != 1 {
		t.Errorf("After completion, ready count = %d, want 1", len(ready))
	}
	if len(ready) > 0 && ready[0].Workstream != "B" {
		t.Errorf("Ready workstream = %s, want B", ready[0].Workstream)
	}
}

func TestReadinessAwareScheduler_NoReadyWorkstreams(t *testing.T) {
	// All workstreams blocked by dependencies that don't exist
	tasks := []*task.Task{
		{
			ID:         "A-001",
			Title:      "Task A",
			Role:       "backend",
			Workstream: "A",
			Status:     task.StatusPending,
			DependsOn:  []string{"missing-001"},
			FilePath:   "A-001.md",
		},
	}

	scheduler, _ := setupTestScheduler(t, tasks)
	scheduler.SetRoleConcurrency("backend", 1)

	if err := scheduler.Initialize(); err != nil {
		t.Fatalf("Initialize() error: %v", err)
	}

	// Should return nil (no ready workstreams)
	next := scheduler.GetNextWorkstream("backend")
	if next != nil {
		t.Errorf("GetNextWorkstream() = %v, want nil", next.Workstream)
	}

	// Should show 1 blocked workstream
	blocked := scheduler.GetBlockedWorkstreams("backend")
	if len(blocked) != 1 {
		t.Errorf("Blocked count = %d, want 1", len(blocked))
	}
}

func TestReadinessAwareScheduler_MultipleReadyWorkstreams(t *testing.T) {
	// Multiple workstreams with ready tasks - should prioritize by score
	tasks := []*task.Task{
		{
			ID:         "WS-A-001",
			Title:      "Task A1",
			Role:       "backend",
			Workstream: "A",
			Status:     task.StatusPending,
			Priority:   task.PriorityLow,
			FilePath:   "WS-A-001.md",
		},
		{
			ID:         "WS-B-001",
			Title:      "Task B1",
			Role:       "backend",
			Workstream: "B",
			Status:     task.StatusPending,
			Priority:   task.PriorityHigh,
			FilePath:   "WS-B-001.md",
		},
		{
			ID:         "WS-C-001",
			Title:      "Task C1",
			Role:       "backend",
			Workstream: "C",
			Status:     task.StatusPending,
			Priority:   task.PriorityCritical,
			FilePath:   "WS-C-001.md",
		},
	}

	scheduler, _ := setupTestScheduler(t, tasks)
	scheduler.SetRoleConcurrency("backend", 3)

	if err := scheduler.Initialize(); err != nil {
		t.Fatalf("Initialize() error: %v", err)
	}

	// All three should be ready
	ready := scheduler.GetReadyWorkstreams("backend")
	if len(ready) != 3 {
		t.Errorf("Ready count = %d, want 3", len(ready))
	}

	// First should be highest priority (C - critical)
	first := scheduler.GetNextWorkstream("backend")
	if first == nil || first.Workstream != "C" {
		name := "nil"
		if first != nil {
			name = first.Workstream
		}
		t.Errorf("First workstream = %s, want C (critical)", name)
	}
}

func TestReadinessAwareScheduler_WorkstreamWithMultipleTasks(t *testing.T) {
	// Workstream with multiple tasks, some blocked
	tasks := []*task.Task{
		{
			ID:         "WS-A-001",
			Title:      "Task A1",
			Role:       "backend",
			Workstream: "A",
			Status:     task.StatusPending,
			FilePath:   "WS-A-001.md",
		},
		{
			ID:         "WS-A-002",
			Title:      "Task A2",
			Role:       "backend",
			Workstream: "A",
			Status:     task.StatusPending,
			DependsOn:  []string{"WS-A-001"},
			FilePath:   "WS-A-002.md",
		},
		{
			ID:         "WS-A-003",
			Title:      "Task A3",
			Role:       "backend",
			Workstream: "A",
			Status:     task.StatusPending,
			DependsOn:  []string{"WS-A-002"},
			FilePath:   "WS-A-003.md",
		},
	}

	scheduler, _ := setupTestScheduler(t, tasks)
	scheduler.SetRoleConcurrency("backend", 1)

	if err := scheduler.Initialize(); err != nil {
		t.Fatalf("Initialize() error: %v", err)
	}

	// Workstream should be ready (has 1 ready task, 2 blocked)
	ws := scheduler.GetNextWorkstream("backend")
	if ws == nil {
		t.Fatal("GetNextWorkstream() returned nil")
	}

	if ws.ReadyTaskCount != 1 {
		t.Errorf("ReadyTaskCount = %d, want 1", ws.ReadyTaskCount)
	}
	if ws.BlockedTaskCount != 2 {
		t.Errorf("BlockedTaskCount = %d, want 2", ws.BlockedTaskCount)
	}
	if ws.TotalTaskCount != 3 {
		t.Errorf("TotalTaskCount = %d, want 3", ws.TotalTaskCount)
	}
}

func TestReadinessAwareScheduler_DeadlockDetection(t *testing.T) {
	// Cross-role mutual dependency - BE waits on FE, FE waits on BE
	tasks := []*task.Task{
		{
			ID:         "BE-001",
			Title:      "Backend Task",
			Role:       "backend",
			Workstream: "main",
			Status:     task.StatusPending,
			DependsOn:  []string{"FE-001"},
			FilePath:   "BE-001.md",
		},
		{
			ID:         "FE-001",
			Title:      "Frontend Task",
			Role:       "frontend",
			Workstream: "main",
			Status:     task.StatusPending,
			DependsOn:  []string{"BE-001"},
			FilePath:   "FE-001.md",
		},
	}

	scheduler, _ := setupTestScheduler(t, tasks)
	scheduler.SetRoleConcurrency("backend", 1)
	scheduler.SetRoleConcurrency("frontend", 1)

	// This should fail during initialization due to cycle detection
	err := scheduler.Initialize()
	if err == nil {
		t.Error("Initialize() should return error for cross-role cycle")
	}
}

func TestWorkstreamReadiness_IsReady(t *testing.T) {
	tests := []struct {
		name           string
		readyTaskCount int
		want           bool
	}{
		{"no ready tasks", 0, false},
		{"one ready task", 1, true},
		{"multiple ready tasks", 5, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wr := &WorkstreamReadiness{ReadyTaskCount: tt.readyTaskCount}
			if got := wr.IsReady(); got != tt.want {
				t.Errorf("IsReady() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWorkstreamReadiness_ReadinessScore(t *testing.T) {
	// Test that ready workstreams score higher than blocked ones
	ready := &WorkstreamReadiness{
		ReadyTaskCount:         2,
		FirstReadyTaskPriority: task.PriorityHigh,
		DependentWorkstreams:   []string{"ws1", "ws2"},
	}

	blocked := &WorkstreamReadiness{
		ReadyTaskCount:       0,
		BlockedTaskCount:     3,
		DependentWorkstreams: []string{"ws1"},
	}

	if ready.ReadinessScore() <= blocked.ReadinessScore() {
		t.Errorf("Ready score (%d) should be higher than blocked score (%d)",
			ready.ReadinessScore(), blocked.ReadinessScore())
	}

	// Test that more dependents = higher score
	moreDeps := &WorkstreamReadiness{
		ReadyTaskCount:         1,
		FirstReadyTaskPriority: task.PriorityMedium,
		DependentWorkstreams:   []string{"ws1", "ws2", "ws3", "ws4"},
	}

	lessDeps := &WorkstreamReadiness{
		ReadyTaskCount:         1,
		FirstReadyTaskPriority: task.PriorityMedium,
		DependentWorkstreams:   []string{"ws1"},
	}

	if moreDeps.ReadinessScore() <= lessDeps.ReadinessScore() {
		t.Errorf("More dependents score (%d) should be higher than less dependents score (%d)",
			moreDeps.ReadinessScore(), lessDeps.ReadinessScore())
	}
}

// replaceStatus replaces the status in a task file content.
func replaceStatus(content, newStatus string) string {
	// Simple replacement - find "status: pending" and replace with "status: complete"
	lines := make([]byte, 0, len(content))
	for _, line := range []byte(content) {
		lines = append(lines, line)
	}

	result := ""
	for _, line := range splitLines(content) {
		if len(line) > 8 && line[:8] == "status: " {
			result += "status: " + newStatus + "\n"
		} else {
			result += line + "\n"
		}
	}
	return result
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
