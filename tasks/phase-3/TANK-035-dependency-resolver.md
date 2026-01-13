---
id: TANK-035
title: Dependency Resolver
status: todo
priority: high
estimate: M
depends_on: [TANK-033]
workstream: A
phase: 3
---

# Dependency Resolver

## Summary

Implement dependency resolution for tasks, including topological sorting and cycle detection. This ensures tasks are executed in the correct order based on their `depends_on` relationships.

## Acceptance Criteria

- [ ] Topological sort of tasks by dependencies
- [ ] Cycle detection with clear error messages
- [ ] Get ready tasks (all dependencies satisfied)
- [ ] Get blocking tasks for a given task
- [ ] Dependency graph visualization (for debugging)
- [ ] Handle missing dependencies gracefully
- [ ] Unit tests with 80%+ coverage

## Technical Details

### Dependency Resolver Interface

```go
// internal/task/resolver.go
package task

import (
    "fmt"
    "strings"
)

// Resolver handles dependency resolution for tasks
type Resolver struct {
    tasks map[string]*Task
}

// NewResolver creates a resolver for a set of tasks
func NewResolver(tasks []*Task) *Resolver {
    taskMap := make(map[string]*Task, len(tasks))
    for _, t := range tasks {
        taskMap[t.ID] = t
    }
    return &Resolver{tasks: taskMap}
}

// GetReady returns tasks that are ready to execute (all deps satisfied)
func (r *Resolver) GetReady() []*Task {
    var ready []*Task

    for _, t := range r.tasks {
        if t.Status != StatusPending {
            continue
        }

        if r.isReady(t) {
            ready = append(ready, t)
        }
    }

    return ready
}

// isReady checks if all dependencies are complete
func (r *Resolver) isReady(t *Task) bool {
    for _, depID := range t.DependsOn {
        dep, ok := r.tasks[depID]
        if !ok {
            // Missing dependency - treat as not ready
            return false
        }
        if dep.Status != StatusComplete {
            return false
        }
    }
    return true
}

// GetBlocking returns incomplete dependencies for a task
func (r *Resolver) GetBlocking(taskID string) ([]string, error) {
    t, ok := r.tasks[taskID]
    if !ok {
        return nil, fmt.Errorf("task %q not found", taskID)
    }

    var blocking []string
    for _, depID := range t.DependsOn {
        dep, ok := r.tasks[depID]
        if !ok {
            blocking = append(blocking, fmt.Sprintf("%s (not found)", depID))
            continue
        }
        if dep.Status != StatusComplete {
            blocking = append(blocking, depID)
        }
    }

    return blocking, nil
}

// IsBlocked returns true if task has incomplete dependencies
func (r *Resolver) IsBlocked(taskID string) bool {
    blocking, err := r.GetBlocking(taskID)
    if err != nil {
        return true // Not found = blocked
    }
    return len(blocking) > 0
}
```

### Topological Sort

```go
// TopologicalSort returns tasks in dependency order
// Tasks with no dependencies come first, then tasks whose deps are satisfied
func (r *Resolver) TopologicalSort() ([]*Task, error) {
    // Check for cycles first
    if cycle := r.DetectCycle(); cycle != nil {
        return nil, fmt.Errorf("dependency cycle detected: %s", strings.Join(cycle, " → "))
    }

    // Kahn's algorithm
    inDegree := make(map[string]int)
    dependents := make(map[string][]string) // dep -> tasks that depend on it

    // Initialize
    for id, t := range r.tasks {
        inDegree[id] = len(t.DependsOn)
        for _, depID := range t.DependsOn {
            dependents[depID] = append(dependents[depID], id)
        }
    }

    // Start with tasks that have no dependencies
    var queue []string
    for id, degree := range inDegree {
        if degree == 0 {
            queue = append(queue, id)
        }
    }

    var sorted []*Task
    for len(queue) > 0 {
        // Pop first
        id := queue[0]
        queue = queue[1:]

        sorted = append(sorted, r.tasks[id])

        // Reduce in-degree of dependents
        for _, depID := range dependents[id] {
            inDegree[depID]--
            if inDegree[depID] == 0 {
                queue = append(queue, depID)
            }
        }
    }

    // If we didn't process all tasks, there's a cycle
    // (Already checked above, but be defensive)
    if len(sorted) != len(r.tasks) {
        return nil, fmt.Errorf("could not resolve all dependencies")
    }

    return sorted, nil
}
```

### Cycle Detection

```go
// DetectCycle returns a cycle if one exists, nil otherwise
func (r *Resolver) DetectCycle() []string {
    // Track visit state: 0=unvisited, 1=visiting, 2=visited
    state := make(map[string]int)
    parent := make(map[string]string) // For reconstructing cycle path

    var cycle []string

    var dfs func(id string) bool
    dfs = func(id string) bool {
        state[id] = 1 // Visiting

        t, ok := r.tasks[id]
        if !ok {
            state[id] = 2 // Visited (missing dep, not a cycle)
            return false
        }

        for _, depID := range t.DependsOn {
            if state[depID] == 1 {
                // Found cycle - reconstruct path
                cycle = r.reconstructCycle(id, depID, parent)
                return true
            }
            if state[depID] == 0 {
                parent[depID] = id
                if dfs(depID) {
                    return true
                }
            }
        }

        state[id] = 2 // Visited
        return false
    }

    for id := range r.tasks {
        if state[id] == 0 {
            if dfs(id) {
                return cycle
            }
        }
    }

    return nil
}

func (r *Resolver) reconstructCycle(start, end string, parent map[string]string) []string {
    path := []string{end}

    current := start
    for current != end {
        path = append([]string{current}, path...)
        current = parent[current]
    }

    path = append([]string{end}, path...) // Complete the cycle
    return path
}
```

### Dependency Levels

```go
// GetLevels returns tasks grouped by dependency level
// Level 0 = no dependencies, Level 1 = depends only on Level 0, etc.
func (r *Resolver) GetLevels() ([][]* Task, error) {
    sorted, err := r.TopologicalSort()
    if err != nil {
        return nil, err
    }

    // Calculate level for each task
    levels := make(map[string]int)

    for _, t := range sorted {
        maxDepLevel := -1
        for _, depID := range t.DependsOn {
            if level, ok := levels[depID]; ok && level > maxDepLevel {
                maxDepLevel = level
            }
        }
        levels[t.ID] = maxDepLevel + 1
    }

    // Group by level
    maxLevel := 0
    for _, level := range levels {
        if level > maxLevel {
            maxLevel = level
        }
    }

    result := make([][]*Task, maxLevel+1)
    for _, t := range sorted {
        level := levels[t.ID]
        result[level] = append(result[level], t)
    }

    return result, nil
}
```

### Dependency Graph

```go
// Graph returns a string representation of the dependency graph
func (r *Resolver) Graph() string {
    var sb strings.Builder

    sb.WriteString("Dependency Graph:\n")

    sorted, err := r.TopologicalSort()
    if err != nil {
        sb.WriteString(fmt.Sprintf("  Error: %v\n", err))
        return sb.String()
    }

    for _, t := range sorted {
        status := string(t.Status)
        if len(t.DependsOn) == 0 {
            sb.WriteString(fmt.Sprintf("  %s [%s]\n", t.ID, status))
        } else {
            deps := strings.Join(t.DependsOn, ", ")
            sb.WriteString(fmt.Sprintf("  %s [%s] ← (%s)\n", t.ID, status, deps))
        }
    }

    return sb.String()
}

// Mermaid returns a Mermaid diagram of dependencies
func (r *Resolver) Mermaid() string {
    var sb strings.Builder

    sb.WriteString("graph TD\n")

    for _, t := range r.tasks {
        // Node
        shape := fmt.Sprintf("    %s[%s]", t.ID, t.Title)
        sb.WriteString(shape + "\n")

        // Edges
        for _, depID := range t.DependsOn {
            sb.WriteString(fmt.Sprintf("    %s --> %s\n", depID, t.ID))
        }
    }

    return sb.String()
}
```

### Integration with TaskManager

```go
// AddToTaskManager shows how to integrate with the TaskManager
func (m *Manager) UpdateBlockedTasks() error {
    tasks, _ := m.Scan()
    resolver := NewResolver(tasks)

    for _, t := range tasks {
        if t.Status == StatusPending || t.Status == StatusBlocked {
            if resolver.IsBlocked(t.ID) {
                if t.Status != StatusBlocked {
                    m.UpdateStatus(t.ID, StatusBlocked)
                }
            } else {
                if t.Status == StatusBlocked {
                    m.UpdateStatus(t.ID, StatusPending)
                }
            }
        }
    }

    return nil
}

// GetExecutionPlan returns tasks in optimal execution order
func (m *Manager) GetExecutionPlan() ([]*Task, error) {
    tasks, err := m.Scan()
    if err != nil {
        return nil, err
    }

    resolver := NewResolver(tasks)
    return resolver.TopologicalSort()
}
```

## Testing

### Unit Tests

```go
func TestResolver_GetReady(t *testing.T) {
    tasks := []*Task{
        {ID: "T1", Status: StatusPending, DependsOn: nil},
        {ID: "T2", Status: StatusPending, DependsOn: []string{"T1"}},
        {ID: "T3", Status: StatusPending, DependsOn: []string{"T1", "T2"}},
    }

    resolver := NewResolver(tasks)
    ready := resolver.GetReady()

    // Only T1 should be ready (no deps)
    if len(ready) != 1 || ready[0].ID != "T1" {
        t.Errorf("GetReady() = %v, want [T1]", taskIDs(ready))
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
    if len(ready) != 1 || ready[0].ID != "T2" {
        t.Errorf("GetReady() = %v, want [T2]", taskIDs(ready))
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

func TestResolver_NoCycle(t *testing.T) {
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
```

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Dependency cycle | Return error with cycle path |
| Missing dependency | Treat task as blocked |
| Task not found | Return error |
| Empty task list | Return empty result |

## Out of Scope

- Dynamic dependency updates
- Soft dependencies (optional)
- Dependency versioning
- Cross-project dependencies

## Notes

The resolver is stateless and works on a snapshot of tasks. If task status changes, create a new resolver or refresh the task list.

Cycle detection runs before topological sort to provide better error messages. The cycle path helps users identify and fix the issue.
