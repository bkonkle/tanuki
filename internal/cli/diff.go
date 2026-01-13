package cli

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/bkonkle/tanuki/internal/config"
	"github.com/bkonkle/tanuki/internal/docker"
	"github.com/bkonkle/tanuki/internal/git"
	"github.com/bkonkle/tanuki/internal/state"
	"github.com/spf13/cobra"
)

var (
	diffStat     bool
	diffNameOnly bool
	diffBase     string
)

var diffCmd = &cobra.Command{
	Use:   "diff <agent>",
	Short: "Show changes made by an agent",
	Long: `Show git diff between an agent's branch and the base branch.

Examples:
  tanuki diff auth-feature
  tanuki diff auth-feature --stat
  tanuki diff auth-feature --name-only
  tanuki diff auth-feature --base develop`,
	Args: cobra.ExactArgs(1),
	RunE: runDiff,
}

func init() {
	diffCmd.Flags().BoolVar(&diffStat, "stat", false, "Show diffstat instead of patch")
	diffCmd.Flags().BoolVar(&diffNameOnly, "name-only", false, "Show only names of changed files")
	diffCmd.Flags().StringVar(&diffBase, "base", "", "Base branch to compare against (default: auto-detect)")
	rootCmd.AddCommand(diffCmd)
}

func runDiff(cmd *cobra.Command, args []string) error {
	agentName := args[0]

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

	// We don't need the full agent manager infrastructure for diff,
	// but we still need to verify the agent exists in state
	ag, err := stateMgr.GetAgent(agentName)
	if err != nil {
		return fmt.Errorf("agent %q not found\n\nList agents with:\n  tanuki list", agentName)
	}

	// Auto-detect base branch if not specified
	baseBranch := diffBase
	if baseBranch == "" {
		var branchErr error
		baseBranch, branchErr = gitMgr.GetMainBranch()
		if branchErr != nil {
			baseBranch = "main" // Fallback
		}
	}

	// Build diff command
	diffArgs := []string{"diff"}

	if diffStat {
		diffArgs = append(diffArgs, "--stat")
	} else if diffNameOnly {
		diffArgs = append(diffArgs, "--name-only")
	}

	// Compare base...agent_branch (three-dot diff)
	diffArgs = append(diffArgs, fmt.Sprintf("%s...%s", baseBranch, ag.Branch))

	gitCmd := exec.Command("git", diffArgs...)
	gitCmd.Stdout = os.Stdout
	gitCmd.Stderr = os.Stderr

	return gitCmd.Run()
}
