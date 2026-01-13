---
id: TANK-011
title: Command - tanuki run
status: done
priority: high
estimate: M
depends_on: [TANK-006, TANK-007]
workstream: D
phase: 1
---

# Command: tanuki run

## Summary

Implement the `tanuki run` command that sends a task to an agent. Supports three execution modes: fire-and-forget (default), follow, and Ralph mode.

## Acceptance Criteria

- [x] Sends prompt to agent via Claude Code `-p` flag
- [x] **Default**: runs asynchronously (returns immediately)
- [x] **`--follow`**: streams output in real-time
- [x] **`--ralph`**: loops until completion signal or max iterations
- [x] Updates agent status during execution
- [x] Stores task info in state for later reference
- [x] Validates agent exists and is not already working

## Technical Details

### Command Definition

```go
var runCmd = &cobra.Command{
    Use:   "run <agent> <prompt>",
    Short: "Send a task to an agent",
    Long: `Send a task to an agent in one of three modes:

  Default:   Fire-and-forget, returns immediately
  --follow:  Stream output until complete
  --ralph:   Loop until completion signal (autonomous mode)

Examples:
  tanuki run auth "Implement OAuth2 login"
  tanuki run auth "Add unit tests" --follow
  tanuki run auth "Fix all lint errors. Say DONE when clean." --ralph
  tanuki run auth "Increase coverage to 80%" --ralph --verify "npm test -- --coverage"`,
    Args: cobra.ExactArgs(2),
    RunE: runRun,
}

func init() {
    // Execution mode flags
    runCmd.Flags().BoolP("follow", "f", false, "Follow output in real-time")
    runCmd.Flags().Bool("ralph", false, "Run in Ralph mode (loop until done)")

    // Ralph mode options
    runCmd.Flags().Int("max-iter", 30, "Max iterations in Ralph mode")
    runCmd.Flags().String("signal", "DONE", "Completion signal for Ralph mode")
    runCmd.Flags().String("verify", "", "Command to verify completion (e.g., 'npm test')")

    // Execution options
    runCmd.Flags().IntP("max-turns", "t", 0, "Max conversation turns per iteration")
    runCmd.Flags().StringSliceP("allow", "a", nil, "Additional allowed tools")
    runCmd.Flags().StringSliceP("deny", "d", nil, "Disallowed tools")

    rootCmd.AddCommand(runCmd)
}
```

### Implementation

```go
func runRun(cmd *cobra.Command, args []string) error {
    agentName := args[0]
    prompt := args[1]

    follow, _ := cmd.Flags().GetBool("follow")
    maxTurns, _ := cmd.Flags().GetInt("max-turns")
    allowTools, _ := cmd.Flags().GetStringSlice("allow")
    denyTools, _ := cmd.Flags().GetStringSlice("deny")

    cfg, err := config.Load()
    if err != nil {
        return err
    }

    agentMgr := agent.NewManager(cfg)

    // Check agent exists and is available
    ag, err := agentMgr.Get(agentName)
    if err != nil {
        return fmt.Errorf("agent %q not found", agentName)
    }

    if ag.Status == "working" {
        return fmt.Errorf("agent %q is already working on a task\nUse 'tanuki logs %s' to see progress", agentName, agentName)
    }

    if ag.Status == "stopped" {
        return fmt.Errorf("agent %q is stopped\nUse 'tanuki start %s' first", agentName, agentName)
    }

    // Build run options
    opts := agent.RunOptions{
        Follow:          follow,
        MaxTurns:        maxTurns,
        AllowedTools:    allowTools,
        DisallowedTools: denyTools,
    }

    if follow {
        fmt.Printf("Running task on %s (following output)...\n\n", agentName)
        return agentMgr.Run(agentName, prompt, opts)
    }

    // Async mode - start in background
    fmt.Printf("Task sent to %s\n", agentName)
    fmt.Printf("  Prompt: %s\n", truncate(prompt, 60))
    fmt.Println()
    fmt.Printf("Check progress:\n")
    fmt.Printf("  tanuki logs %s --follow\n", agentName)
    fmt.Printf("  tanuki status %s\n", agentName)

    // Start task in background goroutine
    go func() {
        agentMgr.Run(agentName, prompt, opts)
    }()

    return nil
}

func truncate(s string, max int) string {
    if len(s) <= max {
        return s
    }
    return s[:max-3] + "..."
}
```

### Async Execution Pattern

For async execution, we need to ensure the task keeps running after the CLI exits. Options:

**Option A: Container runs task directly**
The task runs inside the container, so even if CLI exits, the container keeps running.

```go
func (m *AgentManager) Run(name string, prompt string, opts RunOptions) error {
    agent, _ := m.state.GetAgent(name)

    // Update status to working
    agent.Status = "working"
    agent.LastTask = &TaskInfo{
        Prompt:    prompt,
        StartedAt: time.Now(),
    }
    m.state.SetAgent(agent)

    if opts.Follow {
        // Blocking: stream output
        err := m.executor.RunStreaming(agent.ContainerID, prompt, execOpts, os.Stdout)
        agent.Status = "idle"
        m.state.SetAgent(agent)
        return err
    }

    // Non-blocking: run in detached mode
    go func() {
        m.executor.Run(agent.ContainerID, prompt, execOpts)
        agent.Status = "idle"
        agent.LastTask.CompletedAt = time.Now()
        m.state.SetAgent(agent)
    }()

    return nil
}
```

**Option B: Use `docker exec -d` for detached execution**
```go
func (d *DockerManager) ExecDetached(containerID string, cmd []string) error {
    args := append([]string{"exec", "-d", containerID}, cmd...)
    return exec.Command("docker", args...).Run()
}
```

### Output

```
$ tanuki run auth-feature "Implement OAuth2 login with Google"
Task sent to auth-feature
  Prompt: Implement OAuth2 login with Google

Check progress:
  tanuki logs auth-feature --follow
  tanuki status auth-feature
```

```
$ tanuki run auth-feature "Implement OAuth2 login" --follow
Running task on auth-feature (following output)...

[Claude Code output streams here...]
```

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--follow` | `-f` | Follow output in real-time |
| `--ralph` | | Run in Ralph mode (loop until done) |
| `--max-iter` | | Max iterations in Ralph mode (default: 30) |
| `--signal` | | Completion signal for Ralph mode (default: "DONE") |
| `--verify` | | Command to verify completion |
| `--max-turns` | `-t` | Max conversation turns per iteration |
| `--allow` | `-a` | Additional allowed tools |
| `--deny` | `-d` | Disallowed tools |

### Output Examples

**Default mode:**

```text
$ tanuki run auth "Implement OAuth2 login"
Task sent to auth
  Prompt: Implement OAuth2 login

Check progress:
  tanuki logs auth --follow
  tanuki status auth
```

**Ralph mode:**

```text
$ tanuki run auth "Fix lint errors. Say DONE when clean." --ralph
Running auth in Ralph mode (max 30 iterations)...

=== Ralph iteration 1/30 ===
[Claude output...]

=== Ralph iteration 2/30 ===
[Claude output...]

=== Completion signal detected: DONE ===
Completed in 2 iterations (3m 24s)
```

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Agent not found | Error with list suggestion |
| Agent already working | Error with logs suggestion |
| Agent stopped | Error with start suggestion |
| Empty prompt | Error: prompt required |
| Ralph max iterations | Warning with partial progress |

## Out of Scope

- Reading prompt from file
- Multiple agents at once (future bulk command)

## Notes

The three modes serve different use cases:

- **Default** - Quick tasks, check back later
- **Follow** - Watch progress in real-time, good for debugging
- **Ralph** - Autonomous completion, good for overnight or long-running tasks
