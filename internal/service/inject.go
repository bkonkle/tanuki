package service

import (
	"fmt"
	"strings"
)

// Injector builds environment variables from service connections for agent containers.
type Injector struct {
	manager Manager
}

// NewInjector creates a new service injector.
func NewInjector(manager Manager) *Injector {
	return &Injector{
		manager: manager,
	}
}

// BuildEnvironment builds environment variables for all running services.
// Returns a map of environment variable names to values.
func (i *Injector) BuildEnvironment() map[string]string {
	env := make(map[string]string)

	if i.manager == nil {
		return env
	}

	connections := i.manager.GetAllConnections()
	for name, conn := range connections {
		prefix := strings.ToUpper(name)

		// Core connection info
		env[fmt.Sprintf("%s_HOST", prefix)] = conn.Host
		env[fmt.Sprintf("%s_PORT", prefix)] = fmt.Sprintf("%d", conn.Port)
		env[fmt.Sprintf("%s_URL", prefix)] = conn.URL

		// Credentials (if available)
		if conn.Username != "" {
			env[fmt.Sprintf("%s_USER", prefix)] = conn.Username
		}
		if conn.Password != "" {
			env[fmt.Sprintf("%s_PASSWORD", prefix)] = conn.Password
		}
		if conn.Database != "" {
			env[fmt.Sprintf("%s_DATABASE", prefix)] = conn.Database
		}

		// Build DSN for databases
		dsn := buildDSN(name, conn)
		if dsn != "" {
			env[fmt.Sprintf("%s_DSN", prefix)] = dsn
		}
	}

	return env
}

// BuildEnvironmentSlice returns environment variables as a slice of "KEY=VALUE" strings.
// This is useful for Docker command line arguments.
func (i *Injector) BuildEnvironmentSlice() []string {
	env := i.BuildEnvironment()
	result := make([]string, 0, len(env))
	for k, v := range env {
		result = append(result, fmt.Sprintf("%s=%s", k, v))
	}
	return result
}

// CheckServiceHealth returns warnings for services that are not healthy.
func (i *Injector) CheckServiceHealth() []string {
	var warnings []string

	if i.manager == nil {
		return warnings
	}

	statuses := i.manager.GetAllStatus()
	for name, status := range statuses {
		if status.Running && !status.Healthy {
			warnings = append(warnings, fmt.Sprintf("service %s is running but not healthy", name))
		}
	}

	return warnings
}

// GenerateDocumentation generates markdown documentation for available services.
// This can be appended to an agent's CLAUDE.md file.
func (i *Injector) GenerateDocumentation() string {
	if i.manager == nil {
		return ""
	}

	connections := i.manager.GetAllConnections()
	if len(connections) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n## Available Services\n\n")
	sb.WriteString("The following services are available via environment variables:\n\n")

	for name, conn := range connections {
		prefix := strings.ToUpper(name)
		sb.WriteString(fmt.Sprintf("### %s\n\n", strings.Title(name)))
		sb.WriteString(fmt.Sprintf("- Host: `$%s_HOST` (%s)\n", prefix, conn.Host))
		sb.WriteString(fmt.Sprintf("- Port: `$%s_PORT` (%d)\n", prefix, conn.Port))
		sb.WriteString(fmt.Sprintf("- URL: `$%s_URL` (%s)\n", prefix, conn.URL))

		if conn.Username != "" {
			sb.WriteString(fmt.Sprintf("- User: `$%s_USER`\n", prefix))
		}
		if conn.Database != "" {
			sb.WriteString(fmt.Sprintf("- Database: `$%s_DATABASE`\n", prefix))
		}

		dsn := buildDSN(name, conn)
		if dsn != "" {
			sb.WriteString(fmt.Sprintf("- DSN: `$%s_DSN`\n", prefix))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// buildDSN constructs a database connection string for known service types.
func buildDSN(serviceName string, conn *Connection) string {
	switch serviceName {
	case "postgres":
		if conn.Username != "" && conn.Database != "" {
			return fmt.Sprintf("postgres://%s:%s@%s:%d/%s",
				conn.Username, conn.Password, conn.Host, conn.Port, conn.Database)
		}
	case "redis":
		return fmt.Sprintf("redis://%s:%d", conn.Host, conn.Port)
	case "mysql":
		if conn.Username != "" && conn.Database != "" {
			return fmt.Sprintf("mysql://%s:%s@%s:%d/%s",
				conn.Username, conn.Password, conn.Host, conn.Port, conn.Database)
		}
	}
	return ""
}

// MergeEnvironment merges service environment variables with existing environment.
// Service variables do not override existing values.
func MergeEnvironment(existing map[string]string, serviceEnv map[string]string) map[string]string {
	result := make(map[string]string)

	// Copy service env first (lower priority)
	for k, v := range serviceEnv {
		result[k] = v
	}

	// Copy existing env (higher priority, overrides service env)
	for k, v := range existing {
		result[k] = v
	}

	return result
}
