---
id: TANK-032
title: Task Completion and Validation
status: todo
priority: high
estimate: L
depends_on: [TANK-031]
workstream: C
phase: 3
---

# Task Completion and Validation

## Summary

Implement task completion detection and validation using Ralph-style objective criteria. When an agent finishes work, verify completion via commands or signal detection, then auto-assign the next available task.

## Acceptance Criteria

- [ ] Detect when agent signals task completion
- [ ] Run verification commands (`completion.verify`)
- [ ] Detect completion signals in output (`completion.signal`)
- [ ] Update task status based on validation results
- [ ] Auto-assign next task to idle agent
- [ ] Handle validation failures gracefully
- [ ] Emit events for completion/failure
- [ ] Support max_iterations for Ralph mode
- [ ] Unit tests with 80%+ coverage

## Technical Details

### Completion Flow

```
Agent Output → Signal Detection → Verification → Status Update → Reassignment
                    ↓                  ↓
               "TASK_DONE"      npm test (exit 0?)
                    ↓                  ↓
                 Found?            Passed?
                    ↓                  ↓
                   YES               YES
                    ↓                  ↓
              Mark Complete ← ← ← ← ←
                    ↓
              Assign Next Task
```

### Task Validator

```go
// internal/task/validator.go
package task

import (
    "context"
    "fmt"
    "os/exec"
    "strings"
    "time"
)

type Validator struct {
    workdir string
    timeout time.Duration
}

func NewValidator(workdir string) *Validator {
    return &Validator{
        workdir: workdir,
        timeout: 5 * time.Minute,
    }
}

type ValidationResult struct {
    Task         *Task
    SignalFound  bool
    VerifyPassed bool
    VerifyOutput string
    VerifyError  error
    Status       Status // complete, review, failed
    Message      string
}

// Validate checks if task completion criteria are met
func (v *Validator) Validate(ctx context.Context, t *Task, agentOutput string) *ValidationResult {
    result := &ValidationResult{
        Task:   t,
        Status: StatusReview, // Default: needs human review
    }

    if t.Completion == nil {
        // No completion criteria - needs manual review
        result.Message = "No completion criteria defined"
        return result
    }

    // Check for signal in output
    if t.Completion.Signal != "" {
        result.SignalFound = strings.Contains(agentOutput, t.Completion.Signal)
        if !result.SignalFound {
            result.Status = StatusInProgress // Still working
            result.Message = fmt.Sprintf("Signal %q not found in output", t.Completion.Signal)
            return result
        }
    }

    // Run verify command
    if t.Completion.Verify != "" {
        passed, output, err := v.runVerifyCommand(ctx, t.Completion.Verify)
        result.VerifyPassed = passed
        result.VerifyOutput = output
        result.VerifyError = err

        if err != nil {
            result.Status = StatusFailed
            result.Message = fmt.Sprintf("Verify command failed: %v", err)
            return result
        }

        if !passed {
            result.Status = StatusReview
            result.Message = "Verify command returned non-zero exit code"
            return result
        }
    }

    // Both criteria passed (or only one was specified)
    if (t.Completion.Signal == "" || result.SignalFound) &&
       (t.Completion.Verify == "" || result.VerifyPassed) {
        result.Status = StatusComplete
        result.Message = "All completion criteria met"
    }

    return result
}

func (v *Validator) runVerifyCommand(ctx context.Context, command string) (bool, string, error) {
    ctx, cancel := context.WithTimeout(ctx, v.timeout)
    defer cancel()

    cmd := exec.CommandContext(ctx, "sh", "-c", command)
    cmd.Dir = v.workdir

    output, err := cmd.CombinedOutput()

    if ctx.Err() == context.DeadlineExceeded {
        return false, string(output), fmt.Errorf("command timed out after %v", v.timeout)
    }

    if err != nil {
        if exitErr, ok := err.(*exec.ExitError); ok {
            // Non-zero exit code
            return false, string(output), nil
        }
        return false, string(output), err
    }

    return true, string(output), nil
}
```

### Completion Handler

```go
// internal/task/completion.go
package task

import (
    "context"
    "fmt"
    "log"
    "time"
)

type CompletionHandler struct {
    taskMgr   *Manager
    validator *Validator
    events    chan<- Event
}

type Event struct {
    Type      string    // "complete", "failed", "review", "assigned"
    TaskID    string
    AgentName string
    Message   string
    Timestamp time.Time
}

func NewCompletionHandler(taskMgr *Manager, workdir string, events chan<- Event) *CompletionHandler {
    return &CompletionHandler{
        taskMgr:   taskMgr,
        validator: NewValidator(workdir),
        events:    events,
    }
}

// HandleAgentOutput processes output from an agent working on a task
func (h *CompletionHandler) HandleAgentOutput(ctx context.Context, taskID, agentName, output string) error {
    t, err := h.taskMgr.Get(taskID)
    if err != nil {
        return fmt.Errorf("get task: %w", err)
    }

    result := h.validator.Validate(ctx, t, output)

    // Update task status
    if err := h.taskMgr.UpdateStatus(taskID, result.Status); err != nil {
        return fmt.Errorf("update status: %w", err)
    }

    // Emit event
    h.emitEvent(result, agentName)

    // Handle based on result
    switch result.Status {
    case StatusComplete:
        log.Printf("Task %s completed by %s", taskID, agentName)
        h.taskMgr.Unassign(taskID)
        now := time.Now()
        t.CompletedAt = &now

    case StatusFailed:
        log.Printf("Task %s failed: %s", taskID, result.Message)
        // Keep assigned for retry or manual intervention

    case StatusReview:
        log.Printf("Task %s needs review: %s", taskID, result.Message)
        // Keep assigned, human will review

    case StatusInProgress:
        // Still working, no action needed
    }

    return nil
}

func (h *CompletionHandler) emitEvent(result *ValidationResult, agentName string) {
    if h.events == nil {
        return
    }

    eventType := string(result.Status)
    h.events <- Event{
        Type:      eventType,
        TaskID:    result.Task.ID,
        AgentName: agentName,
        Message:   result.Message,
        Timestamp: time.Now(),
    }
}
```

### Task Runner with Ralph Mode

```go
// internal/task/runner.go
package task

import (
    "context"
    "fmt"
    "log"
    "time"
)

type Runner struct {
    taskMgr    *Manager
    agentMgr   AgentManager
    completion *CompletionHandler
    cooldown   time.Duration
}

type AgentManager interface {
    Run(name, prompt string, opts RunOptions) (string, error)
    Get(name string) (*Agent, error)
}

type RunOptions struct {
    Follow bool
}

func NewRunner(taskMgr *Manager, agentMgr AgentManager, completion *CompletionHandler) *Runner {
    return &Runner{
        taskMgr:    taskMgr,
        agentMgr:   agentMgr,
        completion: completion,
        cooldown:   5 * time.Second,
    }
}

// RunTask executes a task on an agent, using Ralph mode if configured
func (r *Runner) RunTask(ctx context.Context, taskID, agentName string) error {
    t, err := r.taskMgr.Get(taskID)
    if err != nil {
        return fmt.Errorf("get task: %w", err)
    }

    // Update status
    r.taskMgr.UpdateStatus(taskID, StatusInProgress)
    now := time.Now()
    t.StartedAt = &now

    // Build prompt
    prompt := r.buildPrompt(t)

    // Run in Ralph mode if completion criteria defined
    if t.IsRalphMode() {
        return r.runRalphMode(ctx, t, agentName, prompt)
    }

    // Single-shot execution
    output, err := r.agentMgr.Run(agentName, prompt, RunOptions{})
    if err != nil {
        r.taskMgr.UpdateStatus(taskID, StatusFailed)
        return err
    }

    // Check completion
    return r.completion.HandleAgentOutput(ctx, taskID, agentName, output)
}

func (r *Runner) runRalphMode(ctx context.Context, t *Task, agentName, prompt string) error {
    maxIterations := t.Completion.GetMaxIterations()

    for i := 1; i <= maxIterations; i++ {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
        }

        log.Printf("Task %s: iteration %d/%d", t.ID, i, maxIterations)

        // Run agent
        output, err := r.agentMgr.Run(agentName, prompt, RunOptions{})
        if err != nil {
            log.Printf("Task %s: agent error: %v", t.ID, err)
            // Continue trying unless context cancelled
            continue
        }

        // Check completion
        result := r.completion.validator.Validate(ctx, t, output)

        if result.Status == StatusComplete {
            r.taskMgr.UpdateStatus(t.ID, StatusComplete)
            log.Printf("Task %s: completed after %d iterations", t.ID, i)
            return nil
        }

        if result.Status == StatusFailed {
            // Don't retry on hard failures
            r.taskMgr.UpdateStatus(t.ID, StatusFailed)
            return fmt.Errorf("task failed: %s", result.Message)
        }

        // Cooldown before next iteration
        log.Printf("Task %s: not complete, waiting %v before retry", t.ID, r.cooldown)
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-time.After(r.cooldown):
        }
    }

    // Max iterations reached
    r.taskMgr.UpdateStatus(t.ID, StatusReview)
    return fmt.Errorf("max iterations (%d) reached without completion", maxIterations)
}

func (r *Runner) buildPrompt(t *Task) string {
    var prompt strings.Builder

    prompt.WriteString(fmt.Sprintf("# Task: %s\n\n", t.Title))
    prompt.WriteString(t.Content)

    if t.Completion != nil {
        prompt.WriteString("\n\n---\n\n## Completion Instructions\n\n")

        if t.Completion.Verify != "" {
            prompt.WriteString(fmt.Sprintf("**Verify Command:** Run `%s` - it must exit with code 0.\n\n", t.Completion.Verify))
        }

        if t.Completion.Signal != "" {
            prompt.WriteString(fmt.Sprintf("**Completion Signal:** When you are done, output exactly: `%s`\n\n", t.Completion.Signal))
        }

        prompt.WriteString("Do not say you are done until all criteria are met.\n")
    }

    return prompt.String()
}
```

### Auto-Reassignment

```go
// internal/task/reassigner.go
package task

import (
    "context"
    "log"
    "time"
)

type Reassigner struct {
    taskMgr  *Manager
    queue    *Queue
    agentMgr AgentManager
    runner   *Runner
    interval time.Duration
}

func NewReassigner(taskMgr *Manager, queue *Queue, agentMgr AgentManager, runner *Runner) *Reassigner {
    return &Reassigner{
        taskMgr:  taskMgr,
        queue:    queue,
        agentMgr: agentMgr,
        runner:   runner,
        interval: 10 * time.Second,
    }
}

// Start begins the auto-reassignment loop
func (r *Reassigner) Start(ctx context.Context) {
    ticker := time.NewTicker(r.interval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            r.checkAndReassign(ctx)
        }
    }
}

func (r *Reassigner) checkAndReassign(ctx context.Context) {
    // Get all idle agents with roles
    agents, _ := r.agentMgr.List()

    for _, ag := range agents {
        if ag.Status != "idle" || ag.Role == "" {
            continue
        }

        // Try to get next task for this role
        t, err := r.queue.Dequeue(ag.Role)
        if err != nil {
            continue // No tasks for this role
        }

        // Check if task is blocked
        if blocked, _ := r.taskMgr.IsBlocked(t.ID); blocked {
            // Put back in queue
            r.queue.Enqueue(t)
            continue
        }

        // Assign and run
        log.Printf("Auto-assigning %s to %s", t.ID, ag.Name)
        r.taskMgr.Assign(t.ID, ag.Name)

        go func(taskID, agentName string) {
            if err := r.runner.RunTask(ctx, taskID, agentName); err != nil {
                log.Printf("Task %s failed: %v", taskID, err)
            }
        }(t.ID, ag.Name)
    }
}

// OnTaskComplete handles task completion events
func (r *Reassigner) OnTaskComplete(taskID, agentName string) {
    ag, err := r.agentMgr.Get(agentName)
    if err != nil {
        return
    }

    // Immediately try to assign next task
    t, err := r.queue.Dequeue(ag.Role)
    if err != nil {
        log.Printf("Agent %s idle, no more tasks for role %s", agentName, ag.Role)
        return
    }

    log.Printf("Assigning next task %s to %s", t.ID, agentName)
    r.taskMgr.Assign(t.ID, agentName)

    go r.runner.RunTask(context.Background(), t.ID, agentName)
}
```

### Validation Result Types

```go
// Status outcomes from validation
const (
    // StatusComplete - task verified complete
    StatusComplete Status = "complete"

    // StatusReview - needs human review
    StatusReview Status = "review"

    // StatusFailed - hard failure, needs intervention
    StatusFailed Status = "failed"

    // StatusInProgress - still working (signal not found)
    StatusInProgress Status = "in_progress"
)
```

## Testing

### Unit Tests

```go
func TestValidator_Validate(t *testing.T) {
    tests := []struct {
        name       string
        task       *Task
        output     string
        wantStatus Status
    }{
        {
            name: "signal found",
            task: &Task{
                Completion: &CompletionConfig{
                    Signal: "TASK_DONE",
                },
            },
            output:     "Working...\nTASK_DONE\n",
            wantStatus: StatusComplete,
        },
        {
            name: "signal not found",
            task: &Task{
                Completion: &CompletionConfig{
                    Signal: "TASK_DONE",
                },
            },
            output:     "Still working...",
            wantStatus: StatusInProgress,
        },
        {
            name: "no completion config",
            task: &Task{},
            output:     "Done!",
            wantStatus: StatusReview,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            v := NewValidator(t.TempDir())
            result := v.Validate(context.Background(), tt.task, tt.output)
            if result.Status != tt.wantStatus {
                t.Errorf("got status %v, want %v", result.Status, tt.wantStatus)
            }
        })
    }
}

func TestValidator_VerifyCommand(t *testing.T) {
    v := NewValidator(t.TempDir())

    // Test passing command
    passed, output, err := v.runVerifyCommand(context.Background(), "exit 0")
    if !passed || err != nil {
        t.Errorf("exit 0 should pass")
    }

    // Test failing command
    passed, output, err = v.runVerifyCommand(context.Background(), "exit 1")
    if passed {
        t.Errorf("exit 1 should fail")
    }

    // Test command with output
    passed, output, err = v.runVerifyCommand(context.Background(), "echo hello")
    if !passed || !strings.Contains(output, "hello") {
        t.Errorf("echo should pass and capture output")
    }
}

func TestRunner_RalphMode(t *testing.T) {
    // Test that Ralph mode iterates until completion
    // ...
}
```

### Integration Tests

```bash
# Test with verify command
cat > .tanuki/tasks/test-verify.md << 'EOF'
---
id: TEST-001
title: Test Verify
role: backend
completion:
  verify: "test -f /tmp/task-done"
---

Create the file /tmp/task-done
EOF

tanuki project start
# Watch agent create file
tanuki project status  # Should show complete

# Test with signal
cat > .tanuki/tasks/test-signal.md << 'EOF'
---
id: TEST-002
title: Test Signal
role: backend
completion:
  signal: "ALL_DONE"
---

Do something and say ALL_DONE
EOF

tanuki project start
# Watch agent output signal
tanuki project status  # Should show complete
```

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Verify command times out | Mark as review |
| Verify command not found | Mark as failed |
| Signal not in output | Continue (in_progress) |
| Max iterations reached | Mark as review |
| Agent crashes | Mark task as pending, unassign |

## Out of Scope

- Custom validators (script-based)
- Partial completion (checkpoints)
- Automatic retry with backoff
- External notification (Slack, email)

## Notes

The completion system is the core of autonomous operation. It should be reliable and predictable. When in doubt, mark for review rather than incorrectly marking complete.

Ralph mode is powerful but expensive (fresh context each iteration). Use verify commands when possible - they're faster and more reliable than signal detection.
