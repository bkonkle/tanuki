package task

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Validator checks if task completion criteria are met.
type Validator struct {
	workdir string
	timeout time.Duration
}

// NewValidator creates a new task validator.
func NewValidator(workdir string) *Validator {
	return &Validator{
		workdir: workdir,
		timeout: 5 * time.Minute,
	}
}

// ValidationResult contains the outcome of a validation check.
type ValidationResult struct {
	Task         *Task
	SignalFound  bool
	VerifyPassed bool
	VerifyOutput string
	VerifyError  error
	Status       Status // complete, review, failed, in_progress
	Message      string
}

// Validate checks if task completion criteria are met.
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

// runVerifyCommand executes a verification command and returns pass/fail status.
func (v *Validator) runVerifyCommand(ctx context.Context, command string) (bool, string, error) {
	ctx, cancel := context.WithTimeout(ctx, v.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	if v.workdir != "" {
		cmd.Dir = v.workdir
	}

	output, err := cmd.CombinedOutput()

	if ctx.Err() == context.DeadlineExceeded {
		return false, string(output), fmt.Errorf("command timed out after %v", v.timeout)
	}

	if err != nil {
		// Check if it's a non-zero exit code
		if _, ok := err.(*exec.ExitError); ok {
			// Non-zero exit code - not an error, just verification failed
			return false, string(output), nil
		}
		return false, string(output), err
	}

	return true, string(output), nil
}

// SetTimeout sets the timeout for verify commands.
func (v *Validator) SetTimeout(timeout time.Duration) {
	v.timeout = timeout
}
