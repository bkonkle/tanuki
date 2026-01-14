// Package task provides task file parsing and management for Tanuki project mode.
//
// Tasks are defined in markdown files with YAML front matter. They support
// dependency tracking, priority ordering, and Ralph-style objective completion
// criteria for autonomous execution.
package task

import (
	"time"
)

// Task represents a work item to be assigned to an agent.
// Tasks are defined in markdown files with YAML front matter and support
// dependency tracking, priority levels, and completion criteria.
type Task struct {
	// From front matter
	ID         string            `yaml:"id"`
	Title      string            `yaml:"title"`
	Role       string            `yaml:"role"`
	Workstream string            `yaml:"workstream,omitempty"` // Groups related tasks for sequential execution
	Priority   Priority          `yaml:"priority"`
	Status     Status            `yaml:"status"`
	DependsOn  []string          `yaml:"depends_on"`
	AssignedTo string            `yaml:"assigned_to,omitempty"`
	Completion *CompletionConfig `yaml:"completion,omitempty"`
	Tags       []string          `yaml:"tags,omitempty"`

	// Derived fields (not in YAML)
	FilePath    string     `yaml:"-"`
	Content     string     `yaml:"-"` // Markdown body (after front matter)
	CompletedAt *time.Time `yaml:"completed_at,omitempty"`
	StartedAt   *time.Time `yaml:"started_at,omitempty"`
}

// GetWorkstream returns the workstream identifier for this task.
// If not explicitly set, returns the task ID (single-task workstream).
func (t *Task) GetWorkstream() string {
	if t.Workstream != "" {
		return t.Workstream
	}
	return t.ID
}

// Priority levels for tasks.
// Tasks are ordered by priority with critical being highest.
type Priority string

const (
	// PriorityCritical - Must be done immediately
	PriorityCritical Priority = "critical"
	// PriorityHigh - Important, do soon
	PriorityHigh Priority = "high"
	// PriorityMedium - Normal priority (default)
	PriorityMedium Priority = "medium"
	// PriorityLow - Nice to have
	PriorityLow Priority = "low"
)

// Status values for task lifecycle.
type Status string

const (
	// StatusPending - Not yet started
	StatusPending Status = "pending"
	// StatusAssigned - Agent assigned, not yet started
	StatusAssigned Status = "assigned"
	// StatusInProgress - Agent actively working
	StatusInProgress Status = "in_progress"
	// StatusReview - Work complete, needs human review
	StatusReview Status = "review"
	// StatusComplete - Verified and done (terminal state)
	StatusComplete Status = "complete"
	// StatusFailed - Failed, needs attention
	StatusFailed Status = "failed"
	// StatusBlocked - Waiting on dependencies
	StatusBlocked Status = "blocked"
)

// CompletionConfig defines how to determine task completion (Ralph-style).
// Either Verify or Signal (or both) can be specified for autonomous validation.
type CompletionConfig struct {
	// Verify is a command that must exit 0 for completion
	Verify string `yaml:"verify,omitempty"`

	// Signal is a string to detect in agent output
	Signal string `yaml:"signal,omitempty"`

	// MaxIterations for Ralph mode (default: 30)
	MaxIterations int `yaml:"max_iterations,omitempty"`
}

// IsRalphMode returns true if task should use Ralph-style iteration.
// Ralph mode continuously runs until completion criteria are met.
func (t *Task) IsRalphMode() bool {
	return t.Completion != nil && (t.Completion.Verify != "" || t.Completion.Signal != "")
}

// GetMaxIterations returns max iterations with default fallback.
func (c *CompletionConfig) GetMaxIterations() int {
	if c == nil || c.MaxIterations <= 0 {
		return 30 // Default
	}
	return c.MaxIterations
}

// IsValid checks if a Priority value is valid.
func (p Priority) IsValid() bool {
	switch p {
	case PriorityCritical, PriorityHigh, PriorityMedium, PriorityLow, "":
		return true
	default:
		return false
	}
}

// IsValid checks if a Status value is valid.
func (s Status) IsValid() bool {
	switch s {
	case StatusPending, StatusAssigned, StatusInProgress, StatusReview,
		StatusComplete, StatusFailed, StatusBlocked, "":
		return true
	default:
		return false
	}
}

// IsTerminal returns true if the status is a terminal state.
func (s Status) IsTerminal() bool {
	return s == StatusComplete
}

// PriorityOrder returns the sort order for a priority (lower = higher priority).
func (p Priority) Order() int {
	switch p {
	case PriorityCritical:
		return 0
	case PriorityHigh:
		return 1
	case PriorityMedium:
		return 2
	case PriorityLow:
		return 3
	default:
		return 2 // Default to medium
	}
}

// ValidationError represents a task validation error.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	if e.Field != "" {
		return "task." + e.Field + ": " + e.Message
	}
	return e.Message
}
