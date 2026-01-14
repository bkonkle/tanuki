package task

import (
	"testing"
)

func TestPriority_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		priority Priority
		want     bool
	}{
		{"critical", PriorityCritical, true},
		{"high", PriorityHigh, true},
		{"medium", PriorityMedium, true},
		{"low", PriorityLow, true},
		{"empty", "", true},
		{"invalid", "urgent", false},
		{"invalid case", "HIGH", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.priority.IsValid(); got != tt.want {
				t.Errorf("Priority.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPriority_Order(t *testing.T) {
	tests := []struct {
		name     string
		priority Priority
		want     int
	}{
		{"critical", PriorityCritical, 0},
		{"high", PriorityHigh, 1},
		{"medium", PriorityMedium, 2},
		{"low", PriorityLow, 3},
		{"empty defaults to medium", "", 2},
		{"invalid defaults to medium", "unknown", 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.priority.Order(); got != tt.want {
				t.Errorf("Priority.Order() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStatus_IsValid(t *testing.T) {
	tests := []struct {
		name   string
		status Status
		want   bool
	}{
		{"pending", StatusPending, true},
		{"assigned", StatusAssigned, true},
		{"in_progress", StatusInProgress, true},
		{"review", StatusReview, true},
		{"complete", StatusComplete, true},
		{"failed", StatusFailed, true},
		{"blocked", StatusBlocked, true},
		{"empty", "", true},
		{"invalid", "done", false},
		{"invalid case", "PENDING", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.IsValid(); got != tt.want {
				t.Errorf("Status.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStatus_IsTerminal(t *testing.T) {
	tests := []struct {
		name   string
		status Status
		want   bool
	}{
		{"complete is terminal", StatusComplete, true},
		{"pending is not terminal", StatusPending, false},
		{"in_progress is not terminal", StatusInProgress, false},
		{"failed is not terminal", StatusFailed, false},
		{"blocked is not terminal", StatusBlocked, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.IsTerminal(); got != tt.want {
				t.Errorf("Status.IsTerminal() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTask_IsRalphMode(t *testing.T) {
	tests := []struct {
		name string
		task *Task
		want bool
	}{
		{
			name: "with verify",
			task: &Task{
				Completion: &CompletionConfig{
					Verify: "npm test",
				},
			},
			want: true,
		},
		{
			name: "with signal",
			task: &Task{
				Completion: &CompletionConfig{
					Signal: "DONE",
				},
			},
			want: true,
		},
		{
			name: "with both",
			task: &Task{
				Completion: &CompletionConfig{
					Verify: "npm test",
					Signal: "DONE",
				},
			},
			want: true,
		},
		{
			name: "no completion config",
			task: &Task{},
			want: false,
		},
		{
			name: "empty completion config",
			task: &Task{
				Completion: &CompletionConfig{},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.task.IsRalphMode(); got != tt.want {
				t.Errorf("Task.IsRalphMode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCompletionConfig_GetMaxIterations(t *testing.T) {
	tests := []struct {
		name   string
		config *CompletionConfig
		want   int
	}{
		{
			name:   "nil config returns default",
			config: nil,
			want:   30,
		},
		{
			name:   "zero value returns default",
			config: &CompletionConfig{MaxIterations: 0},
			want:   30,
		},
		{
			name:   "negative value returns default",
			config: &CompletionConfig{MaxIterations: -1},
			want:   30,
		},
		{
			name:   "custom value",
			config: &CompletionConfig{MaxIterations: 50},
			want:   50,
		},
		{
			name:   "small value",
			config: &CompletionConfig{MaxIterations: 5},
			want:   5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.config.GetMaxIterations(); got != tt.want {
				t.Errorf("CompletionConfig.GetMaxIterations() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidationError_Error(t *testing.T) {
	tests := []struct {
		name    string
		err     *ValidationError
		wantMsg string
	}{
		{
			name:    "with field",
			err:     &ValidationError{Field: "id", Message: "is required"},
			wantMsg: "task.id: is required",
		},
		{
			name:    "without field",
			err:     &ValidationError{Message: "task is nil"},
			wantMsg: "task is nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.wantMsg {
				t.Errorf("ValidationError.Error() = %q, want %q", got, tt.wantMsg)
			}
		})
	}
}
