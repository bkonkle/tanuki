package task

import (
	"context"
	"testing"
	"time"
)

// mockTaskManager implements TaskManagerInterface for testing.
type mockTaskMgr struct {
	tasks map[string]*Task
}

func newMockTaskMgr() *mockTaskMgr {
	return &mockTaskMgr{
		tasks: make(map[string]*Task),
	}
}

func (m *mockTaskMgr) addTask(t *Task) {
	m.tasks[t.ID] = t
}

func (m *mockTaskMgr) Get(id string) (*Task, error) {
	t, ok := m.tasks[id]
	if !ok {
		return nil, &ValidationError{Message: "not found"}
	}
	return t, nil
}

func (m *mockTaskMgr) UpdateStatus(id string, status Status) error {
	t, ok := m.tasks[id]
	if !ok {
		return &ValidationError{Message: "not found"}
	}
	t.Status = status
	return nil
}

func (m *mockTaskMgr) Unassign(id string) error {
	t, ok := m.tasks[id]
	if !ok {
		return &ValidationError{Message: "not found"}
	}
	t.AssignedTo = ""
	return nil
}

func TestCompletionHandler_HandleAgentOutput(t *testing.T) {
	taskMgr := newMockTaskMgr()
	events := make(chan Event, 10)
	handler := NewCompletionHandler(taskMgr, "", events)

	tests := []struct {
		name       string
		task       *Task
		output     string
		wantStatus Status
	}{
		{
			name: "complete with signal",
			task: &Task{
				ID:    "T1",
				Title: "Test",
				Completion: &CompletionConfig{
					Signal: "DONE",
				},
			},
			output:     "Working... DONE",
			wantStatus: StatusComplete,
		},
		{
			name: "in progress - signal not found",
			task: &Task{
				ID:    "T2",
				Title: "Test",
				Completion: &CompletionConfig{
					Signal: "DONE",
				},
			},
			output:     "Still working...",
			wantStatus: StatusInProgress,
		},
		{
			name: "review - no criteria",
			task: &Task{
				ID:    "T3",
				Title: "Test",
			},
			output:     "Some output",
			wantStatus: StatusReview,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskMgr.addTask(tt.task)

			err := handler.HandleAgentOutput(context.Background(), tt.task.ID, "agent-1", tt.output)
			if err != nil {
				t.Errorf("HandleAgentOutput() error: %v", err)
			}

			// Check task status was updated
			task, err := taskMgr.Get(tt.task.ID)
			if err != nil {
				t.Fatalf("Get() error: %v", err)
			}
			if task.Status != tt.wantStatus {
				t.Errorf("Status = %v, want %v", task.Status, tt.wantStatus)
			}
		})
	}
}

func TestCompletionHandler_EmitsEvents(t *testing.T) {
	taskMgr := newMockTaskMgr()
	events := make(chan Event, 10)
	handler := NewCompletionHandler(taskMgr, "", events)

	task := &Task{
		ID:    "T1",
		Title: "Test",
		Completion: &CompletionConfig{
			Signal: "DONE",
		},
	}
	taskMgr.addTask(task)

	err := handler.HandleAgentOutput(context.Background(), "T1", "agent-1", "DONE")
	if err != nil {
		t.Fatalf("HandleAgentOutput() error: %v", err)
	}

	// Check event was emitted
	select {
	case event := <-events:
		if event.Type != EventTaskCompleted {
			t.Errorf("Event type = %s, want %s", event.Type, EventTaskCompleted)
		}
		if event.TaskID != "T1" {
			t.Errorf("Event TaskID = %s, want T1", event.TaskID)
		}
		if event.AgentName != "agent-1" {
			t.Errorf("Event AgentName = %s, want agent-1", event.AgentName)
		}
	case <-time.After(time.Second):
		t.Error("Expected event not received")
	}
}

func TestStatusToEventType(t *testing.T) {
	tests := []struct {
		status Status
		want   string
	}{
		{StatusAssigned, EventTaskAssigned},
		{StatusInProgress, EventTaskStarted},
		{StatusComplete, EventTaskCompleted},
		{StatusFailed, EventTaskFailed},
		{StatusBlocked, EventTaskBlocked},
		{StatusPending, EventTaskUnblocked},
		{StatusReview, "task.status_changed"},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			got := statusToEventType(tt.status)
			if got != tt.want {
				t.Errorf("statusToEventType(%s) = %s, want %s", tt.status, got, tt.want)
			}
		})
	}
}
