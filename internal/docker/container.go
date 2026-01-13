// Package docker provides Docker container operations for agent execution.
package docker

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bkonkle/tanuki/internal/config"
)

// ErrDockerNotRunning indicates the Docker daemon is not running.
var ErrDockerNotRunning = errors.New("docker daemon not running")

// ErrContainerNotFound indicates the container does not exist.
var ErrContainerNotFound = errors.New("container not found")

// ErrImageNotFound indicates the Docker image was not found.
var ErrImageNotFound = errors.New("image not found")

// ContainerConfig represents configuration for creating a container.
type ContainerConfig struct {
	Name         string
	Image        string
	WorktreePath string
	WorkDir      string
	Env          map[string]string
	Mounts       []Mount
	Network      string
	Resources    ResourceLimits
}

// Mount represents a volume mount in a container.
type Mount struct {
	Source   string
	Target   string
	ReadOnly bool
}

// ResourceLimits represents container resource constraints.
type ResourceLimits struct {
	Memory string // e.g., "4g"
	CPUs   string // e.g., "2"
}

// ExecOptions configures command execution in a container.
type ExecOptions struct {
	Stdin       io.Reader
	Stdout      io.Writer
	Stderr      io.Writer
	TTY         bool
	Interactive bool
}

// ContainerInfo holds information about a container.
type ContainerInfo struct {
	ID      string
	Name    string
	State   string
	Status  string
	Image   string
	Created string
}

// Manager handles Docker container operations.
type Manager struct {
	config *config.Config
}

// NewManager creates a new Docker container manager.
func NewManager(cfg *config.Config) (*Manager, error) {
	// Verify Docker is running
	if err := checkDockerRunning(); err != nil {
		return nil, err
	}

	return &Manager{
		config: cfg,
	}, nil
}

// checkDockerRunning verifies the Docker daemon is accessible.
func checkDockerRunning() error {
	cmd := exec.Command("docker", "info")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if strings.Contains(stderr.String(), "Cannot connect") {
			return ErrDockerNotRunning
		}
		return fmt.Errorf("docker check failed: %s", stderr.String())
	}
	return nil
}

// EnsureNetwork creates a Docker network if it doesn't already exist.
func (m *Manager) EnsureNetwork(name string) error {
	return EnsureNetwork(name)
}

// CreateContainer creates a new container with the given configuration.
func (m *Manager) CreateContainer(config ContainerConfig) (string, error) {
	args := []string{
		"create",
		"--name", config.Name,
		"--workdir", config.WorkDir,
	}

	// Add network if specified
	if config.Network != "" {
		args = append(args, "--network", config.Network)
	}

	// Add mounts
	for _, mount := range config.Mounts {
		mountStr := fmt.Sprintf("%s:%s", mount.Source, mount.Target)
		if mount.ReadOnly {
			mountStr += ":ro"
		}
		args = append(args, "-v", mountStr)
	}

	// Add resource limits
	if config.Resources.Memory != "" {
		args = append(args, "--memory", config.Resources.Memory)
	}
	if config.Resources.CPUs != "" {
		args = append(args, "--cpus", config.Resources.CPUs)
	}

	// Add environment variables
	for k, v := range config.Env {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}

	// Image and command - run sleep infinity to keep container alive
	args = append(args, config.Image, "sleep", "infinity")

	cmd := exec.Command("docker", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		errMsg := stderr.String()
		if strings.Contains(errMsg, "No such image") {
			return "", fmt.Errorf("%w: %s", ErrImageNotFound, config.Image)
		}
		return "", fmt.Errorf("failed to create container: %s", errMsg)
	}

	containerID := strings.TrimSpace(string(output))
	return containerID, nil
}

// CreateAgentContainer creates a container configured for a Tanuki agent.
func (m *Manager) CreateAgentContainer(name string, worktreePath string) (string, error) {
	absWorktree, err := filepath.Abs(worktreePath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Verify worktree exists
	if _, err := os.Stat(absWorktree); os.IsNotExist(err) {
		return "", fmt.Errorf("worktree does not exist: %s", absWorktree)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	// Build image reference
	image := m.config.Image.Name + ":" + m.config.Image.Tag

	config := ContainerConfig{
		Name:    fmt.Sprintf("tanuki-%s", name),
		Image:   image,
		WorkDir: "/workspace",
		Mounts: []Mount{
			{
				Source:   absWorktree,
				Target:   "/workspace",
				ReadOnly: false,
			},
			{
				Source:   filepath.Join(homeDir, ".config", "claude-code"),
				Target:   "/home/dev/.config/claude-code",
				ReadOnly: true,
			},
		},
		Network: m.config.Network.Name,
		Resources: ResourceLimits{
			Memory: m.config.Defaults.Resources.Memory,
			CPUs:   m.config.Defaults.Resources.CPUs,
		},
		Env: map[string]string{
			"TANUKI_AGENT": name,
		},
	}

	return m.CreateContainer(config)
}

// StartContainer starts a stopped container.
func (m *Manager) StartContainer(containerID string) error {
	cmd := exec.Command("docker", "start", containerID)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start container: %s", stderr.String())
	}
	return nil
}

// StopContainer stops a running container.
func (m *Manager) StopContainer(containerID string) error {
	cmd := exec.Command("docker", "stop", containerID)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to stop container: %s", stderr.String())
	}
	return nil
}

// RemoveContainer removes a container (stopped or running with force).
func (m *Manager) RemoveContainer(containerID string) error {
	cmd := exec.Command("docker", "rm", "-f", containerID)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to remove container: %s", stderr.String())
	}
	return nil
}

// Exec executes a command in a running container with streaming I/O.
func (m *Manager) Exec(containerID string, command []string, opts ExecOptions) error {
	args := []string{"exec"}

	if opts.Interactive {
		args = append(args, "-i")
	}
	if opts.TTY {
		args = append(args, "-t")
	}

	args = append(args, containerID)
	args = append(args, command...)

	cmd := exec.Command("docker", args...)

	if opts.Stdin != nil {
		cmd.Stdin = opts.Stdin
	}
	if opts.Stdout != nil {
		cmd.Stdout = opts.Stdout
	}
	if opts.Stderr != nil {
		cmd.Stderr = opts.Stderr
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("exec failed: %w", err)
	}

	return nil
}

// ExecWithOutput executes a command in a container and returns the output.
func (m *Manager) ExecWithOutput(containerID string, command []string) (string, error) {
	args := []string{"exec", containerID}
	args = append(args, command...)

	cmd := exec.Command("docker", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("exec failed: %s", stderr.String())
	}

	return stdout.String(), nil
}

// StreamLogs returns a reader for streaming container logs.
func (m *Manager) StreamLogs(containerID string, follow bool) (io.ReadCloser, error) {
	args := []string{"logs"}
	if follow {
		args = append(args, "-f")
	}
	args = append(args, containerID)

	cmd := exec.Command("docker", args...)

	// Combine stdout and stderr for logs
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start logs command: %w", err)
	}

	// Create a combined reader
	reader := io.MultiReader(stdout, stderr)

	// Return a ReadCloser that also ensures the command is cleaned up
	return &logReader{
		reader: reader,
		cmd:    cmd,
		stdout: stdout,
		stderr: stderr,
	}, nil
}

// logReader wraps the log stream and command process.
type logReader struct {
	reader io.Reader
	cmd    *exec.Cmd
	stdout io.ReadCloser
	stderr io.ReadCloser
}

func (lr *logReader) Read(p []byte) (n int, err error) {
	return lr.reader.Read(p)
}

func (lr *logReader) Close() error {
	lr.stdout.Close()
	lr.stderr.Close()
	// Kill the process if it's still running
	if lr.cmd.Process != nil {
		lr.cmd.Process.Kill()
	}
	return nil
}

// ContainerExists checks if a container exists.
func (m *Manager) ContainerExists(containerID string) bool {
	cmd := exec.Command("docker", "inspect", "--type", "container", containerID)
	return cmd.Run() == nil
}

// ContainerRunning checks if a container is currently running.
func (m *Manager) ContainerRunning(containerID string) bool {
	cmd := exec.Command("docker", "inspect", "--format", "{{.State.Running}}", containerID)
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(output)) == "true"
}

// ContainerStatus checks if a container exists and is running.
// This implements the state.ContainerChecker interface.
func (m *Manager) ContainerStatus(containerID string) (exists bool, running bool, err error) {
	cmd := exec.Command("docker", "inspect", "--format", "{{.State.Running}}", containerID)
	output, cmdErr := cmd.Output()
	if cmdErr != nil {
		// Check if container doesn't exist
		if strings.Contains(cmdErr.Error(), "No such") || strings.Contains(cmdErr.Error(), "no such") {
			return false, false, nil
		}
		return false, false, fmt.Errorf("failed to inspect container: %w", cmdErr)
	}

	isRunning := strings.TrimSpace(string(output)) == "true"
	return true, isRunning, nil
}

// InspectContainer returns detailed information about a container.
func (m *Manager) InspectContainer(containerID string) (*ContainerInfo, error) {
	format := "{{.Id}}|{{.Name}}|{{.State.Status}}|{{.Config.Image}}|{{.Created}}"
	cmd := exec.Command("docker", "inspect", "--format", format, containerID)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		if strings.Contains(stderr.String(), "No such") || strings.Contains(stderr.String(), "no such") {
			return nil, ErrContainerNotFound
		}
		return nil, fmt.Errorf("failed to inspect container: %s", stderr.String())
	}

	parts := strings.Split(strings.TrimSpace(string(output)), "|")
	if len(parts) < 5 {
		return nil, fmt.Errorf("unexpected inspect output format")
	}

	return &ContainerInfo{
		ID:      parts[0],
		Name:    strings.TrimPrefix(parts[1], "/"), // Docker prefixes names with /
		State:   parts[2],
		Status:  parts[2], // Use State.Status for both fields
		Image:   parts[3],
		Created: parts[4],
	}, nil
}

// ImageExists checks if a Docker image exists locally.
func (m *Manager) ImageExists(imageName string) bool {
	cmd := exec.Command("docker", "image", "inspect", imageName)
	return cmd.Run() == nil
}

// PullImage pulls a Docker image from a registry.
func (m *Manager) PullImage(imageName string) error {
	cmd := exec.Command("docker", "pull", imageName)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to pull image: %s", stderr.String())
	}
	return nil
}

// ResourceUsage contains container resource usage statistics.
type ResourceUsage struct {
	Memory string
	CPU    string
}

// GetResourceUsage retrieves current resource usage statistics for a container.
// Returns nil if the container is not running or stats are unavailable.
func (m *Manager) GetResourceUsage(containerID string) (*ResourceUsage, error) {
	// Check if container is running first
	if !m.ContainerRunning(containerID) {
		return nil, nil
	}

	cmd := exec.Command("docker", "stats", "--no-stream", "--format",
		"{{.MemUsage}}\t{{.CPUPerc}}", containerID)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		// If stats fail, return nil (not an error - stats might not be available)
		return nil, nil
	}

	parts := strings.Split(strings.TrimSpace(string(output)), "\t")
	if len(parts) < 2 {
		return nil, nil
	}

	return &ResourceUsage{
		Memory: parts[0],
		CPU:    parts[1],
	}, nil
}
