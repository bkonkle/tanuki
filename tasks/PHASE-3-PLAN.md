# Phase 3: Task Queue and Project Mode

Phase 3 extends Tanuki with automated task distribution across multiple agents. Users can define tasks in markdown files and let Tanuki automatically spawn agents, assign work, validate completion, and reassign when agents become idle.

## Goals

1. **Task File System** - Define tasks in markdown with YAML front matter, including completion criteria
2. **Task Queue Management** - Intelligent queue with dependency resolution and priority handling
3. **Automatic Distribution** - Spawn agents by role and auto-assign tasks based on availability
4. **Completion Validation** - Use Ralph-style verification (commands or signals) to validate work
5. **Workload Balancing** - Distribute tasks evenly across available agents
6. **Status Dashboard** - Real-time visibility into project progress

## Architecture Overview

```
┌────────────────────────────────────────────────────────────────────────────┐
│                         Tanuki Phase 3                                      │
├────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌──────────────┐   ┌──────────────┐   ┌──────────────┐                   │
│  │ Task File    │   │ Task Manager │   │ Dependency   │                   │
│  │ Parser       │──▶│              │◀──│ Resolver     │                   │
│  │              │   │              │   │              │                   │
│  └──────────────┘   └──────────────┘   └──────────────┘                   │
│                            │                                               │
│                            ▼                                               │
│  ┌──────────────┐   ┌──────────────┐   ┌──────────────┐                   │
│  │ Task Queue   │◀──│ Workload     │──▶│ Status       │                   │
│  │              │   │ Balancer     │   │ Tracker      │                   │
│  └──────────────┘   └──────────────┘   └──────────────┘                   │
│                            │                                               │
│                            ▼                                               │
│                   ┌──────────────┐                                         │
│                   │ Project      │                                         │
│                   │ Orchestrator │                                         │
│                   └──────────────┘                                         │
│                            │                                               │
└────────────────────────────┴────────────────────────────────────────────────┘
                             │
                             ▼
          ┌─────────────────────────────────────────────┐
          │            Phase 1 + Phase 2                 │
          │  ┌─────────────┐   ┌─────────────┐          │
          │  │ Agent       │   │ Role        │          │
          │  │ Manager     │   │ Manager     │          │
          │  └─────────────┘   └─────────────┘          │
          └─────────────────────────────────────────────┘
```

## Workstreams

Phase 3 is organized into **3 fully independent workstreams** that can be executed in parallel
from the start. Each workstream owns a complete vertical slice with no cross-dependencies.

```
Phase 3: Task Queue and Project Mode
├── Workstream A: Task Engine (internal/task/ core)
│   └── TANK-030 (Schema) + TANK-033 (Manager) + TANK-035 (Dependencies)
│
├── Workstream B: Queue + Balancer + Status (standalone components)
│   └── TANK-034 (Queue) + TANK-036 (Balancer) + TANK-037 (Status)
│
└── Workstream C: Project Commands + Orchestrator (interface-based)
    └── TANK-031 (Commands) + TANK-032 (Validation) + TANK-038 (Orchestrator)
```

---

## Task Breakdown

### Workstream A: Task Engine

Complete task infrastructure — parser, manager, and dependency resolution. No queue or CLI code.

| ID       | Task                        | Priority | Est | WS | Description                    |
|----------|-----------------------------| ---------|-----|----|--------------------------------|
| TANK-030 | Task File Schema            | high     | M   | A  | Task struct, markdown parser   |
| TANK-033 | Task Manager Implementation | high     | M   | A  | Scan/get/update/assign tasks   |
| TANK-035 | Dependency Resolver         | high     | M   | A  | Topological sort, cycle detect |

**Scope:** `internal/task/schema.go`, `internal/task/parser.go`, `internal/task/manager.go`,
`internal/task/dependency.go` + tests

### Workstream B: Queue + Balancer + Status

Standalone queue mechanics and workload distribution. No external dependencies.

| ID       | Task               | Priority | Est | WS | Description                 |
|----------|--------------------| ---------|-----|----|------------------------------|
| TANK-034 | Task Queue         | high     | M   | B  | Priority queue, role-aware  |
| TANK-036 | Workload Balancing | medium   | M   | B  | Agent assignment strategy   |
| TANK-037 | Status Tracking    | medium   | S   | B  | Status transitions, history |

**Scope:** `internal/task/queue.go`, `internal/task/balancer.go`, `internal/task/status.go` + tests

### Workstream C: Project Commands + Orchestrator

CLI commands and orchestration loop, coded against interfaces (not concrete implementations).

| ID       | Task                           | Priority | Est | WS | Description                    |
|----------|--------------------------------| ---------|-----|----|--------------------------------|
| TANK-031 | Project Commands               | high     | L   | C  | project init/start/status/stop |
| TANK-032 | Task Completion and Validation | high     | L   | C  | Ralph-style verify, reassign   |
| TANK-038 | Project Orchestrator           | high     | L   | C  | Main control loop, events      |

**Scope:** `internal/cli/project.go`, `internal/project/orchestrator.go`,
`internal/project/validator.go`, `internal/project/status.go` + tests

---

## Parallelization Guide

### True Parallel Execution (3 tabs/agents)

All workstreams start immediately with no waiting:

```text
Tab A (Task Engine):    TANK-030 → TANK-033 → TANK-035
Tab B (Queue+Balancer): TANK-034, TANK-036, TANK-037
Tab C (Project CLI):    TANK-031, TANK-032, TANK-038
```

**Integration Phase:** After all tabs complete, one agent integrates:

- Wire TaskManager into orchestrator
- Connect queue and balancer to orchestrator loop
- Run end-to-end tests with sample task files

### Sequential Path (1 agent)

```text
TANK-030 → TANK-033 → TANK-035 → TANK-034 → TANK-036 → TANK-037
    → TANK-031 → TANK-032 → TANK-038
```

Minimum viable: Stop after TANK-033 + TANK-031 for basic `project init/start/status`.

---

## Key Concepts

### Task File Format

Tasks are markdown files with YAML front matter in `.tanuki/tasks/`:

```markdown
---
id: TASK-001
title: Implement User Authentication
role: backend
priority: high
status: pending
depends_on: []

completion:
  verify: "npm test -- --grep 'auth'"
  signal: "AUTH_COMPLETE"
  max_iterations: 20

tags:
  - auth
  - security
---

# Implement User Authentication

Add OAuth2-based authentication to the API.

## Requirements

1. **OAuth2 Flow** - Google as identity provider
2. **JWT Tokens** - 15min access, 7day refresh
...
```

### Completion Criteria (Ralph-style)

Tasks can specify how to verify completion:

1. **Verify Command** - A command that must exit 0
   ```yaml
   completion:
     verify: "npm test"
   ```

2. **Signal Detection** - Look for a string in output
   ```yaml
   completion:
     signal: "TASK_COMPLETE"
   ```

3. **Both** - Command AND signal (most reliable)
   ```yaml
   completion:
     verify: "npm test"
     signal: "ALL_TESTS_PASS"
   ```

### Task States

```
pending → assigned → in_progress → [review|complete|failed]
                          ↓
                       blocked
```

| State | Description |
|-------|-------------|
| `pending` | Not yet started, waiting for assignment |
| `assigned` | Agent assigned but not yet started |
| `in_progress` | Agent actively working |
| `review` | Work done, needs human review |
| `complete` | Verified and done |
| `failed` | Failed and needs attention |
| `blocked` | Dependencies not satisfied |

### Workflow

```bash
# 1. Initialize project tasks
tanuki project init

# 2. Create task files in .tanuki/tasks/
#    (or copy from templates)

# 3. Start the project
tanuki project start
#    - Scans for tasks
#    - Spawns agents by role
#    - Assigns tasks automatically

# 4. Monitor progress
tanuki project status

# 5. View specific agent work
tanuki logs backend-agent
tanuki diff backend-agent

# 6. Stop when done
tanuki project stop
```

---

## Integration with Phase 1 and Phase 2

### Phase 1 Dependencies

| Component | Usage |
|-----------|-------|
| Agent Manager | Spawn/stop agents, run tasks |
| State Manager | Track task assignments in agent state |
| Docker Manager | Container lifecycle for project agents |
| Git Manager | Worktrees for task isolation |
| Claude Executor | Execute tasks with Ralph mode |

### Phase 2 Dependencies

| Component | Usage |
|-----------|-------|
| Role Manager | Match tasks to agent roles |
| Role Config | Task `role` field maps to Role definitions |
| Context Files | Automatically provided to task agents |
| Tool Filtering | Respect role tool restrictions |

### New Interfaces

```go
// Task Manager (Phase 3 core)
type TaskManager interface {
    Scan(dir string) ([]*Task, error)
    Get(id string) (*Task, error)
    GetByRole(role string) ([]*Task, error)
    GetNextAvailable(role string) (*Task, error)
    UpdateStatus(id string, status string) error
    Assign(id string, agentName string) error
}

// Task Queue
type TaskQueue interface {
    Enqueue(task *Task) error
    Dequeue(role string) (*Task, error)
    Peek(role string) (*Task, error)
    Size() int
    SizeByRole(role string) int
}

// Workload Balancer
type WorkloadBalancer interface {
    AssignTask(task *Task, agents []*Agent) (*Agent, error)
    GetIdleAgents(role string) ([]*Agent, error)
    GetWorkload(agentName string) int
}

// Project Orchestrator
type ProjectOrchestrator interface {
    Start(ctx context.Context) error
    Stop() error
    Status() *ProjectStatus
    OnTaskComplete(taskID string)
    OnAgentIdle(agentName string)
}
```

---

## Task Status Summary

| ID       | Task                           | Status | WS | Description                    |
|----------|--------------------------------|--------|----|--------------------------------|
| TANK-030 | Task File Schema               | todo   | A  | Task struct, markdown parser   |
| TANK-033 | Task Manager Implementation    | todo   | A  | Scan/get/update/assign tasks   |
| TANK-035 | Dependency Resolver            | todo   | A  | Topological sort, cycle detect |
| TANK-034 | Task Queue                     | todo   | B  | Priority queue, role-aware     |
| TANK-036 | Workload Balancing             | todo   | B  | Agent assignment strategy      |
| TANK-037 | Status Tracking                | todo   | B  | Status transitions, history    |
| TANK-031 | Project Commands               | todo   | C  | project init/start/status/stop |
| TANK-032 | Task Completion and Validation | todo   | C  | Ralph-style verify, reassign   |
| TANK-038 | Project Orchestrator           | todo   | C  | Main control loop, events      |

Total: 9 tasks

By Workstream:

- A (Task Engine): 3 tasks (TANK-030, TANK-033, TANK-035)
- B (Queue+Balancer): 3 tasks (TANK-034, TANK-036, TANK-037)
- C (Project CLI): 3 tasks (TANK-031, TANK-032, TANK-038)

Estimates:

- Small (S): 1 task
- Medium (M): 5 tasks
- Large (L): 3 tasks

---

## Success Metrics

Phase 3 is successful when:

1. Users can create task files and run `tanuki project start`
2. Agents are spawned automatically based on task roles
3. Tasks are assigned respecting dependencies and priorities
4. Completion is verified via commands or signals (Ralph-style)
5. Idle agents automatically pick up next available task
6. `tanuki project status` shows real-time progress
7. Project can be stopped and resumed cleanly

## Example Session

```bash
$ cd my-project

$ tanuki project init
Created .tanuki/tasks/
Created example task: .tanuki/tasks/TASK-001-example.md

$ ls .tanuki/tasks/
TASK-001-auth.md
TASK-002-api.md
TASK-003-frontend.md
TASK-004-tests.md

$ tanuki project status
Project: my-project
Tasks: 4 total (4 pending, 0 in progress, 0 complete)

ID         TITLE                   ROLE       STATUS    ASSIGNED
--         -----                   ----       ------    --------
TASK-001   User Authentication     backend    pending   -
TASK-002   API Refactor            backend    pending   -
TASK-003   Dashboard UI            frontend   pending   -
TASK-004   Integration Tests       qa         pending   -

$ tanuki project start
Scanning tasks...
  Found 4 tasks across 3 roles

Spawning agents...
  ✓ backend-agent (role: backend)
  ✓ frontend-agent (role: frontend)
  ✓ qa-agent (role: qa)

Assigning tasks...
  TASK-001 → backend-agent
  TASK-003 → frontend-agent
  (TASK-002, TASK-004 waiting for dependencies)

Project started! Monitor with: tanuki project status

$ tanuki project status
Project: my-project
Tasks: 4 total (1 pending, 2 in progress, 1 blocked)

Agents:
  backend-agent (working) → TASK-001
  frontend-agent (working) → TASK-003
  qa-agent (idle)

ID         TITLE                   ROLE       STATUS        ASSIGNED
--         -----                   ----       ------        --------
TASK-001   User Authentication     backend    in_progress   backend-agent
TASK-002   API Refactor            backend    blocked       -
TASK-003   Dashboard UI            frontend   in_progress   frontend-agent
TASK-004   Integration Tests       qa         pending       -

# Later...

$ tanuki project status
Project: my-project
Tasks: 4 total (0 pending, 1 in progress, 3 complete)

Agents:
  backend-agent (working) → TASK-002
  frontend-agent (idle)
  qa-agent (working) → TASK-004

ID         TITLE                   ROLE       STATUS        ASSIGNED
--         -----                   ----       ------        --------
TASK-001   User Authentication     backend    complete      -
TASK-002   API Refactor            backend    in_progress   backend-agent
TASK-003   Dashboard UI            frontend   complete      -
TASK-004   Integration Tests       qa         in_progress   qa-agent

$ tanuki project stop
Stopping all project agents...
  ✓ Stopped backend-agent
  ✓ Stopped frontend-agent
  ✓ Stopped qa-agent

Project stopped. Resume with: tanuki project start
```
