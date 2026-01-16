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
	defer func() { _ = os.Chdir(oldWd) }()
	_ = os.Chdir(tempDir)

	// Run init with project name (now required)
	err := runProjectInit(nil, []string{"test-project"})
	if err != nil {
		t.Fatalf("runProjectInit() error: %v", err)
	}

	// Check task directory was created (defaults to "tasks")
	taskDir := filepath.Join(tempDir, "tasks")
	if _, statErr := os.Stat(taskDir); os.IsNotExist(statErr) {
		t.Error("task directory was not created")
	}

	// Check top-level README.md was created
	tasksReadme := filepath.Join(taskDir, "README.md")
	if _, statErr := os.Stat(tasksReadme); os.IsNotExist(statErr) {
		t.Error("tasks README.md was not created")
	}

	// Check project directory was created
	projectDir := filepath.Join(taskDir, "test-project")
	if _, statErr := os.Stat(projectDir); os.IsNotExist(statErr) {
		t.Error("project directory was not created")
	}

	// Check project README.md was created
	projectReadme := filepath.Join(projectDir, "README.md")
	if _, statErr := os.Stat(projectReadme); os.IsNotExist(statErr) {
		t.Error("project README.md was not created")
	}

	// Check example task was created
	examplePath := filepath.Join(projectDir, "001-main-example-task.md")
	if _, statErr := os.Stat(examplePath); os.IsNotExist(statErr) {
		t.Error("example task was not created")
	}

	// Verify example task can be parsed
	content, err := os.ReadFile(examplePath) //nolint:gosec // G304: Test file path is from t.TempDir()
	if err != nil {
		t.Fatalf("read example task: %v", err)
	}

	if !strings.Contains(string(content), "id: test-project-001") {
		t.Error("example task missing id")
	}
	if !strings.Contains(string(content), "workstream: main") {
		t.Error("example task missing workstream")
	}
}

func TestProjectInitIdempotent(t *testing.T) {
	// Create temp directory
	tempDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	_ = os.Chdir(tempDir)

	// Run init twice with same project name
	if err := runProjectInit(nil, []string{"test-project"}); err != nil {
		t.Fatalf("first runProjectInit() error: %v", err)
	}

	if err := runProjectInit(nil, []string{"test-project"}); err != nil {
		t.Fatalf("second runProjectInit() error: %v", err)
	}

	// Should still have exactly one example task (plus README.md) in project dir
	projectDir := filepath.Join(tempDir, "tasks", "test-project")
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		t.Fatalf("read project dir: %v", err)
	}

	// Expect 2 files: README.md and 001-main-example-task.md
	if len(entries) != 2 {
		t.Errorf("expected 2 files (README.md + example task), got %d", len(entries))
	}
}

func TestProjectInitWithDifferentName(t *testing.T) {
	// Create temp directory
	tempDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	_ = os.Chdir(tempDir)

	// Run init with a different project name
	err := runProjectInit(nil, []string{"auth-feature"})
	if err != nil {
		t.Fatalf("runProjectInit(auth-feature) error: %v", err)
	}

	// Check tasks directory was created
	taskDir := filepath.Join(tempDir, "tasks")
	if _, statErr := os.Stat(taskDir); os.IsNotExist(statErr) {
		t.Error("tasks directory was not created")
	}

	// Check project folder was created
	projectDir := filepath.Join(taskDir, "auth-feature")
	if _, statErr := os.Stat(projectDir); os.IsNotExist(statErr) {
		t.Error("project folder was not created")
	}

	// Check README.md was created in project folder
	readmePath := filepath.Join(projectDir, "README.md")
	if _, statErr := os.Stat(readmePath); os.IsNotExist(statErr) {
		t.Error("README.md was not created")
	}

	// Check example task was created in project folder
	examplePath := filepath.Join(projectDir, "001-main-example-task.md")
	if _, statErr := os.Stat(examplePath); os.IsNotExist(statErr) {
		t.Error("example task was not created")
	}

	// Verify example task can be parsed and has correct project ID prefix
	content, err := os.ReadFile(examplePath) //nolint:gosec // G304: Test file path is from t.TempDir()
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

func TestProjectInitMultipleProjects(t *testing.T) {
	// Create temp directory
	tempDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	_ = os.Chdir(tempDir)

	// Create two different projects
	if err := runProjectInit(nil, []string{"project-one"}); err != nil {
		t.Fatalf("runProjectInit(project-one) error: %v", err)
	}

	if err := runProjectInit(nil, []string{"project-two"}); err != nil {
		t.Fatalf("runProjectInit(project-two) error: %v", err)
	}

	// Check both project directories exist
	taskDir := filepath.Join(tempDir, "tasks")
	entries, err := os.ReadDir(taskDir)
	if err != nil {
		t.Fatalf("read tasks dir: %v", err)
	}

	// Expect 3 entries: README.md, project-one, project-two
	if len(entries) != 3 {
		t.Errorf("expected 3 entries (README.md + 2 projects), got %d", len(entries))
	}

	// Verify each project has README.md and example task
	for _, projectName := range []string{"project-one", "project-two"} {
		projectDir := filepath.Join(taskDir, projectName)
		projectEntries, err := os.ReadDir(projectDir)
		if err != nil {
			t.Fatalf("read %s dir: %v", projectName, err)
		}
		if len(projectEntries) != 2 {
			t.Errorf("%s: expected 2 files (README.md + example task), got %d", projectName, len(projectEntries))
		}
	}
}

func TestMockTaskManagerScan(t *testing.T) {
	// Create temp directory with task files
	tempDir := t.TempDir()
	taskDir := filepath.Join(tempDir, "tasks")
	if err := os.MkdirAll(taskDir, 0750); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}

	// Create test task
	taskContent := `---
id: TEST-001
title: Test Task
workstream: backend
priority: high
status: pending
---

Test content
`
	taskPath := filepath.Join(taskDir, "TEST-001-test.md")
	if err := os.WriteFile(taskPath, []byte(taskContent), 0600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

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

func TestMockTaskManagerGetByWorkstream(t *testing.T) {
	// Create temp directory with task files
	tempDir := t.TempDir()
	taskDir := filepath.Join(tempDir, "tasks")
	if err := os.MkdirAll(taskDir, 0750); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}

	// Create api workstream task
	apiTask := `---
id: API-001
title: API Task
workstream: api
status: pending
---

API content
`
	if err := os.WriteFile(filepath.Join(taskDir, "API-001.md"), []byte(apiTask), 0600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	// Create web workstream task
	webTask := `---
id: WEB-001
title: Web Task
workstream: web
status: pending
---

Web content
`
	if err := os.WriteFile(filepath.Join(taskDir, "WEB-001.md"), []byte(webTask), 0600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	// Scan and filter by workstream
	mgr := newMockTaskManager(taskDir)
	_, _ = mgr.Scan()

	apiTasks := mgr.GetByWorkstream("api")
	if len(apiTasks) != 1 {
		t.Errorf("GetByWorkstream(api) returned %d tasks, want 1", len(apiTasks))
	}

	webTasks := mgr.GetByWorkstream("web")
	if len(webTasks) != 1 {
		t.Errorf("GetByWorkstream(web) returned %d tasks, want 1", len(webTasks))
	}

	otherTasks := mgr.GetByWorkstream("other")
	if len(otherTasks) != 0 {
		t.Errorf("GetByWorkstream(other) returned %d tasks, want 0", len(otherTasks))
	}
}

func TestMockTaskManagerIsBlocked(t *testing.T) {
	// Create temp directory with task files
	tempDir := t.TempDir()
	taskDir := filepath.Join(tempDir, "tasks")
	if err := os.MkdirAll(taskDir, 0750); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}

	// Create T1 (complete)
	t1 := `---
id: T1
title: Task 1
workstream: backend
status: complete
---

Done
`
	if err := os.WriteFile(filepath.Join(taskDir, "T1.md"), []byte(t1), 0600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	// Create T2 (pending)
	t2 := `---
id: T2
title: Task 2
workstream: backend
status: pending
---

Pending
`
	if err := os.WriteFile(filepath.Join(taskDir, "T2.md"), []byte(t2), 0600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	// Create T3 (depends on T1 - not blocked)
	t3 := `---
id: T3
title: Task 3
workstream: backend
status: pending
depends_on:
  - T1
---

Depends on T1
`
	if err := os.WriteFile(filepath.Join(taskDir, "T3.md"), []byte(t3), 0600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	// Create T4 (depends on T2 - blocked)
	t4 := `---
id: T4
title: Task 4
workstream: backend
status: pending
depends_on:
  - T2
---

Depends on T2
`
	if err := os.WriteFile(filepath.Join(taskDir, "T4.md"), []byte(t4), 0600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	// Scan
	mgr := newMockTaskManager(taskDir)
	_, _ = mgr.Scan()

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
	if err := os.MkdirAll(taskDir, 0750); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}

	taskContent := `---
id: TEST-001
title: Test Task
workstream: backend
status: pending
---

Test content
`
	taskPath := filepath.Join(taskDir, "TEST-001.md")
	if err := os.WriteFile(taskPath, []byte(taskContent), 0600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	// Scan
	mgr := newMockTaskManager(taskDir)
	_, _ = mgr.Scan()

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
	_, _ = mgr2.Scan()
	tsk2, _ := mgr2.Get("TEST-001")
	if tsk2.Status != task.StatusInProgress {
		t.Errorf("Persisted status = %v, want in_progress", tsk2.Status)
	}
}

func TestMockTaskManagerAssign(t *testing.T) {
	// Create temp directory with task file
	tempDir := t.TempDir()
	taskDir := filepath.Join(tempDir, "tasks")
	if err := os.MkdirAll(taskDir, 0750); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}

	taskContent := `---
id: TEST-001
title: Test Task
workstream: backend
status: pending
---

Test content
`
	taskPath := filepath.Join(taskDir, "TEST-001.md")
	if err := os.WriteFile(taskPath, []byte(taskContent), 0600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	// Scan
	mgr := newMockTaskManager(taskDir)
	_, _ = mgr.Scan()

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
