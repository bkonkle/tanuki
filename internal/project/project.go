package project

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/bkonkle/tanuki/internal/task"
)

// Project represents a project folder containing tasks.
// A project is identified by a folder with a project.md file inside tasks/.
type Project struct {
	// Name is the folder name (e.g., "auth-feature")
	Name string

	// Path is the full path to the project folder
	Path string

	// Description is extracted from project.md
	Description string

	// Tasks are the tasks belonging to this project
	Tasks []*task.Task
}

// GetWorkstreams returns all unique workstreams in this project.
func (p *Project) GetWorkstreams() []string {
	wsSet := make(map[string]bool)
	for _, t := range p.Tasks {
		wsSet[t.GetWorkstream()] = true
	}

	workstreams := make([]string, 0, len(wsSet))
	for ws := range wsSet {
		workstreams = append(workstreams, ws)
	}

	sort.Strings(workstreams)
	return workstreams
}

// GetWorkstreamsByRole returns workstreams grouped by role.
func (p *Project) GetWorkstreamsByRole() map[string][]string {
	roleWorkstreams := make(map[string]map[string]bool)

	for _, t := range p.Tasks {
		if roleWorkstreams[t.Role] == nil {
			roleWorkstreams[t.Role] = make(map[string]bool)
		}
		roleWorkstreams[t.Role][t.GetWorkstream()] = true
	}

	result := make(map[string][]string)
	for role, wsSet := range roleWorkstreams {
		workstreams := make([]string, 0, len(wsSet))
		for ws := range wsSet {
			workstreams = append(workstreams, ws)
		}
		sort.Strings(workstreams)
		result[role] = workstreams
	}

	return result
}

// GetRoles returns all unique roles in this project.
func (p *Project) GetRoles() []string {
	roleSet := make(map[string]bool)
	for _, t := range p.Tasks {
		roleSet[t.Role] = true
	}

	roles := make([]string, 0, len(roleSet))
	for role := range roleSet {
		roles = append(roles, role)
	}

	sort.Strings(roles)
	return roles
}

// Stats returns statistics for this project.
func (p *Project) Stats() *ProjectStats {
	stats := &ProjectStats{
		Total:      len(p.Tasks),
		ByStatus:   make(map[task.Status]int),
		ByRole:     make(map[string]int),
		ByPriority: make(map[task.Priority]int),
	}

	for _, t := range p.Tasks {
		stats.ByStatus[t.Status]++
		stats.ByRole[t.Role]++
		stats.ByPriority[t.Priority]++
	}

	return stats
}

// ProjectStats contains project-level statistics.
type ProjectStats struct {
	Total      int
	ByStatus   map[task.Status]int
	ByRole     map[string]int
	ByPriority map[task.Priority]int
}

// ProjectManager handles loading and managing projects.
type ProjectManager struct {
	mu         sync.RWMutex
	tasksDir   string
	projects   map[string]*Project // name -> project
	rootTasks  []*task.Task        // tasks not in any project folder
	taskMgr    *task.Manager
}

// NewProjectManager creates a new ProjectManager.
func NewProjectManager(tasksDir string, taskMgr *task.Manager) *ProjectManager {
	return &ProjectManager{
		tasksDir: tasksDir,
		projects: make(map[string]*Project),
		taskMgr:  taskMgr,
	}
}

// Scan discovers all projects and their tasks.
// It relies on the task manager to scan tasks, then groups them by project.
func (pm *ProjectManager) Scan() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Clear existing data
	pm.projects = make(map[string]*Project)
	pm.rootTasks = nil

	// Scan tasks using the task manager
	tasks, err := pm.taskMgr.Scan()
	if err != nil {
		return fmt.Errorf("scan tasks: %w", err)
	}

	// Group tasks by project
	projectTasks := make(map[string][]*task.Task)
	for _, t := range tasks {
		if t.Project == "" {
			pm.rootTasks = append(pm.rootTasks, t)
		} else {
			projectTasks[t.Project] = append(projectTasks[t.Project], t)
		}
	}

	// Create Project objects for each project folder
	for name, tasks := range projectTasks {
		projectPath := filepath.Join(pm.tasksDir, name)
		desc, _ := pm.parseProjectMd(filepath.Join(projectPath, "project.md"))

		pm.projects[name] = &Project{
			Name:        name,
			Path:        projectPath,
			Description: desc,
			Tasks:       tasks,
		}
	}

	return nil
}

// parseProjectMd reads project.md and extracts the description.
// Returns the first non-header paragraph as description.
func (pm *ProjectManager) parseProjectMd(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(content), "\n")
	var desc strings.Builder
	inDesc := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip empty lines before description
		if !inDesc && line == "" {
			continue
		}

		// Skip headers
		if strings.HasPrefix(line, "#") {
			if inDesc {
				break // End of description section
			}
			continue
		}

		// Capture description lines
		if !inDesc {
			inDesc = true
		}

		if line == "" {
			break // End of first paragraph
		}

		if desc.Len() > 0 {
			desc.WriteString(" ")
		}
		desc.WriteString(line)
	}

	return desc.String(), nil
}

// List returns all discovered projects.
func (pm *ProjectManager) List() []*Project {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	projects := make([]*Project, 0, len(pm.projects))
	for _, p := range pm.projects {
		projects = append(projects, p)
	}

	// Sort by name
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].Name < projects[j].Name
	})

	return projects
}

// Get returns a project by name.
func (pm *ProjectManager) Get(name string) (*Project, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	p, ok := pm.projects[name]
	if !ok {
		return nil, fmt.Errorf("project %q not found", name)
	}
	return p, nil
}

// GetRootTasks returns tasks that are not part of any project.
func (pm *ProjectManager) GetRootTasks() []*task.Task {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	result := make([]*task.Task, len(pm.rootTasks))
	copy(result, pm.rootTasks)
	return result
}

// HasProjects returns true if any project folders exist.
func (pm *ProjectManager) HasProjects() bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return len(pm.projects) > 0
}

// CreateProject creates a new project folder with project.md.
func (pm *ProjectManager) CreateProject(name, description string) (*Project, error) {
	projectPath := filepath.Join(pm.tasksDir, name)

	// Check if already exists
	if _, err := os.Stat(projectPath); err == nil {
		return nil, fmt.Errorf("project %q already exists", name)
	}

	// Create directory
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		return nil, fmt.Errorf("create project directory: %w", err)
	}

	// Create project.md
	projectMd := fmt.Sprintf(`# Project: %s

%s

## Architecture

Describe key components and their relationships here.

## Conventions

- Code style guidelines
- Testing requirements
- Documentation standards

## Context Files

Files agents should understand:
- README.md
- CLAUDE.md (if exists)
`, name, description)

	projectMdPath := filepath.Join(projectPath, "project.md")
	if err := os.WriteFile(projectMdPath, []byte(projectMd), 0644); err != nil {
		return nil, fmt.Errorf("write project.md: %w", err)
	}

	project := &Project{
		Name:        name,
		Path:        projectPath,
		Description: description,
		Tasks:       nil,
	}

	pm.mu.Lock()
	pm.projects[name] = project
	pm.mu.Unlock()

	return project, nil
}

// ProjectExists checks if a project folder exists.
func (pm *ProjectManager) ProjectExists(name string) bool {
	projectPath := filepath.Join(pm.tasksDir, name)
	projectMdPath := filepath.Join(projectPath, "project.md")
	_, err := os.Stat(projectMdPath)
	return err == nil
}

// AgentName returns the standardized agent name for a project and workstream.
// Format: {project}-{workstream} (e.g., "auth-feature-a")
func AgentName(project, workstream string) string {
	// Clean the names: lowercase, replace spaces with hyphens
	project = strings.ToLower(strings.ReplaceAll(project, " ", "-"))
	workstream = strings.ToLower(strings.ReplaceAll(workstream, " ", "-"))
	return fmt.Sprintf("%s-%s", project, workstream)
}

// WorktreeBranch returns the standardized branch name for a project and workstream.
// Format: tanuki/{project}-{workstream}
func WorktreeBranch(project, workstream string) string {
	return fmt.Sprintf("tanuki/%s", AgentName(project, workstream))
}
