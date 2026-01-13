package git

import (
	"os/exec"
)

// IsGitRepo checks if the current directory is within a Git repository.
func IsGitRepo() bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	err := cmd.Run()
	return err == nil
}

// GetRepoRoot returns the root directory of the Git repository.
func GetRepoRoot() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	// Trim trailing newline
	root := string(output)
	if len(root) > 0 && root[len(root)-1] == '\n' {
		root = root[:len(root)-1]
	}
	return root, nil
}
