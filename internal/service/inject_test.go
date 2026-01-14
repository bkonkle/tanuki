package service

import (
	"strings"
	"testing"
)

// mockManager implements ManagerInterface for testing.
type mockManager struct {
	connections map[string]*Connection
	statuses    map[string]*Status
}

func (m *mockManager) StartServices() error        { return nil }
func (m *mockManager) StopServices() error         { return nil }
func (m *mockManager) StartService(_ string) error { return nil }
func (m *mockManager) StopService(_ string) error  { return nil }

func (m *mockManager) GetStatus(name string) (*Status, error) {
	if status, ok := m.statuses[name]; ok {
		return status, nil
	}
	return nil, ErrServiceNotFound
}

func (m *mockManager) GetAllStatus() map[string]*Status {
	return m.statuses
}

func (m *mockManager) IsHealthy(name string) bool {
	if status, ok := m.statuses[name]; ok {
		return status.Running && status.Healthy
	}
	return false
}

func (m *mockManager) GetConnectionInfo(name string) (*Connection, error) {
	if conn, ok := m.connections[name]; ok {
		return conn, nil
	}
	return nil, ErrServiceNotFound
}

func (m *mockManager) GetAllConnections() map[string]*Connection {
	return m.connections
}

func TestInjectorBuildEnvironment(t *testing.T) {
	mgr := &mockManager{
		connections: map[string]*Connection{
			"postgres": {
				Host:     "tanuki-svc-postgres",
				Port:     5432,
				URL:      "tanuki-svc-postgres:5432",
				Username: "tanuki",
				Password: "secret",
				Database: "tanuki_dev",
			},
			"redis": {
				Host: "tanuki-svc-redis",
				Port: 6379,
				URL:  "tanuki-svc-redis:6379",
			},
		},
	}

	injector := NewInjector(mgr)
	env := injector.BuildEnvironment()

	// Check postgres vars
	if env["POSTGRES_HOST"] != "tanuki-svc-postgres" {
		t.Errorf("POSTGRES_HOST = %q, want tanuki-svc-postgres", env["POSTGRES_HOST"])
	}
	if env["POSTGRES_PORT"] != "5432" {
		t.Errorf("POSTGRES_PORT = %q, want 5432", env["POSTGRES_PORT"])
	}
	if env["POSTGRES_USER"] != "tanuki" {
		t.Errorf("POSTGRES_USER = %q, want tanuki", env["POSTGRES_USER"])
	}
	if env["POSTGRES_PASSWORD"] != "secret" {
		t.Errorf("POSTGRES_PASSWORD = %q, want secret", env["POSTGRES_PASSWORD"])
	}
	if env["POSTGRES_DATABASE"] != "tanuki_dev" {
		t.Errorf("POSTGRES_DATABASE = %q, want tanuki_dev", env["POSTGRES_DATABASE"])
	}
	if !strings.Contains(env["POSTGRES_DSN"], "postgres://") {
		t.Errorf("POSTGRES_DSN = %q, want postgres:// prefix", env["POSTGRES_DSN"])
	}

	// Check redis vars
	if env["REDIS_HOST"] != "tanuki-svc-redis" {
		t.Errorf("REDIS_HOST = %q, want tanuki-svc-redis", env["REDIS_HOST"])
	}
	if env["REDIS_PORT"] != "6379" {
		t.Errorf("REDIS_PORT = %q, want 6379", env["REDIS_PORT"])
	}
	if !strings.Contains(env["REDIS_DSN"], "redis://") {
		t.Errorf("REDIS_DSN = %q, want redis:// prefix", env["REDIS_DSN"])
	}
}

func TestInjectorBuildEnvironmentNilManager(t *testing.T) {
	injector := NewInjector(nil)
	env := injector.BuildEnvironment()

	if len(env) != 0 {
		t.Errorf("BuildEnvironment() with nil manager should return empty map, got %d items", len(env))
	}
}

func TestInjectorBuildEnvironmentSlice(t *testing.T) {
	mgr := &mockManager{
		connections: map[string]*Connection{
			"postgres": {
				Host: "host1",
				Port: 5432,
				URL:  "host1:5432",
			},
		},
	}

	injector := NewInjector(mgr)
	slice := injector.BuildEnvironmentSlice()

	found := false
	for _, item := range slice {
		if item == "POSTGRES_HOST=host1" {
			found = true
			break
		}
	}

	if !found {
		t.Error("BuildEnvironmentSlice() should contain POSTGRES_HOST=host1")
	}
}

func TestInjectorCheckServiceHealth(t *testing.T) {
	mgr := &mockManager{
		statuses: map[string]*Status{
			"postgres": {
				Name:    "postgres",
				Running: true,
				Healthy: false, // Unhealthy
			},
			"redis": {
				Name:    "redis",
				Running: true,
				Healthy: true, // Healthy
			},
		},
	}

	injector := NewInjector(mgr)
	warnings := injector.CheckServiceHealth()

	if len(warnings) != 1 {
		t.Errorf("CheckServiceHealth() returned %d warnings, want 1", len(warnings))
	}

	if len(warnings) > 0 && !strings.Contains(warnings[0], "postgres") {
		t.Errorf("Warning should mention postgres, got: %s", warnings[0])
	}
}

func TestInjectorCheckServiceHealthNilManager(t *testing.T) {
	injector := NewInjector(nil)
	warnings := injector.CheckServiceHealth()

	if len(warnings) != 0 {
		t.Errorf("CheckServiceHealth() with nil manager should return empty slice, got %d warnings", len(warnings))
	}
}

func TestInjectorGenerateDocumentation(t *testing.T) {
	mgr := &mockManager{
		connections: map[string]*Connection{
			"postgres": {
				Host:     "tanuki-svc-postgres",
				Port:     5432,
				URL:      "tanuki-svc-postgres:5432",
				Username: "tanuki",
				Database: "tanuki_dev",
			},
		},
	}

	injector := NewInjector(mgr)
	docs := injector.GenerateDocumentation()

	if !strings.Contains(docs, "## Available Services") {
		t.Error("Documentation should contain '## Available Services' header")
	}
	if !strings.Contains(docs, "POSTGRES_HOST") {
		t.Error("Documentation should contain POSTGRES_HOST")
	}
	if !strings.Contains(docs, "tanuki-svc-postgres") {
		t.Error("Documentation should contain actual host value")
	}
}

func TestInjectorGenerateDocumentationEmpty(t *testing.T) {
	mgr := &mockManager{
		connections: map[string]*Connection{},
	}

	injector := NewInjector(mgr)
	docs := injector.GenerateDocumentation()

	if docs != "" {
		t.Errorf("GenerateDocumentation() with no connections should return empty string, got: %s", docs)
	}
}

func TestInjectorGenerateDocumentationNilManager(t *testing.T) {
	injector := NewInjector(nil)
	docs := injector.GenerateDocumentation()

	if docs != "" {
		t.Errorf("GenerateDocumentation() with nil manager should return empty string, got: %s", docs)
	}
}

func TestBuildDSN(t *testing.T) {
	tests := []struct {
		name     string
		conn     *Connection
		expected string
	}{
		{
			name: "postgres",
			conn: &Connection{
				Host:     "localhost",
				Port:     5432,
				Username: "user",
				Password: "pass",
				Database: "db",
			},
			expected: "postgres://user:pass@localhost:5432/db",
		},
		{
			name: "redis",
			conn: &Connection{
				Host: "localhost",
				Port: 6379,
			},
			expected: "redis://localhost:6379",
		},
		{
			name: "mysql",
			conn: &Connection{
				Host:     "localhost",
				Port:     3306,
				Username: "user",
				Password: "pass",
				Database: "db",
			},
			expected: "mysql://user:pass@localhost:3306/db",
		},
		{
			name: "unknown",
			conn: &Connection{
				Host: "localhost",
				Port: 8080,
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildDSN(tt.name, tt.conn)
			if result != tt.expected {
				t.Errorf("buildDSN(%q) = %q, want %q", tt.name, result, tt.expected)
			}
		})
	}
}

func TestMergeEnvironment(t *testing.T) {
	existing := map[string]string{
		"FOO": "bar",
		"BAZ": "qux",
	}

	serviceEnv := map[string]string{
		"SERVICE_HOST": "localhost",
		"FOO":          "should_not_override",
	}

	result := MergeEnvironment(existing, serviceEnv)

	// Existing should take precedence
	if result["FOO"] != "bar" {
		t.Errorf("FOO should be 'bar', got %q", result["FOO"])
	}

	// Service env should be included
	if result["SERVICE_HOST"] != "localhost" {
		t.Errorf("SERVICE_HOST should be 'localhost', got %q", result["SERVICE_HOST"])
	}

	// Existing should be preserved
	if result["BAZ"] != "qux" {
		t.Errorf("BAZ should be 'qux', got %q", result["BAZ"])
	}
}
