---
id: TANK-012
title: Command - tanuki logs
status: done
priority: medium
estimate: S
depends_on: [TANK-005]
workstream: D
phase: 1
---

# Command: tanuki logs

## Summary

Implement the `tanuki logs` command that shows output from an agent's container.

## Acceptance Criteria

- [x] Shows container stdout/stderr
- [x] `--follow` flag for real-time streaming
- [x] `--tail` flag to limit lines shown
- [x] `--all` flag to show logs from all agents
- [x] Handles case where agent has no output yet

## Technical Details

### Command Definition

```go
var logsCmd = &cobra.Command{
    Use:   "logs <agent>",
    Short: "Show agent output",
    Long: `Show output from an agent's container.

Examples:
  tanuki logs auth-feature
  tanuki logs auth-feature --follow
  tanuki logs auth-feature --tail 100
  tanuki logs --all`,
    Args: cobra.MaximumNArgs(1),
    RunE: runLogs,
}

func init() {
    logsCmd.Flags().BoolP("follow", "f", false, "Follow log output")
    logsCmd.Flags().IntP("tail", "n", 0, "Number of lines to show from end (0 = all)")
    logsCmd.Flags().Bool("all", false, "Show logs from all agents")
    rootCmd.AddCommand(logsCmd)
}
```

### Implementation

```go
func runLogs(cmd *cobra.Command, args []string) error {
    follow, _ := cmd.Flags().GetBool("follow")
    tail, _ := cmd.Flags().GetInt("tail")
    all, _ := cmd.Flags().GetBool("all")

    cfg, err := config.Load()
    if err != nil {
        return err
    }

    agentMgr := agent.NewManager(cfg)

    if all {
        return showAllLogs(agentMgr, follow, tail)
    }

    if len(args) == 0 {
        return fmt.Errorf("agent name required (or use --all)")
    }

    agentName := args[0]
    ag, err := agentMgr.Get(agentName)
    if err != nil {
        return fmt.Errorf("agent %q not found", agentName)
    }

    return streamLogs(ag, follow, tail)
}

func streamLogs(ag *agent.Agent, follow bool, tail int) error {
    args := []string{"logs"}

    if follow {
        args = append(args, "-f")
    }

    if tail > 0 {
        args = append(args, "--tail", strconv.Itoa(tail))
    }

    args = append(args, ag.ContainerID)

    cmd := exec.Command("docker", args...)
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr

    return cmd.Run()
}

func showAllLogs(agentMgr *agent.Manager, follow bool, tail int) error {
    agents, err := agentMgr.List()
    if err != nil {
        return err
    }

    if len(agents) == 0 {
        fmt.Println("No agents found.")
        return nil
    }

    // For --all without --follow, show sequentially
    if !follow {
        for _, ag := range agents {
            fmt.Printf("=== %s ===\n", ag.Name)
            streamLogs(ag, false, tail)
            fmt.Println()
        }
        return nil
    }

    // For --all with --follow, use goroutines with prefixed output
    var wg sync.WaitGroup
    for _, ag := range agents {
        wg.Add(1)
        go func(ag *agent.Agent) {
            defer wg.Done()
            streamLogsWithPrefix(ag, ag.Name)
        }(ag)
    }
    wg.Wait()

    return nil
}

func streamLogsWithPrefix(ag *agent.Agent, prefix string) {
    cmd := exec.Command("docker", "logs", "-f", ag.ContainerID)
    stdout, _ := cmd.StdoutPipe()
    cmd.Start()

    scanner := bufio.NewScanner(stdout)
    for scanner.Scan() {
        fmt.Printf("[%s] %s\n", prefix, scanner.Text())
    }
    cmd.Wait()
}
```

### Output

```
$ tanuki logs auth-feature
[Output from Claude Code session...]
```

```
$ tanuki logs auth-feature --follow
[Streaming output as Claude Code runs...]
```

```
$ tanuki logs --all
=== auth-feature ===
[auth-feature output...]

=== api-refactor ===
[api-refactor output...]
```

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--follow` | `-f` | Follow log output |
| `--tail` | `-n` | Number of lines from end |
| `--all` | | Show all agents |

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Agent not found | Error with list suggestion |
| Container not running | Show existing logs, note container is stopped |
| No output yet | Empty output (not an error) |

## Out of Scope

- Log persistence/history
- Log searching

## Notes

Docker logs include both stdout and stderr. The `--follow` flag is essential for monitoring active tasks.
