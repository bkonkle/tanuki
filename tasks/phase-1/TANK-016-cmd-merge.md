---
id: TANK-016
title: Command - tanuki merge
status: done
priority: medium
estimate: M
depends_on: [TANK-004]
workstream: D
phase: 1
---

# Command: tanuki merge

## Summary

Implement the `tanuki merge` command that merges an agent's work back into the current branch or creates a PR.

## Acceptance Criteria

- [x] Merges agent branch into current branch (default)
- [x] `--squash` option for squash merge
- [x] `--pr` option to create GitHub PR instead
- [x] Shows diff summary before merge
- [x] Handles merge conflicts gracefully
- [x] Optionally removes agent after successful merge (`--remove`)

## Technical Details

### Command Definition

```go
var mergeCmd = &cobra.Command{
    Use:   "merge <agent>",
    Short: "Merge an agent's work",
    Long: `Merge an agent's branch into the current branch.

By default, performs a regular merge. Use --squash to squash all commits.
Use --pr to create a GitHub pull request instead of merging locally.

Examples:
  tanuki merge auth-feature
  tanuki merge auth-feature --squash
  tanuki merge auth-feature --pr
  tanuki merge auth-feature --remove`,
    Args: cobra.ExactArgs(1),
    RunE: runMerge,
}

func init() {
    mergeCmd.Flags().Bool("squash", false, "Squash merge")
    mergeCmd.Flags().Bool("pr", false, "Create GitHub PR instead of merging")
    mergeCmd.Flags().Bool("remove", false, "Remove agent after successful merge")
    mergeCmd.Flags().Bool("no-edit", false, "Use default merge message")
    mergeCmd.Flags().StringP("message", "m", "", "Merge commit message")
    rootCmd.AddCommand(mergeCmd)
}
```

### Implementation

```go
func runMerge(cmd *cobra.Command, args []string) error {
    agentName := args[0]
    squash, _ := cmd.Flags().GetBool("squash")
    createPR, _ := cmd.Flags().GetBool("pr")
    removeAfter, _ := cmd.Flags().GetBool("remove")
    noEdit, _ := cmd.Flags().GetBool("no-edit")
    message, _ := cmd.Flags().GetString("message")

    cfg, _ := config.Load()
    agentMgr := agent.NewManager(cfg)
    gitMgr := git.NewManager(cfg)

    ag, err := agentMgr.Get(agentName)
    if err != nil {
        return fmt.Errorf("agent %q not found", agentName)
    }

    // Check for uncommitted changes in agent worktree
    status, _ := gitMgr.GetStatus(agentName)
    if status != "" {
        fmt.Println("Warning: Agent has uncommitted changes:")
        fmt.Println(status)
        fmt.Print("Continue anyway? [y/N]: ")
        var response string
        fmt.Scanln(&response)
        if strings.ToLower(response) != "y" {
            return nil
        }
    }

    // Show summary
    fmt.Printf("Agent: %s\n", ag.Name)
    fmt.Printf("Branch: %s\n", ag.Branch)
    diff, _ := gitMgr.GetDiffStat(agentName, gitMgr.GetCurrentBranch())
    fmt.Printf("\nChanges:\n%s\n", diff)

    if createPR {
        return createPullRequest(ag, cfg)
    }

    return mergeBranch(ag, gitMgr, squash, noEdit, message, removeAfter, agentMgr)
}

func mergeBranch(ag *agent.Agent, gitMgr *git.Manager, squash, noEdit bool, message string, removeAfter bool, agentMgr *agent.Manager) error {
    mergeArgs := []string{"merge"}

    if squash {
        mergeArgs = append(mergeArgs, "--squash")
    }

    if noEdit {
        mergeArgs = append(mergeArgs, "--no-edit")
    }

    if message != "" {
        mergeArgs = append(mergeArgs, "-m", message)
    }

    mergeArgs = append(mergeArgs, ag.Branch)

    cmd := exec.Command("git", mergeArgs...)
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr

    if err := cmd.Run(); err != nil {
        return fmt.Errorf("merge failed: %w\nResolve conflicts and run: git merge --continue", err)
    }

    // For squash merge, need to commit
    if squash {
        commitMsg := message
        if commitMsg == "" {
            commitMsg = fmt.Sprintf("Merge work from agent %s", ag.Name)
        }
        commitCmd := exec.Command("git", "commit", "-m", commitMsg)
        commitCmd.Stdout = os.Stdout
        commitCmd.Stderr = os.Stderr
        commitCmd.Run()
    }

    fmt.Printf("\nSuccessfully merged %s\n", ag.Branch)

    if removeAfter {
        fmt.Printf("Removing agent %s...\n", ag.Name)
        agentMgr.Remove(ag.Name, agent.RemoveOptions{Force: true, KeepBranch: false})
    }

    return nil
}

func createPullRequest(ag *agent.Agent, cfg *config.Config) error {
    // Push branch to remote first
    fmt.Println("Pushing branch to remote...")
    pushCmd := exec.Command("git", "push", "-u", "origin", ag.Branch)
    pushCmd.Stdout = os.Stdout
    pushCmd.Stderr = os.Stderr
    if err := pushCmd.Run(); err != nil {
        return fmt.Errorf("failed to push branch: %w", err)
    }

    // Create PR using gh CLI
    fmt.Println("Creating pull request...")
    prCmd := exec.Command("gh", "pr", "create",
        "--head", ag.Branch,
        "--title", fmt.Sprintf("[Tanuki] %s", ag.Name),
        "--body", fmt.Sprintf("Work completed by Tanuki agent `%s`.\n\nCreated automatically by `tanuki merge --pr`.", ag.Name),
    )
    prCmd.Stdout = os.Stdout
    prCmd.Stderr = os.Stderr
    prCmd.Stdin = os.Stdin

    return prCmd.Run()
}
```

### Output

```
$ tanuki merge auth-feature
Agent: auth-feature
Branch: tanuki/auth-feature

Changes:
 src/auth/oauth.ts       | 50 +++++++++++++++++++++++++++++++++++
 src/auth/index.ts       |  2 ++
 tests/auth/oauth.test.ts| 30 +++++++++++++++++++++
 3 files changed, 82 insertions(+)

Merge made by the 'recursive' strategy.

Successfully merged tanuki/auth-feature
```

```
$ tanuki merge auth-feature --pr
Agent: auth-feature
Branch: tanuki/auth-feature

Changes:
 3 files changed, 82 insertions(+)

Pushing branch to remote...
Creating pull request...

https://github.com/user/repo/pull/42
```

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--squash` | | Squash all commits into one |
| `--pr` | | Create GitHub PR instead |
| `--remove` | | Remove agent after merge |
| `--no-edit` | | Use default merge message |
| `--message` | `-m` | Custom merge message |

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Agent not found | Error with list suggestion |
| Merge conflicts | Show resolution instructions |
| gh CLI not installed | Error with install suggestion |
| Not authenticated to GitHub | Error with auth suggestion |
| Uncommitted changes | Warning with confirmation |

## Out of Scope

- Interactive conflict resolution
- Rebase option
- Auto-merge when conflicts exist

## Notes

The `--pr` option requires GitHub CLI (`gh`) to be installed and authenticated. This is the recommended workflow for team collaboration.
