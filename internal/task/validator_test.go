package task

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestValidator_ValidateWithSignal(t *testing.T) {
	v := NewValidator("")

	task := &Task{
		ID:    "T1",
		Title: "Test",
		Completion: &CompletionConfig{
			Signal: "TASK_DONE",
		},
	}

	tests := []struct {
		name       string
		output     string
		wantStatus Status
		wantSignal bool
	}{
		{
			name:       "signal found",
			output:     "Working... TASK_DONE",
			wantStatus: StatusComplete,
			wantSignal: true,
		},
		{
			name:       "signal not found",
			output:     "Still working...",
			wantStatus: StatusInProgress,
			wantSignal: false,
		},
		{
			name:       "signal in middle of output",
			output:     "Line 1\nLine 2\nTASK_DONE\nLine 4",
			wantStatus: StatusComplete,
			wantSignal: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := v.Validate(context.Background(), task, tt.output)
			if result.Status != tt.wantStatus {
				t.Errorf("Status = %v, want %v", result.Status, tt.wantStatus)
			}
			if result.SignalFound != tt.wantSignal {
				t.Errorf("SignalFound = %v, want %v", result.SignalFound, tt.wantSignal)
			}
		})
	}
}

func TestValidator_ValidateWithVerify(t *testing.T) {
	v := NewValidator("")
	v.SetTimeout(5 * time.Second)

	tests := []struct {
		name       string
		verify     string
		wantStatus Status
		wantPassed bool
	}{
		{
			name:       "verify passes",
			verify:     "exit 0",
			wantStatus: StatusComplete,
			wantPassed: true,
		},
		{
			name:       "verify fails",
			verify:     "exit 1",
			wantStatus: StatusReview,
			wantPassed: false,
		},
		{
			name:       "echo test passes",
			verify:     "echo 'test'",
			wantStatus: StatusComplete,
			wantPassed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &Task{
				ID:    "T1",
				Title: "Test",
				Completion: &CompletionConfig{
					Verify: tt.verify,
				},
			}

			result := v.Validate(context.Background(), task, "")
			if result.Status != tt.wantStatus {
				t.Errorf("Status = %v, want %v", result.Status, tt.wantStatus)
			}
			if result.VerifyPassed != tt.wantPassed {
				t.Errorf("VerifyPassed = %v, want %v", result.VerifyPassed, tt.wantPassed)
			}
		})
	}
}

func TestValidator_ValidateWithBothCriteria(t *testing.T) {
	v := NewValidator("")

	task := &Task{
		ID:    "T1",
		Title: "Test",
		Completion: &CompletionConfig{
			Signal: "DONE",
			Verify: "exit 0",
		},
	}

	tests := []struct {
		name       string
		output     string
		wantStatus Status
	}{
		{
			name:       "both pass",
			output:     "DONE",
			wantStatus: StatusComplete,
		},
		{
			name:       "signal missing",
			output:     "Still working",
			wantStatus: StatusInProgress,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := v.Validate(context.Background(), task, tt.output)
			if result.Status != tt.wantStatus {
				t.Errorf("Status = %v, want %v", result.Status, tt.wantStatus)
			}
		})
	}
}

func TestValidator_NoCompletionCriteria(t *testing.T) {
	v := NewValidator("")

	task := &Task{
		ID:    "T1",
		Title: "Test",
		// No Completion config
	}

	result := v.Validate(context.Background(), task, "any output")
	if result.Status != StatusReview {
		t.Errorf("Status = %v, want review", result.Status)
	}
}

func TestValidator_VerifyTimeout(t *testing.T) {
	v := NewValidator("")
	v.SetTimeout(100 * time.Millisecond)

	task := &Task{
		ID:    "T1",
		Title: "Test",
		Completion: &CompletionConfig{
			Verify: "sleep 10",
		},
	}

	result := v.Validate(context.Background(), task, "")
	if result.Status != StatusFailed {
		t.Errorf("Status = %v, want failed", result.Status)
	}
	if result.VerifyError == nil {
		t.Error("VerifyError should not be nil for timeout")
	}
	if !strings.Contains(result.VerifyError.Error(), "timed out") {
		t.Errorf("Error message should mention timeout: %v", result.VerifyError)
	}
}

func TestValidator_VerifyWithWorkdir(t *testing.T) {
	tempDir := t.TempDir()
	v := NewValidator(tempDir)

	task := &Task{
		ID:    "T1",
		Title: "Test",
		Completion: &CompletionConfig{
			Verify: "pwd",
		},
	}

	result := v.Validate(context.Background(), task, "")
	if result.Status != StatusComplete {
		t.Errorf("Status = %v, want complete", result.Status)
	}
	if !strings.Contains(result.VerifyOutput, tempDir) {
		t.Errorf("Verify should run in workdir: %s", result.VerifyOutput)
	}
}
