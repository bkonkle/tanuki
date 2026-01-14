package task

import "time"

// Event types for task lifecycle.
const (
	EventTaskCreated   = "task.created"
	EventTaskAssigned  = "task.assigned"
	EventTaskStarted   = "task.started"
	EventTaskCompleted = "task.completed"
	EventTaskFailed    = "task.failed"
	EventTaskBlocked   = "task.blocked"
	EventTaskUnblocked = "task.unblocked"
)

// Event represents a task lifecycle event.
type Event struct {
	Type      string // "task.created", "task.assigned", etc.
	TaskID    string
	TaskTitle string
	AgentName string
	Message   string
	Timestamp time.Time
}

// statusToEventType converts a Status to an event type string.
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
