// Package config provides configuration management for Tanuki.
//
// Configuration is loaded from multiple sources with the following precedence
// (highest to lowest):
//  1. CLI flags (set via SetOverride)
//  2. Project config: ./tanuki.yaml or ./.tanuki/config/tanuki.yaml
//  3. Global config: ~/.config/tanuki/config.yaml
//  4. Built-in defaults
//
// The package uses Viper for configuration merging and supports automatic
// environment variable binding with the TANUKI_ prefix.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// Config represents the tanuki.yaml configuration file.
// This is the root configuration structure containing all settings for Tanuki.
type Config struct {
	// Version is the configuration schema version (currently "1")
	Version string `yaml:"version" mapstructure:"version" validate:"required,eq=1"`

	// TasksDir is the directory for task files, relative to project root.
	// Defaults to "tasks" (visible in project). Can be set to ".tanuki/tasks" for hidden tasks.
	TasksDir string `yaml:"tasks_dir,omitempty" mapstructure:"tasks_dir"`

	// Image specifies the Docker image configuration for agent containers
	Image ImageConfig `yaml:"image" mapstructure:"image"`

	// Defaults contains default settings applied to all agents
	Defaults AgentDefaults `yaml:"defaults" mapstructure:"defaults"`

	// Workstreams contains workstream-specific configuration overrides.
	// Workstreams can be organized however makes sense for your project -
	// by feature area (auth, payments), by discipline (backend, frontend),
	// or any other grouping that fits your workflow.
	Workstreams map[string]*WorkstreamConfig `yaml:"workstreams,omitempty" mapstructure:"workstreams"`

	// Git contains Git-related settings for branch management
	Git GitConfig `yaml:"git" mapstructure:"git"`

	// Network contains Docker network settings
	Network NetworkConfig `yaml:"network" mapstructure:"network"`
}

// WorkstreamConfig contains configuration for a specific workstream.
// These settings override AgentDefaults when an agent is spawned for this workstream.
// Workstreams can be organized by feature area, discipline, or any grouping that fits your workflow.
type WorkstreamConfig struct {
	// Concurrency is the maximum number of concurrent instances for this workstream
	Concurrency int `yaml:"concurrency,omitempty" mapstructure:"concurrency" validate:"omitempty,gte=1,lte=10"`

	// SystemPrompt is the workstream-specific system prompt
	SystemPrompt string `yaml:"system_prompt,omitempty" mapstructure:"system_prompt"`

	// SystemPromptFile is the path to a file containing the system prompt
	SystemPromptFile string `yaml:"system_prompt_file,omitempty" mapstructure:"system_prompt_file"`

	// AllowedTools overrides the default allowed tools for this workstream
	AllowedTools []string `yaml:"allowed_tools,omitempty" mapstructure:"allowed_tools"`

	// DisallowedTools lists tools explicitly denied for this workstream
	DisallowedTools []string `yaml:"disallowed_tools,omitempty" mapstructure:"disallowed_tools"`

	// Model overrides the default Claude model for this workstream
	Model string `yaml:"model,omitempty" mapstructure:"model"`

	// MaxTurns overrides the default maximum conversation turns
	MaxTurns int `yaml:"max_turns,omitempty" mapstructure:"max_turns" validate:"omitempty,gte=1,lte=1000"`

	// Resources overrides the default container resource limits
	Resources *ResourceConfig `yaml:"resources,omitempty" mapstructure:"resources"`
}

// GetConcurrency returns the concurrency setting with a default of 1.
func (w *WorkstreamConfig) GetConcurrency() int {
	if w == nil || w.Concurrency <= 0 {
		return 1
	}
	return w.Concurrency
}

// ImageConfig specifies which Docker image to use for agents.
// Either Name+Tag or Build should be specified, not both.
type ImageConfig struct {
	// Name is the Docker image name (e.g., "node")
	Name string `yaml:"name" mapstructure:"name" validate:"required_without=Build"`

	// Tag is the Docker image tag (e.g., "latest", "v1.0.0")
	Tag string `yaml:"tag" mapstructure:"tag" validate:"required_without=Build"`

	// Build specifies how to build the image locally instead of pulling
	Build *BuildConfig `yaml:"build,omitempty" mapstructure:"build"`
}

// BuildConfig specifies how to build the Docker image locally.
// Use this when you want to build from a local Dockerfile instead of pulling.
type BuildConfig struct {
	// Context is the build context path (e.g., ".")
	Context string `yaml:"context" mapstructure:"context" validate:"required"`

	// Dockerfile is the path to the Dockerfile relative to Context
	Dockerfile string `yaml:"dockerfile" mapstructure:"dockerfile" validate:"required"`
}

// AgentDefaults contains default settings for all agents.
// These can be overridden per-agent in spawn commands.
type AgentDefaults struct {
	// AllowedTools lists the Claude Code tools agents are permitted to use
	// Common tools: Read, Write, Edit, Bash, Glob, Grep, WebFetch
	AllowedTools []string `yaml:"allowed_tools" mapstructure:"allowed_tools"`

	// MaxTurns is the maximum number of conversation turns before stopping
	MaxTurns int `yaml:"max_turns" mapstructure:"max_turns" validate:"gte=1,lte=1000"`

	// MaxWorkstreamTurns is the max turns before context reset in a workstream
	// When exceeded, the workstream session is saved and a fresh instance starts
	MaxWorkstreamTurns int `yaml:"max_workstream_turns,omitempty" mapstructure:"max_workstream_turns" validate:"omitempty,gte=50,lte=1000"`

	// Model is the Claude model to use (e.g., "claude-haiku-4-5-20250514")
	Model string `yaml:"model" mapstructure:"model" validate:"required"`

	// Resources specifies container resource limits
	Resources ResourceConfig `yaml:"resources" mapstructure:"resources"`
}

// GetMaxWorkstreamTurns returns the max workstream turns with default fallback.
func (a *AgentDefaults) GetMaxWorkstreamTurns() int {
	if a.MaxWorkstreamTurns <= 0 {
		return 200 // Default
	}
	return a.MaxWorkstreamTurns
}

// ResourceConfig specifies resource limits for agent containers.
// These map directly to Docker container resource constraints.
type ResourceConfig struct {
	// Memory limit (e.g., "4g", "512m")
	Memory string `yaml:"memory" mapstructure:"memory" validate:"required"`

	// CPUs limit (e.g., "2", "0.5")
	CPUs string `yaml:"cpus" mapstructure:"cpus" validate:"required"`
}

// GitConfig specifies Git-related settings for branch and worktree management.
type GitConfig struct {
	// BranchPrefix is prepended to agent names when creating branches
	// e.g., with prefix "tanuki/", agent "feature-x" gets branch "tanuki/feature-x"
	BranchPrefix string `yaml:"branch_prefix" mapstructure:"branch_prefix"`

	// AutoPush automatically pushes commits to remote when true
	AutoPush bool `yaml:"auto_push" mapstructure:"auto_push"`
}

// NetworkConfig specifies Docker network settings for agent communication.
type NetworkConfig struct {
	// Name is the Docker network name that agents will be attached to
	Name string `yaml:"name" mapstructure:"name" validate:"required"`
}

// ValidationError represents a configuration validation error with field details.
type ValidationError struct {
	Field   string
	Tag     string
	Value   interface{}
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

// ValidationErrors is a collection of validation errors.
type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return "no validation errors"
	}
	msgs := make([]string, 0, len(e))
	for _, err := range e {
		msgs = append(msgs, err.Message)
	}
	return strings.Join(msgs, "; ")
}

// Loader handles configuration loading from multiple sources.
type Loader struct {
	v         *viper.Viper
	validator *validator.Validate
	overrides map[string]interface{}
}

// NewLoader creates a new configuration loader.
func NewLoader() *Loader {
	v := viper.New()
	v.SetConfigType("yaml")

	return &Loader{
		v:         v,
		validator: validator.New(),
		overrides: make(map[string]interface{}),
	}
}

// SetOverride sets a CLI override value that takes highest precedence.
// Use dot notation for nested keys (e.g., "defaults.max_turns").
func (l *Loader) SetOverride(key string, value interface{}) {
	l.overrides[key] = value
}

// Load reads configuration from all sources and returns the merged result.
// It searches for config files in the following order:
//  1. ./tanuki.yaml
//  2. ./.tanuki/config/tanuki.yaml
//  3. ~/.config/tanuki/config.yaml
//
// All found configs are merged with CLI overrides taking highest precedence.
func (l *Loader) Load() (*Config, error) {
	// Start with defaults
	cfg := DefaultConfig()
	l.setDefaults()

	// Load global config (~/.config/tanuki/config.yaml)
	globalPath := l.globalConfigPath()
	if globalPath != "" && fileExists(globalPath) {
		if err := l.loadConfigFile(globalPath); err != nil {
			return nil, fmt.Errorf("failed to load global config %s: %w", globalPath, err)
		}
	}

	// Load project config (./tanuki.yaml or ./.tanuki/config/tanuki.yaml)
	projectPath := l.findProjectConfig()
	if projectPath != "" {
		if err := l.loadConfigFile(projectPath); err != nil {
			return nil, fmt.Errorf("failed to load project config %s: %w", projectPath, err)
		}
	}

	// Apply CLI overrides (highest precedence)
	for key, value := range l.overrides {
		l.v.Set(key, value)
	}

	// Unmarshal into config struct
	if err := l.v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate the config
	if err := l.Validate(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// LoadFromPath loads configuration from a specific file path.
// This is useful for testing or when a config path is explicitly specified.
func (l *Loader) LoadFromPath(path string) (*Config, error) {
	l.setDefaults()

	if err := l.loadConfigFile(path); err != nil {
		return nil, err
	}

	// Apply CLI overrides
	for key, value := range l.overrides {
		l.v.Set(key, value)
	}

	cfg := &Config{}
	if err := l.v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if err := l.Validate(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate checks the configuration against the schema.
// Returns ValidationErrors with detailed information about any issues.
func (l *Loader) Validate(cfg *Config) error {
	var errs ValidationErrors

	// Validate struct tags
	err := l.validator.Struct(cfg)
	if err != nil {
		var validationErrs validator.ValidationErrors
		if errors.As(err, &validationErrs) {
			for _, e := range validationErrs {
				errs = append(errs, ValidationError{
					Field:   e.Namespace(),
					Tag:     e.Tag(),
					Value:   e.Value(),
					Message: formatValidationError(e),
				})
			}
		} else {
			return fmt.Errorf("validation error: %w", err)
		}
	}

	if len(errs) > 0 {
		return errs
	}

	return nil
}

func (l *Loader) setDefaults() {
	defaults := DefaultConfig()

	l.v.SetDefault("version", defaults.Version)
	l.v.SetDefault("tasks_dir", defaults.TasksDir)
	l.v.SetDefault("image.name", defaults.Image.Name)
	l.v.SetDefault("image.tag", defaults.Image.Tag)
	l.v.SetDefault("defaults.allowed_tools", defaults.Defaults.AllowedTools)
	l.v.SetDefault("defaults.max_turns", defaults.Defaults.MaxTurns)
	l.v.SetDefault("defaults.model", defaults.Defaults.Model)
	l.v.SetDefault("defaults.resources.memory", defaults.Defaults.Resources.Memory)
	l.v.SetDefault("defaults.resources.cpus", defaults.Defaults.Resources.CPUs)
	l.v.SetDefault("git.branch_prefix", defaults.Git.BranchPrefix)
	l.v.SetDefault("git.auto_push", defaults.Git.AutoPush)
	l.v.SetDefault("network.name", defaults.Network.Name)
}

func (l *Loader) loadConfigFile(path string) error {
	l.v.SetConfigFile(path)
	return l.v.MergeInConfig()
}

func (l *Loader) globalConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "tanuki", "config.yaml")
}

func (l *Loader) findProjectConfig() string {
	// Check ./tanuki.yaml first
	if fileExists("tanuki.yaml") {
		return "tanuki.yaml"
	}

	// Check ./.tanuki/config/tanuki.yaml
	altPath := filepath.Join(".tanuki", "config", "tanuki.yaml")
	if fileExists(altPath) {
		return altPath
	}

	return ""
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func formatValidationError(e validator.FieldError) string {
	field := e.Namespace()
	// Remove the "Config." prefix for cleaner messages
	field = strings.TrimPrefix(field, "Config.")

	switch e.Tag() {
	case "required":
		return fmt.Sprintf("'%s' is required", field)
	case "required_without":
		return fmt.Sprintf("'%s' is required when '%s' is not set", field, e.Param())
	case "eq":
		return fmt.Sprintf("'%s' must be '%s' (got '%v')", field, e.Param(), e.Value())
	case "gte":
		return fmt.Sprintf("'%s' must be at least %s (got '%v')", field, e.Param(), e.Value())
	case "lte":
		return fmt.Sprintf("'%s' must be at most %s (got '%v')", field, e.Param(), e.Value())
	default:
		return fmt.Sprintf("'%s' failed validation '%s'", field, e.Tag())
	}
}

// DefaultConfig returns a new Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Version:  "1",
		TasksDir: "tasks",
		Image: ImageConfig{
			Name: "node",
			Tag:  "22",
		},
		Defaults: AgentDefaults{
			AllowedTools: []string{
				"Read",
				"Write",
				"Edit",
				"Bash",
				"Glob",
				"Grep",
			},
			MaxTurns: 50,
			Model:    "claude-haiku-4-5-20251001",
			Resources: ResourceConfig{
				Memory: "4g",
				CPUs:   "2",
			},
		},
		Git: GitConfig{
			BranchPrefix: "tanuki/",
			AutoPush:     false,
		},
		Network: NetworkConfig{
			Name: "tanuki-net",
		},
	}
}

// WriteDefault writes the default configuration to the specified path.
func WriteDefault(path string) error {
	cfg := DefaultConfig()
	return Write(cfg, path)
}

// Write writes the configuration to the specified path.
func Write(cfg *Config, path string) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if dir != "." {
		if err := os.MkdirAll(dir, 0750); err != nil {
			return err
		}
	}

	return os.WriteFile(path, data, 0600)
}

// Load is a convenience function that creates a Loader and loads the config.
// For more control over loading behavior, use NewLoader() directly.
func Load() (*Config, error) {
	return NewLoader().Load()
}

// Exists checks if a configuration file exists at the given path.
func Exists(path string) bool {
	return fileExists(path)
}

// GlobalConfigPath returns the path to the global configuration file.
func GlobalConfigPath() string {
	return NewLoader().globalConfigPath()
}

// FindProjectConfig returns the path to the project configuration file,
// or an empty string if no project config is found.
func FindProjectConfig() string {
	return NewLoader().findProjectConfig()
}

// GetWorkstreamConfig returns the configuration for a specific workstream.
// Returns nil if no workstream-specific config is defined.
func (c *Config) GetWorkstreamConfig(workstreamName string) *WorkstreamConfig {
	if c.Workstreams == nil {
		return nil
	}
	return c.Workstreams[workstreamName]
}

// GetWorkstreamConcurrency returns the concurrency for a specific workstream.
// Returns 1 (default) if the workstream has no specific config.
func (c *Config) GetWorkstreamConcurrency(workstreamName string) int {
	wc := c.GetWorkstreamConfig(workstreamName)
	return wc.GetConcurrency()
}
