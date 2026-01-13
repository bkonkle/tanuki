---
id: TANK-007
title: Claude Code Executor
status: done
priority: high
estimate: L
depends_on: [TANK-002, TANK-005]
workstream: C
phase: 1
---

# Claude Code Executor

## Summary

Implement the execution layer for running Claude Code commands inside agent containers. Supports three execution modes: fire-and-forget, follow (streaming), and Ralph mode (autonomous loop).

## Acceptance Criteria

- [x] Execute Claude Code with `-p` flag in container
- [x] Pass `--allowedTools` from config
- [x] **Fire-and-forget mode**: detached execution, returns immediately
- [x] **Follow mode**: stream output in real-time
- [x] **Ralph mode**: loop until completion signal or max iterations
- [x] Capture and store session ID for potential resume
- [x] Update agent status during execution (idle → working → idle)
- [x] Handle Claude Code errors gracefully
- [x] Support custom system prompts via `--append-system-prompt`

## Technical Details

### Executor Interface

```go
type ClaudeExecutor interface {
    // Run executes a prompt (detached, fire-and-forget)
    Run(containerID string, prompt string, opts ExecuteOptions) error

    // RunFollow executes and streams output until complete
    RunFollow(containerID string, prompt string, opts ExecuteOptions, output io.Writer) (*ExecutionResult, error)

    // RunRalph executes in loop mode until completion signal or max iterations
    RunRalph(containerID string, prompt string, opts RalphOptions, output io.Writer) (*RalphResult, error)

    // IsRunning checks if Claude is currently executing in a container
    IsRunning(containerID string) (bool, error)
}

type ExecuteOptions struct {
    AllowedTools       []string
    DisallowedTools    []string
    MaxTurns           int
    Model              string
    SystemPrompt       string   // Appended to default system prompt
    WorkDir            string   // Working directory inside container
}

type RalphOptions struct {
    ExecuteOptions
    MaxIterations    int      // Max loop iterations (default: 30)
    CompletionSignal string   // String that signals done (default: "DONE")
    VerifyCommand    string   // Optional command to verify success
    CooldownSeconds  int      // Pause between iterations (default: 5)
}

type ExecutionResult struct {
    SessionID   string
    Output      string
    ExitCode    int
    StartedAt   time.Time
    CompletedAt time.Time
}

type RalphResult struct {
    ExecutionResult
    Iterations      int
    CompletedBy     string   // "signal", "verify", "max_iterations", "error"
}
```

### Command Construction

```go
func (e *ClaudeExecutor) buildCommand(prompt string, opts ExecuteOptions) []string {
    cmd := []string{"claude", "-p", prompt}

    // Output format for machine parsing
    cmd = append(cmd, "--output-format", "stream-json")

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
```

### Execution Flow

```go
func (e *ClaudeExecutor) Run(containerID string, prompt string, opts ExecuteOptions) (*ExecutionResult, error) {
    cmd := e.buildCommand(prompt, opts)
    startedAt := time.Now()

    // Execute in container
    output, err := e.docker.ExecWithOutput(containerID, cmd)
    completedAt := time.Now()

    if err != nil {
        return nil, fmt.Errorf("claude execution failed: %w", err)
    }

    // Parse output for session ID and result
    result := &ExecutionResult{
        Output:      output,
        StartedAt:   startedAt,
        CompletedAt: completedAt,
    }

    // Extract session ID from stream-json output
    result.SessionID = e.extractSessionID(output)

    return result, nil
}
```

### Streaming Execution

```go
func (e *ClaudeExecutor) RunStreaming(containerID string, prompt string, opts ExecuteOptions, output io.Writer) error {
    cmd := e.buildCommand(prompt, opts)

    // Create exec with TTY for streaming
    execOpts := docker.ExecOptions{
        Stdout: output,
        Stderr: output,
        TTY:    false, // No TTY for clean output
    }

    return e.docker.Exec(containerID, cmd, execOpts)
}
```

### Output Parsing (stream-json format)

```go
type StreamMessage struct {
    Type      string `json:"type"`
    SessionID string `json:"session_id,omitempty"`
    Content   string `json:"content,omitempty"`
    // ... other fields
}

func (e *ClaudeExecutor) extractSessionID(output string) string {
    lines := strings.Split(output, "\n")
    for _, line := range lines {
        var msg StreamMessage
        if err := json.Unmarshal([]byte(line), &msg); err == nil {
            if msg.SessionID != "" {
                return msg.SessionID
            }
        }
    }
    return ""
}
```

### Integration with Agent Manager

```go
// In agent/manager.go
func (m *AgentManager) Run(name string, prompt string, opts RunOptions) error {
    agent, err := m.state.GetAgent(name)
    if err != nil {
        return err
    }

    // Check if already working
    if agent.Status == "working" {
        return fmt.Errorf("agent %q is already working on a task", name)
    }

    // Update status
    agent.Status = "working"
    agent.UpdatedAt = time.Now()
    m.state.SetAgent(agent)

    // Build execute options
    execOpts := ExecuteOptions{
        AllowedTools: opts.AllowedTools,
        MaxTurns:     opts.MaxTurns,
        SystemPrompt: opts.SystemPrompt,
    }
    if len(execOpts.AllowedTools) == 0 {
        execOpts.AllowedTools = m.config.Defaults.AllowedTools
    }

    // Execute (with or without streaming)
    var result *ExecutionResult
    var execErr error

    if opts.Follow {
        execErr = m.executor.RunStreaming(agent.ContainerID, prompt, execOpts, os.Stdout)
    } else {
        result, execErr = m.executor.Run(agent.ContainerID, prompt, execOpts)
    }

    // Update status back to idle
    agent.Status = "idle"
    agent.UpdatedAt = time.Now()
    agent.LastTask = &TaskInfo{
        Prompt:      prompt,
        StartedAt:   result.StartedAt,
        CompletedAt: result.CompletedAt,
        SessionID:   result.SessionID,
    }
    m.state.SetAgent(agent)

    return execErr
}
```

### Default Allowed Tools

```go
var defaultAllowedTools = []string{
    "Read",
    "Write",
    "Edit",
    "Bash",
    "Glob",
    "Grep",
    "TodoWrite",
}
```

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Claude Code not installed | Clear error with install instructions |
| Auth failure | Suggest checking ~/.config/claude-code mount |
| Rate limit | Report error, suggest waiting |
| Container not running | Offer to start it |
| Max turns exceeded | Report completion status |

### Ralph Loop Implementation

The Ralph loop pattern: run the same prompt repeatedly until a completion signal is detected or a verify command succeeds.

```go
func (e *ClaudeExecutor) RunRalph(containerID string, prompt string, opts RalphOptions, output io.Writer) (*RalphResult, error) {
    result := &RalphResult{
        ExecutionResult: ExecutionResult{StartedAt: time.Now()},
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

    for i := 1; i <= opts.MaxIterations; i++ {
        result.Iterations = i
        fmt.Fprintf(output, "\n=== Ralph iteration %d/%d ===\n", i, opts.MaxIterations)

        // Run single iteration
        iterOutput, err := e.runSingleIteration(containerID, prompt, opts.ExecuteOptions)
        if err != nil {
            result.CompletedBy = "error"
            return result, err
        }

        // Check for completion signal in output
        if strings.Contains(iterOutput, opts.CompletionSignal) {
            fmt.Fprintf(output, "\n=== Completion signal detected: %s ===\n", opts.CompletionSignal)
            result.CompletedBy = "signal"
            result.CompletedAt = time.Now()
            return result, nil
        }

        // Run verify command if specified
        if opts.VerifyCommand != "" {
            if err := e.runVerifyCommand(containerID, opts.VerifyCommand); err == nil {
                fmt.Fprintf(output, "\n=== Verify command passed: %s ===\n", opts.VerifyCommand)
                result.CompletedBy = "verify"
                result.CompletedAt = time.Now()
                return result, nil
            }
        }

        // Cooldown between iterations
        if i < opts.MaxIterations {
            time.Sleep(time.Duration(opts.CooldownSeconds) * time.Second)
        }
    }

    result.CompletedBy = "max_iterations"
    result.CompletedAt = time.Now()
    return result, fmt.Errorf("reached max iterations (%d) without completion", opts.MaxIterations)
}
```

### Why Ralph Mode Works

1. **Fresh context each iteration** - Avoids context window bloat
2. **Filesystem as memory** - Progress persists in code, not in context
3. **Objective completion** - Either signal detected or verify command passes
4. **Predictable failure** - Max iterations prevents runaway loops

## Out of Scope

- Session resume (`--resume` flag - future enhancement)
- Conversation continuation (`--continue` flag - future enhancement)
- Multi-turn interactive mode
- Cost tracking per iteration (future enhancement)

## Notes

The `-p` flag is essential - it makes Claude Code non-interactive. Always use `--output-format stream-json` for machine-parseable output even if not streaming to the user.

Ralph mode is particularly useful for:

- **Refactoring**: "Refactor to reduce duplication. Run tests. Say DONE when tests pass."
- **Test coverage**: "Add tests until coverage reaches 80%. Say DONE when complete."
- **Linting**: "Fix all ESLint errors. Say DONE when `npm run lint` passes."
