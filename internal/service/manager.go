package service

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// DockerManager defines the interface for Docker operations needed by ServiceManager.
type DockerManager interface {
	EnsureNetwork(name string) error
	ContainerExists(containerID string) bool
	ContainerRunning(containerID string) bool
}

// ServiceManager implements the Manager interface for shared services.
type ServiceManager struct {
	// services is the configuration for all services.
	services map[string]*Config

	// networkName is the Docker network services connect to.
	networkName string

	// docker provides Docker operations.
	docker DockerManager

	// status caches the current status of each service.
	status map[string]*Status

	// mu protects concurrent access to status.
	mu sync.RWMutex

	// healthMonitor monitors service health and restarts unhealthy services.
	healthMonitor *HealthMonitor

	// healthCtx controls the health monitor lifecycle.
	healthCtx context.Context

	// healthCancel stops the health monitor.
	healthCancel context.CancelFunc
}

// NewManager creates a new ServiceManager.
func NewManager(services map[string]*Config, networkName string, docker DockerManager) (*ServiceManager, error) {
	if services == nil {
		services = make(map[string]*Config)
	}
	if networkName == "" {
		networkName = "tanuki-net"
	}

	m := &ServiceManager{
		services:    services,
		networkName: networkName,
		docker:      docker,
		status:      make(map[string]*Status),
	}

	// Initialize status for all configured services
	for name := range services {
		m.status[name] = &Status{
			Name:          name,
			ContainerName: ContainerName(name),
		}
	}

	// Initialize health monitor
	m.healthMonitor = NewHealthMonitor(services, docker)

	return m, nil
}

// StartServices starts all enabled services.
func (m *ServiceManager) StartServices() error {
	// Ensure network exists
	if m.docker != nil {
		if err := m.docker.EnsureNetwork(m.networkName); err != nil {
			return fmt.Errorf("failed to ensure network: %w", err)
		}
	}

	var errs []string
	for name, cfg := range m.services {
		if !cfg.Enabled {
			continue
		}

		if err := m.StartService(name); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", name, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to start services: %s", strings.Join(errs, "; "))
	}

	// Start health monitoring
	m.healthCtx, m.healthCancel = context.WithCancel(context.Background())
	go m.healthMonitor.Start(m.healthCtx)

	return nil
}

// StopServices stops all running services.
func (m *ServiceManager) StopServices() error {
	// Stop health monitoring first
	if m.healthCancel != nil {
		m.healthCancel()
		m.healthCancel = nil
	}

	var errs []string
	for name := range m.services {
		if err := m.StopService(name); err != nil {
			// Ignore "not running" errors when stopping all
			if err != ErrServiceNotRunning {
				errs = append(errs, fmt.Sprintf("%s: %v", name, err))
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to stop services: %s", strings.Join(errs, "; "))
	}

	return nil
}

// StartService starts a specific service by name.
func (m *ServiceManager) StartService(name string) error {
	cfg, ok := m.services[name]
	if !ok {
		return fmt.Errorf("%w: %s", ErrServiceNotFound, name)
	}

	containerName := ContainerName(name)

	// Check if already running
	if m.isContainerRunning(containerName) {
		return fmt.Errorf("%w: %s", ErrServiceAlreadyRunning, name)
	}

	// Remove any existing stopped container
	m.removeContainer(containerName)

	// Build docker run command
	args := []string{
		"run", "-d",
		"--name", containerName,
		"--network", m.networkName,
	}

	// Add port mapping for host access
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

	// Add health check if configured
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

	// Run the container
	cmd := exec.Command("docker", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to start %s: %s", name, stderr.String())
	}

	containerID := strings.TrimSpace(string(output))

	// Update status
	m.mu.Lock()
	m.status[name] = &Status{
		Name:          name,
		Running:       true,
		Healthy:       false, // Will be updated by health check
		ContainerID:   containerID,
		ContainerName: containerName,
		StartedAt:     time.Now(),
		Port:          cfg.Port,
	}
	m.mu.Unlock()

	// Wait for health check if configured
	if cfg.Healthcheck != nil && len(cfg.Healthcheck.Command) > 0 {
		if err := m.waitForHealth(name, containerName); err != nil {
			// Log warning but don't fail - service started
			m.mu.Lock()
			m.status[name].Error = err.Error()
			m.mu.Unlock()
		} else {
			m.mu.Lock()
			m.status[name].Healthy = true
			m.mu.Unlock()
		}
	} else {
		// No health check - assume healthy after short delay
		time.Sleep(2 * time.Second)
		m.mu.Lock()
		m.status[name].Healthy = true
		m.mu.Unlock()
	}

	return nil
}

// StopService stops a specific service by name.
func (m *ServiceManager) StopService(name string) error {
	_, ok := m.services[name]
	if !ok {
		return fmt.Errorf("%w: %s", ErrServiceNotFound, name)
	}

	containerName := ContainerName(name)

	// Check if running
	if !m.isContainerRunning(containerName) {
		return ErrServiceNotRunning
	}

	// Stop the container
	cmd := exec.Command("docker", "stop", containerName)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to stop %s: %s", name, stderr.String())
	}

	// Remove the container
	m.removeContainer(containerName)

	// Update status
	m.mu.Lock()
	if status, exists := m.status[name]; exists {
		status.Running = false
		status.Healthy = false
		status.ContainerID = ""
	}
	m.mu.Unlock()

	return nil
}

// GetStatus returns the status of a specific service.
func (m *ServiceManager) GetStatus(name string) (*Status, error) {
	_, ok := m.services[name]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrServiceNotFound, name)
	}

	m.mu.RLock()
	status := m.status[name]
	m.mu.RUnlock()

	if status == nil {
		return &Status{
			Name:          name,
			ContainerName: ContainerName(name),
		}, nil
	}

	// Refresh running status from Docker
	containerName := ContainerName(name)
	if m.isContainerRunning(containerName) {
		status.Running = true
		// Use health monitor status if available
		if healthStatus := m.healthMonitor.GetStatus(name); healthStatus != nil {
			status.Healthy = healthStatus.Healthy
		} else {
			// Fallback for services without health monitoring
			status.Healthy = status.Running
		}
	} else {
		status.Running = false
		status.Healthy = false
	}

	return status, nil
}

// GetAllStatus returns the status of all configured services.
func (m *ServiceManager) GetAllStatus() map[string]*Status {
	result := make(map[string]*Status)

	for name := range m.services {
		status, err := m.GetStatus(name)
		if err == nil && status != nil {
			result[name] = status
		}
	}

	return result
}

// IsHealthy checks if a service is healthy.
func (m *ServiceManager) IsHealthy(name string) bool {
	// Use health monitor for accurate health status
	return m.healthMonitor.IsHealthy(name)
}

// GetConnectionInfo returns connection information for a service.
func (m *ServiceManager) GetConnectionInfo(name string) (*Connection, error) {
	cfg, ok := m.services[name]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrServiceNotFound, name)
	}

	if !cfg.Enabled {
		return nil, fmt.Errorf("service %s is not enabled", name)
	}

	containerName := ContainerName(name)

	conn := &Connection{
		Host: containerName, // Docker DNS resolves container names
		Port: cfg.Port,
		URL:  fmt.Sprintf("%s:%d", containerName, cfg.Port),
	}

	// Extract credentials from environment
	for key, value := range cfg.Environment {
		switch {
		case strings.HasSuffix(key, "_USER") || strings.HasSuffix(key, "_USERNAME"):
			conn.Username = value
		case strings.HasSuffix(key, "_PASSWORD"):
			conn.Password = value
		case strings.HasSuffix(key, "_DB") || strings.HasSuffix(key, "_DATABASE"):
			conn.Database = value
		}
	}

	return conn, nil
}

// GetAllConnections returns connection info for all enabled services.
func (m *ServiceManager) GetAllConnections() map[string]*Connection {
	result := make(map[string]*Connection)

	for name, cfg := range m.services {
		if !cfg.Enabled {
			continue
		}

		conn, err := m.GetConnectionInfo(name)
		if err == nil {
			result[name] = conn
		}
	}

	return result
}

// waitForHealth waits for a service to become healthy.
func (m *ServiceManager) waitForHealth(name, containerName string) error {
	cfg := m.services[name]
	if cfg.Healthcheck == nil {
		return nil
	}

	retries := cfg.Healthcheck.Retries
	if retries <= 0 {
		retries = 30 // Default to 30 retries
	}

	for i := 0; i < retries; i++ {
		if m.checkHealth(name, containerName) {
			return nil
		}
		time.Sleep(time.Second)
	}

	return fmt.Errorf("%w: %s not ready after %d attempts", ErrHealthCheckFailed, name, retries)
}

// checkHealth checks if a service is healthy.
func (m *ServiceManager) checkHealth(name, containerName string) bool {
	cfg := m.services[name]
	if cfg.Healthcheck == nil || len(cfg.Healthcheck.Command) == 0 {
		return m.isContainerRunning(containerName)
	}

	// Execute health check command in container
	args := []string{"exec", containerName}
	args = append(args, cfg.Healthcheck.Command...)

	cmd := exec.Command("docker", args...)
	return cmd.Run() == nil
}

// isContainerRunning checks if a container is running.
func (m *ServiceManager) isContainerRunning(containerName string) bool {
	// Use docker manager if available
	if m.docker != nil {
		return m.docker.ContainerRunning(containerName)
	}

	// Fallback to direct docker command
	cmd := exec.Command("docker", "inspect", "--format", "{{.State.Running}}", containerName)
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(output)) == "true"
}

// removeContainer removes a container (forcefully).
func (m *ServiceManager) removeContainer(containerName string) {
	cmd := exec.Command("docker", "rm", "-f", containerName)
	_ = cmd.Run()
}
