# Phase 2: Roles and System Prompts

Phase 2 extends Tanuki with a role system that allows spawning agents with pre-configured behavior patterns, specialized system prompts, and context awareness. This enables creating domain-specific agents (backend, frontend, QA, etc.) without repeating configuration.

## Goals

1. **Role Configuration System** - Define roles with YAML that specify prompts, tools, and context
2. **Role-Aware Spawning** - Spawn agents with roles that configure their behavior
3. **Built-in Role Library** - Ship with useful default roles (backend, frontend, qa, docs)
4. **Context Injection** - Automatically provide roles with relevant project documentation
5. **Tool Restrictions** - Limit agent capabilities based on role (e.g., QA can't write code)

## Architecture Overview

```
┌────────────────────────────────────────────────────────────┐
│                    Tanuki Phase 2                          │
├────────────────────────────────────────────────────────────┤
│                                                            │
│  ┌─────────────┐   ┌─────────────┐   ┌─────────────┐     │
│  │ Role        │   │ System      │   │ Context     │     │
│  │ Manager     │──▶│ Prompt      │   │ Files       │     │
│  │             │   │ Generator   │   │ Manager     │     │
│  └─────────────┘   └─────────────┘   └─────────────┘     │
│         │                  │                  │           │
│         └──────────────────┴──────────────────┘           │
│                            │                              │
│                            ▼                              │
│                   ┌─────────────┐                         │
│                   │ Agent       │                         │
│                   │ Manager     │                         │
│                   │ (Enhanced)  │                         │
│                   └─────────────┘                         │
│                            │                              │
└────────────────────────────┴───────────────────────────────┘
                             │
                             ▼
            ┌─────────────────────────────────┐
            │  Agent Spawned with Role        │
            │  • CLAUDE.md (role prompt)      │
            │  • Context files copied         │
            │  • Tool restrictions applied    │
            └─────────────────────────────────┘
```

## Workstreams

Phase 2 is organized into **3 concurrent workstreams** that can be executed in parallel after the foundation is complete.

```
Phase 2: Roles and System Prompts
├── Workstream A: Role Foundation (sequential)
│   └── TANK-020 → TANK-023 → TANK-024
│
├── Workstream B: Role Integration (parallel after A)
│   ├── TANK-021 (Spawn with Role)
│   ├── TANK-025 (Context Files)
│   └── TANK-026 (Built-in Roles)
│
└── Workstream C: Role Commands (parallel after A)
    ├── TANK-022 (Role Commands)
    └── TANK-027 (Role Templates)
```

---

## Task Breakdown

### Workstream A: Role Foundation (sequential)

These tasks build the core role system infrastructure and must be completed first.

| ID | Task | Priority | Estimate | Depends On | Description |
|----|------|----------|----------|------------|-------------|
| TANK-020 | Role Configuration Schema | high | M | TANK-002 | Define YAML schema for role definitions |
| TANK-023 | Role Manager Implementation | high | M | TANK-020 | Implement core RoleManager with load/validate |
| TANK-024 | Role-Based Tool Filtering | high | M | TANK-023 | Apply allowed/disallowed tools per role |

### Workstream B: Role Integration (parallel after A)

These tasks integrate roles into the agent lifecycle. Can be worked on concurrently once Workstream A is complete.

| ID | Task | Priority | Estimate | Depends On | Description |
|----|------|----------|----------|------------|-------------|
| TANK-021 | Spawn with Role Assignment | high | M | TANK-009, TANK-023 | Add `--role` flag to spawn command |
| TANK-025 | Context File Management | high | M | TANK-021 | Copy context files to agent worktree |
| TANK-026 | Built-in Role Library | high | L | TANK-023 | Create default roles (backend, frontend, qa, docs) |

### Workstream C: Role Commands (parallel after A)

These tasks add CLI commands for managing roles. Can be worked on in parallel with Workstream B.

| ID | Task | Priority | Estimate | Depends On | Description |
|----|------|----------|----------|------------|-------------|
| TANK-022 | Role Management Commands | medium | S | TANK-023 | Implement role list/show/init commands |
| TANK-027 | Role Template Generation | medium | S | TANK-023 | Generate custom role templates |

---

## Parallelization Guide

### Maximum Parallelism (3 agents)

**Sprint 1: Role Foundation**
```
Agent 1: TANK-020 → TANK-023 → TANK-024
Agent 2: (wait for TANK-020) → TANK-026 (Built-in Roles)
Agent 3: (wait for TANK-023) → TANK-022 (Role Commands)
```

**Sprint 2: Integration & Polish**
```
Agent 1: TANK-021 (Spawn with Role)
Agent 2: TANK-025 (Context Files)
Agent 3: TANK-027 (Role Templates)
```

### Sequential Path (1 agent)

```
TANK-020 → TANK-023 → TANK-024 → TANK-021 → TANK-025 → TANK-026 → TANK-022 → TANK-027
```

Minimum viable: Stop after TANK-021 to have basic role support.

---

## Task Status Summary

| ID | Task | Status | Workstream |
|----|------|--------|------------|
| TANK-020 | Role Configuration Schema | todo | A |
| TANK-023 | Role Manager Implementation | todo | A |
| TANK-024 | Role-Based Tool Filtering | todo | A |
| TANK-021 | Spawn with Role Assignment | todo | B |
| TANK-025 | Context File Management | todo | B |
| TANK-026 | Built-in Role Library | todo | B |
| TANK-022 | Role Management Commands | todo | C |
| TANK-027 | Role Template Generation | todo | C |

**Total: 8 tasks** (3 existing + 5 new)

**Estimates:**
- Small (S): 2 tasks
- Medium (M): 5 tasks
- Large (L): 1 task

---

## Success Metrics

Phase 2 is successful when:

1. Users can spawn agents with roles: `tanuki spawn api --role backend`
2. Roles configure agent behavior via CLAUDE.md
3. Tool restrictions are enforced (QA can't write code)
4. Context files are automatically provided to agents
5. Built-in roles cover common use cases
6. Users can create custom roles easily
7. Role commands provide visibility (list, show)
