package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
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
	mergeSquash  bool
	mergePR      bool
	mergeRemove  bool
	mergeNoEdit  bool
	mergeMessage string
)

var mergeCmd = &cobra.Command{
	Use:   "merge <agent>",
	Short: "Merge an agent's work",
	Long: `Merge an agent's branch into the current branch.

By default, performs a regular merge. Use --squash to squash all commits.
Use --pr to create a GitHub pull request instead of merging locally.

Examples:
  tanuki merge auth-feature
  tanuki merge auth-feature --squash
  tanuki merge auth-feature --pr
  tanuki merge auth-feature --remove`,
	Args: cobra.ExactArgs(1),
	RunE: runMerge,
}

func init() {
	mergeCmd.Flags().BoolVar(&mergeSquash, "squash", false, "Squash merge")
	mergeCmd.Flags().BoolVar(&mergePR, "pr", false, "Create GitHub PR instead of merging")
	mergeCmd.Flags().BoolVar(&mergeRemove, "remove", false, "Remove agent after successful merge")
	mergeCmd.Flags().BoolVar(&mergeNoEdit, "no-edit", false, "Use default merge message")
	mergeCmd.Flags().StringVarP(&mergeMessage, "message", "m", "", "Merge commit message")
	rootCmd.AddCommand(mergeCmd)
}

func runMerge(_ *cobra.Command, args []string) error {
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

	// Create executor
	exec := executor.NewExecutor(dockerMgr)

	// Create agent manager
	agentMgr, err := agent.NewManager(cfg, gitMgr, dockerMgr, stateMgr, exec)
	if err != nil {
		return fmt.Errorf("failed to create agent manager: %w", err)
	}

	// Get agent
	ag, err := agentMgr.Get(agentName)
	if err != nil {
		return fmt.Errorf("agent %q not found\n\nList agents with:\n  tanuki list", agentName)
	}

	// Check for uncommitted changes in agent worktree
	status, err := gitMgr.GetStatus(agentName)
	if err == nil && status != "" {
		fmt.Println("Warning: Agent has uncommitted changes:")
		fmt.Println(status)
		fmt.Print("\nContinue anyway? [y/N]: ")

		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))

		if response != "y" && response != "yes" {
			fmt.Println("Merge cancelled.")
			return nil
		}
	}

	// Show summary
	fmt.Printf("Agent: %s\n", ag.Name)
	fmt.Printf("Branch: %s\n", ag.Branch)
	fmt.Println()

	// Get and show diff stat
	currentBranch, err := gitMgr.GetCurrentBranch()
	if err != nil || currentBranch == "" {
		// Fall back to main branch
		currentBranch, err = gitMgr.GetMainBranch()
		if err != nil {
			currentBranch = "main"
		}
	}

	diffStat, err := getDiffStat(ag.Branch, currentBranch)
	if err == nil && diffStat != "" {
		fmt.Printf("Changes:\n%s\n", diffStat)
	}

	// Handle PR creation
	if mergePR {
		return createPullRequest(ag, gitMgr)
	}

	// Perform local merge
	return mergeBranch(ag, agentMgr, gitMgr)
}

func mergeBranch(ag *agent.Agent, agentMgr *agent.Manager, _ *git.Manager) error {
	mergeArgs := []string{"merge"}

	if mergeSquash {
		mergeArgs = append(mergeArgs, "--squash")
	}

	if mergeNoEdit {
		mergeArgs = append(mergeArgs, "--no-edit")
	}

	if mergeMessage != "" {
		mergeArgs = append(mergeArgs, "-m", mergeMessage)
	}

	mergeArgs = append(mergeArgs, ag.Branch)

	cmd := exec.Command("git", mergeArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("merge failed: %w\n\nResolve conflicts and run:\n  git merge --continue\n\nOr abort with:\n  git merge --abort", err)
	}

	// For squash merge, need to commit
	if mergeSquash {
		commitMsg := mergeMessage
		if commitMsg == "" {
			commitMsg = fmt.Sprintf("Merge work from agent %s", ag.Name)
		}
		commitCmd := exec.Command("git", "commit", "-m", commitMsg) //nolint:gosec // G204: commitMsg is user-controlled for git commit
		commitCmd.Stdout = os.Stdout
		commitCmd.Stderr = os.Stderr
		if err := commitCmd.Run(); err != nil {
			return fmt.Errorf("failed to commit squash merge: %w", err)
		}
	}

	fmt.Printf("\nSuccessfully merged %s\n", ag.Branch)

	// Remove agent if requested
	if mergeRemove {
		fmt.Printf("\nRemoving agent %s...\n", ag.Name)
		if err := agentMgr.Remove(ag.Name, agent.RemoveOptions{Force: true, KeepBranch: false}); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to remove agent: %v\n", err)
		} else {
			fmt.Printf("Agent %s removed.\n", ag.Name)
		}
	}

	return nil
}

func createPullRequest(ag *agent.Agent, gitMgr *git.Manager) error {
	// Check if gh CLI is available
	if _, err := exec.LookPath("gh"); err != nil {
		return fmt.Errorf("GitHub CLI (gh) not found\n\nInstall with:\n  brew install gh\n  # or visit https://cli.github.com/")
	}

	// Push branch to remote first
	fmt.Println("Pushing branch to remote...")
	pushCmd := exec.Command("git", "push", "-u", "origin", ag.Branch) //nolint:gosec // G204: Branch name is from internal agent state
	pushCmd.Stdout = os.Stdout
	pushCmd.Stderr = os.Stderr
	if err := pushCmd.Run(); err != nil {
		return fmt.Errorf("failed to push branch: %w\n\nMake sure you have push access to the remote repository", err)
	}

	// Create PR using gh CLI
	fmt.Println("\nCreating pull request...")

	// Get base branch
	baseBranch, err := gitMgr.GetMainBranch()
	if err != nil {
		baseBranch = "main" // Fallback
	}

	prTitle := fmt.Sprintf("[Tanuki] %s", ag.Name)
	prBody := fmt.Sprintf("Work completed by Tanuki agent `%s`.\n\nCreated automatically by `tanuki merge --pr`.", ag.Name)

	prCmd := exec.Command("gh", "pr", "create", //nolint:gosec // G204: PR arguments are from internal agent state
		"--base", baseBranch,
		"--head", ag.Branch,
		"--title", prTitle,
		"--body", prBody,
	)
	prCmd.Stdout = os.Stdout
	prCmd.Stderr = os.Stderr
	prCmd.Stdin = os.Stdin

	if err := prCmd.Run(); err != nil {
		return fmt.Errorf("failed to create PR: %w\n\nMake sure you're authenticated with:\n  gh auth login", err)
	}

	return nil
}

// getDiffStat returns the diff stat between two branches
func getDiffStat(branchName, baseBranch string) (string, error) {
	cmd := exec.Command("git", "diff", "--stat", baseBranch+"..."+branchName) //nolint:gosec // G204: Branch names are from internal state
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}
