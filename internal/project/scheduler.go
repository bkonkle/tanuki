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

	// Role this workstream belongs to
	Role string

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
	return workstreamKey(wr.Role, wr.Workstream)
}

// DeadlockInfo contains information about a detected deadlock.
type DeadlockInfo struct {
	// AffectedRoles lists roles involved in the deadlock
	AffectedRoles []string

	// BlockedBy maps each role to the roles it's waiting on
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

	// roleConcurrency maps role names to their concurrency limits
	roleConcurrency map[string]int

	// activeWorkstreams tracks currently running workstreams
	// Key: "role:workstream"
	activeWorkstreams map[string]*WorkstreamReadiness

	// readyQueues holds sorted lists of ready workstreams by role
	readyQueues map[string][]*WorkstreamReadiness

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
		taskMgr:            taskMgr,
		roleConcurrency:    make(map[string]int),
		activeWorkstreams:  make(map[string]*WorkstreamReadiness),
		readyQueues:        make(map[string][]*WorkstreamReadiness),
		blockedWorkstreams: make(map[string]*WorkstreamReadiness),
		allWorkstreams:     make(map[string]*WorkstreamReadiness),
		taskToWorkstream:   make(map[string]string),
	}
}

// SetRoleConcurrency sets the concurrency limit for a role.
func (s *ReadinessAwareScheduler) SetRoleConcurrency(role string, limit int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit <= 0 {
		limit = 1
	}
	s.roleConcurrency[role] = limit
}

// GetRoleConcurrency returns the concurrency limit for a role.
func (s *ReadinessAwareScheduler) GetRoleConcurrency(role string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if c, ok := s.roleConcurrency[role]; ok {
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

	// Group tasks by project/role/workstream
	type wsKey struct {
		project    string
		role       string
		workstream string
	}
	workstreamTasks := make(map[wsKey][]*task.Task)

	for _, t := range tasks {
		key := wsKey{
			project:    t.Project,
			role:       t.Role,
			workstream: t.GetWorkstream(),
		}
		workstreamTasks[key] = append(workstreamTasks[key], t)

		// Map task to workstream
		wsKeyStr := workstreamKey(t.Role, t.GetWorkstream())
		s.taskToWorkstream[t.ID] = wsKeyStr
	}

	// Build readiness info for each workstream
	for key, wsTasks := range workstreamTasks {
		readiness := s.computeReadiness(key.project, key.role, key.workstream, wsTasks)
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
func (s *ReadinessAwareScheduler) computeReadiness(project, role, workstream string, tasks []*task.Task) *WorkstreamReadiness {
	readiness := &WorkstreamReadiness{
		Project:    project,
		Role:       role,
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

// addToReadyQueue adds a workstream to the ready queue for its role, maintaining sort order.
func (s *ReadinessAwareScheduler) addToReadyQueue(ws *WorkstreamReadiness) {
	queue := s.readyQueues[ws.Role]
	queue = append(queue, ws)

	// Sort by readiness score (descending)
	sort.Slice(queue, func(i, j int) bool {
		return queue[i].ReadinessScore() > queue[j].ReadinessScore()
	})

	s.readyQueues[ws.Role] = queue
}

// GetNextWorkstream returns the best workstream to spawn for a role.
// Returns nil if no ready workstreams are available or concurrency limit is reached.
func (s *ReadinessAwareScheduler) GetNextWorkstream(role string) *WorkstreamReadiness {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check concurrency limit
	limit := s.roleConcurrency[role]
	if limit <= 0 {
		limit = 1
	}

	activeCount := s.countActiveForRole(role)
	if activeCount >= limit {
		return nil // At capacity
	}

	// Get ready queue for this role
	queue := s.readyQueues[role]
	if len(queue) == 0 {
		return nil // No ready workstreams
	}

	// Pop highest priority ready workstream
	best := queue[0]
	s.readyQueues[role] = queue[1:]

	return best
}

// countActiveForRole returns the number of active workstreams for a role.
func (s *ReadinessAwareScheduler) countActiveForRole(role string) int {
	count := 0
	for _, ws := range s.activeWorkstreams {
		if ws.Role == role {
			count++
		}
	}
	return count
}

// ActivateWorkstream marks a workstream as active.
func (s *ReadinessAwareScheduler) ActivateWorkstream(role, workstream string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := workstreamKey(role, workstream)
	if ws, ok := s.allWorkstreams[key]; ok {
		s.activeWorkstreams[key] = ws
	}
}

// ReleaseWorkstream marks a workstream as no longer active.
func (s *ReadinessAwareScheduler) ReleaseWorkstream(role, workstream string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := workstreamKey(role, workstream)
	delete(s.activeWorkstreams, key)
}

// OnTaskComplete is called when a task finishes execution.
// It triggers re-evaluation of blocked workstreams and may make new workstreams ready.
func (s *ReadinessAwareScheduler) OnTaskComplete(taskID string) {
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
		wsTasks := s.getTasksForWorkstream(tasks, ws.Project, ws.Role, ws.Workstream)
		newReadiness := s.computeReadiness(ws.Project, ws.Role, ws.Workstream, wsTasks)

		// Update the stored readiness
		s.allWorkstreams[key] = newReadiness

		if newReadiness.IsReady() {
			delete(s.blockedWorkstreams, key)
			newlyReady = append(newlyReady, newReadiness)
		} else {
			s.blockedWorkstreams[key] = newReadiness
		}
	}

	// Add newly ready workstreams to their queues
	for _, ws := range newlyReady {
		s.addToReadyQueue(ws)
		if s.onWorkstreamReady != nil {
			s.onWorkstreamReady(ws)
		}
	}

	// Update active workstreams' readiness info
	for key := range s.activeWorkstreams {
		if ws, ok := s.allWorkstreams[key]; ok {
			wsTasks := s.getTasksForWorkstream(tasks, ws.Project, ws.Role, ws.Workstream)
			newReadiness := s.computeReadiness(ws.Project, ws.Role, ws.Workstream, wsTasks)
			s.allWorkstreams[key] = newReadiness
			s.activeWorkstreams[key] = newReadiness
		}
	}
}

// getTasksForWorkstream filters tasks belonging to a specific workstream.
func (s *ReadinessAwareScheduler) getTasksForWorkstream(tasks []*task.Task, project, role, workstream string) []*task.Task {
	var result []*task.Task
	for _, t := range tasks {
		if t.Project == project && t.Role == role && t.GetWorkstream() == workstream {
			result = append(result, t)
		}
	}
	return result
}

// OnWorkstreamComplete is called when all tasks in a workstream finish.
func (s *ReadinessAwareScheduler) OnWorkstreamComplete(role, workstream string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := workstreamKey(role, workstream)
	delete(s.activeWorkstreams, key)
}

// SetOnWorkstreamReady sets a callback for when a workstream becomes ready.
func (s *ReadinessAwareScheduler) SetOnWorkstreamReady(fn func(ws *WorkstreamReadiness)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onWorkstreamReady = fn
}

// DetectPotentialDeadlock checks for deadlocks caused by cross-role dependencies.
// Returns nil if no deadlock is detected.
func (s *ReadinessAwareScheduler) DetectPotentialDeadlock() *DeadlockInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Check if any role has no ready workstreams but has blocked workstreams
	rolesWithOnlyBlocked := make(map[string]bool)
	roleWaitingOn := make(map[string]map[string]bool)

	for _, ws := range s.blockedWorkstreams {
		// Check if this role has any ready workstreams
		hasReady := len(s.readyQueues[ws.Role]) > 0
		if !hasReady {
			rolesWithOnlyBlocked[ws.Role] = true

			// Track what roles this role is waiting on
			if roleWaitingOn[ws.Role] == nil {
				roleWaitingOn[ws.Role] = make(map[string]bool)
			}

			for _, blockerKey := range ws.BlockingWorkstreams {
				if blocker, ok := s.allWorkstreams[blockerKey]; ok {
					if blocker.Role != ws.Role {
						roleWaitingOn[ws.Role][blocker.Role] = true
					}
				}
			}
		}
	}

	// Check for circular waiting between roles
	for role := range rolesWithOnlyBlocked {
		for waitingOnRole := range roleWaitingOn[role] {
			if roleWaitingOn[waitingOnRole][role] {
				// Check if both roles are fully blocked
				otherHasReady := len(s.readyQueues[waitingOnRole]) > 0
				if !otherHasReady && rolesWithOnlyBlocked[waitingOnRole] {
					blockedBy := make(map[string][]string)
					for r, waiting := range roleWaitingOn {
						for w := range waiting {
							blockedBy[r] = append(blockedBy[r], w)
						}
					}

					return &DeadlockInfo{
						AffectedRoles: []string{role, waitingOnRole},
						BlockedBy:     blockedBy,
						Message:       fmt.Sprintf("Roles %s and %s are mutually blocked", role, waitingOnRole),
						Suggestion:    "Increase concurrency for one role, or resolve cross-role dependencies first",
					}
				}
			}
		}
	}

	return nil
}

// GetReadyWorkstreams returns all ready workstreams for a role.
func (s *ReadinessAwareScheduler) GetReadyWorkstreams(role string) []*WorkstreamReadiness {
	s.mu.RLock()
	defer s.mu.RUnlock()

	queue := s.readyQueues[role]
	result := make([]*WorkstreamReadiness, len(queue))
	copy(result, queue)
	return result
}

// GetBlockedWorkstreams returns all blocked workstreams for a role.
func (s *ReadinessAwareScheduler) GetBlockedWorkstreams(role string) []*WorkstreamReadiness {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*WorkstreamReadiness
	for _, ws := range s.blockedWorkstreams {
		if ws.Role == role {
			result = append(result, ws)
		}
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

// GetAllRoles returns all roles that have workstreams.
func (s *ReadinessAwareScheduler) GetAllRoles() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	roleSet := make(map[string]bool)
	for _, ws := range s.allWorkstreams {
		roleSet[ws.Role] = true
	}

	roles := make([]string, 0, len(roleSet))
	for role := range roleSet {
		roles = append(roles, role)
	}
	sort.Strings(roles)
	return roles
}

// appendUnique appends a string to a slice only if it's not already present.
func appendUnique(slice []string, s string) []string {
	if slices.Contains(slice, s) {
		return slice
	}
	return append(slice, s)
}
