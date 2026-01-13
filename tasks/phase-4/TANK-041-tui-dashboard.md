---
id: TANK-041
title: Dashboard Framework
status: todo
priority: low
estimate: L
depends_on: [TANK-010]
workstream: C
phase: 4
---

# Dashboard Framework

## Summary

Set up the BubbleTea TUI framework with basic model, layout, and navigation. This provides the foundation for the agent, task, and log panes. Similar to lazydocker or k9s.

## Acceptance Criteria

- [ ] Real-time agent status view
- [ ] Task list with status
- [ ] Log streaming for selected agent
- [ ] Keyboard navigation
- [ ] Actions: start, stop, attach, merge
- [ ] Split panes for multi-agent view

## Technical Details

### Libraries

Use `github.com/charmbracelet/bubbletea` and `github.com/charmbracelet/lipgloss` for the TUI framework.

### Layout

```
┌─────────────────────────────────────────────────────────────────┐
│ Tanuki Dashboard                              [q]uit [?]help    │
├─────────────────────────────────────────────────────────────────┤
│ Agents                          │ Tasks                         │
│ ─────────────────────────────── │ ───────────────────────────── │
│ > backend-agent  [working]      │   TASK-001 ✓ User Auth        │
│   frontend-agent [idle]         │ > TASK-002 ◐ API Refactor     │
│   qa-agent       [idle]         │   TASK-003 ○ Dashboard UI     │
│                                 │   TASK-004 ○ Integration Tests│
│                                 │                               │
├─────────────────────────────────┴───────────────────────────────┤
│ Logs: backend-agent                                    [f]ollow │
│ ─────────────────────────────────────────────────────────────── │
│ [10:15:32] Reading file src/api/routes.ts                       │
│ [10:15:33] Analyzing current API structure...                   │
│ [10:15:35] Found 12 endpoints to refactor                       │
│ [10:15:40] Starting with /api/users endpoint                    │
│ [10:15:42] Editing file src/api/routes.ts                       │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Model

```go
type DashboardModel struct {
    agents       []*agent.Agent
    tasks        []*task.Task
    logs         []string
    selectedPane int // 0: agents, 1: tasks, 2: logs
    agentIndex   int
    taskIndex    int
    logFollow    bool
    width        int
    height       int
}

type pane int

const (
    paneAgents pane = iota
    paneTasks
    paneLogs
)
```

### Key Bindings

| Key | Action |
|-----|--------|
| `q` | Quit |
| `?` | Help |
| `Tab` | Switch pane |
| `j/k` or `↓/↑` | Navigate list |
| `Enter` | Select/expand |
| `s` | Stop selected agent |
| `r` | Run task on selected agent |
| `a` | Attach to selected agent |
| `m` | Merge selected agent's work |
| `d` | Show diff for selected agent |
| `f` | Toggle log follow |
| `l` | Focus logs pane |

### Update Loop

```go
func (m DashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "q", "ctrl+c":
            return m, tea.Quit
        case "tab":
            m.selectedPane = (m.selectedPane + 1) % 3
        case "j", "down":
            m.navigateDown()
        case "k", "up":
            m.navigateUp()
        case "enter":
            return m, m.handleSelect()
        case "s":
            return m, m.stopSelectedAgent()
        case "a":
            return m, m.attachToAgent()
        case "f":
            m.logFollow = !m.logFollow
        }

    case tickMsg:
        return m, tea.Batch(
            m.refreshAgents(),
            m.refreshTasks(),
            m.refreshLogs(),
            tick(),
        )

    case agentsUpdated:
        m.agents = msg.agents
    case tasksUpdated:
        m.tasks = msg.tasks
    case logLine:
        m.logs = append(m.logs, msg.line)
        if len(m.logs) > 1000 {
            m.logs = m.logs[100:] // Trim old logs
        }
    }

    return m, nil
}
```

### Status Indicators

```go
func agentStatusIcon(status string) string {
    switch status {
    case "idle":
        return lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render("●")
    case "working":
        return lipgloss.NewStyle().Foreground(lipgloss.Color("226")).Render("◐")
    case "stopped":
        return lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("○")
    default:
        return "?"
    }
}

func taskStatusIcon(status string) string {
    switch status {
    case "complete":
        return lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render("✓")
    case "in_progress":
        return lipgloss.NewStyle().Foreground(lipgloss.Color("226")).Render("◐")
    case "pending":
        return lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render("○")
    case "failed":
        return lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("✗")
    default:
        return "?"
    }
}
```

### Command

```go
var dashboardCmd = &cobra.Command{
    Use:     "dashboard",
    Aliases: []string{"ui", "tui"},
    Short:   "Open interactive dashboard",
    RunE:    runDashboard,
}

func runDashboard(cmd *cobra.Command, args []string) error {
    cfg, _ := config.Load()

    model := NewDashboardModel(cfg)
    p := tea.NewProgram(model, tea.WithAltScreen())

    _, err := p.Run()
    return err
}
```

### Refresh Interval

```go
func tick() tea.Cmd {
    return tea.Tick(time.Second, func(t time.Time) tea.Msg {
        return tickMsg(t)
    })
}
```

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Terminal too small | Show message, don't crash |
| Agent action fails | Show error in status bar |
| Refresh fails | Show stale data indicator |

## Out of Scope

- Mouse support
- Custom themes
- Detached mode (run in background)

## Notes

This is a nice-to-have feature. The CLI commands should remain the primary interface. The TUI is for monitoring and quick actions.
