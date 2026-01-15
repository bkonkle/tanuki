package task

import (
	"fmt"
	"strings"
)

// Resolver handles dependency resolution for tasks.
// It provides topological sorting, cycle detection, and determines
// which tasks are ready to execute based on their dependencies.
type Resolver struct {
	tasks map[string]*Task
}

// NewResolver creates a resolver for a set of tasks.
func NewResolver(tasks []*Task) *Resolver {
	taskMap := make(map[string]*Task, len(tasks))
	for _, t := range tasks {
		taskMap[t.ID] = t
	}
	return &Resolver{tasks: taskMap}
}

// GetReady returns tasks that are ready to execute (all deps satisfied).
// Only returns pending tasks with all dependencies complete.
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

// isReady checks if all dependencies are complete.
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

// GetBlocking returns incomplete dependencies for a task.
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

// IsBlocked returns true if task has incomplete dependencies.
func (r *Resolver) IsBlocked(taskID string) bool {
	blocking, err := r.GetBlocking(taskID)
	if err != nil {
		return true // Not found = blocked
	}
	return len(blocking) > 0
}

// TopologicalSort returns tasks in dependency order.
// Tasks with no dependencies come first, then tasks whose deps are satisfied.
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

// DetectCycle returns a cycle if one exists, nil otherwise.
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

// reconstructCycle builds the cycle path from start to end.
func (r *Resolver) reconstructCycle(start, end string, parent map[string]string) []string {
	path := []string{end}

	current := start
	for current != end && current != "" {
		path = append([]string{current}, path...)
		current = parent[current]
	}

	path = append([]string{end}, path...) // Complete the cycle
	return path
}

// GetLevels returns tasks grouped by dependency level.
// Level 0 = no dependencies, Level 1 = depends only on Level 0, etc.
func (r *Resolver) GetLevels() ([][]*Task, error) {
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

	// Find max level
	maxLevel := 0
	for _, level := range levels {
		if level > maxLevel {
			maxLevel = level
		}
	}

	// Handle empty task set
	if len(sorted) == 0 {
		return nil, nil
	}

	// Group by level
	result := make([][]*Task, maxLevel+1)
	for _, t := range sorted {
		level := levels[t.ID]
		result[level] = append(result[level], t)
	}

	return result, nil
}

// Graph returns a string representation of the dependency graph.
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

// Mermaid returns a Mermaid diagram of dependencies.
func (r *Resolver) Mermaid() string {
	var sb strings.Builder

	sb.WriteString("graph TD\n")

	for _, t := range r.tasks {
		// Node
		shape := fmt.Sprintf("    %s[\"%s\"]", t.ID, t.Title)
		sb.WriteString(shape + "\n")

		// Edges
		for _, depID := range t.DependsOn {
			sb.WriteString(fmt.Sprintf("    %s --> %s\n", depID, t.ID))
		}
	}

	return sb.String()
}

// HasTask returns true if the resolver has a task with the given ID.
func (r *Resolver) HasTask(id string) bool {
	_, ok := r.tasks[id]
	return ok
}

// TaskCount returns the number of tasks in the resolver.
func (r *Resolver) TaskCount() int {
	return len(r.tasks)
}

// GetReadyTasksForWorkstream returns unblocked tasks for a specific workstream.
func (r *Resolver) GetReadyTasksForWorkstream(workstream string) []*Task {
	var ready []*Task

	for _, t := range r.tasks {
		if t.GetWorkstream() != workstream {
			continue
		}
		if t.Status != StatusPending {
			continue
		}
		if r.isReady(t) {
			ready = append(ready, t)
		}
	}

	return ready
}

// GetWorkstreamDependencies returns cross-workstream dependencies.
// Returns a map of workstream -> workstreams it depends on.
func (r *Resolver) GetWorkstreamDependencies() map[string][]string {
	// Group tasks by workstream
	wsMap := make(map[string][]*Task)
	for _, t := range r.tasks {
		ws := t.GetWorkstream()
		wsMap[ws] = append(wsMap[ws], t)
	}

	// Build workstream dependency graph
	deps := make(map[string][]string)

	for ws, wsTasks := range wsMap {
		depsSet := make(map[string]bool)

		for _, t := range wsTasks {
			for _, depID := range t.DependsOn {
				depTask, ok := r.tasks[depID]
				if !ok {
					continue
				}

				depWS := depTask.GetWorkstream()
				if depWS != ws {
					depsSet[depWS] = true
				}
			}
		}

		for depWS := range depsSet {
			deps[ws] = append(deps[ws], depWS)
		}
	}

	return deps
}
