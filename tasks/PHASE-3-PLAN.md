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

Phase 3 is organized into **3 concurrent workstreams** that can be executed in parallel after the foundation is complete.

```
Phase 3: Task Queue and Project Mode
├── Workstream A: Task Infrastructure (sequential)
│   └── TANK-030 → TANK-033 → TANK-035
│
├── Workstream B: Queue and Distribution (parallel after TANK-030)
│   ├── TANK-034 (Task Queue)
│   ├── TANK-036 (Workload Balancing)
│   └── TANK-037 (Status Tracking)
│
└── Workstream C: Orchestration and Commands (after A + B)
    ├── TANK-031 (Project Commands)
    ├── TANK-032 (Task Completion/Validation)
    └── TANK-038 (Project Orchestrator)
```

---

## Task Breakdown

### Workstream A: Task Infrastructure (sequential)

These tasks build the core task system infrastructure and must be completed first.

| ID | Task | Priority | Estimate | Depends On | Description |
|----|------|----------|----------|------------|-------------|
| TANK-030 | Task File Schema | high | M | TANK-020 | Define markdown + YAML schema for tasks |
| TANK-033 | Task Manager Implementation | high | M | TANK-030 | Core TaskManager with scan/get/update |
| TANK-035 | Dependency Resolver | high | M | TANK-033 | Topological sort, cycle detection |

### Workstream B: Queue and Distribution (parallel after TANK-030)

These tasks implement the queue mechanics. Can be worked on concurrently once TANK-030 is complete.

| ID | Task | Priority | Estimate | Depends On | Description |
|----|------|----------|----------|------------|-------------|
| TANK-034 | Task Queue | high | M | TANK-030 | Priority queue with role-aware dequeue |
| TANK-036 | Workload Balancing | medium | M | TANK-030, TANK-006 | Agent assignment strategy |
| TANK-037 | Status Tracking | medium | S | TANK-030 | Task status updates, history, events |

### Workstream C: Orchestration and Commands (after A + B)

These tasks tie everything together into user-facing functionality.

| ID | Task | Priority | Estimate | Depends On | Description |
|----|------|----------|----------|------------|-------------|
| TANK-031 | Project Commands | high | L | TANK-033, TANK-034 | CLI: project init/start/status/stop |
| TANK-032 | Task Completion and Validation | high | L | TANK-031 | Ralph-style verify, auto-reassign |
| TANK-038 | Project Orchestrator | high | L | TANK-031, TANK-032, TANK-036 | Main control loop, event handling |

---

## Parallelization Guide

### Maximum Parallelism (3 agents)

**Sprint 1: Foundation + Queue Infrastructure**
```
Agent 1: TANK-030 → TANK-033 → TANK-035
Agent 2: (wait for TANK-030) → TANK-034 → TANK-037
Agent 3: (wait for TANK-030) → TANK-036
```

**Sprint 2: Commands and Orchestration**
```
Agent 1: TANK-031 (Project Commands)
Agent 2: TANK-032 (Task Completion)
Agent 3: TANK-038 (Project Orchestrator)
```

### Moderate Parallelism (2 agents)

**Sprint 1: Core Infrastructure**
```
Agent 1: TANK-030 → TANK-033 → TANK-035 → TANK-031
Agent 2: (wait for TANK-030) → TANK-034 → TANK-036 → TANK-037
```

**Sprint 2: Orchestration and Validation**
```
Agent 1: TANK-032 (Task Completion)
Agent 2: TANK-038 (Project Orchestrator)
```

### Sequential Path (1 agent)

```
TANK-030 → TANK-033 → TANK-035 → TANK-034 → TANK-036 → TANK-037
    → TANK-031 → TANK-032 → TANK-038
```

**Minimum viable:** Stop after TANK-031 to have basic `project init/start/status`.

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

| ID | Task | Status | Workstream |
|----|------|--------|------------|
| TANK-030 | Task File Schema | todo | A |
| TANK-033 | Task Manager Implementation | todo | A |
| TANK-035 | Dependency Resolver | todo | A |
| TANK-034 | Task Queue | todo | B |
| TANK-036 | Workload Balancing | todo | B |
| TANK-037 | Status Tracking | todo | B |
| TANK-031 | Project Commands | todo | C |
| TANK-032 | Task Completion and Validation | todo | C |
| TANK-038 | Project Orchestrator | todo | C |

**Total: 9 tasks** (3 existing expanded + 6 new)

**Estimates:**
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
