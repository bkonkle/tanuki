---
id: TANK-025
title: Context File Management
status: done
priority: high
estimate: M
depends_on: [TANK-021]
workstream: C
phase: 2
---

# Context File Management

## Summary

Implement smart context file copying that provides agents with relevant project documentation. Context files are specified in roles and automatically made available to agents in their worktree.

## Acceptance Criteria

- [ ] Copy context files specified in role to agent worktree
- [ ] Support glob patterns in context_files (e.g., `docs/**/*.md`)
- [ ] Symlinks vs copies (configurable)
- [ ] Handle missing context files gracefully (warning, not error)
- [ ] Context files organized in `.tanuki/context/` within worktree
- [ ] Context file references in CLAUDE.md
- [ ] Clear output showing which files were copied
- [ ] Test coverage for glob expansion and error cases

## Technical Details

### Context File Manager

```go
// internal/context/manager.go
package context

import (
    "fmt"
    "io"
    "os"
    "path/filepath"
)

type Manager struct {
    projectRoot string
    useSymlinks bool
    logger      Logger
}

type Logger interface {
    Infof(format string, args ...interface{})
    Warnf(format string, args ...interface{})
}

func NewManager(projectRoot string, useSymlinks bool, logger Logger) *Manager {
    return &Manager{
        projectRoot: projectRoot,
        useSymlinks: useSymlinks,
        logger:      logger,
    }
}

type CopyResult struct {
    Copied  []string
    Skipped []string
    Errors  []error
}

func (m *Manager) CopyContextFiles(
    worktreePath string,
    contextPatterns []string,
) (*CopyResult, error) {
    result := &CopyResult{
        Copied:  make([]string, 0),
        Skipped: make([]string, 0),
        Errors:  make([]error, 0),
    }

    contextDir := filepath.Join(worktreePath, ".tanuki", "context")
    if err := os.MkdirAll(contextDir, 0755); err != nil {
        return nil, fmt.Errorf("create context directory: %w", err)
    }

    for _, pattern := range contextPatterns {
        matches, err := m.expandGlob(pattern)
        if err != nil {
            m.logger.Warnf("Invalid pattern %q: %v", pattern, err)
            result.Errors = append(result.Errors, err)
            continue
        }

        if len(matches) == 0 {
            m.logger.Warnf("No files match pattern %q", pattern)
            result.Skipped = append(result.Skipped, pattern)
            continue
        }

        for _, srcPath := range matches {
            relPath, err := filepath.Rel(m.projectRoot, srcPath)
            if err != nil {
                m.logger.Warnf("Cannot determine relative path for %s: %v", srcPath, err)
                result.Errors = append(result.Errors, err)
                continue
            }

            dstPath := filepath.Join(contextDir, relPath)

            if err := m.copyFile(srcPath, dstPath); err != nil {
                m.logger.Warnf("Failed to copy %s: %v", relPath, err)
                result.Errors = append(result.Errors, err)
                continue
            }

            result.Copied = append(result.Copied, relPath)
            m.logger.Infof("  ✓ %s", relPath)
        }
    }

    return result, nil
}

func (m *Manager) expandGlob(pattern string) ([]string, error) {
    // Make pattern relative to project root
    fullPattern := filepath.Join(m.projectRoot, pattern)

    matches, err := filepath.Glob(fullPattern)
    if err != nil {
        return nil, fmt.Errorf("glob pattern: %w", err)
    }

    // Filter out directories
    files := make([]string, 0, len(matches))
    for _, match := range matches {
        info, err := os.Stat(match)
        if err != nil {
            continue // Skip files we can't stat
        }
        if !info.IsDir() {
            files = append(files, match)
        }
    }

    return files, nil
}

func (m *Manager) copyFile(src, dst string) error {
    // Create parent directory
    if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
        return fmt.Errorf("create parent directory: %w", err)
    }

    if m.useSymlinks {
        // Remove existing symlink/file if present
        os.Remove(dst)

        // Create symlink
        if err := os.Symlink(src, dst); err != nil {
            return fmt.Errorf("create symlink: %w", err)
        }
    } else {
        // Copy file
        srcFile, err := os.Open(src)
        if err != nil {
            return fmt.Errorf("open source: %w", err)
        }
        defer srcFile.Close()

        dstFile, err := os.Create(dst)
        if err != nil {
            return fmt.Errorf("create destination: %w", err)
        }
        defer dstFile.Close()

        if _, err := io.Copy(dstFile, srcFile); err != nil {
            return fmt.Errorf("copy contents: %w", err)
        }

        // Copy permissions
        srcInfo, _ := srcFile.Stat()
        if err := os.Chmod(dst, srcInfo.Mode()); err != nil {
            return fmt.Errorf("set permissions: %w", err)
        }
    }

    return nil
}

func (m *Manager) GetContextDir(worktreePath string) string {
    return filepath.Join(worktreePath, ".tanuki", "context")
}
```

### Integration with Agent Spawn

```go
// internal/agent/manager.go

func (m *Manager) Spawn(name string, opts SpawnOptions) (*Agent, error) {
    // ... existing spawn logic (worktree, container, etc.)

    // If role specified, apply role configuration
    if opts.Role != "" {
        role, err := m.roleManager.Get(opts.Role)
        if err != nil {
            return nil, fmt.Errorf("role %q not found", opts.Role)
        }

        // Copy context files
        if len(role.ContextFiles) > 0 {
            fmt.Println("\nCopying context files...")

            contextMgr := context.NewManager(
                m.config.ProjectRoot,
                m.config.Context.UseSymlinks,
                m.logger,
            )

            result, err := contextMgr.CopyContextFiles(worktreePath, role.ContextFiles)
            if err != nil {
                return nil, fmt.Errorf("copy context files: %w", err)
            }

            if len(result.Copied) > 0 {
                fmt.Printf("Copied %d context file(s)\n", len(result.Copied))
            }
            if len(result.Skipped) > 0 {
                fmt.Printf("Skipped %d pattern(s) with no matches\n", len(result.Skipped))
            }
            if len(result.Errors) > 0 {
                fmt.Printf("Warning: %d error(s) occurred while copying context files\n", len(result.Errors))
            }
        }

        // Generate CLAUDE.md with context file references
        if err := m.generateClaudeMD(worktreePath, role, contextMgr.GetContextDir(worktreePath)); err != nil {
            return nil, fmt.Errorf("generate CLAUDE.md: %w", err)
        }

        // Store role in agent state
        agent.Role = opts.Role
        agent.AllowedTools = role.AllowedTools
        agent.DisallowedTools = role.DisallowedTools
    }

    // ... save state
}
```

### CLAUDE.md Generation with Context

```go
func (m *Manager) generateClaudeMD(worktreePath string, role *Role, contextDir string) error {
    claudeMDPath := filepath.Join(worktreePath, "CLAUDE.md")

    var content strings.Builder

    // Add role system prompt
    content.WriteString("# Agent Instructions\n\n")
    content.WriteString(role.SystemPrompt)
    content.WriteString("\n\n")

    // Add context file references
    if len(role.ContextFiles) > 0 {
        content.WriteString("## Context Files\n\n")
        content.WriteString("Review these files for project context:\n\n")

        // List actual copied files (not just patterns)
        err := filepath.Walk(contextDir, func(path string, info os.FileInfo, err error) error {
            if err != nil || info.IsDir() {
                return err
            }

            relPath, _ := filepath.Rel(worktreePath, path)
            content.WriteString(fmt.Sprintf("- [%s](%s)\n", info.Name(), relPath))
            return nil
        })

        if err != nil {
            return fmt.Errorf("walk context directory: %w", err)
        }
    }

    return os.WriteFile(claudeMDPath, []byte(content.String()), 0644)
}
```

### Configuration

```yaml
# tanuki.yaml

# Context file settings (Phase 2)
context:
  # Use symlinks instead of copying files
  use_symlinks: true

  # Base directory for context files within worktree
  context_dir: ".tanuki/context"

  # Maximum total size of context files (prevent huge copies)
  max_total_size: "10MB"

  # Auto-update context files on each run (expensive)
  auto_update: false
```

## Directory Structure

### Worktree Layout

```
.tanuki/worktrees/backend-worker/
├── src/                           # Project code
├── docs/                          # Project docs
├── .tanuki/
│   ├── context/                   # Context files from role
│   │   ├── docs/
│   │   │   ├── architecture.md    # Symlink or copy
│   │   │   └── api-conventions.md
│   │   └── CONTRIBUTING.md
│   └── ...
├── CLAUDE.md                      # Generated with context references
└── ...
```

### Example CLAUDE.md

```markdown
# Agent Instructions

You are a backend development specialist. Focus on:
- API design and implementation
- Database operations and optimization
- Server-side business logic
- Security best practices

Always write tests for new functionality.
Follow the project's existing patterns and conventions.

## Context Files

Review these files for project context:

- [architecture.md](.tanuki/context/docs/architecture.md)
- [api-conventions.md](.tanuki/context/docs/api-conventions.md)
- [database-schema.md](.tanuki/context/docs/database-schema.md)
- [CONTRIBUTING.md](.tanuki/context/CONTRIBUTING.md)
```

## Output Examples

```bash
$ tanuki spawn backend-worker --role backend
Spawning agent backend-worker with role 'backend'...
  Created worktree: .tanuki/worktrees/backend-worker
  Created container: tanuki-backend-worker

Copying context files...
  ✓ docs/architecture.md
  ✓ docs/api-conventions.md
  ✓ docs/database-schema.md
  ✓ CONTRIBUTING.md
Copied 4 context file(s)

Generated CLAUDE.md with role instructions.

Agent backend-worker ready
  Role:      backend
  Branch:    tanuki/backend-worker
  Container: tanuki-backend-worker
  Worktree:  .tanuki/worktrees/backend-worker

Run a task:
  tanuki run backend-worker "your task here"
```

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Context file not found | Warning (continue without file) |
| Invalid glob pattern | Warning (skip pattern) |
| No matches for pattern | Warning (skip pattern) |
| Permission denied | Warning (skip file) |
| Destination exists | Overwrite (symlinks/copies) |
| Context directory creation fails | Error (fatal) |
| All context files failed | Warning (not fatal) |

## Testing

### Unit Tests

```go
func TestManager_CopyContextFiles(t *testing.T) {
    tests := []struct {
        name         string
        patterns     []string
        setupFiles   map[string]string // path -> content
        wantCopied   []string
        wantSkipped  []string
        wantErrors   int
    }{
        {
            name:     "single file",
            patterns: []string{"docs/README.md"},
            setupFiles: map[string]string{
                "docs/README.md": "content",
            },
            wantCopied: []string{"docs/README.md"},
        },
        {
            name:     "glob pattern",
            patterns: []string{"docs/**/*.md"},
            setupFiles: map[string]string{
                "docs/README.md":          "content1",
                "docs/api/endpoints.md":   "content2",
                "docs/guides/setup.md":    "content3",
            },
            wantCopied: []string{
                "docs/README.md",
                "docs/api/endpoints.md",
                "docs/guides/setup.md",
            },
        },
        {
            name:        "no matches",
            patterns:    []string{"nonexistent/**/*.md"},
            setupFiles:  map[string]string{},
            wantSkipped: []string{"nonexistent/**/*.md"},
        },
        {
            name:     "mixed success and failure",
            patterns: []string{"docs/exists.md", "docs/missing.md"},
            setupFiles: map[string]string{
                "docs/exists.md": "content",
            },
            wantCopied:  []string{"docs/exists.md"},
            wantSkipped: []string{"docs/missing.md"},
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Setup test environment
            projectRoot := t.TempDir()
            worktreeRoot := t.TempDir()

            // Create test files
            for path, content := range tt.setupFiles {
                fullPath := filepath.Join(projectRoot, path)
                os.MkdirAll(filepath.Dir(fullPath), 0755)
                os.WriteFile(fullPath, []byte(content), 0644)
            }

            m := NewManager(projectRoot, false, &testLogger{})
            result, err := m.CopyContextFiles(worktreeRoot, tt.patterns)

            if err != nil {
                t.Fatalf("unexpected error: %v", err)
            }

            if len(result.Copied) != len(tt.wantCopied) {
                t.Errorf("copied %d files, want %d", len(result.Copied), len(tt.wantCopied))
            }

            // ... more assertions
        })
    }
}

func TestManager_Symlinks(t *testing.T) {
    // Test symlink mode vs copy mode
    // ...
}
```

### Integration Tests

```bash
# Test with role that has context files
tanuki spawn test-backend --role backend
ls -la .tanuki/worktrees/test-backend/.tanuki/context/
# Verify: docs/architecture.md, docs/api-conventions.md, etc. present

# Test CLAUDE.md contains context references
cat .tanuki/worktrees/test-backend/CLAUDE.md
# Verify: Contains "Context Files" section with links

# Test with non-existent pattern (should warn, not fail)
# Edit role to include "nonexistent/**/*.md"
tanuki spawn test-warn --role modified-backend
# Verify: Warning about no matches, but spawn succeeds

# Test symlinks vs copies
# Edit tanuki.yaml: use_symlinks: true
tanuki spawn test-symlink --role backend
file .tanuki/worktrees/test-symlink/.tanuki/context/docs/architecture.md
# Verify: Reports as symbolic link
```

## Out of Scope

- Dynamic context file updates during agent execution
- Context file size limits (just copy all)
- Context file deduplication across agents
- Selective context file copying (all or nothing per role)
- Context file versioning

## Notes

- Symlinks are faster and save disk space but may break if project root moves
- Copies are safer for portability but use more disk space
- Context files are read-only from agent's perspective
- Missing context files generate warnings but don't block spawn
- Context directory is excluded from git in worktree (in `.tanuki/`)
