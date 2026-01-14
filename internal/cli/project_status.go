package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"text/tabwriter"

	"github.com/bkonkle/tanuki/internal/project"
	"github.com/bkonkle/tanuki/internal/task"
	"github.com/spf13/cobra"
)

var (
	statusShowWorkstreams bool
)

var projectStatusCmd = &cobra.Command{
	Use:   "status [name]",
	Short: "Show project status",
	Long: `Displays all tasks, their status, workstreams, and assigned agents.

Without a name argument, shows status for all tasks (flat structure or all projects).

With a name argument, shows status for a specific project folder.`,
	RunE: runProjectStatus,
}

func init() {
	projectStatusCmd.Flags().BoolP("watch", "w", false, "Watch for changes (not yet implemented)")
	projectStatusCmd.Flags().BoolVarP(&statusShowWorkstreams, "workstreams", "W", false, "Show workstream details")
	projectCmd.AddCommand(projectStatusCmd)
}

func runProjectStatus(cmd *cobra.Command, args []string) error {
	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	// Check if task directory exists
	taskDir := getTasksDir(projectRoot)
	if _, err := os.Stat(taskDir); os.IsNotExist(err) {
		fmt.Println("No tasks found.")
		fmt.Printf("Create tasks in %s/ or run: tanuki project init\n", taskDir)
		return nil
	}

	// Use the real task manager
	taskMgr := task.NewManager(&task.Config{ProjectRoot: projectRoot})
	tasks, err := taskMgr.Scan()
	if err != nil {
		return fmt.Errorf("scan tasks: %w", err)
	}

	if len(tasks) == 0 {
		fmt.Println("No tasks found.")
		fmt.Printf("Create tasks in %s/ or run: tanuki project init\n", taskDir)
		return nil
	}

	// If a project name is provided, filter tasks
	var projectName string
	if len(args) > 0 {
		projectName = args[0]
		tasks = taskMgr.GetByProject(projectName)
		if len(tasks) == 0 {
			fmt.Printf("No tasks found for project '%s'.\n", projectName)
			fmt.Println("Run: tanuki project list")
			return nil
		}
	}

	// Count by status
	counts := make(map[task.Status]int)
	for _, t := range tasks {
		counts[t.Status]++
	}

	// Collect workstream info
	workstreams := collectWorkstreams(tasks)

	// Print summary
	if projectName != "" {
		fmt.Printf("Project: %s\n", projectName)
	} else {
		// Show all projects summary
		projects := taskMgr.GetProjects()
		if len(projects) > 0 {
			fmt.Printf("Projects: %s\n", filepath.Base(projectRoot))
			fmt.Printf("  Subprojects: %v\n", projects)
		} else {
			fmt.Printf("Project: %s\n", filepath.Base(projectRoot))
		}
	}
	fmt.Printf("Tasks: %d total (%d pending, %d in progress, %d complete)\n",
		len(tasks),
		counts[task.StatusPending]+counts[task.StatusBlocked],
		counts[task.StatusInProgress]+counts[task.StatusAssigned],
		counts[task.StatusComplete],
	)
	fmt.Printf("Workstreams: %d\n", len(workstreams))
	fmt.Println()

	// Print agents (placeholder for now - will integrate with agent manager later)
	printProjectAgents()

	// Print workstream summary if requested
	if statusShowWorkstreams {
		printWorkstreamSummary(workstreams, tasks)
	}

	// Print task table
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if projectName == "" && len(taskMgr.GetProjects()) > 0 {
		// Include project column when showing all
		fmt.Fprintln(w, "PROJECT\tID\tTITLE\tROLE\tWORKSTREAM\tPRIORITY\tSTATUS\tASSIGNED")
		fmt.Fprintln(w, "-------\t--\t-----\t----\t----------\t--------\t------\t--------")
	} else {
		fmt.Fprintln(w, "ID\tTITLE\tROLE\tWORKSTREAM\tPRIORITY\tSTATUS\tASSIGNED")
		fmt.Fprintln(w, "--\t-----\t----\t----------\t--------\t------\t--------")
	}

	// Sort by priority, then status
	sortTasks(tasks)

	showProjectColumn := projectName == "" && len(taskMgr.GetProjects()) > 0
	for _, t := range tasks {
		assigned := "-"
		if t.AssignedTo != "" {
			assigned = t.AssignedTo
		}
		ws := t.GetWorkstream()
		if ws == t.ID {
			ws = "-" // Don't show if same as task ID
		}

		if showProjectColumn {
			proj := t.Project
			if proj == "" {
				proj = "(root)"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				truncate(proj, 15),
				t.ID,
				truncate(t.Title, 25),
				t.Role,
				truncate(ws, 12),
				t.Priority,
				t.Status,
				assigned,
			)
		} else {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				t.ID,
				truncate(t.Title, 30),
				t.Role,
				truncate(ws, 15),
				t.Priority,
				t.Status,
				assigned,
			)
		}
	}

	return w.Flush()
}

// workstreamInfo holds summary info for a workstream.
type workstreamInfo struct {
	Name       string
	Role       string
	Total      int
	Complete   int
	Pending    int
	InProgress int
}

func collectWorkstreams(tasks []*task.Task) map[string]*workstreamInfo {
	workstreams := make(map[string]*workstreamInfo)

	for _, t := range tasks {
		ws := t.GetWorkstream()
		info, exists := workstreams[ws]
		if !exists {
			info = &workstreamInfo{
				Name: ws,
				Role: t.Role,
			}
			workstreams[ws] = info
		}

		info.Total++
		switch t.Status {
		case task.StatusComplete:
			info.Complete++
		case task.StatusPending, task.StatusBlocked:
			info.Pending++
		case task.StatusInProgress, task.StatusAssigned:
			info.InProgress++
		}
	}

	return workstreams
}

func printWorkstreamSummary(workstreams map[string]*workstreamInfo, tasks []*task.Task) {
	if len(workstreams) == 0 {
		return
	}

	fmt.Println("Workstreams:")

	// Sort by role, then name
	var wsList []*workstreamInfo
	for _, ws := range workstreams {
		wsList = append(wsList, ws)
	}
	sort.Slice(wsList, func(i, j int) bool {
		if wsList[i].Role != wsList[j].Role {
			return wsList[i].Role < wsList[j].Role
		}
		return wsList[i].Name < wsList[j].Name
	})

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "  WORKSTREAM\tROLE\tTOTAL\tCOMPLETE\tIN PROGRESS\tPENDING")
	fmt.Fprintln(w, "  ----------\t----\t-----\t--------\t-----------\t-------")

	for _, ws := range wsList {
		fmt.Fprintf(w, "  %s\t%s\t%d\t%d\t%d\t%d\n",
			truncate(ws.Name, 20),
			ws.Role,
			ws.Total,
			ws.Complete,
			ws.InProgress,
			ws.Pending,
		)
	}

	w.Flush()
	fmt.Println()
}

func printProjectAgents() {
	// Placeholder - will integrate with agent manager later
	// This function will list agents with roles assigned
}

func sortTasks(tasks []*task.Task) {
	sort.Slice(tasks, func(i, j int) bool {
		// Sort by priority first (lower order = higher priority)
		pi := tasks[i].Priority.Order()
		pj := tasks[j].Priority.Order()
		if pi != pj {
			return pi < pj
		}
		// Then by status (in_progress before pending)
		si := statusOrder(tasks[i].Status)
		sj := statusOrder(tasks[j].Status)
		if si != sj {
			return si < sj
		}
		// Then by workstream
		wi := tasks[i].GetWorkstream()
		wj := tasks[j].GetWorkstream()
		if wi != wj {
			return wi < wj
		}
		// Then by ID
		return tasks[i].ID < tasks[j].ID
	})
}

func statusOrder(s task.Status) int {
	switch s {
	case task.StatusInProgress:
		return 0
	case task.StatusAssigned:
		return 1
	case task.StatusPending:
		return 2
	case task.StatusBlocked:
		return 3
	case task.StatusReview:
		return 4
	case task.StatusComplete:
		return 5
	case task.StatusFailed:
		return 6
	default:
		return 7
	}
}

// mockTaskManager is a temporary implementation until the real TaskManager is available.
// It provides basic task scanning functionality for the status command.
type mockTaskManager struct {
	taskDir string
	tasks   map[string]*task.Task
}

func newMockTaskManager(taskDir string) *mockTaskManager {
	return &mockTaskManager{
		taskDir: taskDir,
		tasks:   make(map[string]*task.Task),
	}
}

func (m *mockTaskManager) Scan() ([]*task.Task, error) {
	entries, err := os.ReadDir(m.taskDir)
	if err != nil {
		return nil, fmt.Errorf("read tasks directory: %w", err)
	}

	var tasks []*task.Task
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}

		path := filepath.Join(m.taskDir, entry.Name())
		t, err := task.ParseFile(path)
		if err != nil {
			// Log warning but continue scanning
			fmt.Fprintf(os.Stderr, "Warning: parse %s: %v\n", entry.Name(), err)
			continue
		}

		m.tasks[t.ID] = t
		tasks = append(tasks, t)
	}

	return tasks, nil
}

func (m *mockTaskManager) Get(id string) (*task.Task, error) {
	t, ok := m.tasks[id]
	if !ok {
		return nil, fmt.Errorf("task %q not found", id)
	}
	return t, nil
}

func (m *mockTaskManager) GetByRole(role string) []*task.Task {
	var tasks []*task.Task
	for _, t := range m.tasks {
		if t.Role == role {
			tasks = append(tasks, t)
		}
	}
	return tasks
}

func (m *mockTaskManager) GetByStatus(status task.Status) []*task.Task {
	var tasks []*task.Task
	for _, t := range m.tasks {
		if t.Status == status {
			tasks = append(tasks, t)
		}
	}
	return tasks
}

func (m *mockTaskManager) GetPending() []*task.Task {
	tasks := m.GetByStatus(task.StatusPending)
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].Priority.Order() < tasks[j].Priority.Order()
	})
	return tasks
}

func (m *mockTaskManager) UpdateStatus(id string, status task.Status) error {
	t, ok := m.tasks[id]
	if !ok {
		return fmt.Errorf("task %q not found", id)
	}
	t.Status = status
	return task.WriteFile(t)
}

func (m *mockTaskManager) Assign(id string, agentName string) error {
	t, ok := m.tasks[id]
	if !ok {
		return fmt.Errorf("task %q not found", id)
	}
	t.AssignedTo = agentName
	t.Status = task.StatusAssigned
	return task.WriteFile(t)
}

func (m *mockTaskManager) Unassign(id string) error {
	t, ok := m.tasks[id]
	if !ok {
		return fmt.Errorf("task %q not found", id)
	}
	t.AssignedTo = ""
	if t.Status == task.StatusAssigned || t.Status == task.StatusInProgress {
		t.Status = task.StatusPending
	}
	return task.WriteFile(t)
}

func (m *mockTaskManager) IsBlocked(id string) (bool, error) {
	t, ok := m.tasks[id]
	if !ok {
		return false, fmt.Errorf("task %q not found", id)
	}
	if len(t.DependsOn) == 0 {
		return false, nil
	}
	for _, depID := range t.DependsOn {
		dep, ok := m.tasks[depID]
		if !ok {
			return true, nil // Missing dependency = blocked
		}
		if dep.Status != task.StatusComplete {
			return true, nil
		}
	}
	return false, nil
}

func (m *mockTaskManager) Stats() *project.TaskStats {
	stats := &project.TaskStats{
		ByStatus:   make(map[task.Status]int),
		ByRole:     make(map[string]int),
		ByPriority: make(map[task.Priority]int),
	}

	for _, t := range m.tasks {
		stats.Total++
		stats.ByStatus[t.Status]++
		stats.ByRole[t.Role]++
		stats.ByPriority[t.Priority]++
	}

	return stats
}
