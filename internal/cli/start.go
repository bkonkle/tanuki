package cli

import (
	"fmt"

	"github.com/bkonkle/tanuki/internal/agent"
	"github.com/bkonkle/tanuki/internal/config"
	"github.com/bkonkle/tanuki/internal/docker"
	"github.com/bkonkle/tanuki/internal/executor"
	"github.com/bkonkle/tanuki/internal/git"
	"github.com/bkonkle/tanuki/internal/state"
	"github.com/spf13/cobra"
)

var (
	startAll bool
)

var startCmd = &cobra.Command{
	Use:   "start <agent>",
	Short: "Start a stopped agent",
	Long: `Start a previously stopped agent's container.

Examples:
  tanuki start auth-feature
  tanuki start --all`,
	Args: cobra.MaximumNArgs(1),
	RunE: runStart,
}

func init() {
	startCmd.Flags().BoolVar(&startAll, "all", false, "Start all stopped agents")
	rootCmd.AddCommand(startCmd)
}

func runStart(_ *cobra.Command, args []string) error {
	// Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Create dependencies
	gitMgr, err := git.NewManager(cfg)
	if err != nil {
		return fmt.Errorf("failed to create git manager: %w", err)
	}

	dockerMgr, err := docker.NewManager(cfg)
	if err != nil {
		return fmt.Errorf("failed to create docker manager: %w", err)
	}

	stateMgr, err := state.NewFileStateManager(state.DefaultStatePath(), dockerMgr)
	if err != nil {
		return fmt.Errorf("failed to create state manager: %w", err)
	}

	// Create executor
	exec := executor.NewExecutor(dockerMgr)

	// Create agent manager
	agentMgr, err := agent.NewManager(cfg, gitMgr, dockerMgr, stateMgr, exec)
	if err != nil {
		return fmt.Errorf("failed to create agent manager: %w", err)
	}

	if startAll {
		agents, err := agentMgr.List()
		if err != nil {
			return err
		}

		for _, ag := range agents {
			if ag.Status != state.StatusStopped {
				continue
			}
			fmt.Printf("Starting %s...\n", ag.Name)
			if err := agentMgr.Start(ag.Name); err != nil {
				fmt.Printf("  Error: %v\n", err)
			} else {
				fmt.Printf("  Started\n")
			}
		}
		return nil
	}

	if len(args) == 0 {
		return fmt.Errorf("agent name required (or use --all)")
	}

	agentName := args[0]
	if err := agentMgr.Start(agentName); err != nil {
		return err
	}

	fmt.Printf("Started %s\n", agentName)
	return nil
}
