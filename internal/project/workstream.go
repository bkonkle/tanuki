package project

import (
	"fmt"
	"sync"
	"time"

	"github.com/bkonkle/tanuki/internal/task"
)

// WorkstreamStatus represents the current state of a workstream.
type WorkstreamStatus string

const (
	// WorkstreamPending indicates the workstream has not started.
	WorkstreamPending WorkstreamStatus = "pending"
	// WorkstreamActive indicates the workstream is currently running.
	WorkstreamActive WorkstreamStatus = "active"
	// WorkstreamCompleted indicates all tasks in the workstream are complete.
	WorkstreamCompleted WorkstreamStatus = "completed"
	// WorkstreamFailed indicates a task in the workstream failed.
	WorkstreamFailed WorkstreamStatus = "failed"
)

// WorkstreamState tracks the state of a workstream.
type WorkstreamState struct {
	// Role this workstream belongs to
	Role string

	// Workstream identifier
	Workstream string

	// AgentName assigned to this workstream (owns the worktree)
	AgentName string

	// Status of the workstream
	Status WorkstreamStatus

	// CurrentTask is the currently executing task ID
	CurrentTask string

	// Tasks is the ordered list of task IDs in this workstream
	Tasks []string

	// CompletedTasks tracks which tasks have completed
	CompletedTasks map[string]bool

	// StartedAt is when the workstream started executing
	StartedAt time.Time

	// CompletedAt is when the workstream completed (if applicable)
	CompletedAt *time.Time
}

// IsComplete returns true if all tasks are complete.
func (ws *WorkstreamState) IsComplete() bool {
	for _, taskID := range ws.Tasks {
		if !ws.CompletedTasks[taskID] {
			return false
		}
	}
	return true
}

// NextTask returns the next pending task in the workstream.
func (ws *WorkstreamState) NextTask() string {
	for _, taskID := range ws.Tasks {
		if !ws.CompletedTasks[taskID] {
			return taskID
		}
	}
	return ""
}

// Progress returns the completion percentage.
func (ws *WorkstreamState) Progress() float64 {
	if len(ws.Tasks) == 0 {
		return 100.0
	}
	completed := 0
	for _, done := range ws.CompletedTasks {
		if done {
			completed++
		}
	}
	return float64(completed) / float64(len(ws.Tasks)) * 100
}

// WorkstreamScheduler manages workstream scheduling with per-role concurrency.
type WorkstreamScheduler struct {
	mu sync.RWMutex

	// taskMgr provides task access
	taskMgr TaskManager

	// roleConcurrency maps role names to their concurrency limits
	roleConcurrency map[string]int

	// activeWorkstreams tracks currently active workstreams by role
	activeWorkstreams map[string][]*WorkstreamState

	// pendingWorkstreams tracks workstreams waiting to be scheduled
	pendingWorkstreams map[string][]string // role -> []workstream IDs

	// workstreamStates stores all workstream states
	workstreamStates map[string]*WorkstreamState // "role:workstream" -> state
}

// NewWorkstreamScheduler creates a new workstream scheduler.
func NewWorkstreamScheduler(taskMgr TaskManager) *WorkstreamScheduler {
	return &WorkstreamScheduler{
		taskMgr:            taskMgr,
		roleConcurrency:    make(map[string]int),
		activeWorkstreams:  make(map[string][]*WorkstreamState),
		pendingWorkstreams: make(map[string][]string),
		workstreamStates:   make(map[string]*WorkstreamState),
	}
}

// SetRoleConcurrency sets the concurrency limit for a role.
func (s *WorkstreamScheduler) SetRoleConcurrency(role string, concurrency int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if concurrency <= 0 {
		concurrency = 1
	}
	s.roleConcurrency[role] = concurrency
}

// GetRoleConcurrency returns the concurrency limit for a role.
func (s *WorkstreamScheduler) GetRoleConcurrency(role string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if c, ok := s.roleConcurrency[role]; ok {
		return c
	}
	return 1 // Default
}

// Initialize scans tasks and builds initial workstream state.
func (s *WorkstreamScheduler) Initialize() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tasks, err := s.taskMgr.Scan()
	if err != nil {
		return fmt.Errorf("scan tasks: %w", err)
	}

	// Group tasks by role and workstream
	workstreamTasks := make(map[string]map[string][]*task.Task) // role -> workstream -> tasks

	for _, t := range tasks {
		role := t.Role
		ws := t.GetWorkstream()

		if workstreamTasks[role] == nil {
			workstreamTasks[role] = make(map[string][]*task.Task)
		}
		workstreamTasks[role][ws] = append(workstreamTasks[role][ws], t)
	}

	// Create workstream states
	for role, workstreams := range workstreamTasks {
		for wsName, wsTasks := range workstreams {
			key := workstreamKey(role, wsName)

			// Sort tasks by priority and ID
			taskIDs := make([]string, len(wsTasks))
			for i, t := range wsTasks {
				taskIDs[i] = t.ID
			}

			state := &WorkstreamState{
				Role:           role,
				Workstream:     wsName,
				Status:         WorkstreamPending,
				Tasks:          taskIDs,
				CompletedTasks: make(map[string]bool),
			}

			// Mark already completed tasks
			for _, t := range wsTasks {
				if t.Status == task.StatusComplete {
					state.CompletedTasks[t.ID] = true
				}
			}

			// Determine initial status
			if state.IsComplete() {
				state.Status = WorkstreamCompleted
			} else {
				// Add to pending queue
				s.pendingWorkstreams[role] = append(s.pendingWorkstreams[role], wsName)
			}

			s.workstreamStates[key] = state
		}
	}

	return nil
}

// GetNextWorkstream returns the next workstream to schedule for a role.
// Returns nil if no workstreams are available or concurrency limit is reached.
func (s *WorkstreamScheduler) GetNextWorkstream(role string) *WorkstreamState {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check concurrency limit
	concurrency := s.roleConcurrency[role]
	if concurrency <= 0 {
		concurrency = 1
	}

	activeCount := len(s.activeWorkstreams[role])
	if activeCount >= concurrency {
		return nil // At capacity
	}

	// Get next pending workstream
	pending := s.pendingWorkstreams[role]
	if len(pending) == 0 {
		return nil // No pending workstreams
	}

	// Pop the first pending workstream
	wsName := pending[0]
	s.pendingWorkstreams[role] = pending[1:]

	// Get the workstream state
	key := workstreamKey(role, wsName)
	state := s.workstreamStates[key]
	if state == nil {
		return nil // Should not happen
	}

	return state
}

// ActivateWorkstream marks a workstream as active and assigns it to an agent.
func (s *WorkstreamScheduler) ActivateWorkstream(role, workstream, agentName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := workstreamKey(role, workstream)
	state := s.workstreamStates[key]
	if state == nil {
		return fmt.Errorf("workstream %q not found for role %q", workstream, role)
	}

	state.Status = WorkstreamActive
	state.AgentName = agentName
	state.StartedAt = time.Now()
	state.CurrentTask = state.NextTask()

	s.activeWorkstreams[role] = append(s.activeWorkstreams[role], state)

	return nil
}

// CompleteTask marks a task as complete within its workstream.
func (s *WorkstreamScheduler) CompleteTask(taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Find the workstream containing this task
	for _, state := range s.workstreamStates {
		for _, tid := range state.Tasks {
			if tid == taskID {
				state.CompletedTasks[taskID] = true
				state.CurrentTask = state.NextTask()

				// Check if workstream is complete
				if state.IsComplete() {
					state.Status = WorkstreamCompleted
					now := time.Now()
					state.CompletedAt = &now
					s.removeFromActive(state.Role, state.Workstream)
				}

				return nil
			}
		}
	}

	return fmt.Errorf("task %q not found in any workstream", taskID)
}

// FailTask marks a task as failed, which fails the entire workstream.
func (s *WorkstreamScheduler) FailTask(taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, state := range s.workstreamStates {
		for _, tid := range state.Tasks {
			if tid == taskID {
				state.Status = WorkstreamFailed
				now := time.Now()
				state.CompletedAt = &now
				s.removeFromActive(state.Role, state.Workstream)
				return nil
			}
		}
	}

	return fmt.Errorf("task %q not found in any workstream", taskID)
}

// removeFromActive removes a workstream from the active list.
func (s *WorkstreamScheduler) removeFromActive(role, workstream string) {
	active := s.activeWorkstreams[role]
	for i, ws := range active {
		if ws.Workstream == workstream {
			s.activeWorkstreams[role] = append(active[:i], active[i+1:]...)
			return
		}
	}
}

// GetActiveWorkstreams returns all active workstreams for a role.
func (s *WorkstreamScheduler) GetActiveWorkstreams(role string) []*WorkstreamState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*WorkstreamState, len(s.activeWorkstreams[role]))
	copy(result, s.activeWorkstreams[role])
	return result
}

// GetPendingCount returns the number of pending workstreams for a role.
func (s *WorkstreamScheduler) GetPendingCount(role string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.pendingWorkstreams[role])
}

// GetWorkstreamState returns the state of a specific workstream.
func (s *WorkstreamScheduler) GetWorkstreamState(role, workstream string) *WorkstreamState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.workstreamStates[workstreamKey(role, workstream)]
}

// GetAllWorkstreamStates returns all workstream states.
func (s *WorkstreamScheduler) GetAllWorkstreamStates() []*WorkstreamState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*WorkstreamState, 0, len(s.workstreamStates))
	for _, state := range s.workstreamStates {
		result = append(result, state)
	}
	return result
}

// Stats returns workstream statistics.
func (s *WorkstreamScheduler) Stats() *WorkstreamStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &WorkstreamStats{
		ByRole:   make(map[string]*RoleWorkstreamStats),
		ByStatus: make(map[WorkstreamStatus]int),
	}

	for _, state := range s.workstreamStates {
		stats.Total++
		stats.ByStatus[state.Status]++

		roleStats := stats.ByRole[state.Role]
		if roleStats == nil {
			roleStats = &RoleWorkstreamStats{Role: state.Role}
			stats.ByRole[state.Role] = roleStats
		}

		roleStats.Total++
		switch state.Status {
		case WorkstreamActive:
			roleStats.Active++
		case WorkstreamPending:
			roleStats.Pending++
		case WorkstreamCompleted:
			roleStats.Completed++
		case WorkstreamFailed:
			roleStats.Failed++
		}
	}

	return stats
}

// WorkstreamStats contains workstream statistics.
type WorkstreamStats struct {
	Total    int
	ByRole   map[string]*RoleWorkstreamStats
	ByStatus map[WorkstreamStatus]int
}

// RoleWorkstreamStats contains workstream stats for a specific role.
type RoleWorkstreamStats struct {
	Role      string
	Total     int
	Active    int
	Pending   int
	Completed int
	Failed    int
}

// workstreamKey creates a unique key for a role:workstream pair.
func workstreamKey(role, workstream string) string {
	return role + ":" + workstream
}
