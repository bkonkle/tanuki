---
id: TANK-004
title: Git Worktree Manager
status: done
priority: high
estimate: M
depends_on: [TANK-001]
workstream: B
phase: 1
---

# Git Worktree Manager

## Summary

Implement Git worktree operations for creating isolated workspaces per agent. Each agent gets its own worktree with a dedicated branch.

## Acceptance Criteria

- [x] Create worktree in `.tanuki/worktrees/<name>/`
- [x] Create branch `tanuki/<name>` from current HEAD
- [x] Remove worktree and branch on cleanup
- [x] Get diff between agent branch and main
- [x] Verify repo is clean before operations (or handle dirty state)
- [x] Handle edge cases: branch exists, worktree exists, not a git repo

## Technical Details

### Worktree Operations

```go
type GitManager interface {
    // CreateWorktree creates a new worktree with a new branch
    CreateWorktree(name string) (worktreePath string, err error)

    // RemoveWorktree removes the worktree and optionally the branch
    RemoveWorktree(name string, deleteBranch bool) error

    // GetDiff returns the diff between agent branch and base branch
    GetDiff(name string, baseBranch string) (string, error)

    // GetStatus returns uncommitted changes in the worktree
    GetStatus(name string) (string, error)

    // IsGitRepo checks if current directory is a git repository
    IsGitRepo() bool

    // GetCurrentBranch returns the current branch name
    GetCurrentBranch() (string, error)

    // GetMainBranch detects main/master/trunk
    GetMainBranch() (string, error)
}
```

### Directory Structure

```
project/
├── .git/
├── .tanuki/
│   └── worktrees/
│       ├── auth-feature/      # Worktree for agent
│       │   ├── .git           # (file, points to main .git)
│       │   ├── src/
│       │   └── ...
│       └── api-refactor/
│           └── ...
└── src/
```

### Implementation

```go
func (g *GitManager) CreateWorktree(name string) (string, error) {
    branchName := fmt.Sprintf("%s%s", g.config.Git.BranchPrefix, name)
    worktreePath := filepath.Join(".tanuki", "worktrees", name)

    // Check if branch already exists
    if g.branchExists(branchName) {
        return "", fmt.Errorf("branch %s already exists", branchName)
    }

    // Check if worktree path already exists
    if _, err := os.Stat(worktreePath); err == nil {
        return "", fmt.Errorf("worktree path %s already exists", worktreePath)
    }

    // Create worktree with new branch
    cmd := exec.Command("git", "worktree", "add", "-b", branchName, worktreePath)
    if err := cmd.Run(); err != nil {
        return "", fmt.Errorf("failed to create worktree: %w", err)
    }

    return worktreePath, nil
}

func (g *GitManager) RemoveWorktree(name string, deleteBranch bool) error {
    worktreePath := filepath.Join(".tanuki", "worktrees", name)
    branchName := fmt.Sprintf("%s%s", g.config.Git.BranchPrefix, name)

    // Remove worktree
    cmd := exec.Command("git", "worktree", "remove", worktreePath, "--force")
    if err := cmd.Run(); err != nil {
        return fmt.Errorf("failed to remove worktree: %w", err)
    }

    // Optionally delete branch
    if deleteBranch {
        cmd = exec.Command("git", "branch", "-D", branchName)
        cmd.Run() // Ignore error if branch doesn't exist
    }

    return nil
}

func (g *GitManager) GetDiff(name string, baseBranch string) (string, error) {
    branchName := fmt.Sprintf("%s%s", g.config.Git.BranchPrefix, name)

    cmd := exec.Command("git", "diff", baseBranch+"..."+branchName)
    output, err := cmd.Output()
    if err != nil {
        return "", fmt.Errorf("failed to get diff: %w", err)
    }

    return string(output), nil
}
```

### Error Handling

| Scenario | Behavior |
|----------|----------|
| Not a git repo | Return clear error, suggest `git init` |
| Branch exists | Return error with suggestion to use different name |
| Worktree exists | Return error, don't overwrite |
| Dirty working tree | Warn but allow (agents work in isolated worktrees) |
| Detached HEAD | Use current commit as base |

### Git Configuration

Ensure worktrees are ignored:

```gitignore
# .gitignore
.tanuki/worktrees/
.tanuki/state/
```

## Out of Scope

- Merge/rebase operations (separate task)
- Push to remote (separate task)
- PR creation (Phase 2+)

## Notes

Use `git` CLI directly rather than a Go git library - more reliable and easier to debug. Shell out to git commands.
