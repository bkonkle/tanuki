---
id: TANK-003
title: State Management
status: done
priority: high
estimate: M
depends_on: [TANK-001]
workstream: A
phase: 1
---

# State Management

## Summary

Implement persistent state tracking for agents. Tanuki needs to know which agents exist, their status, container IDs, and associated metadata.

## Acceptance Criteria

- [x] State stored in `.tanuki/state/agents.json`
- [x] Atomic writes to prevent corruption
- [x] State includes: agent name, container ID, branch, status, timestamps
- [x] State survives CLI restarts
- [x] Stale state detection (container exists but state says running)
- [x] State cleanup on agent removal

## Technical Details

### State File Location

```
.tanuki/
├── state/
│   └── agents.json    # Agent state (gitignored)
├── worktrees/         # Git worktrees (gitignored)
└── config/
    └── tanuki.yaml    # Project config (committed)
```

### State Schema

```json
{
  "version": "1",
  "project": "/Users/brandon/code/my-project",
  "agents": {
    "auth-feature": {
      "name": "auth-feature",
      "container_id": "abc123def456...",
      "container_name": "tanuki-auth-feature",
      "branch": "tanuki/auth-feature",
      "worktree_path": ".tanuki/worktrees/auth-feature",
      "status": "idle",
      "created_at": "2026-01-13T10:00:00Z",
      "updated_at": "2026-01-13T10:30:00Z",
      "last_task": {
        "prompt": "Implement OAuth2 login flow",
        "started_at": "2026-01-13T10:15:00Z",
        "completed_at": "2026-01-13T10:30:00Z",
        "session_id": "session-xyz..."
      }
    }
  }
}
```

### Status Values

| Status | Description |
|--------|-------------|
| `creating` | Worktree/container being set up |
| `idle` | Ready to accept tasks |
| `working` | Currently executing a task |
| `stopped` | Container stopped but exists |
| `error` | Something went wrong |

### Go Implementation

```go
type State struct {
    Version string            `json:"version"`
    Project string            `json:"project"`
    Agents  map[string]*Agent `json:"agents"`
}

type Agent struct {
    Name          string     `json:"name"`
    ContainerID   string     `json:"container_id"`
    ContainerName string     `json:"container_name"`
    Branch        string     `json:"branch"`
    WorktreePath  string     `json:"worktree_path"`
    Status        string     `json:"status"`
    CreatedAt     time.Time  `json:"created_at"`
    UpdatedAt     time.Time  `json:"updated_at"`
    LastTask      *TaskInfo  `json:"last_task,omitempty"`
}

type StateManager interface {
    Load() (*State, error)
    Save(state *State) error
    GetAgent(name string) (*Agent, error)
    SetAgent(agent *Agent) error
    RemoveAgent(name string) error
    ListAgents() ([]*Agent, error)
}
```

### Atomic Writes

```go
func (m *FileStateManager) Save(state *State) error {
    // Write to temp file first
    tmpFile := m.path + ".tmp"
    // ... write to tmpFile ...

    // Atomic rename
    return os.Rename(tmpFile, m.path)
}
```

### Stale State Detection

On `tanuki list`, verify each agent's container actually exists:

```go
func (m *StateManager) Reconcile() error {
    for _, agent := range state.Agents {
        exists, running := docker.ContainerStatus(agent.ContainerID)
        if !exists {
            agent.Status = "removed"
        } else if !running && agent.Status == "working" {
            agent.Status = "stopped"
        }
    }
}
```

## Out of Scope

- Session persistence for Claude Code conversations
- Task queue state (Phase 3)

## Notes

Keep state minimal - only what's needed to manage agent lifecycle. Don't store conversation history or logs in state file.
