package task

import (
	"context"
	"testing"
	"time"
)

// mockAgentExecutor implements AgentExecutor for testing.
type mockAgentExecutor struct {
	outputs []string
	errors  []error
	calls   int
}

func newMockAgentExecutor(outputs []string, errors []error) *mockAgentExecutor {
	return &mockAgentExecutor{
		outputs: outputs,
		errors:  errors,
	}
}

func (m *mockAgentExecutor) Run(_, _ string) (string, error) {
	idx := m.calls
	m.calls++

	var output string
	var err error

	if idx < len(m.outputs) {
		output = m.outputs[idx]
	}
	if idx < len(m.errors) {
		err = m.errors[idx]
	}

	return output, err
}

func TestRunner_RunTask_SingleShot(t *testing.T) {
	taskMgr := newMockTaskMgr()
	agentMgr := newMockAgentExecutor([]string{"Output with DONE"}, nil)
	events := make(chan Event, 10)
	completion := NewCompletionHandler(taskMgr, "", events)
	runner := NewRunner(taskMgr, agentMgr, completion)

	task := &Task{
		ID:     "T1",
		Title:  "Test Task",
		Status: StatusAssigned,
		Completion: &CompletionConfig{
			Signal: "DONE",
		},
	}
	taskMgr.addTask(task)

	err := runner.RunTask(context.Background(), "T1", "agent-1")
	if err != nil {
		t.Fatalf("RunTask() error: %v", err)
	}

	// Check task was completed
	updatedTask, _ := taskMgr.Get("T1")
	if updatedTask.Status != StatusComplete {
		t.Errorf("Status = %v, want complete", updatedTask.Status)
	}
}

func TestRunner_RunTask_RalphMode_Success(t *testing.T) {
	taskMgr := newMockTaskMgr()
	// First two calls don't have signal, third does
	agentMgr := newMockAgentExecutor([]string{"Working...", "Still working...", "DONE"}, nil)
	events := make(chan Event, 10)
	completion := NewCompletionHandler(taskMgr, "", events)
	runner := NewRunner(taskMgr, agentMgr, completion)
	runner.SetCooldown(1 * time.Millisecond) // Speed up tests

	task := &Task{
		ID:     "T1",
		Title:  "Test Task",
		Status: StatusAssigned,
		Completion: &CompletionConfig{
			Signal:        "DONE",
			MaxIterations: 5,
		},
	}
	taskMgr.addTask(task)

	err := runner.RunTask(context.Background(), "T1", "agent-1")
	if err != nil {
		t.Fatalf("RunTask() error: %v", err)
	}

	// Should have completed after 3 iterations
	if agentMgr.calls != 3 {
		t.Errorf("Agent called %d times, want 3", agentMgr.calls)
	}

	updatedTask, _ := taskMgr.Get("T1")
	if updatedTask.Status != StatusComplete {
		t.Errorf("Status = %v, want complete", updatedTask.Status)
	}
}

func TestRunner_RunTask_RalphMode_MaxIterations(t *testing.T) {
	taskMgr := newMockTaskMgr()
	// Never outputs the signal
	agentMgr := newMockAgentExecutor([]string{"Working...", "Working...", "Working..."}, nil)
	events := make(chan Event, 10)
	completion := NewCompletionHandler(taskMgr, "", events)
	runner := NewRunner(taskMgr, agentMgr, completion)
	runner.SetCooldown(1 * time.Millisecond)

	task := &Task{
		ID:     "T1",
		Title:  "Test Task",
		Status: StatusAssigned,
		Completion: &CompletionConfig{
			Signal:        "DONE",
			MaxIterations: 3,
		},
	}
	taskMgr.addTask(task)

	err := runner.RunTask(context.Background(), "T1", "agent-1")
	if err == nil {
		t.Error("Expected error for max iterations")
	}

	// Should be in review status
	updatedTask, _ := taskMgr.Get("T1")
	if updatedTask.Status != StatusReview {
		t.Errorf("Status = %v, want review", updatedTask.Status)
	}
}

func TestRunner_BuildPrompt(t *testing.T) {
	runner := NewRunner(nil, nil, nil)

	task := &Task{
		Title:   "Test Task",
		Content: "Do the thing.",
		Completion: &CompletionConfig{
			Verify: "npm test",
			Signal: "TASK_COMPLETE",
		},
	}

	prompt := runner.buildPrompt(task)

	// Check title
	if !containsString(prompt, "# Task: Test Task") {
		t.Error("Prompt missing title")
	}

	// Check content
	if !containsString(prompt, "Do the thing.") {
		t.Error("Prompt missing content")
	}

	// Check verify command
	if !containsString(prompt, "npm test") {
		t.Error("Prompt missing verify command")
	}

	// Check signal
	if !containsString(prompt, "TASK_COMPLETE") {
		t.Error("Prompt missing signal")
	}
}

func TestRunner_BuildPrompt_NoCompletion(t *testing.T) {
	runner := NewRunner(nil, nil, nil)

	task := &Task{
		Title:   "Simple Task",
		Content: "Just do it.",
	}

	prompt := runner.buildPrompt(task)

	// Should have title and content
	if !containsString(prompt, "# Task: Simple Task") {
		t.Error("Prompt missing title")
	}
	if !containsString(prompt, "Just do it.") {
		t.Error("Prompt missing content")
	}

	// Should NOT have completion instructions
	if containsString(prompt, "Completion Instructions") {
		t.Error("Prompt should not have completion instructions")
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr) >= 0))
}

func findSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
