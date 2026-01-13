---
id: TANK-024
title: Role-Based Tool Filtering
status: todo
priority: high
estimate: M
depends_on: [TANK-023]
workstream: A
phase: 2
---

# Role-Based Tool Filtering

## Summary

Implement tool filtering based on role configuration. Ensures agents respect allowed/disallowed tools when executing tasks, providing security boundaries for different agent types.

## Acceptance Criteria

- [ ] Agent stores role's tool restrictions in state
- [ ] `tanuki run` applies tool restrictions to Claude Code invocation
- [ ] `--allowedTools` CLI flag can override role defaults
- [ ] `--disallowedTools` CLI flag can extend role restrictions
- [ ] Tool restriction errors are clear and actionable
- [ ] Test coverage for various tool filter scenarios
- [ ] Documentation for tool filtering behavior

## Technical Details

### Updated Agent State

```go
// internal/state/types.go
type Agent struct {
    Name            string    `json:"name"`
    ContainerID     string    `json:"container_id"`
    Branch          string    `json:"branch"`
    WorktreePath    string    `json:"worktree_path"`
    Status          string    `json:"status"`
    CreatedAt       time.Time `json:"created_at"`

    // Phase 2: Role support
    Role            string   `json:"role,omitempty"`
    AllowedTools    []string `json:"allowed_tools,omitempty"`
    DisallowedTools []string `json:"disallowed_tools,omitempty"`

    LastTask        *Task    `json:"last_task,omitempty"`
}
```

### Tool Filtering in Run Command

```go
// internal/agent/manager.go

type RunOptions struct {
    Prompt          string
    Follow          bool
    Ralph           bool
    MaxIterations   int
    VerifyCommand   string

    // Tool overrides
    AllowedTools    []string
    DisallowedTools []string
}

func (m *Manager) Run(name string, opts RunOptions) error {
    agent, err := m.state.GetAgent(name)
    if err != nil {
        return fmt.Errorf("get agent: %w", err)
    }

    // Build final tool lists
    allowedTools, disallowedTools := m.resolveTools(agent, opts)

    // Prepare execution options
    execOpts := executor.ExecuteOptions{
        Prompt:          opts.Prompt,
        AllowedTools:    allowedTools,
        DisallowedTools: disallowedTools,
        Follow:          opts.Follow,
        Ralph:           opts.Ralph,
        MaxIterations:   opts.MaxIterations,
        VerifyCommand:   opts.VerifyCommand,
    }

    // Execute in container
    return m.executor.Execute(agent, execOpts)
}

func (m *Manager) resolveTools(agent *Agent, opts RunOptions) (allowed, disallowed []string) {
    // Priority:
    // 1. Explicit --allowedTools flag (complete override)
    // 2. Agent's role-based allowed tools
    // 3. No restrictions (allow all)

    if len(opts.AllowedTools) > 0 {
        // Explicit override from CLI
        allowed = opts.AllowedTools
    } else if len(agent.AllowedTools) > 0 {
        // Use role-based restrictions
        allowed = agent.AllowedTools
    }
    // else: no allowed tools specified = allow all

    // Disallowed tools are additive
    disallowedSet := make(map[string]bool)

    // Add agent's role-based disallowed tools
    for _, tool := range agent.DisallowedTools {
        disallowedSet[tool] = true
    }

    // Add CLI-specified disallowed tools
    for _, tool := range opts.DisallowedTools {
        disallowedSet[tool] = true
    }

    disallowed = make([]string, 0, len(disallowedSet))
    for tool := range disallowedSet {
        disallowed = append(disallowed, tool)
    }

    return allowed, disallowed
}
```

### Extended Run Command CLI

```go
// internal/cli/run.go

var runCmd = &cobra.Command{
    Use:   "run <agent> <prompt>",
    Short: "Send a task to an agent",
    Args:  cobra.ExactArgs(2),
    RunE:  runRun,
}

func init() {
    runCmd.Flags().BoolP("follow", "f", false, "Follow output in real-time")
    runCmd.Flags().Bool("ralph", false, "Run in Ralph mode (loop until done)")
    runCmd.Flags().Int("max-iterations", 30, "Max Ralph iterations")
    runCmd.Flags().String("verify", "", "Verification command for Ralph mode")

    // Phase 2: Tool filtering
    runCmd.Flags().StringSlice("allowed-tools", nil, "Override allowed tools (comma-separated)")
    runCmd.Flags().StringSlice("disallowed-tools", nil, "Additional disallowed tools")
}

func runRun(cmd *cobra.Command, args []string) error {
    agentName := args[0]
    prompt := args[1]

    // Parse flags
    follow, _ := cmd.Flags().GetBool("follow")
    ralph, _ := cmd.Flags().GetBool("ralph")
    maxIter, _ := cmd.Flags().GetInt("max-iterations")
    verify, _ := cmd.Flags().GetString("verify")
    allowedTools, _ := cmd.Flags().GetStringSlice("allowed-tools")
    disallowedTools, _ := cmd.Flags().GetStringSlice("disallowed-tools")

    // Load managers
    cfg, _ := config.Load()
    stateMgr := state.NewManager(cfg)
    agentMgr := agent.NewManager(cfg, stateMgr)

    opts := agent.RunOptions{
        Prompt:          prompt,
        Follow:          follow,
        Ralph:           ralph,
        MaxIterations:   maxIter,
        VerifyCommand:   verify,
        AllowedTools:    allowedTools,
        DisallowedTools: disallowedTools,
    }

    return agentMgr.Run(agentName, opts)
}
```

### Claude Code Integration

```go
// internal/executor/executor.go

type ExecuteOptions struct {
    Prompt          string
    AllowedTools    []string
    DisallowedTools []string
    Follow          bool
    Ralph           bool
    MaxIterations   int
    VerifyCommand   string
}

func (e *Executor) Execute(agent *Agent, opts ExecuteOptions) error {
    // Build claude command
    args := []string{"--headless"}

    // Add allowed tools
    if len(opts.AllowedTools) > 0 {
        args = append(args, "--allowedTools", strings.Join(opts.AllowedTools, ","))
    }

    // Add disallowed tools
    if len(opts.DisallowedTools) > 0 {
        args = append(args, "--disallowedTools", strings.Join(opts.DisallowedTools, ","))
    }

    // Add prompt
    args = append(args, opts.Prompt)

    // Execute in container
    cmd := fmt.Sprintf("claude %s", strings.Join(args, " "))

    if opts.Ralph {
        return e.executeRalph(agent, cmd, opts)
    }

    return e.executeOnce(agent, cmd, opts.Follow)
}
```

### Example Usage

```bash
# Use role-based tool restrictions (QA role: read-only)
$ tanuki spawn qa-agent --role qa
$ tanuki run qa-agent "Run test suite and report coverage"
# Uses: --allowedTools=Read,Bash,Glob,Grep --disallowedTools=Write,Edit

# Override role restrictions for special case
$ tanuki run qa-agent "Fix typo in test" --allowed-tools Read,Write,Edit,Bash
# Overrides role restrictions to allow editing

# Add additional restrictions
$ tanuki run backend-api "Review code" --disallowed-tools Bash
# Backend role allows Bash, but this run explicitly disallows it

# No role, no restrictions
$ tanuki spawn free-agent
$ tanuki run free-agent "Do anything"
# No --allowedTools or --disallowedTools = full access
```

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Invalid tool name in CLI | Error: "unknown tool: %s" + list valid tools |
| Tool in both allowed and disallowed | Error: "tool %s cannot be both allowed and disallowed" |
| Empty allowed tools list | Allow all (not an error) |
| Agent tries to use disallowed tool | Claude Code enforces, shows permission error |

## Tool Reference

Valid Claude Code tools:
- `Read` - Read files
- `Write` - Create new files
- `Edit` - Modify existing files
- `Bash` - Execute shell commands
- `Glob` - Find files by pattern
- `Grep` - Search file contents
- `TodoWrite` - Manage task lists
- `Task` - Spawn sub-agents
- `WebFetch` - Fetch web content
- `WebSearch` - Search the web

## Testing

### Unit Tests

```go
func TestResolveTools(t *testing.T) {
    tests := []struct {
        name             string
        agent            *Agent
        opts             RunOptions
        wantAllowed      []string
        wantDisallowed   []string
    }{
        {
            name: "use role restrictions",
            agent: &Agent{
                AllowedTools: []string{"Read", "Bash"},
                DisallowedTools: []string{"Write"},
            },
            opts: RunOptions{},
            wantAllowed: []string{"Read", "Bash"},
            wantDisallowed: []string{"Write"},
        },
        {
            name: "CLI override allowed tools",
            agent: &Agent{
                AllowedTools: []string{"Read"},
            },
            opts: RunOptions{
                AllowedTools: []string{"Read", "Write", "Edit"},
            },
            wantAllowed: []string{"Read", "Write", "Edit"},
        },
        {
            name: "additive disallowed tools",
            agent: &Agent{
                DisallowedTools: []string{"Write"},
            },
            opts: RunOptions{
                DisallowedTools: []string{"Bash"},
            },
            wantDisallowed: []string{"Write", "Bash"},
        },
        {
            name: "no restrictions",
            agent: &Agent{},
            opts: RunOptions{},
            wantAllowed: nil,
            wantDisallowed: nil,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            m := &Manager{}
            allowed, disallowed := m.resolveTools(tt.agent, tt.opts)

            if !slicesEqual(allowed, tt.wantAllowed) {
                t.Errorf("allowed = %v, want %v", allowed, tt.wantAllowed)
            }
            if !slicesEqual(disallowed, tt.wantDisallowed) {
                t.Errorf("disallowed = %v, want %v", disallowed, tt.wantDisallowed)
            }
        })
    }
}
```

### Integration Tests

```bash
# Test QA role cannot write
tanuki spawn test-qa --role qa
tanuki run test-qa "Fix bug in auth.go"
# Verify: Claude refuses to edit or shows permission error

# Test override works
tanuki spawn test-qa2 --role qa
tanuki run test-qa2 "Fix typo" --allowed-tools Read,Write,Edit,Bash
# Verify: Edit succeeds despite QA role

# Test no role = no restrictions
tanuki spawn test-free
tanuki run test-free "Do anything"
# Verify: All tools available
```

## Documentation

Add to README.md:

```markdown
### Tool Restrictions

Roles can limit which Claude Code tools agents can use:

```bash
# QA role: read-only, cannot modify code
tanuki spawn qa --role qa
tanuki run qa "Review and test the auth module"

# Override for special cases
tanuki run qa "Fix typo in comment" --allowed-tools Read,Write,Edit,Bash
```

Available tools:
- Read, Write, Edit - File operations
- Bash - Shell commands
- Glob, Grep - Search operations
- TodoWrite - Task management
- Task, WebFetch, WebSearch - Advanced features

See [roles.md](docs/roles.md) for role-specific restrictions.
```

## Out of Scope

- Dynamic tool restriction changes (must respawn agent)
- Tool usage monitoring/logging
- Granular tool permissions (e.g., "Bash but only read-only commands")
- Custom tool definitions

## Notes

- Tool filtering is enforced by Claude Code, not Tanuki
- Empty allowed list = allow all (not deny all)
- Disallowed tools are additive (CLI + role)
- Allowed tools are override (CLI replaces role)
