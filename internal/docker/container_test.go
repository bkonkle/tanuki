package docker

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bkonkle/tanuki/internal/config"
)

// skipIfDockerNotRunning skips the test if Docker is not running.
func skipIfDockerNotRunning(t *testing.T) {
	t.Helper()
	if err := checkDockerRunning(); err != nil {
		t.Skip("Docker is not running")
	}
}

// createTestImage ensures a test image is available.
// Uses node:22-alpine which has the 'node' user that the code expects.
func createTestImage(t *testing.T) string {
	t.Helper()
	skipIfDockerNotRunning(t)

	// Use node:22-alpine which has the 'node' user that the exec functions require
	imageName := "node:22-alpine"

	// Check if image exists, if not pull it
	cmd := exec.Command("docker", "image", "inspect", imageName)
	if err := cmd.Run(); err != nil {
		t.Logf("Pulling test image %s...", imageName)
		cmd = exec.Command("docker", "pull", imageName)
		if err := cmd.Run(); err != nil {
			t.Fatalf("Failed to pull test image: %v", err)
		}
	}

	return imageName
}

// cleanupContainer removes a test container if it exists.
func cleanupContainer(t *testing.T, containerID string) {
	t.Helper()
	if containerID == "" {
		return
	}
	cmd := exec.Command("docker", "rm", "-f", containerID)
	_ = cmd.Run() // Ignore errors - container may already be removed
}

// createTestManager creates a Manager for testing.
func createTestManager(t *testing.T) *Manager {
	t.Helper()
	skipIfDockerNotRunning(t)

	cfg := config.DefaultConfig()
	manager, err := NewManager(cfg)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	return manager
}

func TestNewManager(t *testing.T) {
	skipIfDockerNotRunning(t)

	cfg := config.DefaultConfig()
	manager, err := NewManager(cfg)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	if manager == nil {
		t.Fatal("NewManager returned nil")
	}

	if manager.config != cfg {
		t.Error("Manager config not set correctly")
	}
}

func TestCreateContainer(t *testing.T) {
	manager := createTestManager(t)
	imageName := createTestImage(t)

	config := ContainerConfig{
		Name:    "tanuki-test-create",
		Image:   imageName,
		WorkDir: "/test",
		Env: map[string]string{
			"TEST_VAR": "test_value",
		},
		Resources: ResourceLimits{
			Memory: "128m",
			CPUs:   "0.5",
		},
	}

	containerID, err := manager.CreateContainer(config)
	if err != nil {
		t.Fatalf("CreateContainer failed: %v", err)
	}
	defer cleanupContainer(t, containerID)

	if containerID == "" {
		t.Error("CreateContainer returned empty container ID")
	}

	// Verify container exists
	if !manager.ContainerExists(containerID) {
		t.Error("Container does not exist after creation")
	}
}

func TestCreateContainer_WithMounts(t *testing.T) {
	manager := createTestManager(t)
	imageName := createTestImage(t)

	// Create a temporary directory to mount
	tmpDir, err := os.MkdirTemp("", "tanuki-test-mount-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	config := ContainerConfig{
		Name:    "tanuki-test-mount",
		Image:   imageName,
		WorkDir: "/workspace",
		Mounts: []Mount{
			{
				Source:   tmpDir,
				Target:   "/workspace",
				ReadOnly: false,
			},
		},
	}

	containerID, err := manager.CreateContainer(config)
	if err != nil {
		t.Fatalf("CreateContainer with mounts failed: %v", err)
	}
	defer cleanupContainer(t, containerID)

	if containerID == "" {
		t.Error("CreateContainer returned empty container ID")
	}
}

func TestCreateContainer_ImageNotFound(t *testing.T) {
	manager := createTestManager(t)

	config := ContainerConfig{
		Name:  "tanuki-test-noimage",
		Image: "nonexistent-image:latest",
	}

	_, err := manager.CreateContainer(config)
	if err == nil {
		t.Error("CreateContainer should fail with nonexistent image")
	}

	// The error should mention either "image not found" or "Unable to find image"
	errStr := err.Error()
	if !strings.Contains(errStr, "image not found") && !strings.Contains(errStr, "Unable to find image") {
		t.Errorf("Expected image not found error, got: %v", err)
	}
}

func TestStartStopContainer(t *testing.T) {
	manager := createTestManager(t)
	imageName := createTestImage(t)

	config := ContainerConfig{
		Name:  "tanuki-test-startstop",
		Image: imageName,
	}

	containerID, err := manager.CreateContainer(config)
	if err != nil {
		t.Fatalf("CreateContainer failed: %v", err)
	}
	defer cleanupContainer(t, containerID)

	// Start the container
	if err := manager.StartContainer(containerID); err != nil {
		t.Fatalf("StartContainer failed: %v", err)
	}

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	// Check if running
	if !manager.ContainerRunning(containerID) {
		t.Error("Container should be running after start")
	}

	// Stop the container
	if err := manager.StopContainer(containerID); err != nil {
		t.Fatalf("StopContainer failed: %v", err)
	}

	// Give it a moment to stop
	time.Sleep(100 * time.Millisecond)

	// Check if stopped
	if manager.ContainerRunning(containerID) {
		t.Error("Container should not be running after stop")
	}
}

func TestRemoveContainer(t *testing.T) {
	manager := createTestManager(t)
	imageName := createTestImage(t)

	config := ContainerConfig{
		Name:  "tanuki-test-remove",
		Image: imageName,
	}

	containerID, err := manager.CreateContainer(config)
	if err != nil {
		t.Fatalf("CreateContainer failed: %v", err)
	}

	// Start the container first
	if err := manager.StartContainer(containerID); err != nil {
		t.Fatalf("StartContainer failed: %v", err)
	}

	// Remove the container (should force remove even though running)
	if err := manager.RemoveContainer(containerID); err != nil {
		t.Fatalf("RemoveContainer failed: %v", err)
	}

	// Verify it's gone
	if manager.ContainerExists(containerID) {
		t.Error("Container should not exist after removal")
	}
}

func TestExecWithOutput(t *testing.T) {
	manager := createTestManager(t)
	imageName := createTestImage(t)

	config := ContainerConfig{
		Name:  "tanuki-test-exec",
		Image: imageName,
	}

	containerID, err := manager.CreateContainer(config)
	if err != nil {
		t.Fatalf("CreateContainer failed: %v", err)
	}
	defer cleanupContainer(t, containerID)

	// Start the container
	if startErr := manager.StartContainer(containerID); startErr != nil {
		t.Fatalf("StartContainer failed: %v", startErr)
	}

	// Execute a command
	output, err := manager.ExecWithOutput(containerID, []string{"echo", "hello"})
	if err != nil {
		t.Fatalf("ExecWithOutput failed: %v", err)
	}

	if !strings.Contains(output, "hello") {
		t.Errorf("Expected output to contain 'hello', got: %s", output)
	}
}

func TestExec(t *testing.T) {
	manager := createTestManager(t)
	imageName := createTestImage(t)

	config := ContainerConfig{
		Name:  "tanuki-test-exec-stream",
		Image: imageName,
	}

	containerID, err := manager.CreateContainer(config)
	if err != nil {
		t.Fatalf("CreateContainer failed: %v", err)
	}
	defer cleanupContainer(t, containerID)

	// Start the container
	if err := manager.StartContainer(containerID); err != nil {
		t.Fatalf("StartContainer failed: %v", err)
	}

	// Execute with streaming I/O
	var stdout bytes.Buffer
	opts := ExecOptions{
		Stdout: &stdout,
	}

	if err := manager.Exec(containerID, []string{"echo", "test"}, opts); err != nil {
		t.Fatalf("Exec failed: %v", err)
	}

	if !strings.Contains(stdout.String(), "test") {
		t.Errorf("Expected stdout to contain 'test', got: %s", stdout.String())
	}
}

func TestStreamLogs(t *testing.T) {
	manager := createTestManager(t)
	imageName := createTestImage(t)

	config := ContainerConfig{
		Name:  "tanuki-test-logs",
		Image: imageName,
	}

	containerID, err := manager.CreateContainer(config)
	if err != nil {
		t.Fatalf("CreateContainer failed: %v", err)
	}
	defer cleanupContainer(t, containerID)

	// Start the container
	if startErr := manager.StartContainer(containerID); startErr != nil {
		t.Fatalf("StartContainer failed: %v", startErr)
	}

	// Execute a command to generate logs
	_, err = manager.ExecWithOutput(containerID, []string{"echo", "log test"})
	if err != nil {
		t.Fatalf("Failed to generate logs: %v", err)
	}

	// Stream logs (non-following)
	reader, err := manager.StreamLogs(containerID, false)
	if err != nil {
		t.Fatalf("StreamLogs failed: %v", err)
	}
	defer func() { _ = reader.Close() }()

	// Read some logs
	buf := make([]byte, 1024)
	_, err = reader.Read(buf)
	if err != nil && err != io.EOF {
		t.Fatalf("Failed to read logs: %v", err)
	}
}

func TestContainerExists(t *testing.T) {
	manager := createTestManager(t)
	imageName := createTestImage(t)

	// Should not exist
	if manager.ContainerExists("nonexistent-container") {
		t.Error("ContainerExists should return false for nonexistent container")
	}

	// Create a container
	config := ContainerConfig{
		Name:  "tanuki-test-exists",
		Image: imageName,
	}

	containerID, err := manager.CreateContainer(config)
	if err != nil {
		t.Fatalf("CreateContainer failed: %v", err)
	}
	defer cleanupContainer(t, containerID)

	// Should exist
	if !manager.ContainerExists(containerID) {
		t.Error("ContainerExists should return true for existing container")
	}
}

func TestContainerRunning(t *testing.T) {
	manager := createTestManager(t)
	imageName := createTestImage(t)

	config := ContainerConfig{
		Name:  "tanuki-test-running",
		Image: imageName,
	}

	containerID, err := manager.CreateContainer(config)
	if err != nil {
		t.Fatalf("CreateContainer failed: %v", err)
	}
	defer cleanupContainer(t, containerID)

	// Should not be running initially
	if manager.ContainerRunning(containerID) {
		t.Error("Container should not be running initially")
	}

	// Start it
	if err := manager.StartContainer(containerID); err != nil {
		t.Fatalf("StartContainer failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Should be running now
	if !manager.ContainerRunning(containerID) {
		t.Error("Container should be running after start")
	}
}

func TestInspectContainer(t *testing.T) {
	manager := createTestManager(t)
	imageName := createTestImage(t)

	config := ContainerConfig{
		Name:  "tanuki-test-inspect",
		Image: imageName,
	}

	containerID, err := manager.CreateContainer(config)
	if err != nil {
		t.Fatalf("CreateContainer failed: %v", err)
	}
	defer cleanupContainer(t, containerID)

	info, err := manager.InspectContainer(containerID)
	if err != nil {
		t.Fatalf("InspectContainer failed: %v", err)
	}

	if info.Name != "tanuki-test-inspect" {
		t.Errorf("Expected name 'tanuki-test-inspect', got: %s", info.Name)
	}

	if info.Image != imageName {
		t.Errorf("Expected image '%s', got: %s", imageName, info.Image)
	}

	if info.State != "created" {
		t.Errorf("Expected state 'created', got: %s", info.State)
	}
}

func TestInspectContainer_NotFound(t *testing.T) {
	manager := createTestManager(t)

	_, err := manager.InspectContainer("nonexistent-container")
	if err == nil {
		t.Error("InspectContainer should fail for nonexistent container")
	}

	if err != ErrContainerNotFound {
		t.Errorf("Expected ErrContainerNotFound, got: %v", err)
	}
}

func TestEnsureNetwork(t *testing.T) {
	manager := createTestManager(t)

	networkName := "tanuki-test-network"

	// Clean up any existing test network
	_ = exec.Command("docker", "network", "rm", networkName).Run()

	// Create the network
	if err := manager.EnsureNetwork(networkName); err != nil {
		t.Fatalf("EnsureNetwork failed: %v", err)
	}
	defer func() { _ = exec.Command("docker", "network", "rm", networkName).Run() }()

	// Verify it exists
	exists, err := NetworkExists(networkName)
	if err != nil {
		t.Fatalf("NetworkExists failed: %v", err)
	}
	if !exists {
		t.Error("Network should exist after EnsureNetwork")
	}

	// Call again - should not fail
	if err := manager.EnsureNetwork(networkName); err != nil {
		t.Fatalf("EnsureNetwork should not fail when network exists: %v", err)
	}
}

func TestImageExists(t *testing.T) {
	manager := createTestManager(t)
	imageName := createTestImage(t)

	// Should exist (we just pulled it)
	if !manager.ImageExists(imageName) {
		t.Error("ImageExists should return true for existing image")
	}

	// Should not exist
	if manager.ImageExists("nonexistent-image:latest") {
		t.Error("ImageExists should return false for nonexistent image")
	}
}

func TestCreateAgentContainer(t *testing.T) {
	manager := createTestManager(t)
	_ = createTestImage(t) // Ensure alpine image is available

	// Override config to use test image
	manager.config.Image.Name = "alpine"
	manager.config.Image.Tag = "latest"

	// Create a temporary worktree directory
	tmpDir, err := os.MkdirTemp("", "tanuki-test-worktree-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Ensure the network exists
	if netErr := manager.EnsureNetwork(manager.config.Network.Name); netErr != nil {
		t.Fatalf("EnsureNetwork failed: %v", netErr)
	}
	defer func() { _ = exec.Command("docker", "network", "rm", manager.config.Network.Name).Run() }() //nolint:gosec // G204: test cleanup with trusted config value

	containerID, err := manager.CreateAgentContainer("test-agent", tmpDir)
	if err != nil {
		t.Fatalf("CreateAgentContainer failed: %v", err)
	}
	defer cleanupContainer(t, containerID)

	if containerID == "" {
		t.Error("CreateAgentContainer returned empty container ID")
	}

	// Verify container exists and has correct name
	info, err := manager.InspectContainer(containerID)
	if err != nil {
		t.Fatalf("InspectContainer failed: %v", err)
	}

	if info.Name != "tanuki-test-agent" {
		t.Errorf("Expected name 'tanuki-test-agent', got: %s", info.Name)
	}
}

func TestCreateAgentContainer_WorktreeNotFound(t *testing.T) {
	manager := createTestManager(t)

	// Use a non-existent path
	nonexistentPath := filepath.Join(os.TempDir(), "nonexistent-worktree-12345")

	_, err := manager.CreateAgentContainer("test-agent", nonexistentPath)
	if err == nil {
		t.Error("CreateAgentContainer should fail when worktree doesn't exist")
	}

	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("Expected 'does not exist' error, got: %v", err)
	}
}
