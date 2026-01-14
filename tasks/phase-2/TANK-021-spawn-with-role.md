---
id: TANK-021
title: Spawn with Role Assignment
status: done
priority: high
estimate: M
depends_on: [TANK-009, TANK-023]
workstream: C
phase: 2
---

# Spawn with Role Assignment

## Summary

Extend the `tanuki spawn` command to support role assignment. When an agent is spawned with a role, it inherits the role's configuration.

## Acceptance Criteria

- [ ] `--role` flag for spawn command
- [ ] Role config applied to agent (allowed tools, etc.)
- [ ] CLAUDE.md generated from role system prompt
- [ ] Context files copied to worktree
- [ ] Role stored in agent state for reference

## Technical Details

### Extended Spawn Command

```go
func init() {
    spawnCmd.Flags().StringP("role", "r", "", "Role to assign to agent")
    // ... existing flags
}

func runSpawn(cmd *cobra.Command, args []string) error {
    roleName, _ := cmd.Flags().GetString("role")

    // ... existing validation

    opts := agent.SpawnOptions{
        Branch: branch,
        Role:   roleName,
    }

    // Spawn agent
    ag, err := agentMgr.Spawn(name, opts)
    // ...
}
```

### Spawn with Role Flow

```go
func (m *AgentManager) Spawn(name string, opts SpawnOptions) (*Agent, error) {
    // ... existing spawn logic (worktree, container)

    // If role specified, apply role configuration
    if opts.Role != "" {
        role, err := m.roleManager.Get(opts.Role)
        if err != nil {
            return nil, fmt.Errorf("role %q not found", opts.Role)
        }

        // Generate CLAUDE.md in worktree
        if err := m.generateClaudeMD(worktreePath, role); err != nil {
            return nil, fmt.Errorf("failed to generate CLAUDE.md: %w", err)
        }

        // Copy context files
        if err := m.copyContextFiles(worktreePath, role.ContextFiles); err != nil {
            return nil, fmt.Errorf("failed to copy context files: %w", err)
        }

        // Store role in agent state
        agent.Role = opts.Role
        agent.AllowedTools = role.AllowedTools
        agent.DisallowedTools = role.DisallowedTools
    }

    // ... save state
}
```

### CLAUDE.md Generation

```go
func (m *AgentManager) generateClaudeMD(worktreePath string, role *Role) error {
    claudeMDPath := filepath.Join(worktreePath, "CLAUDE.md")

    var content strings.Builder

    // Add role system prompt
    content.WriteString("# Agent Instructions\n\n")
    content.WriteString(role.SystemPrompt)
    content.WriteString("\n\n")

    // Add context file references
    if len(role.ContextFiles) > 0 {
        content.WriteString("## Context Files\n\n")
        content.WriteString("Review these files for project context:\n\n")
        for _, file := range role.ContextFiles {
            content.WriteString(fmt.Sprintf("- %s\n", file))
        }
    }

    return os.WriteFile(claudeMDPath, []byte(content.String()), 0644)
}
```

### Updated Agent State

```go
type Agent struct {
    // ... existing fields
    Role            string   `json:"role,omitempty"`
    AllowedTools    []string `json:"allowed_tools,omitempty"`
    DisallowedTools []string `json:"disallowed_tools,omitempty"`
}
```

### Run with Role Config

```go
func (m *AgentManager) Run(name string, prompt string, opts RunOptions) error {
    agent, _ := m.state.GetAgent(name)

    // Use agent's role-based tools if not overridden
    allowedTools := opts.AllowedTools
    if len(allowedTools) == 0 && len(agent.AllowedTools) > 0 {
        allowedTools = agent.AllowedTools
    }

    execOpts := ExecuteOptions{
        AllowedTools:    allowedTools,
        DisallowedTools: agent.DisallowedTools,
        // ...
    }

    // ...
}
```

### Output

```
$ tanuki spawn backend-worker --role backend
Spawning agent backend-worker with role 'backend'...
  Created agent backend-worker
    Branch:    tanuki/backend-worker
    Container: tanuki-backend-worker
    Role:      backend
    Worktree:  .tanuki/worktrees/backend-worker

Generated CLAUDE.md with role instructions.
Copied 3 context files.

Run a task:
  tanuki run backend-worker "your task here"
```

### List with Role

```
$ tanuki list
NAME             STATUS    ROLE       BRANCH                      UPTIME
----             ------    ----       ------                      ------
backend-worker   idle      backend    tanuki/backend-worker       2m
frontend-ui      working   frontend   tanuki/frontend-ui          15m
qa-tests         idle      qa         tanuki/qa-tests             1h
```

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Role not found | Error with available roles list |
| Context file not found | Warning, continue without file |
| CLAUDE.md already exists | Overwrite (role takes precedence) |

## Out of Scope

- Changing role after spawn
- Multiple roles per agent

## Notes

The CLAUDE.md file is the primary way roles influence agent behavior. It's read by Claude Code at the start of each session.
