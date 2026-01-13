---
id: TANK-047
title: Dashboard Task Pane
status: todo
priority: low
estimate: M
depends_on: [TANK-041, TANK-031]
workstream: C
phase: 4
---

# Dashboard Task Pane

## Summary

Implement the task list pane for the TUI dashboard with status filtering and task details. This pane shows all project tasks with their current status, assigned agent, and allows basic task management.

## Acceptance Criteria

- [ ] List all project tasks with status icons
- [ ] Show assigned agent for each task
- [ ] Status icons: pending (○), in_progress (◐), complete (✓), failed (✗)
- [ ] Selection highlighting with cursor navigation
- [ ] Filter by status (all, pending, in_progress, complete, failed)
- [ ] Filter by role (all, backend, frontend, qa, etc.)
- [ ] Show task details on selection
- [ ] Actions: assign task, view details

## Technical Details

### Task Pane Model

```go
type TaskPane struct {
    tasks        []*task.Task
    filtered     []*task.Task
    cursor       int
    focused      bool
    width        int
    height       int
    taskManager  task.Manager
    statusFilter string // "all", "pending", "in_progress", "complete", "failed"
    roleFilter   string // "all" or specific role
}

func NewTaskPane(tm task.Manager) *TaskPane {
    return &TaskPane{
        tasks:        make([]*task.Task, 0),
        filtered:     make([]*task.Task, 0),
        taskManager:  tm,
        statusFilter: "all",
        roleFilter:   "all",
    }
}
```

### Rendering

```go
func (p *TaskPane) View() string {
    var sb strings.Builder

    // Header with counts
    header := p.renderHeader()
    sb.WriteString(header)
    sb.WriteString("\n")
    sb.WriteString(strings.Repeat("─", p.width))
    sb.WriteString("\n")

    // Filter indicator (if active)
    if p.statusFilter != "all" || p.roleFilter != "all" {
        filter := lipgloss.NewStyle().
            Foreground(lipgloss.Color("245")).
            Render(p.filterString())
        sb.WriteString(filter)
        sb.WriteString("\n")
    }

    // Task list
    for i, t := range p.filtered {
        line := p.renderTask(i, t)
        sb.WriteString(line)
        sb.WriteString("\n")
    }

    // Fill remaining space
    remaining := p.height - len(p.filtered) - 4
    for i := 0; i < remaining; i++ {
        sb.WriteString("\n")
    }

    return sb.String()
}

func (p *TaskPane) renderHeader() string {
    counts := p.countByStatus()
    return lipgloss.NewStyle().
        Bold(true).
        Foreground(lipgloss.Color("12")).
        Render(fmt.Sprintf("Tasks [%d] (%d pending, %d working, %d done)",
            len(p.tasks),
            counts["pending"],
            counts["in_progress"],
            counts["complete"]))
}

func (p *TaskPane) renderTask(index int, t *task.Task) string {
    // Selection indicator
    prefix := "  "
    if index == p.cursor && p.focused {
        prefix = "> "
    }

    // Status icon
    icon := taskStatusIcon(t.Status)

    // Task ID
    id := lipgloss.NewStyle().
        Width(10).
        Foreground(lipgloss.Color("245")).
        Render(t.ID)

    // Title
    title := lipgloss.NewStyle().
        Width(20).
        Render(truncate(t.Title, 18))

    // Role
    role := lipgloss.NewStyle().
        Width(10).
        Foreground(lipgloss.Color("14")).
        Render(t.Role)

    // Assigned agent (if any)
    assigned := ""
    if t.AssignedTo != "" {
        assigned = lipgloss.NewStyle().
            Foreground(lipgloss.Color("226")).
            Render(fmt.Sprintf("→ %s", t.AssignedTo))
    }

    return fmt.Sprintf("%s%s %s %s %s %s", prefix, icon, id, title, role, assigned)
}

func taskStatusIcon(status string) string {
    switch status {
    case "pending":
        return lipgloss.NewStyle().
            Foreground(lipgloss.Color("245")).
            Render("○")
    case "assigned":
        return lipgloss.NewStyle().
            Foreground(lipgloss.Color("33")).
            Render("◔")
    case "in_progress":
        return lipgloss.NewStyle().
            Foreground(lipgloss.Color("226")).
            Render("◐")
    case "complete":
        return lipgloss.NewStyle().
            Foreground(lipgloss.Color("42")).
            Render("✓")
    case "failed":
        return lipgloss.NewStyle().
            Foreground(lipgloss.Color("196")).
            Render("✗")
    case "blocked":
        return lipgloss.NewStyle().
            Foreground(lipgloss.Color("208")).
            Render("⊘")
    default:
        return "?"
    }
}
```

### Display Format

```
Tasks [4] (1 pending, 2 working, 1 done)
───────────────────────────────────────────
  ✓ TASK-001   User Auth          backend
> ◐ TASK-002   API Refactor       backend    → backend-agent
  ○ TASK-003   Dashboard UI       frontend
  ○ TASK-004   Integration Tests  qa
```

### Filtering

```go
func (p *TaskPane) applyFilters() {
    p.filtered = make([]*task.Task, 0)

    for _, t := range p.tasks {
        // Status filter
        if p.statusFilter != "all" && t.Status != p.statusFilter {
            continue
        }

        // Role filter
        if p.roleFilter != "all" && t.Role != p.roleFilter {
            continue
        }

        p.filtered = append(p.filtered, t)
    }

    // Keep cursor in bounds
    if p.cursor >= len(p.filtered) {
        p.cursor = max(0, len(p.filtered)-1)
    }
}

func (p *TaskPane) cycleStatusFilter() {
    filters := []string{"all", "pending", "in_progress", "complete", "failed"}
    for i, f := range filters {
        if f == p.statusFilter {
            p.statusFilter = filters[(i+1)%len(filters)]
            break
        }
    }
    p.applyFilters()
}

func (p *TaskPane) cycleRoleFilter() {
    roles := p.getUniqueRoles()
    roles = append([]string{"all"}, roles...)

    for i, r := range roles {
        if r == p.roleFilter {
            p.roleFilter = roles[(i+1)%len(roles)]
            break
        }
    }
    p.applyFilters()
}

func (p *TaskPane) filterString() string {
    parts := make([]string, 0)
    if p.statusFilter != "all" {
        parts = append(parts, fmt.Sprintf("status:%s", p.statusFilter))
    }
    if p.roleFilter != "all" {
        parts = append(parts, fmt.Sprintf("role:%s", p.roleFilter))
    }
    return fmt.Sprintf("[Filter: %s]", strings.Join(parts, ", "))
}
```

### Navigation and Actions

```go
func (p *TaskPane) Update(msg tea.Msg) (*TaskPane, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        if !p.focused {
            return p, nil
        }

        switch msg.String() {
        case "j", "down":
            if p.cursor < len(p.filtered)-1 {
                p.cursor++
            }
        case "k", "up":
            if p.cursor > 0 {
                p.cursor--
            }
        case "f":
            p.cycleStatusFilter()
        case "F":
            p.cycleRoleFilter()
        case "enter":
            return p, p.showTaskDetails()
        }

    case tasksRefreshed:
        p.tasks = msg.tasks
        p.applyFilters()
    }

    return p, nil
}

func (p *TaskPane) showTaskDetails() tea.Cmd {
    if p.cursor >= len(p.filtered) {
        return nil
    }

    task := p.filtered[p.cursor]
    return func() tea.Msg {
        return showTaskDetails{task: task}
    }
}
```

### Task Details View

When Enter is pressed, show a modal/overlay with full task details:

```go
type TaskDetailsModal struct {
    task   *task.Task
    width  int
    height int
}

func (m *TaskDetailsModal) View() string {
    if m.task == nil {
        return ""
    }

    style := lipgloss.NewStyle().
        Border(lipgloss.RoundedBorder()).
        Padding(1).
        Width(m.width - 4)

    var sb strings.Builder
    sb.WriteString(fmt.Sprintf("ID: %s\n", m.task.ID))
    sb.WriteString(fmt.Sprintf("Title: %s\n", m.task.Title))
    sb.WriteString(fmt.Sprintf("Status: %s\n", m.task.Status))
    sb.WriteString(fmt.Sprintf("Role: %s\n", m.task.Role))
    sb.WriteString(fmt.Sprintf("Priority: %s\n", m.task.Priority))
    if m.task.AssignedTo != "" {
        sb.WriteString(fmt.Sprintf("Assigned: %s\n", m.task.AssignedTo))
    }
    if len(m.task.DependsOn) > 0 {
        sb.WriteString(fmt.Sprintf("Depends: %s\n", strings.Join(m.task.DependsOn, ", ")))
    }
    sb.WriteString("\n[Press Esc to close]")

    return style.Render(sb.String())
}
```

## Key Bindings (when focused)

| Key | Action |
|-----|--------|
| `j` / `↓` | Move cursor down |
| `k` / `↑` | Move cursor up |
| `f` | Cycle status filter |
| `F` | Cycle role filter |
| `Enter` | Show task details |
| `Esc` | Close details modal |

## Error Handling

| Scenario | Behavior |
|----------|----------|
| No tasks | Show "No tasks" message |
| Filter yields no results | Show "No matching tasks" |
| Refresh fails | Keep old data, show stale indicator |

## Testing

- Test rendering with various task states
- Test cursor navigation bounds
- Test filter cycling
- Test filter combination (status + role)
- Test details modal display

## Files to Create/Modify

- `internal/tui/pane_tasks.go` - TaskPane implementation
- `internal/tui/modal_task.go` - Task details modal
- `internal/tui/style.go` - Add task status icons/colors

## Notes

The task pane depends on Phase 3's task management system. If Phase 3 isn't complete, this pane can show a placeholder or be disabled. Filter state should persist during the TUI session but not across restarts.
