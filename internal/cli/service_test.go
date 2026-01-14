package cli

import (
	"testing"
	"time"

	"github.com/bkonkle/tanuki/internal/service"
)

func TestFormatDurationService(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{"seconds", 30 * time.Second, "30s"},
		{"minutes", 5 * time.Minute, "5m"},
		{"hours", 3 * time.Hour, "3h"},
		{"days", 48 * time.Hour, "2d"},
		{"under_minute", 45 * time.Second, "45s"},
		{"over_hour", 90 * time.Minute, "1h"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDuration(tt.duration)
			if result != tt.expected {
				t.Errorf("formatDuration(%v) = %q, want %q", tt.duration, result, tt.expected)
			}
		})
	}
}

func TestStubManagerGetAllStatus(t *testing.T) {
	services := map[string]*service.Config{
		"postgres": service.DefaultPostgresConfig(),
		"redis":    service.DefaultRedisConfig(),
	}

	mgr := service.NewStubManager(services, "test-net")
	statuses := mgr.GetAllStatus()

	// Should return status for both services
	if len(statuses) != 2 {
		t.Errorf("GetAllStatus() returned %d statuses, want 2", len(statuses))
	}

	// Check postgres status exists
	if _, ok := statuses["postgres"]; !ok {
		t.Error("GetAllStatus() missing postgres status")
	}

	// Check redis status exists
	if _, ok := statuses["redis"]; !ok {
		t.Error("GetAllStatus() missing redis status")
	}
}

func TestStubManagerGetStatus(t *testing.T) {
	services := map[string]*service.Config{
		"postgres": service.DefaultPostgresConfig(),
	}

	mgr := service.NewStubManager(services, "test-net")

	// Test getting status for configured service
	status, err := mgr.GetStatus("postgres")
	if err != nil {
		t.Errorf("GetStatus(postgres) error: %v", err)
	}
	if status == nil {
		t.Fatal("GetStatus(postgres) returned nil")
	}
	if status.Name != "postgres" {
		t.Errorf("GetStatus(postgres).Name = %q, want postgres", status.Name)
	}
	if status.Port != 5432 {
		t.Errorf("GetStatus(postgres).Port = %d, want 5432", status.Port)
	}
	if status.ContainerName != "tanuki-svc-postgres" {
		t.Errorf("GetStatus(postgres).ContainerName = %q, want tanuki-svc-postgres", status.ContainerName)
	}

	// Test getting status for non-existent service
	_, err = mgr.GetStatus("nonexistent")
	if err != service.ErrServiceNotFound {
		t.Errorf("GetStatus(nonexistent) error = %v, want ErrServiceNotFound", err)
	}
}

func TestStubManagerGetConfig(t *testing.T) {
	postgresConfig := service.DefaultPostgresConfig()
	services := map[string]*service.Config{
		"postgres": postgresConfig,
	}

	mgr := service.NewStubManager(services, "test-net")

	// Test getting config for configured service
	cfg, err := mgr.GetConfig("postgres")
	if err != nil {
		t.Errorf("GetConfig(postgres) error: %v", err)
	}
	if cfg != postgresConfig {
		t.Error("GetConfig(postgres) returned different config")
	}

	// Test getting config for non-existent service
	_, err = mgr.GetConfig("nonexistent")
	if err != service.ErrServiceNotFound {
		t.Errorf("GetConfig(nonexistent) error = %v, want ErrServiceNotFound", err)
	}
}

func TestContainerName(t *testing.T) {
	tests := []struct {
		serviceName string
		expected    string
	}{
		{"postgres", "tanuki-svc-postgres"},
		{"redis", "tanuki-svc-redis"},
		{"mysql", "tanuki-svc-mysql"},
	}

	for _, tt := range tests {
		t.Run(tt.serviceName, func(t *testing.T) {
			result := service.ContainerName(tt.serviceName)
			if result != tt.expected {
				t.Errorf("ContainerName(%q) = %q, want %q", tt.serviceName, result, tt.expected)
			}
		})
	}
}

func TestDefaultConfigs(t *testing.T) {
	t.Run("postgres", func(t *testing.T) {
		cfg := service.DefaultPostgresConfig()
		if cfg.Image != "postgres:16" {
			t.Errorf("DefaultPostgresConfig().Image = %q, want postgres:16", cfg.Image)
		}
		if cfg.Port != 5432 {
			t.Errorf("DefaultPostgresConfig().Port = %d, want 5432", cfg.Port)
		}
		if cfg.Enabled {
			t.Error("DefaultPostgresConfig().Enabled should be false")
		}
		if cfg.Environment["POSTGRES_USER"] != "tanuki" {
			t.Errorf("DefaultPostgresConfig() POSTGRES_USER = %q, want tanuki", cfg.Environment["POSTGRES_USER"])
		}
	})

	t.Run("redis", func(t *testing.T) {
		cfg := service.DefaultRedisConfig()
		if cfg.Image != "redis:7" {
			t.Errorf("DefaultRedisConfig().Image = %q, want redis:7", cfg.Image)
		}
		if cfg.Port != 6379 {
			t.Errorf("DefaultRedisConfig().Port = %d, want 6379", cfg.Port)
		}
		if cfg.Enabled {
			t.Error("DefaultRedisConfig().Enabled should be false")
		}
	})
}

func TestStubManagerIsHealthy(t *testing.T) {
	services := map[string]*service.Config{
		"postgres": service.DefaultPostgresConfig(),
	}

	mgr := service.NewStubManager(services, "test-net")

	// Without a running container, IsHealthy should return false
	if mgr.IsHealthy("postgres") {
		t.Error("IsHealthy(postgres) should be false when not running")
	}

	// Non-existent service should return false
	if mgr.IsHealthy("nonexistent") {
		t.Error("IsHealthy(nonexistent) should be false")
	}
}

func TestStubManagerGetConnectionInfo(t *testing.T) {
	services := map[string]*service.Config{
		"postgres": service.DefaultPostgresConfig(),
	}

	mgr := service.NewStubManager(services, "test-net")

	// Without a running container, should get ErrServiceNotRunning
	_, err := mgr.GetConnectionInfo("postgres")
	if err != service.ErrServiceNotRunning {
		t.Errorf("GetConnectionInfo(postgres) error = %v, want ErrServiceNotRunning", err)
	}

	// Non-existent service should get ErrServiceNotFound
	_, err = mgr.GetConnectionInfo("nonexistent")
	if err != service.ErrServiceNotFound {
		t.Errorf("GetConnectionInfo(nonexistent) error = %v, want ErrServiceNotFound", err)
	}
}

func TestStubManagerGetAllConnections(t *testing.T) {
	services := map[string]*service.Config{
		"postgres": service.DefaultPostgresConfig(),
		"redis":    service.DefaultRedisConfig(),
	}

	mgr := service.NewStubManager(services, "test-net")

	// Without running containers, should return empty map
	connections := mgr.GetAllConnections()
	if len(connections) != 0 {
		t.Errorf("GetAllConnections() returned %d connections, want 0", len(connections))
	}
}

func TestServiceCommandStructure(t *testing.T) {
	// Test that all subcommands are registered
	subcommands := serviceCmd.Commands()

	expectedCmds := []string{"start", "stop", "status", "logs", "connect"}
	cmdMap := make(map[string]bool)
	for _, cmd := range subcommands {
		cmdMap[cmd.Name()] = true
	}

	for _, expected := range expectedCmds {
		if !cmdMap[expected] {
			t.Errorf("service command missing subcommand: %s", expected)
		}
	}
}

func TestServiceCommandUsage(t *testing.T) {
	// Test service command use string
	if serviceCmd.Use != "service" {
		t.Errorf("serviceCmd.Use = %q, want service", serviceCmd.Use)
	}

	// Test start command accepts optional argument
	if serviceStartCmd.Use != "start [name]" {
		t.Errorf("serviceStartCmd.Use = %q, want start [name]", serviceStartCmd.Use)
	}

	// Test logs command requires argument
	if serviceLogsCmd.Use != "logs <name>" {
		t.Errorf("serviceLogsCmd.Use = %q, want logs <name>", serviceLogsCmd.Use)
	}

	// Test connect command requires argument
	if serviceConnectCmd.Use != "connect <name>" {
		t.Errorf("serviceConnectCmd.Use = %q, want connect <name>", serviceConnectCmd.Use)
	}
}

func TestServiceLogsFlags(t *testing.T) {
	// Test that --follow flag exists
	flag := serviceLogsCmd.Flags().Lookup("follow")
	if flag == nil {
		t.Error("serviceLogsCmd missing --follow flag")
	}
	if flag != nil && flag.Shorthand != "f" {
		t.Errorf("--follow shorthand = %q, want f", flag.Shorthand)
	}

	// Test that --tail flag exists
	tailFlag := serviceLogsCmd.Flags().Lookup("tail")
	if tailFlag == nil {
		t.Error("serviceLogsCmd missing --tail flag")
	}
}
