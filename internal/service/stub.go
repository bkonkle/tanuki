package service

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// StubManager is a minimal implementation of Manager that CLI commands can use.
// This stub provides basic functionality using Docker commands directly.
// It will be replaced by the full implementation from Workstream A.
type StubManager struct {
	// Services is a map of service name to configuration
	Services map[string]*Config
	// NetworkName is the Docker network to connect services to
	NetworkName string
}

// NewStubManager creates a new stub service manager.
func NewStubManager(services map[string]*Config, networkName string) *StubManager {
	return &StubManager{
		Services:    services,
		NetworkName: networkName,
	}
}

// StartServices starts all enabled services.
func (m *StubManager) StartServices() error {
	for name, cfg := range m.Services {
		if !cfg.Enabled {
			continue
		}
		if err := m.StartService(name); err != nil {
			return fmt.Errorf("failed to start %s: %w", name, err)
		}
	}
	return nil
}

// StopServices stops all running services.
func (m *StubManager) StopServices() error {
	for name := range m.Services {
		status, err := m.GetStatus(name)
		if err != nil {
			continue
		}
		if status.Running {
			if err := m.StopService(name); err != nil {
				return fmt.Errorf("failed to stop %s: %w", name, err)
			}
		}
	}
	return nil
}

// StartService starts a specific service by name.
func (m *StubManager) StartService(name string) error {
	cfg, ok := m.Services[name]
	if !ok {
		return ErrServiceNotFound
	}

	containerName := ContainerName(name)

	// Check if already running
	if m.containerRunning(containerName) {
		return nil // Already running
	}

	// Check if container exists but is stopped
	if m.containerExists(containerName) {
		return m.startContainer(containerName)
	}

	// Create and start new container
	return m.createContainer(name, cfg)
}

// StopService stops a specific service by name.
func (m *StubManager) StopService(name string) error {
	if _, ok := m.Services[name]; !ok {
		return ErrServiceNotFound
	}

	containerName := ContainerName(name)
	if !m.containerExists(containerName) {
		return nil // Not running
	}

	cmd := exec.Command("docker", "stop", containerName)
	return cmd.Run()
}

// GetStatus returns the status of a specific service.
func (m *StubManager) GetStatus(name string) (*Status, error) {
	cfg, ok := m.Services[name]
	if !ok {
		return nil, ErrServiceNotFound
	}

	containerName := ContainerName(name)
	status := &Status{
		Name:          name,
		ContainerName: containerName,
		Port:          cfg.Port,
	}

	// Check if container exists
	if !m.containerExists(containerName) {
		return status, nil
	}

	// Get container info
	cmd := exec.Command("docker", "inspect", "--format",
		"{{.Id}}|{{.State.Running}}|{{.State.Health.Status}}|{{.State.StartedAt}}",
		containerName)
	output, err := cmd.Output()
	if err != nil {
		return status, nil
	}

	parts := strings.Split(strings.TrimSpace(string(output)), "|")
	if len(parts) >= 4 {
		status.ContainerID = parts[0]
		status.Running = parts[1] == "true"
		status.Healthy = parts[2] == "healthy"
		if t, err := time.Parse(time.RFC3339Nano, parts[3]); err == nil {
			status.StartedAt = t
		}
	}

	return status, nil
}

// GetAllStatus returns the status of all configured services.
func (m *StubManager) GetAllStatus() map[string]*Status {
	statuses := make(map[string]*Status)
	for name := range m.Services {
		status, err := m.GetStatus(name)
		if err == nil {
			statuses[name] = status
		}
	}
	return statuses
}

// IsHealthy checks if a service is healthy.
func (m *StubManager) IsHealthy(name string) bool {
	status, err := m.GetStatus(name)
	if err != nil {
		return false
	}
	return status.Running && status.Healthy
}

// GetConnectionInfo returns connection information for a service.
func (m *StubManager) GetConnectionInfo(name string) (*Connection, error) {
	cfg, ok := m.Services[name]
	if !ok {
		return nil, ErrServiceNotFound
	}

	status, err := m.GetStatus(name)
	if err != nil {
		return nil, err
	}
	if !status.Running {
		return nil, ErrServiceNotRunning
	}

	containerName := ContainerName(name)
	conn := &Connection{
		Host: containerName,
		Port: cfg.Port,
		URL:  fmt.Sprintf("%s:%d", containerName, cfg.Port),
	}

	// Extract credentials from environment
	if user, ok := cfg.Environment["POSTGRES_USER"]; ok {
		conn.Username = user
	}
	if pass, ok := cfg.Environment["POSTGRES_PASSWORD"]; ok {
		conn.Password = pass
	}
	if db, ok := cfg.Environment["POSTGRES_DB"]; ok {
		conn.Database = db
	}

	return conn, nil
}

// GetAllConnections returns connection info for all running services.
func (m *StubManager) GetAllConnections() map[string]*Connection {
	connections := make(map[string]*Connection)
	for name := range m.Services {
		conn, err := m.GetConnectionInfo(name)
		if err == nil {
			connections[name] = conn
		}
	}
	return connections
}

// containerExists checks if a container exists.
func (m *StubManager) containerExists(name string) bool {
	cmd := exec.Command("docker", "container", "inspect", name)
	return cmd.Run() == nil
}

// containerRunning checks if a container is running.
func (m *StubManager) containerRunning(name string) bool {
	cmd := exec.Command("docker", "inspect", "--format", "{{.State.Running}}", name)
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(output)) == "true"
}

// startContainer starts an existing stopped container.
func (m *StubManager) startContainer(name string) error {
	cmd := exec.Command("docker", "start", name)
	return cmd.Run()
}

// createContainer creates and starts a new service container.
func (m *StubManager) createContainer(name string, cfg *Config) error {
	containerName := ContainerName(name)

	args := []string{
		"run", "-d",
		"--name", containerName,
		"--network", m.NetworkName,
	}

	// Add port mapping
	if cfg.Port > 0 {
		args = append(args, "-p", fmt.Sprintf("%d:%d", cfg.Port, cfg.Port))
	}

	// Add environment variables
	for k, v := range cfg.Environment {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}

	// Add volumes
	for _, vol := range cfg.Volumes {
		args = append(args, "-v", vol)
	}

	// Add healthcheck if configured
	if cfg.Healthcheck != nil && len(cfg.Healthcheck.Command) > 0 {
		args = append(args, "--health-cmd", strings.Join(cfg.Healthcheck.Command, " "))
		if cfg.Healthcheck.Interval != "" {
			args = append(args, "--health-interval", cfg.Healthcheck.Interval)
		}
		if cfg.Healthcheck.Timeout != "" {
			args = append(args, "--health-timeout", cfg.Healthcheck.Timeout)
		}
		if cfg.Healthcheck.Retries > 0 {
			args = append(args, "--health-retries", fmt.Sprintf("%d", cfg.Healthcheck.Retries))
		}
	}

	// Add image
	args = append(args, cfg.Image)

	cmd := exec.Command("docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker run failed: %s", strings.TrimSpace(string(output)))
	}

	return nil
}

// GetConfig returns the configuration for a service.
func (m *StubManager) GetConfig(name string) (*Config, error) {
	cfg, ok := m.Services[name]
	if !ok {
		return nil, ErrServiceNotFound
	}
	return cfg, nil
}
