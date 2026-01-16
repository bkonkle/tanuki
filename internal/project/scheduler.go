package project

import (
	"fmt"
	"slices"
	"sort"
	"sync"

	"github.com/bkonkle/tanuki/internal/task"
)

// WorkstreamReadiness tracks the readiness state of a workstream for scheduling.
// It's used to determine which workstreams can make progress without deadlocking.
type WorkstreamReadiness struct {
	// Project name (empty for root tasks)
	Project string

	// Workstream identifier
	Workstream string

	// ReadyTaskCount is the number of tasks that can execute immediately (no blocking deps)
	ReadyTaskCount int

	// BlockedTaskCount is the number of tasks waiting on dependencies
	BlockedTaskCount int

	// TotalTaskCount is total pending tasks in this workstream
	TotalTaskCount int

	// FirstReadyTaskID is the ID of the first ready task (for display/logging)
	FirstReadyTaskID string

	// FirstReadyTaskPriority is the priority of the first ready task
	FirstReadyTaskPriority task.Priority

	// BlockingWorkstreams lists other workstreams this one is waiting on
	BlockingWorkstreams []string

	// DependentWorkstreams lists workstreams that depend on tasks in this workstream
	DependentWorkstreams []string
}

// IsReady returns true if the workstream has at least one ready (unblocked) task.
func (wr *WorkstreamReadiness) IsReady() bool {
	return wr.ReadyTaskCount > 0
}

// ReadinessScore returns a numeric score for priority ordering.
// Higher score = should be scheduled first.
func (wr *WorkstreamReadiness) ReadinessScore() int {
	score := 0
	if wr.ReadyTaskCount > 0 {
		score += 1000 // Base score for being ready
		score += wr.ReadyTaskCount * 10
		score -= wr.FirstReadyTaskPriority.Order() // Lower order = higher priority
		score += len(wr.DependentWorkstreams) * 5  // Unblock more workstreams = higher priority
	}
	return score
}

// Key returns the unique key for this workstream.
func (wr *WorkstreamReadiness) Key() string {
	return wr.Workstream
}

// DeadlockInfo contains information about a detected deadlock.
type DeadlockInfo struct {
	// AffectedWorkstreams lists workstreams involved in the deadlock
	AffectedWorkstreams []string

	// BlockedBy maps each workstream to the workstreams it's waiting on
	BlockedBy map[string][]string

	// Message is a human-readable description
	Message string

	// Suggestion is a suggested resolution
	Suggestion string
}

// ReadinessAwareScheduler manages workstream scheduling with deadlock prevention.
// It analyzes task dependencies to ensure only workstreams with ready tasks are spawned.
type ReadinessAwareScheduler struct {
	mu sync.RWMutex

	// taskMgr provides task access
	taskMgr *task.Manager

	// resolver for dependency checking
	resolver *task.Resolver

	// workstreamConcurrency maps workstream names to their concurrency limits
	workstreamConcurrency map[string]int

	// activeWorkstreams tracks currently running workstreams
	activeWorkstreams map[string]*WorkstreamReadiness

	// readyQueue holds sorted list of ready workstreams
	readyQueue []*WorkstreamReadiness

	// blockedWorkstreams tracks workstreams with no ready tasks
	blockedWorkstreams map[string]*WorkstreamReadiness

	// allWorkstreams stores all workstream readiness info
	allWorkstreams map[string]*WorkstreamReadiness

	// taskToWorkstream maps task IDs to their workstream keys
	taskToWorkstream map[string]string

	// callbacks for events
	onWorkstreamReady func(ws *WorkstreamReadiness)
}

// NewReadinessAwareScheduler creates a new scheduler with deadlock prevention.
func NewReadinessAwareScheduler(taskMgr *task.Manager) *ReadinessAwareScheduler {
	return &ReadinessAwareScheduler{
		taskMgr:               taskMgr,
		workstreamConcurrency: make(map[string]int),
		activeWorkstreams:     make(map[string]*WorkstreamReadiness),
		readyQueue:            []*WorkstreamReadiness{},
		blockedWorkstreams:    make(map[string]*WorkstreamReadiness),
		allWorkstreams:        make(map[string]*WorkstreamReadiness),
		taskToWorkstream:      make(map[string]string),
	}
}

// SetWorkstreamConcurrency sets the concurrency limit for a workstream.
func (s *ReadinessAwareScheduler) SetWorkstreamConcurrency(workstream string, limit int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit <= 0 {
		limit = 1
	}
	s.workstreamConcurrency[workstream] = limit
}

// GetWorkstreamConcurrency returns the concurrency limit for a workstream.
func (s *ReadinessAwareScheduler) GetWorkstreamConcurrency(workstream string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if c, ok := s.workstreamConcurrency[workstream]; ok {
		return c
	}
	return 1
}

// Initialize analyzes all tasks and builds the readiness graph.
// Returns an error if a dependency cycle is detected.
func (s *ReadinessAwareScheduler) Initialize() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tasks, err := s.taskMgr.Scan()
	if err != nil {
		return fmt.Errorf("scan tasks: %w", err)
	}

	if len(tasks) == 0 {
		return nil
	}

	// Create resolver for dependency analysis
	s.resolver = task.NewResolver(tasks)

	// Check for cycles - fail fast
	if cycle := s.resolver.DetectCycle(); cycle != nil {
		return fmt.Errorf("dependency cycle detected: %v", cycle)
	}

	// Group tasks by project/workstream
	type wsKey struct {
		project    string
		workstream string
	}
	workstreamTasks := make(map[wsKey][]*task.Task)

	for _, t := range tasks {
		key := wsKey{
			project:    t.Project,
			workstream: t.GetWorkstream(),
		}
		workstreamTasks[key] = append(workstreamTasks[key], t)

		// Map task to workstream
		s.taskToWorkstream[t.ID] = t.GetWorkstream()
	}

	// Build readiness info for each workstream
	for key, wsTasks := range workstreamTasks {
		readiness := s.computeReadiness(key.project, key.workstream, wsTasks)
		s.allWorkstreams[readiness.Key()] = readiness

		if readiness.IsReady() {
			s.addToReadyQueue(readiness)
		} else if readiness.TotalTaskCount > 0 {
			s.blockedWorkstreams[readiness.Key()] = readiness
		}
	}

	// Build dependent workstreams graph
	s.buildDependentWorkstreams()

	return nil
}

// computeReadiness calculates readiness metrics for a workstream.
func (s *ReadinessAwareScheduler) computeReadiness(project, workstream string, tasks []*task.Task) *WorkstreamReadiness {
	readiness := &WorkstreamReadiness{
		Project:    project,
		Workstream: workstream,
	}

	blockingWSSet := make(map[string]bool)

	for _, t := range tasks {
		// Skip already completed tasks
		if t.Status == task.StatusComplete {
			continue
		}

		readiness.TotalTaskCount++

		blocked := s.resolver.IsBlocked(t.ID)
		if blocked {
			readiness.BlockedTaskCount++

			// Track which workstreams are blocking this one
			blockers, _ := s.resolver.GetBlocking(t.ID)
			for _, blockerID := range blockers {
				blockerWS, ok := s.taskToWorkstream[blockerID]
				if ok && blockerWS != readiness.Key() {
					blockingWSSet[blockerWS] = true
				}
			}
		} else {
			readiness.ReadyTaskCount++
			if readiness.FirstReadyTaskID == "" {
				readiness.FirstReadyTaskID = t.ID
				readiness.FirstReadyTaskPriority = t.Priority
			}
		}
	}

	for ws := range blockingWSSet {
		readiness.BlockingWorkstreams = append(readiness.BlockingWorkstreams, ws)
	}
	sort.Strings(readiness.BlockingWorkstreams)

	return readiness
}

// buildDependentWorkstreams populates the DependentWorkstreams field for each workstream.
func (s *ReadinessAwareScheduler) buildDependentWorkstreams() {
	// For each workstream that is blocked, add this workstream as a dependent of its blockers
	for _, ws := range s.allWorkstreams {
		for _, blockerKey := range ws.BlockingWorkstreams {
			if blocker, ok := s.allWorkstreams[blockerKey]; ok {
				blocker.DependentWorkstreams = appendUnique(blocker.DependentWorkstreams, ws.Key())
			}
		}
	}
}

// addToReadyQueue adds a workstream to the ready queue, maintaining sort order.
func (s *ReadinessAwareScheduler) addToReadyQueue(ws *WorkstreamReadiness) {
	s.readyQueue = append(s.readyQueue, ws)

	// Sort by readiness score (descending)
	sort.Slice(s.readyQueue, func(i, j int) bool {
		return s.readyQueue[i].ReadinessScore() > s.readyQueue[j].ReadinessScore()
	})
}

// GetNextWorkstream returns the best workstream to spawn.
// Returns nil if no ready workstreams are available or concurrency limit is reached.
func (s *ReadinessAwareScheduler) GetNextWorkstream(workstream string) *WorkstreamReadiness {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check concurrency limit for this specific workstream
	limit := s.workstreamConcurrency[workstream]
	if limit <= 0 {
		limit = 1
	}

	activeCount := s.countActiveForWorkstream(workstream)
	if activeCount >= limit {
		return nil // At capacity
	}

	// Find the workstream in the ready queue
	for i, ws := range s.readyQueue {
		if ws.Workstream == workstream {
			// Remove from queue
			s.readyQueue = append(s.readyQueue[:i], s.readyQueue[i+1:]...)
			return ws
		}
	}

	return nil // No ready workstream with this name
}

// GetNextReadyWorkstream returns the next highest priority ready workstream.
// Returns nil if no ready workstreams are available.
func (s *ReadinessAwareScheduler) GetNextReadyWorkstream() *WorkstreamReadiness {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.readyQueue) == 0 {
		return nil
	}

	// Find first workstream that hasn't reached its concurrency limit
	for i, ws := range s.readyQueue {
		limit := s.workstreamConcurrency[ws.Workstream]
		if limit <= 0 {
			limit = 1
		}

		activeCount := s.countActiveForWorkstream(ws.Workstream)
		if activeCount < limit {
			// Remove from queue
			s.readyQueue = append(s.readyQueue[:i], s.readyQueue[i+1:]...)
			return ws
		}
	}

	return nil // All ready workstreams are at capacity
}

// countActiveForWorkstream returns the number of active runners for a workstream.
func (s *ReadinessAwareScheduler) countActiveForWorkstream(workstream string) int {
	if ws, ok := s.activeWorkstreams[workstream]; ok {
		if ws != nil {
			return 1
		}
	}
	return 0
}

// ActivateWorkstream marks a workstream as active.
func (s *ReadinessAwareScheduler) ActivateWorkstream(workstream string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if ws, ok := s.allWorkstreams[workstream]; ok {
		s.activeWorkstreams[workstream] = ws
	}
}

// ReleaseWorkstream marks a workstream as no longer active.
func (s *ReadinessAwareScheduler) ReleaseWorkstream(workstream string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.activeWorkstreams, workstream)
}

// OnTaskComplete is called when a task finishes execution.
// It triggers re-evaluation of blocked workstreams and may make new workstreams ready.
func (s *ReadinessAwareScheduler) OnTaskComplete(_ string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Re-scan tasks to get updated states
	tasks, err := s.taskMgr.Scan()
	if err != nil {
		return
	}

	// Update resolver with new task states
	s.resolver = task.NewResolver(tasks)

	// Find workstreams that may now be unblocked
	var newlyReady []*WorkstreamReadiness

	for key, ws := range s.blockedWorkstreams {
		// Get tasks for this workstream
		wsTasks := s.getTasksForWorkstream(tasks, ws.Project, ws.Workstream)
		newReadiness := s.computeReadiness(ws.Project, ws.Workstream, wsTasks)

		// Update the stored readiness
		s.allWorkstreams[key] = newReadiness

		if newReadiness.IsReady() {
			delete(s.blockedWorkstreams, key)
			newlyReady = append(newlyReady, newReadiness)
		} else {
			s.blockedWorkstreams[key] = newReadiness
		}
	}

	// Add newly ready workstreams to the queue
	for _, ws := range newlyReady {
		s.addToReadyQueue(ws)
		if s.onWorkstreamReady != nil {
			s.onWorkstreamReady(ws)
		}
	}

	// Update active workstreams' readiness info
	for key := range s.activeWorkstreams {
		if ws, ok := s.allWorkstreams[key]; ok {
			wsTasks := s.getTasksForWorkstream(tasks, ws.Project, ws.Workstream)
			newReadiness := s.computeReadiness(ws.Project, ws.Workstream, wsTasks)
			s.allWorkstreams[key] = newReadiness
			s.activeWorkstreams[key] = newReadiness
		}
	}
}

// getTasksForWorkstream filters tasks belonging to a specific workstream.
func (s *ReadinessAwareScheduler) getTasksForWorkstream(tasks []*task.Task, project, workstream string) []*task.Task {
	var result []*task.Task
	for _, t := range tasks {
		if t.Project == project && t.GetWorkstream() == workstream {
			result = append(result, t)
		}
	}
	return result
}

// OnWorkstreamComplete is called when all tasks in a workstream finish.
func (s *ReadinessAwareScheduler) OnWorkstreamComplete(workstream string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.activeWorkstreams, workstream)
}

// SetOnWorkstreamReady sets a callback for when a workstream becomes ready.
func (s *ReadinessAwareScheduler) SetOnWorkstreamReady(fn func(ws *WorkstreamReadiness)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onWorkstreamReady = fn
}

// DetectPotentialDeadlock checks for deadlocks caused by cross-workstream dependencies.
// Returns nil if no deadlock is detected.
func (s *ReadinessAwareScheduler) DetectPotentialDeadlock() *DeadlockInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Check if any workstream has no ready tasks but has blocked tasks
	blockedWorkstreams := make(map[string]bool)
	wsWaitingOn := make(map[string]map[string]bool)

	for _, ws := range s.blockedWorkstreams {
		// Check if this workstream has any ready tasks
		hasReady := false
		for _, readyWS := range s.readyQueue {
			if readyWS.Workstream == ws.Workstream {
				hasReady = true
				break
			}
		}
		if !hasReady {
			blockedWorkstreams[ws.Workstream] = true

			// Track what workstreams this one is waiting on
			if wsWaitingOn[ws.Workstream] == nil {
				wsWaitingOn[ws.Workstream] = make(map[string]bool)
			}

			for _, blockerKey := range ws.BlockingWorkstreams {
				if blockerKey != ws.Workstream {
					wsWaitingOn[ws.Workstream][blockerKey] = true
				}
			}
		}
	}

	// Check for circular waiting between workstreams
	for ws := range blockedWorkstreams {
		for waitingOnWS := range wsWaitingOn[ws] {
			if wsWaitingOn[waitingOnWS][ws] {
				// Check if the other workstream is also fully blocked
				if blockedWorkstreams[waitingOnWS] {
					blockedBy := make(map[string][]string)
					for w, waiting := range wsWaitingOn {
						for dep := range waiting {
							blockedBy[w] = append(blockedBy[w], dep)
						}
					}

					return &DeadlockInfo{
						AffectedWorkstreams: []string{ws, waitingOnWS},
						BlockedBy:           blockedBy,
						Message:             fmt.Sprintf("Workstreams %s and %s are mutually blocked", ws, waitingOnWS),
						Suggestion:          "Increase concurrency for one workstream, or resolve cross-workstream dependencies first",
					}
				}
			}
		}
	}

	return nil
}

// GetReadyWorkstreams returns all ready workstreams.
func (s *ReadinessAwareScheduler) GetReadyWorkstreams() []*WorkstreamReadiness {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*WorkstreamReadiness, len(s.readyQueue))
	copy(result, s.readyQueue)
	return result
}

// GetBlockedWorkstreams returns all blocked workstreams.
func (s *ReadinessAwareScheduler) GetBlockedWorkstreams() []*WorkstreamReadiness {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*WorkstreamReadiness, 0, len(s.blockedWorkstreams))
	for _, ws := range s.blockedWorkstreams {
		result = append(result, ws)
	}
	return result
}

// GetActiveWorkstreams returns all active workstreams.
func (s *ReadinessAwareScheduler) GetActiveWorkstreams() []*WorkstreamReadiness {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*WorkstreamReadiness, 0, len(s.activeWorkstreams))
	for _, ws := range s.activeWorkstreams {
		result = append(result, ws)
	}
	return result
}

// GetAllWorkstreams returns all workstreams.
func (s *ReadinessAwareScheduler) GetAllWorkstreams() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	workstreams := make([]string, 0, len(s.allWorkstreams))
	for ws := range s.allWorkstreams {
		workstreams = append(workstreams, ws)
	}
	sort.Strings(workstreams)
	return workstreams
}

// appendUnique appends a string to a slice only if it's not already present.
func appendUnique(slice []string, s string) []string {
	if slices.Contains(slice, s) {
		return slice
	}
	return append(slice, s)
}
