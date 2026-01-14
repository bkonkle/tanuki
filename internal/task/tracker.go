package task

import (
	"fmt"
	"sync"
	"time"
)

// StatusTracker tracks task status changes and history.
// It provides an in-memory record of all status transitions for visibility
// and monitoring purposes.
type StatusTracker struct {
	mu        sync.RWMutex
	history   map[string][]StatusChange // taskID -> history
	listeners []StatusListener
}

// StatusChange represents a single status transition.
type StatusChange struct {
	TaskID    string
	From      Status
	To        Status
	Timestamp time.Time
	AgentName string
	Message   string
}

// StatusListener receives status change notifications.
type StatusListener func(change StatusChange)

// NewStatusTracker creates a new tracker.
func NewStatusTracker() *StatusTracker {
	return &StatusTracker{
		history: make(map[string][]StatusChange),
	}
}

// RecordChange records a status change for a task.
// Returns an error if the transition is invalid.
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
		func() {
			defer func() {
				// Recover from panicking listeners
				recover()
			}()
			listener(change)
		}()
	}

	return nil
}

// GetHistory returns status history for a task.
func (st *StatusTracker) GetHistory(taskID string) []StatusChange {
	st.mu.RLock()
	defer st.mu.RUnlock()

	history := st.history[taskID]
	result := make([]StatusChange, len(history))
	copy(result, history)
	return result
}

// GetLastChange returns the most recent status change for a task.
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

// AddListener registers a callback for status changes.
func (st *StatusTracker) AddListener(listener StatusListener) {
	st.mu.Lock()
	defer st.mu.Unlock()

	st.listeners = append(st.listeners, listener)
}

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

// validateTransition checks if a status transition is valid.
func validateTransition(from, to Status) error {
	// Allow recording initial state (from empty)
	if from == "" {
		return nil
	}

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

// CanTransition returns true if the transition is valid.
func CanTransition(from, to Status) bool {
	return validateTransition(from, to) == nil
}

// GetValidTransitions returns valid next states for a status.
func GetValidTransitions(from Status) []Status {
	return validTransitions[from]
}

// GetTasksByStatus returns tasks currently in a given status.
func (st *StatusTracker) GetTasksByStatus(tasks []*Task, status Status) []*Task {
	var result []*Task
	for _, t := range tasks {
		if t.Status == status {
			result = append(result, t)
		}
	}
	return result
}

// GetRecentChanges returns status changes in the last duration.
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

// GetCompletedToday returns task IDs completed today.
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

// GetTaskDuration returns time spent in each status.
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

// GetTotalWorkTime returns total time spent in in_progress status.
func (st *StatusTracker) GetTotalWorkTime(taskID string) time.Duration {
	durations := st.GetTaskDuration(taskID)
	return durations[StatusInProgress]
}

// GetAverageCompletionTime returns average time from assigned to complete.
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

// NewEventFromChange creates an Event from a StatusChange.
// Event type and constants are defined in events.go.
func NewEventFromChange(change StatusChange, taskTitle string) Event {
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

// Clear removes all history data.
func (st *StatusTracker) Clear() {
	st.mu.Lock()
	defer st.mu.Unlock()

	st.history = make(map[string][]StatusChange)
}

// TaskCount returns the number of unique tasks being tracked.
func (st *StatusTracker) TaskCount() int {
	st.mu.RLock()
	defer st.mu.RUnlock()

	return len(st.history)
}
