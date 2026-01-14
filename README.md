# Tanuki

A multi-agent orchestration CLI for code agents. Spawn isolated agents, each with their own git
worktree and Docker container, to work on tasks in parallel.

## Features

- **Isolated Agents**: Each agent gets its own git branch and Docker container
- **Parallel Execution**: Run multiple Claude Code instances simultaneously
- **Three Execution Modes**: Fire-and-forget, follow (streaming), and Ralph (autonomous loop)
- **Git Integration**: Automatic worktree management, diff viewing, and merge support
- **State Management**: Persistent tracking of all agents and their tasks

## Installation

### From Source

```bash
cd tanuki
make build
make install  # Installs to $GOPATH/bin
```

### Using Go

```bash
go install github.com/bkonkle/tanuki/cmd/tanuki@latest
```

## Quick Start

1. **Initialize Tanuki** in your project:

   ```bash
   cd your-project
   tanuki init
   ```

2. **Spawn an agent**:

   ```bash
   tanuki spawn auth-feature
   ```

3. **Send a task**:

   ```bash
   tanuki run auth-feature "Implement OAuth2 login with Google"
   ```

4. **Check progress**:

   ```bash
   tanuki status auth-feature
   tanuki logs auth-feature --follow
   ```

5. **Review and merge**:

   ```bash
   tanuki diff auth-feature
   tanuki merge auth-feature
   ```

## Commands

### Agent Lifecycle

| Command                | Description                                    |
| ---------------------- | ---------------------------------------------- |
| `tanuki spawn <name>`  | Create a new agent with worktree and container |
| `tanuki list`          | List all agents and their status               |
| `tanuki status <name>` | Show detailed agent status                     |
| `tanuki stop <name>`   | Stop an agent's container                      |
| `tanuki start <name>`  | Start a stopped agent                          |
| `tanuki remove <name>` | Remove agent completely                        |

### Task Execution

| Command                                  | Description                      |
| ---------------------------------------- | -------------------------------- |
| `tanuki run <agent> "<prompt>"`          | Send a task (async by default)   |
| `tanuki run <agent> "<prompt>" --follow` | Stream output in real-time       |
| `tanuki run <agent> "<prompt>" --ralph`  | Loop until completion signal     |
| `tanuki logs <agent>`                    | View agent's Claude Code output  |
| `tanuki attach <agent>`                  | Attach to running Claude session |

### Git Operations

| Command                     | Description                |
| --------------------------- | -------------------------- |
| `tanuki diff <agent>`       | Show changes made by agent |
| `tanuki merge <agent>`      | Merge agent's branch       |
| `tanuki merge <agent> --pr` | Create GitHub PR instead   |

## Execution Modes

### Default (Async)

Returns immediately, task runs in background:

```bash
tanuki run auth "Implement OAuth2 login"
# Task sent to auth
# Check progress: tanuki logs auth --follow
```

### Follow Mode

Streams output in real-time:

```bash
tanuki run auth "Add unit tests" --follow
# Running task on auth (following output)...
# [Claude Code output streams here]
```

### Ralph Mode

Loops until completion signal or max iterations:

```bash
tanuki run auth "Fix all lint errors. Say DONE when clean." --ralph
# Running auth in Ralph mode (max 30 iterations)...
# === Ralph iteration 1/30 ===
# [Claude output...]
# === Completion signal detected: DONE ===
```

With verification command:

```bash
tanuki run auth "Get tests passing" --ralph --verify "npm test"
```

## Configuration

Tanuki works without configuration using sensible defaults. Optionally create `tanuki.yaml`:

```yaml
version: '1'

image:
  name: bkonkle/tanuki
  tag: latest

network:
  name: tanuki

worktrees:
  prefix: tanuki
  base_dir: .tanuki/worktrees

defaults:
  max_turns: 50
  model: claude-sonnet-4-20250514
```

## Docker Image

Build the agent container image:

```bash
cd tanuki
./scripts/build-image.sh
```

Or pull from Docker Hub:

```bash
docker pull bkonkle/tanuki:latest
```

## Requirements

- Go 1.21+
- Docker
- Git
- GitHub CLI (`gh`) for `--pr` option

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        tanuki CLI                           │
├─────────────────────────────────────────────────────────────┤
│  Agent Manager  │  Git Manager  │  Docker Manager  │  State │
├─────────────────────────────────────────────────────────────┤
│                     Claude Code Executor                    │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│   ┌─────────┐    ┌─────────┐    ┌─────────┐                │
│   │ Agent 1 │    │ Agent 2 │    │ Agent N │                │
│   │─────────│    │─────────│    │─────────│                │
│   │Container│    │Container│    │Container│                │
│   │Worktree │    │Worktree │    │Worktree │                │
│   │ Branch  │    │ Branch  │    │ Branch  │                │
│   └─────────┘    └─────────┘    └─────────┘                │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

## License

MIT
