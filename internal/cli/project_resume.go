package cli

import (
	"fmt"
	"os"

	"github.com/bkonkle/tanuki/internal/state"
	"github.com/bkonkle/tanuki/internal/task"
	"github.com/spf13/cobra"
)

var projectResumeCmd = &cobra.Command{
	Use:   "resume [name]",
	Short: "Resume a stopped project",
	Long: `Restarts stopped agents and reassigns incomplete tasks.

Without a name argument, resumes all projects.

With a name argument, resumes only the specified project.

This command:
  1. Resets in_progress and assigned tasks back to pending
  2. Starts any stopped project agents
  3. Reassigns tasks to available agents

This is useful after running 'tanuki project stop' to continue work.`,
	RunE: runProjectResume,
}

func init() {
	projectCmd.AddCommand(projectResumeCmd)
}

func runProjectResume(_ *cobra.Command, args []string) error {
	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	taskDir := getTasksDir(projectRoot)

	// Check if task directory exists
	if _, statErr := os.Stat(taskDir); os.IsNotExist(statErr) {
		fmt.Println("No tasks found. Run: tanuki project init")
		return nil
	}

	// Use the real task manager
	taskMgr := task.NewManager(&task.Config{ProjectRoot: projectRoot})
	allTasks, err := taskMgr.Scan()
	if err != nil {
		return fmt.Errorf("scan tasks: %w", err)
	}

	if len(allTasks) == 0 {
		fmt.Println("No tasks found.")
		return nil
	}

	// Filter by project name if provided
	var projectName string
	var tasks []*task.Task
	if len(args) > 0 {
		projectName = args[0]
		tasks = taskMgr.GetByProject(projectName)
		if len(tasks) == 0 {
			fmt.Printf("No tasks found for project '%s'.\n", projectName)
			fmt.Println("Run: tanuki project list")
			return nil
		}
		fmt.Printf("Resuming project: %s\n", projectName)
	} else {
		tasks = allTasks
		fmt.Println("Resuming project...")
	}

	// Reset in_progress and assigned tasks to pending
	resetCount := 0
	for _, t := range tasks {
		if t.Status == task.StatusInProgress || t.Status == task.StatusAssigned {
			fmt.Printf("  Resetting %s to pending\n", t.ID)
			_ = taskMgr.Unassign(t.ID)
			_ = taskMgr.UpdateStatus(t.ID, task.StatusPending)
			resetCount++
		}
	}

	if resetCount > 0 {
		fmt.Printf("  Reset %d tasks to pending\n", resetCount)
		fmt.Println()
	}

	// Create agent manager
	agentMgr, err := createAgentManager(projectRoot)
	if err != nil {
		return fmt.Errorf("create agent manager: %w", err)
	}

	// Start stopped agents
	fmt.Println("Starting stopped agents...")
	agents, err := agentMgr.List()
	if err != nil {
		fmt.Printf("  Warning: failed to list agents: %v\n", err)
	} else {
		startedCount := 0
		for _, ag := range agents {
			if ag.Status == state.StatusStopped {
				fmt.Printf("  Starting %s...\n", ag.Name)
				if err := agentMgr.Start(ag.Name); err != nil {
					fmt.Printf("    Failed: %v\n", err)
				} else {
					startedCount++
				}
			}
		}
		if startedCount == 0 {
			fmt.Println("  No stopped agents found")
		} else {
			fmt.Printf("  Started %d agents\n", startedCount)
		}
	}
	fmt.Println()

	// Delegate to start for task assignment
	fmt.Println("Reassigning tasks...")
	fmt.Println("  Run 'tanuki project start' to assign tasks to agents")

	fmt.Println()
	fmt.Println("Project resumed!")
	if projectName != "" {
		fmt.Printf("Monitor with: tanuki project status %s\n", projectName)
	} else {
		fmt.Println("Monitor with: tanuki project status")
	}

	return nil
}
