# TODO: Ralph-First Projects, Roles, Workstreams

## Status: COMPLETED

All items from this plan have been implemented. See below for details.

## Goal

Make the README workflow real: Projects -> Roles -> Workstreams -> Tasks, with Ralph-only execution,
project docs, role-scoped prompts, and per-role workstream concurrency.

## Completed Items

- [x] Define the workstream model and defaults: how to assign `workstream` when missing, how to
      order workstreams per role, and how to detect a workstream as complete.
      - Created `docs/design/workstream-model.md`

- [x] Extend task schema to include `workstream` and update parsing/validation. Update managers and
      tests to support `role + workstream` filtering and ordering.
      - Added `Workstream` field to `internal/task/types.go`
      - Added `GetWorkstream()`, `GetByWorkstream()`, `GetWorkstreams()` to manager
      - Added tests in `internal/task/manager_test.go`

- [x] Add role config to `tanuki.yaml` with `system_prompt` and `concurrency`, wire into config
      loader/validator and defaults.
      - Added `RoleConfig` struct to `internal/config/config.go`
      - Added `GetRoleConcurrency()`, `GetRoleConfig()` helpers

- [x] Merge role config with existing role files and builtin roles (define precedence), and ensure
      CLI role listing/show includes config-defined roles.
      - Added `NewManagerWithConfig()` to `internal/role/manager.go`
      - Implements precedence: config > project roles > builtin

- [x] Update project init to create `.tanuki/project.md` and sample ticket with `workstream`.
      - Updated `internal/cli/project_init.go`

- [x] Load project doc into agent context (either auto-context files or prompt injection).
      - Added `CopyProjectDoc()`, `CopyAllContext()` to `internal/context/context.go`

- [x] Implement workstream scheduler with per-role concurrency and rolling window behavior.
      - Created `internal/project/workstream.go` with `WorkstreamScheduler`
      - Integrated into orchestrator

- [x] Make Ralph the only execution mode:
      - Updated `internal/cli/run.go` - always uses Ralph loop
      - Removed `--ralph` and `--follow` flags

- [x] Add context-budget logic for workstreams (max turns/iterations).
      - Added `WorkstreamSession` to `internal/state/state.go`
      - Added `MaxWorkstreamTurns` config option

- [x] Update project CLI and status/dashboard to surface workstreams:
      - Added `--workstreams` flag to `project status`
      - Shows workstream column in task table

- [x] Add or update tests for new config/roles, task schema, scheduler behavior.
      - Added workstream tests to `internal/task/manager_test.go`

- [x] Document migration (this file and design doc)

## Key Changes

### Task Schema

```yaml
---
id: AUTH-001
title: Implement login
role: backend
workstream: auth-feature  # NEW - groups related tasks
priority: high
status: pending
---
```

### tanuki.yaml

```yaml
roles:
  backend:
    concurrency: 2          # Max concurrent workstreams
    system_prompt: "..."    # Role-specific prompt
```

### CLI

- `tanuki run` now always uses Ralph mode (autonomous loop)
- `tanuki project status --workstreams` shows workstream details
- `tanuki project init` creates `project.md`

### Precedence

Role configuration merges from:

1. `tanuki.yaml` `roles` section (highest)
2. `.tanuki/roles/*.yaml` files
3. Builtin roles (lowest)
