package docker

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// EnsureNetwork creates a Docker network if it doesn't already exist.
func EnsureNetwork(name string) error {
	// Check if network exists
	cmd := exec.Command("docker", "network", "ls", "--format", "{{.Name}}")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to list networks: %w", err)
	}

	networks := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, n := range networks {
		if n == name {
			return nil // Network already exists
		}
	}

	// Create network
	cmd = exec.Command("docker", "network", "create", name)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create network %s: %s", name, stderr.String())
	}

	return nil
}

// NetworkExists checks if a Docker network exists.
func NetworkExists(name string) (bool, error) {
	cmd := exec.Command("docker", "network", "inspect", name)
	err := cmd.Run()
	if err != nil {
		// Network doesn't exist
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
