package task

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

// Manager handles scanning, loading, querying, and updating tasks.
// It maintains an in-memory cache for fast reads but always writes through
// to disk for persistence.
type Manager struct {
	config   *Config
	tasksDir string
	tasks    map[string]*Task
	mu       sync.RWMutex
}

// Config holds configuration for the TaskManager.
type Config struct {
	ProjectRoot string
	// TasksDir is the directory for task files, relative to ProjectRoot.
	// Defaults to "tasks" if empty.
	TasksDir string
}

// NewManager creates a new TaskManager.
func NewManager(cfg *Config) *Manager {
	tasksDir := cfg.TasksDir
	if tasksDir == "" {
		tasksDir = "tasks"
	}
	return &Manager{
		config:   cfg,
		tasksDir: filepath.Join(cfg.ProjectRoot, tasksDir),
		tasks:    make(map[string]*Task),
	}
}

// Scan loads all task files from the configured tasks directory.
// It supports project folder structure (tasks/project-name/*.md where
// project-name contains a README.md to identify it as a project).
// Invalid task files are logged as warnings but don't stop the scan.
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

	tasks := make([]*Task, 0, len(entries))
	var parseErrors []error

	for _, entry := range entries {
		if entry.IsDir() {
			// Check if it's a project folder (contains README.md)
			readmePath := filepath.Join(m.tasksDir, entry.Name(), "README.md")
			if _, err := os.Stat(readmePath); err == nil {
				// It's a project folder - scan it for tasks
				projectName := entry.Name()
				projectTasks, errs := m.scanProjectDir(filepath.Join(m.tasksDir, entry.Name()), projectName)
				tasks = append(tasks, projectTasks...)
				parseErrors = append(parseErrors, errs...)
			}
			continue
		}

		if filepath.Ext(entry.Name()) != ".md" {
			continue
		}

		// Skip README.md in root tasks directory
		if entry.Name() == "README.md" {
			continue
		}

		path := filepath.Join(m.tasksDir, entry.Name())
		task, err := ParseFile(path)
		if err != nil {
			// Log warning but continue scanning
			parseErrors = append(parseErrors, fmt.Errorf("parse %s: %w", entry.Name(), err))
			continue
		}

		// Root tasks have no project
		task.Project = ""
		m.tasks[task.ID] = task
		tasks = append(tasks, task)
	}

	// Log any errors encountered
	for _, err := range parseErrors {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
	}

	return tasks, nil
}

// scanProjectDir scans a project folder for task files.
// The projectName is set on each task's Project field.
func (m *Manager) scanProjectDir(dir, projectName string) ([]*Task, []error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, []error{fmt.Errorf("read project directory %s: %w", projectName, err)}
	}

	tasks := make([]*Task, 0, len(entries))
	var parseErrors []error

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}

		// Skip README.md - it's project metadata, not a task
		if entry.Name() == "README.md" {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		task, err := ParseFile(path)
		if err != nil {
			parseErrors = append(parseErrors, fmt.Errorf("parse %s/%s: %w", projectName, entry.Name(), err))
			continue
		}

		// Set project name on task
		task.Project = projectName
		m.tasks[task.ID] = task
		tasks = append(tasks, task)
	}

	return tasks, parseErrors
}

// Get returns a task by ID.
func (m *Manager) Get(id string) (*Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, ok := m.tasks[id]
	if !ok {
		return nil, fmt.Errorf("task %q not found", id)
	}

	return task, nil
}

// listOptions holds options for List().
type listOptions struct {
	sortByPriority bool
}

// ListOption is a functional option for List().
type ListOption func(*listOptions)

// SortByPriority returns a ListOption that sorts tasks by priority.
func SortByPriority() ListOption {
	return func(o *listOptions) {
		o.sortByPriority = true
	}
}

// List returns all tasks, optionally sorted.
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
			return tasks[i].Priority.Order() < tasks[j].Priority.Order()
		})
	}

	return tasks
}

// GetByRole returns tasks for a specific role.
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

// GetByStatus returns tasks with a specific status.
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

// GetByWorkstream returns tasks for a specific workstream.
// Tasks are returned sorted by priority, then by ID for stability.
func (m *Manager) GetByWorkstream(workstream string) []*Task {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var tasks []*Task
	for _, t := range m.tasks {
		if t.GetWorkstream() == workstream {
			tasks = append(tasks, t)
		}
	}

	// Sort by priority, then by ID
	sort.Slice(tasks, func(i, j int) bool {
		if tasks[i].Priority.Order() != tasks[j].Priority.Order() {
			return tasks[i].Priority.Order() < tasks[j].Priority.Order()
		}
		return tasks[i].ID < tasks[j].ID
	})

	return tasks
}

// GetByRoleAndWorkstream returns tasks for a specific role and workstream.
func (m *Manager) GetByRoleAndWorkstream(role, workstream string) []*Task {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var tasks []*Task
	for _, t := range m.tasks {
		if t.Role == role && t.GetWorkstream() == workstream {
			tasks = append(tasks, t)
		}
	}

	// Sort by priority, then by ID
	sort.Slice(tasks, func(i, j int) bool {
		if tasks[i].Priority.Order() != tasks[j].Priority.Order() {
			return tasks[i].Priority.Order() < tasks[j].Priority.Order()
		}
		return tasks[i].ID < tasks[j].ID
	})

	return tasks
}

// GetWorkstreams returns all unique workstreams for a role.
// Returns workstreams sorted by the priority of their highest-priority pending task.
func (m *Manager) GetWorkstreams(role string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Map workstream to its highest priority (lowest order number)
	workstreamPriority := make(map[string]int)

	for _, t := range m.tasks {
		if t.Role != role {
			continue
		}
		ws := t.GetWorkstream()
		currentPriority, exists := workstreamPriority[ws]
		taskPriority := t.Priority.Order()

		// Track the highest priority (lowest number) for each workstream
		if !exists || taskPriority < currentPriority {
			workstreamPriority[ws] = taskPriority
		}
	}

	// Convert to slice and sort by priority
	workstreams := make([]string, 0, len(workstreamPriority))
	for ws := range workstreamPriority {
		workstreams = append(workstreams, ws)
	}

	sort.Slice(workstreams, func(i, j int) bool {
		if workstreamPriority[workstreams[i]] != workstreamPriority[workstreams[j]] {
			return workstreamPriority[workstreams[i]] < workstreamPriority[workstreams[j]]
		}
		return workstreams[i] < workstreams[j]
	})

	return workstreams
}

// GetPending returns all pending tasks, sorted by priority.
func (m *Manager) GetPending() []*Task {
	tasks := m.GetByStatus(StatusPending)
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].Priority.Order() < tasks[j].Priority.Order()
	})
	return tasks
}

// GetNextAvailable returns the highest priority pending task for a role.
// It skips blocked tasks.
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
		return candidates[i].Priority.Order() < candidates[j].Priority.Order()
	})

	// Return first non-blocked task
	for _, t := range candidates {
		blocked, err := m.isBlockedInternal(t.ID)
		if err != nil {
			continue // Skip tasks we can't check
		}
		if !blocked {
			return t, nil
		}
	}

	return nil, fmt.Errorf("all pending tasks for role %q are blocked", role)
}

// UpdateStatus changes task status and persists to file.
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

// UpdateFailure marks a task as failed and persists error information to the task file.
// This method sets the task status to failed and stores the error message and log file path.
func (m *Manager) UpdateFailure(id string, err error, logPath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[id]
	if !ok {
		return fmt.Errorf("task %q not found", id)
	}

	task.Status = StatusFailed
	if err != nil {
		task.FailureMessage = err.Error()
	}
	task.LogFilePath = logPath

	// Write back to file
	if writeErr := WriteFile(task); writeErr != nil {
		return fmt.Errorf("write task file: %w", writeErr)
	}

	return nil
}

// Update persists task changes to file.
// This is a general update method that writes all task fields.
func (m *Manager) Update(task *Task) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if task == nil {
		return fmt.Errorf("task is nil")
	}

	// Ensure task exists in cache
	if _, ok := m.tasks[task.ID]; !ok {
		return fmt.Errorf("task %q not found", task.ID)
	}

	// Update cache
	m.tasks[task.ID] = task

	// Write back to file
	if err := WriteFile(task); err != nil {
		return fmt.Errorf("write task file: %w", err)
	}

	return nil
}

// Assign assigns a task to an agent.
// The task must be in pending or blocked status.
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

// Unassign removes agent assignment from a task.
// If the task is assigned or in_progress, it reverts to pending.
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

// IsBlocked checks if a task's dependencies are all complete.
func (m *Manager) IsBlocked(id string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.isBlockedInternal(id)
}

// isBlockedInternal checks if a task is blocked without acquiring a lock.
// The caller must hold at least a read lock.
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

// GetBlockingTasks returns the IDs of incomplete dependencies.
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

// UpdateBlockedStatus checks all tasks and updates blocked status.
// Tasks with unmet dependencies are marked as blocked.
// Tasks that were blocked but now have all dependencies complete are marked as pending.
func (m *Manager) UpdateBlockedStatus() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, task := range m.tasks {
		if task.Status == StatusPending || task.Status == StatusBlocked {
			blocked, err := m.isBlockedInternal(task.ID)
			if err != nil {
				continue // Skip tasks we can't check
			}
			if blocked && task.Status != StatusBlocked {
				task.Status = StatusBlocked
				_ = WriteFile(task)
			} else if !blocked && task.Status == StatusBlocked {
				task.Status = StatusPending
				_ = WriteFile(task)
			}
		}
	}

	return nil
}

// Stats holds statistics about tasks.
type Stats struct {
	Total        int
	ByStatus     map[Status]int
	ByRole       map[string]int
	ByPriority   map[Priority]int
	ByWorkstream map[string]int // Workstream -> task count
}

// Stats returns task statistics.
func (m *Manager) Stats() *Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &Stats{
		ByStatus:     make(map[Status]int),
		ByRole:       make(map[string]int),
		ByPriority:   make(map[Priority]int),
		ByWorkstream: make(map[string]int),
	}

	for _, t := range m.tasks {
		stats.Total++
		stats.ByStatus[t.Status]++
		stats.ByRole[t.Role]++
		stats.ByPriority[t.Priority]++
		stats.ByWorkstream[t.GetWorkstream()]++
	}

	return stats
}

// TasksDir returns the path to the tasks directory.
func (m *Manager) TasksDir() string {
	return m.tasksDir
}

// GetProjects returns all unique project names from scanned tasks.
// Returns an empty slice if no project folders are used.
func (m *Manager) GetProjects() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	projects := make(map[string]bool)
	for _, t := range m.tasks {
		if t.Project != "" {
			projects[t.Project] = true
		}
	}

	result := make([]string, 0, len(projects))
	for p := range projects {
		result = append(result, p)
	}

	sort.Strings(result)
	return result
}

// GetByProject returns all tasks belonging to a specific project.
// Tasks are sorted by priority, then by ID.
func (m *Manager) GetByProject(project string) []*Task {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var tasks []*Task
	for _, t := range m.tasks {
		if t.Project == project {
			tasks = append(tasks, t)
		}
	}

	sort.Slice(tasks, func(i, j int) bool {
		if tasks[i].Priority.Order() != tasks[j].Priority.Order() {
			return tasks[i].Priority.Order() < tasks[j].Priority.Order()
		}
		return tasks[i].ID < tasks[j].ID
	})

	return tasks
}

// GetProjectWorkstreams returns all unique workstreams within a project for a role.
// Returns workstreams sorted by priority.
func (m *Manager) GetProjectWorkstreams(project, role string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	workstreamPriority := make(map[string]int)

	for _, t := range m.tasks {
		if t.Project != project || t.Role != role {
			continue
		}
		ws := t.GetWorkstream()
		currentPriority, exists := workstreamPriority[ws]
		taskPriority := t.Priority.Order()

		if !exists || taskPriority < currentPriority {
			workstreamPriority[ws] = taskPriority
		}
	}

	workstreams := make([]string, 0, len(workstreamPriority))
	for ws := range workstreamPriority {
		workstreams = append(workstreams, ws)
	}

	sort.Slice(workstreams, func(i, j int) bool {
		if workstreamPriority[workstreams[i]] != workstreamPriority[workstreams[j]] {
			return workstreamPriority[workstreams[i]] < workstreamPriority[workstreams[j]]
		}
		return workstreams[i] < workstreams[j]
	})

	return workstreams
}

// GetByProjectAndWorkstream returns tasks for a specific project, role, and workstream.
func (m *Manager) GetByProjectAndWorkstream(project, role, workstream string) []*Task {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var tasks []*Task
	for _, t := range m.tasks {
		if t.Project == project && t.Role == role && t.GetWorkstream() == workstream {
			tasks = append(tasks, t)
		}
	}

	sort.Slice(tasks, func(i, j int) bool {
		if tasks[i].Priority.Order() != tasks[j].Priority.Order() {
			return tasks[i].Priority.Order() < tasks[j].Priority.Order()
		}
		return tasks[i].ID < tasks[j].ID
	})

	return tasks
}

// ReconcileStaleAssignments resets tasks that are stuck in assigned, in_progress,
// or failed status but whose agent is no longer active. Pass a set of active agent names;
// tasks assigned to agents not in the set will be reset to pending.
// If activeAgents is nil, all non-terminal tasks are reset.
func (m *Manager) ReconcileStaleAssignments(activeAgents map[string]bool) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	for _, task := range m.tasks {
		// Only reset non-terminal, non-pending states
		if task.Status != StatusAssigned && task.Status != StatusInProgress && task.Status != StatusFailed {
			continue
		}

		// If no active agents provided, reset all stale tasks
		// Otherwise, only reset if agent is not in the active set
		shouldReset := activeAgents == nil || !activeAgents[task.AssignedTo]

		if shouldReset {
			task.AssignedTo = ""
			task.Status = StatusPending
			if err := WriteFile(task); err != nil {
				return count, fmt.Errorf("write task %s: %w", task.ID, err)
			}
			count++
		}
	}

	return count, nil
}
