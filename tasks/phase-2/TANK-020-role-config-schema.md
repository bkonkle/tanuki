---
id: TANK-020
title: Role Configuration Schema
status: done
priority: high
estimate: M
depends_on: [TANK-002]
workstream: A
phase: 2
---

# Role Configuration Schema

## Summary

Define and implement the YAML schema for role definitions. Roles allow spawning agents with pre-configured system prompts, allowed tools, and context files.

## Acceptance Criteria

- [ ] YAML schema for role definitions
- [ ] Roles stored in `.tanuki/roles/` directory
- [ ] Role validation on load
- [ ] Built-in default roles (backend, frontend, qa)
- [ ] Custom roles per project

## Technical Details

### Role Schema

```yaml
# .tanuki/roles/backend.yaml
name: backend
description: Backend development specialist

# System prompt appended to Claude Code's default
system_prompt: |
  You are a backend development specialist. Focus on:
  - API design and implementation
  - Database operations and optimization
  - Server-side business logic
  - Security best practices

  Always write tests for new functionality.
  Follow the project's existing patterns and conventions.

# Or load from file
# system_prompt_file: .tanuki/prompts/backend.md

# Tools this role is allowed to use
allowed_tools:
  - Read
  - Write
  - Edit
  - Bash
  - Glob
  - Grep
  - TodoWrite

# Tools explicitly denied
disallowed_tools: []

# Files to include as context (relative to project root)
context_files:
  - docs/architecture.md
  - docs/api-conventions.md
  - CONTRIBUTING.md

# Model override (optional)
model: claude-sonnet-4-5-20250514

# Resource limits override (optional)
resources:
  memory: "8g"
  cpus: "4"

# Max conversation turns override (optional)
max_turns: 100
```

### Built-in Roles

```yaml
# Default: backend role
name: backend
description: Backend development specialist
system_prompt: |
  You are a backend development specialist...

# Default: frontend role
name: frontend
description: Frontend development specialist
system_prompt: |
  You are a frontend development specialist. Focus on:
  - UI components and styling
  - State management
  - User experience
  - Accessibility

# Default: qa role
name: qa
description: Quality assurance specialist
system_prompt: |
  You are a QA specialist. Focus on:
  - Writing comprehensive tests
  - Finding edge cases
  - Ensuring code quality
  - Running existing test suites
allowed_tools:
  - Read
  - Bash
  - Glob
  - Grep
# Note: No Write/Edit - QA only verifies, doesn't modify

# Default: docs role
name: docs
description: Documentation specialist
system_prompt: |
  You are a documentation specialist. Focus on:
  - Writing clear documentation
  - Updating READMEs
  - API documentation
  - Code comments where needed
```

### Go Implementation

```go
type Role struct {
    Name             string            `yaml:"name"`
    Description      string            `yaml:"description"`
    SystemPrompt     string            `yaml:"system_prompt"`
    SystemPromptFile string            `yaml:"system_prompt_file"`
    AllowedTools     []string          `yaml:"allowed_tools"`
    DisallowedTools  []string          `yaml:"disallowed_tools"`
    ContextFiles     []string          `yaml:"context_files"`
    Model            string            `yaml:"model"`
    Resources        *ResourceConfig   `yaml:"resources"`
    MaxTurns         int               `yaml:"max_turns"`
}

type RoleManager interface {
    // List all available roles
    List() ([]*Role, error)

    // Get a specific role by name
    Get(name string) (*Role, error)

    // Load role from file
    LoadFromFile(path string) (*Role, error)

    // Get built-in roles
    GetBuiltinRoles() []*Role

    // Create role directory structure
    InitRoles() error
}
```

### Directory Structure

```
.tanuki/
├── roles/
│   ├── backend.yaml
│   ├── frontend.yaml
│   └── custom-role.yaml
├── prompts/
│   └── backend-detailed.md  # Optional: longer prompts in files
└── ...
```

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Invalid YAML | Clear error with line number |
| Unknown fields | Warning, continue |
| Missing required fields | Error with field name |
| File not found | Error with path |

## Out of Scope

- Role inheritance
- Dynamic role modification
- Role versioning

## Notes

Keep roles simple and focused. A role should represent a clear specialization, not a complex configuration.
