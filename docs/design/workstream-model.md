# Workstream Model Design

## Overview

Workstreams are the primary organizational unit for tasks in Tanuki. They group related
tasks that should be executed sequentially on the same agent/worktree. They enable:

1. **Sequential task execution** within a workstream (preserving context)
2. **Parallel workstream execution** across different workstreams (up to concurrency limit)
3. **Rolling window scheduling** - new workstreams start as others complete

**Note:** The workstream concept is flexible. While discipline-based organization
(e.g., "api", "frontend", "database") is one approach, workstreams can represent any
logical grouping: features, components, phases, or any other way you want to organize work.

## Workstream Definition

### Task Schema

```yaml
---
id: user-auth-001
title: Implement login endpoint
workstream: api    # Named workstream (e.g., "api", "auth", "database", "ui", "feature-x")
priority: high
status: pending
depends_on: []
---
```

### Workstream Semantics

- **Identifier**: String, defaults to task ID if not specified
- **Ordering**: Tasks within a workstream are ordered by:
  1. Dependencies (`depends_on`)
  2. Priority
  3. Task ID (alphabetical for stability)

### Default Assignment

When `workstream` is not specified in task frontmatter:

1. If task has dependencies: inherit workstream from first dependency
2. Otherwise: use task ID as a single-task workstream

## Workstream Configuration

### tanuki.yaml Structure

```yaml
workstreams:
  api:
    concurrency: 2              # Max concurrent agents for this workstream
    system_prompt: |            # Workstream-specific prompt (optional)
      You are working on API development...
    system_prompt_file: ""      # Alternative: path to prompt file
```

### Defaults

If a workstream is not explicitly configured, it uses default settings:

- Concurrency: 1 (one agent at a time)
- No additional system prompt

## Workstream Scheduler

### State Tracking

```go
type WorkstreamState struct {
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
For each workstream:
  1. Get workstream concurrency limit (default: 1)
  2. Get active agents for workstream
  3. If active < limit:
     - Find next pending task by priority
     - Assign to idle agent (or spawn new if needed)
     - Mark workstream as active
  4. When workstream completes all tasks:
     - Release agent back to pool
     - Start next pending workstream
```

### Rolling Window Behavior

- Each workstream has `concurrency` slots for active agents
- When a workstream completes, its slot opens for the next
- Agents can be reused across workstreams

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
3. Start fresh agent instance on same worktree
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
2. Referenced in workstream system prompts
3. Loaded as initial context for new agent sessions

## Migration

### Existing Tasks Without Workstream

Tasks without explicit `workstream` field:

1. If `depends_on` is set: inherit from first dependency
2. Otherwise: use task ID (single-task workstream)

This preserves existing behavior while enabling workstream features.
