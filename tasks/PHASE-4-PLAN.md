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

Phase 4 is organized into **3 concurrent workstreams** that can be executed in parallel.

```
Phase 4: Shared Services and Advanced Features
├── Workstream A: Shared Services (sequential)
│   └── TANK-040 → TANK-042 → TANK-043
│
├── Workstream B: Service Integration (parallel after TANK-040)
│   ├── TANK-044 (Service Commands)
│   └── TANK-045 (Agent Service Injection)
│
└── Workstream C: TUI Dashboard (independent)
    ├── TANK-041 → TANK-046 → TANK-047 → TANK-048
    └── (can start in parallel with Workstream A)
```

---

## Task Breakdown

### Workstream A: Shared Services (sequential)

These tasks build the core service infrastructure.

| ID | Task | Priority | Estimate | Depends On | Description |
|----|------|----------|----------|------------|-------------|
| TANK-040 | Service Manager Core | medium | L | TANK-005 | Service lifecycle: start, stop, health checks |
| TANK-042 | Service Configuration Schema | medium | M | TANK-040 | YAML schema for service definitions in tanuki.yaml |
| TANK-043 | Service Health Monitoring | medium | M | TANK-040 | Health check loop, restart on failure, status reporting |

### Workstream B: Service Integration (parallel after TANK-040)

These tasks integrate services with the agent system.

| ID | Task | Priority | Estimate | Depends On | Description |
|----|------|----------|----------|------------|-------------|
| TANK-044 | Service Commands | medium | S | TANK-040 | CLI: service start/stop/status/connect |
| TANK-045 | Agent Service Injection | medium | M | TANK-040, TANK-006 | Inject connection info into agent containers |

### Workstream C: TUI Dashboard (independent)

These tasks build the interactive dashboard. Can run in parallel with Workstream A.

| ID | Task | Priority | Estimate | Depends On | Description |
|----|------|----------|----------|------------|-------------|
| TANK-041 | Dashboard Framework | low | L | TANK-010 | BubbleTea setup, model, basic layout |
| TANK-046 | Dashboard Agent Pane | low | M | TANK-041 | Agent list view with status, selection |
| TANK-047 | Dashboard Task Pane | low | M | TANK-041, TANK-031 | Task list view with status, filtering |
| TANK-048 | Dashboard Log Pane | low | L | TANK-041, TANK-012 | Real-time log streaming, follow mode |

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

### Maximum Parallelism (3 agents)

**Sprint 1: Foundation**
```
Agent 1: TANK-040 → TANK-042 → TANK-043
Agent 2: TANK-041 → TANK-046
Agent 3: (wait for TANK-041) → TANK-047
```

**Sprint 2: Integration & Polish**
```
Agent 1: TANK-044 → TANK-045
Agent 2: TANK-048
Agent 3: (integration testing / polish)
```

### Moderate Parallelism (2 agents)

**Sprint 1: Core Features**
```
Agent 1: TANK-040 → TANK-042 → TANK-043 → TANK-044 → TANK-045
Agent 2: TANK-041 → TANK-046 → TANK-047 → TANK-048
```

### Sequential Path (1 agent)

```
TANK-040 → TANK-042 → TANK-043 → TANK-044 → TANK-045
    → TANK-041 → TANK-046 → TANK-047 → TANK-048
```

**Minimum viable:**
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

| ID | Task | Status | Workstream | Priority |
|----|------|--------|------------|----------|
| TANK-040 | Service Manager Core | todo | A | medium |
| TANK-042 | Service Configuration Schema | todo | A | medium |
| TANK-043 | Service Health Monitoring | todo | A | medium |
| TANK-044 | Service Commands | todo | B | medium |
| TANK-045 | Agent Service Injection | todo | B | medium |
| TANK-041 | Dashboard Framework | todo | C | low |
| TANK-046 | Dashboard Agent Pane | todo | C | low |
| TANK-047 | Dashboard Task Pane | todo | C | low |
| TANK-048 | Dashboard Log Pane | todo | C | low |

**Total: 9 tasks** (2 existing expanded + 7 new)

**Estimates:**
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
