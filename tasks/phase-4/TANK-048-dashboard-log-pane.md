---
id: TANK-048
title: Dashboard Log Pane
status: todo
priority: low
estimate: L
depends_on: [TANK-041, TANK-012]
workstream: C
phase: 4
---

# Dashboard Log Pane

## Summary

Implement real-time log streaming in the dashboard for monitoring agent activity. The log pane shows output from the selected agent with timestamps, follow mode, and search capability.

## Acceptance Criteria

- [ ] Stream logs from selected agent in real-time
- [ ] Follow mode (auto-scroll to bottom)
- [ ] Timestamp formatting with configurable format
- [ ] Log buffer with configurable size limit
- [ ] Toggle between agents without losing position
- [ ] Pause/resume streaming
- [ ] Basic search/filter (optional)
- [ ] Different colors for log levels (if detectable)

## Technical Details

### Log Pane Model

```go
type LogPane struct {
    logs         []LogLine
    agentName    string
    follow       bool
    paused       bool
    scrollOffset int
    width        int
    height       int
    maxLines     int
    logReader    *LogReader
    focused      bool
}

type LogLine struct {
    Timestamp time.Time
    Agent     string
    Content   string
    Level     string // "info", "warn", "error", or ""
}

func NewLogPane(maxLines int) *LogPane {
    return &LogPane{
        logs:     make([]LogLine, 0),
        follow:   true,
        maxLines: maxLines,
    }
}
```

### Log Reader

```go
type LogReader struct {
    containerName string
    outputCh      chan LogLine
    stopCh        chan struct{}
    cmd           *exec.Cmd
}

func NewLogReader(agentName string) *LogReader {
    return &LogReader{
        containerName: fmt.Sprintf("tanuki-agent-%s", agentName),
        outputCh:      make(chan LogLine, 100),
        stopCh:        make(chan struct{}),
    }
}

func (r *LogReader) Start() error {
    r.cmd = exec.Command("docker", "logs", "-f", "--tail", "100", r.containerName)

    stdout, err := r.cmd.StdoutPipe()
    if err != nil {
        return err
    }

    stderr, err := r.cmd.StderrPipe()
    if err != nil {
        return err
    }

    if err := r.cmd.Start(); err != nil {
        return err
    }

    // Read stdout
    go r.readOutput(stdout)
    // Read stderr (often where Claude Code writes)
    go r.readOutput(stderr)

    return nil
}

func (r *LogReader) readOutput(reader io.Reader) {
    scanner := bufio.NewScanner(reader)
    for scanner.Scan() {
        select {
        case <-r.stopCh:
            return
        default:
            line := LogLine{
                Timestamp: time.Now(),
                Agent:     r.containerName,
                Content:   scanner.Text(),
                Level:     detectLevel(scanner.Text()),
            }
            r.outputCh <- line
        }
    }
}

func (r *LogReader) Stop() {
    close(r.stopCh)
    if r.cmd != nil && r.cmd.Process != nil {
        r.cmd.Process.Kill()
    }
}

func detectLevel(line string) string {
    lower := strings.ToLower(line)
    if strings.Contains(lower, "error") || strings.Contains(lower, "err:") {
        return "error"
    }
    if strings.Contains(lower, "warn") {
        return "warn"
    }
    return "info"
}
```

### Rendering

```go
func (p *LogPane) View() string {
    var sb strings.Builder

    // Header
    header := p.renderHeader()
    sb.WriteString(header)
    sb.WriteString("\n")
    sb.WriteString(strings.Repeat("─", p.width))
    sb.WriteString("\n")

    // Calculate visible lines
    visibleLines := p.height - 3
    startIdx := p.scrollOffset
    endIdx := min(startIdx+visibleLines, len(p.logs))

    // Render visible logs
    for i := startIdx; i < endIdx; i++ {
        line := p.renderLogLine(p.logs[i])
        sb.WriteString(line)
        sb.WriteString("\n")
    }

    // Fill remaining space
    for i := endIdx - startIdx; i < visibleLines; i++ {
        sb.WriteString("\n")
    }

    return sb.String()
}

func (p *LogPane) renderHeader() string {
    agentStr := "none"
    if p.agentName != "" {
        agentStr = p.agentName
    }

    followIndicator := ""
    if p.follow {
        followIndicator = lipgloss.NewStyle().
            Foreground(lipgloss.Color("42")).
            Render(" [follow]")
    }

    pausedIndicator := ""
    if p.paused {
        pausedIndicator = lipgloss.NewStyle().
            Foreground(lipgloss.Color("226")).
            Render(" [paused]")
    }

    return lipgloss.NewStyle().
        Bold(true).
        Foreground(lipgloss.Color("12")).
        Render(fmt.Sprintf("Logs: %s%s%s", agentStr, followIndicator, pausedIndicator))
}

func (p *LogPane) renderLogLine(line LogLine) string {
    // Timestamp
    ts := lipgloss.NewStyle().
        Foreground(lipgloss.Color("245")).
        Render(line.Timestamp.Format("[15:04:05]"))

    // Content with level-based coloring
    contentStyle := lipgloss.NewStyle()
    switch line.Level {
    case "error":
        contentStyle = contentStyle.Foreground(lipgloss.Color("196"))
    case "warn":
        contentStyle = contentStyle.Foreground(lipgloss.Color("226"))
    }

    content := contentStyle.Render(truncate(line.Content, p.width-12))

    return fmt.Sprintf("%s %s", ts, content)
}
```

### Display Format

```
Logs: backend-agent [follow]
─────────────────────────────────────────────────────────
[10:15:32] Reading file src/api/routes.ts
[10:15:33] Analyzing current API structure...
[10:15:35] Found 12 endpoints to refactor
[10:15:40] Starting with /api/users endpoint
[10:15:42] Editing file src/api/routes.ts
```

### Navigation and Controls

```go
func (p *LogPane) Update(msg tea.Msg) (*LogPane, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        if !p.focused {
            return p, nil
        }

        switch msg.String() {
        case "j", "down":
            if p.scrollOffset < len(p.logs)-1 {
                p.scrollOffset++
                p.follow = false
            }
        case "k", "up":
            if p.scrollOffset > 0 {
                p.scrollOffset--
                p.follow = false
            }
        case "f":
            p.follow = !p.follow
            if p.follow {
                p.scrollToBottom()
            }
        case "p":
            p.paused = !p.paused
        case "G":
            p.scrollToBottom()
        case "g":
            p.scrollOffset = 0
            p.follow = false
        case "c":
            p.clear()
        }

    case LogLineMsg:
        if !p.paused {
            p.addLine(msg.line)
        }

    case agentSelected:
        return p, p.switchAgent(msg.name)
    }

    return p, nil
}

func (p *LogPane) addLine(line LogLine) {
    p.logs = append(p.logs, line)

    // Trim if over max
    if len(p.logs) > p.maxLines {
        p.logs = p.logs[len(p.logs)-p.maxLines:]
        if p.scrollOffset > 0 {
            p.scrollOffset--
        }
    }

    // Auto-scroll if following
    if p.follow {
        p.scrollToBottom()
    }
}

func (p *LogPane) scrollToBottom() {
    visibleLines := p.height - 3
    p.scrollOffset = max(0, len(p.logs)-visibleLines)
}

func (p *LogPane) clear() {
    p.logs = make([]LogLine, 0)
    p.scrollOffset = 0
}
```

### Agent Switching

```go
func (p *LogPane) switchAgent(name string) tea.Cmd {
    return func() tea.Msg {
        // Stop existing reader
        if p.logReader != nil {
            p.logReader.Stop()
        }

        // Clear logs for new agent
        p.logs = make([]LogLine, 0)
        p.scrollOffset = 0
        p.agentName = name

        // Start new reader
        p.logReader = NewLogReader(name)
        if err := p.logReader.Start(); err != nil {
            return logError{err: err}
        }

        // Return command to listen for logs
        return p.listenForLogs()
    }
}

func (p *LogPane) listenForLogs() tea.Cmd {
    return func() tea.Msg {
        if p.logReader == nil {
            return nil
        }

        select {
        case line := <-p.logReader.outputCh:
            return LogLineMsg{line: line}
        case <-time.After(100 * time.Millisecond):
            return nil
        }
    }
}
```

### Continuous Log Subscription

```go
func (p *LogPane) subscribeToLogs() tea.Cmd {
    return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
        return checkLogsMsg{}
    })
}

func (p *LogPane) handleCheckLogs() tea.Cmd {
    if p.logReader == nil || p.paused {
        return nil
    }

    // Non-blocking check for new logs
    select {
    case line := <-p.logReader.outputCh:
        return func() tea.Msg {
            return LogLineMsg{line: line}
        }
    default:
        return nil
    }
}
```

## Key Bindings (when focused)

| Key | Action |
|-----|--------|
| `j` / `↓` | Scroll down (disables follow) |
| `k` / `↑` | Scroll up (disables follow) |
| `f` | Toggle follow mode |
| `p` | Pause/resume streaming |
| `G` | Jump to bottom |
| `g` | Jump to top |
| `c` | Clear log buffer |

## Error Handling

| Scenario | Behavior |
|----------|----------|
| No agent selected | Show "Select an agent" message |
| Container not running | Show "Agent not running" message |
| Log read error | Show error, retry after delay |
| Buffer overflow | Trim oldest entries |

## Testing

- Test log rendering with various line lengths
- Test scroll navigation bounds
- Test follow mode behavior
- Test pause/resume
- Test agent switching clears logs
- Test buffer trimming at max capacity
- Test level detection and coloring

## Files to Create/Modify

- `internal/tui/pane_logs.go` - LogPane implementation
- `internal/tui/logreader.go` - LogReader implementation
- `internal/tui/style.go` - Add log level colors

## Notes

The log pane is the most complex of the three panes due to real-time streaming. Consider using a goroutine-safe ring buffer for logs to handle high-volume output. The default max of 1000 lines balances memory usage with usefulness.

For very long lines, consider wrapping instead of truncating, or allow horizontal scrolling.
