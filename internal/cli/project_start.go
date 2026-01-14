package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/bkonkle/tanuki/internal/agent"
	"github.com/bkonkle/tanuki/internal/config"
	"github.com/bkonkle/tanuki/internal/docker"
	"github.com/bkonkle/tanuki/internal/executor"
	"github.com/bkonkle/tanuki/internal/git"
	"github.com/bkonkle/tanuki/internal/project"
	"github.com/bkonkle/tanuki/internal/role"
	"github.com/bkonkle/tanuki/internal/state"
	"github.com/bkonkle/tanuki/internal/task"
	"github.com/spf13/cobra"
)

var projectStartCmd = &cobra.Command{
	Use:   "start [name]",
	Short: "Start agents and assign tasks",
	Long: `Spawns agents for each workstream and assigns pending tasks.

Without a name argument, starts agents for all tasks in the tasks directory.

With a name argument, starts agents only for the specified project folder.

Agent naming follows the pattern: {project}-{workstream} (e.g., "auth-feature-main")

This command:
  1. Scans tasks/ directory for task files
  2. Determines which roles and workstreams are needed
  3. Spawns agents for each workstream
  4. Assigns pending tasks to idle agents
  5. Starts task execution

Use --dry-run to see what would happen without making changes.`,
	RunE: runProjectStart,
}

func init() {
	projectStartCmd.Flags().Bool("dry-run", false, "Show what would happen without doing it")
	projectCmd.AddCommand(projectStartCmd)
}

func runProjectStart(cmd *cobra.Command, args []string) error {
	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	dryRun, _ := cmd.Flags().GetBool("dry-run")

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
		fmt.Println("No tasks found. Run: tanuki project init")
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
		fmt.Printf("Starting project: %s\n", projectName)
	} else {
		tasks = allTasks
		fmt.Println("Scanning tasks...")
	}

	// Collect workstreams by role
	type workstreamKey struct {
		project    string
		role       string
		workstream string
	}
	workstreams := make(map[workstreamKey]int) // count of pending tasks
	pendingTasks := 0

	for _, t := range tasks {
		if t.Status == task.StatusPending || t.Status == task.StatusBlocked {
			key := workstreamKey{
				project:    t.Project,
				role:       t.Role,
				workstream: t.GetWorkstream(),
			}
			workstreams[key]++
			pendingTasks++
		}
	}

	if pendingTasks == 0 {
		fmt.Println("No pending tasks.")
		return nil
	}

	// Count unique roles
	roles := make(map[string]bool)
	for key := range workstreams {
		roles[key.role] = true
	}

	fmt.Printf("  Found %d tasks across %d roles, %d workstreams\n",
		len(tasks), len(roles), len(workstreams))
	fmt.Println()

	if dryRun {
		fmt.Println("[DRY RUN] Would spawn agents:")
		for key, count := range workstreams {
			agentName := buildAgentName(key.project, key.workstream)
			branchName := project.WorktreeBranch(key.project, key.workstream)
			fmt.Printf("  %s (role: %s) - %d tasks\n", agentName, key.role, count)
			fmt.Printf("    Branch: %s\n", branchName)
		}
		return nil
	}

	// Create agent manager and dependencies
	agentMgr, err := createAgentManager(projectRoot)
	if err != nil {
		return fmt.Errorf("create agent manager: %w", err)
	}

	// Create workstream orchestrator
	wsConfig := agent.DefaultWorkstreamConfig()
	orchestrator := agent.NewWorkstreamOrchestrator(agentMgr, taskMgr, wsConfig)

	// Set role concurrency limits (default to 1 per role)
	for key := range workstreams {
		orchestrator.SetRoleConcurrency(key.role, 1)
	}

	// Spawn agents for each workstream
	fmt.Println("Spawning agents...")
	runners := make(map[workstreamKey]*agent.WorkstreamRunner)

	for key := range workstreams {
		agentName := buildAgentName(key.project, key.workstream)

		fmt.Printf("  Spawning %s (role: %s)...\n", agentName, key.role)

		start := time.Now()
		runner, err := orchestrator.StartWorkstream(key.project, key.role, key.workstream)
		elapsed := time.Since(start)

		if err != nil {
			fmt.Printf("    Failed: %v\n", err)
			continue
		}

		runners[key] = runner
		fmt.Printf("    Created (%.1fs)\n", elapsed.Seconds())
	}
	fmt.Println()

	// Assign tasks to idle agents
	fmt.Println("Assigning tasks...")
	assigned := 0

	for key := range workstreams {
		// Get tasks for this workstream
		var wsTasks []*task.Task
		if key.project != "" {
			wsTasks = taskMgr.GetByProjectAndWorkstream(key.project, key.role, key.workstream)
		} else {
			wsTasks = taskMgr.GetByRoleAndWorkstream(key.role, key.workstream)
		}

		for _, t := range wsTasks {
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

			agentName := buildAgentName(key.project, key.workstream)
			fmt.Printf("  %s -> %s\n", t.ID, agentName)

			// Update task status
			_ = taskMgr.Assign(t.ID, agentName)
			assigned++

			// Build prompt and start task (placeholder)
			// prompt := buildTaskPrompt(t)
			// go agentMgr.Run(agentName, prompt, agent.RunOptions{})

			// Only assign first pending task per workstream
			break
		}
	}

	if assigned == 0 {
		fmt.Println("  No tasks assigned (all agents busy or no matching tasks)")
	}

	fmt.Println()
	fmt.Println("Project started!")
	if projectName != "" {
		fmt.Printf("Monitor with: tanuki project status %s\n", projectName)
	} else {
		fmt.Println("Monitor with: tanuki project status")
	}

	return nil
}

// buildAgentName creates the agent name from project and workstream.
// Uses project.AgentName for standardization.
func buildAgentName(projectName, workstream string) string {
	if projectName == "" {
		// For root tasks, use just the workstream
		return strings.ToLower(strings.ReplaceAll(workstream, " ", "-"))
	}
	return project.AgentName(projectName, workstream)
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

// createAgentManager creates an agent.Manager with all dependencies.
func createAgentManager(projectRoot string) (*agent.Manager, error) {
	// Load config
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	// Create git manager
	gitMgr, err := git.NewManager(cfg)
	if err != nil {
		return nil, fmt.Errorf("create git manager: %w", err)
	}

	// Create docker manager
	dockerMgr, err := docker.NewManager(cfg)
	if err != nil {
		return nil, fmt.Errorf("create docker manager: %w", err)
	}

	// Create state manager
	stateMgr, err := state.NewFileStateManager(state.DefaultStatePath(), dockerMgr)
	if err != nil {
		return nil, fmt.Errorf("create state manager: %w", err)
	}

	// Create executor
	exec := executor.NewExecutor(dockerMgr)

	// Create agent manager
	agentMgr, err := agent.NewManager(cfg, gitMgr, dockerMgr, stateMgr, exec)
	if err != nil {
		return nil, fmt.Errorf("create agent manager: %w", err)
	}

	// Set up role manager
	roleMgr := role.NewManager(projectRoot)
	agentMgr.SetRoleManager(newRoleManagerAdapter(roleMgr))

	return agentMgr, nil
}
