# Tanuki

A CLI for orchestrating multiple Claude Code agents in isolated Docker containers. Combines:

- **Conductor's** parallel agent management (without Mac-only GUI)
- **Ralph Loop's** autonomous convergence (with container isolation)
- **Docker's** reproducibility (without MCP complexity)

```bash
# Quick start
tanuki spawn auth api tests     # Create 3 isolated agents
tanuki run auth "Add OAuth"     # Fire-and-forget
tanuki run api "Add endpoints" --follow  # Stream output
tanuki run tests "Add coverage to 80%" --ralph  # Loop until done
tanuki list                     # Check status
tanuki merge auth               # Merge completed work
```

## Why Tanuki?

| Tool | Approach | Limitation |
|------|----------|------------|
| **Conductor** | GUI + worktrees | Mac-only, no container isolation, no automation |
| **Claude Squad** | tmux + worktrees | No isolation, manual monitoring |
| **Ralph Loop** | Single agent in bash loop | No parallelism, no isolation |
| **container-use** | Docker + MCP | Requires MCP (security concerns) |
| **Tanuki** | Docker + worktrees + Ralph | All the good parts, none of the bad |

## Design Principles

1. **Container Isolation** - Each agent runs in its own Docker container
2. **Ralph Mode Built-in** - Native support for autonomous loop-until-done execution
3. **Parallel by Default** - Spawn multiple agents, run tasks concurrently
4. **No MCP** - Direct CLI invocation only, following Simon Willison's security guidance
5. **Unix Philosophy** - Composable, pipeable, scriptable
6. **Zero Config Start** - `tanuki spawn foo && tanuki run foo "do thing"` just works

---

## Architecture

### Isolation Strategy

```
Host Machine
├── project/                         (main checkout)
├── .tanuki/
│   ├── worktrees/
│   │   ├── agent-1/                 (git worktree)
│   │   ├── agent-2/                 (git worktree)
│   │   └── agent-3/                 (git worktree)
│   ├── state/
│   │   └── agents.json              (agent state tracking)
│   └── config/
│       └── tanuki.yaml              (project config)
│
Docker Network (tanuki-net)
├── tanuki-agent-1 (container)
│   └── /workspace (bind mount → .tanuki/worktrees/agent-1/)
├── tanuki-agent-2 (container)
│   └── /workspace (bind mount → .tanuki/worktrees/agent-2/)
└── tanuki-shared (optional: postgres, redis, etc.)
```

**Why Hybrid:**

1. **Git worktrees** handle branch isolation efficiently
2. **Docker containers** provide process/network/resource isolation
3. **Bind mounts** give real-time file access without copy overhead
4. **Shared network** allows future postgres/kafka integration
5. **Host worktrees** allow easy inspection with standard git tools

### Component Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                         tanuki CLI                               │
│                        (Go binary)                               │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐           │
│  │ Agent Manager │  │ Git Manager  │  │ Docker Manager│          │
│  │              │  │              │  │               │           │
│  │ - spawn      │  │ - worktree   │  │ - container   │           │
│  │ - stop       │  │ - branch     │  │ - network     │           │
│  │ - status     │  │ - merge      │  │ - volumes     │           │
│  │ - run        │  │ - diff       │  │ - exec        │           │
│  └──────────────┘  └──────────────┘  └──────────────┘           │
│                                                                  │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐           │
│  │ State Store  │  │ Config       │  │ Claude Exec  │           │
│  │              │  │              │  │              │           │
│  │ - agents.json│  │ - tanuki.yaml│  │ - headless   │           │
│  │ - sessions   │  │ - roles/     │  │ - streaming  │           │
│  └──────────────┘  └──────────────┘  └──────────────┘           │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Docker Network                              │
│                      (tanuki-net)                                │
│                                                                  │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐  │
│  │ tanuki-agent-1  │  │ tanuki-agent-2  │  │ tanuki-agent-3  │  │
│  │                 │  │                 │  │                 │  │
│  │ Claude Code     │  │ Claude Code     │  │ Claude Code     │  │
│  │ + dev tools     │  │ + dev tools     │  │ + dev tools     │  │
│  │                 │  │                 │  │                 │  │
│  │ /workspace ────►│  │ /workspace ────►│  │ /workspace ────►│  │
│  │ (bind mount)    │  │ (bind mount)    │  │ (bind mount)    │  │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘  │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

---

## CLI Reference

### Commands

```bash
# Core workflow (Phase 1)
tanuki init                          # Initialize .tanuki/ directory
tanuki spawn <name>                  # Create agent (worktree + container)
tanuki run <name> "<prompt>"         # Send task to agent
tanuki list                          # List all agents and status

# Execution modes
tanuki run foo "do thing"            # Fire-and-forget (default)
tanuki run foo "do thing" -f         # Follow output in real-time
tanuki run foo "do thing" --ralph    # Loop until done (Ralph mode)

# Agent management
tanuki logs <name>                   # Stream agent output
tanuki diff <name>                   # Show uncommitted changes
tanuki attach <name>                 # Shell into container
tanuki stop/start/remove <name>      # Lifecycle commands
tanuki merge <name>                  # Merge branch (or create PR)

# Bulk operations
tanuki spawn -n 3                    # Spawn agent-1, agent-2, agent-3
tanuki stop --all                    # Stop all agents

# Role commands (Phase 2)
tanuki spawn --role backend foo      # Spawn with role
tanuki role list                     # List available roles

# Project mode (Phase 3)
tanuki project start                 # Auto-assign tasks to agents
tanuki project status                # Dashboard view
```

### Execution Modes

**Default (fire-and-forget):**

```bash
tanuki run auth "Add OAuth login"
# Returns immediately, task runs in background
# Check with: tanuki logs auth -f
```

**Follow mode:**

```bash
tanuki run auth "Add OAuth login" --follow
# Streams output until complete, blocks terminal
```

**Ralph mode (autonomous loop):**

```bash
tanuki run auth "Add OAuth login. Say DONE when tests pass." --ralph
# Loops until DONE detected or max iterations reached
# Fresh context each iteration, progress persists in files
```

Ralph mode is ideal for:

- Refactoring with test coverage as completion signal
- Iterative improvements until linter passes
- Long-running tasks that benefit from "fresh eyes"

---

## Configuration

### tanuki.yaml

```yaml
version: "1"

# Docker image settings
image:
  name: "bkonkle/tanuki"
  tag: "latest"

# Default agent settings
defaults:
  allowed_tools: [Read, Write, Edit, Bash, Glob, Grep]
  max_turns: 50
  model: "claude-sonnet-4-5-20250514"

# Ralph mode settings
ralph:
  max_iterations: 30
  completion_signal: "DONE"
  verify_command: ""
  cooldown_seconds: 5

# Git settings
git:
  branch_prefix: "tanuki/"
  auto_commit: true

# Roles (Phase 2)
roles:
  backend:
    system_prompt_file: ".tanuki/roles/backend.md"
    allowed_tools: [Read, Write, Edit, Bash, Glob, Grep]
  frontend:
    system_prompt_file: ".tanuki/roles/frontend.md"
    allowed_tools: [Read, Write, Edit, Bash, Glob, Grep]
  qa:
    system_prompt_file: ".tanuki/roles/qa.md"
    allowed_tools: [Read, Bash, Glob, Grep]  # No Write/Edit
```

### agents.json

```json
{
  "agents": {
    "auth-feature": {
      "name": "auth-feature",
      "container_id": "abc123...",
      "branch": "tanuki/auth-feature",
      "worktree_path": ".tanuki/worktrees/auth-feature",
      "status": "working",
      "created_at": "2026-01-13T10:00:00Z",
      "last_task": {
        "prompt": "Implement OAuth2 login flow",
        "started_at": "2026-01-13T10:05:00Z"
      }
    }
  }
}
```

---

## Security Model

Following Simon Willison's guidance on avoiding MCP's "lethal trifecta":

1. **No MCP** - Direct CLI invocation only, no dynamic tool registration
2. **Container Isolation** - No host network access, resource limits
3. **Explicit Permissions** - `--allowedTools` set per invocation
4. **Git as Audit Trail** - All changes in isolated branches, human review required
5. **Secrets Management** - Claude auth via read-only volume mount

---

## Decisions Made

1. **Language**: Go for CLI (faster development, good Docker tooling)
2. **Async by default**: `tanuki run` is fire-and-forget with `--follow` flag for streaming
3. **Branch naming**: `tanuki/<name>` namespace
4. **Cleanup**: Manual removal, warn about stale agents
5. **Claude auth**: Volume mount from host (consistent with existing setup)

---

## Implementation Tasks

Tasks are organized into **workstreams** that can be executed concurrently by multiple agents.

```
Phase 1: Core CLI (MVP)
├── Workstream A: Foundation (sequential)
│   └── TANK-001 → TANK-002 → TANK-003
│
├── Workstream B: Infrastructure (parallel after TANK-001)
│   ├── TANK-004 (Git)
│   ├── TANK-005 (Docker)
│   └── TANK-018 (Dockerfile)
│
├── Workstream C: Orchestration (after A + B)
│   ├── TANK-006 (Agent Manager)
│   └── TANK-007 (Claude Executor)
│
└── Workstream D: Commands (after C)
    ├── TANK-008, TANK-009, TANK-010, TANK-011 (high priority)
    └── TANK-012 through TANK-017 (medium priority)
```

### Phase 1: Core CLI (MVP)

**Workstream A: Foundation** (sequential)

| ID | Task | Priority | Estimate | Depends On |
|----|------|----------|----------|------------|
| TANK-001 | CLI Skeleton and Project Structure | high | M | - |
| TANK-002 | Configuration Management | high | M | TANK-001 |
| TANK-003 | State Management | high | M | TANK-001 |

**Workstream B: Infrastructure** (parallel after TANK-001)

| ID | Task | Priority | Estimate | Depends On |
|----|------|----------|----------|------------|
| TANK-004 | Git Worktree Manager | high | M | TANK-001 |
| TANK-005 | Docker Container Manager | high | L | TANK-001 |
| TANK-018 | Dockerfile for Agent Container | high | M | - |

**Workstream C: Orchestration** (requires A + B)

| ID | Task | Priority | Estimate | Depends On |
|----|------|----------|----------|------------|
| TANK-006 | Agent Manager | high | M | TANK-003, TANK-004, TANK-005 |
| TANK-007 | Claude Code Executor | high | L | TANK-002, TANK-005 |

**Workstream D: Commands**

High Priority:

| ID | Task | Priority | Estimate | Depends On |
|----|------|----------|----------|------------|
| TANK-008 | Command: tanuki init | high | S | TANK-002, TANK-003 |
| TANK-009 | Command: tanuki spawn | high | M | TANK-006 |
| TANK-010 | Command: tanuki list | high | S | TANK-003 |
| TANK-011 | Command: tanuki run | high | M | TANK-006, TANK-007 |

Medium Priority:

| ID | Task | Priority | Estimate | Depends On |
|----|------|----------|----------|------------|
| TANK-012 | Command: tanuki logs | medium | S | TANK-005 |
| TANK-013 | Command: tanuki diff | medium | S | TANK-004 |
| TANK-014 | Command: tanuki attach | medium | S | TANK-005 |
| TANK-015 | Commands: stop/start/remove | medium | M | TANK-006 |
| TANK-016 | Command: tanuki merge | medium | M | TANK-004 |
| TANK-017 | Command: tanuki status | medium | S | TANK-003 |

### Phase 2: Roles and System Prompts

See [PHASE-2-PLAN.md](PHASE-2-PLAN.md) for detailed breakdown with concurrent workstreams.

| ID | Task | Priority | Estimate | Depends On | Workstream |
|----|------|----------|----------|------------| ----------|
| TANK-020 | Role Configuration Schema | high | M | TANK-002 | A |
| TANK-023 | Role Manager Implementation | high | M | TANK-020 | A |
| TANK-024 | Role-Based Tool Filtering | high | M | TANK-023 | A |
| TANK-021 | Spawn with Role Assignment | high | M | TANK-009, TANK-023 | B |
| TANK-025 | Context File Management | high | M | TANK-021 | B |
| TANK-026 | Built-in Role Library | high | L | TANK-023 | B |
| TANK-022 | Role Management Commands | medium | S | TANK-023 | C |
| TANK-027 | Role Template Generation | medium | S | TANK-023 | C |

### Phase 3: Task Queue and Project Mode

See [PHASE-3-PLAN.md](PHASE-3-PLAN.md) for detailed breakdown with concurrent workstreams.

**Workstream A: Task Infrastructure** (sequential)

| ID | Task | Priority | Estimate | Depends On | Workstream |
|----|------|----------|----------|------------|------------|
| TANK-030 | Task File Schema | high | M | TANK-020 | A |
| TANK-033 | Task Manager Implementation | high | M | TANK-030 | A |
| TANK-035 | Dependency Resolver | high | M | TANK-033 | A |

**Workstream B: Queue and Distribution** (parallel after TANK-030)

| ID | Task | Priority | Estimate | Depends On | Workstream |
|----|------|----------|----------|------------|------------|
| TANK-034 | Task Queue | high | M | TANK-030 | B |
| TANK-036 | Workload Balancing | medium | M | TANK-030, TANK-006 | B |
| TANK-037 | Status Tracking | medium | S | TANK-030 | B |

**Workstream C: Orchestration and Commands** (after A + B)

| ID | Task | Priority | Estimate | Depends On | Workstream |
|----|------|----------|----------|------------|------------|
| TANK-031 | Project Commands | high | L | TANK-033, TANK-034 | C |
| TANK-032 | Task Completion and Validation | high | L | TANK-031 | C |
| TANK-038 | Project Orchestrator | high | L | TANK-031, TANK-032, TANK-036 | C |

### Phase 4: Shared Services and Advanced Features

| ID | Task | Priority | Estimate | Depends On |
|----|------|----------|----------|------------|
| TANK-040 | Shared Services (Postgres, Redis) | medium | L | TANK-005 |
| TANK-041 | TUI Dashboard | low | XL | TANK-010, TANK-031 |

---

## Parallelization Guide

### Maximum Parallelism (3 agents)

**Sprint 1: Foundation + Infrastructure**

```
Agent 1: TANK-001 → TANK-002 → TANK-003
Agent 2: TANK-018 → (wait for TANK-001) → TANK-004
Agent 3: (wait for TANK-001) → TANK-005
```

**Sprint 2: Orchestration + Commands**

```
Agent 1: TANK-006 → TANK-009
Agent 2: TANK-007 → TANK-011
Agent 3: TANK-008 → TANK-010 → TANK-017
```

**Sprint 3: Supporting Commands**

```
Agent 1: TANK-012 → TANK-014
Agent 2: TANK-013 → TANK-016
Agent 3: TANK-015
```

### Minimum Viable Path (1 agent)

```
TANK-001 → TANK-002 → TANK-003 → TANK-018 → TANK-004 → TANK-005
    → TANK-006 → TANK-007 → TANK-008 → TANK-009 → TANK-010 → TANK-011
```

This gives you `init`, `spawn`, `list`, and `run` - the core workflow.

---

## Task Format

Each task file in `phase-*/` uses YAML front matter:

```yaml
---
id: TANK-001
title: Short title
status: todo | in-progress | done
priority: high | medium | low
estimate: S | M | L | XL
depends_on: [TANK-000]
workstream: A | B | C | D
---
```

**Estimates:**

- **S** (Small): < 2 hours
- **M** (Medium): 2-4 hours
- **L** (Large): 4-8 hours
- **XL** (Extra Large): 1-2 days

---

## Status

- **Phase 1**: In progress (18 tasks)
- **Phase 2**: Not started (8 tasks)
- **Phase 3**: Not started (9 tasks)
- **Phase 4**: Not started (2 tasks)

**Total**: 37 tasks
