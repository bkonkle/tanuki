// Package service provides shared service management for Tanuki.
//
// Services are Docker containers running infrastructure like databases (Postgres, Redis)
// that agent containers can connect to. Services run on the same Docker network
// (tanuki-net) and their connection info is automatically injected into agent containers.
package service

import (
	"errors"
	"time"
)

var (
	// ErrServiceNotFound indicates the service is not configured.
	ErrServiceNotFound = errors.New("service not found")

	// ErrServiceNotRunning indicates the service is not currently running.
	ErrServiceNotRunning = errors.New("service not running")

	// ErrServiceAlreadyRunning indicates the service is already running.
	ErrServiceAlreadyRunning = errors.New("service already running")

	// ErrHealthCheckFailed indicates the service health check failed.
	ErrHealthCheckFailed = errors.New("health check failed")
)

// ManagerInterface defines the interface for service lifecycle management.
type ManagerInterface interface {
	// StartServices starts all enabled services.
	StartServices() error

	// StopServices stops all running services.
	StopServices() error

	// StartService starts a specific service by name.
	StartService(name string) error

	// StopService stops a specific service by name.
	StopService(name string) error

	// GetStatus returns the status of a specific service.
	GetStatus(name string) (*Status, error)

	// GetAllStatus returns the status of all configured services.
	GetAllStatus() map[string]*Status

	// IsHealthy checks if a service is healthy.
	IsHealthy(name string) bool

	// GetConnectionInfo returns connection information for a service.
	GetConnectionInfo(name string) (*Connection, error)

	// GetAllConnections returns connection info for all running services.
	GetAllConnections() map[string]*Connection
}

// Status represents the current state of a service.
type Status struct {
	// Name is the service identifier (e.g., "postgres", "redis").
	Name string

	// Running indicates if the service container is running.
	Running bool

	// Healthy indicates if the service is responding to health checks.
	Healthy bool

	// ContainerID is the Docker container ID.
	ContainerID string

	// ContainerName is the Docker container name.
	ContainerName string

	// StartedAt is when the service was started.
	StartedAt time.Time

	// Port is the exposed port on the host.
	Port int

	// Error contains any error message if the service failed.
	Error string
}

// Connection contains information needed to connect to a service.
type Connection struct {
	// Host is the hostname to use for connections.
	// For inter-container communication, this is the container name.
	Host string

	// Port is the service port number.
	Port int

	// URL is the combined host:port URL.
	URL string

	// Username is the service username (if applicable).
	Username string

	// Password is the service password (if applicable).
	Password string

	// Database is the database name (if applicable).
	Database string
}

// Config represents the configuration for a single service.
type Config struct {
	// Enabled controls whether the service should be started.
	Enabled bool `yaml:"enabled" mapstructure:"enabled"`

	// Image is the Docker image to use (e.g., "postgres:16").
	Image string `yaml:"image" mapstructure:"image"`

	// Port is the port the service listens on.
	Port int `yaml:"port" mapstructure:"port"`

	// Environment contains environment variables for the container.
	Environment map[string]string `yaml:"environment" mapstructure:"environment"`

	// Volumes contains volume mounts in Docker format (e.g., "name:/path").
	Volumes []string `yaml:"volumes" mapstructure:"volumes"`

	// Healthcheck configures the health check for the service.
	Healthcheck *HealthcheckConfig `yaml:"healthcheck,omitempty" mapstructure:"healthcheck"`
}

// HealthcheckConfig configures health checking for a service.
type HealthcheckConfig struct {
	// Command is the health check command to run.
	Command []string `yaml:"command" mapstructure:"command"`

	// Interval is how often to run the health check.
	Interval string `yaml:"interval" mapstructure:"interval"`

	// Timeout is the maximum time to wait for a health check response.
	Timeout string `yaml:"timeout" mapstructure:"timeout"`

	// Retries is the number of consecutive failures before marking unhealthy.
	Retries int `yaml:"retries" mapstructure:"retries"`
}

// DefaultPostgresConfig returns the default configuration for PostgreSQL.
func DefaultPostgresConfig() *Config {
	return &Config{
		Enabled: false,
		Image:   "postgres:16",
		Port:    5432,
		Environment: map[string]string{
			"POSTGRES_USER":     "tanuki",
			"POSTGRES_PASSWORD": "tanuki",
			"POSTGRES_DB":       "tanuki_dev",
		},
		Volumes: []string{
			"tanuki-postgres:/var/lib/postgresql/data",
		},
		Healthcheck: &HealthcheckConfig{
			Command:  []string{"pg_isready", "-U", "tanuki"},
			Interval: "5s",
			Timeout:  "3s",
			Retries:  5,
		},
	}
}

// DefaultRedisConfig returns the default configuration for Redis.
func DefaultRedisConfig() *Config {
	return &Config{
		Enabled:     false,
		Image:       "redis:7",
		Port:        6379,
		Environment: map[string]string{},
		Volumes: []string{
			"tanuki-redis:/data",
		},
		Healthcheck: &HealthcheckConfig{
			Command:  []string{"redis-cli", "ping"},
			Interval: "5s",
			Timeout:  "3s",
			Retries:  5,
		},
	}
}

// ContainerNamePrefix is the prefix used for service container names.
const ContainerNamePrefix = "tanuki-svc-"

// ContainerName returns the Docker container name for a service.
func ContainerName(serviceName string) string {
	return ContainerNamePrefix + serviceName
}
