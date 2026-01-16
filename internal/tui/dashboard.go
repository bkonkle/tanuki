package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Pane represents the different panes in the dashboard.
type Pane int

const (
	// PaneAgents is the agents list pane.
	PaneAgents Pane = iota
	// PaneTasks is the tasks list pane.
	PaneTasks
	// PaneLogs is the log viewer pane.
	PaneLogs
)

// AgentInfo represents agent information for display.
type AgentInfo struct {
	Name        string
	Status      string
	Workstream  string
	CurrentTask string
	Branch      string
	Uptime      time.Duration
}

// TaskInfo represents task information for display.
type TaskInfo struct {
	ID             string
	Title          string
	Status         string
	Workstream     string
	AssignedTo     string
	Priority       string
	FailureMessage string
	LogFilePath    string
	ValidationLog  string
	DependsOn      []string
	StartedAt      *time.Time
	CompletedAt    *time.Time
	ErrorPreview   string // Truncated error for list display
}

// LogLine represents a log entry.
type LogLine struct {
	Timestamp time.Time
	Agent     string
	Content   string
	Level     string
}

// AgentProvider is the interface for fetching agent data.
type AgentProvider interface {
	ListAgents() ([]*AgentInfo, error)
	StopAgent(name string) error
	StartAgent(name string) error
}

// TaskProvider is the interface for fetching task data.
type TaskProvider interface {
	ListTasks() ([]*TaskInfo, error)
}

// KeyMap defines the key bindings for the dashboard.
type KeyMap struct {
	Quit             key.Binding
	Help             key.Binding
	Tab              key.Binding
	ShiftTab         key.Binding
	Up               key.Binding
	Down             key.Binding
	Enter            key.Binding
	Stop             key.Binding
	Start            key.Binding
	Attach           key.Binding
	Diff             key.Binding
	Follow           key.Binding
	Filter           key.Binding
	FilterWorkstream key.Binding
	Clear            key.Binding
	Pause            key.Binding
	Top              key.Binding
	Bottom           key.Binding
}

// DefaultKeyMap returns the default key bindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "next pane"),
		),
		ShiftTab: key.NewBinding(
			key.WithKeys("shift+tab"),
			key.WithHelp("shift+tab", "prev pane"),
		),
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("k/↑", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("j/↓", "down"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		),
		Stop: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "stop agent"),
		),
		Start: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "start agent"),
		),
		Attach: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "attach"),
		),
		Diff: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "show diff"),
		),
		Follow: key.NewBinding(
			key.WithKeys("f"),
			key.WithHelp("f", "follow logs"),
		),
		Filter: key.NewBinding(
			key.WithKeys("f"),
			key.WithHelp("f", "filter status"),
		),
		FilterWorkstream: key.NewBinding(
			key.WithKeys("F"),
			key.WithHelp("F", "filter workstream"),
		),
		Clear: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "clear logs"),
		),
		Pause: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "pause logs"),
		),
		Top: key.NewBinding(
			key.WithKeys("g"),
			key.WithHelp("g", "go to top"),
		),
		Bottom: key.NewBinding(
			key.WithKeys("G"),
			key.WithHelp("G", "go to bottom"),
		),
	}
}

// Model is the main dashboard model.
type Model struct {
	// Data
	agents []*AgentInfo
	tasks  []*TaskInfo
	logs   []LogLine

	// UI State
	activePane       Pane
	agentCursor      int
	taskCursor       int
	logOffset        int
	logFollow        bool
	logPaused        bool
	showHelp         bool
	showTaskDetails  bool
	taskDetailsModal *TaskDetailsModal
	statusFilter     string
	workstreamFilter string
	statusMsg        string
	errorMsg         string

	// Dimensions
	width  int
	height int

	// Key bindings
	keys KeyMap

	// Providers (injected dependencies)
	agentProvider AgentProvider
	taskProvider  TaskProvider

	// Log streaming
	logReader      *LogReader
	selectedAgent  string
	maxLogs        int
	logCheckTicker time.Duration

	// Refresh interval
	refreshInterval time.Duration

	// Project root for log file paths
	projectRoot string
}

// NewModel creates a new dashboard model.
func NewModel(agentProvider AgentProvider, taskProvider TaskProvider) Model {
	return Model{
		agents:           make([]*AgentInfo, 0),
		tasks:            make([]*TaskInfo, 0),
		logs:             make([]LogLine, 0),
		activePane:       PaneAgents,
		logFollow:        true,
		statusFilter:     "all",
		workstreamFilter: "all",
		keys:             DefaultKeyMap(),
		agentProvider:    agentProvider,
		taskProvider:     taskProvider,
		maxLogs:          1000,
		logCheckTicker:   100 * time.Millisecond,
		refreshInterval:  time.Second,
	}
}

// SetProjectRoot sets the project root path for resolving log files.
func (m *Model) SetProjectRoot(projectRoot string) {
	m.projectRoot = projectRoot
}

// tickMsg is sent on each refresh interval.
type tickMsg time.Time

// agentsRefreshedMsg contains refreshed agent data.
type agentsRefreshedMsg struct {
	agents []*AgentInfo
	err    error
}

// tasksRefreshedMsg contains refreshed task data.
type tasksRefreshedMsg struct {
	tasks []*TaskInfo
	err   error
}

// actionResultMsg contains the result of an action.
type actionResultMsg struct {
	action string
	agent  string
	err    error
}

// logLineMsg contains a new log line from a log reader.
type logLineMsg struct {
	line LogLine
}

// checkLogsMsg triggers a non-blocking check for new log lines.
type checkLogsMsg time.Time

// Init initializes the model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.tick(),
		m.refreshAgents(),
		m.refreshTasks(),
		m.checkLogsTick(),
	)
}

// tick returns a command that sends tickMsg after the refresh interval.
func (m Model) tick() tea.Cmd {
	return tea.Tick(m.refreshInterval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// refreshAgents fetches agent data from the provider.
func (m Model) refreshAgents() tea.Cmd {
	return func() tea.Msg {
		if m.agentProvider == nil {
			return agentsRefreshedMsg{agents: nil, err: nil}
		}
		agents, err := m.agentProvider.ListAgents()
		return agentsRefreshedMsg{agents: agents, err: err}
	}
}

// refreshTasks fetches task data from the provider.
func (m Model) refreshTasks() tea.Cmd {
	return func() tea.Msg {
		if m.taskProvider == nil {
			return tasksRefreshedMsg{tasks: nil, err: nil}
		}
		tasks, err := m.taskProvider.ListTasks()
		return tasksRefreshedMsg{tasks: tasks, err: err}
	}
}

// checkLogsTick returns a command that periodically checks for new log lines.
func (m Model) checkLogsTick() tea.Cmd {
	return tea.Tick(m.logCheckTicker, func(t time.Time) tea.Msg {
		return checkLogsMsg(t)
	})
}

// checkLogs performs a non-blocking check for new log lines from the log reader.
func (m Model) checkLogs() tea.Cmd {
	return func() tea.Msg {
		if m.logReader == nil || m.logPaused {
			return nil
		}

		// Non-blocking receive from the log reader channel
		select {
		case line := <-m.logReader.OutputCh():
			return logLineMsg{line: line}
		default:
			// No logs available right now
			return nil
		}
	}
}

// Update handles messages and updates the model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		// Handle Esc to close modals/help
		if msg.Type == tea.KeyEsc {
			if m.showTaskDetails {
				m.showTaskDetails = false
				return m, nil
			}
			if m.showHelp {
				m.showHelp = false
				return m, nil
			}
		}

		// If modal is open, pass keys to modal for tab navigation and scrolling
		if m.showTaskDetails && m.taskDetailsModal != nil {
			cmd := m.taskDetailsModal.Update(msg)
			return m, cmd
		}

		// Clear any error/status messages on key press
		m.errorMsg = ""
		m.statusMsg = ""

		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit

		case key.Matches(msg, m.keys.Help):
			m.showHelp = !m.showHelp
			return m, nil

		case key.Matches(msg, m.keys.Tab):
			m.activePane = (m.activePane + 1) % 3
			return m, nil

		case key.Matches(msg, m.keys.ShiftTab):
			m.activePane = (m.activePane + 2) % 3
			return m, nil

		case key.Matches(msg, m.keys.Up):
			m.navigateUp()
			return m, nil

		case key.Matches(msg, m.keys.Down):
			m.navigateDown()
			return m, nil

		case key.Matches(msg, m.keys.Stop):
			if m.activePane == PaneAgents {
				return m, m.stopSelectedAgent()
			}

		case key.Matches(msg, m.keys.Start):
			if m.activePane == PaneAgents {
				return m, m.startSelectedAgent()
			}

		case key.Matches(msg, m.keys.Follow):
			switch m.activePane {
			case PaneLogs:
				m.logFollow = !m.logFollow
				if m.logFollow {
					m.scrollLogsToBottom()
				}
			case PaneTasks:
				m.cycleStatusFilter()
			}
			return m, nil

		case key.Matches(msg, m.keys.FilterWorkstream):
			if m.activePane == PaneTasks {
				m.cycleWorkstreamFilter()
			}
			return m, nil

		case key.Matches(msg, m.keys.Pause):
			if m.activePane == PaneLogs {
				m.logPaused = !m.logPaused
			}
			return m, nil

		case key.Matches(msg, m.keys.Clear):
			if m.activePane == PaneLogs {
				m.logs = make([]LogLine, 0)
				m.logOffset = 0
			}
			return m, nil

		case key.Matches(msg, m.keys.Top):
			if m.activePane == PaneLogs {
				m.logOffset = 0
				m.logFollow = false
			}
			return m, nil

		case key.Matches(msg, m.keys.Bottom):
			if m.activePane == PaneLogs {
				m.scrollLogsToBottom()
			}
			return m, nil

		case key.Matches(msg, m.keys.Enter):
			if m.activePane == PaneAgents && len(m.agents) > 0 {
				// Select agent for log viewing
				agent := m.agents[m.agentCursor]
				m.statusMsg = fmt.Sprintf("Selected agent: %s", agent.Name)
				m.activePane = PaneLogs
				return m, m.switchLogAgent(agent.Name)
			} else if m.activePane == PaneTasks {
				// Show task details modal
				filteredTasks := m.filteredTasks()
				if m.taskCursor < len(filteredTasks) {
					task := filteredTasks[m.taskCursor]
					m.taskDetailsModal = NewTaskDetailsModal(task, m.projectRoot, m.width, m.height)
					m.showTaskDetails = true
				}
			}
			return m, nil
		}

	case tickMsg:
		return m, tea.Batch(
			m.tick(),
			m.refreshAgents(),
			m.refreshTasks(),
		)

	case agentsRefreshedMsg:
		if msg.err != nil {
			m.errorMsg = fmt.Sprintf("Error refreshing agents: %v", msg.err)
		} else {
			m.agents = msg.agents
			// Keep cursor in bounds
			if m.agentCursor >= len(m.agents) {
				m.agentCursor = max(0, len(m.agents)-1)
			}
		}
		return m, nil

	case tasksRefreshedMsg:
		if msg.err != nil {
			m.errorMsg = fmt.Sprintf("Error refreshing tasks: %v", msg.err)
		} else {
			m.tasks = msg.tasks
			// Keep cursor in bounds
			filteredTasks := m.filteredTasks()
			if m.taskCursor >= len(filteredTasks) {
				m.taskCursor = max(0, len(filteredTasks)-1)
			}
		}
		return m, nil

	case actionResultMsg:
		if msg.err != nil {
			m.errorMsg = fmt.Sprintf("%s %s failed: %v", msg.action, msg.agent, msg.err)
		} else {
			m.statusMsg = fmt.Sprintf("%s %s: success", msg.action, msg.agent)
		}
		return m, m.refreshAgents()

	case logLineMsg:
		if !m.logPaused {
			m.AddLogLine(msg.line)
		}
		return m, nil

	case checkLogsMsg:
		// Check for new log lines and schedule next check
		return m, tea.Batch(
			m.checkLogs(),
			m.checkLogsTick(),
		)
	}

	return m, nil
}

// navigateUp moves the cursor up in the current pane.
func (m *Model) navigateUp() {
	switch m.activePane {
	case PaneAgents:
		if m.agentCursor > 0 {
			m.agentCursor--
		}
	case PaneTasks:
		if m.taskCursor > 0 {
			m.taskCursor--
		}
	case PaneLogs:
		if m.logOffset > 0 {
			m.logOffset--
			m.logFollow = false
		}
	}
}

// navigateDown moves the cursor down in the current pane.
func (m *Model) navigateDown() {
	switch m.activePane {
	case PaneAgents:
		if m.agentCursor < len(m.agents)-1 {
			m.agentCursor++
		}
	case PaneTasks:
		filteredTasks := m.filteredTasks()
		if m.taskCursor < len(filteredTasks)-1 {
			m.taskCursor++
		}
	case PaneLogs:
		visibleLines := m.logPaneHeight()
		if m.logOffset < len(m.logs)-visibleLines {
			m.logOffset++
			m.logFollow = false
		}
	}
}

// scrollLogsToBottom scrolls the log view to the bottom.
func (m *Model) scrollLogsToBottom() {
	visibleLines := m.logPaneHeight()
	m.logOffset = max(0, len(m.logs)-visibleLines)
}

// logPaneHeight returns the height available for log lines.
func (m *Model) logPaneHeight() int {
	// Account for header, border, etc.
	return max(1, m.height/3-4)
}

// stopSelectedAgent stops the selected agent.
func (m Model) stopSelectedAgent() tea.Cmd {
	if m.agentCursor >= len(m.agents) || m.agentProvider == nil {
		return nil
	}
	agent := m.agents[m.agentCursor]
	return func() tea.Msg {
		err := m.agentProvider.StopAgent(agent.Name)
		return actionResultMsg{action: "Stop", agent: agent.Name, err: err}
	}
}

// startSelectedAgent starts the selected agent.
func (m Model) startSelectedAgent() tea.Cmd {
	if m.agentCursor >= len(m.agents) || m.agentProvider == nil {
		return nil
	}
	agent := m.agents[m.agentCursor]
	return func() tea.Msg {
		err := m.agentProvider.StartAgent(agent.Name)
		return actionResultMsg{action: "Start", agent: agent.Name, err: err}
	}
}

// cycleStatusFilter cycles through status filter options.
func (m *Model) cycleStatusFilter() {
	filters := []string{"all", "pending", "in_progress", "complete", "failed", "blocked"}
	for i, f := range filters {
		if f == m.statusFilter {
			m.statusFilter = filters[(i+1)%len(filters)]
			m.taskCursor = 0
			return
		}
	}
	m.statusFilter = "all"
}

// cycleWorkstreamFilter cycles through workstream filter options.
func (m *Model) cycleWorkstreamFilter() {
	workstreams := m.getUniqueWorkstreams()
	workstreams = append([]string{"all"}, workstreams...)

	for i, ws := range workstreams {
		if ws == m.workstreamFilter {
			m.workstreamFilter = workstreams[(i+1)%len(workstreams)]
			m.taskCursor = 0
			return
		}
	}
	m.workstreamFilter = "all"
}

// getUniqueWorkstreams returns unique workstreams from tasks.
func (m *Model) getUniqueWorkstreams() []string {
	wsSet := make(map[string]bool)
	for _, t := range m.tasks {
		if t.Workstream != "" {
			wsSet[t.Workstream] = true
		}
	}
	workstreams := make([]string, 0, len(wsSet))
	for ws := range wsSet {
		workstreams = append(workstreams, ws)
	}
	return workstreams
}

// filteredTasks returns tasks matching current filters.
func (m *Model) filteredTasks() []*TaskInfo {
	filtered := make([]*TaskInfo, 0, len(m.tasks))
	for _, t := range m.tasks {
		if m.statusFilter != "all" && t.Status != m.statusFilter {
			continue
		}
		if m.workstreamFilter != "all" && t.Workstream != m.workstreamFilter {
			continue
		}
		filtered = append(filtered, t)
	}
	return filtered
}

// View renders the dashboard.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	if m.showHelp {
		return m.renderHelp()
	}

	// Render main dashboard
	mainView := m.renderMainView()

	// Overlay task details modal if active
	if m.showTaskDetails && m.taskDetailsModal != nil {
		// Create an overlay effect by rendering the modal on top
		modalView := m.taskDetailsModal.View()
		return lipgloss.Place(m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			modalView,
			lipgloss.WithWhitespaceChars(" "),
			lipgloss.WithWhitespaceForeground(lipgloss.Color("0")))
	}

	return mainView
}

// renderMainView renders the main dashboard layout.
func (m Model) renderMainView() string {
	var sb strings.Builder

	// Title bar
	title := TitleStyle.Width(m.width).Render("Tanuki Dashboard")
	sb.WriteString(title)
	sb.WriteString("\n")

	// Calculate pane dimensions
	topHeight := (m.height - 4) * 2 / 3 // 2/3 for top panes
	bottomHeight := m.height - 4 - topHeight
	leftWidth := m.width / 2
	rightWidth := m.width - leftWidth

	// Render agent pane
	agentPane := m.renderAgentPane(leftWidth-2, topHeight-2)
	agentBox := PaneBorder(m.activePane == PaneAgents).
		Width(leftWidth - 2).
		Height(topHeight).
		Render(agentPane)

	// Render task pane
	taskPane := m.renderTaskPane(rightWidth-2, topHeight-2)
	taskBox := PaneBorder(m.activePane == PaneTasks).
		Width(rightWidth - 2).
		Height(topHeight).
		Render(taskPane)

	// Join top panes horizontally
	topRow := lipgloss.JoinHorizontal(lipgloss.Top, agentBox, taskBox)
	sb.WriteString(topRow)
	sb.WriteString("\n")

	// Render log pane
	logPane := m.renderLogPane(m.width-4, bottomHeight-2)
	logBox := PaneBorder(m.activePane == PaneLogs).
		Width(m.width - 2).
		Height(bottomHeight).
		Render(logPane)
	sb.WriteString(logBox)
	sb.WriteString("\n")

	// Status bar
	statusBar := m.renderStatusBar()
	sb.WriteString(statusBar)

	return sb.String()
}

// renderAgentPane renders the agents list pane.
func (m Model) renderAgentPane(width, height int) string {
	var sb strings.Builder

	// Header
	header := HeaderStyle.Render(fmt.Sprintf("Agents [%d]", len(m.agents)))
	sb.WriteString(header)
	sb.WriteString("\n")
	sb.WriteString(strings.Repeat("─", width))
	sb.WriteString("\n")

	if len(m.agents) == 0 {
		sb.WriteString(MutedStyle.Render("No agents"))
		return sb.String()
	}

	// Agent list
	for i, agent := range m.agents {
		if i >= height-3 {
			sb.WriteString(MutedStyle.Render(fmt.Sprintf("... and %d more", len(m.agents)-i)))
			break
		}

		// Selection indicator
		prefix := "  "
		if i == m.agentCursor && m.activePane == PaneAgents {
			prefix = "> "
		}

		// Status icon
		icon := AgentStatusIcon(agent.Status)

		// Name
		name := lipgloss.NewStyle().
			Width(15).
			Render(Truncate(agent.Name, 14))

		// Status
		statusStyle := lipgloss.NewStyle().
			Width(10).
			Foreground(AgentStatusColor(agent.Status))
		status := statusStyle.Render(fmt.Sprintf("[%s]", agent.Status))

		// Current task
		task := ""
		if agent.CurrentTask != "" {
			task = MutedStyle.Render(fmt.Sprintf("→ %s", Truncate(agent.CurrentTask, 15)))
		}

		line := fmt.Sprintf("%s%s %s %s %s", prefix, icon, name, status, task)
		if i == m.agentCursor && m.activePane == PaneAgents {
			line = SelectedStyle.Render(line)
		}
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	return sb.String()
}

// renderTaskPane renders the tasks list pane.
func (m Model) renderTaskPane(width, height int) string {
	var sb strings.Builder

	filteredTasks := m.filteredTasks()

	// Header with counts
	header := HeaderStyle.Render(fmt.Sprintf("Tasks [%d]", len(filteredTasks)))
	sb.WriteString(header)
	sb.WriteString("\n")
	sb.WriteString(strings.Repeat("─", width))
	sb.WriteString("\n")

	// Filter indicator
	if m.statusFilter != "all" || m.workstreamFilter != "all" {
		var filters []string
		if m.statusFilter != "all" {
			filters = append(filters, fmt.Sprintf("status:%s", m.statusFilter))
		}
		if m.workstreamFilter != "all" {
			filters = append(filters, fmt.Sprintf("workstream:%s", m.workstreamFilter))
		}
		filterStr := MutedStyle.Render(fmt.Sprintf("[Filter: %s]", strings.Join(filters, ", ")))
		sb.WriteString(filterStr)
		sb.WriteString("\n")
	}

	if len(filteredTasks) == 0 {
		sb.WriteString(MutedStyle.Render("No tasks"))
		return sb.String()
	}

	// Task list
	for i, task := range filteredTasks {
		if i >= height-4 {
			sb.WriteString(MutedStyle.Render(fmt.Sprintf("... and %d more", len(filteredTasks)-i)))
			break
		}

		// Selection indicator
		prefix := "  "
		if i == m.taskCursor && m.activePane == PaneTasks {
			prefix = "> "
		}

		// Status icon
		icon := TaskStatusIcon(task.Status)

		// Task ID
		id := MutedStyle.Width(10).Render(Truncate(task.ID, 9))

		// Title
		title := lipgloss.NewStyle().
			Width(18).
			Render(Truncate(task.Title, 17))

		// Workstream
		workstream := InfoStyle.Width(10).Render(Truncate(task.Workstream, 9))

		// Assigned agent
		assigned := ""
		if task.AssignedTo != "" {
			assigned = WarningStyle.Render(fmt.Sprintf("→ %s", Truncate(task.AssignedTo, 10)))
		}

		line := fmt.Sprintf("%s%s %s %s %s %s", prefix, icon, id, title, workstream, assigned)
		if i == m.taskCursor && m.activePane == PaneTasks {
			line = SelectedStyle.Render(line)
		}
		sb.WriteString(line)
		sb.WriteString("\n")

		// Show error preview for failed tasks
		if task.Status == "failed" && task.ErrorPreview != "" {
			errorStyle := lipgloss.NewStyle().
				Foreground(ColorError).
				Italic(true)
			errorLine := fmt.Sprintf("    ↳ %s", task.ErrorPreview)
			sb.WriteString(errorStyle.Render(errorLine))
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// renderLogPane renders the log viewer pane.
func (m Model) renderLogPane(width, height int) string {
	var sb strings.Builder

	// Header
	agentName := "none"
	if m.agentCursor < len(m.agents) {
		agentName = m.agents[m.agentCursor].Name
	}

	headerParts := []string{fmt.Sprintf("Logs: %s", agentName)}
	if m.logFollow {
		headerParts = append(headerParts, SuccessStyle.Render("[follow]"))
	}
	if m.logPaused {
		headerParts = append(headerParts, WarningStyle.Render("[paused]"))
	}

	header := HeaderStyle.Render(strings.Join(headerParts, " "))
	sb.WriteString(header)
	sb.WriteString("\n")
	sb.WriteString(strings.Repeat("─", width))
	sb.WriteString("\n")

	if len(m.logs) == 0 {
		sb.WriteString(MutedStyle.Render("No logs yet. Select an agent and press Enter."))
		return sb.String()
	}

	// Calculate visible lines
	visibleLines := height - 3
	endIdx := min(m.logOffset+visibleLines, len(m.logs))

	// Render visible logs
	for i := m.logOffset; i < endIdx; i++ {
		line := m.logs[i]

		// Timestamp
		ts := MutedStyle.Render(line.Timestamp.Format("[15:04:05]"))

		// Content with level coloring
		contentStyle := lipgloss.NewStyle().Foreground(LogLevelColor(line.Level))
		content := contentStyle.Render(Truncate(line.Content, width-12))

		sb.WriteString(fmt.Sprintf("%s %s\n", ts, content))
	}

	return sb.String()
}

// renderStatusBar renders the bottom status bar.
func (m Model) renderStatusBar() string {
	// Left side: status/error message
	left := ""
	if m.errorMsg != "" {
		left = ErrorStyle.Render(m.errorMsg)
	} else if m.statusMsg != "" {
		left = InfoStyle.Render(m.statusMsg)
	}

	// Right side: help hint
	right := HelpStyle.Render("[?] help  [q] quit  [tab] switch pane")

	// Pad to fill width
	padding := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if padding < 0 {
		padding = 0
	}

	return StatusBarStyle.Width(m.width).Render(
		left + strings.Repeat(" ", padding) + right,
	)
}

// renderHelp renders the help screen.
func (m Model) renderHelp() string {
	var sb strings.Builder

	title := TitleStyle.Width(m.width).Render("Tanuki Dashboard - Help")
	sb.WriteString(title)
	sb.WriteString("\n\n")

	sections := []struct {
		title string
		keys  []string
	}{
		{
			title: "Navigation",
			keys: []string{
				"Tab / Shift+Tab  Switch pane",
				"j/k or ↓/↑       Navigate list",
				"Enter            Select item",
			},
		},
		{
			title: "Agent Actions (Agents pane)",
			keys: []string{
				"s                Stop agent",
				"r                Start agent",
				"a                Attach to agent",
				"d                Show diff",
			},
		},
		{
			title: "Task Actions (Tasks pane)",
			keys: []string{
				"Enter            Show task details",
				"f                Cycle status filter",
				"F                Cycle workstream filter",
			},
		},
		{
			title: "Log Actions (Logs pane)",
			keys: []string{
				"f                Toggle follow mode",
				"p                Pause/resume",
				"c                Clear logs",
				"g                Go to top",
				"G                Go to bottom",
			},
		},
		{
			title: "General",
			keys: []string{
				"?                Toggle help",
				"q / Ctrl+C       Quit",
			},
		},
	}

	for _, section := range sections {
		sb.WriteString(HeaderStyle.Render(section.title))
		sb.WriteString("\n")
		for _, k := range section.keys {
			sb.WriteString("  " + k + "\n")
		}
		sb.WriteString("\n")
	}

	sb.WriteString(HelpStyle.Render("Press ? to close help"))

	return sb.String()
}

// AddLogLine adds a log line to the buffer.
func (m *Model) AddLogLine(line LogLine) {
	m.logs = append(m.logs, line)
	if len(m.logs) > m.maxLogs {
		m.logs = m.logs[len(m.logs)-m.maxLogs:]
		if m.logOffset > 0 {
			m.logOffset--
		}
	}

	if m.logFollow {
		m.scrollLogsToBottom()
	}
}

// switchLogAgent switches the log viewer to a different agent.
func (m *Model) switchLogAgent(agentName string) tea.Cmd {
	// Stop existing log reader if any
	if m.logReader != nil {
		m.logReader.Stop()
		m.logReader = nil
	}

	// Clear logs for new agent
	m.logs = make([]LogLine, 0)
	m.logOffset = 0
	m.selectedAgent = agentName
	m.logFollow = true

	// Start new log reader
	m.logReader = NewLogReader(agentName)
	if err := m.logReader.Start(); err != nil {
		m.errorMsg = fmt.Sprintf("Failed to start log reader: %v", err)
		return nil
	}

	return nil
}

// SetAgents sets the agents list (for testing).
func (m *Model) SetAgents(agents []*AgentInfo) {
	m.agents = agents
}

// SetTasks sets the tasks list (for testing).
func (m *Model) SetTasks(tasks []*TaskInfo) {
	m.tasks = tasks
}

// GetActivePane returns the currently active pane.
func (m Model) GetActivePane() Pane {
	return m.activePane
}

// GetAgentCursor returns the current agent cursor position.
func (m Model) GetAgentCursor() int {
	return m.agentCursor
}

// GetTaskCursor returns the current task cursor position.
func (m Model) GetTaskCursor() int {
	return m.taskCursor
}
