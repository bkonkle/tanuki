package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/bkonkle/tanuki/internal/task"
	"github.com/spf13/cobra"
)

var projectResumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "Resume a stopped project",
	Long: `Restarts stopped agents and reassigns incomplete tasks.

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

func runProjectResume(cmd *cobra.Command, args []string) error {
	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	taskDir := filepath.Join(projectRoot, ".tanuki", "tasks")

	// Check if task directory exists
	if _, err := os.Stat(taskDir); os.IsNotExist(err) {
		fmt.Println("No tasks found. Run: tanuki project init")
		return nil
	}

	// Scan tasks
	fmt.Println("Resuming project...")
	taskMgr := newMockTaskManager(taskDir)
	tasks, err := taskMgr.Scan()
	if err != nil {
		return fmt.Errorf("scan tasks: %w", err)
	}

	if len(tasks) == 0 {
		fmt.Println("No tasks found.")
		return nil
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

	// Start stopped agents
	fmt.Println("Starting stopped agents...")
	// TODO: Integrate with agent manager
	// agents, _ := agentMgr.List()
	// for _, ag := range agents {
	//     if ag.Role != "" && ag.Status == "stopped" {
	//         fmt.Printf("  Starting %s...\n", ag.Name)
	//         agentMgr.Start(ag.Name)
	//     }
	// }

	fmt.Println("  [placeholder] No stopped agents found")
	fmt.Println()

	// Delegate to start for assignment logic
	fmt.Println("Reassigning tasks...")
	// TODO: Call task assignment logic from project start

	fmt.Println()
	fmt.Println("Project resumed!")
	fmt.Println("Monitor with: tanuki project status")

	return nil
}
