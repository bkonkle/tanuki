package task

import (
	"context"
	"log"
	"time"
)

// Reassigner automatically assigns tasks to idle agents.
type Reassigner struct {
	taskMgr  TaskManagerInterface
	queue    QueueInterface
	agentMgr AgentLister
	runner   *Runner
	interval time.Duration
}

// QueueInterface defines the methods needed by Reassigner for queue operations.
type QueueInterface interface {
	Dequeue(role string) (*Task, error)
	Enqueue(t *Task) error
}

// AgentLister defines the methods needed by Reassigner for listing agents.
type AgentLister interface {
	List() ([]*AgentInfo, error)
}

// AgentInfo contains the agent information needed for reassignment.
type AgentInfo struct {
	Name   string
	Role   string
	Status string // "idle", "working", "stopped"
}

// DependencyChecker checks if a task is blocked by dependencies.
type DependencyChecker interface {
	IsBlocked(id string) (bool, error)
}

// NewReassigner creates a new auto-reassigner.
func NewReassigner(taskMgr TaskManagerInterface, queue QueueInterface, agentMgr AgentLister, runner *Runner) *Reassigner {
	return &Reassigner{
		taskMgr:  taskMgr,
		queue:    queue,
		agentMgr: agentMgr,
		runner:   runner,
		interval: 10 * time.Second,
	}
}

// Start begins the auto-reassignment loop.
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

// checkAndReassign checks for idle agents and assigns pending tasks.
func (r *Reassigner) checkAndReassign(ctx context.Context) {
	// Get all agents
	agents, err := r.agentMgr.List()
	if err != nil {
		log.Printf("Reassigner: failed to list agents: %v", err)
		return
	}

	for _, ag := range agents {
		if ag.Status != "idle" || ag.Role == "" {
			continue
		}

		// Try to get next task for this role
		t, err := r.queue.Dequeue(ag.Role)
		if err != nil {
			continue // No tasks for this role
		}

		// Assign and run
		log.Printf("Auto-assigning %s to %s", t.ID, ag.Name)

		// Start task execution in background
		go func(taskID, agentName string) {
			if err := r.runner.RunTask(ctx, taskID, agentName); err != nil {
				log.Printf("Task %s failed: %v", taskID, err)
			}
		}(t.ID, ag.Name)
	}
}

// OnTaskComplete handles task completion events.
// Called when a task completes to immediately assign the next task.
func (r *Reassigner) OnTaskComplete(ctx context.Context, taskID, agentName, agentRole string) {
	// Immediately try to assign next task
	t, err := r.queue.Dequeue(agentRole)
	if err != nil {
		log.Printf("Agent %s idle, no more tasks for role %s", agentName, agentRole)
		return
	}

	log.Printf("Assigning next task %s to %s", t.ID, agentName)

	go func() {
		if err := r.runner.RunTask(ctx, t.ID, agentName); err != nil {
			log.Printf("Task %s failed: %v", t.ID, err)
		}
	}()
}

// SetInterval sets the interval between reassignment checks.
func (r *Reassigner) SetInterval(interval time.Duration) {
	r.interval = interval
}
