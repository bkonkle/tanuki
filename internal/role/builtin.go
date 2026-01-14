package role

import (
	"fmt"
	"log"

	"github.com/bkonkle/tanuki/internal/role/builtin"
	"gopkg.in/yaml.v3"
)

// BuiltinRoles returns the default built-in roles provided by Tanuki.
// These roles can be overridden by project-specific role files.
func BuiltinRoles() []*Role {
	roles, err := loadBuiltinRoles()
	if err != nil {
		// This should never happen since embedded YAML is compile-time validated
		log.Fatalf("Failed to load built-in roles: %v", err)
	}

	// Return as slice in consistent order
	return []*Role{
		roles["backend"],
		roles["frontend"],
		roles["qa"],
		roles["docs"],
		roles["devops"],
		roles["fullstack"],
	}
}

// loadBuiltinRoles loads all built-in roles from embedded YAML files.
func loadBuiltinRoles() (map[string]*Role, error) {
	roles := make(map[string]*Role)

	// Load each built-in role from embedded YAML
	roleData := map[string]string{
		"backend":   builtin.BackendYAML,
		"frontend":  builtin.FrontendYAML,
		"qa":        builtin.QAYAML,
		"docs":      builtin.DocsYAML,
		"devops":    builtin.DevOpsYAML,
		"fullstack": builtin.FullstackYAML,
	}

	for name, yamlContent := range roleData {
		role := &Role{}
		if err := yaml.Unmarshal([]byte(yamlContent), role); err != nil {
			return nil, fmt.Errorf("failed to parse built-in role %s: %w", name, err)
		}

		// Ensure builtin flag is set
		role.Builtin = true

		// Validate the role
		if err := role.Validate(); err != nil {
			return nil, fmt.Errorf("invalid built-in role %s: %w", name, err)
		}

		roles[name] = role
	}

	return roles, nil
}

// GetBuiltinRole returns a specific built-in role by name.
// Returns nil if the role doesn't exist.
func GetBuiltinRole(name string) (*Role, error) {
	roles, err := loadBuiltinRoles()
	if err != nil {
		return nil, err
	}

	role, ok := roles[name]
	if !ok {
		return nil, fmt.Errorf("built-in role not found: %s", name)
	}

	return role, nil
}

// ListBuiltinRoleNames returns the names of all available built-in roles.
func ListBuiltinRoleNames() []string {
	return []string{"backend", "frontend", "qa", "docs", "devops", "fullstack"}
}
