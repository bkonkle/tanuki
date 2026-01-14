package tui

import (
	"strings"
	"testing"
)

func TestNewTaskDetailsModal(t *testing.T) {
	task := &TaskInfo{
		ID:     "TASK-001",
		Title:  "Test Task",
		Status: "in_progress",
		Role:   "backend",
	}

	modal := NewTaskDetailsModal(task, 100, 40)

	if modal.task != task {
		t.Error("expected task to be set")
	}
	if modal.width != 100 {
		t.Errorf("expected width 100, got %d", modal.width)
	}
	if modal.height != 40 {
		t.Errorf("expected height 40, got %d", modal.height)
	}
}

func TestTaskDetailsModal_View_NilTask(t *testing.T) {
	modal := &TaskDetailsModal{
		task:   nil,
		width:  100,
		height: 40,
	}

	view := modal.View()
	if view != "" {
		t.Error("expected empty view for nil task")
	}
}

func TestTaskDetailsModal_View_WithTask(t *testing.T) {
	task := &TaskInfo{
		ID:         "TASK-001",
		Title:      "Test Task",
		Status:     "in_progress",
		Role:       "backend",
		Priority:   "high",
		AssignedTo: "agent-1",
	}

	modal := NewTaskDetailsModal(task, 100, 40)
	view := modal.View()

	// Should contain task ID
	if !strings.Contains(view, "TASK-001") {
		t.Error("expected view to contain task ID")
	}

	// Should contain title
	if !strings.Contains(view, "Test Task") {
		t.Error("expected view to contain task title")
	}

	// Should contain status
	if !strings.Contains(view, "in_progress") {
		t.Error("expected view to contain status")
	}

	// Should contain role
	if !strings.Contains(view, "backend") {
		t.Error("expected view to contain role")
	}

	// Should contain priority
	if !strings.Contains(view, "high") {
		t.Error("expected view to contain priority")
	}

	// Should contain assigned agent
	if !strings.Contains(view, "agent-1") {
		t.Error("expected view to contain assigned agent")
	}

	// Should contain help text
	if !strings.Contains(view, "Esc") {
		t.Error("expected view to contain Esc help text")
	}
}

func TestTaskDetailsModal_View_NoAssignment(t *testing.T) {
	task := &TaskInfo{
		ID:     "TASK-002",
		Title:  "Unassigned Task",
		Status: "pending",
		Role:   "frontend",
	}

	modal := NewTaskDetailsModal(task, 100, 40)
	view := modal.View()

	// Should indicate no assignment
	if !strings.Contains(view, "(none)") {
		t.Error("expected view to indicate no assignment")
	}
}

func TestPriorityColor(t *testing.T) {
	tests := []struct {
		priority string
		expected string
	}{
		{"critical", string(ColorError)},
		{"high", string(ColorWarning)},
		{"medium", string(ColorInfo)},
		{"low", string(ColorSecondary)},
		{"unknown", "255"},
	}

	for _, tt := range tests {
		color := priorityColor(tt.priority)
		if string(color) != tt.expected {
			t.Errorf("priorityColor(%s) = %s, want %s", tt.priority, color, tt.expected)
		}
	}
}
