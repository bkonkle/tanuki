---
id: TANK-008
title: Command - tanuki init
status: done
priority: high
estimate: S
depends_on: [TANK-002, TANK-003]
workstream: D
phase: 1
---

# Command: tanuki init

## Summary

Implement the `tanuki init` command that initializes a project for use with Tanuki. Creates the necessary directory structure and default configuration.

## Acceptance Criteria

- [x] Creates `.tanuki/` directory structure
- [x] Creates default `tanuki.yaml` config
- [x] Adds `.tanuki/` entries to `.gitignore`
- [x] Detects if already initialized (idempotent)
- [x] Works in any git repository
- [x] Creates Docker network if needed

## Technical Details

### Command Definition

```go
var initCmd = &cobra.Command{
    Use:   "init",
    Short: "Initialize tanuki in the current project",
    Long: `Initialize tanuki in the current git repository.

Creates the .tanuki/ directory structure and default configuration.
Safe to run multiple times - will not overwrite existing config.`,
    RunE: runInit,
}
```

### Directory Structure Created

```
.tanuki/
├── config/
│   └── (tanuki.yaml created here if not in root)
├── worktrees/
│   └── .gitkeep
└── state/
    └── .gitkeep
```

### Default Config

```yaml
# tanuki.yaml
version: "1"

image:
  name: "bkonkle/tanuki"
  tag: "latest"

defaults:
  allowed_tools:
    - Read
    - Write
    - Edit
    - Bash
    - Glob
    - Grep
  max_turns: 50
  model: "claude-sonnet-4-5-20250514"
  resources:
    memory: "4g"
    cpus: "2"

git:
  branch_prefix: "tanuki/"
  auto_push: false

network:
  name: "tanuki-net"
```

### .gitignore Additions

```gitignore
# Tanuki
.tanuki/worktrees/
.tanuki/state/
```

### Implementation

```go
func runInit(cmd *cobra.Command, args []string) error {
    // Check if in a git repo
    if !git.IsGitRepo() {
        return fmt.Errorf("not a git repository, run 'git init' first")
    }

    // Create directory structure
    dirs := []string{
        ".tanuki/config",
        ".tanuki/worktrees",
        ".tanuki/state",
    }
    for _, dir := range dirs {
        if err := os.MkdirAll(dir, 0755); err != nil {
            return fmt.Errorf("failed to create %s: %w", dir, err)
        }
    }

    // Create .gitkeep files
    for _, dir := range []string{".tanuki/worktrees", ".tanuki/state"} {
        gitkeep := filepath.Join(dir, ".gitkeep")
        if _, err := os.Stat(gitkeep); os.IsNotExist(err) {
            os.WriteFile(gitkeep, []byte{}, 0644)
        }
    }

    // Create config if not exists
    configPath := "tanuki.yaml"
    if _, err := os.Stat(configPath); os.IsNotExist(err) {
        if err := writeDefaultConfig(configPath); err != nil {
            return fmt.Errorf("failed to create config: %w", err)
        }
        fmt.Printf("Created %s\n", configPath)
    } else {
        fmt.Printf("Config %s already exists, skipping\n", configPath)
    }

    // Update .gitignore
    if err := updateGitignore(); err != nil {
        fmt.Printf("Warning: failed to update .gitignore: %v\n", err)
    }

    // Ensure Docker network exists
    if err := docker.EnsureNetwork("tanuki-net"); err != nil {
        fmt.Printf("Warning: failed to create Docker network: %v\n", err)
        fmt.Printf("You may need to run: docker network create tanuki-net\n")
    }

    fmt.Println("Tanuki initialized successfully!")
    fmt.Println("\nNext steps:")
    fmt.Println("  tanuki spawn <name>     # Create an agent")
    fmt.Println("  tanuki run <name> \"...\" # Send a task")

    return nil
}

func updateGitignore() error {
    gitignorePath := ".gitignore"
    entries := []string{
        "# Tanuki",
        ".tanuki/worktrees/",
        ".tanuki/state/",
    }

    // Read existing content
    existing, _ := os.ReadFile(gitignorePath)
    content := string(existing)

    // Check if already has tanuki entries
    if strings.Contains(content, ".tanuki/worktrees/") {
        return nil // Already configured
    }

    // Append entries
    if len(content) > 0 && !strings.HasSuffix(content, "\n") {
        content += "\n"
    }
    content += "\n" + strings.Join(entries, "\n") + "\n"

    return os.WriteFile(gitignorePath, []byte(content), 0644)
}
```

### Output

```
$ tanuki init
Created tanuki.yaml
Updated .gitignore
Created Docker network tanuki-net
Tanuki initialized successfully!

Next steps:
  tanuki spawn <name>     # Create an agent
  tanuki run <name> "..." # Send a task
```

### Flags

| Flag | Description |
|------|-------------|
| `--force` | Overwrite existing config |

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Not a git repo | Error with suggestion to run `git init` |
| No write permission | Clear error message |
| Docker not running | Warning, continue without network |

## Out of Scope

- Creating Dockerfile (separate task)
- Pulling Docker image

## Notes

This command should be safe to run multiple times. Never overwrite existing config without `--force`.
