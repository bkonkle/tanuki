package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bkonkle/tanuki/internal/task"
)

func TestProjectInit(t *testing.T) {
	// Create temp directory
	tempDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tempDir)

	// Run init
	err := runProjectInit(nil, nil)
	if err != nil {
		t.Fatalf("runProjectInit() error: %v", err)
	}

	// Check task directory was created (defaults to "tasks")
	taskDir := filepath.Join(tempDir, "tasks")
	if _, err := os.Stat(taskDir); os.IsNotExist(err) {
		t.Error("task directory was not created")
	}

	// Check example task was created
	examplePath := filepath.Join(taskDir, "TASK-001-example.md")
	if _, err := os.Stat(examplePath); os.IsNotExist(err) {
		t.Error("example task was not created")
	}

	// Verify example task can be parsed
	content, err := os.ReadFile(examplePath)
	if err != nil {
		t.Fatalf("read example task: %v", err)
	}

	if !strings.Contains(string(content), "id: TASK-001") {
		t.Error("example task missing id")
	}
	if !strings.Contains(string(content), "role: backend") {
		t.Error("example task missing role")
	}
}

func TestProjectInitIdempotent(t *testing.T) {
	// Create temp directory
	tempDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tempDir)

	// Run init twice
	err := runProjectInit(nil, nil)
	if err != nil {
		t.Fatalf("first runProjectInit() error: %v", err)
	}

	err = runProjectInit(nil, nil)
	if err != nil {
		t.Fatalf("second runProjectInit() error: %v", err)
	}

	// Should still have exactly one example task (plus project.md)
	taskDir := filepath.Join(tempDir, "tasks")
	entries, err := os.ReadDir(taskDir)
	if err != nil {
		t.Fatalf("read task dir: %v", err)
	}

	// Expect 2 files: project.md and TASK-001-example.md
	if len(entries) != 2 {
		t.Errorf("expected 2 files (project.md + example task), got %d", len(entries))
	}
}

func TestProjectInitWithName(t *testing.T) {
	// Create temp directory
	tempDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tempDir)

	// Run init with a project name
	err := runProjectInit(nil, []string{"auth-feature"})
	if err != nil {
		t.Fatalf("runProjectInit(auth-feature) error: %v", err)
	}

	// Check tasks directory was created
	taskDir := filepath.Join(tempDir, "tasks")
	if _, err := os.Stat(taskDir); os.IsNotExist(err) {
		t.Error("tasks directory was not created")
	}

	// Check project folder was created
	projectDir := filepath.Join(taskDir, "auth-feature")
	if _, err := os.Stat(projectDir); os.IsNotExist(err) {
		t.Error("project folder was not created")
	}

	// Check project.md was created in project folder
	projectMdPath := filepath.Join(projectDir, "project.md")
	if _, err := os.Stat(projectMdPath); os.IsNotExist(err) {
		t.Error("project.md was not created")
	}

	// Check example task was created in project folder
	examplePath := filepath.Join(projectDir, "001-example.md")
	if _, err := os.Stat(examplePath); os.IsNotExist(err) {
		t.Error("example task was not created")
	}

	// Verify example task can be parsed and has correct project ID prefix
	content, err := os.ReadFile(examplePath)
	if err != nil {
		t.Fatalf("read example task: %v", err)
	}

	if !strings.Contains(string(content), "id: auth-feature-001") {
		t.Error("example task should have project-prefixed id")
	}
	if !strings.Contains(string(content), "workstream: main") {
		t.Error("example task should have workstream: main")
	}
}

func TestProjectInitWithNameIdempotent(t *testing.T) {
	// Create temp directory
	tempDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tempDir)

	// Run init with project name twice
	err := runProjectInit(nil, []string{"my-project"})
	if err != nil {
		t.Fatalf("first runProjectInit(my-project) error: %v", err)
	}

	err = runProjectInit(nil, []string{"my-project"})
	if err != nil {
		t.Fatalf("second runProjectInit(my-project) error: %v", err)
	}

	// Should still have exactly one example task (plus project.md)
	projectDir := filepath.Join(tempDir, "tasks", "my-project")
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		t.Fatalf("read project dir: %v", err)
	}

	// Expect 2 files: project.md and 001-example.md
	if len(entries) != 2 {
		t.Errorf("expected 2 files (project.md + example task), got %d", len(entries))
	}
}

func TestMockTaskManagerScan(t *testing.T) {
	// Create temp directory with task files
	tempDir := t.TempDir()
	taskDir := filepath.Join(tempDir, "tasks")
	os.MkdirAll(taskDir, 0755)

	// Create test task
	taskContent := `---
id: TEST-001
title: Test Task
role: backend
priority: high
status: pending
---

Test content
`
	taskPath := filepath.Join(taskDir, "TEST-001-test.md")
	os.WriteFile(taskPath, []byte(taskContent), 0644)

	// Scan
	mgr := newMockTaskManager(taskDir)
	tasks, err := mgr.Scan()
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}

	if len(tasks) != 1 {
		t.Errorf("Scan() returned %d tasks, want 1", len(tasks))
	}

	if tasks[0].ID != "TEST-001" {
		t.Errorf("Task ID = %q, want TEST-001", tasks[0].ID)
	}
}

func TestMockTaskManagerGetByRole(t *testing.T) {
	// Create temp directory with task files
	tempDir := t.TempDir()
	taskDir := filepath.Join(tempDir, "tasks")
	os.MkdirAll(taskDir, 0755)

	// Create backend task
	backendTask := `---
id: BE-001
title: Backend Task
role: backend
status: pending
---

Backend content
`
	os.WriteFile(filepath.Join(taskDir, "BE-001.md"), []byte(backendTask), 0644)

	// Create frontend task
	frontendTask := `---
id: FE-001
title: Frontend Task
role: frontend
status: pending
---

Frontend content
`
	os.WriteFile(filepath.Join(taskDir, "FE-001.md"), []byte(frontendTask), 0644)

	// Scan and filter by role
	mgr := newMockTaskManager(taskDir)
	mgr.Scan()

	backendTasks := mgr.GetByRole("backend")
	if len(backendTasks) != 1 {
		t.Errorf("GetByRole(backend) returned %d tasks, want 1", len(backendTasks))
	}

	frontendTasks := mgr.GetByRole("frontend")
	if len(frontendTasks) != 1 {
		t.Errorf("GetByRole(frontend) returned %d tasks, want 1", len(frontendTasks))
	}

	qaTasks := mgr.GetByRole("qa")
	if len(qaTasks) != 0 {
		t.Errorf("GetByRole(qa) returned %d tasks, want 0", len(qaTasks))
	}
}

func TestMockTaskManagerIsBlocked(t *testing.T) {
	// Create temp directory with task files
	tempDir := t.TempDir()
	taskDir := filepath.Join(tempDir, "tasks")
	os.MkdirAll(taskDir, 0755)

	// Create T1 (complete)
	t1 := `---
id: T1
title: Task 1
role: backend
status: complete
---

Done
`
	os.WriteFile(filepath.Join(taskDir, "T1.md"), []byte(t1), 0644)

	// Create T2 (pending)
	t2 := `---
id: T2
title: Task 2
role: backend
status: pending
---

Pending
`
	os.WriteFile(filepath.Join(taskDir, "T2.md"), []byte(t2), 0644)

	// Create T3 (depends on T1 - not blocked)
	t3 := `---
id: T3
title: Task 3
role: backend
status: pending
depends_on:
  - T1
---

Depends on T1
`
	os.WriteFile(filepath.Join(taskDir, "T3.md"), []byte(t3), 0644)

	// Create T4 (depends on T2 - blocked)
	t4 := `---
id: T4
title: Task 4
role: backend
status: pending
depends_on:
  - T2
---

Depends on T2
`
	os.WriteFile(filepath.Join(taskDir, "T4.md"), []byte(t4), 0644)

	// Scan
	mgr := newMockTaskManager(taskDir)
	mgr.Scan()

	// Test blocked status
	tests := []struct {
		id      string
		blocked bool
	}{
		{"T1", false},
		{"T2", false},
		{"T3", false}, // T1 is complete
		{"T4", true},  // T2 is pending
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
}

func TestMockTaskManagerUpdateStatus(t *testing.T) {
	// Create temp directory with task file
	tempDir := t.TempDir()
	taskDir := filepath.Join(tempDir, "tasks")
	os.MkdirAll(taskDir, 0755)

	taskContent := `---
id: TEST-001
title: Test Task
role: backend
status: pending
---

Test content
`
	taskPath := filepath.Join(taskDir, "TEST-001.md")
	os.WriteFile(taskPath, []byte(taskContent), 0644)

	// Scan
	mgr := newMockTaskManager(taskDir)
	mgr.Scan()

	// Update status
	err := mgr.UpdateStatus("TEST-001", task.StatusInProgress)
	if err != nil {
		t.Fatalf("UpdateStatus() error: %v", err)
	}

	// Verify in memory
	tsk, _ := mgr.Get("TEST-001")
	if tsk.Status != task.StatusInProgress {
		t.Errorf("Status = %v, want in_progress", tsk.Status)
	}

	// Verify on disk (re-scan)
	mgr2 := newMockTaskManager(taskDir)
	mgr2.Scan()
	tsk2, _ := mgr2.Get("TEST-001")
	if tsk2.Status != task.StatusInProgress {
		t.Errorf("Persisted status = %v, want in_progress", tsk2.Status)
	}
}

func TestMockTaskManagerAssign(t *testing.T) {
	// Create temp directory with task file
	tempDir := t.TempDir()
	taskDir := filepath.Join(tempDir, "tasks")
	os.MkdirAll(taskDir, 0755)

	taskContent := `---
id: TEST-001
title: Test Task
role: backend
status: pending
---

Test content
`
	taskPath := filepath.Join(taskDir, "TEST-001.md")
	os.WriteFile(taskPath, []byte(taskContent), 0644)

	// Scan
	mgr := newMockTaskManager(taskDir)
	mgr.Scan()

	// Assign
	err := mgr.Assign("TEST-001", "backend-agent")
	if err != nil {
		t.Fatalf("Assign() error: %v", err)
	}

	// Verify assignment
	tsk, _ := mgr.Get("TEST-001")
	if tsk.AssignedTo != "backend-agent" {
		t.Errorf("AssignedTo = %q, want backend-agent", tsk.AssignedTo)
	}
	if tsk.Status != task.StatusAssigned {
		t.Errorf("Status = %v, want assigned", tsk.Status)
	}

	// Unassign
	err = mgr.Unassign("TEST-001")
	if err != nil {
		t.Fatalf("Unassign() error: %v", err)
	}

	// Verify unassignment
	tsk, _ = mgr.Get("TEST-001")
	if tsk.AssignedTo != "" {
		t.Errorf("AssignedTo = %q, want empty", tsk.AssignedTo)
	}
	if tsk.Status != task.StatusPending {
		t.Errorf("Status = %v, want pending", tsk.Status)
	}
}

func TestSortTasks(t *testing.T) {
	tasks := []*task.Task{
		{ID: "LOW", Priority: task.PriorityLow, Status: task.StatusPending},
		{ID: "CRITICAL", Priority: task.PriorityCritical, Status: task.StatusPending},
		{ID: "MEDIUM", Priority: task.PriorityMedium, Status: task.StatusPending},
		{ID: "HIGH", Priority: task.PriorityHigh, Status: task.StatusPending},
	}

	sortTasks(tasks)

	expected := []string{"CRITICAL", "HIGH", "MEDIUM", "LOW"}
	for i, e := range expected {
		if tasks[i].ID != e {
			t.Errorf("sortTasks()[%d] = %s, want %s", i, tasks[i].ID, e)
		}
	}
}

func TestSortTasksByStatus(t *testing.T) {
	tasks := []*task.Task{
		{ID: "PENDING", Priority: task.PriorityMedium, Status: task.StatusPending},
		{ID: "IN_PROGRESS", Priority: task.PriorityMedium, Status: task.StatusInProgress},
		{ID: "COMPLETE", Priority: task.PriorityMedium, Status: task.StatusComplete},
		{ID: "ASSIGNED", Priority: task.PriorityMedium, Status: task.StatusAssigned},
	}

	sortTasks(tasks)

	// in_progress should come first, then assigned, then pending, then complete
	expected := []string{"IN_PROGRESS", "ASSIGNED", "PENDING", "COMPLETE"}
	for i, e := range expected {
		if tasks[i].ID != e {
			t.Errorf("sortTasks()[%d] = %s, want %s", i, tasks[i].ID, e)
		}
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		max      int
		expected string
	}{
		{"hello", 10, "hello"},
		{"hello world", 10, "hello w..."},
		{"hi", 5, "hi"},
		{"exactly10!", 10, "exactly10!"},
	}

	for _, tt := range tests {
		result := truncate(tt.input, tt.max)
		if result != tt.expected {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.max, result, tt.expected)
		}
	}
}

func TestBuildTaskPrompt(t *testing.T) {
	tsk := &task.Task{
		Title:   "Test Task",
		Content: "Do the thing.",
		Completion: &task.CompletionConfig{
			Verify: "npm test",
			Signal: "DONE",
		},
	}

	prompt := buildTaskPrompt(tsk)

	if !strings.Contains(prompt, "# Task: Test Task") {
		t.Error("prompt missing title")
	}
	if !strings.Contains(prompt, "Do the thing.") {
		t.Error("prompt missing content")
	}
	if !strings.Contains(prompt, "npm test") {
		t.Error("prompt missing verify command")
	}
	if !strings.Contains(prompt, "DONE") {
		t.Error("prompt missing signal")
	}
}
