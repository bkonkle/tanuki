package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var projectInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize project task structure",
	Long: `Creates .tanuki/tasks/ directory, project.md, and example task file.

This sets up the task directory structure for project mode. You can then
create task files in .tanuki/tasks/ to define work items for agents.`,
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

	tanukiDir := filepath.Join(projectRoot, ".tanuki")
	taskDir := filepath.Join(tanukiDir, "tasks")

	// Check if already initialized
	if _, err := os.Stat(taskDir); err == nil {
		fmt.Println("Project tasks already initialized.")
		fmt.Printf("Task directory: %s\n", taskDir)
		return nil
	}

	// Create task directory
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		return fmt.Errorf("create task directory: %w", err)
	}

	// Create project.md
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

	projectPath := filepath.Join(tanukiDir, "project.md")
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
	fmt.Printf("  Created: %s\n", tanukiDir)
	fmt.Printf("  Project: %s\n", projectPath)
	fmt.Printf("  Tasks:   %s\n", taskDir)
	fmt.Printf("  Example: %s\n", examplePath)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Edit .tanuki/project.md with your project description")
	fmt.Println("  2. Create task files in .tanuki/tasks/")
	fmt.Println("  3. Run: tanuki project status")
	fmt.Println("  4. Run: tanuki project start")

	return nil
}
