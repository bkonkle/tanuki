---
id: TANK-005
title: Docker Container Manager
status: done
priority: high
estimate: L
depends_on: [TANK-001]
workstream: B
phase: 1
---

# Docker Container Manager

## Summary

Implement Docker container operations for creating, managing, and executing commands in agent containers. Each agent runs in its own isolated container with the worktree bind-mounted.

## Acceptance Criteria

- [x] Create container with worktree bind mount at `/workspace`
- [x] Mount Claude Code auth directory read-only
- [x] Create shared Docker network (`tanuki-net`)
- [x] Start/stop containers
- [x] Execute commands in containers (`docker exec`)
- [x] Stream container logs
- [x] Remove containers on cleanup
- [x] Apply resource limits (memory, CPU)

## Technical Details

### Container Configuration

```go
type ContainerConfig struct {
    Name         string
    Image        string
    WorktreePath string            // Host path to bind mount
    WorkDir      string            // Container working directory
    Env          map[string]string
    Mounts       []Mount
    Network      string
    Resources    ResourceLimits
}

type Mount struct {
    Source   string
    Target   string
    ReadOnly bool
}

type ResourceLimits struct {
    Memory string // e.g., "4g"
    CPUs   string // e.g., "2"
}
```

### Docker Manager Interface

```go
type DockerManager interface {
    // Network operations
    EnsureNetwork(name string) error

    // Container lifecycle
    CreateContainer(config ContainerConfig) (containerID string, err error)
    StartContainer(containerID string) error
    StopContainer(containerID string) error
    RemoveContainer(containerID string) error

    // Container interaction
    Exec(containerID string, cmd []string, opts ExecOptions) error
    ExecWithOutput(containerID string, cmd []string) (string, error)
    StreamLogs(containerID string, follow bool) (io.ReadCloser, error)

    // Status
    ContainerExists(containerID string) bool
    ContainerRunning(containerID string) bool
    InspectContainer(containerID string) (*ContainerInfo, error)
}

type ExecOptions struct {
    Stdin       io.Reader
    Stdout      io.Writer
    Stderr      io.Writer
    TTY         bool
    Interactive bool
}
```

### Container Creation

```go
func (d *DockerManager) CreateAgentContainer(name string, worktreePath string) (string, error) {
    absWorktree, _ := filepath.Abs(worktreePath)
    homeDir, _ := os.UserHomeDir()

    config := ContainerConfig{
        Name:    fmt.Sprintf("tanuki-%s", name),
        Image:   d.config.Image.Name + ":" + d.config.Image.Tag,
        WorkDir: "/workspace",
        Mounts: []Mount{
            {
                Source:   absWorktree,
                Target:   "/workspace",
                ReadOnly: false,
            },
            {
                Source:   filepath.Join(homeDir, ".config", "claude-code"),
                Target:   "/home/dev/.config/claude-code",
                ReadOnly: true,
            },
        },
        Network: "tanuki-net",
        Resources: ResourceLimits{
            Memory: d.config.Defaults.Resources.Memory,
            CPUs:   d.config.Defaults.Resources.CPUs,
        },
        Env: map[string]string{
            "TANUKI_AGENT": name,
        },
    }

    return d.CreateContainer(config)
}
```

### Docker CLI Commands

Using Docker CLI directly (more reliable than Docker SDK for this use case):

```go
func (d *DockerManager) CreateContainer(config ContainerConfig) (string, error) {
    args := []string{
        "create",
        "--name", config.Name,
        "--workdir", config.WorkDir,
        "--network", config.Network,
    }

    // Add mounts
    for _, mount := range config.Mounts {
        mountStr := fmt.Sprintf("%s:%s", mount.Source, mount.Target)
        if mount.ReadOnly {
            mountStr += ":ro"
        }
        args = append(args, "-v", mountStr)
    }

    // Add resource limits
    if config.Resources.Memory != "" {
        args = append(args, "--memory", config.Resources.Memory)
    }
    if config.Resources.CPUs != "" {
        args = append(args, "--cpus", config.Resources.CPUs)
    }

    // Add environment variables
    for k, v := range config.Env {
        args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
    }

    // Image and command
    args = append(args, config.Image, "sleep", "infinity")

    cmd := exec.Command("docker", args...)
    output, err := cmd.Output()
    if err != nil {
        return "", fmt.Errorf("failed to create container: %w", err)
    }

    return strings.TrimSpace(string(output)), nil
}
```

### Network Setup

```go
func (d *DockerManager) EnsureNetwork(name string) error {
    // Check if network exists
    cmd := exec.Command("docker", "network", "inspect", name)
    if err := cmd.Run(); err == nil {
        return nil // Network exists
    }

    // Create network
    cmd = exec.Command("docker", "network", "create", name)
    return cmd.Run()
}
```

### Log Streaming

```go
func (d *DockerManager) StreamLogs(containerID string, follow bool) (io.ReadCloser, error) {
    args := []string{"logs"}
    if follow {
        args = append(args, "-f")
    }
    args = append(args, containerID)

    cmd := exec.Command("docker", args...)
    stdout, err := cmd.StdoutPipe()
    if err != nil {
        return nil, err
    }

    if err := cmd.Start(); err != nil {
        return nil, err
    }

    return stdout, nil
}
```

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Docker not running | Clear error: "Docker daemon not running" |
| Image not found | Offer to pull or build |
| Container name conflict | Use unique suffix or error |
| Network creation fails | Detailed error with troubleshooting |
| Mount path doesn't exist | Create directory or error |

## Out of Scope

- Building Docker images (separate task)
- Docker Compose integration (Phase 4)
- Container health checks

## Notes

Use Docker CLI rather than Docker SDK - simpler, more debuggable, and matches what users would run manually. The SDK adds complexity without significant benefit for this use case.
