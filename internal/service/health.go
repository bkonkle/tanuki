package service

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// HealthMonitor monitors service health and restarts unhealthy services.
type HealthMonitor struct {
	// services contains all configured services to monitor.
	services map[string]*Config

	// docker provides Docker operations.
	docker DockerManager

	// status tracks health status for each service.
	status map[string]*HealthStatus

	// mu protects concurrent access to status.
	mu sync.RWMutex

	// stopCh signals the monitor to stop.
	stopCh chan struct{}

	// checkInterval is how often to check service health.
	checkInterval time.Duration
}

// HealthStatus tracks health check results for a service.
type HealthStatus struct {
	// Healthy indicates if the service passed its last health check.
	Healthy bool

	// LastCheck is when the last health check was performed.
	LastCheck time.Time

	// LastHealthy is when the service was last confirmed healthy.
	LastHealthy time.Time

	// FailureCount is the number of consecutive health check failures.
	FailureCount int

	// Error contains the last health check error message.
	Error string
}

// NewHealthMonitor creates a new health monitor.
func NewHealthMonitor(services map[string]*Config, docker DockerManager) *HealthMonitor {
	return &HealthMonitor{
		services:      services,
		docker:        docker,
		status:        make(map[string]*HealthStatus),
		checkInterval: 10 * time.Second,
	}
}

// Start begins the health monitoring loop.
func (m *HealthMonitor) Start(ctx context.Context) {
	m.stopCh = make(chan struct{})
	ticker := time.NewTicker(m.checkInterval)
	defer ticker.Stop()

	// Perform initial check
	m.checkAllServices()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.checkAllServices()
		}
	}
}

// Stop halts the health monitoring loop.
func (m *HealthMonitor) Stop() {
	if m.stopCh != nil {
		close(m.stopCh)
	}
}

// checkAllServices checks health of all enabled services.
func (m *HealthMonitor) checkAllServices() {
	for name, svc := range m.services {
		if !svc.Enabled {
			continue
		}
		m.checkService(name, svc)
	}
}

// checkService performs a health check on a single service.
func (m *HealthMonitor) checkService(name string, svc *Config) {
	containerName := ContainerName(name)

	// Check if container is running
	if !m.isContainerRunning(containerName) {
		m.markUnhealthy(name, "container not running")
		return
	}

	// Run health check command
	var healthy bool
	var err error

	if svc.Healthcheck != nil && len(svc.Healthcheck.Command) > 0 {
		healthy, err = m.runHealthcheck(containerName, svc.Healthcheck)
	} else {
		// Fallback to built-in checks
		healthy, err = m.builtinHealthcheck(name, containerName)
	}

	if err != nil {
		m.markUnhealthy(name, err.Error())
		m.maybeRestart(name, svc)
		return
	}

	if healthy {
		m.markHealthy(name)
	} else {
		m.markUnhealthy(name, "health check failed")
		m.maybeRestart(name, svc)
	}
}

// runHealthcheck executes a configured health check command.
func (m *HealthMonitor) runHealthcheck(container string, hc *HealthcheckConfig) (bool, error) {
	timeout := 5 * time.Second
	if hc.Timeout != "" {
		if parsed, err := time.ParseDuration(hc.Timeout); err == nil {
			timeout = parsed
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	args := make([]string, 0, 2+len(hc.Command))
	args = append(args, "exec", container)
	args = append(args, hc.Command...)

	cmd := exec.CommandContext(ctx, "docker", args...) //nolint:gosec // args are constructed from trusted config
	err := cmd.Run()
	return err == nil, err
}

// builtinHealthcheck provides default health checks for known services.
func (m *HealthMonitor) builtinHealthcheck(name, container string) (bool, error) {
	switch name {
	case "postgres":
		return m.checkPostgres(container)
	case "redis":
		return m.checkRedis(container)
	default:
		// Generic: container running = healthy
		return true, nil
	}
}

// checkPostgres checks if PostgreSQL is ready.
func (m *HealthMonitor) checkPostgres(container string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "exec", container,
		"pg_isready", "-U", "tanuki")
	err := cmd.Run()
	return err == nil, err
}

// checkRedis checks if Redis is responding.
func (m *HealthMonitor) checkRedis(container string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "exec", container,
		"redis-cli", "ping")
	output, err := cmd.Output()
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(output)) == "PONG", nil
}

// markHealthy marks a service as healthy.
func (m *HealthMonitor) markHealthy(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	status, exists := m.status[name]
	if !exists {
		status = &HealthStatus{}
		m.status[name] = status
	}

	wasUnhealthy := !status.Healthy
	status.Healthy = true
	status.LastCheck = now
	status.LastHealthy = now
	status.FailureCount = 0
	status.Error = ""

	if wasUnhealthy {
		log.Printf("Service %s is now healthy", name)
	}
}

// markUnhealthy marks a service as unhealthy.
func (m *HealthMonitor) markUnhealthy(name, errMsg string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	status, exists := m.status[name]
	if !exists {
		status = &HealthStatus{}
		m.status[name] = status
	}

	wasHealthy := status.Healthy
	status.Healthy = false
	status.LastCheck = now
	status.FailureCount++
	status.Error = errMsg

	if wasHealthy {
		log.Printf("Service %s is now unhealthy: %s", name, errMsg)
	}
}

// maybeRestart attempts to restart a service if it has exceeded the retry threshold.
func (m *HealthMonitor) maybeRestart(name string, svc *Config) {
	m.mu.Lock()
	defer m.mu.Unlock()

	status := m.status[name]

	maxRetries := 3
	if svc.Healthcheck != nil && svc.Healthcheck.Retries > 0 {
		maxRetries = svc.Healthcheck.Retries
	}

	if status.FailureCount >= maxRetries {
		log.Printf("Service %s unhealthy after %d checks, restarting...", name, status.FailureCount)

		containerName := ContainerName(name)
		if err := m.restartContainer(containerName); err != nil {
			log.Printf("Failed to restart %s: %v", name, err)
			return
		}

		// Reset failure count, give grace period
		status.FailureCount = 0
		log.Printf("Restarted service %s", name)
	}
}

// restartContainer restarts a Docker container.
func (m *HealthMonitor) restartContainer(containerName string) error {
	cmd := exec.Command("docker", "restart", containerName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker restart failed: %w", err)
	}

	// Give the container a moment to start
	time.Sleep(2 * time.Second)

	return nil
}

// GetStatus returns the health status for a service.
func (m *HealthMonitor) GetStatus(name string) *HealthStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if status, ok := m.status[name]; ok {
		// Return a copy to avoid race conditions
		return &HealthStatus{
			Healthy:      status.Healthy,
			LastCheck:    status.LastCheck,
			LastHealthy:  status.LastHealthy,
			FailureCount: status.FailureCount,
			Error:        status.Error,
		}
	}
	return nil
}

// IsHealthy checks if a service is currently healthy.
func (m *HealthMonitor) IsHealthy(name string) bool {
	status := m.GetStatus(name)
	return status != nil && status.Healthy
}

// GetAllStatus returns health status for all monitored services.
func (m *HealthMonitor) GetAllStatus() map[string]*HealthStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*HealthStatus)
	for k, v := range m.status {
		// Return copies to avoid race conditions
		result[k] = &HealthStatus{
			Healthy:      v.Healthy,
			LastCheck:    v.LastCheck,
			LastHealthy:  v.LastHealthy,
			FailureCount: v.FailureCount,
			Error:        v.Error,
		}
	}
	return result
}

// isContainerRunning checks if a container is running.
func (m *HealthMonitor) isContainerRunning(containerName string) bool {
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
