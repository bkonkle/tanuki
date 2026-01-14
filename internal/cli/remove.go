package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/bkonkle/tanuki/internal/agent"
	"github.com/bkonkle/tanuki/internal/config"
	"github.com/bkonkle/tanuki/internal/docker"
	"github.com/bkonkle/tanuki/internal/executor"
	"github.com/bkonkle/tanuki/internal/git"
	"github.com/bkonkle/tanuki/internal/state"
	"github.com/spf13/cobra"
)

var (
	removeForce      bool
	removeKeepBranch bool
	removeAll        bool
)

var removeCmd = &cobra.Command{
	Use:     "remove <agent>",
	Aliases: []string{"rm"},
	Short:   "Remove an agent completely",
	Long: `Remove an agent's container, worktree, and branch.

This action is destructive. Use --keep-branch to preserve the git branch.

Examples:
  tanuki remove auth-feature
  tanuki remove auth-feature --keep-branch
  tanuki remove --all --force`,
	Args: cobra.MaximumNArgs(1),
	RunE: runRemove,
}

func init() {
	removeCmd.Flags().BoolVar(&removeForce, "force", false, "Skip confirmation")
	removeCmd.Flags().BoolVar(&removeKeepBranch, "keep-branch", false, "Keep the git branch")
	removeCmd.Flags().BoolVar(&removeAll, "all", false, "Remove all agents")
	rootCmd.AddCommand(removeCmd)
}

func runRemove(_ *cobra.Command, args []string) error {
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

	if removeAll {
		if !removeForce {
			if !confirm("Remove ALL agents? This cannot be undone.") {
				return nil
			}
		}

		agents, err := agentMgr.List()
		if err != nil {
			return err
		}

		for _, ag := range agents {
			fmt.Printf("Removing %s...\n", ag.Name)
			opts := agent.RemoveOptions{
				Force:      true,
				KeepBranch: removeKeepBranch,
			}
			if err := agentMgr.Remove(ag.Name, opts); err != nil {
				fmt.Printf("  Error: %v\n", err)
			} else {
				fmt.Printf("  Removed\n")
			}
		}
		return nil
	}

	if len(args) == 0 {
		return fmt.Errorf("agent name required (or use --all)")
	}

	agentName := args[0]

	// Confirmation
	if !removeForce {
		ag, err := agentMgr.Get(agentName)
		if err != nil {
			return fmt.Errorf("agent %q not found", agentName)
		}

		if ag.Status == state.StatusWorking {
			fmt.Printf("Agent %q is currently working!\n", agentName)
		}

		if !confirm(fmt.Sprintf("Remove agent %q?", agentName)) {
			return nil
		}
	}

	opts := agent.RemoveOptions{
		Force:      removeForce,
		KeepBranch: removeKeepBranch,
	}

	if err := agentMgr.Remove(agentName, opts); err != nil {
		return err
	}

	fmt.Printf("Removed agent %s\n", agentName)
	if removeKeepBranch {
		gitMgr, _ := git.NewManager(cfg)
		branchName := gitMgr.GetBranchName(agentName)
		fmt.Printf("  Branch preserved: %s\n", branchName)
	}

	return nil
}

// confirm prompts the user for yes/no confirmation.
func confirm(message string) bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("%s [y/N]: ", message)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes"
}
