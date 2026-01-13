---
id: TANK-014
title: Command - tanuki attach
status: done
priority: medium
estimate: S
depends_on: [TANK-005]
workstream: D
phase: 1
---

# Command: tanuki attach

## Summary

Implement the `tanuki attach` command that opens an interactive shell inside an agent's container.

## Acceptance Criteria

- [x] Opens interactive zsh/bash shell in container
- [x] Works with TTY (proper terminal handling)
- [x] Starts in /workspace directory
- [x] Handles container not running gracefully
- [x] Supports running a specific command

## Technical Details

### Command Definition

```go
var attachCmd = &cobra.Command{
    Use:   "attach <agent> [command]",
    Short: "Open a shell in an agent's container",
    Long: `Open an interactive shell in an agent's container.

If a command is provided, it runs that command instead of opening a shell.

Examples:
  tanuki attach auth-feature
  tanuki attach auth-feature "ls -la"
  tanuki attach auth-feature "git status"`,
    Args: cobra.MinimumNArgs(1),
    RunE: runAttach,
}
```

### Implementation

```go
func runAttach(cmd *cobra.Command, args []string) error {
    agentName := args[0]

    cfg, err := config.Load()
    if err != nil {
        return err
    }

    agentMgr := agent.NewManager(cfg)

    ag, err := agentMgr.Get(agentName)
    if err != nil {
        return fmt.Errorf("agent %q not found", agentName)
    }

    // Check container is running
    dockerMgr := docker.NewManager(cfg)
    if !dockerMgr.ContainerRunning(ag.ContainerID) {
        return fmt.Errorf("agent %q is not running\nUse 'tanuki start %s' first", agentName, agentName)
    }

    // Build exec command
    var execArgs []string

    if len(args) > 1 {
        // Run specific command
        execArgs = []string{
            "exec", "-it",
            "-w", "/workspace",
            ag.ContainerID,
            "sh", "-c", strings.Join(args[1:], " "),
        }
    } else {
        // Interactive shell
        execArgs = []string{
            "exec", "-it",
            "-w", "/workspace",
            ag.ContainerID,
            "zsh", // or "bash" as fallback
        }
    }

    dockerCmd := exec.Command("docker", execArgs...)
    dockerCmd.Stdin = os.Stdin
    dockerCmd.Stdout = os.Stdout
    dockerCmd.Stderr = os.Stderr

    return dockerCmd.Run()
}
```

### Output

```
$ tanuki attach auth-feature
dev@tanuki-auth-feature:/workspace$ ls
src/  tests/  package.json  ...
dev@tanuki-auth-feature:/workspace$ exit
```

```
$ tanuki attach auth-feature "git status"
On branch tanuki/auth-feature
Changes not staged for commit:
  ...
```

### TTY Handling

The `-it` flags in `docker exec` handle:
- `-i`: Keep STDIN open
- `-t`: Allocate a pseudo-TTY

This enables proper terminal behavior (colors, line editing, etc.).

### Shell Selection

```go
func getShell(containerID string) string {
    // Try zsh first
    cmd := exec.Command("docker", "exec", containerID, "which", "zsh")
    if err := cmd.Run(); err == nil {
        return "zsh"
    }

    // Fall back to bash
    cmd = exec.Command("docker", "exec", containerID, "which", "bash")
    if err := cmd.Run(); err == nil {
        return "bash"
    }

    // Last resort
    return "sh"
}
```

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Agent not found | Error with list suggestion |
| Container not running | Error with start suggestion |
| No TTY available | Warn, continue without TTY |

## Out of Scope

- Multiplexing (tmux inside container)
- Port forwarding

## Notes

This is the "escape hatch" for when you need to debug or manually intervene in an agent's work.
