---
id: TANK-017
title: Command - tanuki status
status: done
priority: medium
estimate: S
depends_on: [TANK-003]
workstream: D
phase: 1
---

# Command: tanuki status

## Summary

Implement the `tanuki status` command that shows detailed status for a specific agent.

## Acceptance Criteria

- [x] Shows comprehensive agent status
- [x] Includes: container status, git status, last task info
- [x] Resource usage (memory, CPU) if available
- [x] Time since last activity
- [x] Support JSON output for scripting

## Technical Details

### Command Definition

```go
var statusCmd = &cobra.Command{
    Use:   "status <agent>",
    Short: "Show detailed agent status",
    Long: `Show detailed status information for an agent.

Examples:
  tanuki status auth-feature
  tanuki status auth-feature -o json`,
    Args: cobra.ExactArgs(1),
    RunE: runStatus,
}

func init() {
    statusCmd.Flags().StringP("output", "o", "text", "Output format (text, json)")
    rootCmd.AddCommand(statusCmd)
}
```

### Implementation

```go
func runStatus(cmd *cobra.Command, args []string) error {
    agentName := args[0]
    outputFormat, _ := cmd.Flags().GetString("output")

    cfg, _ := config.Load()
    agentMgr := agent.NewManager(cfg)

    status, err := agentMgr.Status(agentName)
    if err != nil {
        return fmt.Errorf("agent %q not found", agentName)
    }

    switch outputFormat {
    case "json":
        return printStatusJSON(status)
    default:
        return printStatusText(status)
    }
}

func printStatusText(s *agent.AgentStatus) error {
    fmt.Printf("Agent: %s\n", s.Name)
    fmt.Printf("Status: %s\n", colorStatus(s.Status))
    fmt.Printf("Uptime: %s\n", formatDuration(s.Uptime))
    fmt.Println()

    fmt.Println("Container:")
    fmt.Printf("  ID:      %s\n", s.Container.ID)
    fmt.Printf("  Running: %v\n", s.Container.Running)
    if s.Container.Memory != "" {
        fmt.Printf("  Memory:  %s\n", s.Container.Memory)
    }
    if s.Container.CPU != "" {
        fmt.Printf("  CPU:     %s\n", s.Container.CPU)
    }
    fmt.Println()

    fmt.Println("Git:")
    fmt.Printf("  Branch:  %s\n", s.Git.Branch)
    fmt.Printf("  Changes: %v\n", s.Git.HasChanges)
    if s.Git.CommitsAhead > 0 {
        fmt.Printf("  Ahead:   %d commits\n", s.Git.CommitsAhead)
    }
    fmt.Println()

    if s.LastTask != nil {
        fmt.Println("Last Task:")
        fmt.Printf("  Prompt:    %s\n", truncate(s.LastTask.Prompt, 60))
        fmt.Printf("  Started:   %s\n", s.LastTask.StartedAt.Format(time.RFC3339))
        if !s.LastTask.CompletedAt.IsZero() {
            fmt.Printf("  Completed: %s\n", s.LastTask.CompletedAt.Format(time.RFC3339))
            duration := s.LastTask.CompletedAt.Sub(s.LastTask.StartedAt)
            fmt.Printf("  Duration:  %s\n", formatDuration(duration))
        }
        if s.LastTask.SessionID != "" {
            fmt.Printf("  Session:   %s\n", s.LastTask.SessionID)
        }
    }

    return nil
}

func printStatusJSON(s *agent.AgentStatus) error {
    enc := json.NewEncoder(os.Stdout)
    enc.SetIndent("", "  ")
    return enc.Encode(s)
}
```

### Container Resource Usage

```go
func (d *DockerManager) GetResourceUsage(containerID string) (*ResourceUsage, error) {
    cmd := exec.Command("docker", "stats", "--no-stream", "--format",
        "{{.MemUsage}}\t{{.CPUPerc}}", containerID)
    output, err := cmd.Output()
    if err != nil {
        return nil, err
    }

    parts := strings.Split(strings.TrimSpace(string(output)), "\t")
    return &ResourceUsage{
        Memory: parts[0],
        CPU:    parts[1],
    }, nil
}
```

### Output

```
$ tanuki status auth-feature
Agent: auth-feature
Status: working
Uptime: 23m

Container:
  ID:      abc123def456
  Running: true
  Memory:  256MiB / 4GiB
  CPU:     12.5%

Git:
  Branch:  tanuki/auth-feature
  Changes: true
  Ahead:   3 commits

Last Task:
  Prompt:    Implement OAuth2 login with Google
  Started:   2026-01-13T10:15:00Z
  Duration:  (in progress)
  Session:   session-xyz123
```

```
$ tanuki status auth-feature -o json
{
  "name": "auth-feature",
  "status": "working",
  "uptime": "23m15s",
  "container": {
    "id": "abc123def456",
    "running": true,
    "memory": "256MiB / 4GiB",
    "cpu": "12.5%"
  },
  "git": {
    "branch": "tanuki/auth-feature",
    "has_changes": true,
    "commits_ahead": 3
  },
  "last_task": {
    "prompt": "Implement OAuth2 login with Google",
    "started_at": "2026-01-13T10:15:00Z",
    "session_id": "session-xyz123"
  }
}
```

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--output` | `-o` | Output format (text, json) |

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Agent not found | Error with list suggestion |
| Container not running | Show status as stopped, no resource usage |
| Stats unavailable | Omit resource usage section |

## Out of Scope

- Historical task list
- Live resource monitoring

## Notes

This is the detailed view for a single agent. Use `tanuki list` for overview of all agents.
