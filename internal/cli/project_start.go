package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bkonkle/tanuki/internal/task"
	"github.com/spf13/cobra"
)

var projectStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start agents and assign tasks",
	Long: `Spawns agents for each role needed and assigns pending tasks.

By default, spawns one agent per role. Use --agents-per-role to spawn more.

This command:
  1. Scans .tanuki/tasks/ for task files
  2. Determines which roles are needed
  3. Spawns agents for each role
  4. Assigns pending tasks to idle agents
  5. Starts task execution

Use --dry-run to see what would happen without making changes.`,
	RunE: runProjectStart,
}

func init() {
	projectStartCmd.Flags().IntP("agents-per-role", "n", 1, "Number of agents per role")
	projectStartCmd.Flags().Bool("dry-run", false, "Show what would happen without doing it")
	projectCmd.AddCommand(projectStartCmd)
}

func runProjectStart(cmd *cobra.Command, args []string) error {
	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	agentsPerRole, _ := cmd.Flags().GetInt("agents-per-role")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	taskDir := filepath.Join(projectRoot, ".tanuki", "tasks")

	// Check if task directory exists
	if _, err := os.Stat(taskDir); os.IsNotExist(err) {
		fmt.Println("No tasks found. Run: tanuki project init")
		return nil
	}

	// Scan tasks
	fmt.Println("Scanning tasks...")
	taskMgr := newMockTaskManager(taskDir)
	tasks, err := taskMgr.Scan()
	if err != nil {
		return fmt.Errorf("scan tasks: %w", err)
	}

	if len(tasks) == 0 {
		fmt.Println("No tasks found. Run: tanuki project init")
		return nil
	}

	// Determine roles needed
	rolesNeeded := make(map[string]int)
	pendingTasks := 0
	for _, t := range tasks {
		if t.Status == task.StatusPending || t.Status == task.StatusBlocked {
			rolesNeeded[t.Role]++
			pendingTasks++
		}
	}

	if pendingTasks == 0 {
		fmt.Println("No pending tasks.")
		return nil
	}

	fmt.Printf("  Found %d tasks across %d roles\n", len(tasks), len(rolesNeeded))
	fmt.Println()

	if dryRun {
		fmt.Println("[DRY RUN] Would spawn:")
		for role, count := range rolesNeeded {
			fmt.Printf("  %s-agent (role: %s) - %d tasks\n", role, role, count)
		}
		return nil
	}

	// Spawn agents for each role
	fmt.Println("Spawning agents...")
	for role := range rolesNeeded {
		for i := 0; i < agentsPerRole; i++ {
			agentName := role + "-agent"
			if agentsPerRole > 1 {
				agentName = fmt.Sprintf("%s-agent-%d", role, i+1)
			}

			// In the real implementation, this will use the agent manager
			// For now, we output what would happen
			fmt.Printf("  Spawning %s (role: %s)...\n", agentName, role)

			// TODO: Integrate with agent.Manager
			// existing, _ := agentMgr.Get(agentName)
			// if existing != nil {
			//     if existing.Status == "stopped" {
			//         fmt.Printf("  Starting existing agent %s...\n", agentName)
			//         agentMgr.Start(agentName)
			//     } else {
			//         fmt.Printf("  Agent %s already running\n", agentName)
			//     }
			//     continue
			// }
			//
			// _, err := agentMgr.Spawn(agentName, agent.SpawnOptions{Role: role})
			// if err != nil {
			//     fmt.Printf("    Failed: %v\n", err)
			//     continue
			// }

			fmt.Printf("  [placeholder] %s\n", agentName)
		}
	}
	fmt.Println()

	// Assign tasks to idle agents
	fmt.Println("Assigning tasks...")
	assigned := 0

	// Get pending tasks for each role
	for role := range rolesNeeded {
		roleTasks := taskMgr.GetByRole(role)
		for _, t := range roleTasks {
			if t.Status != task.StatusPending {
				continue
			}

			// Check dependencies
			blocked, _ := taskMgr.IsBlocked(t.ID)
			if blocked {
				fmt.Printf("  %s blocked (waiting on dependencies)\n", t.ID)
				_ = taskMgr.UpdateStatus(t.ID, task.StatusBlocked)
				continue
			}

			// TODO: Get idle agent for role and assign
			agentName := fmt.Sprintf("%s-agent", role)
			fmt.Printf("  %s -> %s\n", t.ID, agentName)

			// Update task status
			_ = taskMgr.Assign(t.ID, agentName)
			assigned++

			// Build prompt and start task (placeholder)
			// prompt := buildTaskPrompt(t)
			// go agentMgr.Run(agentName, prompt, agent.RunOptions{})

			// Only assign one task per agent for now
			break
		}
	}

	if assigned == 0 {
		fmt.Println("  No tasks assigned (all agents busy or no matching tasks)")
	}

	fmt.Println()
	fmt.Println("Project started!")
	fmt.Println("Monitor with: tanuki project status")

	return nil
}

// buildTaskPrompt creates the prompt to send to an agent for a task.
func buildTaskPrompt(t *task.Task) string {
	var prompt strings.Builder

	prompt.WriteString(fmt.Sprintf("# Task: %s\n\n", t.Title))
	prompt.WriteString(t.Content)

	if t.Completion != nil {
		prompt.WriteString("\n\n## Completion Criteria\n\n")
		if t.Completion.Verify != "" {
			prompt.WriteString(fmt.Sprintf("Run this command to verify: `%s`\n", t.Completion.Verify))
		}
		if t.Completion.Signal != "" {
			prompt.WriteString(fmt.Sprintf("Say **%s** when complete.\n", t.Completion.Signal))
		}
	}

	return prompt.String()
}
