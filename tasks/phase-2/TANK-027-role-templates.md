---
id: TANK-027
title: Role Template Generation
status: done
priority: medium
estimate: S
depends_on: [TANK-023]
workstream: C
phase: 2
---

# Role Template Generation

## Summary

Add command to generate custom role templates that users can modify. Makes it easy to create new roles by providing a starting point based on existing roles or a default template.

## Acceptance Criteria

- [ ] `tanuki role create <name>` generates role template
- [ ] Template includes all fields with helpful comments
- [ ] Option to base on existing role (`--based-on backend`)
- [ ] Creates role file in `.tanuki/roles/<name>.yaml`
- [ ] Helpful comments explaining each field
- [ ] Interactive prompts for common fields (optional)
- [ ] Error if role already exists

## Technical Details

### Command Implementation

```go
// internal/cli/role.go

var roleCreateCmd = &cobra.Command{
    Use:   "create <name>",
    Short: "Create a new role from template",
    Long: `Create a new custom role from a template.

Examples:
  # Create a blank role template
  tanuki role create ml-engineer

  # Base new role on existing role
  tanuki role create ml-engineer --based-on backend

  # Create with description
  tanuki role create ml-engineer --description "ML model development"
`,
    Args: cobra.ExactArgs(1),
    RunE: runRoleCreate,
}

func init() {
    roleCreateCmd.Flags().String("based-on", "", "Base new role on existing role")
    roleCreateCmd.Flags().StringP("description", "d", "", "Role description")
    roleCreateCmd.Flags().Bool("interactive", false, "Interactive mode with prompts")
    roleCmd.AddCommand(roleCreateCmd)
}

func runRoleCreate(cmd *cobra.Command, args []string) error {
    name := args[0]
    basedOn, _ := cmd.Flags().GetString("based-on")
    description, _ := cmd.Flags().GetString("description")
    interactive, _ := cmd.Flags().GetBool("interactive")

    // Validate role name
    if !isValidRoleName(name) {
        return fmt.Errorf("invalid role name: %q (must be lowercase alphanumeric with hyphens)", name)
    }

    cfg, err := config.Load()
    if err != nil {
        return fmt.Errorf("load config: %w", err)
    }

    roleMgr := role.NewManager(cfg)

    // Check if role already exists
    if _, err := roleMgr.Get(name); err == nil {
        return fmt.Errorf("role %q already exists\nUse 'tanuki role show %s' to view it", name, name)
    }

    var template *role.Role

    if basedOn != "" {
        // Load base role
        base, err := roleMgr.Get(basedOn)
        if err != nil {
            return fmt.Errorf("base role %q not found\nUse 'tanuki role list' to see available roles", basedOn)
        }

        template = base.Clone()
        template.Name = name
        template.Builtin = false
        fmt.Printf("Creating role %q based on %q\n", name, basedOn)
    } else {
        // Create default template
        template = role.DefaultTemplate(name)
        fmt.Printf("Creating role %q\n", name)
    }

    // Override description if provided
    if description != "" {
        template.Description = description
    }

    // Interactive mode
    if interactive {
        if err := promptRoleDetails(template); err != nil {
            return fmt.Errorf("interactive setup: %w", err)
        }
    }

    // Write role file
    roleFile := filepath.Join(cfg.ProjectRoot, ".tanuki", "roles", name+".yaml")
    if err := writeRoleTemplate(roleFile, template); err != nil {
        return fmt.Errorf("write role file: %w", err)
    }

    fmt.Printf("\n✓ Created role file: %s\n", roleFile)
    fmt.Println("\nNext steps:")
    fmt.Println("  1. Edit the role file to customize behavior:")
    fmt.Printf("     vim %s\n", roleFile)
    fmt.Println("  2. Use with: tanuki spawn <agent> --role", name)
    fmt.Println("\nSee 'docs/roles.md' for role creation guide.")

    return nil
}

func isValidRoleName(name string) bool {
    // Must be lowercase alphanumeric with hyphens
    matched, _ := regexp.MatchString(`^[a-z0-9-]+$`, name)
    return matched
}
```

### Template Writing

```go
// internal/role/template.go
package role

import (
    "fmt"
    "os"
    "text/template"
)

const roleTemplate = `# Role: {{.Name}}
#
# Description: {{.Description}}
#
# This role defines the behavior and capabilities of agents spawned with it.
# Customize the system prompt, tools, and context files as needed.
#
# See docs/roles.md for detailed role creation guide.

name: {{.Name}}
description: {{.Description}}

# System prompt appended to Claude Code's default instructions
# Be specific about what this role focuses on, best practices, and what to avoid
system_prompt: |
{{if .SystemPrompt}}{{indent .SystemPrompt "  "}}{{else}}  You are a {{.Name}} specialist.

  ## Responsibilities
  - List key responsibilities here
  - Be specific about what this role does

  ## Best Practices
  - List important best practices
  - Include dos and don'ts

  ## Before Starting
  Review relevant project documentation to understand context.
{{end}}

# Alternative: Load system prompt from external file
# system_prompt_file: .tanuki/prompts/{{.Name}}.md

# Tools this role can use
# Available: Read, Write, Edit, Bash, Glob, Grep, TodoWrite, Task, WebFetch, WebSearch
allowed_tools:
{{if .AllowedTools}}{{range .AllowedTools}}  - {{.}}
{{end}}{{else}}  - Read
  - Write
  - Edit
  - Bash
  - Glob
  - Grep
  - TodoWrite
{{end}}

# Tools explicitly denied (optional)
# Use to restrict dangerous operations for this role
{{if .DisallowedTools}}disallowed_tools:
{{range .DisallowedTools}}  - {{.}}
{{end}}{{else}}# disallowed_tools: []
{{end}}

# Context files to provide to agents (supports globs)
# These files are copied/symlinked to .tanuki/context/ in the agent's worktree
{{if .ContextFiles}}context_files:
{{range .ContextFiles}}  - {{.}}
{{end}}{{else}}# context_files:
#   - docs/architecture.md
#   - docs/**/*.md
#   - CONTRIBUTING.md
{{end}}

# Model override (optional)
# Uncomment to use a specific Claude model for this role
# model: claude-sonnet-4-5-20250514

# Resource overrides (optional)
# Uncomment to customize container resources
# resources:
#   memory: "8g"
#   cpus: "4"

# Max turns override (optional)
# Maximum conversation turns before stopping
# max_turns: 100
`

func writeRoleTemplate(path string, role *Role) error {
    // Ensure directory exists
    dir := filepath.Dir(path)
    if err := os.MkdirAll(dir, 0755); err != nil {
        return fmt.Errorf("create directory: %w", err)
    }

    // Create file
    f, err := os.Create(path)
    if err != nil {
        return fmt.Errorf("create file: %w", err)
    }
    defer f.Close()

    // Execute template
    tmpl := template.Must(template.New("role").Funcs(template.FuncMap{
        "indent": func(s string, prefix string) string {
            lines := strings.Split(s, "\n")
            for i, line := range lines {
                if line != "" {
                    lines[i] = prefix + line
                }
            }
            return strings.Join(lines, "\n")
        },
    }).Parse(roleTemplate))

    if err := tmpl.Execute(f, role); err != nil {
        return fmt.Errorf("execute template: %w", err)
    }

    return nil
}

func DefaultTemplate(name string) *Role {
    return &Role{
        Name:         name,
        Description:  fmt.Sprintf("%s specialist", strings.Title(name)),
        SystemPrompt: "",
        AllowedTools: []string{
            "Read", "Write", "Edit", "Bash",
            "Glob", "Grep", "TodoWrite",
        },
        DisallowedTools: []string{},
        ContextFiles:    []string{},
        Builtin:         false,
    }
}

func (r *Role) Clone() *Role {
    clone := *r
    clone.AllowedTools = append([]string(nil), r.AllowedTools...)
    clone.DisallowedTools = append([]string(nil), r.DisallowedTools...)
    clone.ContextFiles = append([]string(nil), r.ContextFiles...)
    return &clone
}
```

### Interactive Mode (Optional Enhancement)

```go
func promptRoleDetails(role *Role) error {
    reader := bufio.NewReader(os.Stdin)

    // Prompt for description if empty
    if role.Description == "" || role.Description == role.Name+" specialist" {
        fmt.Print("\nEnter role description: ")
        desc, _ := reader.ReadString('\n')
        role.Description = strings.TrimSpace(desc)
    }

    // Prompt for focus areas
    fmt.Print("\nWhat should this role focus on? (comma-separated): ")
    focus, _ := reader.ReadString('\n')
    if focus := strings.TrimSpace(focus); focus != "" {
        // Add to system prompt
        areas := strings.Split(focus, ",")
        prompt := fmt.Sprintf("You are a %s specialist.\n\nFocus on:\n", role.Name)
        for _, area := range areas {
            prompt += fmt.Sprintf("- %s\n", strings.TrimSpace(area))
        }
        role.SystemPrompt = prompt
    }

    // Ask about tool restrictions
    fmt.Print("\nShould this role be read-only? (y/n): ")
    readOnly, _ := reader.ReadString('\n')
    if strings.TrimSpace(strings.ToLower(readOnly)) == "y" {
        role.AllowedTools = []string{"Read", "Bash", "Glob", "Grep", "TodoWrite"}
        role.DisallowedTools = []string{"Write", "Edit"}
    }

    return nil
}
```

## Output Examples

### Default Template

```bash
$ tanuki role create ml-engineer
Creating role "ml-engineer"

✓ Created role file: .tanuki/roles/ml-engineer.yaml

Next steps:
  1. Edit the role file to customize behavior:
     vim .tanuki/roles/ml-engineer.yaml
  2. Use with: tanuki spawn <agent> --role ml-engineer

See 'docs/roles.md' for role creation guide.
```

Generated file:
```yaml
# Role: ml-engineer
#
# Description: Ml-engineer specialist
#
# This role defines the behavior and capabilities of agents spawned with it.
# ...

name: ml-engineer
description: Ml-engineer specialist

system_prompt: |
  You are a ml-engineer specialist.

  ## Responsibilities
  - List key responsibilities here
  - Be specific about what this role does

  ## Best Practices
  - List important best practices
  - Include dos and don'ts

  ## Before Starting
  Review relevant project documentation to understand context.

allowed_tools:
  - Read
  - Write
  - Edit
  - Bash
  - Glob
  - Grep
  - TodoWrite

# disallowed_tools: []

# context_files:
#   - docs/architecture.md
#   - docs/**/*.md
#   - CONTRIBUTING.md
```

### Based on Existing Role

```bash
$ tanuki role create api-specialist --based-on backend --description "API design expert"
Creating role "api-specialist" based on "backend"

✓ Created role file: .tanuki/roles/api-specialist.yaml

Next steps:
  1. Edit the role file to customize behavior:
     vim .tanuki/roles/api-specialist.yaml
  2. Use with: tanuki spawn <agent> --role api-specialist

See 'docs/roles.md' for role creation guide.
```

Generated file inherits backend's system prompt, tools, and context files but with new name.

### Interactive Mode

```bash
$ tanuki role create data-engineer --interactive
Creating role "data-engineer"

Enter role description: Data pipeline and ETL specialist

What should this role focus on? (comma-separated): ETL pipelines, data quality, SQL optimization

Should this role be read-only? (y/n): n

✓ Created role file: .tanuki/roles/data-engineer.yaml
...
```

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Role already exists | Error: "role %q already exists" |
| Invalid role name | Error: "invalid role name" (must be lowercase-hyphen) |
| Base role not found | Error: "base role %q not found" + suggest list |
| Cannot write file | Error with file path and reason |

## Testing

```go
func TestRoleCreate(t *testing.T) {
    tmpDir := t.TempDir()
    cfg := &Config{ProjectRoot: tmpDir}

    tests := []struct {
        name        string
        roleName    string
        basedOn     string
        description string
        wantErr     bool
    }{
        {
            name:        "default template",
            roleName:    "custom",
            description: "Custom role",
            wantErr:     false,
        },
        {
            name:     "based on backend",
            roleName: "api-specialist",
            basedOn:  "backend",
            wantErr:  false,
        },
        {
            name:     "invalid name",
            roleName: "Invalid_Name",
            wantErr:  true,
        },
        {
            name:     "unknown base",
            roleName: "custom",
            basedOn:  "nonexistent",
            wantErr:  true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}

func TestWriteRoleTemplate(t *testing.T) {
    tmpDir := t.TempDir()
    roleFile := filepath.Join(tmpDir, "test-role.yaml")

    role := &Role{
        Name:        "test",
        Description: "Test role",
        SystemPrompt: "Test prompt",
        AllowedTools: []string{"Read", "Write"},
    }

    err := writeRoleTemplate(roleFile, role)
    if err != nil {
        t.Fatalf("writeRoleTemplate failed: %v", err)
    }

    // Verify file exists and is valid YAML
    data, err := os.ReadFile(roleFile)
    if err != nil {
        t.Fatalf("read file: %v", err)
    }

    var parsed Role
    if err := yaml.Unmarshal(data, &parsed); err != nil {
        t.Fatalf("invalid YAML: %v", err)
    }

    if parsed.Name != "test" {
        t.Errorf("name = %q, want %q", parsed.Name, "test")
    }
}
```

## Documentation

Update `docs/roles.md`:

```markdown
## Creating Custom Roles

### Quick Start

```bash
# Create a new role from scratch
tanuki role create my-role

# Base on existing role
tanuki role create my-role --based-on backend

# With description
tanuki role create my-role --description "My custom specialist"
```

### Role Template

The generated template includes:
- System prompt with guidelines
- Tool permissions
- Context files to include
- Model and resource overrides

### Example: Creating a Data Engineer Role

```bash
$ tanuki role create data-engineer --based-on backend
$ vim .tanuki/roles/data-engineer.yaml
```

Customize the system prompt:

```yaml
system_prompt: |
  You are a data engineering specialist.

  ## Focus Areas
  - ETL pipeline development
  - Data quality and validation
  - SQL query optimization
  - Data warehousing

  ## Tools
  - SQL databases (Postgres, MySQL, etc.)
  - Data processing (Pandas, dbt, etc.)
  - Orchestration (Airflow, Dagster, etc.)

  ## Best Practices
  - Write idempotent data pipelines
  - Add data quality checks
  - Optimize for large datasets
  - Document data schemas
```

Use the custom role:

```bash
tanuki spawn etl-worker --role data-engineer
tanuki run etl-worker "Create ETL pipeline for user events"
```
```

## Out of Scope

- Role validation command (validation happens on load)
- Role deletion command (users delete files directly)
- Role marketplace/sharing
- Automatic role generation from codebase analysis

## Notes

- Template includes extensive comments to guide customization
- Based-on feature makes it easy to create role variants
- Interactive mode is optional enhancement (can be added later)
- Role files are plain YAML - users can edit directly
- Invalid role names are caught early with clear error
