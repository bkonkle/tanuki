package cli

import (
	"path/filepath"

	"github.com/bkonkle/tanuki/internal/config"
	"github.com/spf13/cobra"
)

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Manage project tasks and agents",
	Long: `Project mode enables automatic task distribution across multiple agents.

Tasks are defined in the tasks/ directory (configurable via tasks_dir in tanuki.yaml)
as markdown files with YAML front matter. Each task specifies a workstream, priority,
dependencies, and completion criteria.

Commands:
  init    - Initialize project task structure
  status  - Show all tasks, agents, and progress
  start   - Spawn agents by workstream and assign tasks
  stop    - Stop all project agents gracefully
  resume  - Resume a stopped project`,
}

func init() {
	rootCmd.AddCommand(projectCmd)
}

// getTasksDir returns the absolute path to the tasks directory.
// It loads config to get the tasks_dir setting (defaults to "tasks").
func getTasksDir(projectRoot string) string {
	cfg, err := config.Load()
	if err != nil {
		// Fall back to default if config can't be loaded
		return filepath.Join(projectRoot, "tasks")
	}
	return filepath.Join(projectRoot, cfg.TasksDir)
}
