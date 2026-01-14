# TODO: Ralph-First Projects, Roles, Workstreams

## Goal

Make the README workflow real: Projects -> Roles -> Workstreams -> Tasks, with Ralph-only execution,
project docs, role-scoped prompts, and per-role workstream concurrency.

## Plan

- [ ] Define the workstream model and defaults: how to assign `workstream` when missing, how to
      order workstreams per role, and how to detect a workstream as complete. (Design doc)
- [ ] Extend task schema to include `workstream` and update parsing/validation. Update managers and
      tests to support `role + workstream` filtering and ordering. (`internal/task/types.go`,
      `internal/task/parser.go`, `internal/task/manager.go`, `internal/task/queue.go`,
      `internal/task/*_test.go`)
- [ ] Add role config to `tanuki.yaml` with `system_prompt` and `concurrency`, wire into config
      loader/validator and defaults. (`internal/config/config.go`,
      `internal/config/defaults.go`, `internal/config/config_test.go`)
- [ ] Merge role config with existing role files and builtin roles (define precedence), and ensure
      CLI role listing/show includes config-defined roles. (`internal/role/manager.go`,
      `internal/cli/role.go`)
- [ ] Update project init to create `.tanuki/project.md` and sample ticket with `workstream`.
      (`internal/cli/project_init.go`)
- [ ] Load project doc into agent context (either auto-context files or prompt injection). Decide
      whether to copy into worktree, include in system prompt, or both. (`internal/agent/manager.go`,
      `internal/context/context.go`)
- [ ] Implement workstream scheduler with per-role concurrency and rolling window behavior:
      active workstreams limited by `roles.<name>.concurrency`, tasks within a workstream run
      sequentially on the same agent/worktree, and new workstreams start as others complete.
      (`internal/project/orchestrator.go` or new `internal/project/workstream` package)
- [ ] Make Ralph the only execution mode:
      - CLI `tanuki run` always uses Ralph loop (remove async mode and `--ralph` switch).
        (`internal/cli/run.go`)
      - Task runner uses Ralph loop for all tasks (define default completion signal if none).
        (`internal/task/runner.go`, `internal/task/validator.go`)
      - Align verifier location (host vs container) and ensure output is captured for validation.
        (`internal/executor/executor.go`, `internal/agent/manager.go`)
- [ ] Add context-budget logic for workstreams (max turns/iterations). When budget is hit, emit
      artifacts and start a fresh Ralph instance for the same workstream. (`internal/task/runner.go`,
      new workstream state tracking in `internal/state/state.go`)
- [ ] Update project CLI and status/dashboard to surface workstreams:
      - `project start` uses config concurrency (remove `--agents-per-role`).
        (`internal/cli/project_start.go`)
      - `project status` shows active workstreams per role and queue depth.
        (`internal/cli/project_status.go`, `internal/tui/*`)
      - `project stop/resume` handles workstream agents cleanly.
        (`internal/cli/project_stop.go`, `internal/cli/project_resume.go`)
- [ ] Add or update tests for new config/roles, task schema, scheduler behavior, and Ralph-only
      execution. (`internal/config/*_test.go`, `internal/task/*_test.go`,
      `internal/project/*_test.go`, `internal/cli/*_test.go`)
- [ ] Document migration: existing task files without `workstream`, role overrides via
      `.tanuki/roles/`, and removal of async mode. (README + changelog)
