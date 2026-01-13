package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/bkonkle/tanuki/internal/agent"
	"github.com/bkonkle/tanuki/internal/config"
	"github.com/bkonkle/tanuki/internal/docker"
	"github.com/bkonkle/tanuki/internal/executor"
	"github.com/bkonkle/tanuki/internal/git"
	"github.com/bkonkle/tanuki/internal/state"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	listOutput string
)

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all agents",
	Long:    `List all agents and their current status.`,
	RunE:    runList,
}

func init() {
	listCmd.Flags().StringVarP(&listOutput, "output", "o", "table", "Output format (table, json)")
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
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

	// Reconcile state with Docker before listing
	if err := agentMgr.Reconcile(); err != nil {
		// Log warning but continue
		fmt.Fprintf(os.Stderr, "Warning: failed to reconcile state: %v\n", err)
	}

	// Get all agents
	agents, err := agentMgr.List()
	if err != nil {
		return err
	}

	// Handle empty list
	if len(agents) == 0 {
		fmt.Println("No agents found.")
		fmt.Println("\nCreate one with:")
		fmt.Println("  tanuki spawn <name>")
		return nil
	}

	// Output based on format
	switch listOutput {
	case "json":
		return printJSON(agents)
	default:
		return printTable(agents)
	}
}

func printTable(agents []*agent.Agent) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tSTATUS\tBRANCH\tUPTIME")
	fmt.Fprintln(w, "----\t------\t------\t------")

	for _, ag := range agents {
		uptime := formatDuration(time.Since(ag.CreatedAt))
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			ag.Name,
			colorStatus(string(ag.Status)),
			ag.Branch,
			uptime,
		)
	}

	return w.Flush()
}

func colorStatus(status string) string {
	// Check if stdout is a terminal
	if !isTerminal() {
		return status
	}

	switch status {
	case "idle":
		return color.GreenString(status)
	case "working":
		return color.YellowString(status)
	case "stopped":
		return color.RedString(status)
	case "error":
		return color.RedString(status)
	default:
		return status
	}
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

func printJSON(agents []*agent.Agent) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(agents)
}

// isTerminal checks if stdout is a terminal (TTY).
// This is used to determine whether to use colors in output.
func isTerminal() bool {
	fileInfo, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}
