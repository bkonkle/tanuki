# Phase 4: Shared Services and Advanced Features

Phase 4 extends Tanuki with shared infrastructure services (databases, caches) and an interactive TUI dashboard. This phase focuses on developer experience improvements and supporting more complex multi-agent workflows that require shared state.

## Goals

1. **Shared Services** - Run Postgres, Redis, and custom services that agents can connect to
2. **Service Discovery** - Automatic connection info injection into agent containers
3. **TUI Dashboard** - Interactive terminal interface for monitoring and control
4. **Real-time Updates** - Live status, logs, and task progress visualization
5. **Quick Actions** - Start/stop/attach/merge directly from the dashboard

## Architecture Overview

```
┌────────────────────────────────────────────────────────────────────────────┐
│                         Tanuki Phase 4                                      │
├────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌──────────────────────────────────────────────────────────────────────┐  │
│  │                    Shared Services Layer                              │  │
│  │  ┌──────────────┐   ┌──────────────┐   ┌──────────────┐             │  │
│  │  │ Service      │   │ Service      │   │ Health       │             │  │
│  │  │ Manager      │──▶│ Registry     │──▶│ Monitor      │             │  │
│  │  │              │   │              │   │              │             │  │
│  │  └──────────────┘   └──────────────┘   └──────────────┘             │  │
│  └──────────────────────────────────────────────────────────────────────┘  │
│                                                                             │
│  ┌──────────────────────────────────────────────────────────────────────┐  │
│  │                    TUI Dashboard Layer                                │  │
│  │  ┌──────────────┐   ┌──────────────┐   ┌──────────────┐             │  │
│  │  │ Dashboard    │   │ Pane         │   │ Log          │             │  │
│  │  │ Model        │──▶│ Manager      │──▶│ Streamer     │             │  │
│  │  │ (BubbleTea)  │   │              │   │              │             │  │
│  │  └──────────────┘   └──────────────┘   └──────────────┘             │  │
│  └──────────────────────────────────────────────────────────────────────┘  │
│                                                                             │
└────────────────────────────────────────────────────────────────────────────┘
                              │
                              ▼
          ┌─────────────────────────────────────────────────┐
          │            Phase 1 + Phase 2 + Phase 3           │
          │  ┌───────────┐  ┌───────────┐  ┌───────────┐   │
          │  │ Agent     │  │ Role      │  │ Project   │   │
          │  │ Manager   │  │ Manager   │  │ Orchestr. │   │
          │  └───────────┘  └───────────┘  └───────────┘   │
          └─────────────────────────────────────────────────┘
```

## Workstreams

Phase 4 is organized into **3 fully independent workstreams** that can be executed in parallel
from the start. Each workstream owns a complete vertical slice with no cross-dependencies.

```text
Phase 4: Shared Services and Advanced Features
├── Workstream A: Service Engine (internal/service/ core)
│   └── TANK-040 (Manager) + TANK-042 (Config) + TANK-043 (Health)
│
├── Workstream B: Service CLI + Agent Injection (interface-based)
│   └── TANK-044 (Commands) + TANK-045 (Injection)
│
└── Workstream C: TUI Dashboard (fully independent)
    └── TANK-041 (Framework) + TANK-046 (Agents) + TANK-047 (Tasks) + TANK-048 (Logs)
```

---

## Task Breakdown

### Workstream A: Service Engine

Complete service infrastructure — manager, config schema, and health monitoring. No CLI code.

| ID       | Task                         | Priority | Est | WS | Description                   |
|----------|------------------------------| ---------|-----|----|-------------------------------|
| TANK-040 | Service Manager Core         | medium   | L   | A  | Service lifecycle management  |
| TANK-042 | Service Configuration Schema | medium   | M   | A  | YAML schema in tanuki.yaml    |
| TANK-043 | Service Health Monitoring    | medium   | M   | A  | Health check loop, restart    |

**Scope:** `internal/service/manager.go`, `internal/service/schema.go`,
`internal/service/health.go`, `internal/service/types.go` + tests

### Workstream B: Service CLI + Agent Injection

CLI commands and container integration, coded against interfaces.

| ID       | Task                    | Priority | Est | WS | Description                  |
|----------|-------------------------| ---------|-----|----|------------------------------|
| TANK-044 | Service Commands        | medium   | S   | B  | service start/stop/status    |
| TANK-045 | Agent Service Injection | medium   | M   | B  | Inject connection env vars   |

**Scope:** `internal/cli/service.go`, `internal/service/inject.go`,
updates to `internal/docker/container.go` and `internal/agent/manager.go`

### Workstream C: TUI Dashboard

Complete BubbleTea dashboard — fully independent of other Phase 4 work.

| ID       | Task                 | Priority | Est | WS | Description                  |
|----------|----------------------| ---------|-----|----|------------------------------|
| TANK-041 | Dashboard Framework  | low      | L   | C  | BubbleTea setup, layout      |
| TANK-046 | Dashboard Agent Pane | low      | M   | C  | Agent list with status       |
| TANK-047 | Dashboard Task Pane  | low      | M   | C  | Task list with filtering     |
| TANK-048 | Dashboard Log Pane   | low      | L   | C  | Real-time log streaming      |

**Scope:** `internal/tui/dashboard.go`, `internal/tui/agents.go`, `internal/tui/tasks.go`,
`internal/tui/logs.go`, `internal/tui/styles.go`, `internal/cli/dashboard.go` + tests

---

## Detailed Task Specifications

### TANK-040: Service Manager Core

**Summary:** Implement the core ServiceManager that handles lifecycle operations for shared services.

**Acceptance Criteria:**
- [ ] ServiceManager interface defined
- [ ] Start/stop services from configuration
- [ ] Services created on tanuki-net Docker network
- [ ] Volume persistence for service data
- [ ] Basic health check support

**Implementation Notes:**
```go
type ServiceManager interface {
    StartServices() error
    StopServices() error
    StartService(name string) error
    StopService(name string) error
    GetStatus(name string) (*ServiceStatus, error)
    GetAllStatus() map[string]*ServiceStatus
    GetConnectionInfo(name string) (*ServiceConnection, error)
}
```

---

### TANK-042: Service Configuration Schema

**Summary:** Define and validate the YAML configuration schema for shared services.

**Acceptance Criteria:**
- [ ] Service config section in tanuki.yaml
- [ ] Support for image, ports, environment, volumes
- [ ] Validation of service configuration
- [ ] Default service templates (postgres, redis)

**Configuration Example:**
```yaml
services:
  postgres:
    enabled: true
    image: postgres:16
    port: 5432
    environment:
      POSTGRES_USER: tanuki
      POSTGRES_PASSWORD: tanuki
      POSTGRES_DB: tanuki_dev
    volumes:
      - tanuki-postgres:/var/lib/postgresql/data
    healthcheck:
      command: ["pg_isready", "-U", "tanuki"]
      interval: 5s
      timeout: 3s
      retries: 5
```

---

### TANK-043: Service Health Monitoring

**Summary:** Implement health monitoring with automatic recovery for shared services.

**Acceptance Criteria:**
- [ ] Periodic health check loop
- [ ] Service-specific health commands (pg_isready, redis-cli ping)
- [ ] Automatic restart on unhealthy status
- [ ] Health status exposed via ServiceManager
- [ ] Configurable check intervals and retry limits

---

### TANK-044: Service Commands

**Summary:** Add CLI commands for managing shared services.

**Commands:**
```bash
tanuki service start [name]      # Start all or specific service
tanuki service stop [name]       # Stop all or specific service
tanuki service status            # Show service status table
tanuki service logs <name>       # Stream service logs
tanuki service connect <name>    # Open interactive connection (psql, redis-cli)
```

**Acceptance Criteria:**
- [ ] All commands implemented
- [ ] Status shows health, uptime, port mappings
- [ ] Connect command opens appropriate client tool

---

### TANK-045: Agent Service Injection

**Summary:** Automatically inject service connection information into agent containers.

**Acceptance Criteria:**
- [ ] Connection info set as environment variables
- [ ] Variables follow naming convention (SERVICE_HOST, SERVICE_PORT, etc.)
- [ ] Services must be healthy before injection
- [ ] Warning if agent spawned with services not running

**Environment Variables:**
```bash
# Injected into agent containers
POSTGRES_HOST=tanuki-svc-postgres
POSTGRES_PORT=5432
POSTGRES_URL=tanuki-svc-postgres:5432
POSTGRES_USER=tanuki
POSTGRES_PASSWORD=tanuki

REDIS_HOST=tanuki-svc-redis
REDIS_PORT=6379
REDIS_URL=tanuki-svc-redis:6379
```

---

### TANK-041: Dashboard Framework

**Summary:** Set up BubbleTea TUI framework with basic model and layout.

**Acceptance Criteria:**
- [ ] BubbleTea program initialization
- [ ] Lipgloss styling setup
- [ ] Three-pane layout (agents, tasks, logs)
- [ ] Keyboard navigation between panes
- [ ] Responsive to terminal resize
- [ ] Basic quit/help handlers

**Dependencies:**
- `github.com/charmbracelet/bubbletea`
- `github.com/charmbracelet/lipgloss`
- `github.com/charmbracelet/bubbles` (for components)

---

### TANK-046: Dashboard Agent Pane

**Summary:** Implement the agent list pane with real-time status updates.

**Acceptance Criteria:**
- [ ] List all agents with status icons
- [ ] Show current task assignment
- [ ] Selection highlighting
- [ ] Actions: stop, start, attach, run
- [ ] Refresh on interval

**Display:**
```
Agents [3]
─────────────────────────────
> ● backend-agent   [working]  → TASK-002
  ○ frontend-agent  [idle]
  ○ qa-agent        [stopped]
```

---

### TANK-047: Dashboard Task Pane

**Summary:** Implement the task list pane with status and filtering.

**Acceptance Criteria:**
- [ ] List all project tasks
- [ ] Status icons (pending, in_progress, complete, failed)
- [ ] Show assigned agent
- [ ] Filter by status/role
- [ ] Actions: assign, view details

**Display:**
```
Tasks [4]
───────────────────────────────────
  ✓ TASK-001  User Auth        backend
> ◐ TASK-002  API Refactor     backend
  ○ TASK-003  Dashboard UI     frontend
  ○ TASK-004  Integration Tests qa
```

---

### TANK-048: Dashboard Log Pane

**Summary:** Implement real-time log streaming in the dashboard.

**Acceptance Criteria:**
- [ ] Stream logs from selected agent
- [ ] Follow mode (auto-scroll)
- [ ] Timestamp formatting
- [ ] Log buffer with size limit
- [ ] Toggle between agents

**Display:**
```
Logs: backend-agent                     [f]ollow
─────────────────────────────────────────────────
[10:15:32] Reading file src/api/routes.ts
[10:15:33] Analyzing current API structure...
[10:15:35] Found 12 endpoints to refactor
[10:15:40] Starting with /api/users endpoint
```

---

## Parallelization Guide

### True Parallel Execution (3 tabs/agents)

All workstreams start immediately with no waiting:

```text
Tab A (Service Engine): TANK-040 → TANK-042 → TANK-043
Tab B (Service CLI):    TANK-044, TANK-045
Tab C (TUI Dashboard):  TANK-041 → TANK-046 → TANK-047 → TANK-048
```

**Integration Phase:** After all tabs complete, one agent integrates:

- Wire ServiceManager into CLI commands
- Connect service injection to agent spawning
- Run end-to-end tests

### Sequential Path (1 agent)

```text
TANK-040 → TANK-042 → TANK-043 → TANK-044 → TANK-045
    → TANK-041 → TANK-046 → TANK-047 → TANK-048
```

Minimum viable:

- Services: Stop after TANK-044 for basic `service start/stop/status`
- Dashboard: Stop after TANK-046 for basic agent monitoring

---

## Key Interfaces

### Service Manager

```go
type ServiceManager interface {
    // Lifecycle
    StartServices() error
    StopServices() error
    StartService(name string) error
    StopService(name string) error

    // Status
    GetStatus(name string) (*ServiceStatus, error)
    GetAllStatus() map[string]*ServiceStatus
    IsHealthy(name string) bool

    // Connection
    GetConnectionInfo(name string) (*ServiceConnection, error)
    GetAllConnections() map[string]*ServiceConnection
}

type ServiceStatus struct {
    Name        string
    Running     bool
    Healthy     bool
    ContainerID string
    StartedAt   time.Time
    Port        int
    Error       string
}

type ServiceConnection struct {
    Host     string
    Port     int
    URL      string
    Username string
    Password string
}
```

### Dashboard Model

```go
type DashboardModel struct {
    // Data
    agents  []*agent.Agent
    tasks   []*task.Task
    logs    []LogLine

    // UI State
    activePane   Pane
    agentCursor  int
    taskCursor   int
    logFollow    bool

    // Dimensions
    width  int
    height int

    // Dependencies
    agentManager   agent.Manager
    taskManager    task.Manager
    config         *config.Config
}

type Pane int

const (
    PaneAgents Pane = iota
    PaneTasks
    PaneLogs
)

type LogLine struct {
    Timestamp time.Time
    Agent     string
    Content   string
}
```

---

## Task Status Summary

| ID       | Task                         | Status | WS | Description                  |
|----------|------------------------------|--------|----|------------------------------|
| TANK-040 | Service Manager Core         | todo   | A  | Service lifecycle management |
| TANK-042 | Service Configuration Schema | todo   | A  | YAML schema in tanuki.yaml   |
| TANK-043 | Service Health Monitoring    | todo   | A  | Health check loop, restart   |
| TANK-044 | Service Commands             | todo   | B  | service start/stop/status    |
| TANK-045 | Agent Service Injection      | todo   | B  | Inject connection env vars   |
| TANK-041 | Dashboard Framework          | todo   | C  | BubbleTea setup, layout      |
| TANK-046 | Dashboard Agent Pane         | todo   | C  | Agent list with status       |
| TANK-047 | Dashboard Task Pane          | todo   | C  | Task list with filtering     |
| TANK-048 | Dashboard Log Pane           | todo   | C  | Real-time log streaming      |

Total: 9 tasks

By Workstream:

- A (Service Engine): 3 tasks (TANK-040, TANK-042, TANK-043)
- B (Service CLI): 2 tasks (TANK-044, TANK-045)
- C (TUI Dashboard): 4 tasks (TANK-041, TANK-046, TANK-047, TANK-048)

Estimates:

- Small (S): 1 task
- Medium (M): 5 tasks
- Large (L): 3 tasks

---

## Success Metrics

Phase 4 is successful when:

1. Users can configure shared services in tanuki.yaml
2. Services start automatically with `tanuki project start`
3. Agent containers can connect to services via injected environment variables
4. `tanuki service status` shows health and connection info
5. `tanuki dashboard` opens an interactive TUI
6. Dashboard shows real-time agent and task status
7. Users can perform common actions (stop, attach, merge) from the dashboard
8. Log streaming works in the dashboard with follow mode

## Example Session

```bash
$ tanuki init
Initialized .tanuki/

$ cat tanuki.yaml
...
services:
  postgres:
    enabled: true
    image: postgres:16
    port: 5432
    environment:
      POSTGRES_USER: tanuki
      POSTGRES_PASSWORD: tanuki
...

$ tanuki service start
Starting services...
  ✓ postgres (tanuki-svc-postgres:5432)
  ✓ redis (tanuki-svc-redis:6379)

$ tanuki service status
SERVICE     STATUS    HEALTH    PORT    UPTIME
postgres    running   healthy   5432    2m
redis       running   healthy   6379    2m

$ tanuki spawn api --role backend
Created agent: api
Branch: tanuki/api
Container: tanuki-agent-api

# Agent environment includes:
# POSTGRES_HOST=tanuki-svc-postgres
# POSTGRES_PORT=5432
# REDIS_HOST=tanuki-svc-redis
# ...

$ tanuki service connect postgres
psql (16.0)
tanuki_dev=> \dt
...

$ tanuki dashboard
# Opens interactive TUI
```

---

## Out of Scope

- Service scaling (multiple replicas)
- Service dependencies/ordering
- Custom health check commands per service
- Mouse support in TUI
- Custom TUI themes
- Detached dashboard mode
- Service metrics/monitoring beyond health checks
