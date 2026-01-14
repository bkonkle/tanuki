package project

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/bkonkle/tanuki/internal/agent"
	"github.com/bkonkle/tanuki/internal/task"
)

// OrchestratorStatus represents the current state of the orchestrator.
type OrchestratorStatus string

const (
	// StatusStopped indicates the orchestrator is not running.
	StatusStopped OrchestratorStatus = "stopped"
	// StatusStarting indicates the orchestrator is initializing.
	StatusStarting OrchestratorStatus = "starting"
	// StatusRunning indicates the orchestrator is active.
	StatusRunning OrchestratorStatus = "running"
	// StatusStopping indicates the orchestrator is shutting down.
	StatusStopping OrchestratorStatus = "stopping"
)

// OrchestratorConfig configures the orchestrator behavior.
type OrchestratorConfig struct {
	// PollInterval is how often to check for tasks and agents.
	PollInterval time.Duration
	// MaxAgentsPerRole is the maximum agents to spawn per role (deprecated, use RoleConcurrency).
	MaxAgentsPerRole int
	// RoleConcurrency maps role names to their concurrency limits.
	RoleConcurrency map[string]int
	// AutoSpawnAgents enables automatic agent spawning.
	AutoSpawnAgents bool
	// StopWhenComplete stops orchestrator when all tasks complete.
	StopWhenComplete bool
}

// DefaultOrchestratorConfig returns sensible default configuration.
func DefaultOrchestratorConfig() OrchestratorConfig {
	return OrchestratorConfig{
		PollInterval:     10 * time.Second,
		MaxAgentsPerRole: 1,
		RoleConcurrency:  make(map[string]int),
		AutoSpawnAgents:  true,
		StopWhenComplete: false,
	}
}

// GetRoleConcurrency returns the concurrency limit for a role.
// Falls back to MaxAgentsPerRole if no specific limit is set.
func (c *OrchestratorConfig) GetRoleConcurrency(role string) int {
	if c.RoleConcurrency != nil {
		if concurrency, ok := c.RoleConcurrency[role]; ok && concurrency > 0 {
			return concurrency
		}
	}
	if c.MaxAgentsPerRole > 0 {
		return c.MaxAgentsPerRole
	}
	return 1
}

// Orchestrator manages the project lifecycle, coordinating tasks and agents.
type Orchestrator struct {
	// Dependencies (interfaces for loose coupling)
	taskMgr   TaskManager
	agentMgr  AgentManager
	queue     TaskQueue
	balancer  TaskBalancer
	resolver  DependencyResolver
	validator TaskValidator
	runner    TaskRunner

	// Workstream scheduling
	wsScheduler *WorkstreamScheduler

	// State
	mu      sync.RWMutex
	status  OrchestratorStatus
	started time.Time
	events  chan task.Event

	// Config
	config OrchestratorConfig
}

// TaskBalancer selects agents for tasks.
type TaskBalancer interface {
	GetIdleAgents(agents []*AgentInfo, role string) []*AgentInfo
	TrackAssignment(agentName string)
	TrackCompletion(agentName string)
}

// AgentInfo represents agent information for balancing.
type AgentInfo struct {
	Name   string
	Role   string
	Status string // "idle", "working", "stopped"
}

// DependencyResolver checks task dependencies.
type DependencyResolver interface {
	IsBlocked(taskID string) bool
	DetectCycle() []string
}

// TaskValidator validates task completion.
type TaskValidator interface {
	Validate(ctx context.Context, t *task.Task, output string) *task.ValidationResult
}

// TaskRunner executes tasks on agents.
type TaskRunner interface {
	RunTask(ctx context.Context, taskID, agentName string) error
}

// NewOrchestrator creates a new project orchestrator.
func NewOrchestrator(
	taskMgr TaskManager,
	agentMgr AgentManager,
	queue TaskQueue,
	config OrchestratorConfig,
) *Orchestrator {
	wsScheduler := NewWorkstreamScheduler(taskMgr)

	// Initialize role concurrency from config
	for role, concurrency := range config.RoleConcurrency {
		wsScheduler.SetRoleConcurrency(role, concurrency)
	}

	return &Orchestrator{
		taskMgr:     taskMgr,
		agentMgr:    agentMgr,
		queue:       queue,
		wsScheduler: wsScheduler,
		status:      StatusStopped,
		events:      make(chan task.Event, 100),
		config:      config,
	}
}

// SetRoleConcurrency sets the concurrency for a role.
func (o *Orchestrator) SetRoleConcurrency(role string, concurrency int) {
	o.config.RoleConcurrency[role] = concurrency
	o.wsScheduler.SetRoleConcurrency(role, concurrency)
}

// GetWorkstreamScheduler returns the workstream scheduler.
func (o *Orchestrator) GetWorkstreamScheduler() *WorkstreamScheduler {
	return o.wsScheduler
}

// SetBalancer sets the task balancer.
func (o *Orchestrator) SetBalancer(b TaskBalancer) {
	o.balancer = b
}

// SetResolver sets the dependency resolver.
func (o *Orchestrator) SetResolver(r DependencyResolver) {
	o.resolver = r
}

// SetValidator sets the task validator.
func (o *Orchestrator) SetValidator(v TaskValidator) {
	o.validator = v
}

// SetRunner sets the task runner.
func (o *Orchestrator) SetRunner(r TaskRunner) {
	o.runner = r
}

// Start begins the orchestration loop.
func (o *Orchestrator) Start(ctx context.Context) error {
	o.mu.Lock()
	if o.status != StatusStopped {
		o.mu.Unlock()
		return fmt.Errorf("orchestrator already %s", o.status)
	}
	o.status = StatusStarting
	o.started = time.Now()
	o.mu.Unlock()

	log.Println("Starting project orchestrator...")

	// Initialize
	if err := o.initialize(ctx); err != nil {
		o.setStatus(StatusStopped)
		return fmt.Errorf("initialize: %w", err)
	}

	o.setStatus(StatusRunning)
	log.Println("Project orchestrator running")

	// Run main loop
	return o.runLoop(ctx)
}

// Stop gracefully stops the orchestrator.
func (o *Orchestrator) Stop() error {
	o.mu.Lock()
	if o.status != StatusRunning {
		o.mu.Unlock()
		return fmt.Errorf("orchestrator not running")
	}
	o.status = StatusStopping
	o.mu.Unlock()

	log.Println("Stopping project orchestrator...")

	// Stop project agents
	agents, _ := o.agentMgr.List()
	for _, ag := range agents {
		if ag.Role != "" {
			_ = o.agentMgr.Stop(ag.Name)
		}
	}

	o.setStatus(StatusStopped)
	log.Println("Project orchestrator stopped")

	return nil
}

// GetStatus returns the current orchestrator status.
func (o *Orchestrator) GetStatus() *Status {
	o.mu.RLock()
	defer o.mu.RUnlock()

	agents, _ := o.agentMgr.List()

	return &Status{
		Status:     o.status,
		StartedAt:  o.started,
		Uptime:     time.Since(o.started),
		TaskStats:  o.taskMgr.Stats(),
		QueueSize:  o.queue.Size(),
		AgentCount: len(agents),
		IdleAgents: countIdleAgents(agents),
	}
}

// Status contains project status information.
type Status struct {
	Status     OrchestratorStatus
	StartedAt  time.Time
	Uptime     time.Duration
	TaskStats  *TaskStats
	QueueSize  int
	AgentCount int
	IdleAgents int
}

// GetProgress returns detailed progress information.
func (o *Orchestrator) GetProgress() *Progress {
	tasks, _ := o.taskMgr.Scan()

	progress := &Progress{
		Total:    len(tasks),
		ByStatus: make(map[task.Status]int),
		ByRole:   make(map[string]*RoleProgress),
	}

	for _, t := range tasks {
		progress.ByStatus[t.Status]++

		rp, ok := progress.ByRole[t.Role]
		if !ok {
			rp = &RoleProgress{Role: t.Role}
			progress.ByRole[t.Role] = rp
		}
		rp.Total++
		if t.Status == task.StatusComplete {
			rp.Complete++
		}
	}

	progress.Complete = progress.ByStatus[task.StatusComplete]
	progress.InProgress = progress.ByStatus[task.StatusInProgress] + progress.ByStatus[task.StatusAssigned]
	progress.Pending = progress.ByStatus[task.StatusPending] + progress.ByStatus[task.StatusBlocked]

	if progress.Total > 0 {
		progress.Percentage = float64(progress.Complete) / float64(progress.Total) * 100
	}

	return progress
}

// Progress contains detailed progress information.
type Progress struct {
	Total      int
	Complete   int
	InProgress int
	Pending    int
	Percentage float64
	ByStatus   map[task.Status]int
	ByRole     map[string]*RoleProgress
}

// RoleProgress contains progress for a specific role.
type RoleProgress struct {
	Role     string
	Total    int
	Complete int
}

// Events returns the event channel for subscribing to task events.
func (o *Orchestrator) Events() <-chan task.Event {
	return o.events
}

// initialize prepares the orchestrator.
func (o *Orchestrator) initialize(_ context.Context) error {
	// Scan tasks
	tasks, err := o.taskMgr.Scan()
	if err != nil {
		return fmt.Errorf("scan tasks: %w", err)
	}

	if len(tasks) == 0 {
		return fmt.Errorf("no tasks found")
	}

	log.Printf("Found %d tasks", len(tasks))

	// Check for cycles if resolver is set
	if o.resolver != nil {
		if cycle := o.resolver.DetectCycle(); cycle != nil {
			return fmt.Errorf("dependency cycle detected: %v", cycle)
		}
	}

	// Initialize workstream scheduler
	if err := o.wsScheduler.Initialize(); err != nil {
		return fmt.Errorf("initialize workstream scheduler: %w", err)
	}

	wsStats := o.wsScheduler.Stats()
	log.Printf("Workstreams initialized: %d total, %d pending", wsStats.Total, wsStats.ByStatus[WorkstreamPending])

	// Build queue with pending tasks
	for _, t := range tasks {
		if t.Status == task.StatusPending {
			// Skip blocked tasks
			if o.resolver != nil && o.resolver.IsBlocked(t.ID) {
				continue
			}
			_ = o.queue.Enqueue(t)
		}
	}

	log.Printf("Queue initialized with %d pending tasks", o.queue.Size())

	// Spawn agents if configured
	if o.config.AutoSpawnAgents {
		if err := o.spawnAgentsForRoles(tasks); err != nil {
			return fmt.Errorf("spawn agents: %w", err)
		}
	}

	return nil
}

// spawnAgentsForRoles spawns agents for each role that has pending tasks.
// Uses per-role concurrency from config.
func (o *Orchestrator) spawnAgentsForRoles(tasks []*task.Task) error {
	// Collect roles from pending tasks
	roles := make(map[string]bool)
	for _, t := range tasks {
		if t.Status == task.StatusPending || t.Status == task.StatusBlocked {
			roles[t.Role] = true
		}
	}

	// Spawn agents for each role based on concurrency
	for role := range roles {
		concurrency := o.config.GetRoleConcurrency(role)

		for i := 0; i < concurrency; i++ {
			agentName := fmt.Sprintf("%s-agent", role)
			if concurrency > 1 {
				agentName = fmt.Sprintf("%s-agent-%d", role, i+1)
			}

			// Check if exists
			existing, _ := o.agentMgr.Get(agentName)
			if existing != nil {
				log.Printf("Agent %s already exists", agentName)
				continue
			}

			log.Printf("Spawning agent %s for role %s (concurrency: %d)", agentName, role, concurrency)
			// Note: actual spawning would happen here with agentMgr.Spawn
		}
	}

	return nil
}

// runLoop is the main orchestration loop.
func (o *Orchestrator) runLoop(ctx context.Context) error {
	ticker := time.NewTicker(o.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case event := <-o.events:
			o.handleEvent(ctx, event)

		case <-ticker.C:
			o.tick(ctx)
		}

		// Check if complete
		if o.config.StopWhenComplete && o.isComplete() {
			log.Println("All tasks complete")
			return nil
		}
	}
}

// tick performs periodic maintenance.
func (o *Orchestrator) tick(ctx context.Context) {
	// Refresh task states
	tasks, _ := o.taskMgr.Scan()

	// Check for newly unblocked tasks
	for _, t := range tasks {
		if t.Status == task.StatusPending && !o.queue.Contains(t.ID) {
			if o.resolver == nil || !o.resolver.IsBlocked(t.ID) {
				_ = o.queue.Enqueue(t)
				log.Printf("Task %s unblocked, added to queue", t.ID)
			}
		}
	}

	// Assign tasks to idle agents
	o.assignPendingTasks(ctx)
}

// assignPendingTasks assigns tasks to idle agents.
func (o *Orchestrator) assignPendingTasks(ctx context.Context) {
	agents, _ := o.agentMgr.List()

	for _, ag := range agents {
		if ag.Status != "idle" || ag.Role == "" {
			continue
		}

		// Try to get next task for this role
		t, err := o.queue.Dequeue(ag.Role)
		if err != nil {
			continue // No tasks for this role
		}

		// Check if blocked
		if o.resolver != nil && o.resolver.IsBlocked(t.ID) {
			_ = o.queue.Enqueue(t) // Put back
			continue
		}

		// Assign
		o.assignTask(ctx, t, ag.Name)
	}
}

// assignTask assigns a task to an agent and starts execution.
func (o *Orchestrator) assignTask(ctx context.Context, t *task.Task, agentName string) {
	log.Printf("Assigning %s to %s", t.ID, agentName)

	_ = o.taskMgr.Assign(t.ID, agentName)

	if o.balancer != nil {
		o.balancer.TrackAssignment(agentName)
	}

	// Start task execution if runner is set
	if o.runner != nil {
		go func() {
			if err := o.runner.RunTask(ctx, t.ID, agentName); err != nil {
				log.Printf("Task %s failed: %v", t.ID, err)
				o.events <- task.Event{
					Type:      task.EventTaskFailed,
					TaskID:    t.ID,
					AgentName: agentName,
					Message:   err.Error(),
					Timestamp: time.Now(),
				}
			} else {
				o.events <- task.Event{
					Type:      task.EventTaskCompleted,
					TaskID:    t.ID,
					AgentName: agentName,
					Timestamp: time.Now(),
				}
			}
		}()
	}
}

// handleEvent processes task events.
func (o *Orchestrator) handleEvent(ctx context.Context, event task.Event) {
	log.Printf("Event: %s for task %s", event.Type, event.TaskID)

	switch event.Type {
	case task.EventTaskCompleted:
		o.onTaskComplete(ctx, event)

	case task.EventTaskFailed:
		o.onTaskFailed(ctx, event)

	case task.EventTaskBlocked:
		o.onTaskBlocked(event)
	}
}

// onTaskComplete handles task completion.
func (o *Orchestrator) onTaskComplete(ctx context.Context, event task.Event) {
	// Update balancer
	if o.balancer != nil {
		o.balancer.TrackCompletion(event.AgentName)
	}

	// Unassign task
	_ = o.taskMgr.Unassign(event.TaskID)

	// Update workstream scheduler
	if err := o.wsScheduler.CompleteTask(event.TaskID); err != nil {
		log.Printf("Warning: failed to update workstream for task %s: %v", event.TaskID, err)
	}

	// Check for newly unblocked tasks
	tasks, _ := o.taskMgr.Scan()
	for _, t := range tasks {
		if t.Status == task.StatusBlocked {
			if o.resolver == nil || !o.resolver.IsBlocked(t.ID) {
				_ = o.taskMgr.UpdateStatus(t.ID, task.StatusPending)
				_ = o.queue.Enqueue(t)
				log.Printf("Task %s unblocked by completion of %s", t.ID, event.TaskID)
			}
		}
	}

	// Agent is now idle - assign next task
	o.assignPendingTasks(ctx)
}

// onTaskFailed handles task failure.
func (o *Orchestrator) onTaskFailed(ctx context.Context, event task.Event) {
	// Update balancer
	if o.balancer != nil {
		o.balancer.TrackCompletion(event.AgentName)
	}

	// Log failure
	log.Printf("Task %s failed: %s", event.TaskID, event.Message)

	// Update workstream scheduler - marks entire workstream as failed
	if err := o.wsScheduler.FailTask(event.TaskID); err != nil {
		log.Printf("Warning: failed to update workstream for failed task %s: %v", event.TaskID, err)
	}

	// Task stays failed, agent becomes idle
	// assignPendingTasks will pick up next task for idle agent
	o.assignPendingTasks(ctx)
}

// onTaskBlocked handles task becoming blocked.
func (o *Orchestrator) onTaskBlocked(event task.Event) {
	log.Printf("Task %s became blocked", event.TaskID)

	// Update status
	_ = o.taskMgr.UpdateStatus(event.TaskID, task.StatusBlocked)
}

// isComplete checks if all tasks are complete.
func (o *Orchestrator) isComplete() bool {
	tasks, _ := o.taskMgr.Scan()

	for _, t := range tasks {
		switch t.Status {
		case task.StatusPending, task.StatusAssigned, task.StatusInProgress, task.StatusBlocked:
			return false
		}
	}

	return true
}

// setStatus updates the orchestrator status.
func (o *Orchestrator) setStatus(status OrchestratorStatus) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.status = status
}

// countIdleAgents counts agents with idle status.
func countIdleAgents(agents []*agent.Agent) int {
	count := 0
	for _, ag := range agents {
		if ag.Status == "idle" {
			count++
		}
	}
	return count
}
