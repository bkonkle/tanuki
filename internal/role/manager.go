package role

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/bkonkle/tanuki/internal/config"
	"gopkg.in/yaml.v3"
)

// Manager handles role operations.
type Manager interface {
	// List returns all available roles (builtin + project + config)
	List() ([]*Role, error)

	// Get retrieves a role by name. Config overrides project roles, which override builtin roles.
	Get(name string) (*Role, error)

	// LoadFromFile loads a role from a YAML file
	LoadFromFile(path string) (*Role, error)

	// GetBuiltinRoles returns all builtin roles
	GetBuiltinRoles() []*Role

	// InitRoles creates default role files in .tanuki/roles/
	InitRoles() error

	// Validate checks if a role configuration is valid
	Validate(role *Role) error
}

// FileManager implements Manager using filesystem storage.
type FileManager struct {
	projectRoot string
	rolesDir    string
	config      *config.Config
}

// NewManager creates a new role manager.
func NewManager(projectRoot string) *FileManager {
	return &FileManager{
		projectRoot: projectRoot,
		rolesDir:    filepath.Join(projectRoot, ".tanuki", "roles"),
	}
}

// NewManagerWithConfig creates a new role manager with config for merging.
func NewManagerWithConfig(projectRoot string, cfg *config.Config) *FileManager {
	return &FileManager{
		projectRoot: projectRoot,
		rolesDir:    filepath.Join(projectRoot, ".tanuki", "roles"),
		config:      cfg,
	}
}

// SetConfig sets the config for role merging.
func (m *FileManager) SetConfig(cfg *config.Config) {
	m.config = cfg
}

// List returns all available roles (builtin + project + config).
// Precedence: config > project roles > builtin roles.
func (m *FileManager) List() ([]*Role, error) {
	roles := make(map[string]*Role)

	// Add builtin roles first
	for _, role := range BuiltinRoles() {
		roles[role.Name] = role
	}

	// Override with project roles if they exist
	if _, err := os.Stat(m.rolesDir); err == nil {
		entries, err := os.ReadDir(m.rolesDir)
		if err != nil {
			return nil, fmt.Errorf("read roles directory: %w", err)
		}

		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
				continue
			}

			path := filepath.Join(m.rolesDir, entry.Name())
			role, err := m.LoadFromFile(path)
			if err != nil {
				// Log warning but continue
				continue
			}

			roles[role.Name] = role
		}
	}

	// Apply config overrides if present
	for name, role := range roles {
		m.applyConfigOverrides(role)
		roles[name] = role
	}

	// Add any roles defined only in config (not in files)
	if m.config != nil && m.config.Roles != nil {
		for name, roleCfg := range m.config.Roles {
			if _, exists := roles[name]; !exists {
				// Create a new role from config only
				role := m.roleFromConfig(name, roleCfg)
				if role != nil {
					roles[name] = role
				}
			}
		}
	}

	// Convert map to slice
	result := make([]*Role, 0, len(roles))
	for _, role := range roles {
		result = append(result, role)
	}

	return result, nil
}

// Get retrieves a role by name. Config overrides project roles, which override builtin roles.
func (m *FileManager) Get(name string) (*Role, error) {
	var role *Role

	// Check project roles first
	projectRolePath := filepath.Join(m.rolesDir, name+".yaml")
	if _, err := os.Stat(projectRolePath); err == nil {
		r, err := m.LoadFromFile(projectRolePath)
		if err != nil {
			return nil, err
		}
		role = r
	}

	// Fall back to builtin roles if no project role found
	if role == nil {
		for _, r := range BuiltinRoles() {
			if r.Name == name {
				// Copy the builtin role to avoid modifying the original
				roleCopy := *r
				role = &roleCopy
				break
			}
		}
	}

	// Check if role exists only in config
	if role == nil && m.config != nil && m.config.Roles != nil {
		if roleCfg, exists := m.config.Roles[name]; exists {
			role = m.roleFromConfig(name, roleCfg)
		}
	}

	if role == nil {
		return nil, fmt.Errorf("role %q not found", name)
	}

	// Apply config overrides
	m.applyConfigOverrides(role)

	return role, nil
}

// applyConfigOverrides applies config.Roles settings to a role.
// Config settings take precedence over role file/builtin settings.
func (m *FileManager) applyConfigOverrides(role *Role) {
	if m.config == nil || m.config.Roles == nil {
		return
	}

	roleCfg, exists := m.config.Roles[role.Name]
	if !exists || roleCfg == nil {
		return
	}

	// Apply overrides - config values take precedence if set
	if roleCfg.Concurrency > 0 {
		role.Concurrency = roleCfg.Concurrency
	}
	if roleCfg.SystemPrompt != "" {
		role.SystemPrompt = roleCfg.SystemPrompt
	}
	if roleCfg.SystemPromptFile != "" {
		role.SystemPromptFile = roleCfg.SystemPromptFile
		// Load the prompt from file
		promptPath := filepath.Join(m.projectRoot, roleCfg.SystemPromptFile)
		if promptData, err := os.ReadFile(promptPath); err == nil {
			role.SystemPrompt = string(promptData)
		}
	}
	if len(roleCfg.AllowedTools) > 0 {
		role.AllowedTools = roleCfg.AllowedTools
	}
	if len(roleCfg.DisallowedTools) > 0 {
		role.DisallowedTools = roleCfg.DisallowedTools
	}
	if roleCfg.Model != "" {
		role.Model = roleCfg.Model
	}
	if roleCfg.MaxTurns > 0 {
		role.MaxTurns = roleCfg.MaxTurns
	}
	if roleCfg.Resources != nil {
		role.Resources = roleCfg.Resources
	}
}

// roleFromConfig creates a Role from a config.RoleConfig.
// Used when a role is defined only in config, not in files.
func (m *FileManager) roleFromConfig(name string, cfg *config.RoleConfig) *Role {
	if cfg == nil {
		return nil
	}

	role := &Role{
		Name:            name,
		Description:     fmt.Sprintf("Role %s defined in tanuki.yaml", name),
		Builtin:         false,
		Concurrency:     cfg.Concurrency,
		SystemPrompt:    cfg.SystemPrompt,
		SystemPromptFile: cfg.SystemPromptFile,
		AllowedTools:    cfg.AllowedTools,
		DisallowedTools: cfg.DisallowedTools,
		Model:           cfg.Model,
		MaxTurns:        cfg.MaxTurns,
		Resources:       cfg.Resources,
	}

	// Load system prompt from file if specified
	if role.SystemPromptFile != "" {
		promptPath := filepath.Join(m.projectRoot, role.SystemPromptFile)
		if promptData, err := os.ReadFile(promptPath); err == nil {
			role.SystemPrompt = string(promptData)
		}
	}

	return role
}

// LoadFromFile loads a role from a YAML file.
func (m *FileManager) LoadFromFile(path string) (*Role, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read role file: %w", err)
	}

	var role Role
	if err := yaml.Unmarshal(data, &role); err != nil {
		return nil, fmt.Errorf("parse role YAML: %w", err)
	}

	// Mark as not builtin since it's loaded from file
	role.Builtin = false

	// Validate the role
	if err := m.Validate(&role); err != nil {
		return nil, fmt.Errorf("invalid role: %w", err)
	}

	// If SystemPromptFile is set, load the prompt from file
	if role.SystemPromptFile != "" {
		promptPath := filepath.Join(m.projectRoot, role.SystemPromptFile)
		promptData, err := os.ReadFile(promptPath)
		if err != nil {
			return nil, fmt.Errorf("read system prompt file: %w", err)
		}
		role.SystemPrompt = string(promptData)
	}

	return &role, nil
}

// GetBuiltinRoles returns all builtin roles.
func (m *FileManager) GetBuiltinRoles() []*Role {
	return BuiltinRoles()
}

// InitRoles creates default role files in .tanuki/roles/.
func (m *FileManager) InitRoles() error {
	// Create roles directory
	if err := os.MkdirAll(m.rolesDir, 0755); err != nil {
		return fmt.Errorf("create roles directory: %w", err)
	}

	// Write each builtin role to a file
	for _, role := range BuiltinRoles() {
		rolePath := filepath.Join(m.rolesDir, role.Name+".yaml")

		// Skip if file already exists
		if _, err := os.Stat(rolePath); err == nil {
			continue
		}

		data, err := yaml.Marshal(role)
		if err != nil {
			return fmt.Errorf("marshal role %q: %w", role.Name, err)
		}

		if err := os.WriteFile(rolePath, data, 0644); err != nil {
			return fmt.Errorf("write role file %q: %w", role.Name, err)
		}
	}

	return nil
}

// Validate checks if a role configuration is valid.
func (m *FileManager) Validate(role *Role) error {
	return role.Validate()
}
