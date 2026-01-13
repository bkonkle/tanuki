---
id: TANK-009
title: Command - tanuki spawn
status: done
priority: high
estimate: M
depends_on: [TANK-006]
workstream: D
phase: 1
---

# Command: tanuki spawn

## Summary

Implement the `tanuki spawn` command that creates a new agent with its own worktree and container. Designed for minimal friction - one command gets you a working agent.

## Acceptance Criteria

- [x] Creates git worktree for agent
- [x] Creates and starts Docker container
- [x] Registers agent in state
- [x] **Zero-config start**: works without tanuki.yaml (uses defaults)
- [x] **Auto-pull**: Docker pulls image automatically if not present
- [x] **Fast feedback**: shows progress, completes in <10s
- [x] Supports spawning multiple agents at once (`-n` flag)
- [x] Validates agent name (alphanumeric, hyphens)

## Technical Details

### Command Definition

```go
var spawnCmd = &cobra.Command{
    Use:   "spawn <name>",
    Short: "Create a new agent",
    Long: `Create a new agent with an isolated git worktree and Docker container.

Examples:
  tanuki spawn auth           # Create agent named "auth"
  tanuki spawn -n 3           # Create agent-1, agent-2, agent-3
  tanuki spawn auth -b main   # Use existing branch`,
    Args: cobra.MaximumNArgs(1),
    RunE: runSpawn,
}

func init() {
    spawnCmd.Flags().IntP("count", "n", 1, "Number of agents to spawn")
    spawnCmd.Flags().StringP("branch", "b", "", "Base branch (default: current branch)")
    rootCmd.AddCommand(spawnCmd)
}
```

### Implementation

```go
func runSpawn(cmd *cobra.Command, args []string) error {
    count, _ := cmd.Flags().GetInt("count")
    branch, _ := cmd.Flags().GetString("branch")

    // Load config and create managers
    cfg, err := config.Load()
    if err != nil {
        return fmt.Errorf("failed to load config: %w", err)
    }

    agentMgr := agent.NewManager(cfg)

    // Determine names
    var names []string
    if len(args) > 0 {
        if count > 1 {
            return fmt.Errorf("cannot use --count with explicit name")
        }
        names = []string{args[0]}
    } else {
        for i := 1; i <= count; i++ {
            names = append(names, fmt.Sprintf("agent-%d", i))
        }
    }

    // Spawn each agent
    for _, name := range names {
        fmt.Printf("Spawning agent %s...\n", name)

        opts := agent.SpawnOptions{
            Branch: branch,
        }

        spinner := newSpinner()
        spinner.Start()

        ag, err := agentMgr.Spawn(name, opts)
        spinner.Stop()

        if err != nil {
            fmt.Printf("  Failed: %v\n", err)
            continue
        }

        fmt.Printf("  Created agent %s\n", ag.Name)
        fmt.Printf("    Branch:    %s\n", ag.Branch)
        fmt.Printf("    Container: %s\n", ag.ContainerName)
        fmt.Printf("    Worktree:  %s\n", ag.WorktreePath)
    }

    if len(names) == 1 {
        fmt.Printf("\nRun a task:\n")
        fmt.Printf("  tanuki run %s \"your task here\"\n", names[0])
    } else {
        fmt.Printf("\nRun tasks:\n")
        fmt.Printf("  tanuki run <agent-name> \"your task here\"\n")
    }

    return nil
}
```

### Name Generation (when not provided)

```go
var adjectives = []string{"quick", "lazy", "clever", "brave", "calm"}
var nouns = []string{"fox", "wolf", "bear", "hawk", "deer"}

func generateAgentName() string {
    adj := adjectives[rand.Intn(len(adjectives))]
    noun := nouns[rand.Intn(len(nouns))]
    return fmt.Sprintf("%s-%s", adj, noun)
}
```

### Output

Optimized for quick scanning - show what matters, hide implementation details:

```text
$ tanuki spawn auth
Spawning auth... done (3.2s)

  Branch:  tanuki/auth
  Status:  ready

Next: tanuki run auth "your task here"
```

```text
$ tanuki spawn -n 3
Spawning agent-1... done (3.1s)
Spawning agent-2... done (0.8s)  # Faster - image cached
Spawning agent-3... done (0.7s)

3 agents ready. Run tasks with:
  tanuki run <name> "your task"
  tanuki list
```

```text
$ tanuki spawn auth
Error: agent "auth" already exists

Use a different name, or remove the existing agent:
  tanuki remove auth
```

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--count` | `-n` | Number of agents to spawn |
| `--branch` | `-b` | Use existing branch |

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Agent already exists | Error with suggestion to use different name |
| Invalid name | Error with naming rules |
| Docker image missing | Offer to pull image |
| Branch already exists | Error unless `--branch` flag used |

## Out of Scope

- Role assignment (Phase 2)
- Immediate task assignment

## Notes

Keep the output clean and informative. Show the user exactly what was created and what to do next.
