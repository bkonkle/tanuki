package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
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
	runMaxIter  int
	runSignal   string
	runVerify   string
	runMaxTurns int
	runAllow    []string
	runDeny     []string
)

var runCmd = &cobra.Command{
	Use:   "run <agent> <prompt>",
	Short: "Send a task to an agent",
	Long: `Send a task to an agent using Ralph mode (autonomous loop until complete).

The agent will iterate until completion criteria are met:
- Completion signal detected in output (default: "DONE")
- Verify command exits with code 0 (if specified)
- Max iterations reached

Examples:
  tanuki run auth "Implement OAuth2 login"
  tanuki run auth "Fix all lint errors. Say DONE when clean."
  tanuki run auth "Increase coverage to 80%" --verify "npm test -- --coverage"
  tanuki run auth "Add feature" --signal "COMPLETE" --max-iter 50`,
	Args: cobra.ExactArgs(2),
	RunE: runRun,
}

func init() {
	// Ralph mode options (now the default and only mode)
	runCmd.Flags().IntVar(&runMaxIter, "max-iter", 30, "Max iterations before stopping")
	runCmd.Flags().StringVar(&runSignal, "signal", "DONE", "Completion signal to detect in output")
	runCmd.Flags().StringVar(&runVerify, "verify", "", "Command to verify completion (e.g., 'npm test')")

	// Execution options
	runCmd.Flags().IntVarP(&runMaxTurns, "max-turns", "t", 0, "Max conversation turns per iteration")
	runCmd.Flags().StringSliceVarP(&runAllow, "allow", "a", nil, "Additional allowed tools")
	runCmd.Flags().StringSliceVarP(&runDeny, "deny", "d", nil, "Disallowed tools")

	rootCmd.AddCommand(runCmd)
}

func runRun(_ *cobra.Command, args []string) error {
	agentName := args[0]
	prompt := args[1]

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

	// Check agent exists and is available
	ag, err := agentMgr.Get(agentName)
	if err != nil {
		return fmt.Errorf("agent %q not found", agentName)
	}

	if ag.Status == state.StatusWorking {
		return fmt.Errorf("agent %q is already working on a task\nUse 'tanuki logs %s' to see progress", agentName, agentName)
	}

	if ag.Status == state.StatusStopped {
		return fmt.Errorf("agent %q is stopped\nUse 'tanuki start %s' first", agentName, agentName)
	}

	// Build run options
	opts := agent.RunOptions{
		Follow:          true, // Always follow in Ralph mode
		MaxTurns:        runMaxTurns,
		AllowedTools:    runAllow,
		DisallowedTools: runDeny,
	}

	// Always use Ralph mode
	return runRalphMode(agentMgr, agentName, prompt, opts)
}

func runRalphMode(agentMgr *agent.Manager, agentName string, prompt string, opts agent.RunOptions) error {
	fmt.Printf("Running %s (max %d iterations)...\n", agentName, runMaxIter)
	fmt.Printf("Completion signal: %q\n", runSignal)
	if runVerify != "" {
		fmt.Printf("Verify command: %s\n", runVerify)
	}
	fmt.Println()

	startTime := time.Now()

	for i := 1; i <= runMaxIter; i++ {
		fmt.Printf("=== Iteration %d/%d ===\n", i, runMaxIter)

		// Create a pipe to capture output
		pr, pw, err := os.Pipe()
		if err != nil {
			return fmt.Errorf("failed to create pipe: %w", err)
		}

		// Create a buffer to capture output for signal detection
		outputChan := make(chan string, 100)
		doneChan := make(chan bool)

		// Start goroutine to read output
		go func() {
			scanner := bufio.NewScanner(pr)
			for scanner.Scan() {
				line := scanner.Text()
				fmt.Println(line)
				outputChan <- line
			}
			close(doneChan)
		}()

		// Run with output to pipe
		opts.Output = pw
		opts.Follow = true

		execErr := agentMgr.Run(agentName, prompt, opts)
		_ = pw.Close()

		// Wait for output reading to finish
		<-doneChan

		if execErr != nil {
			fmt.Fprintf(os.Stderr, "\nWarning: execution error: %v\n", execErr)
		}

		// Check for completion signal in output
		signalFound := false
		for len(outputChan) > 0 {
			line := <-outputChan
			if strings.Contains(line, runSignal) {
				signalFound = true
				break
			}
		}

		// Run verify command if specified
		if runVerify != "" {
			fmt.Printf("\nRunning verification: %s\n", runVerify)
			if verifyErr := runVerifyCommand(runVerify); verifyErr != nil {
				fmt.Printf("Verification failed: %v\n", verifyErr)
				fmt.Println("Continuing to next iteration...")
				prompt = fmt.Sprintf("Previous verification failed: %v\nPlease fix the issues and say %s when complete.", verifyErr, runSignal)
				continue
			}
			fmt.Println("Verification passed!")
			signalFound = true
		}

		if signalFound {
			duration := time.Since(startTime)
			fmt.Printf("\n=== Completion signal detected: %s ===\n", runSignal)
			fmt.Printf("Completed in %d iteration(s) (%s)\n", i, formatDuration(duration))
			return nil
		}

		if i < runMaxIter {
			fmt.Println("\nContinuing to next iteration...")
			prompt = "Continue with the task."
		}
	}

	fmt.Printf("\n=== Max iterations reached (%d) ===\n", runMaxIter)
	fmt.Println("Task may not be complete. Check logs for details.")
	return nil
}

func runVerifyCommand(cmdStr string) error {
	parts := strings.Fields(cmdStr)
	if len(parts) == 0 {
		return fmt.Errorf("empty verify command")
	}

	cmd := exec.Command(parts[0], parts[1:]...) //nolint:gosec // G204: Verify command is user-provided for task verification
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
