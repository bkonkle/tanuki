// Package tui provides the BubbleTea-based terminal user interface for Tanuki.
package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// Colors used throughout the TUI.
var (
	ColorPrimary   = lipgloss.Color("12")  // Blue
	ColorSecondary = lipgloss.Color("245") // Gray
	ColorSuccess   = lipgloss.Color("42")  // Green
	ColorWarning   = lipgloss.Color("226") // Yellow
	ColorError     = lipgloss.Color("196") // Red
	ColorInfo      = lipgloss.Color("14")  // Cyan
	ColorMuted     = lipgloss.Color("240") // Dark gray
	ColorHighlight = lipgloss.Color("33")  // Blue
	ColorOrange    = lipgloss.Color("208") // Orange
)

// Base styles.
var (
	// HeaderStyle is used for pane headers.
	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary)

	// SelectedStyle highlights the currently selected item.
	SelectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("255")).
			Background(lipgloss.Color("57"))

	// MutedStyle is for secondary/muted text.
	MutedStyle = lipgloss.NewStyle().
			Foreground(ColorSecondary)

	// SuccessStyle is for success indicators.
	SuccessStyle = lipgloss.NewStyle().
			Foreground(ColorSuccess)

	// WarningStyle is for warning indicators.
	WarningStyle = lipgloss.NewStyle().
			Foreground(ColorWarning)

	// ErrorStyle is for error indicators.
	ErrorStyle = lipgloss.NewStyle().
			Foreground(ColorError)

	// InfoStyle is for informational text.
	InfoStyle = lipgloss.NewStyle().
			Foreground(ColorInfo)

	// HelpStyle is for help text at the bottom.
	HelpStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	// TitleStyle is for the main title bar.
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("255")).
			Background(lipgloss.Color("62")).
			Padding(0, 1)

	// StatusBarStyle is for the status bar.
	StatusBarStyle = lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Background(lipgloss.Color("236"))
)

// AgentStatusIcon returns a styled icon for agent status.
func AgentStatusIcon(status string) string {
	switch status {
	case "idle":
		return lipgloss.NewStyle().
			Foreground(ColorSuccess).
			Render("●")
	case "working":
		return lipgloss.NewStyle().
			Foreground(ColorWarning).
			Render("◐")
	case "stopped":
		return lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Render("○")
	case "error":
		return lipgloss.NewStyle().
			Foreground(ColorError).
			Render("✗")
	default:
		return "?"
	}
}

// AgentStatusColor returns the color for an agent status.
func AgentStatusColor(status string) lipgloss.Color {
	switch status {
	case "idle":
		return ColorSuccess
	case "working":
		return ColorWarning
	case "stopped":
		return ColorSecondary
	case "error":
		return ColorError
	default:
		return lipgloss.Color("255")
	}
}

// TaskStatusIcon returns a styled icon for task status.
func TaskStatusIcon(status string) string {
	switch status {
	case "pending":
		return lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Render("○")
	case "assigned":
		return lipgloss.NewStyle().
			Foreground(ColorHighlight).
			Render("◔")
	case "in_progress":
		return lipgloss.NewStyle().
			Foreground(ColorWarning).
			Render("◐")
	case "review":
		return lipgloss.NewStyle().
			Foreground(ColorInfo).
			Render("◑")
	case "complete":
		return lipgloss.NewStyle().
			Foreground(ColorSuccess).
			Render("✓")
	case "failed":
		return lipgloss.NewStyle().
			Foreground(ColorError).
			Render("✗")
	case "blocked":
		return lipgloss.NewStyle().
			Foreground(ColorOrange).
			Render("⊘")
	default:
		return "?"
	}
}

// TaskStatusColor returns the color for a task status.
func TaskStatusColor(status string) lipgloss.Color {
	switch status {
	case "pending":
		return ColorSecondary
	case "assigned":
		return ColorHighlight
	case "in_progress":
		return ColorWarning
	case "review":
		return ColorInfo
	case "complete":
		return ColorSuccess
	case "failed":
		return ColorError
	case "blocked":
		return ColorOrange
	default:
		return lipgloss.Color("255")
	}
}

// LogLevelColor returns the color for a log level.
func LogLevelColor(level string) lipgloss.Color {
	switch level {
	case "error":
		return ColorError
	case "warn":
		return ColorWarning
	case "info":
		return lipgloss.Color("255") // White (default)
	default:
		return lipgloss.Color("255")
	}
}

// PaneBorder returns a border style for panes.
func PaneBorder(focused bool) lipgloss.Style {
	borderColor := ColorMuted
	if focused {
		borderColor = ColorPrimary
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor)
}

// Truncate truncates a string to the given maxLen length, adding "..." if needed.
func Truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
