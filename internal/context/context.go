// Package context provides context file management for Tanuki agents.
//
// Context files are project files that are automatically copied or symlinked
// to agent worktrees to provide relevant context during task execution.
package context

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Manager handles context file operations.
type Manager struct {
	projectRoot string
	useSymlinks bool
}

// NewManager creates a new context manager.
func NewManager(projectRoot string, useSymlinks bool) *Manager {
	return &Manager{
		projectRoot: projectRoot,
		useSymlinks: useSymlinks,
	}
}

// CopyResult contains the results of a copy operation.
type CopyResult struct {
	// Copied lists files that were successfully copied
	Copied []string

	// Skipped lists patterns that had no matches
	Skipped []string

	// Errors lists any errors encountered during copying
	Errors []error
}

// CopyContextFiles copies context files to the agent's worktree.
// Files are placed in .tanuki/context/ within the worktree, maintaining
// their relative path structure from the project root.
func (m *Manager) CopyContextFiles(worktreePath string, contextPatterns []string) (*CopyResult, error) {
	result := &CopyResult{
		Copied:  make([]string, 0),
		Skipped: make([]string, 0),
		Errors:  make([]error, 0),
	}

	if len(contextPatterns) == 0 {
		return result, nil
	}

	contextDir := filepath.Join(worktreePath, ".tanuki", "context")
	if err := os.MkdirAll(contextDir, 0755); err != nil {
		return nil, fmt.Errorf("create context directory: %w", err)
	}

	for _, pattern := range contextPatterns {
		matches, err := m.expandGlob(pattern)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("invalid pattern %q: %w", pattern, err))
			continue
		}

		if len(matches) == 0 {
			result.Skipped = append(result.Skipped, pattern)
			continue
		}

		for _, srcPath := range matches {
			relPath, err := filepath.Rel(m.projectRoot, srcPath)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Errorf("cannot determine relative path for %s: %w", srcPath, err))
				continue
			}

			dstPath := filepath.Join(contextDir, relPath)

			if err := m.copyFile(srcPath, dstPath); err != nil {
				result.Errors = append(result.Errors, fmt.Errorf("failed to copy %s: %w", relPath, err))
				continue
			}

			result.Copied = append(result.Copied, relPath)
		}
	}

	return result, nil
}

// expandGlob expands a glob pattern relative to the project root.
// Returns only regular files (not directories).
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

// copyFile copies a file or creates a symlink depending on configuration.
func (m *Manager) copyFile(src, dst string) error {
	// Create parent directory
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}

	// Remove existing file/symlink if present
	os.Remove(dst)

	if m.useSymlinks {
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

// GetContextDir returns the context directory path within a worktree.
func (m *Manager) GetContextDir(worktreePath string) string {
	return filepath.Join(worktreePath, ".tanuki", "context")
}

// ListContextFiles returns all files in the context directory.
func (m *Manager) ListContextFiles(worktreePath string) ([]string, error) {
	contextDir := m.GetContextDir(worktreePath)

	// Check if context directory exists
	if _, err := os.Stat(contextDir); os.IsNotExist(err) {
		return []string{}, nil
	}

	var files []string
	err := filepath.Walk(contextDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			relPath, _ := filepath.Rel(contextDir, path)
			files = append(files, relPath)
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walk context directory: %w", err)
	}

	return files, nil
}
