package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TaskDetailsModal renders a modal showing task details with tabs.
type TaskDetailsModal struct {
	task        *TaskInfo
	tabs        *TabsModel
	viewport    viewport.Model
	width       int
	height      int
	projectRoot string
	logReader   *TaskLogReader
}

// NewTaskDetailsModal creates a new task details modal with tabs.
func NewTaskDetailsModal(task *TaskInfo, projectRoot string, width, height int) *TaskDetailsModal {
	// Calculate modal dimensions (80% of screen, min 70x25)
	modalWidth := max(min(width*4/5, 100), 70)
	modalHeight := max(min(height*4/5, 35), 25)

	// Create viewport for scrollable content
	// Reserve space for: title (1) + tabs (3) + padding (4) + border (2) + help (1)
	contentHeight := modalHeight - 11
	vp := viewport.New(modalWidth-6, contentHeight)

	m := &TaskDetailsModal{
		task:        task,
		width:       width,
		height:      height,
		projectRoot: projectRoot,
		viewport:    vp,
	}

	// Create log reader if log file path is available
	if task.LogFilePath != "" {
		m.logReader = NewTaskLogReader(projectRoot, task.LogFilePath)
	}

	// Define tabs
	tabs := []Tab{
		{Title: "Details", Content: m.renderDetails},
		{Title: "Error Info", Content: m.renderErrors},
		{Title: "Logs", Content: m.renderLogs},
		{Title: "Validation", Content: m.renderValidation},
	}

	// Create tabs model
	m.tabs = NewTabsModel(tabs, modalWidth-6, contentHeight)

	// Set initial viewport content
	m.updateViewportContent()

	return m
}

// Update handles input for the modal.
func (m *TaskDetailsModal) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle tab switching
		if m.tabs.HandleKey(msg) {
			m.updateViewportContent()
			return nil
		}

		// Handle viewport scrolling
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return cmd
	}

	return nil
}

// updateViewportContent updates the viewport with the active tab's content.
func (m *TaskDetailsModal) updateViewportContent() {
	content := m.tabs.GetActiveContent()
	m.viewport.SetContent(content)
	m.viewport.GotoTop()
}

// View renders the task details modal.
func (m *TaskDetailsModal) View() string {
	if m.task == nil {
		return ""
	}

	// Calculate modal dimensions
	modalWidth := max(min(m.width*4/5, 100), 70)
	modalHeight := max(min(m.height*4/5, 35), 25)

	var sb strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary).
		Width(modalWidth - 6)
	sb.WriteString(titleStyle.Render(fmt.Sprintf("%s: %s", m.task.ID, m.task.Title)))
	sb.WriteString("\n")

	// Render tabs
	m.tabs.SetSize(modalWidth-6, modalHeight-11)
	tabBar := m.tabs.renderTabBar()
	sb.WriteString(tabBar)
	sb.WriteString("\n")

	// Render viewport content
	sb.WriteString(m.viewport.View())
	sb.WriteString("\n\n")

	// Help text
	helpText := MutedStyle.Render("[←/→ or 1-4: Switch tabs | ↑/↓: Scroll | Esc: Close]")
	sb.WriteString(helpText)

	// Create the bordered modal
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary).
		Padding(1, 2).
		Width(modalWidth).
		Height(modalHeight)

	modalContent := modalStyle.Render(sb.String())

	// Center the modal on screen
	verticalPadding := max(0, (m.height-modalHeight)/2)
	horizontalPadding := max(0, (m.width-modalWidth)/2)

	// Build the full view with padding
	var fullView strings.Builder
	for i := 0; i < verticalPadding; i++ {
		fullView.WriteString("\n")
	}

	// Add horizontal padding to each line
	lines := strings.Split(modalContent, "\n")
	padding := strings.Repeat(" ", horizontalPadding)
	for _, line := range lines {
		fullView.WriteString(padding)
		fullView.WriteString(line)
		fullView.WriteString("\n")
	}

	return fullView.String()
}

// renderDetails renders the Details tab content.
func (m *TaskDetailsModal) renderDetails() string {
	var sb strings.Builder

	// Status with icon
	statusIcon := TaskStatusIcon(m.task.Status)
	statusColor := TaskStatusColor(m.task.Status)
	statusStyle := lipgloss.NewStyle().Foreground(statusColor).Bold(true)
	sb.WriteString(fmt.Sprintf("Status: %s %s\n\n",
		statusIcon,
		statusStyle.Render(m.task.Status)))

	// Role
	if m.task.Role != "" {
		roleStyle := lipgloss.NewStyle().Foreground(ColorInfo)
		sb.WriteString(fmt.Sprintf("Role:     %s\n", roleStyle.Render(m.task.Role)))
	}

	// Priority
	if m.task.Priority != "" {
		priorityStyle := lipgloss.NewStyle().Foreground(priorityColor(m.task.Priority))
		sb.WriteString(fmt.Sprintf("Priority: %s\n", priorityStyle.Render(m.task.Priority)))
	}

	// Assigned agent
	if m.task.AssignedTo != "" {
		assignedStyle := lipgloss.NewStyle().Foreground(ColorWarning)
		sb.WriteString(fmt.Sprintf("Assigned: %s\n", assignedStyle.Render(m.task.AssignedTo)))
	} else {
		sb.WriteString(fmt.Sprintf("Assigned: %s\n", MutedStyle.Render("(none)")))
	}

	sb.WriteString("\n")

	// Dependencies
	if len(m.task.DependsOn) > 0 {
		sb.WriteString(fmt.Sprintf("Dependencies:\n"))
		for _, dep := range m.task.DependsOn {
			sb.WriteString(fmt.Sprintf("  • %s\n", dep))
		}
	} else {
		sb.WriteString(MutedStyle.Render("No dependencies\n"))
	}

	sb.WriteString("\n")

	// Timestamps (if available in the future)
	if m.task.StartedAt != nil {
		sb.WriteString(fmt.Sprintf("Started:   %s\n", m.task.StartedAt.Format("2006-01-02 15:04:05")))
	}
	if m.task.CompletedAt != nil {
		sb.WriteString(fmt.Sprintf("Completed: %s\n", m.task.CompletedAt.Format("2006-01-02 15:04:05")))
	}

	return sb.String()
}

// renderErrors renders the Error Info tab content.
func (m *TaskDetailsModal) renderErrors() string {
	if m.task.FailureMessage == "" {
		return MutedStyle.Render("No error information available.\n\nThis task has not failed or error details were not captured.")
	}

	var sb strings.Builder

	// Error message
	errorStyle := lipgloss.NewStyle().Foreground(ColorError).Bold(true)
	sb.WriteString(errorStyle.Render("Error Message:"))
	sb.WriteString("\n\n")
	sb.WriteString(m.task.FailureMessage)
	sb.WriteString("\n\n")

	// Log file path
	if m.task.LogFilePath != "" {
		sb.WriteString(MutedStyle.Render("Full execution logs available in the 'Logs' tab\n"))
		sb.WriteString(MutedStyle.Render(fmt.Sprintf("Log file: %s\n", m.task.LogFilePath)))
	}

	// Timestamps
	if m.task.CompletedAt != nil {
		sb.WriteString("\n")
		sb.WriteString(fmt.Sprintf("Failed at: %s\n", m.task.CompletedAt.Format("2006-01-02 15:04:05")))
	}

	return sb.String()
}

// renderLogs renders the Logs tab content.
func (m *TaskDetailsModal) renderLogs() string {
	if m.task.LogFilePath == "" {
		return MutedStyle.Render("No log file available.\n\nTask execution logs are not available for this task.")
	}

	if m.logReader == nil || !m.logReader.Exists() {
		return MutedStyle.Render(fmt.Sprintf("Log file not found: %s\n\nThe log file may have been deleted or moved.", m.task.LogFilePath))
	}

	// Load logs from file
	logs, err := m.logReader.LoadLogs()
	if err != nil {
		errorStyle := lipgloss.NewStyle().Foreground(ColorError)
		return errorStyle.Render(fmt.Sprintf("Error loading logs: %v", err))
	}

	if logs == "" {
		return MutedStyle.Render("Log file is empty.")
	}

	return logs
}

// renderValidation renders the Validation tab content.
func (m *TaskDetailsModal) renderValidation() string {
	if m.task.ValidationLog == "" {
		return MutedStyle.Render("No validation log available.\n\nValidation output is only available for tasks with verify commands.")
	}

	// For now, just show the path - in the future we could read the validation log file
	var sb strings.Builder

	sb.WriteString(MutedStyle.Render("Validation Log:"))
	sb.WriteString("\n\n")
	sb.WriteString(fmt.Sprintf("Log file: %s\n", m.task.ValidationLog))
	sb.WriteString("\n")
	sb.WriteString(MutedStyle.Render("(Reading validation logs from file will be implemented in a future update)"))

	return sb.String()
}

// priorityColor returns the color for a priority level.
func priorityColor(priority string) lipgloss.Color {
	switch priority {
	case "critical":
		return ColorError
	case "high":
		return ColorWarning
	case "medium":
		return ColorInfo
	case "low":
		return ColorSecondary
	default:
		return lipgloss.Color("255")
	}
}
