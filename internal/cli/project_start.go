package cli

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
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

	// Create readiness-aware scheduler (prevents deadlocks)
	scheduler := project.NewReadinessAwareScheduler(taskMgr)

	// Set role concurrency limits (default to 1 per role)
	for role := range roles {
		scheduler.SetRoleConcurrency(role, 1)
	}

	// Initialize scheduler - analyzes dependencies and builds readiness graph
	if err := scheduler.Initialize(); err != nil {
		return fmt.Errorf("initialize scheduler: %w", err)
	}

	// Check for potential deadlocks before starting
	if deadlock := scheduler.DetectPotentialDeadlock(); deadlock != nil {
		fmt.Println("Warning: Potential deadlock detected:")
		for role, blockedBy := range deadlock.BlockedBy {
			fmt.Printf("  Role %s waiting on: %v\n", role, blockedBy)
		}
		fmt.Printf("  %s\n", deadlock.Suggestion)
		fmt.Println()
	}

	// Create workstream orchestrator for agent spawning
	wsConfig := agent.DefaultWorkstreamConfig()
	orchestrator := agent.NewWorkstreamOrchestrator(agentMgr, taskMgr, wsConfig)

	// Spawn agents only for ready workstreams (those with unblocked tasks)
	fmt.Println("Spawning agents for ready workstreams...")
	runners := make(map[workstreamKey]*agent.WorkstreamRunner)
	spawnedRoles := make(map[string]bool)

	for role := range roles {
		ws := scheduler.GetNextWorkstream(role)
		if ws == nil {
			blockedWS := scheduler.GetBlockedWorkstreams(role)
			if len(blockedWS) > 0 {
				fmt.Printf("  Role %s: all %d workstream(s) blocked by dependencies\n", role, len(blockedWS))
			}
			continue
		}

		agentName := buildAgentName(ws.Project, ws.Workstream)
		key := workstreamKey{project: ws.Project, role: ws.Role, workstream: ws.Workstream}

		fmt.Printf("  Spawning %s (role: %s, ready tasks: %d)...\n", agentName, ws.Role, ws.ReadyTaskCount)

		start := time.Now()
		runner, runErr := orchestrator.StartWorkstream(ws.Project, ws.Role, ws.Workstream)
		elapsed := time.Since(start)

		if runErr != nil {
			fmt.Printf("    Failed: %v\n", runErr)
			continue
		}

		runners[key] = runner
		scheduler.ActivateWorkstream(ws.Role, ws.Workstream)
		spawnedRoles[ws.Role] = true
		fmt.Printf("    Created (%.1fs)\n", elapsed.Seconds())

		// Set up completion callbacks for dynamic rebalancing
		runner.SetOnTaskComplete(func(taskID string) {
			scheduler.OnTaskComplete(taskID)
		})

		runner.SetOnWorkstreamComplete(func(completedRole, completedWS string) {
			scheduler.OnWorkstreamComplete(completedRole, completedWS)
			orchestrator.ReleaseWorkstream(completedRole)

			// Check if another workstream is now ready
			nextWS := scheduler.GetNextWorkstream(completedRole)
			if nextWS != nil {
				// Spawn a new runner for the next ready workstream
				nextAgentName := buildAgentName(nextWS.Project, nextWS.Workstream)
				log.Printf("Starting next ready workstream: %s (role: %s)", nextAgentName, nextWS.Role)

				nextRunner, nextErr := orchestrator.StartWorkstream(nextWS.Project, nextWS.Role, nextWS.Workstream)
				if nextErr != nil {
					log.Printf("Failed to start next workstream %s: %v", nextAgentName, nextErr)
					return
				}

				scheduler.ActivateWorkstream(nextWS.Role, nextWS.Workstream)

				// Set up callbacks for the new runner
				nextRunner.SetOnTaskComplete(func(taskID string) {
					scheduler.OnTaskComplete(taskID)
				})

				// Run the new workstream
				go func() {
					if runErr := nextRunner.Run(); runErr != nil {
						log.Printf("Workstream %s failed: %v", nextAgentName, runErr)
					}
				}()
			}
		})
	}
	fmt.Println()

	// Show summary of blocked workstreams
	for role := range roles {
		if !spawnedRoles[role] {
			blocked := scheduler.GetBlockedWorkstreams(role)
			for _, ws := range blocked {
				fmt.Printf("  %s:%s blocked by workstreams: %v\n", ws.Role, ws.Workstream, ws.BlockingWorkstreams)
			}
		}
	}

	// Start the workstream runners in background goroutines
	if len(runners) > 0 {
		fmt.Printf("Starting %d workstream runner(s)...\n", len(runners))
		fmt.Println()

		var wg sync.WaitGroup
		for key, runner := range runners {
			wg.Add(1)
		go func(k workstreamKey, r *agent.WorkstreamRunner) {
			defer wg.Done()
			if err := r.Run(); err != nil {
				log.Printf("Workstream %s-%s failed: %v", k.project, k.workstream, err)
			}
			}(key, runner)
		}

		fmt.Println("Project running. Press Ctrl+C to stop.")
		if projectName != "" {
			fmt.Printf("Monitor in another terminal with: tanuki project status %s\n", projectName)
		} else {
			fmt.Println("Monitor in another terminal with: tanuki project status")
		}
		fmt.Println()

		// Wait for all workstream runners to complete
		wg.Wait()
		fmt.Println("All workstreams complete!")
	} else {
		fmt.Println("No workstream runners started.")
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
