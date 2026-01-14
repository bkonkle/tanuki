package cli

import (
	"fmt"
	"math/rand"
	"os"
	"regexp"
	"time"

	"github.com/bkonkle/tanuki/internal/agent"
	"github.com/bkonkle/tanuki/internal/config"
	"github.com/bkonkle/tanuki/internal/docker"
	"github.com/bkonkle/tanuki/internal/executor"
	"github.com/bkonkle/tanuki/internal/git"
	"github.com/bkonkle/tanuki/internal/role"
	"github.com/bkonkle/tanuki/internal/state"
	"github.com/spf13/cobra"
)

var (
	spawnCount  int
	spawnBranch string
	spawnRole   string
)

var spawnCmd = &cobra.Command{
	Use:   "spawn <name>",
	Short: "Create a new agent",
	Long: `Create a new agent with an isolated git worktree and Docker container.

Examples:
  tanuki spawn auth              # Create agent named "auth"
  tanuki spawn -n 3              # Create agent-1, agent-2, agent-3
  tanuki spawn auth -b main      # Use existing branch
  tanuki spawn backend --role backend  # Spawn with backend role`,
	Args: cobra.MaximumNArgs(1),
	RunE: runSpawn,
}

func init() {
	spawnCmd.Flags().IntVarP(&spawnCount, "count", "n", 1, "Number of agents to spawn")
	spawnCmd.Flags().StringVarP(&spawnBranch, "branch", "b", "", "Base branch (default: current branch)")
	spawnCmd.Flags().StringVarP(&spawnRole, "role", "r", "", "Role to assign to agent")
	rootCmd.AddCommand(spawnCmd)
}

func runSpawn(cmd *cobra.Command, args []string) error {
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

	// Set up role manager if role is specified
	if spawnRole != "" {
		projectRoot, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
		roleMgr := role.NewManager(projectRoot)
		agentMgr.SetRoleManager(newRoleManagerAdapter(roleMgr))
	}

	// Determine names
	var names []string
	if len(args) > 0 {
		if spawnCount > 1 {
			return fmt.Errorf("cannot use --count with explicit name")
		}
		names = []string{args[0]}
	} else {
		for i := 1; i <= spawnCount; i++ {
			names = append(names, fmt.Sprintf("agent-%d", i))
		}
	}

	// Validate all names before spawning
	for _, name := range names {
		if err := validateAgentName(name); err != nil {
			return err
		}
	}

	// Spawn each agent
	for _, name := range names {
		if spawnRole != "" {
			fmt.Printf("Spawning agent %s with role %q...\n", name, spawnRole)
		} else {
			fmt.Printf("Spawning agent %s...\n", name)
		}

		opts := agent.SpawnOptions{
			Branch: spawnBranch,
			Role:   spawnRole,
		}

		start := time.Now()
		ag, err := agentMgr.Spawn(name, opts)
		elapsed := time.Since(start)

		if err != nil {
			fmt.Printf("  Failed: %v\n", err)
			continue
		}

		fmt.Printf("  Created agent %s (%.1fs)\n", ag.Name, elapsed.Seconds())
		fmt.Printf("    Branch:    %s\n", ag.Branch)
		fmt.Printf("    Container: %s\n", ag.ContainerName)
		if ag.Role != "" {
			fmt.Printf("    Role:      %s\n", ag.Role)
		}
		fmt.Printf("    Worktree:  %s\n", ag.WorktreePath)
		fmt.Println()
	}

	if len(names) == 1 {
		fmt.Printf("Run a task:\n")
		fmt.Printf("  tanuki run %s \"your task here\"\n", names[0])
	} else {
		fmt.Printf("Run tasks:\n")
		fmt.Printf("  tanuki run <agent-name> \"your task here\"\n")
	}

	return nil
}

// validNamePattern enforces agent name constraints:
// - Must start with a lowercase letter
// - Can contain lowercase letters, numbers, and hyphens
// - Must end with a lowercase letter or number
var validNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]*[a-z0-9]$`)

// validateAgentName validates an agent name according to the naming rules.
func validateAgentName(name string) error {
	if len(name) < 2 {
		return fmt.Errorf("agent name must be at least 2 characters (got %q)", name)
	}
	if len(name) > 63 {
		return fmt.Errorf("agent name must be at most 63 characters (got %q)", name)
	}
	if !validNamePattern.MatchString(name) {
		return fmt.Errorf("agent name must start with a letter, contain only lowercase letters, numbers, and hyphens (got %q)", name)
	}
	return nil
}

// Name generation for when no explicit name is provided
var (
	adjectives = []string{"quick", "lazy", "clever", "brave", "calm", "bold", "wise", "swift", "keen", "bright"}
	nouns      = []string{"fox", "wolf", "bear", "hawk", "deer", "eagle", "tiger", "panther", "falcon", "lynx"}
)

// generateAgentName generates a random agent name in the format "adjective-noun".
// This is used when the user doesn't provide an explicit name.
func generateAgentName() string {
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	adj := adjectives[rnd.Intn(len(adjectives))]
	noun := nouns[rnd.Intn(len(nouns))]
	return fmt.Sprintf("%s-%s", adj, noun)
}
