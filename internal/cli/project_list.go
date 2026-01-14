package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/bkonkle/tanuki/internal/project"
	"github.com/bkonkle/tanuki/internal/task"
	"github.com/spf13/cobra"
)

var projectListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all projects",
	Long: `Lists all project folders in the tasks directory.

A project is a folder within tasks/ that contains a project.md file.
Each project can have multiple task files and workstreams.`,
	RunE: runProjectList,
}

func init() {
	projectCmd.AddCommand(projectListCmd)
}

func runProjectList(cmd *cobra.Command, args []string) error {
	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	taskDir := getTasksDir(projectRoot)

	// Check if task directory exists
	if _, err := os.Stat(taskDir); os.IsNotExist(err) {
		fmt.Println("No tasks directory found.")
		fmt.Println("Run: tanuki project init [name]")
		return nil
	}

	// Create task manager and project manager
	taskMgr := task.NewManager(&task.Config{ProjectRoot: projectRoot})
	projMgr := project.NewProjectManager(taskDir, taskMgr)

	// Scan for projects
	if err := projMgr.Scan(); err != nil {
		return fmt.Errorf("scan projects: %w", err)
	}

	projects := projMgr.List()
	rootTasks := projMgr.GetRootTasks()

	if len(projects) == 0 && len(rootTasks) == 0 {
		fmt.Println("No projects or tasks found.")
		fmt.Println("Run: tanuki project init [name]")
		return nil
	}

	// Print projects
	if len(projects) > 0 {
		fmt.Printf("Projects (%d):\n\n", len(projects))

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tTASKS\tWORKSTREAMS\tPENDING\tCOMPLETE\tDESCRIPTION")
		fmt.Fprintln(w, "----\t-----\t-----------\t-------\t--------\t-----------")

		for _, p := range projects {
			stats := p.Stats()
			workstreams := p.GetWorkstreams()

			pending := stats.ByStatus[task.StatusPending] + stats.ByStatus[task.StatusBlocked]
			complete := stats.ByStatus[task.StatusComplete]

			desc := truncate(p.Description, 40)
			if desc == "" {
				desc = "-"
			}

			fmt.Fprintf(w, "%s\t%d\t%d\t%d\t%d\t%s\n",
				p.Name,
				stats.Total,
				len(workstreams),
				pending,
				complete,
				desc,
			)
		}

		w.Flush()
	}

	// Print root tasks summary
	if len(rootTasks) > 0 {
		if len(projects) > 0 {
			fmt.Println()
		}
		fmt.Printf("Root tasks (not in projects): %d\n", len(rootTasks))
		fmt.Println("  These tasks are in tasks/ but not inside a project folder.")
	}

	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  tanuki project status [name]  - Show detailed status")
	fmt.Println("  tanuki project start [name]   - Start project agents")
	fmt.Println("  tanuki project init <name>    - Create new project")

	return nil
}
