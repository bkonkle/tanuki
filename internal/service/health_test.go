package service

import (
	"context"
	"sync"
	"testing"
	"time"
)

// setContainerRunning is a helper to set container running state in mock.
func (m *mockDockerManager) setContainerRunning(containerID string, running bool) {
	m.containerRunning[containerID] = running
	m.containers[containerID] = running
}

func TestHealthMonitor_MarkHealthy(t *testing.T) {
	services := map[string]*Config{
		"test": {
			Enabled: true,
			Image:   "test:latest",
			Port:    8080,
		},
	}

	docker := newMockDockerManager()
	monitor := NewHealthMonitor(services, docker)

	// Mark service as healthy
	monitor.markHealthy("test")

	// Verify status
	status := monitor.GetStatus("test")
	if status == nil {
		t.Fatal("expected status, got nil")
	}
	if !status.Healthy {
		t.Error("expected service to be healthy")
	}
	if status.FailureCount != 0 {
		t.Errorf("expected failure count 0, got %d", status.FailureCount)
	}
	if status.Error != "" {
		t.Errorf("expected no error, got %s", status.Error)
	}
}

func TestHealthMonitor_MarkUnhealthy(t *testing.T) {
	services := map[string]*Config{
		"test": {
			Enabled: true,
			Image:   "test:latest",
			Port:    8080,
		},
	}

	docker := newMockDockerManager()
	monitor := NewHealthMonitor(services, docker)

	// Mark service as unhealthy
	monitor.markUnhealthy("test", "connection refused")

	// Verify status
	status := monitor.GetStatus("test")
	if status == nil {
		t.Fatal("expected status, got nil")
	}
	if status.Healthy {
		t.Error("expected service to be unhealthy")
	}
	if status.FailureCount != 1 {
		t.Errorf("expected failure count 1, got %d", status.FailureCount)
	}
	if status.Error != "connection refused" {
		t.Errorf("expected error 'connection refused', got %s", status.Error)
	}
}

func TestHealthMonitor_FailureCount(t *testing.T) {
	services := map[string]*Config{
		"test": {
			Enabled: true,
			Image:   "test:latest",
			Port:    8080,
		},
	}

	docker := newMockDockerManager()
	monitor := NewHealthMonitor(services, docker)

	// Mark unhealthy multiple times
	for i := 1; i <= 3; i++ {
		monitor.markUnhealthy("test", "check failed")
		status := monitor.GetStatus("test")
		if status.FailureCount != i {
			t.Errorf("iteration %d: expected failure count %d, got %d", i, i, status.FailureCount)
		}
	}

	// Mark healthy should reset count
	monitor.markHealthy("test")
	status := monitor.GetStatus("test")
	if status.FailureCount != 0 {
		t.Errorf("expected failure count 0 after healthy, got %d", status.FailureCount)
	}
}

func TestHealthMonitor_IsHealthy(t *testing.T) {
	services := map[string]*Config{
		"test": {
			Enabled: true,
			Image:   "test:latest",
			Port:    8080,
		},
	}

	docker := newMockDockerManager()
	monitor := NewHealthMonitor(services, docker)

	// Initially no status
	if monitor.IsHealthy("test") {
		t.Error("expected false for service with no status")
	}

	// Mark healthy
	monitor.markHealthy("test")
	if !monitor.IsHealthy("test") {
		t.Error("expected true for healthy service")
	}

	// Mark unhealthy
	monitor.markUnhealthy("test", "failed")
	if monitor.IsHealthy("test") {
		t.Error("expected false for unhealthy service")
	}
}

func TestHealthMonitor_GetAllStatus(t *testing.T) {
	services := map[string]*Config{
		"service1": {
			Enabled: true,
			Image:   "test1:latest",
			Port:    8081,
		},
		"service2": {
			Enabled: true,
			Image:   "test2:latest",
			Port:    8082,
		},
	}

	docker := newMockDockerManager()
	monitor := NewHealthMonitor(services, docker)

	// Mark services with different states
	monitor.markHealthy("service1")
	monitor.markUnhealthy("service2", "error")

	// Get all status
	allStatus := monitor.GetAllStatus()
	if len(allStatus) != 2 {
		t.Errorf("expected 2 statuses, got %d", len(allStatus))
	}

	if !allStatus["service1"].Healthy {
		t.Error("expected service1 to be healthy")
	}
	if allStatus["service2"].Healthy {
		t.Error("expected service2 to be unhealthy")
	}
}

func TestHealthMonitor_ContainerNotRunning(t *testing.T) {
	services := map[string]*Config{
		"test": {
			Enabled: true,
			Image:   "test:latest",
			Port:    8080,
		},
	}

	docker := newMockDockerManager()
	monitor := NewHealthMonitor(services, docker)

	// Container not running
	docker.setContainerRunning(ContainerName("test"), false)

	// Check service
	monitor.checkService("test", services["test"])

	// Should be marked unhealthy
	status := monitor.GetStatus("test")
	if status == nil {
		t.Fatal("expected status, got nil")
	}
	if status.Healthy {
		t.Error("expected service to be unhealthy when container not running")
	}
	if status.Error != "container not running" {
		t.Errorf("expected error 'container not running', got %s", status.Error)
	}
}

func TestHealthMonitor_BuiltinHealthcheck(t *testing.T) {
	services := map[string]*Config{
		"unknown": {
			Enabled: true,
			Image:   "unknown:latest",
			Port:    9999,
		},
	}

	docker := newMockDockerManager()
	monitor := NewHealthMonitor(services, docker)

	// Container running
	containerName := ContainerName("unknown")
	docker.setContainerRunning(containerName, true)

	// Unknown service type should use generic check (running = healthy)
	healthy, err := monitor.builtinHealthcheck("unknown", containerName)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if !healthy {
		t.Error("expected healthy for generic check on running container")
	}
}

func TestHealthMonitor_StartStop(t *testing.T) {
	services := map[string]*Config{
		"test": {
			Enabled: true,
			Image:   "test:latest",
			Port:    8080,
		},
	}

	docker := newMockDockerManager()
	docker.setContainerRunning(ContainerName("test"), true)

	monitor := NewHealthMonitor(services, docker)
	monitor.checkInterval = 50 * time.Millisecond

	// Start monitoring
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go monitor.Start(ctx)

	// Wait for at least one check
	time.Sleep(100 * time.Millisecond)

	// Verify check was performed
	status := monitor.GetStatus("test")
	if status == nil {
		t.Fatal("expected status after health check, got nil")
	}
	if status.LastCheck.IsZero() {
		t.Error("expected LastCheck to be set")
	}

	// Stop monitoring
	cancel()
	time.Sleep(50 * time.Millisecond)
}

func TestHealthMonitor_MultipleServices(t *testing.T) {
	services := map[string]*Config{
		"service1": {
			Enabled: true,
			Image:   "test1:latest",
			Port:    8081,
		},
		"service2": {
			Enabled: true,
			Image:   "test2:latest",
			Port:    8082,
		},
		"disabled": {
			Enabled: false,
			Image:   "test3:latest",
			Port:    8083,
		},
	}

	docker := newMockDockerManager()
	docker.setContainerRunning(ContainerName("service1"), true)
	docker.setContainerRunning(ContainerName("service2"), true)
	docker.setContainerRunning(ContainerName("disabled"), true)

	monitor := NewHealthMonitor(services, docker)

	// Check all services
	monitor.checkAllServices()

	// Service1 and service2 should have status
	if monitor.GetStatus("service1") == nil {
		t.Error("expected status for service1")
	}
	if monitor.GetStatus("service2") == nil {
		t.Error("expected status for service2")
	}

	// Disabled service should not have status
	if monitor.GetStatus("disabled") != nil {
		t.Error("expected no status for disabled service")
	}
}

func TestHealthMonitor_ConcurrentAccess(t *testing.T) {
	services := map[string]*Config{
		"test": {
			Enabled: true,
			Image:   "test:latest",
			Port:    8080,
		},
	}

	docker := newMockDockerManager()
	monitor := NewHealthMonitor(services, docker)

	// Concurrent reads and writes
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(3)

		go func() {
			defer wg.Done()
			monitor.markHealthy("test")
		}()

		go func() {
			defer wg.Done()
			monitor.GetStatus("test")
		}()

		go func() {
			defer wg.Done()
			monitor.IsHealthy("test")
		}()
	}

	wg.Wait()

	// Should not panic and should have valid state
	status := monitor.GetStatus("test")
	if status == nil {
		t.Fatal("expected status after concurrent access")
	}
}

func TestHealthMonitor_StatusCopy(t *testing.T) {
	services := map[string]*Config{
		"test": {
			Enabled: true,
			Image:   "test:latest",
			Port:    8080,
		},
	}

	docker := newMockDockerManager()
	monitor := NewHealthMonitor(services, docker)

	monitor.markHealthy("test")

	// Get status
	status1 := monitor.GetStatus("test")
	status2 := monitor.GetStatus("test")

	// Should be different pointers (copies)
	if status1 == status2 {
		t.Error("expected different pointers for status copies")
	}

	// But same values
	if status1.Healthy != status2.Healthy {
		t.Error("expected same Healthy value")
	}
}
