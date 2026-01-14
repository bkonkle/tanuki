package role

import (
	"testing"

	"github.com/bkonkle/tanuki/internal/config"
	"gopkg.in/yaml.v3"
)

func TestRole_Validate(t *testing.T) {
	tests := []struct {
		name    string
		role    *Role
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid role with system_prompt",
			role: &Role{
				Name:         "backend",
				Description:  "Backend specialist",
				SystemPrompt: "You are a backend developer",
				AllowedTools: []string{"Read", "Write"},
			},
			wantErr: false,
		},
		{
			name: "valid role with system_prompt_file",
			role: &Role{
				Name:             "backend",
				Description:      "Backend specialist",
				SystemPromptFile: "prompts/backend.md",
				AllowedTools:     []string{"Read", "Write"},
			},
			wantErr: false,
		},
		{
			name: "valid role with both prompts (file takes precedence)",
			role: &Role{
				Name:             "backend",
				Description:      "Backend specialist",
				SystemPrompt:     "You are a backend developer",
				SystemPromptFile: "prompts/backend.md",
				AllowedTools:     []string{"Read", "Write"},
			},
			wantErr: false,
		},
		{
			name: "missing name",
			role: &Role{
				Description:  "Backend specialist",
				SystemPrompt: "You are a backend developer",
			},
			wantErr: true,
			errMsg:  "is required",
		},
		{
			name: "missing description",
			role: &Role{
				Name:         "backend",
				SystemPrompt: "You are a backend developer",
			},
			wantErr: true,
			errMsg:  "is required",
		},
		{
			name: "missing both system prompts",
			role: &Role{
				Name:        "backend",
				Description: "Backend specialist",
			},
			wantErr: true,
			errMsg:  "either system_prompt or system_prompt_file must be set",
		},
		{
			name: "max_turns negative (invalid)",
			role: &Role{
				Name:         "backend",
				Description:  "Backend specialist",
				SystemPrompt: "You are a backend developer",
				MaxTurns:     -1,
			},
			wantErr: true,
			errMsg:  "max_turns must be between 1 and 1000",
		},
		{
			name: "max_turns zero (valid - use default)",
			role: &Role{
				Name:         "backend",
				Description:  "Backend specialist",
				SystemPrompt: "You are a backend developer",
				MaxTurns:     0,
			},
			wantErr: false,
		},
		{
			name: "max_turns too high",
			role: &Role{
				Name:         "backend",
				Description:  "Backend specialist",
				SystemPrompt: "You are a backend developer",
				MaxTurns:     1001,
			},
			wantErr: true,
			errMsg:  "max_turns must be between 1 and 1000",
		},
		{
			name: "valid max_turns",
			role: &Role{
				Name:         "backend",
				Description:  "Backend specialist",
				SystemPrompt: "You are a backend developer",
				MaxTurns:     50,
			},
			wantErr: false,
		},
		{
			name: "valid with resources",
			role: &Role{
				Name:         "backend",
				Description:  "Backend specialist",
				SystemPrompt: "You are a backend developer",
				Resources: &config.ResourceConfig{
					Memory: "8g",
					CPUs:   "4",
				},
			},
			wantErr: false,
		},
		{
			name: "valid with all optional fields",
			role: &Role{
				Name:         "backend",
				Description:  "Backend specialist",
				SystemPrompt: "You are a backend developer",
				AllowedTools: []string{"Read", "Write", "Edit"},
				DisallowedTools: []string{"Bash"},
				ContextFiles: []string{"docs/api.md", "CONTRIBUTING.md"},
				Model:        "claude-sonnet-4-5-20250514",
				Resources: &config.ResourceConfig{
					Memory: "8g",
					CPUs:   "4",
				},
				MaxTurns: 100,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.role.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("Validate() expected error but got nil")
					return
				}
				if tt.errMsg != "" {
					verr, ok := err.(*ValidationError)
					if !ok {
						t.Errorf("Validate() expected ValidationError but got %T", err)
						return
					}
					if verr.Message != tt.errMsg {
						t.Errorf("Validate() error message = %q, want %q", verr.Message, tt.errMsg)
					}
				}
			} else {
				if err != nil {
					t.Errorf("Validate() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestRole_YAML_Marshal(t *testing.T) {
	role := &Role{
		Name:         "backend",
		Description:  "Backend development specialist",
		SystemPrompt: "You are a backend developer",
		AllowedTools: []string{"Read", "Write", "Edit"},
		DisallowedTools: []string{"Bash"},
		ContextFiles: []string{"docs/api.md"},
		Model:        "claude-sonnet-4-5-20250514",
		Resources: &config.ResourceConfig{
			Memory: "8g",
			CPUs:   "4",
		},
		MaxTurns: 100,
	}

	data, err := yaml.Marshal(role)
	if err != nil {
		t.Fatalf("Marshal() error: %v", err)
	}

	// Unmarshal back to verify round-trip
	var unmarshaled Role
	if err := yaml.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Unmarshal() error: %v", err)
	}

	// Verify key fields
	if unmarshaled.Name != role.Name {
		t.Errorf("Name = %q, want %q", unmarshaled.Name, role.Name)
	}
	if unmarshaled.Description != role.Description {
		t.Errorf("Description = %q, want %q", unmarshaled.Description, role.Description)
	}
	if unmarshaled.SystemPrompt != role.SystemPrompt {
		t.Errorf("SystemPrompt = %q, want %q", unmarshaled.SystemPrompt, role.SystemPrompt)
	}
	if unmarshaled.Model != role.Model {
		t.Errorf("Model = %q, want %q", unmarshaled.Model, role.Model)
	}
	if unmarshaled.MaxTurns != role.MaxTurns {
		t.Errorf("MaxTurns = %d, want %d", unmarshaled.MaxTurns, role.MaxTurns)
	}
	if len(unmarshaled.AllowedTools) != len(role.AllowedTools) {
		t.Errorf("AllowedTools length = %d, want %d", len(unmarshaled.AllowedTools), len(role.AllowedTools))
	}
	if unmarshaled.Resources.Memory != role.Resources.Memory {
		t.Errorf("Resources.Memory = %q, want %q", unmarshaled.Resources.Memory, role.Resources.Memory)
	}
}

func TestRole_YAML_Unmarshal(t *testing.T) {
	yamlData := `
name: backend
description: Backend development specialist
system_prompt: |
  You are a backend development specialist.
  Focus on API design and database operations.
allowed_tools:
  - Read
  - Write
  - Edit
  - Bash
disallowed_tools:
  - WebFetch
context_files:
  - docs/architecture.md
  - CONTRIBUTING.md
model: claude-sonnet-4-5-20250514
resources:
  memory: "8g"
  cpus: "4"
max_turns: 100
`

	var role Role
	if err := yaml.Unmarshal([]byte(yamlData), &role); err != nil {
		t.Fatalf("Unmarshal() error: %v", err)
	}

	// Verify parsed values
	if role.Name != "backend" {
		t.Errorf("Name = %q, want %q", role.Name, "backend")
	}
	if role.Description != "Backend development specialist" {
		t.Errorf("Description = %q, want %q", role.Description, "Backend development specialist")
	}
	if role.Model != "claude-sonnet-4-5-20250514" {
		t.Errorf("Model = %q, want %q", role.Model, "claude-sonnet-4-5-20250514")
	}
	if role.MaxTurns != 100 {
		t.Errorf("MaxTurns = %d, want %d", role.MaxTurns, 100)
	}
	if len(role.AllowedTools) != 4 {
		t.Errorf("AllowedTools length = %d, want %d", len(role.AllowedTools), 4)
	}
	if len(role.DisallowedTools) != 1 {
		t.Errorf("DisallowedTools length = %d, want %d", len(role.DisallowedTools), 1)
	}
	if len(role.ContextFiles) != 2 {
		t.Errorf("ContextFiles length = %d, want %d", len(role.ContextFiles), 2)
	}
	if role.Resources == nil {
		t.Fatal("Resources is nil")
	}
	if role.Resources.Memory != "8g" {
		t.Errorf("Resources.Memory = %q, want %q", role.Resources.Memory, "8g")
	}
	if role.Resources.CPUs != "4" {
		t.Errorf("Resources.CPUs = %q, want %q", role.Resources.CPUs, "4")
	}

	// Validate the parsed role
	if err := role.Validate(); err != nil {
		t.Errorf("Validate() error: %v", err)
	}
}

func TestValidationError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  *ValidationError
		want string
	}{
		{
			name: "with field",
			err: &ValidationError{
				Field:   "name",
				Message: "is required",
			},
			want: "role.name: is required",
		},
		{
			name: "without field",
			err: &ValidationError{
				Message: "invalid configuration",
			},
			want: "invalid configuration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}
