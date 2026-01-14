package task

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    *Task
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid task with completion",
			content: `---
id: TASK-001
title: Test Task
role: backend
priority: high
completion:
  verify: "npm test"
---

# Test Task

Do the thing.
`,
			want: &Task{
				ID:       "TASK-001",
				Title:    "Test Task",
				Role:     "backend",
				Priority: PriorityHigh,
				Status:   StatusPending, // Default
				Completion: &CompletionConfig{
					Verify: "npm test",
				},
				Content:  "# Test Task\n\nDo the thing.",
				FilePath: "test.md",
			},
			wantErr: false,
		},
		{
			name: "valid task with signal",
			content: `---
id: TASK-002
title: Another Task
role: frontend
completion:
  signal: "DONE"
  max_iterations: 20
---

Content here
`,
			want: &Task{
				ID:       "TASK-002",
				Title:    "Another Task",
				Role:     "frontend",
				Priority: PriorityMedium, // Default
				Status:   StatusPending,
				Completion: &CompletionConfig{
					Signal:        "DONE",
					MaxIterations: 20,
				},
				Content:  "Content here",
				FilePath: "test.md",
			},
			wantErr: false,
		},
		{
			name: "minimal valid task",
			content: `---
id: TASK-001
title: Minimal
role: backend
---

Do it.
`,
			want: &Task{
				ID:       "TASK-001",
				Title:    "Minimal",
				Role:     "backend",
				Priority: PriorityMedium, // Default
				Status:   StatusPending,  // Default
				Content:  "Do it.",
			},
			wantErr: false,
		},
		{
			name: "task with all fields",
			content: `---
id: TASK-003
title: Full Task
role: qa
priority: critical
status: in_progress
depends_on:
  - TASK-001
  - TASK-002
assigned_to: agent-1
tags:
  - testing
  - security
completion:
  verify: "npm run lint"
  signal: "LINT_DONE"
---

# Full Task

This has all fields populated.
`,
			want: &Task{
				ID:         "TASK-003",
				Title:      "Full Task",
				Role:       "qa",
				Priority:   PriorityCritical,
				Status:     StatusInProgress,
				DependsOn:  []string{"TASK-001", "TASK-002"},
				AssignedTo: "agent-1",
				Tags:       []string{"testing", "security"},
				Completion: &CompletionConfig{
					Verify: "npm run lint",
					Signal: "LINT_DONE",
				},
				Content: "# Full Task\n\nThis has all fields populated.",
			},
			wantErr: false,
		},
		{
			name: "missing front matter delimiters",
			content: `id: TASK-001
title: Test Task
role: backend

Content
`,
			wantErr: true,
			errMsg:  "missing front matter delimiters",
		},
		{
			name: "missing id",
			content: `---
title: Test Task
role: backend
---

Content
`,
			wantErr: true,
			errMsg:  "id",
		},
		{
			name: "missing title",
			content: `---
id: TASK-001
role: backend
---

Content
`,
			wantErr: true,
			errMsg:  "title",
		},
		{
			name: "missing role",
			content: `---
id: TASK-001
title: Test Task
---

Content
`,
			wantErr: true,
			errMsg:  "role",
		},
		{
			name: "invalid priority",
			content: `---
id: TASK-001
title: Test Task
role: backend
priority: urgent
---

Content
`,
			wantErr: true,
			errMsg:  "priority",
		},
		{
			name: "invalid status",
			content: `---
id: TASK-001
title: Test Task
role: backend
status: done
---

Content
`,
			wantErr: true,
			errMsg:  "status",
		},
		{
			name: "empty completion config",
			content: `---
id: TASK-001
title: Test Task
role: backend
completion: {}
---

Content
`,
			wantErr: true,
			errMsg:  "completion",
		},
		{
			name: "invalid yaml",
			content: `---
id: TASK-001
title: Test Task
role: backend
  invalid:
yaml: structure
---

Content
`,
			wantErr: true,
			errMsg:  "parse front matter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.content, "test.md")

			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Parse() error = %v, want containing %q", err, tt.errMsg)
				}
				return
			}

			// Compare fields
			if got.ID != tt.want.ID {
				t.Errorf("ID = %q, want %q", got.ID, tt.want.ID)
			}
			if got.Title != tt.want.Title {
				t.Errorf("Title = %q, want %q", got.Title, tt.want.Title)
			}
			if got.Role != tt.want.Role {
				t.Errorf("Role = %q, want %q", got.Role, tt.want.Role)
			}
			if got.Priority != tt.want.Priority {
				t.Errorf("Priority = %q, want %q", got.Priority, tt.want.Priority)
			}
			if got.Status != tt.want.Status {
				t.Errorf("Status = %q, want %q", got.Status, tt.want.Status)
			}
			if got.Content != tt.want.Content {
				t.Errorf("Content = %q, want %q", got.Content, tt.want.Content)
			}
			if tt.want.Completion != nil {
				if got.Completion == nil {
					t.Error("Completion is nil, want non-nil")
				} else {
					if got.Completion.Verify != tt.want.Completion.Verify {
						t.Errorf("Completion.Verify = %q, want %q", got.Completion.Verify, tt.want.Completion.Verify)
					}
					if got.Completion.Signal != tt.want.Completion.Signal {
						t.Errorf("Completion.Signal = %q, want %q", got.Completion.Signal, tt.want.Completion.Signal)
					}
				}
			}
		})
	}
}

func TestParseFile(t *testing.T) {
	// Create temp directory
	dir := t.TempDir()

	// Create a valid task file
	validContent := `---
id: TASK-001
title: Test Task
role: backend
priority: high
---

# Test Task

Do the thing.
`
	validPath := filepath.Join(dir, "valid-task.md")
	if err := os.WriteFile(validPath, []byte(validContent), 0600); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test parsing valid file
	t.Run("valid file", func(t *testing.T) {
		task, err := ParseFile(validPath)
		if err != nil {
			t.Fatalf("ParseFile() error = %v", err)
		}
		if task.ID != "TASK-001" {
			t.Errorf("ID = %q, want TASK-001", task.ID)
		}
		if task.FilePath != validPath {
			t.Errorf("FilePath = %q, want %q", task.FilePath, validPath)
		}
	})

	// Test parsing non-existent file
	t.Run("non-existent file", func(t *testing.T) {
		_, err := ParseFile(filepath.Join(dir, "not-found.md"))
		if err == nil {
			t.Error("ParseFile() expected error for non-existent file")
		}
		if !strings.Contains(err.Error(), "read file") {
			t.Errorf("Error = %v, want containing 'read file'", err)
		}
	})
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		task    *Task
		wantErr bool
		errMsg  string
	}{
		{
			name:    "nil task",
			task:    nil,
			wantErr: true,
			errMsg:  "task is nil",
		},
		{
			name:    "empty id",
			task:    &Task{Title: "Test", Role: "backend"},
			wantErr: true,
			errMsg:  "id",
		},
		{
			name:    "empty title",
			task:    &Task{ID: "T1", Role: "backend"},
			wantErr: true,
			errMsg:  "title",
		},
		{
			name:    "empty role",
			task:    &Task{ID: "T1", Title: "Test"},
			wantErr: true,
			errMsg:  "role",
		},
		{
			name:    "invalid priority",
			task:    &Task{ID: "T1", Title: "Test", Role: "backend", Priority: "urgent"},
			wantErr: true,
			errMsg:  "priority",
		},
		{
			name:    "invalid status",
			task:    &Task{ID: "T1", Title: "Test", Role: "backend", Status: "done"},
			wantErr: true,
			errMsg:  "status",
		},
		{
			name: "empty completion config",
			task: &Task{
				ID:         "T1",
				Title:      "Test",
				Role:       "backend",
				Completion: &CompletionConfig{},
			},
			wantErr: true,
			errMsg:  "completion",
		},
		{
			name: "valid minimal task",
			task: &Task{
				ID:    "T1",
				Title: "Test",
				Role:  "backend",
			},
			wantErr: false,
		},
		{
			name: "valid task with completion",
			task: &Task{
				ID:    "T1",
				Title: "Test",
				Role:  "backend",
				Completion: &CompletionConfig{
					Verify: "npm test",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.task)

			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Validate() error = %v, want containing %q", err, tt.errMsg)
				}
			}
		})
	}
}

func TestValidate_DefaultsApplied(t *testing.T) {
	task := &Task{
		ID:    "T1",
		Title: "Test",
		Role:  "backend",
	}

	if err := Validate(task); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	// Check defaults were applied
	if task.Priority != PriorityMedium {
		t.Errorf("Priority = %q, want %q", task.Priority, PriorityMedium)
	}
	if task.Status != StatusPending {
		t.Errorf("Status = %q, want %q", task.Status, StatusPending)
	}
}
