# Changelog

All notable changes to Tanuki will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- **Default Model** - Changed default Claude model to Haiku 4.5 (claude-haiku-4-5-20250107)
  - Faster execution and lower cost for typical agent tasks
  - Users can override in tanuki.yaml or per-role configuration

- **Container Runtime** - Switched from custom bkonkle/tanuki image to node:22
  - Removed Dockerfile and image build scripts
  - Updated default image configuration to node:22
  - Simplified deployment and maintenance

### Fixed

- **Claude CLI Integration**
  - Fixed `--output-format stream-json` flag compatibility by adding required `--verbose` flag
  - Updated default model from claude-sonnet-4-5-20250514 to claude-sonnet-4-5-20250929

- **API Key Mount**
  - Fixed Claude Code config mount from `~/.config/claude-code` to `~/.claude` (actual location)
  - Changed mount target from `/home/dev/.claude` to `/home/node/.claude` for node:22 image
  - Made mount read-write (was read-only, Claude needs to write debug logs)

- **Container Execution**
  - Added `--user node` to all docker exec commands for proper permissions
  - Fixed file permissions for node user in container

- **Docker Desktop Visibility**
  - Added log streaming to Docker Desktop via tee to `/tmp/tanuki.log`
  - Background tail process streams logs to container stdout for visibility
  - Logs now visible in Docker Desktop container view

## [0.4.0] - 2026-01-13

### Added

- **Shared Services** (TANK-040, TANK-042, TANK-043)
  - Service manager for Postgres, Redis, and custom services
  - YAML configuration schema in `tanuki.yaml`
  - Health monitoring with automatic restart on failure
  - Volume persistence for service data

- **Service CLI Commands** (TANK-044)
  - `tanuki service start [name]` - Start all or specific service
  - `tanuki service stop [name]` - Stop all or specific service
  - `tanuki service status` - Show service health and ports
  - `tanuki service logs <name>` - Stream service logs
  - `tanuki service connect <name>` - Open interactive connection (psql, redis-cli)

- **Agent Service Injection** (TANK-045)
  - Automatic environment variable injection into agent containers
  - Connection info for all running services (host, port, URL, credentials)
  - Services accessible via Docker network

- **TUI Dashboard** (TANK-041, TANK-046, TANK-047, TANK-048)
  - BubbleTea-based interactive terminal interface
  - Three-pane layout: agents, tasks, logs
  - Real-time status updates and log streaming
  - Keyboard navigation and quick actions
  - `tanuki dashboard` command

## [0.3.0] - 2026-01-13

### Added

- **Task File System** (TANK-030)
  - Markdown files with YAML front matter in `.tanuki/tasks/`
  - Task schema: id, title, role, priority, status, depends_on, completion
  - Ralph-style completion criteria (verify commands, signals)

- **Task Manager** (TANK-033)
  - Scan, get, update, and assign tasks
  - Role-based task filtering
  - Status transitions and history tracking

- **Dependency Resolver** (TANK-035)
  - Topological sort for task ordering
  - Cycle detection with clear error messages
  - Blocked state for unmet dependencies

- **Task Queue** (TANK-034)
  - Priority-based queue implementation
  - Role-aware task dequeuing
  - Thread-safe operations

- **Workload Balancer** (TANK-036)
  - Intelligent agent assignment strategy
  - Idle agent detection
  - Even distribution across available agents

- **Status Tracker** (TANK-037)
  - Task lifecycle management
  - Status transition validation
  - Assignment history

- **Project Commands** (TANK-031)
  - `tanuki project init` - Initialize task directory
  - `tanuki project start` - Scan tasks, spawn agents, begin distribution
  - `tanuki project status` - Show task and agent progress
  - `tanuki project stop` - Stop all project agents
  - `tanuki project resume` - Resume a stopped project

- **Task Completion Validation** (TANK-032)
  - Ralph-style verification (commands and signals)
  - Automatic task reassignment on completion
  - Configurable max iterations

- **Project Orchestrator** (TANK-038)
  - Main control loop for automated task distribution
  - Event-driven architecture
  - Graceful shutdown and resume support

## [0.2.0] - 2026-01-13

### Added

- **Role Configuration System** (TANK-020, TANK-023)
  - YAML-based role definitions
  - Role inheritance support
  - Validation with helpful error messages

- **Role-Based Tool Filtering** (TANK-024)
  - Allow/deny lists for Claude tools
  - Role-specific capabilities (e.g., QA can only write tests)

- **Built-in Role Library** (TANK-026)
  - `backend` - Server-side development, APIs, databases
  - `frontend` - UI development, components, styling
  - `qa` - Testing and quality assurance (restricted to tests)
  - `docs` - Documentation and guides
  - `devops` - Infrastructure, CI/CD, deployment
  - `fullstack` - End-to-end feature development

- **Spawn with Role** (TANK-021)
  - `tanuki spawn <name> --role <role>` flag
  - Automatic CLAUDE.md generation with role prompt
  - Role validation on spawn

- **Role Management Commands** (TANK-022)
  - `tanuki role list` - List available roles
  - `tanuki role show <role>` - Display role configuration
  - `tanuki role init` - Create .tanuki/roles/ for customization
  - `tanuki role create <name>` - Generate custom role template

- **Context File Management** (TANK-025)
  - Automatic context file injection per role
  - Project-specific context in `.tanuki/context/`

- **Role Template Generation** (TANK-027)
  - Generate custom role YAML templates
  - Inheritance from built-in roles

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

### Infrastructure

- Makefile with build, test, lint, and install targets
- GoReleaser configuration for multi-platform releases

[Unreleased]: https://github.com/bkonkle/tanuki/compare/v0.4.0...HEAD
[0.4.0]: https://github.com/bkonkle/tanuki/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/bkonkle/tanuki/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/bkonkle/tanuki/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/bkonkle/tanuki/releases/tag/v0.1.0
