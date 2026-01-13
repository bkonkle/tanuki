---
id: TANK-040
title: Service Manager Core
status: todo
priority: medium
estimate: L
depends_on: [TANK-005]
workstream: A
phase: 4
---

# Service Manager Core

## Summary

Implement the core ServiceManager that handles lifecycle operations for shared services like Postgres and Redis. Services run in containers on the same Docker network (tanuki-net) and can be accessed by agent containers.

## Acceptance Criteria

- [ ] Service configuration in tanuki.yaml
- [ ] Services start/stop with project
- [ ] Connection info injected into agent containers
- [ ] Data persisted in volumes
- [ ] Service health checks

## Technical Details

### Configuration

```yaml
# tanuki.yaml
services:
  postgres:
    enabled: true
    image: postgres:16
    port: 5432
    environment:
      POSTGRES_USER: tanuki
      POSTGRES_PASSWORD: tanuki
      POSTGRES_DB: tanuki_dev
    volumes:
      - tanuki-postgres:/var/lib/postgresql/data

  redis:
    enabled: true
    image: redis:7
    port: 6379
    volumes:
      - tanuki-redis:/data

  # Custom service example
  elasticsearch:
    enabled: false
    image: elasticsearch:8.12.0
    port: 9200
    environment:
      discovery.type: single-node
```

### Service Manager

```go
type ServiceManager interface {
    // Start all enabled services
    StartServices() error

    // Stop all services
    StopServices() error

    // Get connection info for a service
    GetConnectionInfo(name string) (*ServiceConnection, error)

    // Check service health
    HealthCheck(name string) (bool, error)
}

type ServiceConnection struct {
    Host     string
    Port     int
    URL      string
    Username string
    Password string
}
```

### Implementation

```go
func (m *ServiceManager) StartServices() error {
    for name, svc := range m.config.Services {
        if !svc.Enabled {
            continue
        }

        containerName := fmt.Sprintf("tanuki-svc-%s", name)

        // Check if already running
        if m.docker.ContainerRunning(containerName) {
            continue
        }

        // Build run args
        args := []string{
            "run", "-d",
            "--name", containerName,
            "--network", "tanuki-net",
        }

        // Add port mapping (for host access during development)
        args = append(args, "-p", fmt.Sprintf("%d:%d", svc.Port, svc.Port))

        // Add environment variables
        for k, v := range svc.Environment {
            args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
        }

        // Add volumes
        for _, vol := range svc.Volumes {
            args = append(args, "-v", vol)
        }

        args = append(args, svc.Image)

        cmd := exec.Command("docker", args...)
        if err := cmd.Run(); err != nil {
            return fmt.Errorf("failed to start %s: %w", name, err)
        }

        // Wait for health
        if err := m.waitForHealth(name); err != nil {
            log.Printf("Warning: %s health check failed: %v", name, err)
        }
    }

    return nil
}

func (m *ServiceManager) GetConnectionInfo(name string) (*ServiceConnection, error) {
    svc, ok := m.config.Services[name]
    if !ok || !svc.Enabled {
        return nil, fmt.Errorf("service %s not configured", name)
    }

    containerName := fmt.Sprintf("tanuki-svc-%s", name)

    return &ServiceConnection{
        Host:     containerName, // Docker DNS
        Port:     svc.Port,
        URL:      fmt.Sprintf("%s:%d", containerName, svc.Port),
        Username: svc.Environment["POSTGRES_USER"],
        Password: svc.Environment["POSTGRES_PASSWORD"],
    }, nil
}
```

### Agent Container Integration

Inject service connection info as environment variables:

```go
func (d *DockerManager) CreateAgentContainer(name string, worktreePath string) (string, error) {
    // ... existing setup

    // Add service connection info
    for svcName, conn := range m.serviceManager.GetAllConnections() {
        envPrefix := strings.ToUpper(svcName)
        config.Env[envPrefix+"_HOST"] = conn.Host
        config.Env[envPrefix+"_PORT"] = strconv.Itoa(conn.Port)
        config.Env[envPrefix+"_URL"] = conn.URL
        if conn.Username != "" {
            config.Env[envPrefix+"_USER"] = conn.Username
        }
        if conn.Password != "" {
            config.Env[envPrefix+"_PASSWORD"] = conn.Password
        }
    }

    // ...
}
```

### Agent Environment

Agents see these environment variables:

```bash
POSTGRES_HOST=tanuki-svc-postgres
POSTGRES_PORT=5432
POSTGRES_URL=tanuki-svc-postgres:5432
POSTGRES_USER=tanuki
POSTGRES_PASSWORD=tanuki

REDIS_HOST=tanuki-svc-redis
REDIS_PORT=6379
REDIS_URL=tanuki-svc-redis:6379
```

### Health Checks

```go
func (m *ServiceManager) waitForHealth(name string) error {
    containerName := fmt.Sprintf("tanuki-svc-%s", name)

    // Service-specific health checks
    switch name {
    case "postgres":
        return m.waitForPostgres(containerName)
    case "redis":
        return m.waitForRedis(containerName)
    default:
        // Generic: just wait for container to be running
        time.Sleep(5 * time.Second)
        return nil
    }
}

func (m *ServiceManager) waitForPostgres(container string) error {
    for i := 0; i < 30; i++ {
        cmd := exec.Command("docker", "exec", container,
            "pg_isready", "-U", "tanuki")
        if err := cmd.Run(); err == nil {
            return nil
        }
        time.Sleep(time.Second)
    }
    return fmt.Errorf("postgres not ready after 30s")
}
```

### CLI Commands

```bash
# Start services (called by project start)
tanuki service start

# Stop services
tanuki service stop

# Show service status
tanuki service status

# Connect to service (for debugging)
tanuki service connect postgres  # Opens psql
tanuki service connect redis     # Opens redis-cli
```

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Service fails to start | Error with container logs |
| Health check timeout | Warning, continue |
| Port conflict | Error with suggestion |
| Volume permission error | Error with fix suggestion |

## Out of Scope

- Service scaling (multiple replicas)
- Service dependencies
- Custom health check commands

## Notes

Services are meant for development, not production. Keep configuration simple and focused on the happy path.
