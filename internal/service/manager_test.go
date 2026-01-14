package service

import (
	"os/exec"
	"strings"
	"testing"
	"time"
)

// skipIfDockerNotRunning skips the test if Docker is not running.
func skipIfDockerNotRunning(t *testing.T) {
	t.Helper()
	cmd := exec.Command("docker", "info")
	if err := cmd.Run(); err != nil {
		t.Skip("Docker is not running")
	}
}

// mockDockerManager implements DockerManager for unit tests without Docker.
type mockDockerManager struct {
	networkEnsured   bool
	containers       map[string]bool // containerName -> running
	ensureNetworkErr error
	containerRunning map[string]bool
}

func newMockDockerManager() *mockDockerManager {
	return &mockDockerManager{
		containers:       make(map[string]bool),
		containerRunning: make(map[string]bool),
	}
}

func (m *mockDockerManager) EnsureNetwork(_ string) error {
	if m.ensureNetworkErr != nil {
		return m.ensureNetworkErr
	}
	m.networkEnsured = true
	return nil
}

func (m *mockDockerManager) ContainerExists(containerID string) bool {
	_, exists := m.containers[containerID]
	return exists
}

func (m *mockDockerManager) ContainerRunning(containerID string) bool {
	if running, ok := m.containerRunning[containerID]; ok {
		return running
	}
	return false
}

// cleanupService stops and removes a service container if it exists.
func cleanupService(t *testing.T, name string) {
	t.Helper()
	containerName := ContainerName(name)
	_ = exec.Command("docker", "stop", containerName).Run()     //nolint:gosec // containerName is constructed from trusted test data
	_ = exec.Command("docker", "rm", "-f", containerName).Run() //nolint:gosec // containerName is constructed from trusted test data
}

func TestNewManager(t *testing.T) {
	services := map[string]*Config{
		"postgres": DefaultPostgresConfig(),
		"redis":    DefaultRedisConfig(),
	}

	mgr, err := NewManager(services, "test-net", newMockDockerManager())
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	if mgr == nil {
		t.Fatal("NewManager returned nil")
	}

	// Check status initialized
	if len(mgr.status) != 2 {
		t.Errorf("Expected 2 services in status, got %d", len(mgr.status))
	}
}

func TestNewManager_DefaultNetwork(t *testing.T) {
	mgr, err := NewManager(nil, "", newMockDockerManager())
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	if mgr.networkName != "tanuki-net" {
		t.Errorf("Expected default network 'tanuki-net', got %s", mgr.networkName)
	}
}

func TestContainerName(t *testing.T) {
	tests := []struct {
		name     string
		expected string
	}{
		{"postgres", "tanuki-svc-postgres"},
		{"redis", "tanuki-svc-redis"},
		{"my-service", "tanuki-svc-my-service"},
	}

	for _, tt := range tests {
		got := ContainerName(tt.name)
		if got != tt.expected {
			t.Errorf("ContainerName(%s) = %s, want %s", tt.name, got, tt.expected)
		}
	}
}

func TestDefaultPostgresConfig(t *testing.T) {
	cfg := DefaultPostgresConfig()

	if cfg.Enabled {
		t.Error("Default postgres should be disabled")
	}
	if cfg.Image != "postgres:16" {
		t.Errorf("Expected postgres:16, got %s", cfg.Image)
	}
	if cfg.Port != 5432 {
		t.Errorf("Expected port 5432, got %d", cfg.Port)
	}
	if cfg.Environment["POSTGRES_USER"] != "tanuki" {
		t.Error("Expected POSTGRES_USER=tanuki")
	}
	if cfg.Environment["POSTGRES_PASSWORD"] != "tanuki" {
		t.Error("Expected POSTGRES_PASSWORD=tanuki")
	}
	if cfg.Healthcheck == nil {
		t.Fatal("Expected healthcheck config")
	}
	if len(cfg.Healthcheck.Command) == 0 {
		t.Error("Expected healthcheck command")
	}
}

func TestDefaultRedisConfig(t *testing.T) {
	cfg := DefaultRedisConfig()

	if cfg.Enabled {
		t.Error("Default redis should be disabled")
	}
	if cfg.Image != "redis:7" {
		t.Errorf("Expected redis:7, got %s", cfg.Image)
	}
	if cfg.Port != 6379 {
		t.Errorf("Expected port 6379, got %d", cfg.Port)
	}
	if cfg.Healthcheck == nil {
		t.Fatal("Expected healthcheck config")
	}
}

func TestGetStatus_NotFound(t *testing.T) {
	mgr, err := NewManager(nil, "", newMockDockerManager())
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	_, err = mgr.GetStatus("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent service")
	}
	if err.Error() != "service not found: nonexistent" {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestGetConnectionInfo(t *testing.T) {
	cfg := DefaultPostgresConfig()
	cfg.Enabled = true
	services := map[string]*Config{"postgres": cfg}

	mgr, err := NewManager(services, "test-net", newMockDockerManager())
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	conn, err := mgr.GetConnectionInfo("postgres")
	if err != nil {
		t.Fatalf("GetConnectionInfo failed: %v", err)
	}

	if conn.Host != "tanuki-svc-postgres" {
		t.Errorf("Expected host tanuki-svc-postgres, got %s", conn.Host)
	}
	if conn.Port != 5432 {
		t.Errorf("Expected port 5432, got %d", conn.Port)
	}
	if conn.URL != "tanuki-svc-postgres:5432" {
		t.Errorf("Expected URL tanuki-svc-postgres:5432, got %s", conn.URL)
	}
	if conn.Username != "tanuki" {
		t.Errorf("Expected username tanuki, got %s", conn.Username)
	}
	if conn.Password != "tanuki" {
		t.Errorf("Expected password tanuki, got %s", conn.Password)
	}
	if conn.Database != "tanuki_dev" {
		t.Errorf("Expected database tanuki_dev, got %s", conn.Database)
	}
}

func TestGetConnectionInfo_NotFound(t *testing.T) {
	mgr, err := NewManager(nil, "", newMockDockerManager())
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	_, err = mgr.GetConnectionInfo("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent service")
	}
}

func TestGetConnectionInfo_NotEnabled(t *testing.T) {
	cfg := DefaultPostgresConfig()
	cfg.Enabled = false
	services := map[string]*Config{"postgres": cfg}

	mgr, err := NewManager(services, "test-net", newMockDockerManager())
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	_, err = mgr.GetConnectionInfo("postgres")
	if err == nil {
		t.Error("Expected error for disabled service")
	}
}

func TestGetAllConnections(t *testing.T) {
	pgCfg := DefaultPostgresConfig()
	pgCfg.Enabled = true
	redisCfg := DefaultRedisConfig()
	redisCfg.Enabled = true
	disabledCfg := &Config{Enabled: false, Image: "foo", Port: 1234}

	services := map[string]*Config{
		"postgres": pgCfg,
		"redis":    redisCfg,
		"disabled": disabledCfg,
	}

	mgr, err := NewManager(services, "test-net", newMockDockerManager())
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	conns := mgr.GetAllConnections()

	if len(conns) != 2 {
		t.Errorf("Expected 2 connections, got %d", len(conns))
	}

	if _, ok := conns["postgres"]; !ok {
		t.Error("Missing postgres connection")
	}
	if _, ok := conns["redis"]; !ok {
		t.Error("Missing redis connection")
	}
	if _, ok := conns["disabled"]; ok {
		t.Error("Should not include disabled service")
	}
}

func TestGetAllStatus(t *testing.T) {
	pgCfg := DefaultPostgresConfig()
	redisCfg := DefaultRedisConfig()

	services := map[string]*Config{
		"postgres": pgCfg,
		"redis":    redisCfg,
	}

	mgr, err := NewManager(services, "test-net", newMockDockerManager())
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	statuses := mgr.GetAllStatus()

	if len(statuses) != 2 {
		t.Errorf("Expected 2 statuses, got %d", len(statuses))
	}
}

func TestIsHealthy_NotFound(t *testing.T) {
	mgr, err := NewManager(nil, "", newMockDockerManager())
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	if mgr.IsHealthy("nonexistent") {
		t.Error("IsHealthy should return false for nonexistent service")
	}
}

func TestStartService_NotFound(t *testing.T) {
	mgr, err := NewManager(nil, "", newMockDockerManager())
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	err = mgr.StartService("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent service")
	}
}

func TestStopService_NotFound(t *testing.T) {
	mgr, err := NewManager(nil, "", newMockDockerManager())
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	err = mgr.StopService("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent service")
	}
}

// Integration tests (require Docker)
// These tests use redis:7 since it has a persistent process and health check.
// Alternatively, tests could use alpine with a custom command.

func TestStartService_Integration(t *testing.T) {
	skipIfDockerNotRunning(t)

	// Use redis - it starts quickly and stays running
	cfg := DefaultRedisConfig()
	cfg.Enabled = true
	services := map[string]*Config{"redis": cfg}

	// Ensure image is available
	_ = exec.Command("docker", "pull", "redis:7").Run()

	mgr, err := NewManager(services, "tanuki-test-net", nil)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Ensure network exists
	_ = exec.Command("docker", "network", "create", "tanuki-test-net").Run()
	defer func() { _ = exec.Command("docker", "network", "rm", "tanuki-test-net").Run() }()

	// Clean up before and after
	cleanupService(t, "redis")
	defer cleanupService(t, "redis")

	err = mgr.StartService("redis")
	if err != nil {
		t.Fatalf("StartService failed: %v", err)
	}

	// Wait a bit for service to start and health check
	time.Sleep(3 * time.Second)

	// Verify it's running
	status, err := mgr.GetStatus("redis")
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}

	if !status.Running {
		t.Error("Service should be running")
	}
}

func TestStopService_Integration(t *testing.T) {
	skipIfDockerNotRunning(t)

	cfg := DefaultRedisConfig()
	cfg.Enabled = true
	services := map[string]*Config{"redis": cfg}

	_ = exec.Command("docker", "pull", "redis:7").Run()

	mgr, err := NewManager(services, "tanuki-test-net", nil)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	_ = exec.Command("docker", "network", "create", "tanuki-test-net").Run()
	defer func() { _ = exec.Command("docker", "network", "rm", "tanuki-test-net").Run() }()

	cleanupService(t, "redis")
	defer cleanupService(t, "redis")

	// Start first
	err = mgr.StartService("redis")
	if err != nil {
		t.Fatalf("StartService failed: %v", err)
	}

	// Wait for it to start
	time.Sleep(3 * time.Second)

	// Stop it
	err = mgr.StopService("redis")
	if err != nil {
		t.Fatalf("StopService failed: %v", err)
	}

	// Verify it's stopped
	status, err := mgr.GetStatus("redis")
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}

	if status.Running {
		t.Error("Service should not be running")
	}
}

func TestStartStopServices_Integration(t *testing.T) {
	skipIfDockerNotRunning(t)

	// Use redis for multiple service tests
	cfg1 := DefaultRedisConfig()
	cfg1.Enabled = true
	cfg1.Port = 6379

	cfg2 := DefaultRedisConfig()
	cfg2.Enabled = true
	cfg2.Port = 6380 // Different port

	cfg3 := DefaultRedisConfig()
	cfg3.Enabled = false // Disabled

	services := map[string]*Config{
		"test1":    cfg1,
		"test2":    cfg2,
		"disabled": cfg3,
	}

	_ = exec.Command("docker", "pull", "redis:7").Run()

	mgr, err := NewManager(services, "tanuki-test-net", nil)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	_ = exec.Command("docker", "network", "create", "tanuki-test-net").Run()
	defer func() { _ = exec.Command("docker", "network", "rm", "tanuki-test-net").Run() }()

	cleanupService(t, "test1")
	cleanupService(t, "test2")
	cleanupService(t, "disabled")
	defer cleanupService(t, "test1")
	defer cleanupService(t, "test2")
	defer cleanupService(t, "disabled")

	// Start all
	err = mgr.StartServices()
	if err != nil {
		t.Fatalf("StartServices failed: %v", err)
	}

	time.Sleep(5 * time.Second)

	// Check statuses
	statuses := mgr.GetAllStatus()

	// test1 and test2 should be running
	if status := statuses["test1"]; status == nil || !status.Running {
		t.Error("test1 should be running")
	}
	if status := statuses["test2"]; status == nil || !status.Running {
		t.Error("test2 should be running")
	}
	// disabled should not be running
	if status := statuses["disabled"]; status != nil && status.Running {
		t.Error("disabled should not be running")
	}

	// Stop all
	err = mgr.StopServices()
	if err != nil {
		t.Fatalf("StopServices failed: %v", err)
	}

	// Check all stopped
	statuses = mgr.GetAllStatus()
	for name, status := range statuses {
		if status.Running {
			t.Errorf("%s should not be running after StopServices", name)
		}
	}
}

func TestStartService_AlreadyRunning_Integration(t *testing.T) {
	skipIfDockerNotRunning(t)

	cfg := DefaultRedisConfig()
	cfg.Enabled = true
	services := map[string]*Config{"redis": cfg}

	_ = exec.Command("docker", "pull", "redis:7").Run()

	mgr, err := NewManager(services, "tanuki-test-net", nil)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	_ = exec.Command("docker", "network", "create", "tanuki-test-net").Run()
	defer func() { _ = exec.Command("docker", "network", "rm", "tanuki-test-net").Run() }()

	cleanupService(t, "redis")
	defer cleanupService(t, "redis")

	// Start first time
	err = mgr.StartService("redis")
	if err != nil {
		t.Fatalf("StartService failed: %v", err)
	}

	time.Sleep(3 * time.Second)

	// Start second time - should error
	err = mgr.StartService("redis")
	if err == nil {
		t.Error("Expected error when starting already running service")
	}
	if err.Error() != ErrServiceAlreadyRunning.Error()+": redis" {
		// Check if the error wraps ErrServiceAlreadyRunning
		if !strings.Contains(err.Error(), "already running") {
			t.Errorf("Expected ErrServiceAlreadyRunning, got: %v", err)
		}
	}
}

func TestStopService_NotRunning_Integration(t *testing.T) {
	skipIfDockerNotRunning(t)

	cfg := DefaultRedisConfig()
	cfg.Enabled = true
	services := map[string]*Config{"redis": cfg}

	mgr, err := NewManager(services, "tanuki-test-net", nil)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	cleanupService(t, "redis")

	// Try to stop a service that isn't running
	err = mgr.StopService("redis")
	if err != ErrServiceNotRunning {
		t.Errorf("Expected ErrServiceNotRunning, got: %v", err)
	}
}
