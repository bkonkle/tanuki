package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var projectInitCmd = &cobra.Command{
	Use:   "init [name]",
	Short: "Initialize project task structure",
	Long: `Creates tasks/ directory and project structure.

Without a name argument, creates flat task structure in tasks/ with an example task.

With a name argument (e.g., "tanuki project init auth-feature"), creates a project
folder structure:
  tasks/
    auth-feature/
      project.md       # Project context and goals
      001-task.md      # Example task file

The tasks directory location is configurable via tasks_dir in tanuki.yaml
(defaults to "tasks").`,
	RunE: runProjectInit,
}

func init() {
	projectCmd.AddCommand(projectInitCmd)
}

func runProjectInit(cmd *cobra.Command, args []string) error {
	// Get current working directory as project root
	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	taskDir := getTasksDir(projectRoot)

	// Ensure base task directory exists
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		return fmt.Errorf("create task directory: %w", err)
	}

	// If a project name is provided, create a project subfolder
	if len(args) > 0 {
		return initProjectFolder(taskDir, args[0])
	}

	// Otherwise, create flat structure (backward compatible)
	return initFlatStructure(taskDir, projectRoot)
}

// initProjectFolder creates a project subfolder with project.md and example task.
func initProjectFolder(taskDir, projectName string) error {
	projectPath := filepath.Join(taskDir, projectName)

	// Check if project already exists
	projectMdPath := filepath.Join(projectPath, "project.md")
	if _, err := os.Stat(projectMdPath); err == nil {
		fmt.Printf("Project '%s' already initialized.\n", projectName)
		fmt.Printf("Project directory: %s\n", projectPath)
		return nil
	}

	// Create project directory
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		return fmt.Errorf("create project directory: %w", err)
	}

	// Create project.md
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

	if err := os.WriteFile(projectMdPath, []byte(projectDoc), 0644); err != nil {
		return fmt.Errorf("write project.md: %w", err)
	}

	// Create example task with workstream
	exampleTask := fmt.Sprintf(`---
id: %s-001
title: Example Task
role: backend
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

	examplePath := filepath.Join(projectPath, "001-example.md")
	if err := os.WriteFile(examplePath, []byte(exampleTask), 0644); err != nil {
		return fmt.Errorf("write example task: %w", err)
	}

	fmt.Printf("Initialized project '%s'\n", projectName)
	fmt.Printf("  Project:  %s\n", projectPath)
	fmt.Printf("  Metadata: %s\n", projectMdPath)
	fmt.Printf("  Example:  %s\n", examplePath)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  1. Edit %s with your project description\n", projectMdPath)
	fmt.Printf("  2. Create task files in %s/\n", projectPath)
	fmt.Printf("  3. Run: tanuki project status %s\n", projectName)
	fmt.Printf("  4. Run: tanuki project start %s\n", projectName)

	return nil
}

// initFlatStructure creates the flat task structure (backward compatible).
func initFlatStructure(taskDir, projectRoot string) error {
	// Check if already initialized (has project.md in tasks/)
	projectPath := filepath.Join(taskDir, "project.md")
	if _, err := os.Stat(projectPath); err == nil {
		fmt.Println("Project tasks already initialized.")
		fmt.Printf("Task directory: %s\n", taskDir)
		return nil
	}

	projectName := filepath.Base(projectRoot)
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

	if err := os.WriteFile(projectPath, []byte(projectDoc), 0644); err != nil {
		return fmt.Errorf("write project.md: %w", err)
	}

	// Create example task with workstream
	exampleTask := `---
id: TASK-001
title: Example Task
role: backend
workstream: example-feature
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
`

	examplePath := filepath.Join(taskDir, "TASK-001-example.md")
	if err := os.WriteFile(examplePath, []byte(exampleTask), 0644); err != nil {
		return fmt.Errorf("write example task: %w", err)
	}

	fmt.Println("Initialized project structure")
	fmt.Printf("  Tasks:   %s\n", taskDir)
	fmt.Printf("  Project: %s\n", projectPath)
	fmt.Printf("  Example: %s\n", examplePath)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  1. Edit %s with your project description\n", projectPath)
	fmt.Printf("  2. Create task files in %s/\n", taskDir)
	fmt.Println("  3. Run: tanuki project status")
	fmt.Println("  4. Run: tanuki project start")

	return nil
}
