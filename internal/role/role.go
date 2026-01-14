// Package role provides role configuration and management for Tanuki agents.
//
// Roles allow spawning agents with pre-configured system prompts, allowed tools,
// and context files. They enable specialization like "backend", "frontend", "qa", etc.
package role

import (
	"github.com/bkonkle/tanuki/internal/config"
)

// Role represents a role configuration for an agent.
// Roles define specialized agent behavior through system prompts, tool restrictions,
// and context files. They can be built-in defaults or custom per-project definitions.
type Role struct {
	// Name is the unique identifier for the role (e.g., "backend", "frontend")
	Name string `yaml:"name" validate:"required"`

	// Description briefly explains the role's purpose
	Description string `yaml:"description" validate:"required"`

	// Builtin indicates if this is a built-in role (not from a file)
	Builtin bool `yaml:"builtin,omitempty"`

	// SystemPrompt is the custom prompt appended to Claude Code's default prompt
	SystemPrompt string `yaml:"system_prompt"`

	// SystemPromptFile is the path to a file containing the system prompt
	// (relative to project root). Takes precedence over SystemPrompt if set.
	SystemPromptFile string `yaml:"system_prompt_file"`

	// AllowedTools lists the Claude Code tools this role is permitted to use.
	// Common tools: Read, Write, Edit, Bash, Glob, Grep, WebFetch, TodoWrite
	AllowedTools []string `yaml:"allowed_tools,omitempty"`

	// DisallowedTools lists tools explicitly denied for this role.
	// Takes precedence over AllowedTools if both are set.
	DisallowedTools []string `yaml:"disallowed_tools,omitempty"`

	// ContextFiles lists files to include as context when spawning agents
	// (paths relative to project root)
	ContextFiles []string `yaml:"context_files,omitempty"`

	// Model overrides the default Claude model for this role
	Model string `yaml:"model"`

	// Resources overrides the default container resource limits
	Resources *config.ResourceConfig `yaml:"resources"`

	// MaxTurns overrides the default maximum conversation turns
	MaxTurns int `yaml:"max_turns" validate:"omitempty,gte=1,lte=1000"`
}

// Validate checks if the role configuration is valid.
// Returns an error if required fields are missing or values are invalid.
func (r *Role) Validate() error {
	if r.Name == "" {
		return &ValidationError{
			Field:   "name",
			Message: "is required",
		}
	}

	if r.Description == "" {
		return &ValidationError{
			Field:   "description",
			Message: "is required",
		}
	}

	// At least one of SystemPrompt or SystemPromptFile should be set
	if r.SystemPrompt == "" && r.SystemPromptFile == "" {
		return &ValidationError{
			Field:   "system_prompt",
			Message: "either system_prompt or system_prompt_file must be set",
		}
	}

	// If MaxTurns is set and non-zero, validate range (0 means use default)
	if r.MaxTurns < 0 || r.MaxTurns > 1000 {
		return &ValidationError{
			Field:   "max_turns",
			Message: "max_turns must be between 1 and 1000",
		}
	}

	return nil
}

// ValidationError represents a role validation error.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	if e.Field != "" {
		return "role." + e.Field + ": " + e.Message
	}
	return e.Message
}
