---
id: TANK-033
title: Task Manager Implementation
status: todo
priority: high
estimate: M
depends_on: [TANK-030]
workstream: A
phase: 3
---

# Task Manager Implementation

## Summary

Implement the TaskManager that handles scanning, loading, querying, and updating tasks. This is the core data access layer for the task system, built on the schema defined in TANK-030.

## Acceptance Criteria

- [ ] Scan directory for task files
- [ ] Load and cache tasks in memory
- [ ] Get task by ID
- [ ] Get tasks by role
- [ ] Get tasks by status
- [ ] Update task status (writes back to file)
- [ ] Assign/unassign tasks to agents
- [ ] Check if task is blocked (dependencies not met)
- [ ] Handle concurrent access safely
- [ ] Unit tests with 80%+ coverage

## Technical Details

### TaskManager Interface

```go
// internal/task/manager.go
package task

import (
    "fmt"
    "os"
    "path/filepath"
    "sort"
    "sync"
)

type Manager struct {
    config    *Config
    tasksDir  string
    tasks     map[string]*Task
    mu        sync.RWMutex
}

type Config struct {
    ProjectRoot string
}

func NewManager(cfg *Config) *Manager {
    return &Manager{
        config:   cfg,
        tasksDir: filepath.Join(cfg.ProjectRoot, ".tanuki", "tasks"),
        tasks:    make(map[string]*Task),
    }
}

// Scan loads all task files from .tanuki/tasks/
func (m *Manager) Scan() ([]*Task, error) {
    m.mu.Lock()
    defer m.mu.Unlock()

    // Clear existing cache
    m.tasks = make(map[string]*Task)

    // Check if directory exists
    if _, err := os.Stat(m.tasksDir); os.IsNotExist(err) {
        return nil, nil // No tasks directory - not an error
    }

    entries, err := os.ReadDir(m.tasksDir)
    if err != nil {
        return nil, fmt.Errorf("read tasks directory: %w", err)
    }

    var tasks []*Task
    var errors []error

    for _, entry := range entries {
        if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
            continue
        }

        path := filepath.Join(m.tasksDir, entry.Name())
        task, err := ParseFile(path)
        if err != nil {
            // Log warning but continue scanning
            errors = append(errors, fmt.Errorf("parse %s: %w", entry.Name(), err))
            continue
        }

        m.tasks[task.ID] = task
        tasks = append(tasks, task)
    }

    // Log any errors encountered
    for _, err := range errors {
        fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
    }

    return tasks, nil
}

// Get returns a task by ID
func (m *Manager) Get(id string) (*Task, error) {
    m.mu.RLock()
    defer m.mu.RUnlock()

    task, ok := m.tasks[id]
    if !ok {
        return nil, fmt.Errorf("task %q not found", id)
    }

    return task, nil
}

// List returns all tasks, optionally sorted
func (m *Manager) List(opts ...ListOption) []*Task {
    m.mu.RLock()
    defer m.mu.RUnlock()

    tasks := make([]*Task, 0, len(m.tasks))
    for _, t := range m.tasks {
        tasks = append(tasks, t)
    }

    // Apply options
    o := &listOptions{}
    for _, opt := range opts {
        opt(o)
    }

    if o.sortByPriority {
        sort.Slice(tasks, func(i, j int) bool {
            return priorityOrder(tasks[i].Priority) < priorityOrder(tasks[j].Priority)
        })
    }

    return tasks
}

type listOptions struct {
    sortByPriority bool
}

type ListOption func(*listOptions)

func SortByPriority() ListOption {
    return func(o *listOptions) {
        o.sortByPriority = true
    }
}

func priorityOrder(p Priority) int {
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
        return 4
    }
}
```

### Query Methods

```go
// GetByRole returns tasks for a specific role
func (m *Manager) GetByRole(role string) []*Task {
    m.mu.RLock()
    defer m.mu.RUnlock()

    var tasks []*Task
    for _, t := range m.tasks {
        if t.Role == role {
            tasks = append(tasks, t)
        }
    }

    return tasks
}

// GetByStatus returns tasks with a specific status
func (m *Manager) GetByStatus(status Status) []*Task {
    m.mu.RLock()
    defer m.mu.RUnlock()

    var tasks []*Task
    for _, t := range m.tasks {
        if t.Status == status {
            tasks = append(tasks, t)
        }
    }

    return tasks
}

// GetPending returns all pending tasks, sorted by priority
func (m *Manager) GetPending() []*Task {
    tasks := m.GetByStatus(StatusPending)
    sort.Slice(tasks, func(i, j int) bool {
        return priorityOrder(tasks[i].Priority) < priorityOrder(tasks[j].Priority)
    })
    return tasks
}

// GetNextAvailable returns the highest priority pending task for a role
func (m *Manager) GetNextAvailable(role string) (*Task, error) {
    m.mu.RLock()
    defer m.mu.RUnlock()

    var candidates []*Task
    for _, t := range m.tasks {
        if t.Role == role && t.Status == StatusPending {
            candidates = append(candidates, t)
        }
    }

    if len(candidates) == 0 {
        return nil, fmt.Errorf("no pending tasks for role %q", role)
    }

    // Sort by priority
    sort.Slice(candidates, func(i, j int) bool {
        return priorityOrder(candidates[i].Priority) < priorityOrder(candidates[j].Priority)
    })

    // Return first non-blocked task
    for _, t := range candidates {
        if blocked, _ := m.isBlockedInternal(t.ID); !blocked {
            return t, nil
        }
    }

    return nil, fmt.Errorf("all pending tasks for role %q are blocked", role)
}
```

### Status Updates

```go
// UpdateStatus changes task status and persists to file
func (m *Manager) UpdateStatus(id string, status Status) error {
    m.mu.Lock()
    defer m.mu.Unlock()

    task, ok := m.tasks[id]
    if !ok {
        return fmt.Errorf("task %q not found", id)
    }

    task.Status = status

    // Write back to file
    if err := WriteFile(task); err != nil {
        return fmt.Errorf("write task file: %w", err)
    }

    return nil
}

// Assign assigns a task to an agent
func (m *Manager) Assign(id string, agentName string) error {
    m.mu.Lock()
    defer m.mu.Unlock()

    task, ok := m.tasks[id]
    if !ok {
        return fmt.Errorf("task %q not found", id)
    }

    if task.Status != StatusPending && task.Status != StatusBlocked {
        return fmt.Errorf("task %q is not available (status: %s)", id, task.Status)
    }

    task.AssignedTo = agentName
    task.Status = StatusAssigned

    if err := WriteFile(task); err != nil {
        return fmt.Errorf("write task file: %w", err)
    }

    return nil
}

// Unassign removes agent assignment from a task
func (m *Manager) Unassign(id string) error {
    m.mu.Lock()
    defer m.mu.Unlock()

    task, ok := m.tasks[id]
    if !ok {
        return fmt.Errorf("task %q not found", id)
    }

    task.AssignedTo = ""

    // Don't change status if complete/failed
    if task.Status == StatusAssigned || task.Status == StatusInProgress {
        task.Status = StatusPending
    }

    if err := WriteFile(task); err != nil {
        return fmt.Errorf("write task file: %w", err)
    }

    return nil
}
```

### Dependency Checking

```go
// IsBlocked checks if a task's dependencies are all complete
func (m *Manager) IsBlocked(id string) (bool, error) {
    m.mu.RLock()
    defer m.mu.RUnlock()

    return m.isBlockedInternal(id)
}

func (m *Manager) isBlockedInternal(id string) (bool, error) {
    task, ok := m.tasks[id]
    if !ok {
        return false, fmt.Errorf("task %q not found", id)
    }

    if len(task.DependsOn) == 0 {
        return false, nil
    }

    for _, depID := range task.DependsOn {
        dep, ok := m.tasks[depID]
        if !ok {
            // Dependency not found - treat as blocked
            return true, nil
        }
        if dep.Status != StatusComplete {
            return true, nil
        }
    }

    return false, nil
}

// GetBlockingTasks returns the IDs of incomplete dependencies
func (m *Manager) GetBlockingTasks(id string) ([]string, error) {
    m.mu.RLock()
    defer m.mu.RUnlock()

    task, ok := m.tasks[id]
    if !ok {
        return nil, fmt.Errorf("task %q not found", id)
    }

    var blocking []string
    for _, depID := range task.DependsOn {
        dep, ok := m.tasks[depID]
        if !ok || dep.Status != StatusComplete {
            blocking = append(blocking, depID)
        }
    }

    return blocking, nil
}

// UpdateBlockedStatus checks all tasks and updates blocked status
func (m *Manager) UpdateBlockedStatus() error {
    m.mu.Lock()
    defer m.mu.Unlock()

    for _, task := range m.tasks {
        if task.Status == StatusPending || task.Status == StatusBlocked {
            blocked, _ := m.isBlockedInternal(task.ID)
            if blocked && task.Status != StatusBlocked {
                task.Status = StatusBlocked
                WriteFile(task)
            } else if !blocked && task.Status == StatusBlocked {
                task.Status = StatusPending
                WriteFile(task)
            }
        }
    }

    return nil
}
```

### Statistics

```go
// Stats returns task statistics
func (m *Manager) Stats() *TaskStats {
    m.mu.RLock()
    defer m.mu.RUnlock()

    stats := &TaskStats{
        ByStatus:   make(map[Status]int),
        ByRole:     make(map[string]int),
        ByPriority: make(map[Priority]int),
    }

    for _, t := range m.tasks {
        stats.Total++
        stats.ByStatus[t.Status]++
        stats.ByRole[t.Role]++
        stats.ByPriority[t.Priority]++
    }

    return stats
}

type TaskStats struct {
    Total      int
    ByStatus   map[Status]int
    ByRole     map[string]int
    ByPriority map[Priority]int
}
```

### Directory Structure

```
internal/
└── task/
    ├── types.go          # Task, Status, Priority (from TANK-030)
    ├── parser.go         # ParseFile, Parse, Validate (from TANK-030)
    ├── serialize.go      # WriteFile (from TANK-030)
    ├── manager.go        # TaskManager implementation
    ├── manager_test.go   # Tests
    └── stats.go          # TaskStats
```

## Testing

### Unit Tests

```go
func TestManager_Scan(t *testing.T) {
    // Setup temp directory with task files
    dir := t.TempDir()
    tasksDir := filepath.Join(dir, ".tanuki", "tasks")
    os.MkdirAll(tasksDir, 0755)

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
    os.WriteFile(filepath.Join(tasksDir, "TASK-001-test.md"), []byte(taskContent), 0644)

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
}

func TestManager_IsBlocked(t *testing.T) {
    mgr := &Manager{
        tasks: map[string]*Task{
            "T1": {ID: "T1", Status: StatusComplete},
            "T2": {ID: "T2", Status: StatusPending},
            "T3": {ID: "T3", DependsOn: []string{"T1"}},           // Not blocked
            "T4": {ID: "T4", DependsOn: []string{"T2"}},           // Blocked
            "T5": {ID: "T5", DependsOn: []string{"T1", "T2"}},     // Blocked
        },
    }

    tests := []struct {
        id      string
        blocked bool
    }{
        {"T1", false},
        {"T2", false},
        {"T3", false}, // T1 is complete
        {"T4", true},  // T2 not complete
        {"T5", true},  // T2 not complete
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

func TestManager_UpdateStatus(t *testing.T) {
    dir := t.TempDir()
    tasksDir := filepath.Join(dir, ".tanuki", "tasks")
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
        }()
    }
    wg.Wait()
}
```

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Tasks directory doesn't exist | Return empty list (not error) |
| Invalid task file | Log warning, skip file, continue |
| Task not found | Return error with ID |
| Write fails | Return error (don't update in-memory) |
| Concurrent access | Use mutex for thread safety |

## Out of Scope

- Task file watching (live reload)
- Task creation API
- Remote task sources
- Task archiving

## Notes

The TaskManager is the central point for all task operations. It maintains an in-memory cache for fast reads but always writes through to disk for persistence.

The mutex strategy uses RWMutex for concurrent reads (Get, List, GetByRole) while serializing writes (UpdateStatus, Assign).
