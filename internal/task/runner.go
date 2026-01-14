package task

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
)

// Runner executes tasks on agents, supporting Ralph-style iteration.
type Runner struct {
	taskMgr    TaskManagerInterface
	agentMgr   AgentExecutor
	completion *CompletionHandler
	cooldown   time.Duration
}

// AgentExecutor defines the interface for running commands on agents.
type AgentExecutor interface {
	Run(name, prompt string) (string, error)
}

// RunnerOptions configures the task runner.
type RunnerOptions struct {
	Cooldown time.Duration
}

// NewRunner creates a new task runner.
func NewRunner(taskMgr TaskManagerInterface, agentMgr AgentExecutor, completion *CompletionHandler) *Runner {
	return &Runner{
		taskMgr:    taskMgr,
		agentMgr:   agentMgr,
		completion: completion,
		cooldown:   5 * time.Second,
	}
}

// RunTask executes a task on an agent, using Ralph mode if configured.
func (r *Runner) RunTask(ctx context.Context, taskID, agentName string) error {
	t, err := r.taskMgr.Get(taskID)
	if err != nil {
		return fmt.Errorf("get task: %w", err)
	}

	// Update status to in_progress
	if err := r.taskMgr.UpdateStatus(taskID, StatusInProgress); err != nil {
		return fmt.Errorf("update status: %w", err)
	}
	now := time.Now()
	t.StartedAt = &now

	// Build prompt
	prompt := r.buildPrompt(t)

	// Run in Ralph mode if completion criteria defined
	if t.IsRalphMode() {
		return r.runRalphMode(ctx, t, agentName, prompt)
	}

	// Single-shot execution
	output, err := r.agentMgr.Run(agentName, prompt)
	if err != nil {
		if statusErr := r.taskMgr.UpdateStatus(taskID, StatusFailed); statusErr != nil {
			log.Printf("Warning: failed to update task status: %v", statusErr)
		}
		return err
	}

	// Check completion
	return r.completion.HandleAgentOutput(ctx, taskID, agentName, output)
}

// runRalphMode iterates until completion criteria are met or max iterations reached.
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
		output, err := r.agentMgr.Run(agentName, prompt)
		if err != nil {
			log.Printf("Task %s: agent error: %v", t.ID, err)
			// Continue trying unless context cancelled
			continue
		}

		// Check completion
		result := r.completion.GetValidator().Validate(ctx, t, output)

		if result.Status == StatusComplete {
			if err := r.taskMgr.UpdateStatus(t.ID, StatusComplete); err != nil {
				log.Printf("Warning: failed to update task status: %v", err)
			}
			log.Printf("Task %s: completed after %d iterations", t.ID, i)
			return nil
		}

		if result.Status == StatusFailed {
			// Don't retry on hard failures
			if err := r.taskMgr.UpdateStatus(t.ID, StatusFailed); err != nil {
				log.Printf("Warning: failed to update task status: %v", err)
			}
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
	if err := r.taskMgr.UpdateStatus(t.ID, StatusReview); err != nil {
		log.Printf("Warning: failed to update task status: %v", err)
	}
	return fmt.Errorf("max iterations (%d) reached without completion", maxIterations)
}

// buildPrompt creates the prompt to send to an agent for a task.
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

// SetCooldown sets the cooldown duration between Ralph mode iterations.
func (r *Runner) SetCooldown(cooldown time.Duration) {
	r.cooldown = cooldown
}
