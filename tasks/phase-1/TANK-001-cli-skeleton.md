---
id: TANK-001
title: CLI Skeleton and Project Structure
status: done
priority: high
estimate: M
depends_on: []
workstream: A
phase: 1
---

# CLI Skeleton and Project Structure

## Summary

Set up the Go project structure with command-line parsing using Cobra. This is the foundation for all other CLI commands.

## Acceptance Criteria

- [x] Go module initialized (`go mod init github.com/bkonkle/tanuki`)
- [x] Cobra CLI framework installed and configured
- [x] Root command with version flag (`tanuki --version`)
- [x] Help text displays available commands
- [x] Project follows standard Go project layout
- [x] Makefile with `build`, `install`, `test` targets
- [x] `.goreleaser.yml` for cross-platform builds (macOS, Linux, Windows)

## Technical Details

### Project Structure

```
tanuki/
├── cmd/
│   └── tanuki/
│       └── main.go           # Entry point
├── internal/
│   ├── cli/
│   │   ├── root.go           # Root command
│   │   ├── spawn.go          # spawn command
│   │   ├── run.go            # run command
│   │   └── ...
│   ├── agent/
│   │   └── manager.go        # Agent lifecycle management
│   ├── git/
│   │   └── worktree.go       # Git worktree operations
│   ├── docker/
│   │   └── container.go      # Docker container operations
│   ├── config/
│   │   └── config.go         # Configuration loading
│   └── state/
│       └── state.go          # State persistence
├── go.mod
├── go.sum
├── Makefile
└── .goreleaser.yml
```

### Dependencies

- `github.com/spf13/cobra` - CLI framework
- `github.com/spf13/viper` - Configuration management
- `gopkg.in/yaml.v3` - YAML parsing

### Root Command

```go
var rootCmd = &cobra.Command{
    Use:   "tanuki",
    Short: "Multi-agent orchestration for Claude Code",
    Long: `Tanuki orchestrates multiple Claude Code agents in isolated
Docker containers, enabling parallel development without conflicts.`,
}
```

## Out of Scope

- Actual command implementations (separate tasks)
- Docker integration
- Git operations

## Notes

Keep the CLI structure extensible for future commands. Use Cobra's subcommand pattern consistently.
