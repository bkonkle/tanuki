---
id: TANK-046
title: Dashboard Agent Pane
status: todo
priority: low
estimate: M
depends_on: [TANK-041]
workstream: C
phase: 4
---

# Dashboard Agent Pane

## Summary

Implement the agent list pane for the TUI dashboard with real-time status updates, selection, and quick actions. This pane shows all agents with their current status and allows users to perform common operations.

## Acceptance Criteria

- [ ] List all agents with status icons (idle, working, stopped)
- [ ] Show current task assignment for each agent
- [ ] Selection highlighting with cursor navigation
- [ ] Actions: stop (s), start (r), attach (a), diff (d)
- [ ] Refresh on configurable interval
- [ ] Show agent creation time and uptime
- [ ] Filter/search agents (optional)

## Technical Details

### Agent Pane Model

```go
type AgentPane struct {
    agents       []*agent.Agent
    cursor       int
    focused      bool
    width        int
    height       int
    agentManager agent.Manager
}

func NewAgentPane(am agent.Manager) *AgentPane {
    return &AgentPane{
        agents:       make([]*agent.Agent, 0),
        agentManager: am,
    }
}
```

### Rendering

```go
func (p *AgentPane) View() string {
    var sb strings.Builder

    // Header
    header := lipgloss.NewStyle().
        Bold(true).
        Foreground(lipgloss.Color("12")).
        Render(fmt.Sprintf("Agents [%d]", len(p.agents)))
    sb.WriteString(header)
    sb.WriteString("\n")
    sb.WriteString(strings.Repeat("─", p.width))
    sb.WriteString("\n")

    // Agent list
    for i, a := range p.agents {
        line := p.renderAgent(i, a)
        sb.WriteString(line)
        sb.WriteString("\n")
    }

    // Fill remaining space
    remaining := p.height - len(p.agents) - 3
    for i := 0; i < remaining; i++ {
        sb.WriteString("\n")
    }

    return sb.String()
}

func (p *AgentPane) renderAgent(index int, a *agent.Agent) string {
    // Selection indicator
    prefix := "  "
    if index == p.cursor && p.focused {
        prefix = "> "
    }

    // Status icon
    icon := statusIcon(a.Status)

    // Name with padding
    name := lipgloss.NewStyle().
        Width(15).
        Render(a.Name)

    // Status text
    statusStyle := lipgloss.NewStyle().
        Width(10).
        Foreground(statusColor(a.Status))
    status := statusStyle.Render(fmt.Sprintf("[%s]", a.Status))

    // Current task (if any)
    task := ""
    if a.CurrentTask != "" {
        task = lipgloss.NewStyle().
            Foreground(lipgloss.Color("245")).
            Render(fmt.Sprintf("→ %s", truncate(a.CurrentTask, 20)))
    }

    return fmt.Sprintf("%s%s %s %s %s", prefix, icon, name, status, task)
}

func statusIcon(status string) string {
    switch status {
    case "idle":
        return lipgloss.NewStyle().
            Foreground(lipgloss.Color("42")).
            Render("●")
    case "working":
        return lipgloss.NewStyle().
            Foreground(lipgloss.Color("226")).
            Render("◐")
    case "stopped":
        return lipgloss.NewStyle().
            Foreground(lipgloss.Color("245")).
            Render("○")
    default:
        return "?"
    }
}

func statusColor(status string) lipgloss.Color {
    switch status {
    case "idle":
        return lipgloss.Color("42")  // Green
    case "working":
        return lipgloss.Color("226") // Yellow
    case "stopped":
        return lipgloss.Color("245") // Gray
    default:
        return lipgloss.Color("255") // White
    }
}
```

### Display Format

```
Agents [3]
─────────────────────────────────────
> ● backend-agent   [working]  → TASK-002
  ○ frontend-agent  [idle]
  ○ qa-agent        [stopped]
```

### Navigation

```go
func (p *AgentPane) Update(msg tea.Msg) (*AgentPane, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        if !p.focused {
            return p, nil
        }

        switch msg.String() {
        case "j", "down":
            if p.cursor < len(p.agents)-1 {
                p.cursor++
            }
        case "k", "up":
            if p.cursor > 0 {
                p.cursor--
            }
        case "s":
            return p, p.stopAgent()
        case "r":
            return p, p.startAgent()
        case "a":
            return p, p.attachToAgent()
        case "d":
            return p, p.showDiff()
        }

    case agentsRefreshed:
        p.agents = msg.agents
        // Keep cursor in bounds
        if p.cursor >= len(p.agents) {
            p.cursor = max(0, len(p.agents)-1)
        }
    }

    return p, nil
}
```

### Actions

```go
func (p *AgentPane) stopAgent() tea.Cmd {
    if p.cursor >= len(p.agents) {
        return nil
    }

    agent := p.agents[p.cursor]
    return func() tea.Msg {
        err := p.agentManager.Stop(agent.Name)
        return actionResult{action: "stop", agent: agent.Name, err: err}
    }
}

func (p *AgentPane) startAgent() tea.Cmd {
    if p.cursor >= len(p.agents) {
        return nil
    }

    agent := p.agents[p.cursor]
    return func() tea.Msg {
        err := p.agentManager.Start(agent.Name)
        return actionResult{action: "start", agent: agent.Name, err: err}
    }
}

func (p *AgentPane) attachToAgent() tea.Cmd {
    if p.cursor >= len(p.agents) {
        return nil
    }

    agent := p.agents[p.cursor]
    // Suspend TUI and attach to container
    return tea.ExecProcess(
        exec.Command("docker", "exec", "-it",
            fmt.Sprintf("tanuki-agent-%s", agent.Name), "/bin/sh"),
        func(err error) tea.Msg {
            return attachComplete{agent: agent.Name, err: err}
        },
    )
}

func (p *AgentPane) showDiff() tea.Cmd {
    if p.cursor >= len(p.agents) {
        return nil
    }

    agent := p.agents[p.cursor]
    return func() tea.Msg {
        diff, err := p.agentManager.GetDiff(agent.Name)
        return diffResult{agent: agent.Name, diff: diff, err: err}
    }
}
```

### Refresh

```go
func (p *AgentPane) Refresh() tea.Cmd {
    return func() tea.Msg {
        agents, err := p.agentManager.List()
        if err != nil {
            return refreshError{pane: "agents", err: err}
        }
        return agentsRefreshed{agents: agents}
    }
}
```

### Selected Agent

```go
func (p *AgentPane) SelectedAgent() *agent.Agent {
    if p.cursor >= len(p.agents) {
        return nil
    }
    return p.agents[p.cursor]
}
```

## Key Bindings (when focused)

| Key | Action |
|-----|--------|
| `j` / `↓` | Move cursor down |
| `k` / `↑` | Move cursor up |
| `s` | Stop selected agent |
| `r` | Start selected agent |
| `a` | Attach to selected agent |
| `d` | Show diff for selected agent |
| `Enter` | Select agent for log viewing |

## Error Handling

| Scenario | Behavior |
|----------|----------|
| No agents | Show "No agents" message |
| Action fails | Show error in status bar |
| Refresh fails | Keep old data, show stale indicator |

## Testing

- Test rendering with various agent states
- Test cursor navigation bounds
- Test action commands return correct messages
- Test refresh updates agent list
- Test selected agent accessor

## Files to Create/Modify

- `internal/tui/pane_agents.go` - AgentPane implementation
- `internal/tui/style.go` - Shared styling (status icons, colors)

## Notes

The attach action suspends the TUI to give the user full terminal access to the container. When they exit, the TUI resumes. This is handled by BubbleTea's ExecProcess.
