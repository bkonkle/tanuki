package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/bkonkle/tanuki/internal/agent"
	"github.com/bkonkle/tanuki/internal/config"
	"github.com/bkonkle/tanuki/internal/docker"
	"github.com/bkonkle/tanuki/internal/executor"
	"github.com/bkonkle/tanuki/internal/git"
	"github.com/bkonkle/tanuki/internal/state"
	"github.com/spf13/cobra"
)

var (
	statusOutput string
)

var statusCmd = &cobra.Command{
	Use:   "status <agent>",
	Short: "Show detailed agent status",
	Long: `Show detailed status information for an agent.

Examples:
  tanuki status auth-feature
  tanuki status auth-feature -o json`,
	Args: cobra.ExactArgs(1),
	RunE: runStatus,
}

func init() {
	statusCmd.Flags().StringVarP(&statusOutput, "output", "o", "text", "Output format (text, json)")
	rootCmd.AddCommand(statusCmd)
}

func runStatus(_ *cobra.Command, args []string) error {
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

	// Get status
	status, err := agentMgr.Status(agentName)
	if err != nil {
		return fmt.Errorf("agent %q not found\n\nUse 'tanuki list' to see available agents", agentName)
	}

	// Output based on format
	switch statusOutput {
	case "json":
		return printStatusJSON(status)
	default:
		return printStatusText(status)
	}
}

func printStatusText(s *agent.Status) error {
	fmt.Printf("Agent: %s\n", s.Name)
	fmt.Printf("Status: %s\n", colorStatus(s.Status))
	fmt.Printf("Uptime: %s\n", formatDuration(s.Uptime))
	fmt.Println()

	fmt.Println("Container:")
	fmt.Printf("  ID:      %s\n", s.Container.ID)
	fmt.Printf("  Running: %v\n", s.Container.Running)
	if s.Container.Memory != "" {
		fmt.Printf("  Memory:  %s\n", s.Container.Memory)
	}
	if s.Container.CPU != "" {
		fmt.Printf("  CPU:     %s\n", s.Container.CPU)
	}
	fmt.Println()

	fmt.Println("Git:")
	fmt.Printf("  Branch:  %s\n", s.Git.Branch)
	fmt.Printf("  Changes: %v\n", s.Git.HasChanges)
	if s.Git.CommitsAhead > 0 {
		fmt.Printf("  Ahead:   %d commits\n", s.Git.CommitsAhead)
	}
	fmt.Println()

	if s.LastTask != nil {
		fmt.Println("Last Task:")
		fmt.Printf("  Prompt:    %s\n", truncate(s.LastTask.Prompt, 60))
		fmt.Printf("  Started:   %s\n", s.LastTask.StartedAt.Format(time.RFC3339))
		if s.LastTask.CompletedAt != nil && !s.LastTask.CompletedAt.IsZero() {
			fmt.Printf("  Completed: %s\n", s.LastTask.CompletedAt.Format(time.RFC3339))
			duration := s.LastTask.CompletedAt.Sub(s.LastTask.StartedAt)
			fmt.Printf("  Duration:  %s\n", formatDuration(duration))
		} else if s.Status == "working" {
			fmt.Printf("  Duration:  %s (in progress)\n", formatDuration(time.Since(s.LastTask.StartedAt)))
		}
		if s.LastTask.SessionID != "" {
			fmt.Printf("  Session:   %s\n", s.LastTask.SessionID)
		}
	}

	return nil
}

func printStatusJSON(s *agent.Status) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(s)
}
