# Changelog

All notable changes to Tanuki will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2026-01-13

### Added

- **CLI Framework** (TANK-001)
  - Cobra-based CLI with version info and help system
  - Consistent error handling and output formatting

- **Configuration Management** (TANK-002)
  - YAML configuration file support (`tanuki.yaml`)
  - Sensible defaults for zero-config operation
  - Environment variable overrides

- **State Management** (TANK-003)
  - Persistent JSON state file (`.tanuki/state.json`)
  - Agent lifecycle tracking
  - Task history and session recording

- **Git Worktree Manager** (TANK-004)
  - Automatic worktree creation with `tanuki/` branch prefix
  - Clean removal with optional branch preservation
  - Main branch auto-detection

- **Docker Container Manager** (TANK-005)
  - Container lifecycle management (create, start, stop, remove)
  - Network isolation with dedicated `tanuki` network
  - Resource usage monitoring

- **Agent Manager** (TANK-006)
  - Unified agent lifecycle orchestration
  - Atomic spawn with rollback on failure
  - Status reconciliation

- **Claude Code Executor** (TANK-007)
  - Execute prompts via `claude -p` flag
  - Streaming output support
  - Tool allow/deny list support

- **Commands**
  - `tanuki init` - Initialize Tanuki in a project (TANK-008)
  - `tanuki spawn` - Create new agents with `-n` for batch creation (TANK-009)
  - `tanuki list` - List all agents with status (TANK-010)
  - `tanuki run` - Execute tasks with `--follow` and `--ralph` modes (TANK-011)
  - `tanuki logs` - View agent output with `--follow` option (TANK-012)
  - `tanuki diff` - Show agent changes with `--stat` and `--name-only` (TANK-013)
  - `tanuki attach` - Attach to running Claude session (TANK-014)
  - `tanuki stop/start/remove` - Agent lifecycle management (TANK-015)
  - `tanuki merge` - Merge agent work with `--squash` and `--pr` options (TANK-016)
  - `tanuki status` - Detailed agent status with resource usage (TANK-017)

- **Docker Image** (TANK-018)
  - Ubuntu 24.04 base with Node.js 22, Python 3
  - Claude Code pre-installed
  - Development tools: git, gh, ripgrep, fd, fzf, vim, neovim

### Infrastructure

- Makefile with build, test, lint, and install targets
- GoReleaser configuration for multi-platform releases
- Multi-platform Docker image build scripts (amd64/arm64)

[Unreleased]: https://github.com/bkonkle/tanuki/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/bkonkle/tanuki/releases/tag/v0.1.0
