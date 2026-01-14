package task

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestStatusTracker_RecordChange(t *testing.T) {
	tracker := NewStatusTracker()

	err := tracker.RecordChange("T1", StatusPending, StatusAssigned, "agent-1", "")
	if err != nil {
		t.Fatalf("RecordChange() error: %v", err)
	}

	history := tracker.GetHistory("T1")
	if len(history) != 1 {
		t.Errorf("History length = %d, want 1", len(history))
	}

	if history[0].From != StatusPending || history[0].To != StatusAssigned {
		t.Errorf("Change = %v → %v, want pending → assigned", history[0].From, history[0].To)
	}

	if history[0].AgentName != "agent-1" {
		t.Errorf("AgentName = %s, want agent-1", history[0].AgentName)
	}
}

func TestStatusTracker_InvalidTransition(t *testing.T) {
	tracker := NewStatusTracker()

	// Can't go from pending to complete directly
	err := tracker.RecordChange("T1", StatusPending, StatusComplete, "", "")
	if err == nil {
		t.Error("Should reject invalid transition pending → complete")
	}
}

func TestStatusTracker_AllowEmptyFrom(t *testing.T) {
	tracker := NewStatusTracker()

	// Should allow empty "from" for initial state recording
	err := tracker.RecordChange("T1", "", StatusPending, "", "")
	if err != nil {
		t.Errorf("Should allow empty from: %v", err)
	}
}

func TestStatusTracker_Listener(t *testing.T) {
	tracker := NewStatusTracker()

	var received []StatusChange
	tracker.AddListener(func(change StatusChange) {
		received = append(received, change)
	})

	tracker.RecordChange("T1", StatusPending, StatusAssigned, "agent-1", "")
	tracker.RecordChange("T1", StatusAssigned, StatusInProgress, "agent-1", "")

	if len(received) != 2 {
		t.Errorf("Listener received %d changes, want 2", len(received))
	}
}

func TestStatusTracker_ListenerPanicRecovery(t *testing.T) {
	tracker := NewStatusTracker()

	// Add a listener that panics
	tracker.AddListener(func(change StatusChange) {
		panic("test panic")
	})

	// Should not panic
	err := tracker.RecordChange("T1", StatusPending, StatusAssigned, "agent-1", "")
	if err != nil {
		t.Errorf("RecordChange() should not error: %v", err)
	}
}

func TestStatusTracker_GetHistory(t *testing.T) {
	tracker := NewStatusTracker()

	tracker.RecordChange("T1", StatusPending, StatusAssigned, "agent-1", "")
	tracker.RecordChange("T1", StatusAssigned, StatusInProgress, "agent-1", "")
	tracker.RecordChange("T1", StatusInProgress, StatusComplete, "agent-1", "done")

	history := tracker.GetHistory("T1")
	if len(history) != 3 {
		t.Errorf("History length = %d, want 3", len(history))
	}

	// Verify order
	if history[0].To != StatusAssigned {
		t.Errorf("First change should be to assigned")
	}
	if history[2].To != StatusComplete {
		t.Errorf("Last change should be to complete")
	}
}

func TestStatusTracker_GetHistoryEmpty(t *testing.T) {
	tracker := NewStatusTracker()

	history := tracker.GetHistory("nonexistent")
	if len(history) != 0 {
		t.Errorf("History for nonexistent task should be empty")
	}
}

func TestStatusTracker_GetLastChange(t *testing.T) {
	tracker := NewStatusTracker()

	tracker.RecordChange("T1", StatusPending, StatusAssigned, "agent-1", "")
	tracker.RecordChange("T1", StatusAssigned, StatusInProgress, "agent-1", "")

	last := tracker.GetLastChange("T1")
	if last == nil {
		t.Fatal("GetLastChange() returned nil")
	}

	if last.To != StatusInProgress {
		t.Errorf("Last change to = %s, want in_progress", last.To)
	}
}

func TestStatusTracker_GetLastChangeEmpty(t *testing.T) {
	tracker := NewStatusTracker()

	last := tracker.GetLastChange("nonexistent")
	if last != nil {
		t.Error("GetLastChange() for nonexistent task should return nil")
	}
}

func TestValidTransitions(t *testing.T) {
	tests := []struct {
		from  Status
		to    Status
		valid bool
	}{
		{StatusPending, StatusAssigned, true},
		{StatusPending, StatusBlocked, true},
		{StatusPending, StatusComplete, false},
		{StatusAssigned, StatusInProgress, true},
		{StatusAssigned, StatusPending, true},
		{StatusInProgress, StatusComplete, true},
		{StatusInProgress, StatusFailed, true},
		{StatusInProgress, StatusReview, true},
		{StatusInProgress, StatusPending, true},
		{StatusReview, StatusComplete, true},
		{StatusReview, StatusInProgress, true},
		{StatusReview, StatusFailed, true},
		{StatusComplete, StatusPending, false}, // Terminal
		{StatusFailed, StatusPending, true},    // Can retry
		{StatusFailed, StatusInProgress, true},
		{StatusBlocked, StatusPending, true},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s→%s", tt.from, tt.to), func(t *testing.T) {
			valid := CanTransition(tt.from, tt.to)
			if valid != tt.valid {
				t.Errorf("CanTransition(%s, %s) = %v, want %v", tt.from, tt.to, valid, tt.valid)
			}
		})
	}
}

func TestValidTransitions_UnknownStatus(t *testing.T) {
	err := validateTransition(Status("unknown"), StatusPending)
	if err == nil {
		t.Error("Should error for unknown status")
	}
}

func TestGetValidTransitions(t *testing.T) {
	transitions := GetValidTransitions(StatusPending)
	if len(transitions) != 2 {
		t.Errorf("GetValidTransitions(pending) = %d transitions, want 2", len(transitions))
	}

	// Complete has no valid transitions
	transitions = GetValidTransitions(StatusComplete)
	if len(transitions) != 0 {
		t.Errorf("GetValidTransitions(complete) = %d transitions, want 0", len(transitions))
	}
}

func TestStatusTracker_GetTasksByStatus(t *testing.T) {
	tracker := NewStatusTracker()

	tasks := []*Task{
		{ID: "T1", Status: StatusPending},
		{ID: "T2", Status: StatusInProgress},
		{ID: "T3", Status: StatusPending},
		{ID: "T4", Status: StatusComplete},
	}

	pending := tracker.GetTasksByStatus(tasks, StatusPending)
	if len(pending) != 2 {
		t.Errorf("GetTasksByStatus(pending) = %d, want 2", len(pending))
	}

	complete := tracker.GetTasksByStatus(tasks, StatusComplete)
	if len(complete) != 1 {
		t.Errorf("GetTasksByStatus(complete) = %d, want 1", len(complete))
	}
}

func TestStatusTracker_GetRecentChanges(t *testing.T) {
	tracker := NewStatusTracker()

	tracker.RecordChange("T1", StatusPending, StatusAssigned, "agent-1", "")
	time.Sleep(10 * time.Millisecond)
	tracker.RecordChange("T2", StatusPending, StatusAssigned, "agent-2", "")

	recent := tracker.GetRecentChanges(1 * time.Hour)
	if len(recent) != 2 {
		t.Errorf("GetRecentChanges() = %d, want 2", len(recent))
	}

	// Very short duration should return none
	recent = tracker.GetRecentChanges(1 * time.Nanosecond)
	if len(recent) != 0 {
		t.Errorf("GetRecentChanges(1ns) = %d, want 0", len(recent))
	}
}

func TestStatusTracker_GetCompletedToday(t *testing.T) {
	tracker := NewStatusTracker()

	// Record a completion
	tracker.RecordChange("T1", StatusInProgress, StatusComplete, "agent-1", "")
	tracker.RecordChange("T2", StatusPending, StatusAssigned, "agent-2", "")

	completed := tracker.GetCompletedToday()
	if len(completed) != 1 {
		t.Errorf("GetCompletedToday() = %d, want 1", len(completed))
	}

	if completed[0] != "T1" {
		t.Errorf("GetCompletedToday()[0] = %s, want T1", completed[0])
	}
}

func TestStatusTracker_GetTaskDuration(t *testing.T) {
	tracker := NewStatusTracker()

	// Simulate task lifecycle
	tracker.RecordChange("T1", "", StatusPending, "", "")
	time.Sleep(10 * time.Millisecond)
	tracker.RecordChange("T1", StatusPending, StatusAssigned, "agent-1", "")
	time.Sleep(10 * time.Millisecond)
	tracker.RecordChange("T1", StatusAssigned, StatusInProgress, "agent-1", "")
	time.Sleep(50 * time.Millisecond)
	tracker.RecordChange("T1", StatusInProgress, StatusComplete, "agent-1", "")

	durations := tracker.GetTaskDuration("T1")

	// InProgress should be ~50ms
	if durations[StatusInProgress] < 40*time.Millisecond {
		t.Errorf("InProgress duration = %v, want ~50ms", durations[StatusInProgress])
	}
}

func TestStatusTracker_GetTotalWorkTime(t *testing.T) {
	tracker := NewStatusTracker()

	tracker.RecordChange("T1", StatusAssigned, StatusInProgress, "agent-1", "")
	time.Sleep(30 * time.Millisecond)
	tracker.RecordChange("T1", StatusInProgress, StatusComplete, "agent-1", "")

	workTime := tracker.GetTotalWorkTime("T1")
	if workTime < 25*time.Millisecond {
		t.Errorf("GetTotalWorkTime() = %v, want ~30ms", workTime)
	}
}

func TestStatusTracker_GetAverageCompletionTime(t *testing.T) {
	tracker := NewStatusTracker()

	// Task 1: 50ms
	tracker.RecordChange("T1", StatusPending, StatusAssigned, "agent-1", "")
	time.Sleep(50 * time.Millisecond)
	tracker.RecordChange("T1", StatusAssigned, StatusInProgress, "agent-1", "")
	tracker.RecordChange("T1", StatusInProgress, StatusComplete, "agent-1", "")

	// Task 2: 50ms
	tracker.RecordChange("T2", StatusPending, StatusAssigned, "agent-2", "")
	time.Sleep(50 * time.Millisecond)
	tracker.RecordChange("T2", StatusAssigned, StatusInProgress, "agent-2", "")
	tracker.RecordChange("T2", StatusInProgress, StatusComplete, "agent-2", "")

	avg := tracker.GetAverageCompletionTime()
	if avg < 40*time.Millisecond || avg > 150*time.Millisecond {
		t.Errorf("GetAverageCompletionTime() = %v, expected ~50ms", avg)
	}
}

func TestStatusTracker_GetAverageCompletionTimeNoCompletions(t *testing.T) {
	tracker := NewStatusTracker()

	// No completions
	tracker.RecordChange("T1", StatusPending, StatusAssigned, "agent-1", "")

	avg := tracker.GetAverageCompletionTime()
	if avg != 0 {
		t.Errorf("GetAverageCompletionTime() = %v, want 0", avg)
	}
}

func TestStatusTracker_ConcurrentAccess(t *testing.T) {
	tracker := NewStatusTracker()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)

		go func(id int) {
			defer wg.Done()
			taskID := fmt.Sprintf("T%d", id)
			tracker.RecordChange(taskID, StatusPending, StatusAssigned, "agent", "")
			tracker.RecordChange(taskID, StatusAssigned, StatusInProgress, "agent", "")
		}(i)

		go func(id int) {
			defer wg.Done()
			taskID := fmt.Sprintf("T%d", id)
			tracker.GetHistory(taskID)
			tracker.GetLastChange(taskID)
			tracker.GetRecentChanges(1 * time.Hour)
		}(i)
	}

	wg.Wait()
}

func TestStatusTracker_Clear(t *testing.T) {
	tracker := NewStatusTracker()

	tracker.RecordChange("T1", StatusPending, StatusAssigned, "agent-1", "")
	tracker.RecordChange("T2", StatusPending, StatusAssigned, "agent-2", "")

	tracker.Clear()

	if tracker.TaskCount() != 0 {
		t.Errorf("TaskCount() after Clear() = %d, want 0", tracker.TaskCount())
	}
}

func TestStatusTracker_TaskCount(t *testing.T) {
	tracker := NewStatusTracker()

	tracker.RecordChange("T1", StatusPending, StatusAssigned, "agent-1", "")
	tracker.RecordChange("T2", StatusPending, StatusAssigned, "agent-2", "")
	tracker.RecordChange("T1", StatusAssigned, StatusInProgress, "agent-1", "") // Same task

	if tracker.TaskCount() != 2 {
		t.Errorf("TaskCount() = %d, want 2", tracker.TaskCount())
	}
}

func TestNewEventFromChange(t *testing.T) {
	change := StatusChange{
		TaskID:    "T1",
		From:      StatusPending,
		To:        StatusAssigned,
		Timestamp: time.Now(),
		AgentName: "agent-1",
		Message:   "test",
	}

	event := NewEventFromChange(change, "Test Task")

	if event.Type != EventTaskAssigned {
		t.Errorf("Event.Type = %s, want %s", event.Type, EventTaskAssigned)
	}

	if event.TaskID != "T1" {
		t.Errorf("Event.TaskID = %s, want T1", event.TaskID)
	}

	if event.AgentName != "agent-1" {
		t.Errorf("Event.AgentName = %s, want agent-1", event.AgentName)
	}
}
