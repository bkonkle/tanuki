// Package executor provides Claude Code execution functionality for agents.
//
// The executor supports three execution modes:
// 1. Fire-and-forget: Run task and return immediately
// 2. Follow: Stream output in real-time until completion
// 3. Ralph: Loop until completion signal or max iterations (autonomous mode)
package executor

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/bkonkle/tanuki/internal/docker"
)

var (
	// ErrAlreadyRunning indicates Claude Code is already executing in the container.
	ErrAlreadyRunning = errors.New("claude code is already running in this container")

	// ErrClaudeNotFound indicates Claude Code is not installed in the container.
	ErrClaudeNotFound = errors.New("claude code not found in container")

	// ErrMaxIterations indicates Ralph mode reached max iterations without completion.
	ErrMaxIterations = errors.New("reached max iterations without completion")
)

// Executor handles Claude Code execution in Docker containers.
type Executor struct {
	docker DockerManager
}

// DockerManager defines the interface for Docker operations needed by the executor.
type DockerManager interface {
	Exec(containerID string, cmd []string, opts docker.ExecOptions) error
	ExecWithOutput(containerID string, cmd []string) (string, error)
	ContainerRunning(containerID string) bool
}

// ExecuteOptions configures Claude Code execution.
type ExecuteOptions struct {
	// AllowedTools lists the tools Claude Code can use
	AllowedTools []string

	// DisallowedTools lists the tools Claude Code cannot use
	DisallowedTools []string

	// MaxTurns limits conversation turns (0 = use Claude default)
	MaxTurns int

	// Model specifies which Claude model to use
	Model string

	// SystemPrompt is appended to the default system prompt
	SystemPrompt string

	// WorkDir is the working directory inside the container
	WorkDir string
}

// RalphOptions configures Ralph mode (autonomous loop) execution.
type RalphOptions struct {
	ExecuteOptions

	// MaxIterations limits the number of loop iterations
	MaxIterations int

	// CompletionSignal is the string that signals task completion
	CompletionSignal string

	// VerifyCommand is an optional command to verify task completion
	VerifyCommand string

	// CooldownSeconds is the pause between iterations
	CooldownSeconds int
}

// ExecutionResult contains the result of a Claude Code execution.
type ExecutionResult struct {
	// SessionID is the Claude Code session identifier
	SessionID string

	// Output is the complete output from Claude Code
	Output string

	// ExitCode is the process exit code
	ExitCode int

	// StartedAt is when execution started
	StartedAt time.Time

	// CompletedAt is when execution completed
	CompletedAt time.Time

	// Error is any error that occurred during execution
	Error error
}

// RalphResult contains the result of a Ralph mode execution.
type RalphResult struct {
	ExecutionResult

	// Iterations is the number of loops completed
	Iterations int

	// CompletedBy indicates how the loop completed
	// Values: "signal", "verify", "max_iterations", "error"
	CompletedBy string
}

// StreamMessage represents a single message from Claude Code stream-json output.
type StreamMessage struct {
	Type      string `json:"type"`
	SessionID string `json:"session_id,omitempty"`
	Content   string `json:"content,omitempty"`
	Error     string `json:"error,omitempty"`
}

// NewExecutor creates a new Claude Code executor.
func NewExecutor(dockerMgr DockerManager) *Executor {
	return &Executor{
		docker: dockerMgr,
	}
}

// Run executes a Claude Code prompt in fire-and-forget mode.
// Returns immediately after starting execution.
func (e *Executor) Run(containerID string, prompt string, opts ExecuteOptions) (*ExecutionResult, error) {
	if !e.docker.ContainerRunning(containerID) {
		return nil, errors.New("container is not running")
	}

	cmd := e.buildCommand(prompt, opts)
	startedAt := time.Now()

	// Execute command and capture output
	output, err := e.docker.ExecWithOutput(containerID, cmd)
	completedAt := time.Now()

	result := &ExecutionResult{
		Output:      output,
		StartedAt:   startedAt,
		CompletedAt: completedAt,
	}

	if err != nil {
		result.Error = err
		result.ExitCode = 1
		// Try to parse error from output
		if strings.Contains(output, "claude: command not found") || strings.Contains(output, "not found") {
			return result, fmt.Errorf("%w: %s", ErrClaudeNotFound, output)
		}
		return result, fmt.Errorf("claude execution failed: %w", err)
	}

	// Extract session ID from output
	result.SessionID = e.extractSessionID(output)

	return result, nil
}

// RunFollow executes a Claude Code prompt and streams output in real-time.
// Blocks until execution completes.
func (e *Executor) RunFollow(containerID string, prompt string, opts ExecuteOptions, output io.Writer) (*ExecutionResult, error) {
	if !e.docker.ContainerRunning(containerID) {
		return nil, errors.New("container is not running")
	}

	cmd := e.buildCommand(prompt, opts)
	startedAt := time.Now()

	// Create a buffer to capture output while streaming
	var outputBuf bytes.Buffer
	multiWriter := io.MultiWriter(output, &outputBuf)

	// Execute with streaming output
	execOpts := docker.ExecOptions{
		Stdout: multiWriter,
		Stderr: multiWriter,
		TTY:    false,
	}

	err := e.docker.Exec(containerID, cmd, execOpts)
	completedAt := time.Now()

	result := &ExecutionResult{
		Output:      outputBuf.String(),
		StartedAt:   startedAt,
		CompletedAt: completedAt,
	}

	if err != nil {
		result.Error = err
		result.ExitCode = 1
		return result, fmt.Errorf("claude execution failed: %w", err)
	}

	// Extract session ID from captured output
	result.SessionID = e.extractSessionID(outputBuf.String())

	return result, nil
}

// RunRalph executes a Claude Code prompt in Ralph mode (autonomous loop).
// Repeats the prompt until a completion signal is detected or max iterations reached.
func (e *Executor) RunRalph(containerID string, prompt string, opts RalphOptions, output io.Writer) (*RalphResult, error) {
	if !e.docker.ContainerRunning(containerID) {
		return nil, errors.New("container is not running")
	}

	// Apply defaults
	if opts.MaxIterations == 0 {
		opts.MaxIterations = 30
	}
	if opts.CompletionSignal == "" {
		opts.CompletionSignal = "DONE"
	}
	if opts.CooldownSeconds == 0 {
		opts.CooldownSeconds = 5
	}

	result := &RalphResult{
		ExecutionResult: ExecutionResult{
			StartedAt: time.Now(),
		},
	}

	_, _ = fmt.Fprintf(output, "Running Ralph mode (max %d iterations, signal: %q)\n\n",
		opts.MaxIterations, opts.CompletionSignal)

	// Loop until completion
	for i := 1; i <= opts.MaxIterations; i++ {
		result.Iterations = i
		_, _ = fmt.Fprintf(output, "\n=== Ralph iteration %d/%d ===\n", i, opts.MaxIterations)

		// Run single iteration
		iterResult, err := e.runSingleIteration(containerID, prompt, opts.ExecuteOptions, output)
		if err != nil {
			result.CompletedBy = "error"
			result.Error = err
			result.CompletedAt = time.Now()
			return result, fmt.Errorf("iteration %d failed: %w", i, err)
		}

		// Store session ID from first iteration
		if i == 1 && result.SessionID == "" {
			result.SessionID = iterResult.SessionID
		}

		// Check for completion signal in output
		if strings.Contains(iterResult.Output, opts.CompletionSignal) {
			_, _ = fmt.Fprintf(output, "\n=== Completion signal detected: %s ===\n", opts.CompletionSignal)
			result.CompletedBy = "signal"
			result.CompletedAt = time.Now()
			return result, nil
		}

		// Run verify command if specified
		if opts.VerifyCommand != "" {
			_, _ = fmt.Fprintf(output, "\n--- Running verify command: %s ---\n", opts.VerifyCommand)
			err := e.runVerifyCommand(containerID, opts.VerifyCommand, output)
			if err == nil {
				_, _ = fmt.Fprintf(output, "\n=== Verify command passed ===\n")
				result.CompletedBy = "verify"
				result.CompletedAt = time.Now()
				return result, nil
			}
			_, _ = fmt.Fprintf(output, "Verify failed: %v\n", err)
		}

		// Cooldown between iterations
		if i < opts.MaxIterations {
			_, _ = fmt.Fprintf(output, "\n--- Cooldown: %ds ---\n", opts.CooldownSeconds)
			time.Sleep(time.Duration(opts.CooldownSeconds) * time.Second)
		}
	}

	// Reached max iterations without completion
	result.CompletedBy = "max_iterations"
	result.CompletedAt = time.Now()
	result.Error = ErrMaxIterations

	_, _ = fmt.Fprintf(output, "\n=== Reached max iterations (%d) ===\n", opts.MaxIterations)
	return result, ErrMaxIterations
}

// IsRunning checks if Claude Code is currently executing in a container.
// This is a best-effort check by looking for claude processes.
func (e *Executor) IsRunning(containerID string) (bool, error) {
	if !e.docker.ContainerRunning(containerID) {
		return false, nil
	}

	// Check for running claude processes
	output, err := e.docker.ExecWithOutput(containerID, []string{"pgrep", "-f", "claude"})
	if err != nil {
		// pgrep returns exit code 1 if no processes found (not an error)
		return false, nil
	}

	// If output is non-empty, claude is running
	return strings.TrimSpace(output) != "", nil
}

// buildCommand constructs the Claude Code command line arguments.
func (e *Executor) buildCommand(prompt string, opts ExecuteOptions) []string {
	cmd := []string{"claude", "-p", prompt}

	// Use stream-json format for machine-parseable output
	// --verbose is required when using --print with --output-format stream-json
	cmd = append(cmd, "--output-format", "stream-json", "--verbose")

	// Allowed tools
	if len(opts.AllowedTools) > 0 {
		cmd = append(cmd, "--allowedTools", strings.Join(opts.AllowedTools, ","))
	}

	// Disallowed tools
	if len(opts.DisallowedTools) > 0 {
		cmd = append(cmd, "--disallowedTools", strings.Join(opts.DisallowedTools, ","))
	}

	// Max turns
	if opts.MaxTurns > 0 {
		cmd = append(cmd, "--max-turns", strconv.Itoa(opts.MaxTurns))
	}

	// Model
	if opts.Model != "" {
		cmd = append(cmd, "--model", opts.Model)
	}

	// System prompt
	if opts.SystemPrompt != "" {
		cmd = append(cmd, "--append-system-prompt", opts.SystemPrompt)
	}

	return cmd
}

// runSingleIteration executes a single Ralph iteration.
func (e *Executor) runSingleIteration(containerID string, prompt string, opts ExecuteOptions, output io.Writer) (*ExecutionResult, error) {
	cmd := e.buildCommand(prompt, opts)
	startedAt := time.Now()

	// Create a buffer to capture output while streaming
	var outputBuf bytes.Buffer
	multiWriter := io.MultiWriter(output, &outputBuf)

	// Execute with streaming output
	execOpts := docker.ExecOptions{
		Stdout: multiWriter,
		Stderr: multiWriter,
		TTY:    false,
	}

	err := e.docker.Exec(containerID, cmd, execOpts)
	completedAt := time.Now()

	result := &ExecutionResult{
		Output:      outputBuf.String(),
		StartedAt:   startedAt,
		CompletedAt: completedAt,
	}

	if err != nil {
		result.Error = err
		result.ExitCode = 1
		return result, err
	}

	// Extract session ID
	result.SessionID = e.extractSessionID(outputBuf.String())

	return result, nil
}

// runVerifyCommand executes a verification command and returns nil if it succeeds.
func (e *Executor) runVerifyCommand(containerID string, command string, output io.Writer) error {
	// Parse the command string into args
	args := parseCommand(command)
	if len(args) == 0 {
		return errors.New("empty verify command")
	}

	execOpts := docker.ExecOptions{
		Stdout: output,
		Stderr: output,
		TTY:    false,
	}

	return e.docker.Exec(containerID, args, execOpts)
}

// extractSessionID parses stream-json output to find the session ID.
func (e *Executor) extractSessionID(output string) string {
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			continue
		}

		var msg StreamMessage
		if err := json.Unmarshal([]byte(line), &msg); err == nil {
			if msg.SessionID != "" {
				return msg.SessionID
			}
		}
	}
	return ""
}

// parseCommand splits a command string into arguments.
// This is a simple implementation that handles quoted strings.
func parseCommand(cmd string) []string {
	var args []string
	var current strings.Builder
	inQuote := false
	quoteChar := rune(0)

	for _, r := range cmd {
		switch {
		case r == '"' || r == '\'':
			if inQuote && r == quoteChar {
				inQuote = false
				quoteChar = 0
			} else if !inQuote {
				inQuote = true
				quoteChar = r
			} else {
				current.WriteRune(r)
			}
		case r == ' ' && !inQuote:
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		args = append(args, current.String())
	}

	return args
}

// VerifyClaudeInstalled checks if Claude Code is installed in the container.
func (e *Executor) VerifyClaudeInstalled(containerID string) error {
	if !e.docker.ContainerRunning(containerID) {
		return errors.New("container is not running")
	}

	output, err := e.docker.ExecWithOutput(containerID, []string{"which", "claude"})
	if err != nil || strings.TrimSpace(output) == "" {
		return ErrClaudeNotFound
	}

	return nil
}

// GetClaudeVersion returns the Claude Code version installed in the container.
func (e *Executor) GetClaudeVersion(containerID string) (string, error) {
	if !e.docker.ContainerRunning(containerID) {
		return "", errors.New("container is not running")
	}

	// Try to get version - some CLIs use --version, some use version subcommand
	output, err := e.docker.ExecWithOutput(containerID, []string{"claude", "--version"})
	if err != nil {
		// Try alternative
		output, err = e.docker.ExecWithOutput(containerID, []string{"claude", "version"})
		if err != nil {
			return "", fmt.Errorf("failed to get claude version: %w", err)
		}
	}

	return strings.TrimSpace(output), nil
}

// CheckContainer verifies the container is ready for Claude Code execution.
func (e *Executor) CheckContainer(containerID string) error {
	if !e.docker.ContainerRunning(containerID) {
		return errors.New("container is not running")
	}

	// Verify Claude Code is installed
	return e.VerifyClaudeInstalled(containerID)
}

// CommandFromShell parses a shell command string into a Docker exec command.
// This is useful for verify commands that may include pipes, redirects, etc.
func CommandFromShell(shellCmd string) []string {
	return []string{"sh", "-c", shellCmd}
}

func init() {
	// Ensure exec.Command works in tests
	_ = exec.Command
}
