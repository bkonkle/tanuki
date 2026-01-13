---
id: TANK-010
title: Command - tanuki list
status: done
priority: high
estimate: S
depends_on: [TANK-003]
workstream: D
phase: 1
---

# Command: tanuki list

## Summary

Implement the `tanuki list` command that shows all agents and their current status.

## Acceptance Criteria

- [x] Shows table of all agents with status
- [x] Includes: name, status, branch, uptime
- [x] Reconciles state with actual container status
- [x] Supports JSON output for scripting
- [x] Shows helpful message when no agents exist

## Technical Details

### Command Definition

```go
var listCmd = &cobra.Command{
    Use:     "list",
    Aliases: []string{"ls"},
    Short:   "List all agents",
    Long:    `List all agents and their current status.`,
    RunE:    runList,
}

func init() {
    listCmd.Flags().StringP("output", "o", "table", "Output format (table, json)")
    rootCmd.AddCommand(listCmd)
}
```

### Implementation

```go
func runList(cmd *cobra.Command, args []string) error {
    outputFormat, _ := cmd.Flags().GetString("output")

    cfg, err := config.Load()
    if err != nil {
        return err
    }

    agentMgr := agent.NewManager(cfg)
    agents, err := agentMgr.List()
    if err != nil {
        return err
    }

    if len(agents) == 0 {
        fmt.Println("No agents found.")
        fmt.Println("\nCreate one with:")
        fmt.Println("  tanuki spawn <name>")
        return nil
    }

    switch outputFormat {
    case "json":
        return printJSON(agents)
    default:
        return printTable(agents)
    }
}

func printTable(agents []*agent.Agent) error {
    w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
    fmt.Fprintln(w, "NAME\tSTATUS\tBRANCH\tUPTIME")
    fmt.Fprintln(w, "----\t------\t------\t------")

    for _, ag := range agents {
        uptime := formatDuration(time.Since(ag.CreatedAt))
        fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
            ag.Name,
            colorStatus(ag.Status),
            ag.Branch,
            uptime,
        )
    }

    return w.Flush()
}

func colorStatus(status string) string {
    switch status {
    case "idle":
        return color.GreenString(status)
    case "working":
        return color.YellowString(status)
    case "stopped":
        return color.RedString(status)
    case "error":
        return color.RedString(status)
    default:
        return status
    }
}

func formatDuration(d time.Duration) string {
    if d < time.Minute {
        return fmt.Sprintf("%ds", int(d.Seconds()))
    }
    if d < time.Hour {
        return fmt.Sprintf("%dm", int(d.Minutes()))
    }
    if d < 24*time.Hour {
        return fmt.Sprintf("%dh", int(d.Hours()))
    }
    return fmt.Sprintf("%dd", int(d.Hours()/24))
}

func printJSON(agents []*agent.Agent) error {
    enc := json.NewEncoder(os.Stdout)
    enc.SetIndent("", "  ")
    return enc.Encode(agents)
}
```

### Output

```
$ tanuki list
NAME           STATUS    BRANCH                    UPTIME
----           ------    ------                    ------
auth-feature   working   tanuki/auth-feature       5m
api-refactor   idle      tanuki/api-refactor       12m
test-coverage  stopped   tanuki/test-coverage      1h
```

```
$ tanuki list -o json
[
  {
    "name": "auth-feature",
    "container_id": "abc123...",
    "branch": "tanuki/auth-feature",
    "status": "working",
    "created_at": "2026-01-13T10:00:00Z",
    "updated_at": "2026-01-13T10:05:00Z"
  }
]
```

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--output` | `-o` | Output format: table (default), json |

### State Reconciliation

Before listing, reconcile stored state with actual Docker state:

```go
func (m *AgentManager) List() ([]*Agent, error) {
    agents, err := m.state.ListAgents()
    if err != nil {
        return nil, err
    }

    // Reconcile with Docker state
    for _, ag := range agents {
        running := m.docker.ContainerRunning(ag.ContainerID)
        if !running && ag.Status == "working" {
            ag.Status = "stopped"
            m.state.SetAgent(ag)
        }
    }

    return agents, nil
}
```

## Error Handling

| Scenario | Behavior |
|----------|----------|
| State file missing | Show "No agents found" |
| Docker not running | Show warning, still list from state |

## Out of Scope

- Filtering by status
- Sorting options

## Notes

Use `github.com/fatih/color` for terminal colors. Detect if stdout is a TTY and disable colors if not.
