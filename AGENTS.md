# Contributing Guidelines

Guidelines for contributing to Tanuki.

## Project Structure

- `bin/` — Built binaries
- `cmd/` — Go entry points
- `internal/` — Go packages
- `scripts/` — Build and utility scripts

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

## Workflow

1. **Plan:** Goal, Changes, Impact, Verification
2. **Edit:** Minimal diff, focused changes
3. **Verify:** Run unit tests before submitting

## Critical Invariants

- **Security:** No secrets committed
- **Docs:** Update related docs and comments with behavior changes
- **Git:** Keep commits reversible
