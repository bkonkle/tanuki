# Agent Rules

- **Structure:** `bin/` (built binaries), `cmd/` (Go entry points), `internal/` (Go packages),
  `scripts/`, `docs/`.

## Git & Decision Guidelines

- **Commit Template:** `<type>(<scope>): <summary>` `Why: <rationale>`
- **Split PR:** Unrelated concerns, too large, mixed refactor/feature.
- **Safety:** Redact secrets. Human approval for irreversible migrations.

## Workflow & Verification

1. **Plan:** Goal, Changes, Impact, Verification.
2. **Edit:** Minimal diff.
3. **Verify:** Unit tests.

## Critical Invariants

- **Security:** No secrets committed.
- **Docs:** Update related docs and comments with behavior changes.
- **Git:** Reversible commits.
