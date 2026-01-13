---
id: TANK-023
title: Role Manager Implementation
status: todo
priority: high
estimate: M
depends_on: [TANK-020]
workstream: A
phase: 2
---

# Role Manager Implementation

## Summary

Implement the RoleManager that handles loading, caching, and validating roles. This is the core infrastructure that powers the role system.

## Acceptance Criteria

- [ ] RoleManager interface with List, Get, LoadFromFile methods
- [ ] Built-in roles embedded in binary
- [ ] Project roles loaded from `.tanuki/roles/`
- [ ] Role priority: project roles override built-in
- [ ] Caching for performance
- [ ] Validation on load (required fields, tool names, etc.)
- [ ] Clear error messages for invalid roles
- [ ] Unit tests with 80%+ coverage

## Technical Details

### RoleManager Interface

```go
type RoleManager interface {
    // List all available roles (built-in + project)
    List() ([]*Role, error)

    // Get a specific role by name
    Get(name string) (*Role, error)

    // Load role from file
    LoadFromFile(path string) (*Role, error)

    // Get built-in roles
    GetBuiltinRoles() []*Role

    // Initialize roles directory with defaults
    InitRoles() error

    // Validate a role
    Validate(role *Role) error
}
```

### Implementation

```go
// internal/role/manager.go
package role

import (
    "fmt"
    "os"
    "path/filepath"

    "gopkg.in/yaml.v3"
)

type Manager struct {
    config       *Config
    builtinRoles map[string]*Role
    projectRoles map[string]*Role
    cache        map[string]*Role
}

func NewManager(cfg *Config) *Manager {
    m := &Manager{
        config:       cfg,
        builtinRoles: loadBuiltinRoles(),
        projectRoles: make(map[string]*Role),
        cache:        make(map[string]*Role),
    }

    // Load project roles from .tanuki/roles/
    if err := m.loadProjectRoles(); err != nil {
        // Log warning but don't fail - project may not have custom roles
        fmt.Fprintf(os.Stderr, "Warning: failed to load project roles: %v\n", err)
    }

    return m
}

func (m *Manager) Get(name string) (*Role, error) {
    // Check cache first
    if role, ok := m.cache[name]; ok {
        return role, nil
    }

    // Project roles override built-in
    if role, ok := m.projectRoles[name]; ok {
        if err := m.Validate(role); err != nil {
            return nil, fmt.Errorf("invalid project role %q: %w", name, err)
        }
        m.cache[name] = role
        return role, nil
    }

    if role, ok := m.builtinRoles[name]; ok {
        m.cache[name] = role
        return role, nil
    }

    return nil, fmt.Errorf("role %q not found", name)
}

func (m *Manager) List() ([]*Role, error) {
    seen := make(map[string]bool)
    roles := make([]*Role, 0)

    // Project roles first (they override built-in)
    for name, role := range m.projectRoles {
        if err := m.Validate(role); err != nil {
            // Skip invalid project roles but log warning
            fmt.Fprintf(os.Stderr, "Warning: skipping invalid role %q: %v\n", name, err)
            continue
        }
        roles = append(roles, role)
        seen[name] = true
    }

    // Add built-in roles not overridden
    for name, role := range m.builtinRoles {
        if !seen[name] {
            roles = append(roles, role)
        }
    }

    return roles, nil
}

func (m *Manager) LoadFromFile(path string) (*Role, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("read file: %w", err)
    }

    var role Role
    if err := yaml.Unmarshal(data, &role); err != nil {
        return nil, fmt.Errorf("parse YAML: %w", err)
    }

    // Load system prompt from file if specified
    if role.SystemPromptFile != "" {
        promptPath := role.SystemPromptFile
        if !filepath.IsAbs(promptPath) {
            promptPath = filepath.Join(m.config.ProjectRoot, promptPath)
        }

        promptData, err := os.ReadFile(promptPath)
        if err != nil {
            return nil, fmt.Errorf("read system prompt file: %w", err)
        }
        role.SystemPrompt = string(promptData)
    }

    if err := m.Validate(&role); err != nil {
        return nil, fmt.Errorf("validation failed: %w", err)
    }

    return &role, nil
}

func (m *Manager) loadProjectRoles() error {
    rolesDir := filepath.Join(m.config.ProjectRoot, ".tanuki", "roles")

    // Check if directory exists
    if _, err := os.Stat(rolesDir); os.IsNotExist(err) {
        return nil // Not an error - project may not have custom roles
    }

    entries, err := os.ReadDir(rolesDir)
    if err != nil {
        return fmt.Errorf("read roles directory: %w", err)
    }

    for _, entry := range entries {
        if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
            continue
        }

        path := filepath.Join(rolesDir, entry.Name())
        role, err := m.LoadFromFile(path)
        if err != nil {
            // Log warning but continue loading other roles
            fmt.Fprintf(os.Stderr, "Warning: failed to load %s: %v\n", entry.Name(), err)
            continue
        }

        m.projectRoles[role.Name] = role
    }

    return nil
}

func (m *Manager) GetBuiltinRoles() []*Role {
    roles := make([]*Role, 0, len(m.builtinRoles))
    for _, role := range m.builtinRoles {
        roles = append(roles, role)
    }
    return roles
}

func (m *Manager) InitRoles() error {
    rolesDir := filepath.Join(m.config.ProjectRoot, ".tanuki", "roles")

    // Create roles directory
    if err := os.MkdirAll(rolesDir, 0755); err != nil {
        return fmt.Errorf("create roles directory: %w", err)
    }

    // Write built-in roles as templates
    for name, role := range m.builtinRoles {
        path := filepath.Join(rolesDir, name+".yaml")

        // Skip if file already exists
        if _, err := os.Stat(path); err == nil {
            continue
        }

        data, err := yaml.Marshal(role)
        if err != nil {
            return fmt.Errorf("marshal role %q: %w", name, err)
        }

        if err := os.WriteFile(path, data, 0644); err != nil {
            return fmt.Errorf("write role %q: %w", name, err)
        }
    }

    return nil
}
```

### Validation

```go
// internal/role/validate.go
package role

import (
    "fmt"
    "strings"
)

var validTools = map[string]bool{
    "Read":      true,
    "Write":     true,
    "Edit":      true,
    "Bash":      true,
    "Glob":      true,
    "Grep":      true,
    "TodoWrite": true,
    "Task":      true,
    "WebFetch":  true,
    "WebSearch": true,
}

func (m *Manager) Validate(role *Role) error {
    if role == nil {
        return fmt.Errorf("role is nil")
    }

    // Required fields
    if role.Name == "" {
        return fmt.Errorf("name is required")
    }

    if role.SystemPrompt == "" && role.SystemPromptFile == "" {
        return fmt.Errorf("either system_prompt or system_prompt_file is required")
    }

    // Validate tool names
    for _, tool := range role.AllowedTools {
        if !validTools[tool] {
            return fmt.Errorf("unknown tool in allowed_tools: %q", tool)
        }
    }

    for _, tool := range role.DisallowedTools {
        if !validTools[tool] {
            return fmt.Errorf("unknown tool in disallowed_tools: %q", tool)
        }
    }

    // Check for conflicts
    for _, tool := range role.AllowedTools {
        for _, disallowed := range role.DisallowedTools {
            if tool == disallowed {
                return fmt.Errorf("tool %q appears in both allowed and disallowed lists", tool)
            }
        }
    }

    return nil
}
```

### Directory Structure

```
internal/
├── role/
│   ├── manager.go          # RoleManager implementation
│   ├── builtin.go          # Built-in role definitions (TANK-026)
│   ├── validate.go         # Role validation logic
│   ├── types.go            # Role struct (from TANK-020)
│   ├── manager_test.go
│   └── validate_test.go
```

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Invalid YAML syntax | Error with line number from yaml.v3 |
| Unknown role | Error: "role %q not found" + suggest similar names |
| Missing required fields | Error: "field %q is required" |
| Invalid tool name | Error: "unknown tool %q" + list valid tools |
| Conflicting tools | Error: "tool %q in both allowed and disallowed" |
| System prompt file not found | Error: "system_prompt_file not found: %s" |
| Roles directory doesn't exist | Not an error (just means no custom roles) |
| Invalid project role | Warning (skip it) + continue loading others |

## Testing

### Unit Tests

```go
func TestManager_Get(t *testing.T) {
    tests := []struct {
        name    string
        roleName string
        want    *Role
        wantErr bool
    }{
        {
            name:     "builtin role",
            roleName: "backend",
            want:     &Role{Name: "backend", ...},
            wantErr:  false,
        },
        {
            name:     "unknown role",
            roleName: "nonexistent",
            wantErr:  true,
        },
        // ... more cases
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            m := NewManager(testConfig())
            got, err := m.Get(tt.roleName)
            if (err != nil) != tt.wantErr {
                t.Errorf("Get() error = %v, wantErr %v", err, tt.wantErr)
            }
            // ... assertions
        })
    }
}

func TestManager_LoadProjectRoles(t *testing.T) {
    // Test loading custom roles from .tanuki/roles/
    tmpDir := t.TempDir()
    rolesDir := filepath.Join(tmpDir, ".tanuki", "roles")
    os.MkdirAll(rolesDir, 0755)

    // Create test role file
    roleYAML := `name: custom
description: Custom role
system_prompt: Test prompt
allowed_tools: [Read, Write]
`
    os.WriteFile(filepath.Join(rolesDir, "custom.yaml"), []byte(roleYAML), 0644)

    cfg := &Config{ProjectRoot: tmpDir}
    m := NewManager(cfg)

    role, err := m.Get("custom")
    if err != nil {
        t.Fatalf("Get(custom) failed: %v", err)
    }

    if role.Name != "custom" {
        t.Errorf("expected name=custom, got %s", role.Name)
    }
}

func TestValidate(t *testing.T) {
    // Test validation logic
    // ... test cases for invalid roles
}
```

## Out of Scope

- Role inheritance (one role extends another)
- Role versioning
- Remote role loading (HTTP, git)
- Dynamic role modification at runtime

## Notes

- Caching improves performance when multiple agents use same role
- Project roles override built-in to allow customization
- Validation happens at load time, not at runtime
- Invalid project roles are skipped with warnings (don't break tanuki)
