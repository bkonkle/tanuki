package task

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// ParseFile reads and parses a task file from disk.
func ParseFile(path string) (*Task, error) {
	content, err := os.ReadFile(path) // #nosec G304 - path is from internal iteration over known task directories
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	return Parse(string(content), path)
}

// Parse parses task content with YAML front matter.
// The format is: ---\nyaml\n---\nmarkdown
func Parse(content string, filePath string) (*Task, error) {
	// Split front matter and content
	// Format: ---\nyaml\n---\nmarkdown
	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid task file format: missing front matter delimiters")
	}

	// parts[0] is empty (before first ---)
	// parts[1] is the YAML front matter
	// parts[2] is the markdown content

	// Parse YAML front matter
	var task Task
	if err := yaml.Unmarshal([]byte(parts[1]), &task); err != nil {
		return nil, fmt.Errorf("parse front matter: %w", err)
	}

	// Set derived fields
	task.FilePath = filePath
	task.Content = strings.TrimSpace(parts[2])

	// Validate
	if err := Validate(&task); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	return &task, nil
}

// Validate checks required fields and values.
func Validate(t *Task) error {
	if t == nil {
		return &ValidationError{
			Message: "task is nil",
		}
	}

	if t.ID == "" {
		return &ValidationError{
			Field:   "id",
			Message: "is required",
		}
	}

	if t.Title == "" {
		return &ValidationError{
			Field:   "title",
			Message: "is required",
		}
	}

	if t.Role == "" {
		return &ValidationError{
			Field:   "role",
			Message: "is required",
		}
	}

	// Validate priority
	if !t.Priority.IsValid() {
		return &ValidationError{
			Field:   "priority",
			Message: fmt.Sprintf("invalid value %q: must be critical, high, medium, or low", t.Priority),
		}
	}

	// Default priority to medium if empty
	if t.Priority == "" {
		t.Priority = PriorityMedium
	}

	// Validate status
	if !t.Status.IsValid() {
		return &ValidationError{
			Field:   "status",
			Message: fmt.Sprintf("invalid value %q", t.Status),
		}
	}

	// Default status to pending if empty
	if t.Status == "" {
		t.Status = StatusPending
	}

	// Validate completion config if present
	if t.Completion != nil {
		if t.Completion.Verify == "" && t.Completion.Signal == "" {
			return &ValidationError{
				Field:   "completion",
				Message: "must have verify or signal",
			}
		}
	}

	return nil
}
