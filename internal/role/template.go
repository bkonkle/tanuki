package role

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// roleTemplate is the template for generating new role files.
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

// indentFunc indents each line of a string with the given prefix.
func indentFunc(s string, prefix string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = prefix + line
		}
	}
	return strings.Join(lines, "\n")
}

// WriteRoleTemplate writes a role template file to the specified path.
func WriteRoleTemplate(path string, role *Role) error {
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
		"indent": indentFunc,
	}).Parse(roleTemplate))

	if err := tmpl.Execute(f, role); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	return nil
}

// DefaultTemplate creates a default role template with the given name.
func DefaultTemplate(name string) *Role {
	// Capitalize first letter for description
	description := name
	if len(description) > 0 {
		description = strings.ToUpper(description[:1]) + description[1:]
	}
	description = strings.ReplaceAll(description, "-", " ") + " specialist"

	return &Role{
		Name:            name,
		Description:     description,
		SystemPrompt:    "",
		AllowedTools:    []string{"Read", "Write", "Edit", "Bash", "Glob", "Grep", "TodoWrite"},
		DisallowedTools: []string{},
		ContextFiles:    []string{},
		Builtin:         false,
	}
}

// Clone creates a deep copy of a role.
func (r *Role) Clone() *Role {
	clone := *r
	clone.AllowedTools = append([]string(nil), r.AllowedTools...)
	clone.DisallowedTools = append([]string(nil), r.DisallowedTools...)
	clone.ContextFiles = append([]string(nil), r.ContextFiles...)
	return &clone
}
