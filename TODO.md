# TODO: Project/Workstream Restructure

Restructure tasks to support project folders with multiple concurrent workstreams per project.

## Target Structure

```
tasks/
  auth-feature/
    project.md              # Project context and goals
    001-oauth-login.md      # Task for workstream A
    002-jwt-tokens.md       # Task for workstream A (depends on 001)
    003-session-store.md    # Task for workstream B
    004-refresh-flow.md     # Task for workstream B (depends on 002)
  api-refactor/
    project.md
    001-endpoints.md
    002-middleware.md
    ...
```

## Concepts

- **Project**: A folder in `tasks/` containing `project.md` and task files
- **Workstream**: A named sequence of tasks within a project (e.g., `A`, `B`, `C`)
- **Agent**: One agent per active workstream, named like `{project}-{workstream}`
  (e.g., `auth-feature-a`, `auth-feature-b`)
- **Concurrency**: Configured per role — limits how many workstreams run simultaneously

## Phase 1: Project Folder Support ✓

### 1.1 Update Task Manager to Scan Subdirectories ✓

- [x] Modify `Manager.Scan()` to walk subdirectories of `tasks/`
- [x] Each subdirectory with a `project.md` is treated as a project
- [x] Task files are scanned from project subdirectories (and root `tasks/`)
- [x] Store project name on each Task (derived from folder name)

### 1.2 Add Project Type ✓

- [x] Create `internal/project/project.go` with `Project` struct
- [x] Add `ProjectManager` to scan and load projects
- [x] Parse `project.md` for project-level context

### 1.3 Update CLI Commands ✓

- [x] `tanuki project init <name>` — Create a new project folder with `project.md`
- [x] `tanuki project list` — List all projects
- [x] `tanuki project status [name]` — Show status for one or all projects
- [x] `tanuki project start <name>` — Start workstreams for a specific project

## Phase 2: Workstream Assignment ✓

### 2.1 Workstream Field on Tasks ✓

- [x] Task frontmatter gets `workstream: oauth` (or any name)
- [x] Tasks without explicit workstream default to task ID
- [x] Workstream is scoped to the project

### 2.2 Workstream Discovery ✓

- [x] `Manager.GetWorkstreams(role) []string` — Return unique workstreams
- [x] `Manager.GetProjectWorkstreams(project, role) []string` — Workstreams in project
- [x] `Manager.GetByRoleAndWorkstream(role, ws)` — Get tasks for workstream
- [x] `Manager.GetByProjectAndWorkstream(proj, role, ws)` — Project-scoped

### 2.3 Agent Naming Convention ✓

- [x] Agent name format: `{project}-{workstream}` via `project.AgentName()`
- [x] Worktree branch: `tanuki/{project}-{workstream}` via `project.WorktreeBranch()`

## Phase 3: Dependency-Aware Task Execution ✓

### 3.1 Cross-Workstream Dependencies ✓

- [x] `depends_on` can reference tasks in other workstreams within same project
- [x] Dependencies are resolved by task ID, not workstream

### 3.2 Blocking and Waiting ✓

- [x] Before starting a task, agent checks if all dependencies are `complete`
- [x] `Manager.IsBlocked()` checks dependency status
- [x] `Manager.GetBlockingTasks()` returns incomplete dependencies

### 3.3 Task Lifecycle Updates ✓

- [x] `WorkstreamScheduler.CompleteTask()` updates state
- [x] Status changes persist to task files via `WriteFile()`

## Phase 4: Concurrency Control ✓

### 4.1 Role-Based Concurrency ✓

- [x] Config: `roles.backend.concurrency: 3` in `tanuki.yaml`
- [x] `WorkstreamScheduler.SetRoleConcurrency()` enforces limits

### 4.2 Workstream Scheduling ✓

- [x] `WorkstreamScheduler` tracks active workstreams per role
- [x] `GetNextWorkstream()` respects concurrency limits
- [x] Workstreams queued in priority order

## Phase 5: Agent Integration ✓

### 5.1 Workstream Agent Loop ✓

- [x] `WorkstreamRunner` executes tasks sequentially within a workstream
- [x] Loop:
  1. Get next pending task for this workstream
  2. Check dependencies — if blocked, wait and retry
  3. Execute task via `agent.Manager.Run()`
  4. Mark task complete
  5. Repeat until no more tasks in workstream
- [x] `WorkstreamOrchestrator` manages concurrency limits per role

### 5.2 Agent Startup ✓

- [x] Connect `project start` to actual agent spawning via `agent.Manager`
- [x] Create git worktrees for each `{project}-{workstream}` branch
- [x] Spawn Docker containers with worktree mounts
- [x] Connect `project resume` to restart stopped agents

### 5.3 Progress Reporting (Future Enhancement)

- [ ] Dashboard shows projects → workstreams → tasks hierarchy
- [ ] Status command shows which workstreams are active/waiting/complete
- [ ] Implement `tanuki project status --watch` for live updates

## Migration Notes

- Existing flat `tasks/*.md` structure continues to work (treated as root tasks)
- Projects are opt-in: create a subfolder with `project.md` to use project mode
- Backward compatible: `tanuki project init` without name creates flat structure

## File Changes Summary

| File | Status | Changes |
|------|--------|---------|
| `internal/task/manager.go` | ✓ | Recursive scan, project field on tasks |
| `internal/task/types.go` | ✓ | Add `Project` field to Task |
| `internal/project/project.go` | ✓ | New: Project struct and ProjectManager |
| `internal/project/workstream.go` | ✓ | WorkstreamScheduler with concurrency |
| `internal/cli/project_init.go` | ✓ | Accept project name, create subfolder |
| `internal/cli/project_list.go` | ✓ | New: List all projects |
| `internal/cli/project_start.go` | ✓ | Spawn per-workstream agents |
| `internal/cli/project_status.go` | ✓ | Show project/workstream hierarchy |
| `internal/cli/project_resume.go` | ✓ | Resume specific project |
| `internal/agent/workstream.go` | ✓ | WorkstreamRunner, WorkstreamOrchestrator |
