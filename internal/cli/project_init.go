package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var projectInitCmd = &cobra.Command{
	Use:   "init <name>",
	Short: "Initialize a project for task management",
	Long: `Creates a project folder within tasks/ for organizing work.

Example: "tanuki project init auth-feature" creates:
  tasks/
    README.md                                  # Tasks directory overview
    auth-feature/
      README.md                                # Project context and goals
      001-main-example-task.md                 # Example task file

Each project is a linear record of tasks that drive workstream-based agents and
document incremental decisions and specifications over time.

The tasks directory location is configurable via tasks_dir in tanuki.yaml
(defaults to "tasks").`,
	Args: func(_ *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("project name required\n\nUsage: tanuki project init <name>\n\nExample: tanuki project init auth-feature")
		}
		if len(args) > 1 {
			return fmt.Errorf("too many arguments - expected project name only")
		}
		return nil
	},
	RunE: runProjectInit,
}

func init() {
	projectCmd.AddCommand(projectInitCmd)
}

func runProjectInit(_ *cobra.Command, args []string) error {
	// Get current working directory as project root
	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	taskDir := getTasksDir(projectRoot)

	// Ensure base task directory exists
	if err := os.MkdirAll(taskDir, 0750); err != nil {
		return fmt.Errorf("create task directory: %w", err)
	}

	// Create top-level tasks README if it doesn't exist
	if err := ensureTasksReadme(taskDir); err != nil {
		return fmt.Errorf("create tasks README: %w", err)
	}

	// Create the project folder
	return initProjectFolder(taskDir, args[0])
}

// initProjectFolder creates a project subfolder with README.md and example task.
func initProjectFolder(taskDir, projectName string) error {
	projectPath := filepath.Join(taskDir, projectName)

	// Check if project already exists
	readmePath := filepath.Join(projectPath, "README.md")
	if _, err := os.Stat(readmePath); err == nil {
		fmt.Printf("Project '%s' already initialized.\n", projectName)
		fmt.Printf("  Tasks:   %s\n", projectPath)
		return nil
	}

	// Create project directory
	if err := os.MkdirAll(projectPath, 0750); err != nil {
		return fmt.Errorf("create project directory: %w", err)
	}

	// Create README.md
	projectDoc := fmt.Sprintf(`# Project: %s

Brief project description. Update this with your project's overview.

## Architecture

Describe key components and their relationships here.

## Conventions

- Code style guidelines
- Testing requirements
- Documentation standards

## Context Files

Files agents should understand:
- README.md
- CLAUDE.md (if exists)
- Any architecture documentation
`, projectName)

	if err := os.WriteFile(readmePath, []byte(projectDoc), 0600); err != nil {
		return fmt.Errorf("write README.md: %w", err)
	}

	// Create example task with workstream
	exampleTask := fmt.Sprintf(`---
id: %s-001
title: Example Task
workstream: main
priority: medium
status: pending
depends_on: []

completion:
  verify: "echo 'Task complete'"
  signal: "TASK_DONE"
---

# Example Task

This is an example task file. Replace this with your actual task.

## Requirements

1. First requirement
2. Second requirement

## Done When

- All requirements are implemented
- Tests pass
- Say TASK_DONE when finished
`, projectName)

	examplePath := filepath.Join(projectPath, "001-main-example-task.md")
	if err := os.WriteFile(examplePath, []byte(exampleTask), 0600); err != nil {
		return fmt.Errorf("write example task: %w", err)
	}

	fmt.Printf("Initialized project '%s'\n", projectName)
	fmt.Printf("  Tasks:   %s\n", projectPath)
	fmt.Printf("  Project: %s\n", readmePath)
	fmt.Printf("  Example: %s\n", examplePath)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  1. Edit %s with your project description\n", readmePath)
	fmt.Printf("  2. Create task files in %s/\n", projectPath)
	fmt.Printf("  3. Run: tanuki project status %s\n", projectName)
	fmt.Printf("  4. Run: tanuki project start %s\n", projectName)

	return nil
}

// ensureTasksReadme creates the top-level tasks/README.md if it doesn't exist.
func ensureTasksReadme(taskDir string) error {
	readmePath := filepath.Join(taskDir, "README.md")
	if _, err := os.Stat(readmePath); err == nil {
		return nil // Already exists
	}

	tasksReadme := `# Tasks

This directory contains project folders for task-driven development with Tanuki.

## Purpose

Each project folder serves as a **linear, historical record** of work:

- **Driving agents**: Tasks define work for agents that execute in parallel
- **Documenting decisions**: Each task captures requirements, context, and completion criteria
- **Tracking evolution**: The sequence of tasks shows how a project evolved over time

## Structure

` + "```" + `
tasks/
  README.md                              # This file
  project-name/
    README.md                            # Project overview and context
    001-backend-main-setup.md            # First task
    002-frontend-ui-dashboard.md         # Second task
    ...
` + "```" + `

## Task File Format

Tasks use YAML front matter with markdown content:

` + "```" + `markdown
---
id: project-001
title: Task Title
workstream: main       # Groups related sequential work
priority: high         # critical, high, medium, low
status: pending        # pending, assigned, in_progress, complete, failed
depends_on: []         # Task IDs that must complete first
completion:
  verify: "npm test"   # Command to verify completion
  signal: "TASK_DONE"  # Signal agent outputs when done
---

# Task Title

Task description and requirements...
` + "```" + `

## Commands

- ` + "`tanuki project init <name>`" + ` - Create a new project folder
- ` + "`tanuki project status [name]`" + ` - Show project and task status
- ` + "`tanuki project start [name]`" + ` - Start agents for pending tasks
- ` + "`tanuki project stop`" + ` - Stop running agents
`

	return os.WriteFile(readmePath, []byte(tasksReadme), 0600)
}
