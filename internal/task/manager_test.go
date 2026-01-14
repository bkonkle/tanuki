package task

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestNewManager(t *testing.T) {
	t.Run("default tasks directory", func(t *testing.T) {
		cfg := &Config{ProjectRoot: "/tmp/test"}
		mgr := NewManager(cfg)

		if mgr == nil {
			t.Fatal("NewManager() returned nil")
		}
		if mgr.config != cfg {
			t.Error("config not set correctly")
		}
		if mgr.tasksDir != "/tmp/test/tasks" {
			t.Errorf("tasksDir = %q, want /tmp/test/tasks", mgr.tasksDir)
		}
		if mgr.tasks == nil {
			t.Error("tasks map not initialized")
		}
	})

	t.Run("custom tasks directory", func(t *testing.T) {
		cfg := &Config{ProjectRoot: "/tmp/test", TasksDir: ".tanuki/tasks"}
		mgr := NewManager(cfg)

		if mgr.tasksDir != "/tmp/test/.tanuki/tasks" {
			t.Errorf("tasksDir = %q, want /tmp/test/.tanuki/tasks", mgr.tasksDir)
		}
	})
}

func TestManager_Scan(t *testing.T) {
	// Setup temp directory with task files
	dir := t.TempDir()
	tasksDir := filepath.Join(dir, "tasks")
	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		t.Fatalf("Failed to create tasks dir: %v", err)
	}

	// Create test task
	taskContent := `---
id: TASK-001
title: Test Task
role: backend
priority: high
status: pending
---

Test content
`
	if err := os.WriteFile(filepath.Join(tasksDir, "TASK-001-test.md"), []byte(taskContent), 0644); err != nil {
		t.Fatalf("Failed to write test task: %v", err)
	}

	// Scan
	mgr := NewManager(&Config{ProjectRoot: dir})
	tasks, err := mgr.Scan()

	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}

	if len(tasks) != 1 {
		t.Errorf("Scan() returned %d tasks, want 1", len(tasks))
	}

	if tasks[0].ID != "TASK-001" {
		t.Errorf("Task ID = %q, want TASK-001", tasks[0].ID)
	}
}

func TestManager_Scan_NoDirectory(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(&Config{ProjectRoot: dir})

	// No .tanuki/tasks directory
	tasks, err := mgr.Scan()

	if err != nil {
		t.Errorf("Scan() error = %v, want nil for missing directory", err)
	}
	if tasks != nil && len(tasks) != 0 {
		t.Errorf("Scan() returned %d tasks, want 0", len(tasks))
	}
}

func TestManager_Scan_SkipsInvalidFiles(t *testing.T) {
	dir := t.TempDir()
	tasksDir := filepath.Join(dir, "tasks")
	os.MkdirAll(tasksDir, 0755)

	// Valid task
	validTask := `---
id: TASK-001
title: Valid Task
role: backend
---

Content
`
	os.WriteFile(filepath.Join(tasksDir, "valid.md"), []byte(validTask), 0644)

	// Invalid task (missing role)
	invalidTask := `---
id: TASK-002
title: Invalid Task
---

Content
`
	os.WriteFile(filepath.Join(tasksDir, "invalid.md"), []byte(invalidTask), 0644)

	// Non-md file (should be skipped)
	os.WriteFile(filepath.Join(tasksDir, "readme.txt"), []byte("text file"), 0644)

	mgr := NewManager(&Config{ProjectRoot: dir})
	tasks, err := mgr.Scan()

	if err != nil {
		t.Errorf("Scan() error = %v", err)
	}

	// Should only have the valid task
	if len(tasks) != 1 {
		t.Errorf("Scan() returned %d tasks, want 1", len(tasks))
	}
	if tasks[0].ID != "TASK-001" {
		t.Errorf("Task ID = %q, want TASK-001", tasks[0].ID)
	}
}

func TestManager_Get(t *testing.T) {
	mgr := &Manager{
		tasks: map[string]*Task{
			"T1": {ID: "T1", Title: "Task 1"},
			"T2": {ID: "T2", Title: "Task 2"},
		},
	}

	t.Run("existing task", func(t *testing.T) {
		task, err := mgr.Get("T1")
		if err != nil {
			t.Errorf("Get(T1) error = %v", err)
		}
		if task.Title != "Task 1" {
			t.Errorf("Task.Title = %q, want Task 1", task.Title)
		}
	})

	t.Run("non-existent task", func(t *testing.T) {
		_, err := mgr.Get("T999")
		if err == nil {
			t.Error("Get(T999) expected error")
		}
	})
}

func TestManager_List(t *testing.T) {
	mgr := &Manager{
		tasks: map[string]*Task{
			"T1": {ID: "T1", Priority: PriorityLow},
			"T2": {ID: "T2", Priority: PriorityCritical},
			"T3": {ID: "T3", Priority: PriorityHigh},
		},
	}

	t.Run("unsorted", func(t *testing.T) {
		tasks := mgr.List()
		if len(tasks) != 3 {
			t.Errorf("List() returned %d tasks, want 3", len(tasks))
		}
	})

	t.Run("sorted by priority", func(t *testing.T) {
		tasks := mgr.List(SortByPriority())
		if len(tasks) != 3 {
			t.Errorf("List() returned %d tasks, want 3", len(tasks))
		}
		// First should be critical
		if tasks[0].Priority != PriorityCritical {
			t.Errorf("First task priority = %s, want critical", tasks[0].Priority)
		}
		// Last should be low
		if tasks[2].Priority != PriorityLow {
			t.Errorf("Last task priority = %s, want low", tasks[2].Priority)
		}
	})
}

func TestManager_GetByRole(t *testing.T) {
	mgr := &Manager{
		tasks: map[string]*Task{
			"T1": {ID: "T1", Role: "backend"},
			"T2": {ID: "T2", Role: "frontend"},
			"T3": {ID: "T3", Role: "backend"},
		},
	}

	backend := mgr.GetByRole("backend")
	if len(backend) != 2 {
		t.Errorf("GetByRole(backend) returned %d, want 2", len(backend))
	}

	frontend := mgr.GetByRole("frontend")
	if len(frontend) != 1 {
		t.Errorf("GetByRole(frontend) returned %d, want 1", len(frontend))
	}

	qa := mgr.GetByRole("qa")
	if len(qa) != 0 {
		t.Errorf("GetByRole(qa) returned %d, want 0", len(qa))
	}
}

func TestManager_GetByStatus(t *testing.T) {
	mgr := &Manager{
		tasks: map[string]*Task{
			"T1": {ID: "T1", Status: StatusPending},
			"T2": {ID: "T2", Status: StatusInProgress},
			"T3": {ID: "T3", Status: StatusPending},
			"T4": {ID: "T4", Status: StatusComplete},
		},
	}

	pending := mgr.GetByStatus(StatusPending)
	if len(pending) != 2 {
		t.Errorf("GetByStatus(pending) returned %d, want 2", len(pending))
	}

	complete := mgr.GetByStatus(StatusComplete)
	if len(complete) != 1 {
		t.Errorf("GetByStatus(complete) returned %d, want 1", len(complete))
	}
}

func TestManager_GetPending(t *testing.T) {
	mgr := &Manager{
		tasks: map[string]*Task{
			"T1": {ID: "T1", Status: StatusPending, Priority: PriorityLow},
			"T2": {ID: "T2", Status: StatusComplete, Priority: PriorityCritical},
			"T3": {ID: "T3", Status: StatusPending, Priority: PriorityHigh},
		},
	}

	pending := mgr.GetPending()
	if len(pending) != 2 {
		t.Errorf("GetPending() returned %d, want 2", len(pending))
	}

	// Should be sorted by priority
	if pending[0].Priority != PriorityHigh {
		t.Errorf("First pending priority = %s, want high", pending[0].Priority)
	}
	if pending[1].Priority != PriorityLow {
		t.Errorf("Second pending priority = %s, want low", pending[1].Priority)
	}
}

func TestManager_GetNextAvailable(t *testing.T) {
	mgr := &Manager{
		tasks: map[string]*Task{
			"T1": {ID: "T1", Role: "backend", Status: StatusPending, Priority: PriorityLow},
			"T2": {ID: "T2", Role: "backend", Status: StatusPending, Priority: PriorityHigh},
			"T3": {ID: "T3", Role: "frontend", Status: StatusPending},
			"T4": {ID: "T4", Role: "backend", Status: StatusComplete},
		},
	}

	t.Run("returns highest priority", func(t *testing.T) {
		task, err := mgr.GetNextAvailable("backend")
		if err != nil {
			t.Errorf("GetNextAvailable() error = %v", err)
		}
		if task.ID != "T2" {
			t.Errorf("GetNextAvailable() = %s, want T2 (highest priority)", task.ID)
		}
	})

	t.Run("no pending tasks for role", func(t *testing.T) {
		_, err := mgr.GetNextAvailable("qa")
		if err == nil {
			t.Error("GetNextAvailable(qa) expected error")
		}
	})
}

func TestManager_GetNextAvailable_AllBlocked(t *testing.T) {
	mgr := &Manager{
		tasks: map[string]*Task{
			"T1": {ID: "T1", Role: "backend", Status: StatusPending, DependsOn: []string{"T2"}},
			"T2": {ID: "T2", Role: "backend", Status: StatusPending}, // T1 depends on T2 which is not complete
		},
	}

	_, err := mgr.GetNextAvailable("backend")
	// Should find T2 since it has no dependencies
	if err != nil {
		t.Errorf("GetNextAvailable() error = %v, expected to find T2", err)
	}
}

func TestManager_IsBlocked(t *testing.T) {
	mgr := &Manager{
		tasks: map[string]*Task{
			"T1": {ID: "T1", Status: StatusComplete},
			"T2": {ID: "T2", Status: StatusPending},
			"T3": {ID: "T3", DependsOn: []string{"T1"}},       // Not blocked
			"T4": {ID: "T4", DependsOn: []string{"T2"}},       // Blocked
			"T5": {ID: "T5", DependsOn: []string{"T1", "T2"}}, // Blocked
			"T6": {ID: "T6", DependsOn: []string{"missing"}},  // Blocked (missing dep)
		},
	}

	tests := []struct {
		id      string
		blocked bool
	}{
		{"T1", false},
		{"T2", false}, // No dependencies
		{"T3", false}, // T1 is complete
		{"T4", true},  // T2 not complete
		{"T5", true},  // T2 not complete
		{"T6", true},  // missing dependency
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			blocked, err := mgr.IsBlocked(tt.id)
			if err != nil {
				t.Errorf("IsBlocked(%s) error: %v", tt.id, err)
			}
			if blocked != tt.blocked {
				t.Errorf("IsBlocked(%s) = %v, want %v", tt.id, blocked, tt.blocked)
			}
		})
	}

	t.Run("not found", func(t *testing.T) {
		_, err := mgr.IsBlocked("missing")
		if err == nil {
			t.Error("IsBlocked(missing) expected error")
		}
	})
}

func TestManager_GetBlockingTasks(t *testing.T) {
	mgr := &Manager{
		tasks: map[string]*Task{
			"T1": {ID: "T1", Status: StatusComplete},
			"T2": {ID: "T2", Status: StatusPending},
			"T3": {ID: "T3", DependsOn: []string{"T1", "T2"}},
		},
	}

	blocking, err := mgr.GetBlockingTasks("T3")
	if err != nil {
		t.Errorf("GetBlockingTasks() error = %v", err)
	}

	if len(blocking) != 1 {
		t.Errorf("GetBlockingTasks() returned %d, want 1", len(blocking))
	}
	if blocking[0] != "T2" {
		t.Errorf("Blocking task = %s, want T2", blocking[0])
	}
}

func TestManager_UpdateStatus(t *testing.T) {
	dir := t.TempDir()
	tasksDir := filepath.Join(dir, "tasks")
	os.MkdirAll(tasksDir, 0755)

	taskPath := filepath.Join(tasksDir, "TASK-001.md")
	os.WriteFile(taskPath, []byte(`---
id: TASK-001
title: Test
role: backend
status: pending
---

Content
`), 0644)

	mgr := NewManager(&Config{ProjectRoot: dir})
	mgr.Scan()

	// Update status
	err := mgr.UpdateStatus("TASK-001", StatusInProgress)
	if err != nil {
		t.Fatalf("UpdateStatus() error: %v", err)
	}

	// Verify in memory
	task, _ := mgr.Get("TASK-001")
	if task.Status != StatusInProgress {
		t.Errorf("Status = %v, want in_progress", task.Status)
	}

	// Verify on disk (re-scan)
	mgr2 := NewManager(&Config{ProjectRoot: dir})
	mgr2.Scan()
	task2, _ := mgr2.Get("TASK-001")
	if task2.Status != StatusInProgress {
		t.Errorf("Persisted status = %v, want in_progress", task2.Status)
	}
}

func TestManager_UpdateStatus_NotFound(t *testing.T) {
	mgr := &Manager{tasks: make(map[string]*Task)}

	err := mgr.UpdateStatus("missing", StatusComplete)
	if err == nil {
		t.Error("UpdateStatus() expected error for missing task")
	}
}

func TestManager_Assign(t *testing.T) {
	dir := t.TempDir()
	tasksDir := filepath.Join(dir, "tasks")
	os.MkdirAll(tasksDir, 0755)

	taskPath := filepath.Join(tasksDir, "TASK-001.md")
	os.WriteFile(taskPath, []byte(`---
id: TASK-001
title: Test
role: backend
status: pending
---

Content
`), 0644)

	mgr := NewManager(&Config{ProjectRoot: dir})
	mgr.Scan()

	err := mgr.Assign("TASK-001", "agent-1")
	if err != nil {
		t.Fatalf("Assign() error: %v", err)
	}

	task, _ := mgr.Get("TASK-001")
	if task.AssignedTo != "agent-1" {
		t.Errorf("AssignedTo = %q, want agent-1", task.AssignedTo)
	}
	if task.Status != StatusAssigned {
		t.Errorf("Status = %s, want assigned", task.Status)
	}
}

func TestManager_Assign_NotAvailable(t *testing.T) {
	mgr := &Manager{
		tasks: map[string]*Task{
			"T1": {ID: "T1", Status: StatusInProgress},
		},
	}

	err := mgr.Assign("T1", "agent-1")
	if err == nil {
		t.Error("Assign() expected error for in_progress task")
	}
}

func TestManager_Unassign(t *testing.T) {
	dir := t.TempDir()
	tasksDir := filepath.Join(dir, "tasks")
	os.MkdirAll(tasksDir, 0755)

	taskPath := filepath.Join(tasksDir, "TASK-001.md")
	os.WriteFile(taskPath, []byte(`---
id: TASK-001
title: Test
role: backend
status: assigned
assigned_to: agent-1
---

Content
`), 0644)

	mgr := NewManager(&Config{ProjectRoot: dir})
	mgr.Scan()

	err := mgr.Unassign("TASK-001")
	if err != nil {
		t.Fatalf("Unassign() error: %v", err)
	}

	task, _ := mgr.Get("TASK-001")
	if task.AssignedTo != "" {
		t.Errorf("AssignedTo = %q, want empty", task.AssignedTo)
	}
	if task.Status != StatusPending {
		t.Errorf("Status = %s, want pending", task.Status)
	}
}

func TestManager_Unassign_KeepsCompleteStatus(t *testing.T) {
	dir := t.TempDir()
	tasksDir := filepath.Join(dir, "tasks")
	os.MkdirAll(tasksDir, 0755)

	taskPath := filepath.Join(tasksDir, "TASK-001.md")
	os.WriteFile(taskPath, []byte(`---
id: TASK-001
title: Test
role: backend
status: complete
assigned_to: agent-1
---

Content
`), 0644)

	mgr := NewManager(&Config{ProjectRoot: dir})
	mgr.Scan()

	err := mgr.Unassign("TASK-001")
	if err != nil {
		t.Fatalf("Unassign() error: %v", err)
	}

	task, _ := mgr.Get("TASK-001")
	if task.Status != StatusComplete {
		t.Errorf("Status = %s, want complete (should not change)", task.Status)
	}
}

func TestManager_UpdateBlockedStatus(t *testing.T) {
	dir := t.TempDir()
	tasksDir := filepath.Join(dir, "tasks")
	os.MkdirAll(tasksDir, 0755)

	// T1 is pending with no deps
	os.WriteFile(filepath.Join(tasksDir, "T1.md"), []byte(`---
id: T1
title: Task 1
role: backend
status: pending
---
Content
`), 0644)

	// T2 depends on T1 (should become blocked)
	os.WriteFile(filepath.Join(tasksDir, "T2.md"), []byte(`---
id: T2
title: Task 2
role: backend
status: pending
depends_on: [T1]
---
Content
`), 0644)

	mgr := NewManager(&Config{ProjectRoot: dir})
	mgr.Scan()

	err := mgr.UpdateBlockedStatus()
	if err != nil {
		t.Fatalf("UpdateBlockedStatus() error: %v", err)
	}

	// T1 should still be pending
	t1, _ := mgr.Get("T1")
	if t1.Status != StatusPending {
		t.Errorf("T1 status = %s, want pending", t1.Status)
	}

	// T2 should be blocked
	t2, _ := mgr.Get("T2")
	if t2.Status != StatusBlocked {
		t.Errorf("T2 status = %s, want blocked", t2.Status)
	}

	// Now complete T1 and update again
	mgr.UpdateStatus("T1", StatusComplete)
	mgr.UpdateBlockedStatus()

	// T2 should now be pending
	t2, _ = mgr.Get("T2")
	if t2.Status != StatusPending {
		t.Errorf("T2 status after unblock = %s, want pending", t2.Status)
	}
}

func TestManager_Stats(t *testing.T) {
	mgr := &Manager{
		tasks: map[string]*Task{
			"T1": {ID: "T1", Status: StatusPending, Role: "backend", Priority: PriorityHigh},
			"T2": {ID: "T2", Status: StatusPending, Role: "frontend", Priority: PriorityHigh},
			"T3": {ID: "T3", Status: StatusComplete, Role: "backend", Priority: PriorityLow},
		},
	}

	stats := mgr.Stats()

	if stats.Total != 3 {
		t.Errorf("Total = %d, want 3", stats.Total)
	}
	if stats.ByStatus[StatusPending] != 2 {
		t.Errorf("ByStatus[pending] = %d, want 2", stats.ByStatus[StatusPending])
	}
	if stats.ByRole["backend"] != 2 {
		t.Errorf("ByRole[backend] = %d, want 2", stats.ByRole["backend"])
	}
	if stats.ByPriority[PriorityHigh] != 2 {
		t.Errorf("ByPriority[high] = %d, want 2", stats.ByPriority[PriorityHigh])
	}
}

func TestManager_TasksDir(t *testing.T) {
	cfg := &Config{ProjectRoot: "/test/project"}
	mgr := NewManager(cfg)

	dir := mgr.TasksDir()
	expected := "/test/project/tasks"
	if dir != expected {
		t.Errorf("TasksDir() = %q, want %q", dir, expected)
	}
}

func TestManager_ConcurrentAccess(t *testing.T) {
	mgr := &Manager{
		tasks: map[string]*Task{
			"T1": {ID: "T1", Role: "backend", Status: StatusPending},
		},
	}

	// Concurrent reads
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mgr.Get("T1")
			mgr.GetByRole("backend")
			mgr.IsBlocked("T1")
			mgr.List()
			mgr.Stats()
		}()
	}
	wg.Wait()
}

func TestManager_GetByWorkstream(t *testing.T) {
	mgr := &Manager{
		tasks: map[string]*Task{
			"T1": {ID: "T1", Role: "backend", Workstream: "auth-feature", Priority: PriorityHigh},
			"T2": {ID: "T2", Role: "backend", Workstream: "auth-feature", Priority: PriorityLow},
			"T3": {ID: "T3", Role: "backend", Workstream: "api-refactor"},
			"T4": {ID: "T4", Role: "frontend"}, // No workstream - uses ID
		},
	}

	t.Run("explicit workstream", func(t *testing.T) {
		tasks := mgr.GetByWorkstream("auth-feature")
		if len(tasks) != 2 {
			t.Errorf("GetByWorkstream(auth-feature) returned %d, want 2", len(tasks))
		}
		// Should be sorted by priority
		if tasks[0].ID != "T1" {
			t.Errorf("First task = %s, want T1 (higher priority)", tasks[0].ID)
		}
	})

	t.Run("implicit workstream from ID", func(t *testing.T) {
		tasks := mgr.GetByWorkstream("T4")
		if len(tasks) != 1 {
			t.Errorf("GetByWorkstream(T4) returned %d, want 1", len(tasks))
		}
	})

	t.Run("no matches", func(t *testing.T) {
		tasks := mgr.GetByWorkstream("nonexistent")
		if len(tasks) != 0 {
			t.Errorf("GetByWorkstream(nonexistent) returned %d, want 0", len(tasks))
		}
	})
}

func TestManager_GetByRoleAndWorkstream(t *testing.T) {
	mgr := &Manager{
		tasks: map[string]*Task{
			"T1": {ID: "T1", Role: "backend", Workstream: "auth-feature"},
			"T2": {ID: "T2", Role: "backend", Workstream: "api-refactor"},
			"T3": {ID: "T3", Role: "frontend", Workstream: "auth-feature"},
		},
	}

	tasks := mgr.GetByRoleAndWorkstream("backend", "auth-feature")
	if len(tasks) != 1 {
		t.Errorf("GetByRoleAndWorkstream() returned %d, want 1", len(tasks))
	}
	if tasks[0].ID != "T1" {
		t.Errorf("Task ID = %s, want T1", tasks[0].ID)
	}
}

func TestManager_GetWorkstreams(t *testing.T) {
	mgr := &Manager{
		tasks: map[string]*Task{
			"T1": {ID: "T1", Role: "backend", Workstream: "auth-feature", Priority: PriorityLow},
			"T2": {ID: "T2", Role: "backend", Workstream: "api-refactor", Priority: PriorityCritical},
			"T3": {ID: "T3", Role: "backend", Workstream: "auth-feature", Priority: PriorityHigh},
			"T4": {ID: "T4", Role: "frontend", Workstream: "ui-redesign"},
		},
	}

	workstreams := mgr.GetWorkstreams("backend")
	if len(workstreams) != 2 {
		t.Errorf("GetWorkstreams(backend) returned %d, want 2", len(workstreams))
	}

	// Should be sorted by priority
	// api-refactor has critical priority, auth-feature has high (from T3)
	if workstreams[0] != "api-refactor" {
		t.Errorf("First workstream = %s, want api-refactor (critical priority)", workstreams[0])
	}
	if workstreams[1] != "auth-feature" {
		t.Errorf("Second workstream = %s, want auth-feature", workstreams[1])
	}
}

func TestTask_GetWorkstream(t *testing.T) {
	t.Run("explicit workstream", func(t *testing.T) {
		task := &Task{ID: "T1", Workstream: "auth-feature"}
		if task.GetWorkstream() != "auth-feature" {
			t.Errorf("GetWorkstream() = %s, want auth-feature", task.GetWorkstream())
		}
	})

	t.Run("implicit workstream from ID", func(t *testing.T) {
		task := &Task{ID: "T2"}
		if task.GetWorkstream() != "T2" {
			t.Errorf("GetWorkstream() = %s, want T2", task.GetWorkstream())
		}
	})
}

func TestManager_Scan_ProjectFolders(t *testing.T) {
	// Setup temp directory with project folder structure
	dir := t.TempDir()
	tasksDir := filepath.Join(dir, "tasks")
	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		t.Fatalf("Failed to create tasks dir: %v", err)
	}

	// Create a project folder
	projectDir := filepath.Join(tasksDir, "auth-feature")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("Failed to create project dir: %v", err)
	}

	// Create project.md (makes it a project folder)
	projectMd := `# Project: auth-feature

Auth feature implementation.
`
	if err := os.WriteFile(filepath.Join(projectDir, "project.md"), []byte(projectMd), 0644); err != nil {
		t.Fatalf("Failed to write project.md: %v", err)
	}

	// Create task in project folder
	projectTask := `---
id: AUTH-001
title: Implement OAuth
role: backend
priority: high
status: pending
workstream: oauth
---

Implement OAuth flow.
`
	if err := os.WriteFile(filepath.Join(projectDir, "001-oauth.md"), []byte(projectTask), 0644); err != nil {
		t.Fatalf("Failed to write project task: %v", err)
	}

	// Create root task (not in project)
	rootTask := `---
id: ROOT-001
title: Root Task
role: backend
priority: medium
status: pending
---

A root task.
`
	if err := os.WriteFile(filepath.Join(tasksDir, "ROOT-001.md"), []byte(rootTask), 0644); err != nil {
		t.Fatalf("Failed to write root task: %v", err)
	}

	// Scan
	mgr := NewManager(&Config{ProjectRoot: dir})
	tasks, err := mgr.Scan()

	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}

	if len(tasks) != 2 {
		t.Errorf("Scan() returned %d tasks, want 2", len(tasks))
	}

	// Verify project task has project field set
	authTask, err := mgr.Get("AUTH-001")
	if err != nil {
		t.Fatalf("Get(AUTH-001) error: %v", err)
	}
	if authTask.Project != "auth-feature" {
		t.Errorf("AUTH-001 project = %q, want auth-feature", authTask.Project)
	}

	// Verify root task has no project
	rootTaskResult, err := mgr.Get("ROOT-001")
	if err != nil {
		t.Fatalf("Get(ROOT-001) error: %v", err)
	}
	if rootTaskResult.Project != "" {
		t.Errorf("ROOT-001 project = %q, want empty", rootTaskResult.Project)
	}
}

func TestManager_GetByProject(t *testing.T) {
	mgr := &Manager{
		tasks: map[string]*Task{
			"T1": {ID: "T1", Role: "backend", Project: "auth-feature", Priority: PriorityHigh},
			"T2": {ID: "T2", Role: "backend", Project: "auth-feature", Priority: PriorityLow},
			"T3": {ID: "T3", Role: "backend", Project: "api-refactor"},
			"T4": {ID: "T4", Role: "frontend"}, // No project
		},
	}

	t.Run("get project tasks", func(t *testing.T) {
		tasks := mgr.GetByProject("auth-feature")
		if len(tasks) != 2 {
			t.Errorf("GetByProject(auth-feature) returned %d, want 2", len(tasks))
		}
		// Should be sorted by priority
		if tasks[0].ID != "T1" {
			t.Errorf("First task = %s, want T1 (higher priority)", tasks[0].ID)
		}
	})

	t.Run("no matches", func(t *testing.T) {
		tasks := mgr.GetByProject("nonexistent")
		if len(tasks) != 0 {
			t.Errorf("GetByProject(nonexistent) returned %d, want 0", len(tasks))
		}
	})
}

func TestManager_GetProjects(t *testing.T) {
	mgr := &Manager{
		tasks: map[string]*Task{
			"T1": {ID: "T1", Project: "auth-feature"},
			"T2": {ID: "T2", Project: "auth-feature"},
			"T3": {ID: "T3", Project: "api-refactor"},
			"T4": {ID: "T4"}, // No project
		},
	}

	projects := mgr.GetProjects()
	if len(projects) != 2 {
		t.Errorf("GetProjects() returned %d, want 2", len(projects))
	}

	// Should be sorted alphabetically
	if projects[0] != "api-refactor" {
		t.Errorf("First project = %s, want api-refactor", projects[0])
	}
	if projects[1] != "auth-feature" {
		t.Errorf("Second project = %s, want auth-feature", projects[1])
	}
}

func TestManager_GetProjectWorkstreams(t *testing.T) {
	mgr := &Manager{
		tasks: map[string]*Task{
			"T1": {ID: "T1", Role: "backend", Project: "auth", Workstream: "oauth", Priority: PriorityLow},
			"T2": {ID: "T2", Role: "backend", Project: "auth", Workstream: "jwt", Priority: PriorityCritical},
			"T3": {ID: "T3", Role: "backend", Project: "auth", Workstream: "oauth", Priority: PriorityHigh},
			"T4": {ID: "T4", Role: "frontend", Project: "auth", Workstream: "ui"},
		},
	}

	workstreams := mgr.GetProjectWorkstreams("auth", "backend")
	if len(workstreams) != 2 {
		t.Errorf("GetProjectWorkstreams(auth, backend) returned %d, want 2", len(workstreams))
	}

	// Should be sorted by priority (jwt has critical, oauth has high)
	if workstreams[0] != "jwt" {
		t.Errorf("First workstream = %s, want jwt (critical priority)", workstreams[0])
	}
	if workstreams[1] != "oauth" {
		t.Errorf("Second workstream = %s, want oauth", workstreams[1])
	}
}

func TestManager_GetByProjectAndWorkstream(t *testing.T) {
	mgr := &Manager{
		tasks: map[string]*Task{
			"T1": {ID: "T1", Role: "backend", Project: "auth", Workstream: "oauth", Priority: PriorityHigh},
			"T2": {ID: "T2", Role: "backend", Project: "auth", Workstream: "jwt"},
			"T3": {ID: "T3", Role: "backend", Project: "auth", Workstream: "oauth", Priority: PriorityLow},
			"T4": {ID: "T4", Role: "frontend", Project: "auth", Workstream: "oauth"},
		},
	}

	tasks := mgr.GetByProjectAndWorkstream("auth", "backend", "oauth")
	if len(tasks) != 2 {
		t.Errorf("GetByProjectAndWorkstream() returned %d, want 2", len(tasks))
	}

	// Should be sorted by priority
	if tasks[0].ID != "T1" {
		t.Errorf("First task = %s, want T1 (higher priority)", tasks[0].ID)
	}
}

func TestManager_Stats_IncludesWorkstreams(t *testing.T) {
	mgr := &Manager{
		tasks: map[string]*Task{
			"T1": {ID: "T1", Status: StatusPending, Role: "backend", Workstream: "ws1"},
			"T2": {ID: "T2", Status: StatusPending, Role: "backend", Workstream: "ws1"},
			"T3": {ID: "T3", Status: StatusComplete, Role: "backend", Workstream: "ws2"},
		},
	}

	stats := mgr.Stats()

	if stats.ByWorkstream["ws1"] != 2 {
		t.Errorf("ByWorkstream[ws1] = %d, want 2", stats.ByWorkstream["ws1"])
	}
	if stats.ByWorkstream["ws2"] != 1 {
		t.Errorf("ByWorkstream[ws2] = %d, want 1", stats.ByWorkstream["ws2"])
	}
}
