# Contributing Guidelines

Guidelines for contributing to Tanuki.

## Project Structure

- `bin/` — Built binaries
- `cmd/` — Go entry points
- `internal/` — Go packages
- `docs/` — Design documentation

## Git Guidelines

**Commit Template:** `<type>(<scope>): <summary>`

Types: `feat`, `fix`, `docs`, `refactor`, `test`, `chore`

**When to Split PRs:**

- Unrelated concerns in one PR
- PR is too large for effective review
- Mixed refactoring with feature work

**Safety:**

- Redact secrets before committing
- Human approval required for irreversible migrations

## Changelog

**REQUIRED:** Update `CHANGELOG.md` for all user-facing changes:

- **Added** — New features, commands, or capabilities
- **Changed** — Changes to existing functionality
- **Deprecated** — Features marked for removal
- **Removed** — Deleted features or files
- **Fixed** — Bug fixes
- **Security** — Vulnerability fixes

Format: Follow [Keep a Changelog](https://keepachangelog.com/) conventions. Add entries to the `[Unreleased]` section. Include ticket references when applicable.

**When to update:**
- New CLI commands or flags
- Configuration schema changes
- Breaking changes to behavior
- Bug fixes that affect users
- Security patches

**When to skip:**
- Internal refactoring with no user impact
- Test-only changes
- Documentation typo fixes

## Workflow

1. **Plan:** Define goal, changes, impact, and verification steps
2. **Edit:** Make minimal, focused changes
3. **Test:**
   - Run `make test` for unit tests
   - Run `make lint` to check code quality
   - Test manually with real workflows when changing container/CLI behavior
4. **Document:** Update CHANGELOG.md, README.md, and inline docs as needed
5. **Commit:** Follow commit template with clear rationale

## Development Setup

**Prerequisites:**
- Go 1.21+
- Docker (with daemon running)
- Git
- Make

**Build & Install:**
```bash
make build    # Build binary to bin/
make install  # Install to $GOPATH/bin
make test     # Run tests
make lint     # Run linters
```

**Testing Changes:**
```bash
# Build and install locally
go install ./cmd/tanuki

# Test basic workflow
tanuki spawn test-agent
tanuki run test-agent "echo hello"
tanuki logs test-agent
tanuki remove test-agent
```

## Container Development

Tanuki uses Docker containers to isolate agent execution. Key considerations:

**Container Image:**
- Uses standard `node:22` image (no custom builds)
- Claude Code CLI installed at runtime via npm
- Runs as `node` user (uid 1000) for permissions

**Mounts:**
- Worktree: `<project>/.tanuki/worktrees/<agent>` → `/workspace`
- Claude config: `~/.claude` → `/home/node/.claude` (read-write)

**Logging:**
- Commands tee output to `/tmp/tanuki.log` inside container
- Background `tail -F` process streams to container stdout
- Visible in Docker Desktop container logs

**Testing Container Changes:**
1. Make changes to `internal/docker/` or `internal/executor/`
2. Rebuild: `go install ./cmd/tanuki`
3. Clean old containers: `docker rm -f $(docker ps -aq --filter "name=tanuki")`
4. Test: `tanuki spawn test && tanuki run test "test command"`
5. Verify logs: `docker logs tanuki-test`

## Critical Invariants

- **Security:** No secrets committed
- **Changelog:** Update CHANGELOG.md for user-facing changes
- **Docs:** Update related docs and comments with behavior changes
- **Git:** Keep commits reversible
- **Testing:** Verify changes don't break existing workflows
