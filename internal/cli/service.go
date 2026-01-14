package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/bkonkle/tanuki/internal/service"
	"github.com/spf13/cobra"
)

var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "Manage shared services",
	Long: `Manage shared services like Postgres and Redis.

Services are Docker containers running infrastructure that agents can connect to.
Connection information is automatically injected into agent containers.

Commands:
  start   - Start all or specific services
  stop    - Stop all or specific services
  status  - Show service status
  logs    - Stream service logs
  connect - Open interactive connection`,
}

var serviceStartCmd = &cobra.Command{
	Use:   "start [name]",
	Short: "Start services",
	Long: `Start all enabled services or a specific service by name.

Examples:
  tanuki service start           # Start all enabled services
  tanuki service start postgres  # Start only postgres`,
	Args: cobra.MaximumNArgs(1),
	RunE: runServiceStart,
}

var serviceStopCmd = &cobra.Command{
	Use:   "stop [name]",
	Short: "Stop services",
	Long: `Stop all services or a specific service by name.

Examples:
  tanuki service stop           # Stop all services
  tanuki service stop postgres  # Stop only postgres`,
	Args: cobra.MaximumNArgs(1),
	RunE: runServiceStop,
}

var serviceStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show service status",
	Long: `Display the status of all configured services.

Shows:
  - Service name
  - Running status
  - Health status
  - Port mapping
  - Uptime`,
	RunE: runServiceStatus,
}

var serviceLogsCmd = &cobra.Command{
	Use:   "logs <name>",
	Short: "Stream service logs",
	Long: `Stream logs from a service container.

Examples:
  tanuki service logs postgres
  tanuki service logs postgres -f         # Follow log output
  tanuki service logs postgres --tail 50  # Show last 50 lines`,
	Args: cobra.ExactArgs(1),
	RunE: runServiceLogs,
}

var serviceConnectCmd = &cobra.Command{
	Use:   "connect <name>",
	Short: "Connect to a service interactively",
	Long: `Open an interactive connection to a service.

For databases, this opens the appropriate client tool:
  - postgres: psql
  - redis: redis-cli
  - other: sh

Examples:
  tanuki service connect postgres  # Opens psql
  tanuki service connect redis     # Opens redis-cli`,
	Args: cobra.ExactArgs(1),
	RunE: runServiceConnect,
}

var (
	serviceLogsFollow bool
	serviceLogsTail   int
)

func init() {
	serviceLogsCmd.Flags().BoolVarP(&serviceLogsFollow, "follow", "f", false, "Follow log output")
	serviceLogsCmd.Flags().IntVar(&serviceLogsTail, "tail", 100, "Number of lines to show")

	serviceCmd.AddCommand(serviceStartCmd)
	serviceCmd.AddCommand(serviceStopCmd)
	serviceCmd.AddCommand(serviceStatusCmd)
	serviceCmd.AddCommand(serviceLogsCmd)
	serviceCmd.AddCommand(serviceConnectCmd)

	rootCmd.AddCommand(serviceCmd)
}

// getServiceManager creates a service manager from the configuration.
// This returns a stub manager until the full implementation is available.
func getServiceManager() (*service.StubManager, error) {
	// For now, use default service configurations
	// TODO: Load from tanuki.yaml once service config is integrated
	services := map[string]*service.Config{
		"postgres": service.DefaultPostgresConfig(),
		"redis":    service.DefaultRedisConfig(),
	}

	// Enable services that have running containers
	for name := range services {
		containerName := service.ContainerName(name)
		cmd := exec.Command("docker", "container", "inspect", containerName) //nolint:gosec // G204: Container name is from internal config
		if cmd.Run() == nil {
			services[name].Enabled = true
		}
	}

	return service.NewStubManager(services, "tanuki-net"), nil
}

func runServiceStart(_ *cobra.Command, args []string) error {
	svcMgr, err := getServiceManager()
	if err != nil {
		return err
	}

	if len(args) == 0 {
		// Start all enabled services
		fmt.Println("Starting services...")

		// Enable all services for startup
		for name := range svcMgr.Services {
			svcMgr.Services[name].Enabled = true
		}

		if startErr := svcMgr.StartServices(); startErr != nil {
			return startErr
		}

		// Print status
		for name, status := range svcMgr.GetAllStatus() {
			icon := "✓"
			if !status.Running {
				icon = "✗"
			}
			fmt.Printf("  %s %s (%s:%d)\n", icon, name, status.ContainerName, status.Port)
		}
		return nil
	}

	// Start specific service
	name := args[0]
	cfg, err := svcMgr.GetConfig(name)
	if err != nil {
		return fmt.Errorf("service %q not configured", name)
	}
	cfg.Enabled = true

	fmt.Printf("Starting %s...\n", name)
	if err := svcMgr.StartService(name); err != nil {
		return err
	}

	status, _ := svcMgr.GetStatus(name)
	if status != nil && status.Running {
		fmt.Printf("  ✓ %s started (%s:%d)\n", name, status.ContainerName, status.Port)
	}

	return nil
}

func runServiceStop(_ *cobra.Command, args []string) error {
	svcMgr, err := getServiceManager()
	if err != nil {
		return err
	}

	if len(args) == 0 {
		fmt.Println("Stopping services...")
		if err := svcMgr.StopServices(); err != nil {
			return err
		}
		fmt.Println("  ✓ All services stopped")
		return nil
	}

	name := args[0]
	fmt.Printf("Stopping %s...\n", name)
	if err := svcMgr.StopService(name); err != nil {
		return err
	}
	fmt.Printf("  ✓ %s stopped\n", name)
	return nil
}

func runServiceStatus(_ *cobra.Command, _ []string) error {
	svcMgr, err := getServiceManager()
	if err != nil {
		return err
	}

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

		portStr := "-"
		if status.Port > 0 {
			portStr = strconv.Itoa(status.Port)
		}

		fmt.Printf("%-15s %-10s %-10s %-10s %s\n",
			name, statusStr, healthStr, portStr, uptimeStr)
	}

	return nil
}

func runServiceLogs(_ *cobra.Command, args []string) error {
	name := args[0]
	containerName := service.ContainerName(name)

	dockerArgs := []string{"logs"}
	if serviceLogsFollow {
		dockerArgs = append(dockerArgs, "-f")
	}
	dockerArgs = append(dockerArgs, "--tail", strconv.Itoa(serviceLogsTail))
	dockerArgs = append(dockerArgs, containerName)

	dockerCmd := exec.Command("docker", dockerArgs...)
	dockerCmd.Stdout = os.Stdout
	dockerCmd.Stderr = os.Stderr
	return dockerCmd.Run()
}

func runServiceConnect(_ *cobra.Command, args []string) error {
	name := args[0]

	svcMgr, err := getServiceManager()
	if err != nil {
		return err
	}

	cfg, err := svcMgr.GetConfig(name)
	if err != nil {
		return fmt.Errorf("service %q not configured", name)
	}

	containerName := service.ContainerName(name)

	// Check if container is running
	status, err := svcMgr.GetStatus(name)
	if err != nil {
		return err
	}
	if !status.Running {
		return fmt.Errorf("service %q is not running", name)
	}

	var connectCmd *exec.Cmd
	switch name {
	case "postgres":
		user := cfg.Environment["POSTGRES_USER"]
		db := cfg.Environment["POSTGRES_DB"]
		if user == "" {
			user = "postgres"
		}
		if db == "" {
			db = "postgres"
		}
		connectCmd = exec.Command("docker", "exec", "-it", containerName, //nolint:gosec // G204: Arguments are from internal service config
			"psql", "-U", user, "-d", db)
	case "redis":
		connectCmd = exec.Command("docker", "exec", "-it", containerName, //nolint:gosec // G204: Container name is from internal config
			"redis-cli")
	default:
		// Generic shell
		connectCmd = exec.Command("docker", "exec", "-it", containerName, "sh") //nolint:gosec // G204: Container name is from internal config
	}

	connectCmd.Stdin = os.Stdin
	connectCmd.Stdout = os.Stdout
	connectCmd.Stderr = os.Stderr
	return connectCmd.Run()
}
