---
id: TANK-002
title: Configuration Management
status: done
priority: high
estimate: M
depends_on: [TANK-001]
workstream: A
phase: 1
---

# Configuration Management

## Summary

Implement configuration file loading and management. Tanuki needs to read project-level configuration from `tanuki.yaml` and manage global settings.

## Acceptance Criteria

- [x] Load `tanuki.yaml` from project root or `.tanuki/config/`
- [x] Support global config at `~/.config/tanuki/config.yaml`
- [x] Merge configs with precedence: CLI flags > project > global > defaults
- [x] `tanuki init` command creates default config
- [x] Validate config schema and report errors clearly
- [x] Config struct is well-documented and type-safe

## Technical Details

### Config File Locations (in priority order)

1. CLI flags (highest priority)
2. `./tanuki.yaml` or `./.tanuki/config/tanuki.yaml`
3. `~/.config/tanuki/config.yaml`
4. Built-in defaults (lowest priority)

### Config Schema

```yaml
# tanuki.yaml
version: "1"

# Docker image for agent containers
image:
  name: "bkonkle/tanuki"
  tag: "latest"
  # Alternative: build from local Dockerfile
  build:
    context: "."
    dockerfile: "Dockerfile.tanuki"

# Default settings for all agents
defaults:
  # Tools Claude Code is allowed to use
  allowed_tools:
    - Read
    - Write
    - Edit
    - Bash
    - Glob
    - Grep
  # Maximum conversation turns
  max_turns: 50
  # Claude model to use
  model: "claude-sonnet-4-5-20250514"
  # Resource limits
  resources:
    memory: "4g"
    cpus: "2"

# Branch naming
git:
  branch_prefix: "tanuki/"
  auto_push: false

# Network settings
network:
  name: "tanuki-net"
```

### Go Struct

```go
type Config struct {
    Version  string        `yaml:"version"`
    Image    ImageConfig   `yaml:"image"`
    Defaults AgentDefaults `yaml:"defaults"`
    Git      GitConfig     `yaml:"git"`
    Network  NetworkConfig `yaml:"network"`
}

type ImageConfig struct {
    Name  string       `yaml:"name"`
    Tag   string       `yaml:"tag"`
    Build *BuildConfig `yaml:"build,omitempty"`
}

type AgentDefaults struct {
    AllowedTools []string         `yaml:"allowed_tools"`
    MaxTurns     int              `yaml:"max_turns"`
    Model        string           `yaml:"model"`
    Resources    ResourceConfig   `yaml:"resources"`
}
```

### Init Command

```bash
tanuki init
# Creates:
# - tanuki.yaml (if not exists)
# - .tanuki/ directory structure
# - .gitignore entries for .tanuki/state/
```

## Out of Scope

- Role definitions (Phase 2)
- Service configurations (Phase 4)

## Notes

Use Viper for config management - it handles merging, env vars, and file watching well.
