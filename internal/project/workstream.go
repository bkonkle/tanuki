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

// WorkstreamScheduler manages workstream scheduling with per-workstream concurrency.
type WorkstreamScheduler struct {
	mu sync.RWMutex

	// taskMgr provides task access
	taskMgr TaskManager

	// workstreamConcurrency maps workstream names to their concurrency limits
	workstreamConcurrency map[string]int

	// activeWorkstreams tracks currently active workstreams
	activeWorkstreams map[string]*WorkstreamState

	// pendingWorkstreams tracks workstreams waiting to be scheduled
	pendingWorkstreams []string

	// workstreamStates stores all workstream states
	workstreamStates map[string]*WorkstreamState
}

// NewWorkstreamScheduler creates a new workstream scheduler.
func NewWorkstreamScheduler(taskMgr TaskManager) *WorkstreamScheduler {
	return &WorkstreamScheduler{
		taskMgr:               taskMgr,
		workstreamConcurrency: make(map[string]int),
		activeWorkstreams:     make(map[string]*WorkstreamState),
		pendingWorkstreams:    []string{},
		workstreamStates:      make(map[string]*WorkstreamState),
	}
}

// SetWorkstreamConcurrency sets the concurrency limit for a workstream.
func (s *WorkstreamScheduler) SetWorkstreamConcurrency(workstream string, concurrency int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if concurrency <= 0 {
		concurrency = 1
	}
	s.workstreamConcurrency[workstream] = concurrency
}

// GetWorkstreamConcurrency returns the concurrency limit for a workstream.
func (s *WorkstreamScheduler) GetWorkstreamConcurrency(workstream string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if c, ok := s.workstreamConcurrency[workstream]; ok {
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

	// Group tasks by workstream
	workstreamTasks := make(map[string][]*task.Task)

	for _, t := range tasks {
		ws := t.GetWorkstream()
		workstreamTasks[ws] = append(workstreamTasks[ws], t)
	}

	// Create workstream states
	for wsName, wsTasks := range workstreamTasks {
		// Sort tasks by priority and ID
		taskIDs := make([]string, len(wsTasks))
		for i, t := range wsTasks {
			taskIDs[i] = t.ID
		}

		state := &WorkstreamState{
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
			s.pendingWorkstreams = append(s.pendingWorkstreams, wsName)
		}

		s.workstreamStates[wsName] = state
	}

	return nil
}

// GetNextWorkstream returns the next workstream to schedule.
// Returns nil if no workstreams are available or concurrency limit is reached.
func (s *WorkstreamScheduler) GetNextWorkstream() *WorkstreamState {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.pendingWorkstreams) == 0 {
		return nil // No pending workstreams
	}

	// Find first pending workstream that isn't already active
	for i, wsName := range s.pendingWorkstreams {
		// Check if already active
		if _, active := s.activeWorkstreams[wsName]; active {
			continue // Already running
		}

		// Remove from pending and return
		s.pendingWorkstreams = append(s.pendingWorkstreams[:i], s.pendingWorkstreams[i+1:]...)
		return s.workstreamStates[wsName]
	}

	return nil
}

// ActivateWorkstream marks a workstream as active and assigns it to an agent.
func (s *WorkstreamScheduler) ActivateWorkstream(workstream, agentName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	state := s.workstreamStates[workstream]
	if state == nil {
		return fmt.Errorf("workstream %q not found", workstream)
	}

	state.Status = WorkstreamActive
	state.AgentName = agentName
	state.StartedAt = time.Now()
	state.CurrentTask = state.NextTask()

	s.activeWorkstreams[workstream] = state

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
					delete(s.activeWorkstreams, state.Workstream)
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
				delete(s.activeWorkstreams, state.Workstream)
				return nil
			}
		}
	}

	return fmt.Errorf("task %q not found in any workstream", taskID)
}

// GetActiveWorkstreams returns all active workstreams.
func (s *WorkstreamScheduler) GetActiveWorkstreams() []*WorkstreamState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*WorkstreamState, 0, len(s.activeWorkstreams))
	for _, state := range s.activeWorkstreams {
		result = append(result, state)
	}
	return result
}

// GetPendingCount returns the number of pending workstreams.
func (s *WorkstreamScheduler) GetPendingCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.pendingWorkstreams)
}

// GetWorkstreamState returns the state of a specific workstream.
func (s *WorkstreamScheduler) GetWorkstreamState(workstream string) *WorkstreamState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.workstreamStates[workstream]
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
		ByWorkstream: make(map[string]*WorkstreamStateStats),
		ByStatus:     make(map[WorkstreamStatus]int),
	}

	for _, state := range s.workstreamStates {
		stats.Total++
		stats.ByStatus[state.Status]++

		wsStats := stats.ByWorkstream[state.Workstream]
		if wsStats == nil {
			wsStats = &WorkstreamStateStats{Workstream: state.Workstream}
			stats.ByWorkstream[state.Workstream] = wsStats
		}

		wsStats.TaskCount = len(state.Tasks)
		wsStats.CompletedCount = len(state.CompletedTasks)
		wsStats.Status = state.Status
	}

	return stats
}

// WorkstreamStats contains workstream statistics.
type WorkstreamStats struct {
	Total        int
	ByWorkstream map[string]*WorkstreamStateStats
	ByStatus     map[WorkstreamStatus]int
}

// WorkstreamStateStats contains stats for a specific workstream.
type WorkstreamStateStats struct {
	Workstream     string
	TaskCount      int
	CompletedCount int
	Status         WorkstreamStatus
}
