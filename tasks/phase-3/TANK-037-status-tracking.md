---
id: TANK-037
title: Status Tracking
status: todo
priority: medium
estimate: S
depends_on: [TANK-030]
workstream: B
phase: 3
---

# Status Tracking

## Summary

Implement comprehensive status tracking for tasks, including status history, events, and real-time updates. This provides visibility into task lifecycle and enables monitoring features.

## Acceptance Criteria

- [ ] Track status transitions with timestamps
- [ ] Event system for status changes
- [ ] Status history per task
- [ ] Query tasks by status
- [ ] Status change validation (valid transitions only)
- [ ] Event listeners for real-time updates
- [ ] Unit tests with 80%+ coverage

## Technical Details

### Status Tracker

```go
// internal/task/tracker.go
package task

import (
    "fmt"
    "sync"
    "time"
)

// StatusTracker tracks task status changes and history
type StatusTracker struct {
    mu       sync.RWMutex
    history  map[string][]StatusChange // taskID -> history
    listeners []StatusListener
}

// StatusChange represents a single status transition
type StatusChange struct {
    TaskID    string
    From      Status
    To        Status
    Timestamp time.Time
    AgentName string
    Message   string
}

// StatusListener receives status change notifications
type StatusListener func(change StatusChange)

// NewStatusTracker creates a new tracker
func NewStatusTracker() *StatusTracker {
    return &StatusTracker{
        history: make(map[string][]StatusChange),
    }
}

// RecordChange records a status change for a task
func (st *StatusTracker) RecordChange(taskID string, from, to Status, agentName, message string) error {
    // Validate transition
    if err := validateTransition(from, to); err != nil {
        return err
    }

    change := StatusChange{
        TaskID:    taskID,
        From:      from,
        To:        to,
        Timestamp: time.Now(),
        AgentName: agentName,
        Message:   message,
    }

    st.mu.Lock()
    st.history[taskID] = append(st.history[taskID], change)
    listeners := st.listeners // Copy for safe iteration
    st.mu.Unlock()

    // Notify listeners (outside lock)
    for _, listener := range listeners {
        listener(change)
    }

    return nil
}

// GetHistory returns status history for a task
func (st *StatusTracker) GetHistory(taskID string) []StatusChange {
    st.mu.RLock()
    defer st.mu.RUnlock()

    history := st.history[taskID]
    result := make([]StatusChange, len(history))
    copy(result, history)
    return result
}

// GetLastChange returns the most recent status change for a task
func (st *StatusTracker) GetLastChange(taskID string) *StatusChange {
    st.mu.RLock()
    defer st.mu.RUnlock()

    history := st.history[taskID]
    if len(history) == 0 {
        return nil
    }
    last := history[len(history)-1]
    return &last
}

// AddListener registers a callback for status changes
func (st *StatusTracker) AddListener(listener StatusListener) {
    st.mu.Lock()
    defer st.mu.Unlock()

    st.listeners = append(st.listeners, listener)
}
```

### Status Transitions

```go
// internal/task/transitions.go
package task

import "fmt"

// Valid status transitions
var validTransitions = map[Status][]Status{
    StatusPending:    {StatusAssigned, StatusBlocked},
    StatusBlocked:    {StatusPending},
    StatusAssigned:   {StatusInProgress, StatusPending},
    StatusInProgress: {StatusComplete, StatusReview, StatusFailed, StatusPending},
    StatusReview:     {StatusComplete, StatusInProgress, StatusFailed},
    StatusComplete:   {}, // Terminal state
    StatusFailed:     {StatusPending, StatusInProgress},
}

// validateTransition checks if a status transition is valid
func validateTransition(from, to Status) error {
    allowed, ok := validTransitions[from]
    if !ok {
        return fmt.Errorf("unknown status: %s", from)
    }

    for _, s := range allowed {
        if s == to {
            return nil
        }
    }

    return fmt.Errorf("invalid transition: %s → %s", from, to)
}

// CanTransition returns true if the transition is valid
func CanTransition(from, to Status) bool {
    return validateTransition(from, to) == nil
}

// GetValidTransitions returns valid next states for a status
func GetValidTransitions(from Status) []Status {
    return validTransitions[from]
}
```

### Event Types

```go
// internal/task/events.go
package task

import "time"

// Event types for status changes
const (
    EventTaskCreated   = "task.created"
    EventTaskAssigned  = "task.assigned"
    EventTaskStarted   = "task.started"
    EventTaskCompleted = "task.completed"
    EventTaskFailed    = "task.failed"
    EventTaskBlocked   = "task.blocked"
    EventTaskUnblocked = "task.unblocked"
)

// Event represents a task lifecycle event
type Event struct {
    Type      string
    TaskID    string
    TaskTitle string
    AgentName string
    Message   string
    Timestamp time.Time
}

// NewEvent creates an event from a status change
func NewEvent(change StatusChange, taskTitle string) Event {
    eventType := statusToEventType(change.To)

    return Event{
        Type:      eventType,
        TaskID:    change.TaskID,
        TaskTitle: taskTitle,
        AgentName: change.AgentName,
        Message:   change.Message,
        Timestamp: change.Timestamp,
    }
}

func statusToEventType(status Status) string {
    switch status {
    case StatusAssigned:
        return EventTaskAssigned
    case StatusInProgress:
        return EventTaskStarted
    case StatusComplete:
        return EventTaskCompleted
    case StatusFailed:
        return EventTaskFailed
    case StatusBlocked:
        return EventTaskBlocked
    case StatusPending:
        return EventTaskUnblocked
    default:
        return "task.status_changed"
    }
}
```

### Status Queries

```go
// GetTasksByStatus returns tasks currently in a given status
func (st *StatusTracker) GetTasksByStatus(tasks []*Task, status Status) []*Task {
    var result []*Task
    for _, t := range tasks {
        if t.Status == status {
            result = append(result, t)
        }
    }
    return result
}

// GetRecentChanges returns status changes in the last duration
func (st *StatusTracker) GetRecentChanges(since time.Duration) []StatusChange {
    st.mu.RLock()
    defer st.mu.RUnlock()

    cutoff := time.Now().Add(-since)
    var recent []StatusChange

    for _, history := range st.history {
        for _, change := range history {
            if change.Timestamp.After(cutoff) {
                recent = append(recent, change)
            }
        }
    }

    return recent
}

// GetCompletedToday returns tasks completed today
func (st *StatusTracker) GetCompletedToday() []string {
    st.mu.RLock()
    defer st.mu.RUnlock()

    today := time.Now().Truncate(24 * time.Hour)
    var completed []string

    for taskID, history := range st.history {
        for _, change := range history {
            if change.To == StatusComplete && change.Timestamp.After(today) {
                completed = append(completed, taskID)
                break
            }
        }
    }

    return completed
}
```

### Duration Tracking

```go
// GetTaskDuration returns time spent in each status
func (st *StatusTracker) GetTaskDuration(taskID string) map[Status]time.Duration {
    st.mu.RLock()
    defer st.mu.RUnlock()

    history := st.history[taskID]
    durations := make(map[Status]time.Duration)

    for i, change := range history {
        var endTime time.Time
        if i+1 < len(history) {
            endTime = history[i+1].Timestamp
        } else {
            endTime = time.Now()
        }

        duration := endTime.Sub(change.Timestamp)
        durations[change.To] += duration
    }

    return durations
}

// GetTotalWorkTime returns total time spent in in_progress status
func (st *StatusTracker) GetTotalWorkTime(taskID string) time.Duration {
    durations := st.GetTaskDuration(taskID)
    return durations[StatusInProgress]
}

// GetAverageCompletionTime returns average time from assigned to complete
func (st *StatusTracker) GetAverageCompletionTime() time.Duration {
    st.mu.RLock()
    defer st.mu.RUnlock()

    var total time.Duration
    var count int

    for _, history := range st.history {
        var assignedAt, completedAt time.Time

        for _, change := range history {
            if change.To == StatusAssigned && assignedAt.IsZero() {
                assignedAt = change.Timestamp
            }
            if change.To == StatusComplete {
                completedAt = change.Timestamp
            }
        }

        if !assignedAt.IsZero() && !completedAt.IsZero() {
            total += completedAt.Sub(assignedAt)
            count++
        }
    }

    if count == 0 {
        return 0
    }

    return total / time.Duration(count)
}
```

### Integration with TaskManager

```go
// TaskManager with status tracking
type TrackedTaskManager struct {
    *Manager
    tracker *StatusTracker
}

func NewTrackedTaskManager(cfg *Config) *TrackedTaskManager {
    return &TrackedTaskManager{
        Manager: NewManager(cfg),
        tracker: NewStatusTracker(),
    }
}

func (m *TrackedTaskManager) UpdateStatus(id string, newStatus Status) error {
    task, err := m.Get(id)
    if err != nil {
        return err
    }

    oldStatus := task.Status

    // Record before updating
    if err := m.tracker.RecordChange(id, oldStatus, newStatus, task.AssignedTo, ""); err != nil {
        return fmt.Errorf("invalid transition: %w", err)
    }

    // Update in manager
    return m.Manager.UpdateStatus(id, newStatus)
}

func (m *TrackedTaskManager) GetHistory(taskID string) []StatusChange {
    return m.tracker.GetHistory(taskID)
}
```

## Testing

### Unit Tests

```go
func TestStatusTracker_RecordChange(t *testing.T) {
    tracker := NewStatusTracker()

    err := tracker.RecordChange("T1", StatusPending, StatusAssigned, "agent-1", "")
    if err != nil {
        t.Fatalf("RecordChange() error: %v", err)
    }

    history := tracker.GetHistory("T1")
    if len(history) != 1 {
        t.Errorf("History length = %d, want 1", len(history))
    }

    if history[0].From != StatusPending || history[0].To != StatusAssigned {
        t.Errorf("Change = %v → %v, want pending → assigned", history[0].From, history[0].To)
    }
}

func TestStatusTracker_InvalidTransition(t *testing.T) {
    tracker := NewStatusTracker()

    // Can't go from pending to complete directly
    err := tracker.RecordChange("T1", StatusPending, StatusComplete, "", "")
    if err == nil {
        t.Error("Should reject invalid transition pending → complete")
    }
}

func TestStatusTracker_Listener(t *testing.T) {
    tracker := NewStatusTracker()

    var received []StatusChange
    tracker.AddListener(func(change StatusChange) {
        received = append(received, change)
    })

    tracker.RecordChange("T1", StatusPending, StatusAssigned, "agent-1", "")
    tracker.RecordChange("T1", StatusAssigned, StatusInProgress, "agent-1", "")

    if len(received) != 2 {
        t.Errorf("Listener received %d changes, want 2", len(received))
    }
}

func TestValidTransitions(t *testing.T) {
    tests := []struct {
        from    Status
        to      Status
        valid   bool
    }{
        {StatusPending, StatusAssigned, true},
        {StatusPending, StatusBlocked, true},
        {StatusPending, StatusComplete, false},
        {StatusAssigned, StatusInProgress, true},
        {StatusAssigned, StatusPending, true},
        {StatusInProgress, StatusComplete, true},
        {StatusInProgress, StatusFailed, true},
        {StatusComplete, StatusPending, false}, // Terminal
        {StatusFailed, StatusPending, true},    // Can retry
    }

    for _, tt := range tests {
        t.Run(fmt.Sprintf("%s→%s", tt.from, tt.to), func(t *testing.T) {
            valid := CanTransition(tt.from, tt.to)
            if valid != tt.valid {
                t.Errorf("CanTransition(%s, %s) = %v, want %v", tt.from, tt.to, valid, tt.valid)
            }
        })
    }
}

func TestStatusTracker_GetTaskDuration(t *testing.T) {
    tracker := NewStatusTracker()

    // Simulate task lifecycle
    tracker.RecordChange("T1", "", StatusPending, "", "")
    time.Sleep(10 * time.Millisecond)
    tracker.RecordChange("T1", StatusPending, StatusAssigned, "agent-1", "")
    time.Sleep(10 * time.Millisecond)
    tracker.RecordChange("T1", StatusAssigned, StatusInProgress, "agent-1", "")
    time.Sleep(50 * time.Millisecond)
    tracker.RecordChange("T1", StatusInProgress, StatusComplete, "agent-1", "")

    durations := tracker.GetTaskDuration("T1")

    // InProgress should be ~50ms
    if durations[StatusInProgress] < 40*time.Millisecond {
        t.Errorf("InProgress duration = %v, want ~50ms", durations[StatusInProgress])
    }
}
```

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Invalid transition | Return error |
| Unknown status | Return error |
| Empty history | Return empty slice |
| Listener panic | Log and continue |

## Out of Scope

- Persistent history storage
- History pruning/retention
- Undo/rollback
- External event sinks (webhooks)

## Notes

Status tracking is in-memory only. For persistence, the status is written to task files. The tracker provides real-time visibility and analytics but doesn't survive restarts.

The event system enables features like real-time dashboards and notifications without tight coupling.
