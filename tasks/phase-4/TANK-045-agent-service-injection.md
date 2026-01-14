---
id: TANK-045
title: Agent Service Injection
status: todo
priority: medium
estimate: M
depends_on: []
workstream: B
phase: 4
---

# Agent Service Injection

## Summary

Automatically inject service connection information into agent containers as environment variables.
This allows agents to connect to shared services (Postgres, Redis, etc.) without manual
configuration.

**Interface-based:** This task defines a `ServiceManager` interface for retrieving connection info.
It does not depend on concrete implementations from Workstream A. Integration with actual
implementations happens during the integration phase.

## Acceptance Criteria

- [ ] Connection info set as environment variables in agent containers
- [ ] Variables follow naming convention (SERVICE_HOST, SERVICE_PORT, etc.)
- [ ] Only inject for enabled and running services
- [ ] Warning if agent spawned with services not running
- [ ] Support custom environment variable prefixes
- [ ] Document available variables in agent CLAUDE.md

## Technical Details

### Environment Variable Convention

```bash
# Pattern: <SERVICE>_<FIELD>
# All uppercase, underscores for separators

# Postgres
POSTGRES_HOST=tanuki-svc-postgres
POSTGRES_PORT=5432
POSTGRES_URL=tanuki-svc-postgres:5432
POSTGRES_USER=tanuki
POSTGRES_PASSWORD=tanuki
POSTGRES_DATABASE=tanuki_dev
POSTGRES_DSN=postgres://tanuki:tanuki@tanuki-svc-postgres:5432/tanuki_dev

# Redis
REDIS_HOST=tanuki-svc-redis
REDIS_PORT=6379
REDIS_URL=redis://tanuki-svc-redis:6379

# Custom service (elasticsearch)
ELASTICSEARCH_HOST=tanuki-svc-elasticsearch
ELASTICSEARCH_PORT=9200
ELASTICSEARCH_URL=tanuki-svc-elasticsearch:9200
```

### ServiceManager Integration

```go
type ServiceConnection struct {
    Host     string
    Port     int
    URL      string
    Username string
    Password string
    Database string
    DSN      string // Full connection string if applicable
}

func (m *ServiceManager) GetConnectionInfo(name string) (*ServiceConnection, error) {
    svc, ok := m.config.Services[name]
    if !ok || !svc.Enabled {
        return nil, fmt.Errorf("service %s not configured or not enabled", name)
    }

    containerName := fmt.Sprintf("tanuki-svc-%s", name)
    host := containerName // Docker DNS

    conn := &ServiceConnection{
        Host: host,
        Port: svc.Port,
        URL:  fmt.Sprintf("%s:%d", host, svc.Port),
    }

    // Service-specific fields
    switch name {
    case "postgres":
        conn.Username = svc.Environment["POSTGRES_USER"]
        conn.Password = svc.Environment["POSTGRES_PASSWORD"]
        conn.Database = svc.Environment["POSTGRES_DB"]
        conn.DSN = fmt.Sprintf("postgres://%s:%s@%s:%d/%s",
            conn.Username, conn.Password, host, svc.Port, conn.Database)
    case "redis":
        conn.URL = fmt.Sprintf("redis://%s:%d", host, svc.Port)
    }

    return conn, nil
}

func (m *ServiceManager) GetAllConnections() map[string]*ServiceConnection {
    conns := make(map[string]*ServiceConnection)
    for name, svc := range m.config.Services {
        if !svc.Enabled {
            continue
        }
        if conn, err := m.GetConnectionInfo(name); err == nil {
            conns[name] = conn
        }
    }
    return conns
}
```

### DockerManager Integration

```go
func (d *DockerManager) CreateAgentContainer(name string, worktreePath string, opts AgentOptions) (string, error) {
    // Build base environment
    env := []string{
        fmt.Sprintf("TANUKI_AGENT=%s", name),
        fmt.Sprintf("TANUKI_WORKTREE=%s", worktreePath),
    }

    // Inject service connections
    if d.serviceManager != nil {
        serviceEnv := d.buildServiceEnvironment()
        env = append(env, serviceEnv...)
    }

    // ... rest of container creation
}

func (d *DockerManager) buildServiceEnvironment() []string {
    var env []string

    conns := d.serviceManager.GetAllConnections()
    for name, conn := range conns {
        prefix := strings.ToUpper(name)

        env = append(env, fmt.Sprintf("%s_HOST=%s", prefix, conn.Host))
        env = append(env, fmt.Sprintf("%s_PORT=%d", prefix, conn.Port))
        env = append(env, fmt.Sprintf("%s_URL=%s", prefix, conn.URL))

        if conn.Username != "" {
            env = append(env, fmt.Sprintf("%s_USER=%s", prefix, conn.Username))
        }
        if conn.Password != "" {
            env = append(env, fmt.Sprintf("%s_PASSWORD=%s", prefix, conn.Password))
        }
        if conn.Database != "" {
            env = append(env, fmt.Sprintf("%s_DATABASE=%s", prefix, conn.Database))
        }
        if conn.DSN != "" {
            env = append(env, fmt.Sprintf("%s_DSN=%s", prefix, conn.DSN))
        }
    }

    return env
}
```

### Service Health Warning

```go
func (d *DockerManager) CreateAgentContainer(name string, worktreePath string, opts AgentOptions) (string, error) {
    // Check service health before spawning
    if d.serviceManager != nil {
        for svcName := range d.serviceManager.GetAllConnections() {
            if !d.serviceManager.IsHealthy(svcName) {
                log.Printf("Warning: service %s is not healthy, agent may not be able to connect", svcName)
            }
        }
    }

    // ... continue with container creation
}
```

### CLAUDE.md Documentation

Automatically append service info to agent's CLAUDE.md:

```go
func (a *AgentManager) generateCLAUDEmd(agent *Agent) string {
    var sb strings.Builder

    // ... existing content

    // Add service documentation if services are configured
    if len(a.serviceManager.GetAllConnections()) > 0 {
        sb.WriteString("\n## Available Services\n\n")
        sb.WriteString("The following services are available via environment variables:\n\n")

        for name, conn := range a.serviceManager.GetAllConnections() {
            prefix := strings.ToUpper(name)
            sb.WriteString(fmt.Sprintf("### %s\n\n", name))
            sb.WriteString(fmt.Sprintf("- Host: `$%s_HOST` (%s)\n", prefix, conn.Host))
            sb.WriteString(fmt.Sprintf("- Port: `$%s_PORT` (%d)\n", prefix, conn.Port))
            if conn.DSN != "" {
                sb.WriteString(fmt.Sprintf("- DSN: `$%s_DSN`\n", prefix))
            }
            sb.WriteString("\n")
        }
    }

    return sb.String()
}
```

### Example Agent Environment

When an agent is spawned, it sees:

```bash
# Agent-specific
TANUKI_AGENT=api-feature
TANUKI_WORKTREE=/workspace

# Postgres (from services)
POSTGRES_HOST=tanuki-svc-postgres
POSTGRES_PORT=5432
POSTGRES_URL=tanuki-svc-postgres:5432
POSTGRES_USER=tanuki
POSTGRES_PASSWORD=tanuki
POSTGRES_DATABASE=tanuki_dev
POSTGRES_DSN=postgres://tanuki:tanuki@tanuki-svc-postgres:5432/tanuki_dev

# Redis (from services)
REDIS_HOST=tanuki-svc-redis
REDIS_PORT=6379
REDIS_URL=redis://tanuki-svc-redis:6379
```

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Service not running | Warning log, still inject variables |
| Service not healthy | Warning log, still inject variables |
| No services configured | Skip injection, no warning |
| Service config invalid | Skip that service, log error |

## Testing

- Unit tests for environment variable building
- Test connection info generation for each service type
- Test warning on unhealthy services
- Integration test: spawn agent, verify env vars present
- Test CLAUDE.md includes service documentation

## Files to Create/Modify

- `internal/service/connection.go` - ServiceConnection type and builders
- `internal/docker/docker.go` - Add service injection to CreateAgentContainer
- `internal/agent/claude.go` - Add service docs to CLAUDE.md generation

## Notes

Passwords are passed as plain text environment variables. This is acceptable for development services. Production deployments should use secrets management. Document this limitation.
