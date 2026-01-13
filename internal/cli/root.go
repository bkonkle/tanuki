package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version information - set at build time via ldflags
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

var rootCmd = &cobra.Command{
	Use:   "tanuki",
	Short: "Multi-agent orchestration for Claude Code",
	Long: `Tanuki orchestrates multiple Claude Code agents in isolated
Docker containers, enabling parallel development without conflicts.

Each agent operates in its own Git worktree with a dedicated branch,
allowing multiple AI agents to work on different features simultaneously
without stepping on each other's changes.`,
	SilenceUsage: true,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("tanuki %s\n", version)
		fmt.Printf("  commit: %s\n", commit)
		fmt.Printf("  built:  %s\n", date)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose output")
}

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}

// SetVersionInfo sets version information from build flags
func SetVersionInfo(v, c, d string) {
	version = v
	commit = c
	date = d
}

// GetRootCmd returns the root command for testing and subcommand registration
func GetRootCmd() *cobra.Command {
	return rootCmd
}
