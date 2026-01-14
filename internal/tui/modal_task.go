package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// showTaskDetailsMsg is sent when a task detail modal should be shown.
type showTaskDetailsMsg struct {
	task *TaskInfo
}

// closeModalMsg is sent when the modal should be closed.
type closeModalMsg struct{}

// TaskDetailsModal renders a modal showing full task details.
type TaskDetailsModal struct {
	task   *TaskInfo
	width  int
	height int
}

// NewTaskDetailsModal creates a new task details modal.
func NewTaskDetailsModal(task *TaskInfo, width, height int) *TaskDetailsModal {
	return &TaskDetailsModal{
		task:   task,
		width:  width,
		height: height,
	}
}

// View renders the task details modal.
func (m *TaskDetailsModal) View() string {
	if m.task == nil {
		return ""
	}

	// Calculate modal dimensions (80% of screen, max 60x20)
	modalWidth := min(m.width*4/5, 60)
	modalHeight := min(m.height*4/5, 20)

	// Build content
	var sb strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary).
		Width(modalWidth - 4)
	sb.WriteString(titleStyle.Render(fmt.Sprintf("%s: %s", m.task.ID, m.task.Title)))
	sb.WriteString("\n\n")

	// Status with icon
	statusIcon := TaskStatusIcon(m.task.Status)
	statusColor := TaskStatusColor(m.task.Status)
	statusStyle := lipgloss.NewStyle().Foreground(statusColor)
	sb.WriteString(fmt.Sprintf("Status: %s %s\n",
		statusIcon,
		statusStyle.Render(m.task.Status)))

	// Role
	if m.task.Role != "" {
		roleStyle := lipgloss.NewStyle().Foreground(ColorInfo)
		sb.WriteString(fmt.Sprintf("Role:   %s\n", roleStyle.Render(m.task.Role)))
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

	// Help text
	helpText := MutedStyle.Render("[Press Esc to close]")
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
