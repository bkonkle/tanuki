---
id: TANK-006
title: Agent Manager
status: done
priority: high
estimate: M
depends_on: [TANK-003, TANK-004, TANK-005]
workstream: C
phase: 1
---

# Agent Manager

## Summary

Implement the high-level agent lifecycle management that coordinates Git worktrees, Docker containers, and state. This is the core orchestration layer.

## Acceptance Criteria

- [x] Spawn creates worktree + container + state entry atomically
- [x] Remove cleans up container + worktree + state atomically
- [x] Stop/start controls container lifecycle
- [x] Status aggregates info from docker + git + state
- [x] Handles partial failures gracefully (rollback on error)
- [x] Validates agent names (alphanumeric, hyphens, no spaces)

## Technical Details

### Agent Manager Interface

```go
type AgentManager interface {
    // Lifecycle
    Spawn(name string, opts SpawnOptions) (*Agent, error)
    Remove(name string, opts RemoveOptions) error
    Stop(name string) error
    Start(name string) error

    // Status
    Get(name string) (*Agent, error)
    List() ([]*Agent, error)
    Status(name string) (*AgentStatus, error)

    // Task execution
    Run(name string, prompt string, opts RunOptions) error
    IsWorking(name string) (bool, error)
}

type SpawnOptions struct {
    Branch string // Optional: use existing branch
}

type RemoveOptions struct {
    Force        bool // Remove even if working
    KeepBranch   bool // Don't delete git branch
}

type RunOptions struct {
    Follow       bool     // Stream output
    AllowedTools []string // Override default tools
    MaxTurns     int      // Override default max turns
    SystemPrompt string   // Additional system prompt
}

type AgentStatus struct {
    Name         string
    Status       string
    Branch       string
    Container    ContainerStatus
    Git          GitStatus
    LastTask     *TaskInfo
    Uptime       time.Duration
}

type ContainerStatus struct {
    ID      string
    Running bool
    Memory  string
    CPU     string
}

type GitStatus struct {
    Branch        string
    CommitsBehind int
    CommitsAhead  int
    HasChanges    bool
}
```

### Spawn Flow

```go
func (m *AgentManager) Spawn(name string, opts SpawnOptions) (*Agent, error) {
    // 1. Validate name
    if err := validateAgentName(name); err != nil {
        return nil, err
    }

    // 2. Check if agent already exists
    if _, err := m.state.GetAgent(name); err == nil {
        return nil, fmt.Errorf("agent %q already exists", name)
    }

    // 3. Create worktree
    worktreePath, err := m.git.CreateWorktree(name)
    if err != nil {
        return nil, fmt.Errorf("failed to create worktree: %w", err)
    }

    // 4. Create container (rollback worktree on failure)
    containerID, err := m.docker.CreateAgentContainer(name, worktreePath)
    if err != nil {
        m.git.RemoveWorktree(name, true) // Rollback
        return nil, fmt.Errorf("failed to create container: %w", err)
    }

    // 5. Start container (rollback both on failure)
    if err := m.docker.StartContainer(containerID); err != nil {
        m.docker.RemoveContainer(containerID) // Rollback
        m.git.RemoveWorktree(name, true)      // Rollback
        return nil, fmt.Errorf("failed to start container: %w", err)
    }

    // 6. Create state entry
    agent := &Agent{
        Name:          name,
        ContainerID:   containerID,
        ContainerName: fmt.Sprintf("tanuki-%s", name),
        Branch:        fmt.Sprintf("%s%s", m.config.Git.BranchPrefix, name),
        WorktreePath:  worktreePath,
        Status:        "idle",
        CreatedAt:     time.Now(),
        UpdatedAt:     time.Now(),
    }

    if err := m.state.SetAgent(agent); err != nil {
        // Rollback everything
        m.docker.StopContainer(containerID)
        m.docker.RemoveContainer(containerID)
        m.git.RemoveWorktree(name, true)
        return nil, fmt.Errorf("failed to save state: %w", err)
    }

    return agent, nil
}
```

### Remove Flow

```go
func (m *AgentManager) Remove(name string, opts RemoveOptions) error {
    agent, err := m.state.GetAgent(name)
    if err != nil {
        return fmt.Errorf("agent %q not found", name)
    }

    // Check if working
    if agent.Status == "working" && !opts.Force {
        return fmt.Errorf("agent %q is currently working, use --force to remove", name)
    }

    // Stop and remove container
    m.docker.StopContainer(agent.ContainerID)
    m.docker.RemoveContainer(agent.ContainerID)

    // Remove worktree and optionally branch
    m.git.RemoveWorktree(name, !opts.KeepBranch)

    // Remove from state
    return m.state.RemoveAgent(name)
}
```

### Name Validation

```go
var validNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]*[a-z0-9]$`)

func validateAgentName(name string) error {
    if len(name) < 2 {
        return fmt.Errorf("agent name must be at least 2 characters")
    }
    if len(name) > 63 {
        return fmt.Errorf("agent name must be at most 63 characters")
    }
    if !validNamePattern.MatchString(name) {
        return fmt.Errorf("agent name must start with a letter, contain only lowercase letters, numbers, and hyphens")
    }
    return nil
}
```

### Status Aggregation

```go
func (m *AgentManager) Status(name string) (*AgentStatus, error) {
    agent, err := m.state.GetAgent(name)
    if err != nil {
        return nil, err
    }

    // Get container status
    containerInfo, _ := m.docker.InspectContainer(agent.ContainerID)

    // Get git status
    diff, _ := m.git.GetDiff(name, m.git.GetMainBranch())
    status, _ := m.git.GetStatus(name)

    return &AgentStatus{
        Name:   agent.Name,
        Status: agent.Status,
        Branch: agent.Branch,
        Container: ContainerStatus{
            ID:      agent.ContainerID[:12],
            Running: containerInfo != nil && containerInfo.Running,
        },
        Git: GitStatus{
            Branch:     agent.Branch,
            HasChanges: len(diff) > 0 || len(status) > 0,
        },
        LastTask: agent.LastTask,
        Uptime:   time.Since(agent.CreatedAt),
    }, nil
}
```

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Spawn fails mid-way | Rollback all created resources |
| Container dies unexpectedly | Update state on next status check |
| Git operations fail | Clear error with git command that failed |
| State file locked | Retry with backoff, then error |

## Out of Scope

- Task execution (`Run` method - separate task)
- Role assignment (Phase 2)
- Bulk operations (separate task)

## Notes

The agent manager is the coordination layer - it should be thin and delegate to git/docker/state managers. Keep business logic minimal here.
