---
id: TANK-042
title: Service Configuration Schema
status: todo
priority: medium
estimate: M
depends_on: [TANK-040]
workstream: A
phase: 4
---

# Service Configuration Schema

## Summary

Define and validate the YAML configuration schema for shared services in tanuki.yaml. This includes support for common services like Postgres and Redis, as well as custom service definitions.

## Acceptance Criteria

- [ ] Service config section in tanuki.yaml
- [ ] Support for image, ports, environment, volumes
- [ ] Support for healthcheck configuration
- [ ] Validation of service configuration on load
- [ ] Default service templates for postgres and redis
- [ ] Clear error messages for invalid configuration

## Technical Details

### Configuration Schema

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
    healthcheck:
      command: ["pg_isready", "-U", "tanuki"]
      interval: 5s
      timeout: 3s
      retries: 5

  redis:
    enabled: true
    image: redis:7-alpine
    port: 6379
    volumes:
      - tanuki-redis:/data
    healthcheck:
      command: ["redis-cli", "ping"]
      interval: 5s
      timeout: 3s
      retries: 3

  # Custom service example
  elasticsearch:
    enabled: false
    image: elasticsearch:8.12.0
    port: 9200
    environment:
      discovery.type: single-node
      ES_JAVA_OPTS: "-Xms512m -Xmx512m"
```

### Go Types

```go
type ServiceConfig struct {
    Enabled     bool              `yaml:"enabled"`
    Image       string            `yaml:"image"`
    Port        int               `yaml:"port"`
    Environment map[string]string `yaml:"environment"`
    Volumes     []string          `yaml:"volumes"`
    Healthcheck *HealthcheckConfig `yaml:"healthcheck,omitempty"`
}

type HealthcheckConfig struct {
    Command  []string      `yaml:"command"`
    Interval time.Duration `yaml:"interval"`
    Timeout  time.Duration `yaml:"timeout"`
    Retries  int           `yaml:"retries"`
}

// In main config
type Config struct {
    // ... existing fields
    Services map[string]*ServiceConfig `yaml:"services"`
}
```

### Validation

```go
func (c *ServiceConfig) Validate(name string) error {
    if c.Image == "" {
        return fmt.Errorf("service %s: image is required", name)
    }
    if c.Port <= 0 || c.Port > 65535 {
        return fmt.Errorf("service %s: invalid port %d", name, c.Port)
    }
    // Validate volume format
    for _, vol := range c.Volumes {
        if !isValidVolumeSpec(vol) {
            return fmt.Errorf("service %s: invalid volume spec %q", name, vol)
        }
    }
    return nil
}
```

### Default Templates

Provide convenience functions for common services:

```go
func DefaultPostgresConfig() *ServiceConfig {
    return &ServiceConfig{
        Enabled: true,
        Image:   "postgres:16",
        Port:    5432,
        Environment: map[string]string{
            "POSTGRES_USER":     "tanuki",
            "POSTGRES_PASSWORD": "tanuki",
            "POSTGRES_DB":       "tanuki_dev",
        },
        Volumes: []string{"tanuki-postgres:/var/lib/postgresql/data"},
        Healthcheck: &HealthcheckConfig{
            Command:  []string{"pg_isready", "-U", "tanuki"},
            Interval: 5 * time.Second,
            Timeout:  3 * time.Second,
            Retries:  5,
        },
    }
}

func DefaultRedisConfig() *ServiceConfig {
    return &ServiceConfig{
        Enabled: true,
        Image:   "redis:7-alpine",
        Port:    6379,
        Volumes: []string{"tanuki-redis:/data"},
        Healthcheck: &HealthcheckConfig{
            Command:  []string{"redis-cli", "ping"},
            Interval: 5 * time.Second,
            Timeout:  3 * time.Second,
            Retries:  3,
        },
    }
}
```

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Missing image | Error: "service X: image is required" |
| Invalid port | Error: "service X: invalid port Y" |
| Invalid volume format | Error: "service X: invalid volume spec" |
| Unknown service in enable | Warning, ignore |

## Testing

- Unit tests for validation logic
- Test default templates
- Test YAML parsing with various configurations
- Test error messages are clear and actionable

## Files to Create/Modify

- `internal/config/service.go` - ServiceConfig types
- `internal/config/config.go` - Add Services field
- `internal/config/defaults.go` - Default service templates

## Notes

Keep configuration simple. Users who need advanced Docker features should use docker-compose directly. Tanuki services are for convenience, not flexibility.
