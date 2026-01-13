---
id: TANK-031
title: Project Commands
status: todo
priority: high
estimate: L
depends_on: [TANK-033, TANK-034]
workstream: C
phase: 3
---

# Project Commands

## Summary

Implement project-level CLI commands for managing task-driven agent workflows. These commands provide the user interface for initializing, starting, monitoring, and stopping project-based task distribution.

## Acceptance Criteria

- [ ] `tanuki project init` - Create task directory structure with example
- [ ] `tanuki project status` - Show all tasks, agents, and progress
- [ ] `tanuki project start` - Spawn agents by role and assign tasks
- [ ] `tanuki project stop` - Stop all project agents gracefully
- [ ] `tanuki project resume` - Resume stopped project
- [ ] Auto-spawn agents based on task roles
- [ ] Integration with TaskManager and TaskQueue
- [ ] Clear, informative output for all commands
- [ ] Unit tests for command logic

## Technical Details

### Command Structure

```go
// cmd/project.go
package cmd

import "github.com/spf13/cobra"

var projectCmd = &cobra.Command{
    Use:   "project",
    Short: "Manage project tasks and agents",
    Long:  `Project mode enables automatic task distribution across multiple agents.`,
}

func init() {
    rootCmd.AddCommand(projectCmd)
    projectCmd.AddCommand(projectInitCmd)
    projectCmd.AddCommand(projectStatusCmd)
    projectCmd.AddCommand(projectStartCmd)
    projectCmd.AddCommand(projectStopCmd)
    projectCmd.AddCommand(projectResumeCmd)
}
```

### Project Init

```go
// cmd/project_init.go
var projectInitCmd = &cobra.Command{
    Use:   "init",
    Short: "Initialize project task structure",
    Long:  `Creates .tanuki/tasks/ directory and an example task file.`,
    RunE:  runProjectInit,
}

func runProjectInit(cmd *cobra.Command, args []string) error {
    cfg, err := config.Load()
    if err != nil {
        return fmt.Errorf("load config: %w", err)
    }

    taskDir := filepath.Join(cfg.ProjectRoot, ".tanuki", "tasks")

    // Check if already initialized
    if _, err := os.Stat(taskDir); err == nil {
        fmt.Println("Project tasks already initialized.")
        fmt.Printf("Task directory: %s\n", taskDir)
        return nil
    }

    // Create task directory
    if err := os.MkdirAll(taskDir, 0755); err != nil {
        return fmt.Errorf("create task directory: %w", err)
    }

    // Create example task
    exampleTask := `---
id: TASK-001
title: Example Task
role: backend
priority: medium
status: pending
depends_on: []

completion:
  verify: "echo 'Task complete'"
  signal: "TASK_DONE"
---

# Example Task

This is an example task file. Replace this with your actual task.

## Requirements

1. First requirement
2. Second requirement

## Done When

- All requirements are implemented
- Tests pass
- Say TASK_DONE when finished
`

    examplePath := filepath.Join(taskDir, "TASK-001-example.md")
    if err := os.WriteFile(examplePath, []byte(exampleTask), 0644); err != nil {
        return fmt.Errorf("write example task: %w", err)
    }

    fmt.Println("Initialized project tasks")
    fmt.Printf("  Created: %s\n", taskDir)
    fmt.Printf("  Example: %s\n", examplePath)
    fmt.Println()
    fmt.Println("Next steps:")
    fmt.Println("  1. Create task files in .tanuki/tasks/")
    fmt.Println("  2. Run: tanuki project status")
    fmt.Println("  3. Run: tanuki project start")

    return nil
}
```

### Project Status

```go
// cmd/project_status.go
var projectStatusCmd = &cobra.Command{
    Use:   "status",
    Short: "Show project status",
    Long:  `Displays all tasks, their status, and assigned agents.`,
    RunE:  runProjectStatus,
}

func init() {
    projectStatusCmd.Flags().BoolP("watch", "w", false, "Watch for changes")
}

func runProjectStatus(cmd *cobra.Command, args []string) error {
    cfg, _ := config.Load()
    taskMgr := task.NewManager(cfg)
    agentMgr := agent.NewManager(cfg)

    tasks, err := taskMgr.Scan()
    if err != nil {
        return fmt.Errorf("scan tasks: %w", err)
    }

    if len(tasks) == 0 {
        fmt.Println("No tasks found.")
        fmt.Println("Create tasks in .tanuki/tasks/ or run: tanuki project init")
        return nil
    }

    // Count by status
    counts := make(map[task.Status]int)
    for _, t := range tasks {
        counts[t.Status]++
    }

    // Print summary
    fmt.Printf("Project: %s\n", filepath.Base(cfg.ProjectRoot))
    fmt.Printf("Tasks: %d total (%d pending, %d in progress, %d complete)\n",
        len(tasks),
        counts[task.StatusPending]+counts[task.StatusBlocked],
        counts[task.StatusInProgress]+counts[task.StatusAssigned],
        counts[task.StatusComplete],
    )
    fmt.Println()

    // Print agents
    agents, _ := agentMgr.List()
    projectAgents := filterProjectAgents(agents)

    if len(projectAgents) > 0 {
        fmt.Println("Agents:")
        for _, ag := range projectAgents {
            statusIcon := getStatusIcon(ag.Status)
            taskInfo := ""
            if ag.CurrentTask != "" {
                taskInfo = fmt.Sprintf(" → %s", ag.CurrentTask)
            }
            fmt.Printf("  %s %s (%s)%s\n", statusIcon, ag.Name, ag.Status, taskInfo)
        }
        fmt.Println()
    }

    // Print task table
    w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
    fmt.Fprintln(w, "ID\tTITLE\tROLE\tPRIORITY\tSTATUS\tASSIGNED")
    fmt.Fprintln(w, "--\t-----\t----\t--------\t------\t--------")

    // Sort by priority, then status
    sortTasks(tasks)

    for _, t := range tasks {
        assigned := "-"
        if t.AssignedTo != "" {
            assigned = t.AssignedTo
        }
        fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
            t.ID,
            truncate(t.Title, 30),
            t.Role,
            t.Priority,
            t.Status,
            assigned,
        )
    }

    return w.Flush()
}

func getStatusIcon(status string) string {
    switch status {
    case "working":
        return "●"
    case "idle":
        return "○"
    default:
        return "◌"
    }
}

func truncate(s string, max int) string {
    if len(s) <= max {
        return s
    }
    return s[:max-3] + "..."
}
```

### Project Start

```go
// cmd/project_start.go
var projectStartCmd = &cobra.Command{
    Use:   "start",
    Short: "Start agents and assign tasks",
    Long: `Spawns agents for each role needed and assigns pending tasks.

By default, spawns one agent per role. Use --agents-per-role to spawn more.`,
    RunE: runProjectStart,
}

func init() {
    projectStartCmd.Flags().IntP("agents-per-role", "n", 1, "Number of agents per role")
    projectStartCmd.Flags().Bool("dry-run", false, "Show what would happen without doing it")
}

func runProjectStart(cmd *cobra.Command, args []string) error {
    cfg, _ := config.Load()
    taskMgr := task.NewManager(cfg)
    agentMgr := agent.NewManager(cfg)
    queue := task.NewQueue()

    agentsPerRole, _ := cmd.Flags().GetInt("agents-per-role")
    dryRun, _ := cmd.Flags().GetBool("dry-run")

    // Scan tasks
    fmt.Println("Scanning tasks...")
    tasks, err := taskMgr.Scan()
    if err != nil {
        return fmt.Errorf("scan tasks: %w", err)
    }

    if len(tasks) == 0 {
        fmt.Println("No tasks found. Run: tanuki project init")
        return nil
    }

    // Determine roles needed
    rolesNeeded := make(map[string]int)
    pendingTasks := 0
    for _, t := range tasks {
        if t.Status == task.StatusPending || t.Status == task.StatusBlocked {
            rolesNeeded[t.Role]++
            pendingTasks++
        }
    }

    if pendingTasks == 0 {
        fmt.Println("No pending tasks.")
        return nil
    }

    fmt.Printf("  Found %d tasks across %d roles\n", len(tasks), len(rolesNeeded))
    fmt.Println()

    if dryRun {
        fmt.Println("[DRY RUN] Would spawn:")
        for role, count := range rolesNeeded {
            fmt.Printf("  %s-agent (role: %s) - %d tasks\n", role, role, count)
        }
        return nil
    }

    // Spawn agents for each role
    fmt.Println("Spawning agents...")
    for role := range rolesNeeded {
        for i := 0; i < agentsPerRole; i++ {
            agentName := role + "-agent"
            if agentsPerRole > 1 {
                agentName = fmt.Sprintf("%s-agent-%d", role, i+1)
            }

            // Check if agent already exists
            existing, _ := agentMgr.Get(agentName)
            if existing != nil {
                if existing.Status == "stopped" {
                    fmt.Printf("  Starting existing agent %s...\n", agentName)
                    agentMgr.Start(agentName)
                } else {
                    fmt.Printf("  Agent %s already running\n", agentName)
                }
                continue
            }

            fmt.Printf("  Spawning %s (role: %s)...\n", agentName, role)
            _, err := agentMgr.Spawn(agentName, agent.SpawnOptions{Role: role})
            if err != nil {
                fmt.Printf("    Failed: %v\n", err)
                continue
            }
            fmt.Printf("  ✓ %s\n", agentName)
        }
    }
    fmt.Println()

    // Build queue with pending tasks
    for _, t := range tasks {
        if t.Status == task.StatusPending {
            queue.Enqueue(t)
        }
    }

    // Assign tasks to idle agents
    fmt.Println("Assigning tasks...")
    agents, _ := agentMgr.List()
    assigned := 0

    for _, ag := range agents {
        if ag.Status != "idle" || ag.Role == "" {
            continue
        }

        t, err := queue.Dequeue(ag.Role)
        if err != nil {
            continue // No tasks for this role
        }

        // Check dependencies
        if blocked, _ := taskMgr.IsBlocked(t.ID); blocked {
            fmt.Printf("  %s blocked (waiting on dependencies)\n", t.ID)
            t.Status = task.StatusBlocked
            taskMgr.UpdateStatus(t.ID, task.StatusBlocked)
            continue
        }

        fmt.Printf("  %s → %s\n", t.ID, ag.Name)
        taskMgr.Assign(t.ID, ag.Name)
        assigned++

        // Build prompt and start task
        prompt := buildTaskPrompt(t)
        go agentMgr.Run(ag.Name, prompt, agent.RunOptions{
            Ralph: t.IsRalphMode(),
        })
    }

    if assigned == 0 {
        fmt.Println("  No tasks assigned (all agents busy or no matching tasks)")
    }

    fmt.Println()
    fmt.Println("Project started!")
    fmt.Println("Monitor with: tanuki project status")

    return nil
}

func buildTaskPrompt(t *task.Task) string {
    var prompt strings.Builder

    prompt.WriteString(fmt.Sprintf("# Task: %s\n\n", t.Title))
    prompt.WriteString(t.Content)

    if t.Completion != nil {
        prompt.WriteString("\n\n## Completion Criteria\n\n")
        if t.Completion.Verify != "" {
            prompt.WriteString(fmt.Sprintf("Run this command to verify: `%s`\n", t.Completion.Verify))
        }
        if t.Completion.Signal != "" {
            prompt.WriteString(fmt.Sprintf("Say **%s** when complete.\n", t.Completion.Signal))
        }
    }

    return prompt.String()
}
```

### Project Stop

```go
// cmd/project_stop.go
var projectStopCmd = &cobra.Command{
    Use:   "stop",
    Short: "Stop all project agents",
    Long:  `Gracefully stops all agents spawned for this project.`,
    RunE:  runProjectStop,
}

func init() {
    projectStopCmd.Flags().Bool("remove", false, "Remove agents after stopping")
}

func runProjectStop(cmd *cobra.Command, args []string) error {
    cfg, _ := config.Load()
    agentMgr := agent.NewManager(cfg)
    taskMgr := task.NewManager(cfg)
    remove, _ := cmd.Flags().GetBool("remove")

    fmt.Println("Stopping project agents...")

    agents, _ := agentMgr.List()
    projectAgents := filterProjectAgents(agents)

    if len(projectAgents) == 0 {
        fmt.Println("No project agents running.")
        return nil
    }

    for _, ag := range projectAgents {
        // Unassign any current task
        if ag.CurrentTask != "" {
            taskMgr.UpdateStatus(ag.CurrentTask, task.StatusPending)
            taskMgr.Unassign(ag.CurrentTask)
        }

        if remove {
            fmt.Printf("  Removing %s...\n", ag.Name)
            if err := agentMgr.Remove(ag.Name, agent.RemoveOptions{Force: true}); err != nil {
                fmt.Printf("    Failed: %v\n", err)
                continue
            }
        } else {
            fmt.Printf("  Stopping %s...\n", ag.Name)
            if err := agentMgr.Stop(ag.Name); err != nil {
                fmt.Printf("    Failed: %v\n", err)
                continue
            }
        }
        fmt.Printf("  ✓ %s\n", ag.Name)
    }

    fmt.Println()
    if remove {
        fmt.Println("Project agents removed.")
    } else {
        fmt.Println("Project stopped. Resume with: tanuki project start")
    }

    return nil
}

func filterProjectAgents(agents []*agent.Agent) []*agent.Agent {
    // Project agents are those with roles assigned
    var result []*agent.Agent
    for _, ag := range agents {
        if ag.Role != "" {
            result = append(result, ag)
        }
    }
    return result
}
```

### Project Resume

```go
// cmd/project_resume.go
var projectResumeCmd = &cobra.Command{
    Use:   "resume",
    Short: "Resume a stopped project",
    Long:  `Restarts stopped agents and reassigns incomplete tasks.`,
    RunE:  runProjectResume,
}

func runProjectResume(cmd *cobra.Command, args []string) error {
    // Similar to start, but:
    // 1. Start existing stopped agents instead of spawning new
    // 2. Reassign tasks that were in_progress back to pending
    // 3. Then assign as normal

    cfg, _ := config.Load()
    taskMgr := task.NewManager(cfg)
    agentMgr := agent.NewManager(cfg)

    // Reset in_progress tasks to pending
    tasks, _ := taskMgr.Scan()
    for _, t := range tasks {
        if t.Status == task.StatusInProgress || t.Status == task.StatusAssigned {
            taskMgr.UpdateStatus(t.ID, task.StatusPending)
            taskMgr.Unassign(t.ID)
        }
    }

    // Start stopped agents
    agents, _ := agentMgr.List()
    for _, ag := range agents {
        if ag.Role != "" && ag.Status == "stopped" {
            fmt.Printf("Starting %s...\n", ag.Name)
            agentMgr.Start(ag.Name)
        }
    }

    // Delegate to start for assignment logic
    return runProjectStart(cmd, args)
}
```

### Output Examples

```
$ tanuki project init
Initialized project tasks
  Created: .tanuki/tasks/
  Example: .tanuki/tasks/TASK-001-example.md

Next steps:
  1. Create task files in .tanuki/tasks/
  2. Run: tanuki project status
  3. Run: tanuki project start
```

```
$ tanuki project status
Project: my-app
Tasks: 5 total (2 pending, 2 in progress, 1 complete)

Agents:
  ● backend-agent (working) → TASK-002
  ● frontend-agent (working) → TASK-003
  ○ qa-agent (idle)

ID         TITLE                          ROLE       PRIORITY  STATUS        ASSIGNED
--         -----                          ----       --------  ------        --------
TASK-001   Implement Authentication       backend    high      complete      -
TASK-002   Add API Endpoints              backend    high      in_progress   backend-agent
TASK-003   Build Dashboard UI             frontend   high      in_progress   frontend-agent
TASK-004   Integration Tests              qa         medium    pending       -
TASK-005   Update Documentation           docs       low       pending       -
```

```
$ tanuki project start
Scanning tasks...
  Found 5 tasks across 4 roles

Spawning agents...
  Spawning backend-agent (role: backend)...
  ✓ backend-agent
  Spawning frontend-agent (role: frontend)...
  ✓ frontend-agent
  Spawning qa-agent (role: qa)...
  ✓ qa-agent
  Spawning docs-agent (role: docs)...
  ✓ docs-agent

Assigning tasks...
  TASK-001 → backend-agent
  TASK-003 → frontend-agent
  TASK-004 blocked (waiting on dependencies)
  TASK-005 → docs-agent

Project started!
Monitor with: tanuki project status
```

```
$ tanuki project stop
Stopping project agents...
  Stopping backend-agent...
  ✓ backend-agent
  Stopping frontend-agent...
  ✓ frontend-agent
  Stopping qa-agent...
  ✓ qa-agent

Project stopped. Resume with: tanuki project start
```

## Error Handling

| Scenario | Behavior |
|----------|----------|
| No tasks found | Suggest running `project init` |
| Role spawn fails | Log error, continue with other roles |
| Task assignment fails | Log error, continue with other tasks |
| Agent already exists | Reuse existing agent |
| All agents busy | Report "no tasks assigned" |

## Testing

### Unit Tests

```go
func TestBuildTaskPrompt(t *testing.T) {
    task := &task.Task{
        Title:   "Test Task",
        Content: "Do the thing.",
        Completion: &task.CompletionConfig{
            Verify: "npm test",
            Signal: "DONE",
        },
    }

    prompt := buildTaskPrompt(task)

    if !strings.Contains(prompt, "# Task: Test Task") {
        t.Error("prompt missing title")
    }
    if !strings.Contains(prompt, "npm test") {
        t.Error("prompt missing verify command")
    }
    if !strings.Contains(prompt, "DONE") {
        t.Error("prompt missing signal")
    }
}
```

### Integration Tests

```bash
# Test init
tanuki project init
ls .tanuki/tasks/TASK-001-example.md

# Test status with no tasks
rm -rf .tanuki/tasks/*
tanuki project status  # Should suggest init

# Test start dry run
tanuki project start --dry-run

# Test full workflow
tanuki project init
# Create real tasks...
tanuki project start
tanuki project status
tanuki project stop
```

## Out of Scope

- Web-based dashboard (Phase 4: TUI)
- Task creation commands
- Parallel task assignment per role
- Custom agent naming patterns

## Notes

The project commands are the primary user interface for Phase 3. They should be intuitive and provide clear feedback at each step. Error messages should be actionable.
