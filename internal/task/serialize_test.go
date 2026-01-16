package task

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSerialize(t *testing.T) {
	tests := []struct {
		name        string
		task        *Task
		wantErr     bool
		wantContain []string
	}{
		{
			name: "minimal task",
			task: &Task{
				ID:         "TASK-001",
				Title:      "Test Task",
				Workstream: "backend",
				Content:    "# Test\n\nDo the thing.",
			},
			wantErr: false,
			wantContain: []string{
				"---",
				"id: TASK-001",
				"title: Test Task",
				"workstream: backend",
				"# Test",
				"Do the thing.",
			},
		},
		{
			name: "task with all fields",
			task: &Task{
				ID:         "TASK-002",
				Title:      "Full Task",
				Workstream: "qa",
				Priority:   PriorityHigh,
				Status:     StatusInProgress,
				DependsOn:  []string{"TASK-001"},
				AssignedTo: "agent-1",
				Tags:       []string{"test", "security"},
				Completion: &CompletionConfig{
					Verify:        "npm test",
					Signal:        "DONE",
					MaxIterations: 20,
				},
				Content: "Content here",
			},
			wantErr: false,
			wantContain: []string{
				"id: TASK-002",
				"priority: high",
				"status: in_progress",
				"depends_on:",
				"- TASK-001",
				"assigned_to: agent-1",
				"tags:",
				"- test",
				"- security",
				"completion:",
				"verify: npm test",
				"signal: DONE",
				"max_iterations: 20",
				"Content here",
			},
		},
		{
			name:    "nil task",
			task:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Serialize(tt.task)

			if (err != nil) != tt.wantErr {
				t.Errorf("Serialize() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			for _, want := range tt.wantContain {
				if !strings.Contains(got, want) {
					t.Errorf("Serialize() output missing %q\nGot:\n%s", want, got)
				}
			}
		})
	}
}

func TestWriteFile(t *testing.T) {
	dir := t.TempDir()

	t.Run("write and read back", func(t *testing.T) {
		task := &Task{
			ID:         "TASK-001",
			Title:      "Test Task",
			Workstream: "backend",
			Priority:   PriorityHigh,
			Status:     StatusPending,
			Completion: &CompletionConfig{
				Verify: "npm test",
			},
			Content:  "# Test Task\n\nDo the thing.",
			FilePath: filepath.Join(dir, "task-001.md"),
		}

		// Write file
		if err := WriteFile(task); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}

		// Verify file exists
		if _, err := os.Stat(task.FilePath); os.IsNotExist(err) {
			t.Fatal("WriteFile() did not create file")
		}

		// Read back and parse
		parsed, err := ParseFile(task.FilePath)
		if err != nil {
			t.Fatalf("ParseFile() error = %v", err)
		}

		// Compare fields
		if parsed.ID != task.ID {
			t.Errorf("ID = %q, want %q", parsed.ID, task.ID)
		}
		if parsed.Title != task.Title {
			t.Errorf("Title = %q, want %q", parsed.Title, task.Title)
		}
		if parsed.Workstream != task.Workstream {
			t.Errorf("Workstream = %q, want %q", parsed.Workstream, task.Workstream)
		}
		if parsed.Priority != task.Priority {
			t.Errorf("Priority = %q, want %q", parsed.Priority, task.Priority)
		}
		if parsed.Status != task.Status {
			t.Errorf("Status = %q, want %q", parsed.Status, task.Status)
		}
		if parsed.Content != task.Content {
			t.Errorf("Content = %q, want %q", parsed.Content, task.Content)
		}
		if parsed.Completion == nil {
			t.Error("Completion is nil")
		} else if parsed.Completion.Verify != task.Completion.Verify {
			t.Errorf("Completion.Verify = %q, want %q", parsed.Completion.Verify, task.Completion.Verify)
		}
	})

	t.Run("nil task", func(t *testing.T) {
		if err := WriteFile(nil); err == nil {
			t.Error("WriteFile(nil) should return error")
		}
	})

	t.Run("missing file path", func(t *testing.T) {
		task := &Task{
			ID:         "TASK-001",
			Title:      "Test",
			Workstream: "backend",
		}
		if err := WriteFile(task); err == nil {
			t.Error("WriteFile() with empty FilePath should return error")
		}
	})
}

func TestWriteFile_UpdateStatus(t *testing.T) {
	dir := t.TempDir()

	// Create initial task
	task := &Task{
		ID:         "TASK-001",
		Title:      "Test Task",
		Workstream: "backend",
		Priority:   PriorityHigh,
		Status:     StatusPending,
		Content:    "Content",
		FilePath:   filepath.Join(dir, "task-001.md"),
	}

	// Write initial file
	if err := WriteFile(task); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// Update status
	task.Status = StatusInProgress
	task.AssignedTo = "agent-1"

	// Write updated file
	if err := WriteFile(task); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// Read back
	parsed, err := ParseFile(task.FilePath)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}

	// Verify status updated
	if parsed.Status != StatusInProgress {
		t.Errorf("Status = %q, want %q", parsed.Status, StatusInProgress)
	}
	if parsed.AssignedTo != "agent-1" {
		t.Errorf("AssignedTo = %q, want %q", parsed.AssignedTo, "agent-1")
	}
}

func TestSerialize_RoundTrip(t *testing.T) {
	original := &Task{
		ID:         "TASK-001",
		Title:      "Test Task",
		Workstream: "backend",
		Priority:   PriorityHigh,
		Status:     StatusInProgress,
		DependsOn:  []string{"TASK-000"},
		AssignedTo: "agent-1",
		Tags:       []string{"test"},
		Completion: &CompletionConfig{
			Verify:        "npm test",
			Signal:        "DONE",
			MaxIterations: 15,
		},
		Content: "# Task Content\n\nDo the thing.",
	}

	// Serialize
	serialized, err := Serialize(original)
	if err != nil {
		t.Fatalf("Serialize() error = %v", err)
	}

	// Parse back
	parsed, err := Parse(serialized, "test.md")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// Compare
	if parsed.ID != original.ID {
		t.Errorf("ID = %q, want %q", parsed.ID, original.ID)
	}
	if parsed.Title != original.Title {
		t.Errorf("Title = %q, want %q", parsed.Title, original.Title)
	}
	if parsed.Workstream != original.Workstream {
		t.Errorf("Workstream = %q, want %q", parsed.Workstream, original.Workstream)
	}
	if parsed.Priority != original.Priority {
		t.Errorf("Priority = %q, want %q", parsed.Priority, original.Priority)
	}
	if parsed.Status != original.Status {
		t.Errorf("Status = %q, want %q", parsed.Status, original.Status)
	}
	if parsed.AssignedTo != original.AssignedTo {
		t.Errorf("AssignedTo = %q, want %q", parsed.AssignedTo, original.AssignedTo)
	}
	if len(parsed.DependsOn) != len(original.DependsOn) {
		t.Errorf("DependsOn length = %d, want %d", len(parsed.DependsOn), len(original.DependsOn))
	}
	if len(parsed.Tags) != len(original.Tags) {
		t.Errorf("Tags length = %d, want %d", len(parsed.Tags), len(original.Tags))
	}
	if parsed.Completion.MaxIterations != original.Completion.MaxIterations {
		t.Errorf("MaxIterations = %d, want %d", parsed.Completion.MaxIterations, original.Completion.MaxIterations)
	}
	if parsed.Content != original.Content {
		t.Errorf("Content = %q, want %q", parsed.Content, original.Content)
	}
}
