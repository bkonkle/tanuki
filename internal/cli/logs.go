package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"sync"

	"github.com/bkonkle/tanuki/internal/agent"
	"github.com/bkonkle/tanuki/internal/config"
	"github.com/bkonkle/tanuki/internal/docker"
	"github.com/bkonkle/tanuki/internal/executor"
	"github.com/bkonkle/tanuki/internal/git"
	"github.com/bkonkle/tanuki/internal/state"
	"github.com/spf13/cobra"
)

var (
	logsFollow bool
	logsTail   int
	logsAll    bool
)

var logsCmd = &cobra.Command{
	Use:   "logs <agent>",
	Short: "Show agent output",
	Long: `Show output from an agent's container.

Examples:
  tanuki logs auth-feature
  tanuki logs auth-feature --follow
  tanuki logs auth-feature --tail 100
  tanuki logs --all`,
	Args: cobra.MaximumNArgs(1),
	RunE: runLogs,
}

func init() {
	logsCmd.Flags().BoolVarP(&logsFollow, "follow", "f", false, "Follow log output")
	logsCmd.Flags().IntVarP(&logsTail, "tail", "n", 0, "Number of lines to show from end (0 = all)")
	logsCmd.Flags().BoolVar(&logsAll, "all", false, "Show logs from all agents")
	rootCmd.AddCommand(logsCmd)
}

func runLogs(_ *cobra.Command, args []string) error {
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

	// Handle --all flag
	if logsAll {
		return showAllLogs(agentMgr, logsFollow, logsTail)
	}

	// Require agent name if not using --all
	if len(args) == 0 {
		return fmt.Errorf("agent name required (or use --all)")
	}

	agentName := args[0]
	ag, err := agentMgr.Get(agentName)
	if err != nil {
		return fmt.Errorf("agent %q not found", agentName)
	}

	return streamLogs(ag, logsFollow, logsTail)
}

func streamLogs(ag *agent.Agent, follow bool, tail int) error {
	args := []string{"logs"}

	if follow {
		args = append(args, "-f")
	}

	if tail > 0 {
		args = append(args, "--tail", strconv.Itoa(tail))
	}

	args = append(args, ag.ContainerID)

	cmd := exec.Command("docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func showAllLogs(agentMgr *agent.Manager, follow bool, tail int) error {
	agents, err := agentMgr.List()
	if err != nil {
		return err
	}

	if len(agents) == 0 {
		fmt.Println("No agents found.")
		return nil
	}

	// For --all without --follow, show sequentially
	if !follow {
		for _, ag := range agents {
			fmt.Printf("=== %s ===\n", ag.Name)
			if err := streamLogs(ag, false, tail); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to get logs for %s: %v\n", ag.Name, err)
			}
			fmt.Println()
		}
		return nil
	}

	// For --all with --follow, use goroutines with prefixed output
	var wg sync.WaitGroup
	for _, ag := range agents {
		wg.Add(1)
		go func(ag *agent.Agent) {
			defer wg.Done()
			streamLogsWithPrefix(ag, ag.Name)
		}(ag)
	}
	wg.Wait()

	return nil
}

func streamLogsWithPrefix(ag *agent.Agent, prefix string) {
	cmd := exec.Command("docker", "logs", "-f", ag.ContainerID) //nolint:gosec // G204: Container ID is from internal state, not user input
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[%s] Error: %v\n", prefix, err)
		return
	}

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "[%s] Error starting: %v\n", prefix, err)
		return
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		fmt.Printf("[%s] %s\n", prefix, scanner.Text())
	}

	if err := cmd.Wait(); err != nil {
		// Don't print error for normal termination
		if _, ok := err.(*exec.ExitError); !ok {
			fmt.Fprintf(os.Stderr, "[%s] Error: %v\n", prefix, err)
		}
	}
}
