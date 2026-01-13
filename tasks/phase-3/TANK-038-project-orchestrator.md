---
id: TANK-038
title: Project Orchestrator
status: todo
priority: high
estimate: L
depends_on: [TANK-031, TANK-032, TANK-036]
workstream: C
phase: 3
---

# Project Orchestrator

## Summary

Implement the central orchestration loop that coordinates all Phase 3 components. The orchestrator manages the project lifecycle, handles events, and ensures continuous task distribution until all work is complete.

## Acceptance Criteria

- [ ] Main control loop with event handling
- [ ] Coordinate TaskManager, Queue, Balancer, Validator
- [ ] Handle task completion → reassignment flow
- [ ] Handle agent idle → task assignment flow
- [ ] Graceful startup and shutdown
- [ ] Status reporting and progress tracking
- [ ] Error recovery and logging
- [ ] Integration tests for full workflow
- [ ] Unit tests with 80%+ coverage

## Technical Details

### Orchestrator Interface

```go
// internal/project/orchestrator.go
package project

import (
    "context"
    "fmt"
    "log"
    "sync"
    "time"

    "tanuki/internal/agent"
    "tanuki/internal/task"
)

// Orchestrator manages the project lifecycle
type Orchestrator struct {
    // Dependencies
    taskMgr   *task.Manager
    agentMgr  *agent.Manager
    queue     *task.Queue
    balancer  *task.Balancer
    resolver  *task.Resolver
    validator *task.Validator
    tracker   *task.StatusTracker

    // State
    mu      sync.RWMutex
    status  OrchestratorStatus
    started time.Time
    events  chan task.Event

    // Config
    config OrchestratorConfig
}

type OrchestratorStatus string

const (
    StatusStopped  OrchestratorStatus = "stopped"
    StatusStarting OrchestratorStatus = "starting"
    StatusRunning  OrchestratorStatus = "running"
    StatusStopping OrchestratorStatus = "stopping"
)

type OrchestratorConfig struct {
    PollInterval      time.Duration
    MaxAgentsPerRole  int
    AutoSpawnAgents   bool
    StopWhenComplete  bool
}

func DefaultConfig() OrchestratorConfig {
    return OrchestratorConfig{
        PollInterval:      10 * time.Second,
        MaxAgentsPerRole:  1,
        AutoSpawnAgents:   true,
        StopWhenComplete:  false,
    }
}

// NewOrchestrator creates a new project orchestrator
func NewOrchestrator(
    taskMgr *task.Manager,
    agentMgr *agent.Manager,
    config OrchestratorConfig,
) *Orchestrator {
    return &Orchestrator{
        taskMgr:   taskMgr,
        agentMgr:  agentMgr,
        queue:     task.NewQueue(),
        balancer:  task.NewBalancer(),
        tracker:   task.NewStatusTracker(),
        status:    StatusStopped,
        events:    make(chan task.Event, 100),
        config:    config,
    }
}
```

### Lifecycle Management

```go
// Start begins the orchestration loop
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
    if err := o.initialize(); err != nil {
        o.setStatus(StatusStopped)
        return fmt.Errorf("initialize: %w", err)
    }

    o.setStatus(StatusRunning)
    log.Println("Project orchestrator running")

    // Run main loop
    return o.runLoop(ctx)
}

// Stop gracefully stops the orchestrator
func (o *Orchestrator) Stop() error {
    o.mu.Lock()
    if o.status != StatusRunning {
        o.mu.Unlock()
        return fmt.Errorf("orchestrator not running")
    }
    o.status = StatusStopping
    o.mu.Unlock()

    log.Println("Stopping project orchestrator...")

    // Stop agents
    agents, _ := o.agentMgr.List()
    for _, ag := range agents {
        if ag.Role != "" {
            o.agentMgr.Stop(ag.Name)
        }
    }

    o.setStatus(StatusStopped)
    log.Println("Project orchestrator stopped")

    return nil
}

func (o *Orchestrator) setStatus(status OrchestratorStatus) {
    o.mu.Lock()
    defer o.mu.Unlock()
    o.status = status
}

// Status returns current orchestrator status
func (o *Orchestrator) Status() *ProjectStatus {
    o.mu.RLock()
    defer o.mu.RUnlock()

    tasks, _ := o.taskMgr.Scan()
    agents, _ := o.agentMgr.List()

    return &ProjectStatus{
        Status:      o.status,
        StartedAt:   o.started,
        Uptime:      time.Since(o.started),
        TaskStats:   o.taskMgr.Stats(),
        QueueSize:   o.queue.Size(),
        AgentCount:  len(agents),
        IdleAgents:  countIdleAgents(agents),
    }
}

type ProjectStatus struct {
    Status      OrchestratorStatus
    StartedAt   time.Time
    Uptime      time.Duration
    TaskStats   *task.TaskStats
    QueueSize   int
    AgentCount  int
    IdleAgents  int
}
```

### Initialization

```go
func (o *Orchestrator) initialize() error {
    // Scan tasks
    tasks, err := o.taskMgr.Scan()
    if err != nil {
        return fmt.Errorf("scan tasks: %w", err)
    }

    if len(tasks) == 0 {
        return fmt.Errorf("no tasks found")
    }

    log.Printf("Found %d tasks", len(tasks))

    // Build resolver
    o.resolver = task.NewResolver(tasks)

    // Check for cycles
    if cycle := o.resolver.DetectCycle(); cycle != nil {
        return fmt.Errorf("dependency cycle: %v", cycle)
    }

    // Update blocked status
    o.taskMgr.UpdateBlockedStatus()

    // Build queue with pending tasks
    for _, t := range tasks {
        if t.Status == task.StatusPending {
            o.queue.Enqueue(t)
        }
    }

    log.Printf("Queue initialized with %d pending tasks", o.queue.Size())

    // Spawn agents if configured
    if o.config.AutoSpawnAgents {
        if err := o.spawnAgentsForRoles(tasks); err != nil {
            return fmt.Errorf("spawn agents: %w", err)
        }
    }

    // Initial task assignment
    o.assignPendingTasks()

    return nil
}

func (o *Orchestrator) spawnAgentsForRoles(tasks []*task.Task) error {
    // Collect roles from pending tasks
    roles := make(map[string]bool)
    for _, t := range tasks {
        if t.Status == task.StatusPending || t.Status == task.StatusBlocked {
            roles[t.Role] = true
        }
    }

    // Spawn agent for each role
    for role := range roles {
        agentName := fmt.Sprintf("%s-agent", role)

        // Check if exists
        existing, _ := o.agentMgr.Get(agentName)
        if existing != nil {
            if existing.Status == "stopped" {
                log.Printf("Starting existing agent %s", agentName)
                o.agentMgr.Start(agentName)
            }
            continue
        }

        log.Printf("Spawning agent %s for role %s", agentName, role)
        _, err := o.agentMgr.Spawn(agentName, agent.SpawnOptions{Role: role})
        if err != nil {
            log.Printf("Failed to spawn %s: %v", agentName, err)
            // Continue with other roles
        }
    }

    return nil
}
```

### Main Loop

```go
func (o *Orchestrator) runLoop(ctx context.Context) error {
    ticker := time.NewTicker(o.config.PollInterval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return ctx.Err()

        case event := <-o.events:
            o.handleEvent(event)

        case <-ticker.C:
            o.tick()
        }

        // Check if complete
        if o.config.StopWhenComplete && o.isComplete() {
            log.Println("All tasks complete")
            return nil
        }
    }
}

func (o *Orchestrator) tick() {
    // Refresh task status
    o.taskMgr.UpdateBlockedStatus()

    // Check for newly unblocked tasks
    tasks, _ := o.taskMgr.Scan()
    for _, t := range tasks {
        if t.Status == task.StatusPending && !o.queue.Contains(t.ID) {
            if !o.resolver.IsBlocked(t.ID) {
                o.queue.Enqueue(t)
                log.Printf("Task %s unblocked, added to queue", t.ID)
            }
        }
    }

    // Assign tasks to idle agents
    o.assignPendingTasks()
}

func (o *Orchestrator) assignPendingTasks() {
    agents, _ := o.agentMgr.List()

    for _, ag := range o.balancer.GetIdleAgents(agents, "") {
        if ag.Role == "" {
            continue
        }

        // Get next task for role
        t, err := o.queue.Dequeue(ag.Role)
        if err != nil {
            continue // No tasks for this role
        }

        // Check if blocked
        if o.resolver.IsBlocked(t.ID) {
            o.queue.Enqueue(t) // Put back
            continue
        }

        // Assign
        o.assignTask(t, ag)
    }
}

func (o *Orchestrator) assignTask(t *task.Task, ag *agent.Agent) {
    log.Printf("Assigning %s to %s", t.ID, ag.Name)

    o.taskMgr.Assign(t.ID, ag.Name)
    o.balancer.TrackAssignment(ag.Name)
    o.tracker.RecordChange(t.ID, t.Status, task.StatusAssigned, ag.Name, "")

    // Start task execution
    go o.executeTask(t, ag.Name)
}

func (o *Orchestrator) executeTask(t *task.Task, agentName string) {
    // Build prompt
    prompt := o.buildTaskPrompt(t)

    // Update to in_progress
    o.taskMgr.UpdateStatus(t.ID, task.StatusInProgress)
    o.tracker.RecordChange(t.ID, task.StatusAssigned, task.StatusInProgress, agentName, "")

    // Execute
    runner := task.NewRunner(o.taskMgr, o.agentMgr, o.validator)
    err := runner.RunTask(context.Background(), t.ID, agentName)

    if err != nil {
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
}

func (o *Orchestrator) buildTaskPrompt(t *task.Task) string {
    var prompt strings.Builder

    prompt.WriteString(fmt.Sprintf("# Task: %s\n\n", t.Title))
    prompt.WriteString(t.Content)

    if t.Completion != nil {
        prompt.WriteString("\n\n---\n\n## Completion Criteria\n\n")
        if t.Completion.Verify != "" {
            prompt.WriteString(fmt.Sprintf("Verify: `%s` must exit 0\n", t.Completion.Verify))
        }
        if t.Completion.Signal != "" {
            prompt.WriteString(fmt.Sprintf("Signal: Say `%s` when done\n", t.Completion.Signal))
        }
    }

    return prompt.String()
}
```

### Event Handling

```go
func (o *Orchestrator) handleEvent(event task.Event) {
    log.Printf("Event: %s for task %s", event.Type, event.TaskID)

    switch event.Type {
    case task.EventTaskCompleted:
        o.onTaskComplete(event)

    case task.EventTaskFailed:
        o.onTaskFailed(event)

    case task.EventTaskBlocked:
        o.onTaskBlocked(event)
    }
}

func (o *Orchestrator) onTaskComplete(event task.Event) {
    // Update balancer
    o.balancer.TrackCompletion(event.AgentName)

    // Unassign task
    o.taskMgr.Unassign(event.TaskID)

    // Check for newly unblocked tasks
    tasks, _ := o.taskMgr.Scan()
    for _, t := range tasks {
        if t.Status == task.StatusBlocked {
            if !o.resolver.IsBlocked(t.ID) {
                o.taskMgr.UpdateStatus(t.ID, task.StatusPending)
                o.queue.Enqueue(t)
                log.Printf("Task %s unblocked by completion of %s", t.ID, event.TaskID)
            }
        }
    }

    // Agent is now idle - assign next task
    o.assignPendingTasks()
}

func (o *Orchestrator) onTaskFailed(event task.Event) {
    // Update balancer
    o.balancer.TrackCompletion(event.AgentName)

    // Log failure
    log.Printf("Task %s failed: %s", event.TaskID, event.Message)

    // Task stays failed, agent becomes idle
    // assignPendingTasks will pick up next task for idle agent
    o.assignPendingTasks()
}

func (o *Orchestrator) onTaskBlocked(event task.Event) {
    // Task was blocked during execution (shouldn't happen normally)
    log.Printf("Task %s became blocked", event.TaskID)

    // Update status
    o.taskMgr.UpdateStatus(event.TaskID, task.StatusBlocked)
}
```

### Completion Check

```go
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

func countIdleAgents(agents []*agent.Agent) int {
    count := 0
    for _, ag := range agents {
        if ag.Status == "idle" {
            count++
        }
    }
    return count
}
```

### Progress Reporting

```go
// GetProgress returns detailed progress information
func (o *Orchestrator) GetProgress() *ProjectProgress {
    tasks, _ := o.taskMgr.Scan()

    progress := &ProjectProgress{
        Total:       len(tasks),
        ByStatus:    make(map[task.Status]int),
        ByRole:      make(map[string]*RoleProgress),
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

type ProjectProgress struct {
    Total       int
    Complete    int
    InProgress  int
    Pending     int
    Percentage  float64
    ByStatus    map[task.Status]int
    ByRole      map[string]*RoleProgress
}

type RoleProgress struct {
    Role     string
    Total    int
    Complete int
}
```

## Testing

### Unit Tests

```go
func TestOrchestrator_Initialize(t *testing.T) {
    // Setup mocks
    taskMgr := newMockTaskManager([]*task.Task{
        {ID: "T1", Role: "backend", Status: task.StatusPending},
        {ID: "T2", Role: "frontend", Status: task.StatusPending},
    })
    agentMgr := newMockAgentManager()

    orch := NewOrchestrator(taskMgr, agentMgr, DefaultConfig())

    err := orch.initialize()
    if err != nil {
        t.Fatalf("initialize() error: %v", err)
    }

    // Queue should have 2 tasks
    if orch.queue.Size() != 2 {
        t.Errorf("Queue size = %d, want 2", orch.queue.Size())
    }
}

func TestOrchestrator_AssignTask(t *testing.T) {
    // Setup
    taskMgr := newMockTaskManager([]*task.Task{
        {ID: "T1", Role: "backend", Status: task.StatusPending},
    })
    agentMgr := newMockAgentManager()
    agentMgr.AddAgent(&agent.Agent{Name: "be-1", Role: "backend", Status: "idle"})

    orch := NewOrchestrator(taskMgr, agentMgr, DefaultConfig())
    orch.initialize()

    // Should have assigned T1 to be-1
    task, _ := taskMgr.Get("T1")
    if task.AssignedTo != "be-1" {
        t.Errorf("Task assigned to %q, want be-1", task.AssignedTo)
    }
}

func TestOrchestrator_IsComplete(t *testing.T) {
    tests := []struct {
        name     string
        tasks    []*task.Task
        complete bool
    }{
        {
            name: "all complete",
            tasks: []*task.Task{
                {ID: "T1", Status: task.StatusComplete},
                {ID: "T2", Status: task.StatusComplete},
            },
            complete: true,
        },
        {
            name: "some pending",
            tasks: []*task.Task{
                {ID: "T1", Status: task.StatusComplete},
                {ID: "T2", Status: task.StatusPending},
            },
            complete: false,
        },
        {
            name: "some in progress",
            tasks: []*task.Task{
                {ID: "T1", Status: task.StatusComplete},
                {ID: "T2", Status: task.StatusInProgress},
            },
            complete: false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            taskMgr := newMockTaskManager(tt.tasks)
            agentMgr := newMockAgentManager()

            orch := NewOrchestrator(taskMgr, agentMgr, DefaultConfig())
            orch.taskMgr = taskMgr

            if orch.isComplete() != tt.complete {
                t.Errorf("isComplete() = %v, want %v", orch.isComplete(), tt.complete)
            }
        })
    }
}
```

### Integration Tests

```bash
#!/bin/bash
# Integration test for full workflow

# Setup
mkdir -p .tanuki/tasks
cat > .tanuki/tasks/TASK-001.md << 'EOF'
---
id: TASK-001
title: Create File
role: backend
completion:
  verify: "test -f /tmp/tanuki-test-file"
---

Create /tmp/tanuki-test-file
EOF

cat > .tanuki/tasks/TASK-002.md << 'EOF'
---
id: TASK-002
title: Verify File
role: qa
depends_on: [TASK-001]
completion:
  verify: "test -f /tmp/tanuki-test-file"
---

Verify /tmp/tanuki-test-file exists
EOF

# Start project
tanuki project start

# Wait for completion
sleep 60

# Check status
tanuki project status

# Verify both complete
STATUS=$(tanuki project status --json | jq '.tasks[] | select(.status != "complete") | .id')
if [ -n "$STATUS" ]; then
    echo "FAIL: Not all tasks complete"
    exit 1
fi

echo "PASS: All tasks complete"

# Cleanup
rm /tmp/tanuki-test-file
tanuki project stop --remove
```

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Task scan fails | Return error, don't start |
| Dependency cycle | Return error, don't start |
| Agent spawn fails | Log warning, continue |
| Task execution fails | Mark failed, continue with others |
| Agent crashes | Task becomes pending, reassign |

## Out of Scope

- Multiple orchestrators (single instance only)
- Orchestrator high availability
- Cross-project coordination
- Remote/distributed agents

## Notes

The orchestrator is the integration point for all Phase 3 components. It runs as a long-lived process that continuously monitors and manages task distribution.

The event-driven architecture allows responsive handling of task completion while the polling loop catches any missed events or state changes.
