package task

import (
	"bytes"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// taskFrontMatter is a subset of Task fields that are written to the YAML front matter.
// This excludes derived fields like FilePath and Content.
type taskFrontMatter struct {
	ID         string            `yaml:"id"`
	Title      string            `yaml:"title"`
	Workstream string            `yaml:"workstream,omitempty"`
	Priority   Priority          `yaml:"priority,omitempty"`
	Status     Status            `yaml:"status,omitempty"`
	DependsOn  []string          `yaml:"depends_on,omitempty"`
	AssignedTo string            `yaml:"assigned_to,omitempty"`
	Completion *CompletionConfig `yaml:"completion,omitempty"`
	Tags       []string          `yaml:"tags,omitempty"`

	// Error and log tracking
	FailureMessage string `yaml:"failure_message,omitempty"`
	LogFilePath    string `yaml:"log_file,omitempty"`
	ValidationLog  string `yaml:"validation_log,omitempty"`
}

// WriteFile writes task back to file, preserving markdown content.
func WriteFile(t *Task) error {
	if t == nil {
		return fmt.Errorf("task is nil")
	}

	if t.FilePath == "" {
		return fmt.Errorf("task has no file path")
	}

	// Create front matter struct with only serializable fields
	fm := taskFrontMatter{
		ID:             t.ID,
		Title:          t.Title,
		Workstream:     t.Workstream,
		Priority:       t.Priority,
		Status:         t.Status,
		DependsOn:      t.DependsOn,
		AssignedTo:     t.AssignedTo,
		Completion:     t.Completion,
		Tags:           t.Tags,
		FailureMessage: t.FailureMessage,
		LogFilePath:    t.LogFilePath,
		ValidationLog:  t.ValidationLog,
	}

	// Marshal front matter with proper YAML formatting
	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(fm); err != nil {
		return fmt.Errorf("marshal front matter: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return fmt.Errorf("close encoder: %w", err)
	}

	// Combine with markdown content
	content := fmt.Sprintf("---\n%s---\n\n%s\n", buf.String(), t.Content)

	return os.WriteFile(t.FilePath, []byte(content), 0600)
}

// Serialize returns the task as a string in the markdown+YAML format.
func Serialize(t *Task) (string, error) {
	if t == nil {
		return "", fmt.Errorf("task is nil")
	}

	// Create front matter struct with only serializable fields
	fm := taskFrontMatter{
		ID:             t.ID,
		Title:          t.Title,
		Workstream:     t.Workstream,
		Priority:       t.Priority,
		Status:         t.Status,
		DependsOn:      t.DependsOn,
		AssignedTo:     t.AssignedTo,
		Completion:     t.Completion,
		Tags:           t.Tags,
		FailureMessage: t.FailureMessage,
		LogFilePath:    t.LogFilePath,
		ValidationLog:  t.ValidationLog,
	}

	// Marshal front matter with proper YAML formatting
	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(fm); err != nil {
		return "", fmt.Errorf("marshal front matter: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return "", fmt.Errorf("close encoder: %w", err)
	}

	// Combine with markdown content
	return fmt.Sprintf("---\n%s---\n\n%s\n", buf.String(), t.Content), nil
}
