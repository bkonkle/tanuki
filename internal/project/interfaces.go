// Package project provides project-level orchestration for task-driven agent workflows.
//
// This package defines interfaces for TaskManager and TaskQueue that project commands
// code against. The actual implementations live in other packages (internal/task).
// This enables parallel development across workstreams.
package project

import (
	"github.com/bkonkle/tanuki/internal/agent"
	"github.com/bkonkle/tanuki/internal/task"
)

// TaskManager defines the interface for task operations.
// This interface is implemented by internal/task.Manager.
type TaskManager interface {
	// Scan loads all task files from .tanuki/tasks/
	Scan() ([]*task.Task, error)

	// Get returns a task by ID
	Get(id string) (*task.Task, error)

	// GetByWorkstream returns tasks for a specific workstream
	GetByWorkstream(workstream string) []*task.Task

	// GetByStatus returns tasks with a specific status
	GetByStatus(status task.Status) []*task.Task

	// GetPending returns all pending tasks, sorted by priority
	GetPending() []*task.Task

	// UpdateStatus changes task status and persists to file
	UpdateStatus(id string, status task.Status) error

	// Assign assigns a task to an agent
	Assign(id string, agentName string) error

	// Unassign removes agent assignment from a task
	Unassign(id string) error

	// IsBlocked checks if a task's dependencies are all complete
	IsBlocked(id string) (bool, error)

	// Stats returns task statistics
	Stats() *TaskStats
}

// TaskQueue defines the interface for priority-based task queueing.
// This interface is implemented by internal/task.Queue.
type TaskQueue interface {
	// Enqueue adds a task to the queue
	Enqueue(t *task.Task) error

	// Dequeue removes and returns the highest priority task for a workstream
	Dequeue(workstream string) (*task.Task, error)

	// Peek returns the highest priority task without removing it
	Peek(workstream string) (*task.Task, error)

	// Size returns total number of tasks in queue
	Size() int

	// SizeByWorkstream returns number of tasks for a specific workstream
	SizeByWorkstream(workstream string) int

	// Contains checks if a task is in the queue
	Contains(taskID string) bool

	// Clear empties the queue
	Clear()
}

// AgentManager defines the interface for agent operations needed by project commands.
// This is a subset of agent.Manager functionality.
type AgentManager interface {
	// Spawn creates a new agent
	Spawn(name string, opts agent.SpawnOptions) (*agent.Agent, error)

	// Get returns information about a specific agent
	Get(name string) (*agent.Agent, error)

	// List returns all agents
	List() ([]*agent.Agent, error)

	// Start starts a stopped agent's container
	Start(name string) error

	// Stop stops an agent's container without removing it
	Stop(name string) error

	// Remove deletes an agent and cleans up all associated resources
	Remove(name string, opts agent.RemoveOptions) error

	// Run executes a task in the agent's container
	Run(name string, prompt string, opts agent.RunOptions) error
}

// TaskStats holds task statistics.
type TaskStats struct {
	Total        int
	ByStatus     map[task.Status]int
	ByWorkstream map[string]int
	ByPriority   map[task.Priority]int
}
