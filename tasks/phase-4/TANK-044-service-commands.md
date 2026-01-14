---
id: TANK-044
title: Service Commands
status: todo
priority: medium
estimate: S
depends_on: []
workstream: B
phase: 4
---

# Service Commands

## Summary

Add CLI commands for managing shared services. Commands allow starting, stopping, checking status,
viewing logs, and connecting to services interactively.

**Interface-based:** This task defines a `ServiceManager` interface that it codes against. It does
not depend on concrete implementations from Workstream A. Integration with actual implementations
happens during the integration phase.

## Acceptance Criteria

- [ ] `tanuki service start [name]` - Start all or specific service
- [ ] `tanuki service stop [name]` - Stop all or specific service
- [ ] `tanuki service status` - Show service status table
- [ ] `tanuki service logs <name>` - Stream service logs
- [ ] `tanuki service connect <name>` - Open interactive connection
- [ ] Status shows health, uptime, port mappings
- [ ] Connect command opens appropriate client tool

## Technical Details

### Command Structure

```go
var serviceCmd = &cobra.Command{
    Use:   "service",
    Short: "Manage shared services",
    Long:  "Start, stop, and manage shared services like Postgres and Redis",
}

func init() {
    serviceCmd.AddCommand(serviceStartCmd)
    serviceCmd.AddCommand(serviceStopCmd)
    serviceCmd.AddCommand(serviceStatusCmd)
    serviceCmd.AddCommand(serviceLogsCmd)
    serviceCmd.AddCommand(serviceConnectCmd)
    rootCmd.AddCommand(serviceCmd)
}
```

### Start Command

```go
var serviceStartCmd = &cobra.Command{
    Use:   "start [name]",
    Short: "Start services",
    Long:  "Start all enabled services or a specific service by name",
    RunE:  runServiceStart,
}

func runServiceStart(cmd *cobra.Command, args []string) error {
    cfg, err := config.Load()
    if err != nil {
        return err
    }

    svcMgr := service.NewManager(cfg)

    if len(args) == 0 {
        // Start all enabled services
        fmt.Println("Starting services...")
        if err := svcMgr.StartServices(); err != nil {
            return err
        }

        // Print status
        for name, status := range svcMgr.GetAllStatus() {
            icon := "✓"
            if !status.Running {
                icon = "✗"
            }
            fmt.Printf("  %s %s (%s:%d)\n", icon, name, status.Host, status.Port)
        }
    } else {
        // Start specific service
        name := args[0]
        fmt.Printf("Starting %s...\n", name)
        if err := svcMgr.StartService(name); err != nil {
            return err
        }
        fmt.Printf("  ✓ %s started\n", name)
    }

    return nil
}
```

### Stop Command

```go
var serviceStopCmd = &cobra.Command{
    Use:   "stop [name]",
    Short: "Stop services",
    Long:  "Stop all services or a specific service by name",
    RunE:  runServiceStop,
}

func runServiceStop(cmd *cobra.Command, args []string) error {
    cfg, err := config.Load()
    if err != nil {
        return err
    }

    svcMgr := service.NewManager(cfg)

    if len(args) == 0 {
        fmt.Println("Stopping services...")
        return svcMgr.StopServices()
    }

    name := args[0]
    fmt.Printf("Stopping %s...\n", name)
    return svcMgr.StopService(name)
}
```

### Status Command

```go
var serviceStatusCmd = &cobra.Command{
    Use:   "status",
    Short: "Show service status",
    RunE:  runServiceStatus,
}

func runServiceStatus(cmd *cobra.Command, args []string) error {
    cfg, err := config.Load()
    if err != nil {
        return err
    }

    svcMgr := service.NewManager(cfg)
    statuses := svcMgr.GetAllStatus()

    if len(statuses) == 0 {
        fmt.Println("No services configured")
        return nil
    }

    // Print table header
    fmt.Printf("%-15s %-10s %-10s %-10s %s\n",
        "SERVICE", "STATUS", "HEALTH", "PORT", "UPTIME")
    fmt.Println(strings.Repeat("-", 60))

    for name, status := range statuses {
        statusStr := "stopped"
        if status.Running {
            statusStr = "running"
        }

        healthStr := "-"
        if status.Running {
            if status.Healthy {
                healthStr = "healthy"
            } else {
                healthStr = "unhealthy"
            }
        }

        uptimeStr := "-"
        if status.Running && !status.StartedAt.IsZero() {
            uptimeStr = formatDuration(time.Since(status.StartedAt))
        }

        fmt.Printf("%-15s %-10s %-10s %-10d %s\n",
            name, statusStr, healthStr, status.Port, uptimeStr)
    }

    return nil
}
```

### Logs Command

```go
var serviceLogsCmd = &cobra.Command{
    Use:   "logs <name>",
    Short: "Stream service logs",
    Args:  cobra.ExactArgs(1),
    RunE:  runServiceLogs,
}

func init() {
    serviceLogsCmd.Flags().BoolP("follow", "f", false, "Follow log output")
    serviceLogsCmd.Flags().Int("tail", 100, "Number of lines to show")
}

func runServiceLogs(cmd *cobra.Command, args []string) error {
    name := args[0]
    follow, _ := cmd.Flags().GetBool("follow")
    tail, _ := cmd.Flags().GetInt("tail")

    containerName := fmt.Sprintf("tanuki-svc-%s", name)

    dockerArgs := []string{"logs"}
    if follow {
        dockerArgs = append(dockerArgs, "-f")
    }
    dockerArgs = append(dockerArgs, "--tail", strconv.Itoa(tail))
    dockerArgs = append(dockerArgs, containerName)

    dockerCmd := exec.Command("docker", dockerArgs...)
    dockerCmd.Stdout = os.Stdout
    dockerCmd.Stderr = os.Stderr
    return dockerCmd.Run()
}
```

### Connect Command

```go
var serviceConnectCmd = &cobra.Command{
    Use:   "connect <name>",
    Short: "Connect to a service interactively",
    Long:  "Open an interactive connection to a service (e.g., psql for postgres)",
    Args:  cobra.ExactArgs(1),
    RunE:  runServiceConnect,
}

func runServiceConnect(cmd *cobra.Command, args []string) error {
    name := args[0]

    cfg, err := config.Load()
    if err != nil {
        return err
    }

    svc, ok := cfg.Services[name]
    if !ok {
        return fmt.Errorf("service %s not configured", name)
    }

    containerName := fmt.Sprintf("tanuki-svc-%s", name)

    var connectCmd *exec.Cmd
    switch name {
    case "postgres":
        user := svc.Environment["POSTGRES_USER"]
        db := svc.Environment["POSTGRES_DB"]
        connectCmd = exec.Command("docker", "exec", "-it", containerName,
            "psql", "-U", user, "-d", db)
    case "redis":
        connectCmd = exec.Command("docker", "exec", "-it", containerName,
            "redis-cli")
    default:
        // Generic shell
        connectCmd = exec.Command("docker", "exec", "-it", containerName, "sh")
    }

    connectCmd.Stdin = os.Stdin
    connectCmd.Stdout = os.Stdout
    connectCmd.Stderr = os.Stderr
    return connectCmd.Run()
}
```

### Output Examples

```bash
$ tanuki service start
Starting services...
  ✓ postgres (tanuki-svc-postgres:5432)
  ✓ redis (tanuki-svc-redis:6379)

$ tanuki service status
SERVICE         STATUS     HEALTH     PORT       UPTIME
------------------------------------------------------------
postgres        running    healthy    5432       5m
redis           running    healthy    6379       5m

$ tanuki service logs postgres -f --tail 50
2026-01-13 10:00:00.000 UTC [1] LOG:  database system is ready
...

$ tanuki service connect postgres
psql (16.0)
tanuki_dev=>
```

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Service not configured | Error: "service X not configured" |
| Service not running | Error: "service X is not running" |
| Docker not available | Error: "docker is not running" |
| Connect tool not found | Fallback to shell |

## Testing

- Test all command variations
- Test with services running/stopped
- Test status output formatting
- Test logs follow mode
- Test connect for postgres/redis

## Files to Create/Modify

- `cmd/service.go` - Service command group
- `cmd/service_start.go` - Start command
- `cmd/service_stop.go` - Stop command
- `cmd/service_status.go` - Status command
- `cmd/service_logs.go` - Logs command
- `cmd/service_connect.go` - Connect command

## Notes

Keep connect commands simple. Users who need advanced features can use docker exec directly. The goal is quick access for common operations.
