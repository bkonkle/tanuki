package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bkonkle/tanuki/internal/config"
	"github.com/bkonkle/tanuki/internal/docker"
	"github.com/bkonkle/tanuki/internal/git"
	"github.com/spf13/cobra"
)

var initForce bool

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize tanuki in the current project",
	Long: `Initialize tanuki in the current git repository.

Creates the .tanuki/ directory structure and default configuration.
Safe to run multiple times - will not overwrite existing config.`,
	RunE: runInit,
}

func init() {
	initCmd.Flags().BoolVar(&initForce, "force", false, "Overwrite existing config")
	rootCmd.AddCommand(initCmd)
}

func runInit(_ *cobra.Command, _ []string) error {
	// Check if in a git repo
	if !git.IsGitRepo() {
		return fmt.Errorf("not a git repository, run 'git init' first")
	}

	// Create directory structure
	dirs := []string{
		".tanuki/config",
		".tanuki/worktrees",
		".tanuki/state",
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0750); err != nil {
			return fmt.Errorf("failed to create %s: %w", dir, err)
		}
	}

	// Create .gitkeep files
	for _, dir := range []string{".tanuki/worktrees", ".tanuki/state"} {
		gitkeep := filepath.Join(dir, ".gitkeep")
		if _, err := os.Stat(gitkeep); os.IsNotExist(err) {
			if err := os.WriteFile(gitkeep, []byte{}, 0600); err != nil {
				return fmt.Errorf("failed to create %s: %w", gitkeep, err)
			}
		}
	}

	// Create config if not exists (or if --force)
	configPath := "tanuki.yaml"
	if _, err := os.Stat(configPath); os.IsNotExist(err) || initForce {
		if err := config.WriteDefault(configPath); err != nil {
			return fmt.Errorf("failed to create config: %w", err)
		}
		fmt.Printf("Created %s\n", configPath)
	} else {
		fmt.Printf("Config %s already exists, skipping\n", configPath)
	}

	// Update .gitignore
	updated, err := updateGitignore()
	if err != nil {
		fmt.Printf("Warning: failed to update .gitignore: %v\n", err)
	} else if updated {
		fmt.Println("Updated .gitignore")
	}

	// Load config (which may have been just created or already existed)
	cfg, err := config.Load()
	if err != nil {
		// If validation fails, report but continue with defaults
		fmt.Printf("Warning: config validation issue: %v\n", err)
		fmt.Println("Using default configuration.")
		cfg = config.DefaultConfig()
	}

	// Ensure Docker network exists
	if err := docker.EnsureNetwork(cfg.Network.Name); err != nil {
		fmt.Printf("Warning: failed to create Docker network: %v\n", err)
		fmt.Printf("You may need to run: docker network create %s\n", cfg.Network.Name)
	} else {
		fmt.Printf("Created Docker network %s\n", cfg.Network.Name)
	}

	fmt.Println("\nTanuki initialized successfully!")
	fmt.Println("\nNext steps:")
	fmt.Println("  tanuki spawn <name>     # Create an agent")
	fmt.Println("  tanuki run <name> \"...\" # Send a task")

	return nil
}

func updateGitignore() (bool, error) {
	gitignorePath := ".gitignore"
	entries := []string{
		"# Tanuki",
		".tanuki/worktrees/",
		".tanuki/state/",
	}

	// Read existing content
	existing, _ := os.ReadFile(gitignorePath)
	content := string(existing)

	// Check if already has tanuki entries
	if strings.Contains(content, ".tanuki/worktrees/") {
		return false, nil // Already configured
	}

	// Append entries
	if len(content) > 0 && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += "\n" + strings.Join(entries, "\n") + "\n"

	return true, os.WriteFile(gitignorePath, []byte(content), 0600)
}
