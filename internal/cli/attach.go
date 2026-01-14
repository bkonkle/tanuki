// Package cli provides the command-line interface for tanuki.
package cli

import (
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

var attachCmd = &cobra.Command{
	Use:   "attach <agent> [command]",
	Short: "Open a shell in an agent's container",
	Long: `Open an interactive shell in an agent's container.

If a command is provided, it runs that command instead of opening a shell.

Examples:
  tanuki attach auth-feature
  tanuki attach auth-feature "ls -la"
  tanuki attach auth-feature "git status"`,
	Args: cobra.MinimumNArgs(1),
	RunE: runAttach,
}

func init() {
	rootCmd.AddCommand(attachCmd)
}

func runAttach(_ *cobra.Command, args []string) error {
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
		return fmt.Errorf("agent %q not found", agentName)
	}

	// Check container is running
	if !dockerMgr.ContainerRunning(ag.ContainerID) {
		return fmt.Errorf("agent %q is not running\nUse 'tanuki start %s' first", agentName, agentName)
	}

	// Determine shell to use
	shell, err := getShell(dockerMgr, ag.ContainerID)
	if err != nil {
		// Fallback to sh if shell detection fails
		shell = "sh"
	}

	// Build command
	var command []string

	if len(args) > 1 {
		// Run specific command - join all remaining args as a single command string
		commandStr := strings.Join(args[1:], " ")
		command = []string{"sh", "-c", commandStr}
	} else {
		// Interactive shell
		command = []string{shell}
	}

	// Execute with TTY
	execOpts := docker.ExecOptions{
		Stdin:       os.Stdin,
		Stdout:      os.Stdout,
		Stderr:      os.Stderr,
		TTY:         true,
		Interactive: true,
	}

	return dockerMgr.Exec(ag.ContainerID, command, execOpts)
}

// getShell detects which shell is available in the container.
// Tries zsh first, then bash, falls back to sh.
func getShell(dockerMgr *docker.Manager, containerID string) (string, error) {
	// Try zsh first
	if _, err := dockerMgr.ExecWithOutput(containerID, []string{"which", "zsh"}); err == nil {
		return "zsh", nil
	}

	// Fall back to bash
	if _, err := dockerMgr.ExecWithOutput(containerID, []string{"which", "bash"}); err == nil {
		return "bash", nil
	}

	// Last resort
	return "sh", nil
}
