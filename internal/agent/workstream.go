// Package agent provides agent lifecycle management functionality.
package agent

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/bkonkle/tanuki/internal/task"
)

// WorkstreamRunner executes tasks within a workstream sequentially.
// It handles dependency checking, task assignment, and completion signaling.
type WorkstreamRunner struct {
	// Agent manager for running tasks
	agentMgr *Manager

	// Task manager for task operations
	taskMgr *task.Manager

	// Project name (empty for root tasks)
	project string

	// Role for this workstream
	role string

	// Workstream identifier
	workstream string

	// Agent name for this workstream
	agentName string

	// Configuration
	config WorkstreamConfig

	// Event callbacks
	onTaskStart    func(taskID string)
	onTaskComplete func(taskID string)
	onTaskFailed   func(taskID string, err error)
	onBlocked      func(taskID string, blockers []string)

	// Output writer for task execution
	output io.Writer
}

// WorkstreamConfig configures workstream execution behavior.
type WorkstreamConfig struct {
	// PollInterval is how often to check for dependency resolution
	PollInterval time.Duration

	// MaxWaitTime is the maximum time to wait for dependencies
	MaxWaitTime time.Duration

	// MaxTurns per task execution
	MaxTurns int

	// Model to use for task execution
	Model string

	// Follow enables streaming output
	Follow bool
}

// DefaultWorkstreamConfig returns default configuration.
func DefaultWorkstreamConfig() WorkstreamConfig {
	return WorkstreamConfig{
		PollInterval: 30 * time.Second,
		MaxWaitTime:  24 * time.Hour,
		MaxTurns:     50,
		Model:        "",
		Follow:       true,
	}
}

// NewWorkstreamRunner creates a runner for a specific workstream.
func NewWorkstreamRunner(
	agentMgr *Manager,
	taskMgr *task.Manager,
	projectName, role, workstream string,
	config WorkstreamConfig,
) *WorkstreamRunner {
	agentName := buildWorkstreamAgentName(projectName, workstream)

	return &WorkstreamRunner{
		agentMgr:   agentMgr,
		taskMgr:    taskMgr,
		project:    projectName,
		role:       role,
		workstream: workstream,
		agentName:  agentName,
		config:     config,
		output:     os.Stdout,
	}
}

// buildWorkstreamAgentName creates the standardized agent name.
// Format: {project}-{workstream} (e.g., "auth-feature-oauth")
func buildWorkstreamAgentName(projectName, workstream string) string {
	if projectName == "" {
		return strings.ToLower(strings.ReplaceAll(workstream, " ", "-"))
	}
	// Clean the names: lowercase, replace spaces with hyphens
	project := strings.ToLower(strings.ReplaceAll(projectName, " ", "-"))
	ws := strings.ToLower(strings.ReplaceAll(workstream, " ", "-"))
	return fmt.Sprintf("%s-%s", project, ws)
}

// buildWorktreeBranch creates the standardized branch name.
// Format: tanuki/{project}-{workstream}
func buildWorktreeBranch(projectName, workstream string) string {
	return fmt.Sprintf("tanuki/%s", buildWorkstreamAgentName(projectName, workstream))
}

// SetOutput sets the output writer for task execution.
func (r *WorkstreamRunner) SetOutput(w io.Writer) {
	r.output = w
}

// SetOnTaskStart sets the callback for task start events.
func (r *WorkstreamRunner) SetOnTaskStart(fn func(taskID string)) {
	r.onTaskStart = fn
}

// SetOnTaskComplete sets the callback for task completion events.
func (r *WorkstreamRunner) SetOnTaskComplete(fn func(taskID string)) {
	r.onTaskComplete = fn
}

// SetOnTaskFailed sets the callback for task failure events.
func (r *WorkstreamRunner) SetOnTaskFailed(fn func(taskID string, err error)) {
	r.onTaskFailed = fn
}

// SetOnBlocked sets the callback for blocked task events.
func (r *WorkstreamRunner) SetOnBlocked(fn func(taskID string, blockers []string)) {
	r.onBlocked = fn
}

// Run executes all tasks in the workstream sequentially.
// Returns when all tasks are complete or an unrecoverable error occurs.
func (r *WorkstreamRunner) Run() error {
	log.Printf("Starting workstream runner: %s (project=%s, role=%s, workstream=%s)",
		r.agentName, r.project, r.role, r.workstream)

	for {
		// Get next pending task for this workstream
		nextTask, err := r.getNextTask()
		if err != nil {
			if errors.Is(err, errNoMoreTasks) {
				log.Printf("Workstream %s complete: no more tasks", r.agentName)
				return nil
			}
			return fmt.Errorf("get next task: %w", err)
		}

		// Wait for dependencies if blocked
		if blocked, _ := r.taskMgr.IsBlocked(nextTask.ID); blocked {
			if err := r.waitForDependencies(nextTask); err != nil {
				return fmt.Errorf("wait for dependencies: %w", err)
			}
		}

		// Execute the task
		if err := r.executeTask(nextTask); err != nil {
			// Mark task as failed
			if updateErr := r.taskMgr.UpdateStatus(nextTask.ID, task.StatusFailed); updateErr != nil {
				log.Printf("Warning: failed to update task status: %v", updateErr)
			}

			if r.onTaskFailed != nil {
				r.onTaskFailed(nextTask.ID, err)
			}

			// Continue to next task instead of stopping the workstream
			log.Printf("Task %s failed: %v", nextTask.ID, err)
			continue
		}
	}
}

var errNoMoreTasks = errors.New("no more tasks in workstream")

// getNextTask returns the next pending task for this workstream.
func (r *WorkstreamRunner) getNextTask() (*task.Task, error) {
	var tasks []*task.Task

	if r.project != "" {
		tasks = r.taskMgr.GetByProjectAndWorkstream(r.project, r.role, r.workstream)
	} else {
		tasks = r.taskMgr.GetByRoleAndWorkstream(r.role, r.workstream)
	}

	// Find first pending task
	for _, t := range tasks {
		if t.Status == task.StatusPending || t.Status == task.StatusBlocked {
			return t, nil
		}
	}

	return nil, errNoMoreTasks
}

// waitForDependencies waits until all dependencies are complete.
func (r *WorkstreamRunner) waitForDependencies(t *task.Task) error {
	startTime := time.Now()

	for {
		// Check if still blocked
		blocked, err := r.taskMgr.IsBlocked(t.ID)
		if err != nil {
			return fmt.Errorf("check blocked status: %w", err)
		}

		if !blocked {
			return nil // Dependencies resolved
		}

		// Get blocking tasks for logging
		blockers, _ := r.taskMgr.GetBlockingTasks(t.ID)
		log.Printf("Task %s waiting for dependencies: %v", t.ID, blockers)

		if r.onBlocked != nil {
			r.onBlocked(t.ID, blockers)
		}

		// Check timeout
		if time.Since(startTime) > r.config.MaxWaitTime {
			return fmt.Errorf("timeout waiting for dependencies after %v", r.config.MaxWaitTime)
		}

		// Wait before next check
		time.Sleep(r.config.PollInterval)

		// Re-scan tasks to pick up status changes from disk
		if _, err := r.taskMgr.Scan(); err != nil {
			log.Printf("Warning: failed to re-scan tasks: %v", err)
		}
	}
}

// executeTask runs a single task through the agent.
func (r *WorkstreamRunner) executeTask(t *task.Task) error {
	log.Printf("Executing task %s: %s", t.ID, t.Title)

	// Notify task start
	if r.onTaskStart != nil {
		r.onTaskStart(t.ID)
	}

	// Assign task to agent
	if err := r.taskMgr.Assign(t.ID, r.agentName); err != nil {
		return fmt.Errorf("assign task: %w", err)
	}

	// Mark as in progress
	if err := r.taskMgr.UpdateStatus(t.ID, task.StatusInProgress); err != nil {
		return fmt.Errorf("update status to in_progress: %w", err)
	}

	// Build prompt
	prompt := buildTaskPrompt(t)

	// Execute via agent manager
	runOpts := RunOptions{
		Follow:   r.config.Follow,
		MaxTurns: r.config.MaxTurns,
		Model:    r.config.Model,
		Output:   r.output,
	}

	err := r.agentMgr.Run(r.agentName, prompt, runOpts)
	if err != nil {
		return fmt.Errorf("agent run: %w", err)
	}

	// Mark task as complete
	if err := r.taskMgr.UpdateStatus(t.ID, task.StatusComplete); err != nil {
		return fmt.Errorf("update status to complete: %w", err)
	}

	// Notify task complete
	if r.onTaskComplete != nil {
		r.onTaskComplete(t.ID)
	}

	log.Printf("Task %s completed", t.ID)
	return nil
}

// buildTaskPrompt creates the prompt for Claude from a task.
func buildTaskPrompt(t *task.Task) string {
	prompt := fmt.Sprintf("# Task: %s\n\n", t.Title)
	prompt += t.Content

	if t.Completion != nil {
		prompt += "\n\n## Completion Criteria\n\n"
		if t.Completion.Verify != "" {
			prompt += fmt.Sprintf("Run this command to verify: `%s`\n", t.Completion.Verify)
		}
		if t.Completion.Signal != "" {
			prompt += fmt.Sprintf("Say **%s** when complete.\n", t.Completion.Signal)
		}
	}

	return prompt
}

// WorkstreamOrchestrator manages multiple workstream runners with concurrency limits.
type WorkstreamOrchestrator struct {
	agentMgr *Manager
	taskMgr  *task.Manager

	// Concurrency limits per role
	roleConcurrency map[string]int

	// Active runners by role
	activeRunners map[string]int

	// Configuration
	config WorkstreamConfig
}

// NewWorkstreamOrchestrator creates an orchestrator for managing workstreams.
func NewWorkstreamOrchestrator(agentMgr *Manager, taskMgr *task.Manager, config WorkstreamConfig) *WorkstreamOrchestrator {
	return &WorkstreamOrchestrator{
		agentMgr:        agentMgr,
		taskMgr:         taskMgr,
		roleConcurrency: make(map[string]int),
		activeRunners:   make(map[string]int),
		config:          config,
	}
}

// SetRoleConcurrency sets the concurrency limit for a role.
func (o *WorkstreamOrchestrator) SetRoleConcurrency(role string, limit int) {
	if limit <= 0 {
		limit = 1
	}
	o.roleConcurrency[role] = limit
}

// CanStartWorkstream checks if a new workstream can be started for the given role.
func (o *WorkstreamOrchestrator) CanStartWorkstream(role string) bool {
	limit := o.roleConcurrency[role]
	if limit <= 0 {
		limit = 1
	}
	return o.activeRunners[role] < limit
}

// StartWorkstream spawns an agent and starts running the workstream.
// Returns the runner for monitoring, or an error if the workstream cannot be started.
func (o *WorkstreamOrchestrator) StartWorkstream(projectName, role, workstream string) (*WorkstreamRunner, error) {
	if !o.CanStartWorkstream(role) {
		return nil, fmt.Errorf("concurrency limit reached for role %s", role)
	}

	agentName := buildWorkstreamAgentName(projectName, workstream)
	branchName := buildWorktreeBranch(projectName, workstream)

	// Check if agent already exists
	_, err := o.agentMgr.Get(agentName)
	if err != nil {
		// Agent doesn't exist, spawn it
		log.Printf("Spawning agent %s for workstream (branch: %s)", agentName, branchName)

		_, err = o.agentMgr.Spawn(agentName, SpawnOptions{
			Branch: branchName,
			Role:   role,
		})
		if err != nil {
			return nil, fmt.Errorf("spawn agent: %w", err)
		}
	}

	// Create runner
	runner := NewWorkstreamRunner(o.agentMgr, o.taskMgr, projectName, role, workstream, o.config)

	// Track active runner
	o.activeRunners[role]++

	// Set up completion callback to decrement counter
	originalOnComplete := runner.onTaskComplete
	runner.SetOnTaskComplete(func(taskID string) {
		if originalOnComplete != nil {
			originalOnComplete(taskID)
		}
	})

	return runner, nil
}

// ReleaseWorkstream marks a workstream as no longer active.
func (o *WorkstreamOrchestrator) ReleaseWorkstream(role string) {
	if o.activeRunners[role] > 0 {
		o.activeRunners[role]--
	}
}

// GetActiveCount returns the number of active workstreams for a role.
func (o *WorkstreamOrchestrator) GetActiveCount(role string) int {
	return o.activeRunners[role]
}
