package context

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager("/project", false)
	if m.projectRoot != "/project" {
		t.Errorf("projectRoot = %q, want %q", m.projectRoot, "/project")
	}
	if m.useSymlinks {
		t.Error("useSymlinks should be false")
	}
}

func TestManager_CopyContextFiles_SingleFile(t *testing.T) {
	projectRoot := t.TempDir()
	worktreeRoot := t.TempDir()

	// Create test file
	docsDir := filepath.Join(projectRoot, "docs")
	os.MkdirAll(docsDir, 0755)
	testFile := filepath.Join(docsDir, "README.md")
	os.WriteFile(testFile, []byte("# Test"), 0644)

	m := NewManager(projectRoot, false)
	result, err := m.CopyContextFiles(worktreeRoot, []string{"docs/README.md"})
	if err != nil {
		t.Fatalf("CopyContextFiles failed: %v", err)
	}

	if len(result.Copied) != 1 {
		t.Errorf("copied %d files, want 1", len(result.Copied))
	}
	if len(result.Skipped) != 0 {
		t.Errorf("skipped %d patterns, want 0", len(result.Skipped))
	}
	if len(result.Errors) != 0 {
		t.Errorf("got %d errors, want 0", len(result.Errors))
	}

	// Verify file was copied
	copiedPath := filepath.Join(worktreeRoot, ".tanuki", "context", "docs", "README.md")
	if _, err := os.Stat(copiedPath); os.IsNotExist(err) {
		t.Error("file was not copied")
	}
}

func TestManager_CopyContextFiles_GlobPattern(t *testing.T) {
	projectRoot := t.TempDir()
	worktreeRoot := t.TempDir()

	// Create test files
	docsDir := filepath.Join(projectRoot, "docs")
	os.MkdirAll(docsDir, 0755)
	os.WriteFile(filepath.Join(docsDir, "file1.md"), []byte("# File 1"), 0644)
	os.WriteFile(filepath.Join(docsDir, "file2.md"), []byte("# File 2"), 0644)
	os.WriteFile(filepath.Join(docsDir, "file3.txt"), []byte("Not markdown"), 0644)

	m := NewManager(projectRoot, false)
	result, err := m.CopyContextFiles(worktreeRoot, []string{"docs/*.md"})
	if err != nil {
		t.Fatalf("CopyContextFiles failed: %v", err)
	}

	if len(result.Copied) != 2 {
		t.Errorf("copied %d files, want 2", len(result.Copied))
	}
}

func TestManager_CopyContextFiles_NoMatches(t *testing.T) {
	projectRoot := t.TempDir()
	worktreeRoot := t.TempDir()

	m := NewManager(projectRoot, false)
	result, err := m.CopyContextFiles(worktreeRoot, []string{"nonexistent/*.md"})
	if err != nil {
		t.Fatalf("CopyContextFiles failed: %v", err)
	}

	if len(result.Copied) != 0 {
		t.Errorf("copied %d files, want 0", len(result.Copied))
	}
	if len(result.Skipped) != 1 {
		t.Errorf("skipped %d patterns, want 1", len(result.Skipped))
	}
}

func TestManager_CopyContextFiles_EmptyPatterns(t *testing.T) {
	projectRoot := t.TempDir()
	worktreeRoot := t.TempDir()

	m := NewManager(projectRoot, false)
	result, err := m.CopyContextFiles(worktreeRoot, []string{})
	if err != nil {
		t.Fatalf("CopyContextFiles failed: %v", err)
	}

	if len(result.Copied) != 0 {
		t.Errorf("copied %d files, want 0", len(result.Copied))
	}
}

func TestManager_CopyContextFiles_Symlinks(t *testing.T) {
	projectRoot := t.TempDir()
	worktreeRoot := t.TempDir()

	// Create test file
	docsDir := filepath.Join(projectRoot, "docs")
	os.MkdirAll(docsDir, 0755)
	testFile := filepath.Join(docsDir, "README.md")
	os.WriteFile(testFile, []byte("# Test"), 0644)

	m := NewManager(projectRoot, true) // Use symlinks
	result, err := m.CopyContextFiles(worktreeRoot, []string{"docs/README.md"})
	if err != nil {
		t.Fatalf("CopyContextFiles failed: %v", err)
	}

	if len(result.Copied) != 1 {
		t.Errorf("copied %d files, want 1", len(result.Copied))
	}

	// Verify symlink was created
	copiedPath := filepath.Join(worktreeRoot, ".tanuki", "context", "docs", "README.md")
	info, err := os.Lstat(copiedPath)
	if err != nil {
		t.Fatalf("Lstat failed: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("expected symlink, got regular file")
	}
}

func TestManager_CopyContextFiles_MixedPatterns(t *testing.T) {
	projectRoot := t.TempDir()
	worktreeRoot := t.TempDir()

	// Create test files
	docsDir := filepath.Join(projectRoot, "docs")
	os.MkdirAll(docsDir, 0755)
	os.WriteFile(filepath.Join(docsDir, "exists.md"), []byte("# Exists"), 0644)

	m := NewManager(projectRoot, false)
	result, err := m.CopyContextFiles(worktreeRoot, []string{
		"docs/exists.md",
		"docs/missing.md",
	})
	if err != nil {
		t.Fatalf("CopyContextFiles failed: %v", err)
	}

	if len(result.Copied) != 1 {
		t.Errorf("copied %d files, want 1", len(result.Copied))
	}
	if len(result.Skipped) != 1 {
		t.Errorf("skipped %d patterns, want 1", len(result.Skipped))
	}
}

func TestManager_GetContextDir(t *testing.T) {
	m := NewManager("/project", false)
	dir := m.GetContextDir("/worktree")
	expected := "/worktree/.tanuki/context"
	if dir != expected {
		t.Errorf("GetContextDir = %q, want %q", dir, expected)
	}
}

func TestManager_ListContextFiles(t *testing.T) {
	worktreeRoot := t.TempDir()

	// Create context directory with files
	contextDir := filepath.Join(worktreeRoot, ".tanuki", "context", "docs")
	os.MkdirAll(contextDir, 0755)
	os.WriteFile(filepath.Join(contextDir, "file1.md"), []byte("# File 1"), 0644)
	os.WriteFile(filepath.Join(contextDir, "file2.md"), []byte("# File 2"), 0644)

	m := NewManager("/project", false)
	files, err := m.ListContextFiles(worktreeRoot)
	if err != nil {
		t.Fatalf("ListContextFiles failed: %v", err)
	}

	if len(files) != 2 {
		t.Errorf("listed %d files, want 2", len(files))
	}
}

func TestManager_ListContextFiles_EmptyDir(t *testing.T) {
	worktreeRoot := t.TempDir()

	m := NewManager("/project", false)
	files, err := m.ListContextFiles(worktreeRoot)
	if err != nil {
		t.Fatalf("ListContextFiles failed: %v", err)
	}

	if len(files) != 0 {
		t.Errorf("listed %d files, want 0", len(files))
	}
}
