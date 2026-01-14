// Package git provides Git worktree operations for agent isolation.
package git

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bkonkle/tanuki/internal/config"
)

// ErrNotGitRepo indicates the current directory is not a Git repository.
var ErrNotGitRepo = errors.New("not a git repository")

// ErrBranchExists indicates the branch already exists.
var ErrBranchExists = errors.New("branch already exists")

// ErrWorktreeExists indicates the worktree path already exists.
var ErrWorktreeExists = errors.New("worktree already exists")

// Manager handles Git worktree operations for agent isolation.
type Manager struct {
	repoRoot     string
	branchPrefix string
}

// NewManager creates a new Git worktree manager.
// It requires the current directory to be within a Git repository.
func NewManager(cfg *config.Config) (*Manager, error) {
	if !IsGitRepo() {
		return nil, ErrNotGitRepo
	}

	root, err := GetRepoRoot()
	if err != nil {
		return nil, fmt.Errorf("failed to get repo root: %w", err)
	}

	return &Manager{
		repoRoot:     root,
		branchPrefix: cfg.Git.BranchPrefix,
	}, nil
}

// CreateWorktree creates a new worktree with a new branch for an agent.
// The worktree is created at .tanuki/worktrees/<name>/ with branch tanuki/<name>.
func (m *Manager) CreateWorktree(name string) (string, error) {
	branchName := m.branchName(name)
	worktreePath := m.worktreePath(name)
	absWorktreePath := filepath.Join(m.repoRoot, worktreePath)

	// Check if branch already exists
	if m.branchExists(branchName) {
		return "", fmt.Errorf("%w: %s", ErrBranchExists, branchName)
	}

	// Check if worktree path already exists
	if _, err := os.Stat(absWorktreePath); err == nil {
		return "", fmt.Errorf("%w: %s", ErrWorktreeExists, worktreePath)
	}

	// Ensure parent directory exists
	parentDir := filepath.Dir(absWorktreePath)
	if err := os.MkdirAll(parentDir, 0750); err != nil {
		return "", fmt.Errorf("failed to create worktree parent directory: %w", err)
	}

	// Create worktree with new branch
	cmd := exec.Command("git", "worktree", "add", "-b", branchName, absWorktreePath) //nolint:gosec // G204: branchName and absWorktreePath are derived from validated config inputs
	cmd.Dir = m.repoRoot
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to create worktree: %s", stderr.String())
	}

	return absWorktreePath, nil
}

// RemoveWorktree removes the worktree and optionally the associated branch.
func (m *Manager) RemoveWorktree(name string, deleteBranch bool) error {
	absWorktreePath := filepath.Join(m.repoRoot, m.worktreePath(name))
	branchName := m.branchName(name)

	// Remove worktree
	cmd := exec.Command("git", "worktree", "remove", absWorktreePath, "--force") //nolint:gosec // G204: absWorktreePath is derived from validated config inputs
	cmd.Dir = m.repoRoot
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to remove worktree: %s", stderr.String())
	}

	// Optionally delete branch
	if deleteBranch {
		cmd = exec.Command("git", "branch", "-D", branchName) //nolint:gosec // G204: branchName is derived from validated config inputs
		cmd.Dir = m.repoRoot
		// Ignore error if branch doesn't exist
		_ = cmd.Run()
	}

	return nil
}

// GetDiff returns the diff between the agent's branch and the base branch.
// The diff shows all changes made in the agent's worktree.
func (m *Manager) GetDiff(name string, baseBranch string) (string, error) {
	branchName := m.branchName(name)

	// Use three-dot syntax to show changes introduced in branchName
	cmd := exec.Command("git", "diff", baseBranch+"..."+branchName) //nolint:gosec // G204: baseBranch and branchName are derived from validated config inputs
	cmd.Dir = m.repoRoot
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", fmt.Errorf("failed to get diff: %s", string(exitErr.Stderr))
		}
		return "", fmt.Errorf("failed to get diff: %w", err)
	}

	return string(output), nil
}

// GetStatus returns uncommitted changes in the agent's worktree.
func (m *Manager) GetStatus(name string) (string, error) {
	absWorktreePath := filepath.Join(m.repoRoot, m.worktreePath(name))

	// Check if worktree exists
	if _, err := os.Stat(absWorktreePath); os.IsNotExist(err) {
		return "", fmt.Errorf("worktree does not exist: %s", name)
	}

	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = absWorktreePath
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", fmt.Errorf("failed to get status: %s", string(exitErr.Stderr))
		}
		return "", fmt.Errorf("failed to get status: %w", err)
	}

	return string(output), nil
}

// GetCurrentBranch returns the current branch name of the main repository.
func (m *Manager) GetCurrentBranch() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = m.repoRoot
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", fmt.Errorf("failed to get current branch: %s", string(exitErr.Stderr))
		}
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}

	branch := strings.TrimSpace(string(output))
	if branch == "HEAD" {
		// Detached HEAD state
		return "", nil
	}
	return branch, nil
}

// GetMainBranch detects the main branch (main, master, or trunk).
func (m *Manager) GetMainBranch() (string, error) {
	// Check common main branch names in order of preference
	candidates := []string{"main", "master", "trunk"}

	for _, candidate := range candidates {
		if m.branchExists(candidate) {
			return candidate, nil
		}
	}

	// Fall back to checking remote references
	cmd := exec.Command("git", "symbolic-ref", "refs/remotes/origin/HEAD")
	cmd.Dir = m.repoRoot
	output, err := cmd.Output()
	if err == nil {
		// Output is like "refs/remotes/origin/main"
		ref := strings.TrimSpace(string(output))
		parts := strings.Split(ref, "/")
		if len(parts) > 0 {
			return parts[len(parts)-1], nil
		}
	}

	return "", errors.New("could not determine main branch")
}

// WorktreeExists checks if a worktree exists for the given agent name.
func (m *Manager) WorktreeExists(name string) bool {
	absWorktreePath := filepath.Join(m.repoRoot, m.worktreePath(name))
	_, err := os.Stat(absWorktreePath)
	return err == nil
}

// BranchExists checks if the branch for the given agent name exists.
func (m *Manager) BranchExists(name string) bool {
	return m.branchExists(m.branchName(name))
}

// GetWorktreePath returns the path to the worktree for an agent.
func (m *Manager) GetWorktreePath(name string) string {
	return filepath.Join(m.repoRoot, m.worktreePath(name))
}

// GetBranchName returns the branch name for an agent.
func (m *Manager) GetBranchName(name string) string {
	return m.branchName(name)
}

// branchName returns the full branch name for an agent.
func (m *Manager) branchName(name string) string {
	return m.branchPrefix + name
}

// worktreePath returns the relative path to the worktree for an agent.
func (m *Manager) worktreePath(name string) string {
	return filepath.Join(".tanuki", "worktrees", name)
}

// branchExists checks if a branch exists in the repository.
func (m *Manager) branchExists(branchName string) bool {
	cmd := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/"+branchName) //nolint:gosec // G204: branchName is derived from validated config inputs
	cmd.Dir = m.repoRoot
	return cmd.Run() == nil
}
