package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/bkonkle/tanuki/internal/config"
)

// setupTestRepo creates a temporary git repository for testing.
func setupTestRepo(t *testing.T) (string, func()) {
	t.Helper()

	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "tanuki-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	// Initialize a git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		_ = os.RemoveAll(tmpDir)
		t.Fatalf("failed to init git repo: %v", err)
	}

	// Configure git user for commits
	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		_ = os.RemoveAll(tmpDir)
		t.Fatalf("failed to configure git email: %v", err)
	}

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		_ = os.RemoveAll(tmpDir)
		t.Fatalf("failed to configure git username: %v", err)
	}

	// Create an initial commit (required for worktrees)
	testFile := filepath.Join(tmpDir, "README.md")
	if err := os.WriteFile(testFile, []byte("# Test\n"), 0600); err != nil {
		_ = os.RemoveAll(tmpDir)
		t.Fatalf("failed to create test file: %v", err)
	}

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		_ = os.RemoveAll(tmpDir)
		t.Fatalf("failed to git add: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		_ = os.RemoveAll(tmpDir)
		t.Fatalf("failed to git commit: %v", err)
	}

	cleanup := func() {
		// Clean up any worktrees first
		cmd := exec.Command("git", "worktree", "list", "--porcelain")
		cmd.Dir = tmpDir
		_ = cmd.Run()

		_ = os.RemoveAll(tmpDir)
	}

	return tmpDir, cleanup
}

// createTestManager creates a Manager pointing at the test repo.
func createTestManager(t *testing.T, repoPath string) *Manager {
	t.Helper()

	cfg := config.DefaultConfig()
	return &Manager{
		repoRoot:     repoPath,
		branchPrefix: cfg.Git.BranchPrefix,
	}
}

func TestNewManager(t *testing.T) {
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	// Save current dir and change to test repo
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current dir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	if chErr := os.Chdir(repoPath); chErr != nil {
		t.Fatalf("failed to chdir: %v", chErr)
	}

	cfg := config.DefaultConfig()
	manager, err := NewManager(cfg)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	if manager == nil {
		t.Fatal("NewManager returned nil")
	}

	// Resolve symlinks for comparison (macOS /var -> /private/var)
	expectedRoot, _ := filepath.EvalSymlinks(repoPath)
	actualRoot, _ := filepath.EvalSymlinks(manager.repoRoot)
	if actualRoot != expectedRoot {
		t.Errorf("repoRoot = %q, want %q", manager.repoRoot, repoPath)
	}

	if manager.branchPrefix != cfg.Git.BranchPrefix {
		t.Errorf("branchPrefix = %q, want %q", manager.branchPrefix, cfg.Git.BranchPrefix)
	}
}

func TestNewManager_NotGitRepo(t *testing.T) {
	// Create a temp dir that is NOT a git repo
	tmpDir, err := os.MkdirTemp("", "tanuki-test-notgit-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current dir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	if chErr := os.Chdir(tmpDir); chErr != nil {
		t.Fatalf("failed to chdir: %v", chErr)
	}

	cfg := config.DefaultConfig()
	_, err = NewManager(cfg)
	if err != ErrNotGitRepo {
		t.Errorf("NewManager error = %v, want ErrNotGitRepo", err)
	}
}

func TestCreateWorktree(t *testing.T) {
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	manager := createTestManager(t, repoPath)

	worktreePath, err := manager.CreateWorktree("test-agent")
	if err != nil {
		t.Fatalf("CreateWorktree failed: %v", err)
	}

	// Verify worktree was created
	expectedPath := filepath.Join(repoPath, ".tanuki", "worktrees", "test-agent")
	if worktreePath != expectedPath {
		t.Errorf("worktreePath = %q, want %q", worktreePath, expectedPath)
	}

	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		t.Error("worktree directory was not created")
	}

	// Verify branch was created
	if !manager.BranchExists("test-agent") {
		t.Error("branch was not created")
	}
}

func TestCreateWorktree_BranchExists(t *testing.T) {
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	manager := createTestManager(t, repoPath)

	// Create a branch with the same name
	branchName := manager.branchPrefix + "test-agent"
	cmd := exec.Command("git", "branch", branchName) //nolint:gosec // G204: branchName is test data derived from known prefix
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to create branch: %v", err)
	}

	_, err := manager.CreateWorktree("test-agent")
	if err == nil {
		t.Error("CreateWorktree should fail when branch exists")
	}
}

func TestCreateWorktree_WorktreeExists(t *testing.T) {
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	manager := createTestManager(t, repoPath)

	// Create the worktree directory
	worktreeDir := filepath.Join(repoPath, ".tanuki", "worktrees", "test-agent")
	if err := os.MkdirAll(worktreeDir, 0750); err != nil {
		t.Fatalf("failed to create worktree dir: %v", err)
	}

	_, err := manager.CreateWorktree("test-agent")
	if err == nil {
		t.Error("CreateWorktree should fail when worktree path exists")
	}
}

func TestRemoveWorktree(t *testing.T) {
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	manager := createTestManager(t, repoPath)

	// Create a worktree first
	worktreePath, err := manager.CreateWorktree("test-agent")
	if err != nil {
		t.Fatalf("CreateWorktree failed: %v", err)
	}

	// Remove it
	err = manager.RemoveWorktree("test-agent", true)
	if err != nil {
		t.Fatalf("RemoveWorktree failed: %v", err)
	}

	// Verify worktree was removed
	if _, err := os.Stat(worktreePath); !os.IsNotExist(err) {
		t.Error("worktree directory was not removed")
	}

	// Verify branch was deleted
	if manager.BranchExists("test-agent") {
		t.Error("branch was not deleted")
	}
}

func TestRemoveWorktree_KeepBranch(t *testing.T) {
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	manager := createTestManager(t, repoPath)

	// Create a worktree first
	worktreePath, err := manager.CreateWorktree("test-agent")
	if err != nil {
		t.Fatalf("CreateWorktree failed: %v", err)
	}

	// Remove it but keep the branch
	err = manager.RemoveWorktree("test-agent", false)
	if err != nil {
		t.Fatalf("RemoveWorktree failed: %v", err)
	}

	// Verify worktree was removed
	if _, err := os.Stat(worktreePath); !os.IsNotExist(err) {
		t.Error("worktree directory was not removed")
	}

	// Verify branch still exists
	if !manager.BranchExists("test-agent") {
		t.Error("branch should not have been deleted")
	}
}

func TestGetStatus(t *testing.T) {
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	manager := createTestManager(t, repoPath)

	// Create a worktree
	worktreePath, err := manager.CreateWorktree("test-agent")
	if err != nil {
		t.Fatalf("CreateWorktree failed: %v", err)
	}

	// Create an untracked file in the worktree
	testFile := filepath.Join(worktreePath, "new-file.txt")
	if writeErr := os.WriteFile(testFile, []byte("test content\n"), 0600); writeErr != nil {
		t.Fatalf("failed to create test file: %v", writeErr)
	}

	status, err := manager.GetStatus("test-agent")
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}

	if status == "" {
		t.Error("GetStatus should return non-empty status for untracked file")
	}

	// Status should contain the new file
	if len(status) == 0 {
		t.Error("status should contain the new file")
	}
}

func TestGetStatus_NoChanges(t *testing.T) {
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	manager := createTestManager(t, repoPath)

	// Create a worktree
	_, err := manager.CreateWorktree("test-agent")
	if err != nil {
		t.Fatalf("CreateWorktree failed: %v", err)
	}

	status, err := manager.GetStatus("test-agent")
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}

	if status != "" {
		t.Errorf("GetStatus should return empty for clean worktree, got: %q", status)
	}
}

func TestGetDiff(t *testing.T) {
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	manager := createTestManager(t, repoPath)

	// Create a worktree
	worktreePath, err := manager.CreateWorktree("test-agent")
	if err != nil {
		t.Fatalf("CreateWorktree failed: %v", err)
	}

	// Make a change and commit it in the worktree
	testFile := filepath.Join(worktreePath, "new-file.txt")
	if writeErr := os.WriteFile(testFile, []byte("test content\n"), 0600); writeErr != nil {
		t.Fatalf("failed to create test file: %v", writeErr)
	}

	cmd := exec.Command("git", "add", ".")
	cmd.Dir = worktreePath
	if addErr := cmd.Run(); addErr != nil {
		t.Fatalf("failed to git add: %v", addErr)
	}

	cmd = exec.Command("git", "commit", "-m", "Add new file")
	cmd.Dir = worktreePath
	if commitErr := cmd.Run(); commitErr != nil {
		t.Fatalf("failed to git commit: %v", commitErr)
	}

	// Get the current branch name to use as base
	currentBranch, err := manager.GetCurrentBranch()
	if err != nil {
		t.Fatalf("GetCurrentBranch failed: %v", err)
	}
	if currentBranch == "" {
		currentBranch = "main" // fallback for test repo
	}

	// Get diff - use main as base since that's the default in test repo
	diff, err := manager.GetDiff("test-agent", currentBranch)
	if err != nil {
		t.Fatalf("GetDiff failed: %v", err)
	}

	if diff == "" {
		t.Error("GetDiff should return non-empty diff for committed changes")
	}

	// Diff should mention the new file
	if !contains(diff, "new-file.txt") {
		t.Error("diff should mention new-file.txt")
	}
}

func TestGetCurrentBranch(t *testing.T) {
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	manager := createTestManager(t, repoPath)

	branch, err := manager.GetCurrentBranch()
	if err != nil {
		t.Fatalf("GetCurrentBranch failed: %v", err)
	}

	// Default branch in new repos is usually main or master
	if branch != "main" && branch != "master" {
		t.Errorf("GetCurrentBranch = %q, want main or master", branch)
	}
}

func TestGetMainBranch(t *testing.T) {
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	manager := createTestManager(t, repoPath)

	mainBranch, err := manager.GetMainBranch()
	if err != nil {
		t.Fatalf("GetMainBranch failed: %v", err)
	}

	// Should detect main or master
	if mainBranch != "main" && mainBranch != "master" {
		t.Errorf("GetMainBranch = %q, want main or master", mainBranch)
	}
}

func TestWorktreeExists(t *testing.T) {
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	manager := createTestManager(t, repoPath)

	// Should not exist before creation
	if manager.WorktreeExists("test-agent") {
		t.Error("WorktreeExists should return false before creation")
	}

	// Create worktree
	_, err := manager.CreateWorktree("test-agent")
	if err != nil {
		t.Fatalf("CreateWorktree failed: %v", err)
	}

	// Should exist after creation
	if !manager.WorktreeExists("test-agent") {
		t.Error("WorktreeExists should return true after creation")
	}
}

func TestBranchExists(t *testing.T) {
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	manager := createTestManager(t, repoPath)

	// Should not exist before creation
	if manager.BranchExists("test-agent") {
		t.Error("BranchExists should return false before creation")
	}

	// Create worktree (which creates branch)
	_, err := manager.CreateWorktree("test-agent")
	if err != nil {
		t.Fatalf("CreateWorktree failed: %v", err)
	}

	// Should exist after creation
	if !manager.BranchExists("test-agent") {
		t.Error("BranchExists should return true after creation")
	}
}

func TestGetWorktreePath(t *testing.T) {
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	manager := createTestManager(t, repoPath)

	path := manager.GetWorktreePath("test-agent")
	expected := filepath.Join(repoPath, ".tanuki", "worktrees", "test-agent")

	if path != expected {
		t.Errorf("GetWorktreePath = %q, want %q", path, expected)
	}
}

func TestGetBranchName(t *testing.T) {
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	manager := createTestManager(t, repoPath)

	branchName := manager.GetBranchName("test-agent")
	expected := "tanuki/test-agent"

	if branchName != expected {
		t.Errorf("GetBranchName = %q, want %q", branchName, expected)
	}
}

// contains checks if a string contains a substring.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
