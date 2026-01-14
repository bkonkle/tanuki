package role

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Manager handles role operations.
type Manager interface {
	// List returns all available roles (builtin + project)
	List() ([]*Role, error)

	// Get retrieves a role by name. Project roles override builtin roles.
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
}

// NewManager creates a new role manager.
func NewManager(projectRoot string) *FileManager {
	return &FileManager{
		projectRoot: projectRoot,
		rolesDir:    filepath.Join(projectRoot, ".tanuki", "roles"),
	}
}

// List returns all available roles (builtin + project).
// Project roles with the same name override builtin roles.
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

	// Convert map to slice
	result := make([]*Role, 0, len(roles))
	for _, role := range roles {
		result = append(result, role)
	}

	return result, nil
}

// Get retrieves a role by name. Project roles override builtin roles.
func (m *FileManager) Get(name string) (*Role, error) {
	// Check project roles first
	projectRolePath := filepath.Join(m.rolesDir, name+".yaml")
	if _, err := os.Stat(projectRolePath); err == nil {
		return m.LoadFromFile(projectRolePath)
	}

	// Fall back to builtin roles
	for _, role := range BuiltinRoles() {
		if role.Name == name {
			return role, nil
		}
	}

	return nil, fmt.Errorf("role %q not found", name)
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
