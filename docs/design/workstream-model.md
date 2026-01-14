# Workstream Model Design

## Overview

Workstreams group related tasks that should be executed sequentially on the same agent/worktree. They enable:

1. **Sequential task execution** within a workstream (preserving context)
2. **Parallel workstream execution** per role (up to role's concurrency limit)
3. **Rolling window scheduling** - new workstreams start as others complete

## Workstream Definition

### Task Schema Extension

```yaml
---
id: user-auth-001
title: Implement login endpoint
role: backend
workstream: api    # Named workstream (e.g., "api", "auth", "database", "ui")
priority: high
status: pending
depends_on: []
---
```

### Workstream Semantics

- **Identifier**: String, defaults to task ID if not specified
- **Scope**: Per-role (workstreams don't cross roles)
- **Ordering**: Tasks within a workstream are ordered by:
  1. Dependencies (`depends_on`)
  2. Priority
  3. Task ID (alphabetical for stability)

### Default Assignment

When `workstream` is not specified in task frontmatter:

1. If task has dependencies: inherit workstream from first dependency
2. Otherwise: use task ID as a single-task workstream

## Role Configuration Extension

### tanuki.yaml Structure

```yaml
roles:
  backend:
    concurrency: 2              # Max concurrent workstreams for this role
    system_prompt: |            # Role-specific prompt (optional)
      You are a backend developer...
    system_prompt_file: ""      # Alternative: path to prompt file
```

### Precedence

Role configuration merges from multiple sources (highest to lowest):

1. **tanuki.yaml `roles` section** - project-level config
2. **.tanuki/roles/<name>.yaml** - role definition files
3. **Builtin roles** - embedded defaults

For each field, the first non-empty value wins.

## Workstream Scheduler

### State Tracking

```go
type WorkstreamState struct {
    Role        string
    Workstream  string
    AgentName   string      // Assigned agent (owns worktree)
    Status      string      // "active", "completed", "failed"
    CurrentTask string      // Currently executing task ID
    Tasks       []string    // All task IDs in order
    StartedAt   time.Time
}
```

### Scheduling Algorithm

```
For each role:
  1. Get role concurrency limit (default: 1)
  2. Get active workstreams for role
  3. If active < limit:
     - Find next pending workstream (first pending task by priority)
     - Assign to idle agent (or spawn new if needed)
     - Mark workstream as active
  4. When workstream completes all tasks:
     - Release agent back to pool
     - Start next pending workstream
```

### Rolling Window Behavior

- Each role has `concurrency` slots for active workstreams
- When a workstream completes, its slot opens for the next
- Agents are reused across workstreams (within same role)

## Workstream Completion

A workstream is complete when:

1. All tasks in the workstream have status `complete`
2. No tasks have status `failed` or `blocked`

If any task fails, the workstream is marked `failed` and releases its slot.

## Context Budget

### Problem

Long-running workstreams may exhaust Claude's context window.

### Solution

Track turns/iterations per workstream session:

```yaml
defaults:
  max_workstream_turns: 200    # Max turns before context reset
```

When budget is hit:

1. Emit current artifacts (commit, PR summary, etc.)
2. Save workstream state
3. Start fresh Ralph instance on same worktree
4. Resume from next pending task in workstream

## Project Document

### tasks/{project-name}/README.md

Created by `tanuki project init <name>`:

```markdown
# Project: <name>

Brief project description.

## Architecture

Key components and their relationships.

## Conventions

- Code style guidelines
- Testing requirements
- Documentation standards

## Context Files

Files agents should understand:
- README.md
- CLAUDE.md (if exists)
- architecture docs
```

The tasks directory also contains a top-level README.md explaining the multi-project structure
and task format conventions.

### Loading into Agent Context

The project document is:

1. Copied to `.tanuki/context/` in agent worktree
2. Referenced in role system prompts
3. Loaded as initial context for new agent sessions

## Migration

### Existing Tasks Without Workstream

Tasks without explicit `workstream` field:

1. If `depends_on` is set: inherit from first dependency
2. Otherwise: use task ID (single-task workstream)

This preserves existing behavior while enabling workstream features.

### Removal of Async Mode

The `--ralph` flag and async execution are removed:

- `tanuki run` always uses Ralph loop
- Default completion signal: "DONE" (configurable)
- Default max iterations: 30 (configurable)

Tasks without completion criteria get default Ralph behavior.

## Implementation Phases

### Phase 1: Schema & Config
- Add `workstream` to Task struct
- Add `roles` section to Config
- Add `concurrency` to Role struct

### Phase 2: Role Merging
- Implement config → role merging
- Update role manager to use merged config
- Update CLI role commands

### Phase 3: Workstream Scheduler
- Implement WorkstreamState tracking
- Add workstream-aware queue
- Implement rolling window scheduler

### Phase 4: Ralph-Only Mode
- Remove `--ralph` flag (always on)
- Add default completion behavior
- Update task runner

### Phase 5: Context Budget
- Add turn/iteration tracking
- Implement session handoff
- Preserve workstream state across sessions

### Phase 6: Project Doc & CLI
- Add project.md to init
- Load into agent context
- Update status/dashboard for workstreams
