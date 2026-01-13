---
id: TANK-015
title: Commands - tanuki stop/start/remove
status: done
priority: medium
estimate: M
depends_on: [TANK-006]
workstream: D
phase: 1
---

# Commands: tanuki stop/start/remove

## Summary

Implement the lifecycle management commands for stopping, starting, and removing agents.

## Acceptance Criteria

- [x] `stop` stops container but preserves worktree
- [x] `start` restarts a stopped container
- [x] `remove` cleans up container, worktree, and state
- [x] All commands support `--all` flag
- [x] `remove` requires confirmation unless `--force`
- [x] `remove --keep-branch` preserves git branch

## Technical Details

### Command: stop

```go
var stopCmd = &cobra.Command{
    Use:   "stop <agent>",
    Short: "Stop an agent's container",
    Long: `Stop an agent's container while preserving its worktree and branch.

Examples:
  tanuki stop auth-feature
  tanuki stop --all`,
    Args: cobra.MaximumNArgs(1),
    RunE: runStop,
}

func init() {
    stopCmd.Flags().Bool("all", false, "Stop all agents")
    rootCmd.AddCommand(stopCmd)
}

func runStop(cmd *cobra.Command, args []string) error {
    all, _ := cmd.Flags().GetBool("all")

    cfg, _ := config.Load()
    agentMgr := agent.NewManager(cfg)

    if all {
        agents, _ := agentMgr.List()
        for _, ag := range agents {
            if ag.Status == "stopped" {
                continue
            }
            fmt.Printf("Stopping %s...\n", ag.Name)
            agentMgr.Stop(ag.Name)
        }
        return nil
    }

    if len(args) == 0 {
        return fmt.Errorf("agent name required (or use --all)")
    }

    return agentMgr.Stop(args[0])
}
```

### Command: start

```go
var startCmd = &cobra.Command{
    Use:   "start <agent>",
    Short: "Start a stopped agent",
    Long: `Start a previously stopped agent's container.

Examples:
  tanuki start auth-feature
  tanuki start --all`,
    Args: cobra.MaximumNArgs(1),
    RunE: runStart,
}

func init() {
    startCmd.Flags().Bool("all", false, "Start all stopped agents")
    rootCmd.AddCommand(startCmd)
}

func runStart(cmd *cobra.Command, args []string) error {
    all, _ := cmd.Flags().GetBool("all")

    cfg, _ := config.Load()
    agentMgr := agent.NewManager(cfg)

    if all {
        agents, _ := agentMgr.List()
        for _, ag := range agents {
            if ag.Status != "stopped" {
                continue
            }
            fmt.Printf("Starting %s...\n", ag.Name)
            agentMgr.Start(ag.Name)
        }
        return nil
    }

    if len(args) == 0 {
        return fmt.Errorf("agent name required (or use --all)")
    }

    return agentMgr.Start(args[0])
}
```

### Command: remove

```go
var removeCmd = &cobra.Command{
    Use:     "remove <agent>",
    Aliases: []string{"rm"},
    Short:   "Remove an agent completely",
    Long: `Remove an agent's container, worktree, and branch.

This action is destructive. Use --keep-branch to preserve the git branch.

Examples:
  tanuki remove auth-feature
  tanuki remove auth-feature --keep-branch
  tanuki remove --all --force`,
    Args: cobra.MaximumNArgs(1),
    RunE: runRemove,
}

func init() {
    removeCmd.Flags().Bool("force", false, "Skip confirmation")
    removeCmd.Flags().Bool("keep-branch", false, "Keep the git branch")
    removeCmd.Flags().Bool("all", false, "Remove all agents")
    rootCmd.AddCommand(removeCmd)
}

func runRemove(cmd *cobra.Command, args []string) error {
    force, _ := cmd.Flags().GetBool("force")
    keepBranch, _ := cmd.Flags().GetBool("keep-branch")
    all, _ := cmd.Flags().GetBool("all")

    cfg, _ := config.Load()
    agentMgr := agent.NewManager(cfg)

    if all {
        if !force {
            fmt.Print("Remove ALL agents? This cannot be undone. [y/N]: ")
            var response string
            fmt.Scanln(&response)
            if strings.ToLower(response) != "y" {
                return nil
            }
        }

        agents, _ := agentMgr.List()
        for _, ag := range agents {
            fmt.Printf("Removing %s...\n", ag.Name)
            agentMgr.Remove(ag.Name, agent.RemoveOptions{
                Force:      true,
                KeepBranch: keepBranch,
            })
        }
        return nil
    }

    if len(args) == 0 {
        return fmt.Errorf("agent name required (or use --all)")
    }

    agentName := args[0]

    // Confirmation
    if !force {
        ag, _ := agentMgr.Get(agentName)
        if ag.Status == "working" {
            fmt.Printf("Agent %q is currently working!\n", agentName)
        }
        fmt.Printf("Remove agent %q? [y/N]: ", agentName)
        var response string
        fmt.Scanln(&response)
        if strings.ToLower(response) != "y" {
            return nil
        }
    }

    opts := agent.RemoveOptions{
        Force:      force,
        KeepBranch: keepBranch,
    }

    if err := agentMgr.Remove(agentName, opts); err != nil {
        return err
    }

    fmt.Printf("Removed agent %s\n", agentName)
    if keepBranch {
        fmt.Printf("  Branch preserved: tanuki/%s\n", agentName)
    }

    return nil
}
```

### Output Examples

```
$ tanuki stop auth-feature
Stopped auth-feature
```

```
$ tanuki start auth-feature
Started auth-feature
```

```
$ tanuki remove auth-feature
Remove agent "auth-feature"? [y/N]: y
Removed agent auth-feature
```

```
$ tanuki remove auth-feature --keep-branch
Remove agent "auth-feature"? [y/N]: y
Removed agent auth-feature
  Branch preserved: tanuki/auth-feature
```

```
$ tanuki remove --all --force
Removing auth-feature...
Removing api-refactor...
Removing test-coverage...
All agents removed.
```

### Flags Summary

| Command | Flag | Description |
|---------|------|-------------|
| stop | `--all` | Stop all agents |
| start | `--all` | Start all stopped agents |
| remove | `--all` | Remove all agents |
| remove | `--force` | Skip confirmation |
| remove | `--keep-branch` | Preserve git branch |

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Agent not found | Error with list suggestion |
| Already stopped (stop) | No-op, success |
| Already running (start) | No-op, success |
| Working agent (remove) | Require --force or confirmation |

## Out of Scope

- Pausing containers (Docker pause)
- Scheduled auto-cleanup

## Notes

The `remove` command is intentionally destructive. Always confirm unless `--force` is used.
