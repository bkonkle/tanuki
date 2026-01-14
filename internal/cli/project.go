package cli

import (
	"github.com/spf13/cobra"
)

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Manage project tasks and agents",
	Long: `Project mode enables automatic task distribution across multiple agents.

Tasks are defined in .tanuki/tasks/ as markdown files with YAML front matter.
Each task specifies a role, priority, dependencies, and completion criteria.

Commands:
  init    - Initialize project task structure
  status  - Show all tasks, agents, and progress
  start   - Spawn agents by role and assign tasks
  stop    - Stop all project agents gracefully
  resume  - Resume a stopped project`,
}

func init() {
	rootCmd.AddCommand(projectCmd)
}
