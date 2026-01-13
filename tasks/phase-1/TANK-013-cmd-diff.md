---
id: TANK-013
title: Command - tanuki diff
status: done
priority: medium
estimate: S
depends_on: [TANK-004]
workstream: D
phase: 1
---

# Command: tanuki diff

## Summary

Implement the `tanuki diff` command that shows changes made by an agent compared to the base branch.

## Acceptance Criteria

- [x] Shows git diff between agent branch and main/master
- [x] Supports `--stat` for summary view
- [x] Supports `--name-only` to list changed files
- [x] Auto-detects main branch (main, master, trunk)
- [x] Can specify custom base branch

## Technical Details

### Command Definition

```go
var diffCmd = &cobra.Command{
    Use:   "diff <agent>",
    Short: "Show changes made by an agent",
    Long: `Show git diff between an agent's branch and the base branch.

Examples:
  tanuki diff auth-feature
  tanuki diff auth-feature --stat
  tanuki diff auth-feature --name-only
  tanuki diff auth-feature --base develop`,
    Args: cobra.ExactArgs(1),
    RunE: runDiff,
}

func init() {
    diffCmd.Flags().Bool("stat", false, "Show diffstat instead of patch")
    diffCmd.Flags().Bool("name-only", false, "Show only names of changed files")
    diffCmd.Flags().String("base", "", "Base branch to compare against (default: auto-detect)")
    rootCmd.AddCommand(diffCmd)
}
```

### Implementation

```go
func runDiff(cmd *cobra.Command, args []string) error {
    agentName := args[0]
    stat, _ := cmd.Flags().GetBool("stat")
    nameOnly, _ := cmd.Flags().GetBool("name-only")
    baseBranch, _ := cmd.Flags().GetString("base")

    cfg, err := config.Load()
    if err != nil {
        return err
    }

    agentMgr := agent.NewManager(cfg)

    ag, err := agentMgr.Get(agentName)
    if err != nil {
        return fmt.Errorf("agent %q not found", agentName)
    }

    gitMgr := git.NewManager(cfg)

    // Auto-detect base branch if not specified
    if baseBranch == "" {
        baseBranch, err = gitMgr.GetMainBranch()
        if err != nil {
            baseBranch = "main" // Fallback
        }
    }

    // Build diff command
    args := []string{"diff"}

    if stat {
        args = append(args, "--stat")
    } else if nameOnly {
        args = append(args, "--name-only")
    }

    // Compare base...agent_branch (three-dot diff)
    args = append(args, fmt.Sprintf("%s...%s", baseBranch, ag.Branch))

    gitCmd := exec.Command("git", args...)
    gitCmd.Stdout = os.Stdout
    gitCmd.Stderr = os.Stderr

    return gitCmd.Run()
}
```

### Output

```
$ tanuki diff auth-feature
diff --git a/src/auth/oauth.ts b/src/auth/oauth.ts
new file mode 100644
index 0000000..abc1234
--- /dev/null
+++ b/src/auth/oauth.ts
@@ -0,0 +1,50 @@
+export class OAuthProvider {
+  ...
+}
```

```
$ tanuki diff auth-feature --stat
 src/auth/oauth.ts       | 50 +++++++++++++++++++++++++++++++++++
 src/auth/index.ts       |  2 ++
 tests/auth/oauth.test.ts| 30 +++++++++++++++++++++
 3 files changed, 82 insertions(+)
```

```
$ tanuki diff auth-feature --name-only
src/auth/oauth.ts
src/auth/index.ts
tests/auth/oauth.test.ts
```

### Flags

| Flag | Description |
|------|-------------|
| `--stat` | Show diffstat summary |
| `--name-only` | Show only filenames |
| `--base` | Base branch to compare against |

### Main Branch Detection

```go
func (g *GitManager) GetMainBranch() (string, error) {
    // Try common main branch names
    for _, name := range []string{"main", "master", "trunk"} {
        cmd := exec.Command("git", "rev-parse", "--verify", name)
        if err := cmd.Run(); err == nil {
            return name, nil
        }
    }

    // Fall back to HEAD's upstream
    cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "origin/HEAD")
    output, err := cmd.Output()
    if err == nil {
        return strings.TrimPrefix(strings.TrimSpace(string(output)), "origin/"), nil
    }

    return "", fmt.Errorf("could not detect main branch")
}
```

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Agent not found | Error with list suggestion |
| No changes | Empty output (not an error) |
| Base branch not found | Error with suggestion to specify --base |

## Out of Scope

- Interactive diff viewer
- Side-by-side diff

## Notes

Use three-dot diff (`base...branch`) to show only changes introduced in the agent's branch, not changes in base since branching.
