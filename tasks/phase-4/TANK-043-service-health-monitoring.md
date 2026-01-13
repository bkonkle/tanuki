---
id: TANK-043
title: Service Health Monitoring
status: todo
priority: medium
estimate: M
depends_on: [TANK-040]
workstream: A
phase: 4
---

# Service Health Monitoring

## Summary

Implement health monitoring with automatic recovery for shared services. The health monitor runs in a background goroutine and periodically checks service health, restarting unhealthy services when needed.

## Acceptance Criteria

- [ ] Periodic health check loop
- [ ] Service-specific health commands (pg_isready, redis-cli ping)
- [ ] Configurable check intervals and retry limits
- [ ] Automatic restart on unhealthy status
- [ ] Health status exposed via ServiceManager
- [ ] Grace period before marking unhealthy
- [ ] Logging of health state changes

## Technical Details

### Health Monitor

```go
type HealthMonitor struct {
    services      map[string]*ServiceConfig
    docker        *DockerManager
    status        map[string]*HealthStatus
    mu            sync.RWMutex
    stopCh        chan struct{}
    checkInterval time.Duration
}

type HealthStatus struct {
    Healthy       bool
    LastCheck     time.Time
    LastHealthy   time.Time
    FailureCount  int
    Error         string
}

func NewHealthMonitor(services map[string]*ServiceConfig, docker *DockerManager) *HealthMonitor {
    return &HealthMonitor{
        services:      services,
        docker:        docker,
        status:        make(map[string]*HealthStatus),
        checkInterval: 10 * time.Second,
    }
}
```

### Health Check Loop

```go
func (m *HealthMonitor) Start(ctx context.Context) {
    m.stopCh = make(chan struct{})
    ticker := time.NewTicker(m.checkInterval)
    defer ticker.Stop()

    // Initial check
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

func (m *HealthMonitor) Stop() {
    close(m.stopCh)
}

func (m *HealthMonitor) checkAllServices() {
    for name, svc := range m.services {
        if !svc.Enabled {
            continue
        }
        m.checkService(name, svc)
    }
}
```

### Service-Specific Health Checks

```go
func (m *HealthMonitor) checkService(name string, svc *ServiceConfig) {
    containerName := fmt.Sprintf("tanuki-svc-%s", name)

    // Check if container is running
    if !m.docker.ContainerRunning(containerName) {
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
        m.maybeRestart(name)
        return
    }

    if healthy {
        m.markHealthy(name)
    } else {
        m.markUnhealthy(name, "health check failed")
        m.maybeRestart(name)
    }
}

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

func (m *HealthMonitor) checkPostgres(container string) (bool, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    cmd := exec.CommandContext(ctx, "docker", "exec", container,
        "pg_isready", "-U", "tanuki")
    err := cmd.Run()
    return err == nil, err
}

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
```

### Automatic Restart

```go
func (m *HealthMonitor) maybeRestart(name string) {
    m.mu.Lock()
    defer m.mu.Unlock()

    status := m.status[name]
    svc := m.services[name]

    maxRetries := 3
    if svc.Healthcheck != nil {
        maxRetries = svc.Healthcheck.Retries
    }

    if status.FailureCount >= maxRetries {
        log.Printf("Service %s unhealthy after %d checks, restarting...", name, status.FailureCount)

        containerName := fmt.Sprintf("tanuki-svc-%s", name)
        if err := m.docker.RestartContainer(containerName); err != nil {
            log.Printf("Failed to restart %s: %v", name, err)
            return
        }

        // Reset failure count, give grace period
        status.FailureCount = 0
        log.Printf("Restarted service %s", name)
    }
}
```

### Status Access

```go
func (m *HealthMonitor) GetStatus(name string) *HealthStatus {
    m.mu.RLock()
    defer m.mu.RUnlock()
    if status, ok := m.status[name]; ok {
        return status
    }
    return nil
}

func (m *HealthMonitor) IsHealthy(name string) bool {
    status := m.GetStatus(name)
    return status != nil && status.Healthy
}

func (m *HealthMonitor) GetAllStatus() map[string]*HealthStatus {
    m.mu.RLock()
    defer m.mu.RUnlock()

    result := make(map[string]*HealthStatus)
    for k, v := range m.status {
        result[k] = v
    }
    return result
}
```

### Integration with ServiceManager

```go
type ServiceManager struct {
    config        *config.Config
    docker        *DockerManager
    healthMonitor *HealthMonitor
}

func (m *ServiceManager) StartServices() error {
    // ... start containers

    // Start health monitoring
    go m.healthMonitor.Start(context.Background())

    return nil
}

func (m *ServiceManager) StopServices() error {
    // Stop health monitoring
    m.healthMonitor.Stop()

    // ... stop containers
}

func (m *ServiceManager) IsHealthy(name string) bool {
    return m.healthMonitor.IsHealthy(name)
}
```

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Container not running | Mark unhealthy, attempt restart |
| Health check timeout | Mark unhealthy, increment failure count |
| Restart fails | Log error, continue monitoring |
| Max retries exceeded | Stop restarting, log warning |

## Testing

- Unit tests for health check logic
- Test failure counting and restart trigger
- Test grace period after restart
- Integration test with actual containers
- Test built-in checks for postgres/redis

## Files to Create/Modify

- `internal/service/health.go` - HealthMonitor implementation
- `internal/service/manager.go` - Integrate health monitor
- `internal/docker/docker.go` - Add RestartContainer method

## Notes

Health monitoring should be lightweight and not impact performance. The default check interval of 10 seconds balances responsiveness with resource usage.
