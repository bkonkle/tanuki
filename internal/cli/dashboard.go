package cli

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/bkonkle/tanuki/internal/agent"
	"github.com/bkonkle/tanuki/internal/config"
	"github.com/bkonkle/tanuki/internal/docker"
	"github.com/bkonkle/tanuki/internal/executor"
	"github.com/bkonkle/tanuki/internal/git"
	"github.com/bkonkle/tanuki/internal/state"
	"github.com/bkonkle/tanuki/internal/task"
	"github.com/bkonkle/tanuki/internal/tui"
)

var dashboardCmd = &cobra.Command{
	Use:     "dashboard",
	Aliases: []string{"ui", "tui"},
	Short:   "Open interactive TUI dashboard",
	Long: `Open an interactive terminal UI for monitoring and controlling agents and tasks.

The dashboard provides:
  - Real-time agent status view
  - Task list with filtering
  - Log streaming for selected agent
  - Quick actions (start, stop, attach)

Navigation:
  Tab/Shift+Tab - Switch between panes
  j/k or arrows  - Navigate lists
  ?              - Show help
  q              - Quit`,
	RunE: runDashboard,
}

func init() {
	rootCmd.AddCommand(dashboardCmd)
}

func runDashboard(_ *cobra.Command, _ []string) error {
	// Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Create providers
	agentProvider, err := createAgentProvider(cfg)
	if err != nil {
		return fmt.Errorf("create agent provider: %w", err)
	}

	taskProvider, err := createTaskProvider()
	if err != nil {
		return fmt.Errorf("create task provider: %w", err)
	}

	// Create dashboard model
	model := tui.NewModel(agentProvider, taskProvider)

	// Create and run the BubbleTea program
	p := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("run dashboard: %w", err)
	}

	return nil
}

// agentProviderAdapter adapts the agent.Manager to the tui.AgentProvider interface.
type agentProviderAdapter struct {
	manager *agent.Manager
}

func (a *agentProviderAdapter) ListAgents() ([]*tui.AgentInfo, error) {
	agents, err := a.manager.List()
	if err != nil {
		return nil, err
	}

	result := make([]*tui.AgentInfo, len(agents))
	for i, ag := range agents {
		currentTask := ""
		if ag.LastTask != nil {
			currentTask = ag.LastTask.Prompt
			if len(currentTask) > 30 {
				currentTask = currentTask[:30] + "..."
			}
		}

		result[i] = &tui.AgentInfo{
			Name:        ag.Name,
			Status:      string(ag.Status),
			Role:        ag.Role,
			CurrentTask: currentTask,
			Branch:      ag.Branch,
		}
	}

	return result, nil
}

func (a *agentProviderAdapter) StopAgent(name string) error {
	return a.manager.Stop(name)
}

func (a *agentProviderAdapter) StartAgent(name string) error {
	return a.manager.Start(name)
}

// taskProviderAdapter adapts the task.Manager to the tui.TaskProvider interface.
type taskProviderAdapter struct {
	manager *task.Manager
}

func (t *taskProviderAdapter) ListTasks() ([]*tui.TaskInfo, error) {
	// Scan for tasks if not already loaded
	tasks, err := t.manager.Scan()
	if err != nil {
		return nil, err
	}

	result := make([]*tui.TaskInfo, len(tasks))
	for i, tk := range tasks {
		result[i] = &tui.TaskInfo{
			ID:         tk.ID,
			Title:      tk.Title,
			Status:     string(tk.Status),
			Role:       tk.Role,
			AssignedTo: tk.AssignedTo,
			Priority:   string(tk.Priority),
		}
	}

	return result, nil
}

func createAgentProvider(cfg *config.Config) (tui.AgentProvider, error) {
	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("get working directory: %w", err)
	}

	// Create dependencies
	gitMgr, err := git.NewManager(cfg)
	if err != nil {
		return nil, fmt.Errorf("create git manager: %w", err)
	}

	dockerMgr, err := docker.NewManager(cfg)
	if err != nil {
		return nil, fmt.Errorf("create docker manager: %w", err)
	}

	stateMgr, err := state.NewFileStateManager(state.DefaultStatePath(), dockerMgr)
	_ = cwd // Silence unused warning
	if err != nil {
		return nil, fmt.Errorf("create state manager: %w", err)
	}

	exec := executor.NewExecutor(dockerMgr)

	// Create agent manager
	agentMgr, err := agent.NewManager(cfg, gitMgr, dockerMgr, stateMgr, exec)
	if err != nil {
		return nil, fmt.Errorf("create agent manager: %w", err)
	}

	return &agentProviderAdapter{manager: agentMgr}, nil
}

func createTaskProvider() (tui.TaskProvider, error) {
	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("get working directory: %w", err)
	}

	// Create task manager
	taskMgr := task.NewManager(&task.Config{
		ProjectRoot: cwd,
	})

	return &taskProviderAdapter{manager: taskMgr}, nil
}
