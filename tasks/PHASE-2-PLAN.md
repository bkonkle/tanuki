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

Phase 2 is organized into **3 fully independent workstreams** that can be executed in parallel
from the start. Each workstream owns a complete vertical slice with no cross-dependencies.

```
Phase 2: Roles and System Prompts
├── Workstream A: Role Engine (internal/role/ core)
│   └── TANK-020 (Schema) + TANK-023 (Manager) + TANK-024 (Tool Filtering)
│
├── Workstream B: Built-in Roles (static YAML + embed)
│   └── TANK-026 (Built-in Role Library)
│
└── Workstream C: CLI + Spawn Integration (interface-based)
    └── TANK-021 (Spawn) + TANK-022 (Commands) + TANK-025 (Context) + TANK-027 (Templates)
```

---

## Task Breakdown

### Workstream A: Role Engine

Complete internal engine for roles — schema, manager, and tool filtering. No CLI or integration code.

| ID | Task | Priority | Estimate | Workstream | Description |
|----|------|----------|----------|------------|-------------|
| TANK-020 | Role Configuration Schema | high | M | A | Define RoleConfig struct with YAML/validator tags |
| TANK-023 | Role Manager Implementation | high | M | A | Implement RoleManager with load/validate/inheritance |
| TANK-024 | Role-Based Tool Filtering | high | M | A | FilterTools function for role-based restrictions |

**Scope:** `internal/role/schema.go`, `internal/role/manager.go`, `internal/role/tools.go` + tests

### Workstream B: Built-in Roles

Static YAML role definitions with go:embed. No code dependencies on other workstreams.

| ID       | Task                  | Priority | Est | WS | Description                        |
|----------|-----------------------|----------|-----|----|------------------------------------|
| TANK-026 | Built-in Role Library | high     | L   | B  | Embedded YAML roles (backend, etc) |

**Scope:** `internal/role/builtin/*.yaml`, `internal/role/builtin/embed.go`

### Workstream C: CLI + Spawn Integration

CLI commands and spawn integration, coded against interfaces (not concrete implementations).

| ID       | Task                       | Priority | Est | WS | Description                       |
|----------|----------------------------|----------|-----|----|------------------------------------|
| TANK-021 | Spawn with Role Assignment | high     | M   | C  | Add --role flag, CLAUDE.md gen    |
| TANK-022 | Role Management Commands   | medium   | S   | C  | role list/show/init commands      |
| TANK-025 | Context File Management    | high     | M   | C  | CopyContextFiles for role context |
| TANK-027 | Role Template Generation   | medium   | S   | C  | Generate custom role YAML         |

**Scope:** `internal/cli/role.go`, `internal/cli/spawn.go`, `internal/role/context.go`,
`internal/role/template.go`, `internal/state/state.go`

---

## Parallelization Guide

### True Parallel Execution (3 tabs/agents)

All workstreams start immediately with no waiting:

```
Tab A (Role Engine):     TANK-020 → TANK-023 → TANK-024
Tab B (Built-in Roles):  TANK-026
Tab C (CLI + Spawn):     TANK-021, TANK-022, TANK-025, TANK-027
```

**Integration Phase:** After all tabs complete, one agent integrates:

- Wire RoleManager into CLI commands
- Connect built-in roles to manager's LoadBuiltin()
- Run end-to-end tests

### Sequential Path (1 agent)

```
TANK-020 → TANK-023 → TANK-024 → TANK-026 → TANK-021 → TANK-025 → TANK-022 → TANK-027
```

Minimum viable: Stop after TANK-024 + TANK-021 for basic role support.

---

## Task Status Summary

| ID       | Task                        | Status | WS | Description                 |
|----------|-----------------------------| -------|----|-----------------------------|
| TANK-020 | Role Configuration Schema   | done   | A  | RoleConfig struct with tags |
| TANK-023 | Role Manager Implementation | done   | A  | Load/validate/inheritance   |
| TANK-024 | Role-Based Tool Filtering   | done   | A  | FilterTools function        |
| TANK-026 | Built-in Role Library       | done   | B  | Embedded YAML roles         |
| TANK-021 | Spawn with Role Assignment  | done   | C  | --role flag, CLAUDE.md gen  |
| TANK-022 | Role Management Commands    | done   | C  | role list/show/init/create  |
| TANK-025 | Context File Management     | done   | C  | CopyContextFiles            |
| TANK-027 | Role Template Generation    | done   | C  | Custom role YAML templates  |

Total: 8 tasks

By Workstream:

- A (Role Engine): 3 tasks (TANK-020, TANK-023, TANK-024)
- B (Built-in Roles): 1 task (TANK-026)
- C (CLI + Spawn): 4 tasks (TANK-021, TANK-022, TANK-025, TANK-027)

Estimates:

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
