// Package agent provides agent lifecycle management functionality.
package agent

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/bkonkle/tanuki/internal/config"
	"github.com/bkonkle/tanuki/internal/context"
	"github.com/bkonkle/tanuki/internal/docker"
	"github.com/bkonkle/tanuki/internal/executor"
	"github.com/bkonkle/tanuki/internal/state"
)

var (
	// ErrAgentExists indicates an agent with the given name already exists.
	ErrAgentExists = errors.New("agent already exists")

	// ErrAgentNotFound indicates no agent with the given name exists.
	ErrAgentNotFound = errors.New("agent not found")

	// ErrAgentWorking indicates the agent is currently working and cannot be removed.
	ErrAgentWorking = errors.New("agent is currently working")

	// ErrInvalidName indicates the agent name doesn't meet requirements.
	ErrInvalidName = errors.New("invalid agent name")
)

// validNamePattern enforces agent naming rules: start with lowercase letter,
// contain only lowercase letters, numbers, and hyphens, end with letter or number.
var validNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]*[a-z0-9]$`)

// Agent is an alias for state.Agent for convenience.
type Agent = state.Agent

// TaskInfo is an alias for state.TaskInfo for convenience.
type TaskInfo = state.TaskInfo

// Status provides detailed status information about an agent.
type Status struct {
	Name      string
	Status    string
	Branch    string
	Container ContainerStatus
	Git       GitStatus
	LastTask  *TaskInfo
	Uptime    time.Duration
}

// ContainerStatus contains Docker container status information.
type ContainerStatus struct {
	ID      string
	Running bool
	Memory  string
	CPU     string
}

// GitStatus contains Git worktree status information.
type GitStatus struct {
	Branch        string
	CommitsBehind int
	CommitsAhead  int
	HasChanges    bool
}

// SpawnOptions configures agent creation.
type SpawnOptions struct {
	// Branch specifies an existing branch to use (optional)
	Branch string
	// Role specifies the role to assign to the agent (optional)
	Role string
}

// RemoveOptions configures agent removal.
type RemoveOptions struct {
	// Force removes the agent even if it's currently working
	Force bool
	// KeepBranch preserves the git branch after removal
	KeepBranch bool
}

// RunOptions configures task execution.
type RunOptions struct {
	// Follow streams output in real-time
	Follow bool
	// AllowedTools overrides the default tool list
	AllowedTools []string
	// DisallowedTools lists tools that should not be used
	DisallowedTools []string
	// MaxTurns overrides the default max conversation turns
	MaxTurns int
	// Model overrides the default Claude model
	Model string
	// SystemPrompt adds additional system instructions
	SystemPrompt string
	// Output writer for streaming (defaults to os.Stdout)
	Output io.Writer
}

// GitManager defines the interface for Git worktree operations.
type GitManager interface {
	CreateWorktree(name string) (string, error)
	RemoveWorktree(name string, deleteBranch bool) error
	GetDiff(name string, baseBranch string) (string, error)
	GetStatus(name string) (string, error)
	GetCurrentBranch() (string, error)
	GetMainBranch() (string, error)
	WorktreeExists(name string) bool
	BranchExists(name string) bool
	GetWorktreePath(name string) string
	GetBranchName(name string) string
}

// DockerManager defines the interface for Docker container operations.
type DockerManager interface {
	EnsureNetwork(name string) error
	CreateAgentContainer(name string, worktreePath string) (string, error)
	CreateAgentContainerWithOptions(name string, worktreePath string, opts docker.AgentContainerOptions) (string, error)
	StartContainer(containerID string) error
	SetupContainer(containerID string) error
	StopContainer(containerID string) error
	RemoveContainer(containerID string) error
	ContainerExists(containerID string) bool
	ContainerRunning(containerID string) bool
	InspectContainer(containerID string) (*ContainerInfo, error)
	ExecWithOutput(containerID string, cmd []string) (string, error)
	GetResourceUsage(containerID string) (*ResourceUsage, error)
}

// ServiceInjector provides service connection information for agent containers.
type ServiceInjector interface {
	BuildEnvironment() map[string]string
	CheckServiceHealth() []string
	GenerateDocumentation() string
}

// ResourceUsage is an alias for docker.ResourceUsage for convenience.
type ResourceUsage = docker.ResourceUsage

// ClaudeExecutor defines the interface for Claude Code execution.
type ClaudeExecutor interface {
	Run(containerID string, prompt string, opts executor.ExecuteOptions) (*executor.ExecutionResult, error)
	RunFollow(containerID string, prompt string, opts executor.ExecuteOptions, output io.Writer) (*executor.ExecutionResult, error)
	CheckContainer(containerID string) error
}

// ContainerInfo is an alias for docker.ContainerInfo for convenience.
type ContainerInfo = docker.ContainerInfo

// StateManager defines the interface for state persistence.
type StateManager interface {
	Load() (*State, error)
	Save(state *State) error
	GetAgent(name string) (*Agent, error)
	SetAgent(agent *Agent) error
	RemoveAgent(name string) error
	ListAgents() ([]*Agent, error)
}

// State is an alias for state.State for convenience.
type State = state.State

// RoleInfo contains role information needed by the agent manager.
type RoleInfo struct {
	Name            string
	SystemPrompt    string
	AllowedTools    []string
	DisallowedTools []string
	ContextFiles    []string
}

// RoleManager defines the interface for role operations.
type RoleManager interface {
	GetRoleInfo(name string) (*RoleInfo, error)
}

// Manager handles agent lifecycle operations, coordinating Git worktrees,
// Docker containers, persistent state, and Claude Code execution.
type Manager struct {
	config          *config.Config
	git             GitManager
	docker          DockerManager
	state           StateManager
	executor        ClaudeExecutor
	roleManager     RoleManager
	serviceInjector ServiceInjector
}

// NewManager creates a new agent manager.
func NewManager(cfg *config.Config, git GitManager, docker DockerManager, state StateManager, executor ClaudeExecutor) (*Manager, error) {
	if cfg == nil {
		return nil, errors.New("config is required")
	}
	if git == nil {
		return nil, errors.New("git manager is required")
	}
	if docker == nil {
		return nil, errors.New("docker manager is required")
	}
	if state == nil {
		return nil, errors.New("state manager is required")
	}
	if executor == nil {
		return nil, errors.New("executor is required")
	}

	// Ensure Docker network exists
	if err := docker.EnsureNetwork(cfg.Network.Name); err != nil {
		return nil, fmt.Errorf("failed to ensure Docker network: %w", err)
	}

	return &Manager{
		config:          cfg,
		git:             git,
		docker:          docker,
		state:           state,
		executor:        executor,
		roleManager:     nil, // Will be set via SetRoleManager
		serviceInjector: nil, // Will be set via SetServiceInjector
	}, nil
}

// SetRoleManager sets the role manager for this agent manager.
// This is optional and allows role-based agent spawning.
func (m *Manager) SetRoleManager(roleManager RoleManager) {
	m.roleManager = roleManager
}

// SetServiceInjector sets the service injector for this agent manager.
// This is optional and allows service connection injection into agent containers.
func (m *Manager) SetServiceInjector(injector ServiceInjector) {
	m.serviceInjector = injector
}

// Spawn creates a new agent with an isolated worktree and container.
// This operation is atomic - if any step fails, all created resources are cleaned up.
func (m *Manager) Spawn(name string, opts SpawnOptions) (*Agent, error) {
	// 1. Validate name
	if err := validateAgentName(name); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidName, err)
	}

	// 2. Check if agent already exists
	if _, err := m.state.GetAgent(name); err == nil {
		return nil, fmt.Errorf("%w: %q", ErrAgentExists, name)
	}

	// 3. Create worktree
	worktreePath, err := m.git.CreateWorktree(name)
	if err != nil {
		return nil, fmt.Errorf("failed to create worktree: %w", err)
	}

	// 4. Handle role if specified
	var roleInfo *RoleInfo
	if opts.Role != "" {
		if m.roleManager == nil {
			_ = m.git.RemoveWorktree(name, true) // Rollback
			return nil, fmt.Errorf("role specified but role manager not configured")
		}

		roleInfo, err = m.roleManager.GetRoleInfo(opts.Role)
		if err != nil {
			_ = m.git.RemoveWorktree(name, true) // Rollback
			return nil, fmt.Errorf("failed to get role %q: %w", opts.Role, err)
		}

		// Copy context files if specified
		if len(roleInfo.ContextFiles) > 0 {
			// Get project root from current working directory
			projectRoot, cwdErr := os.Getwd()
			if cwdErr != nil {
				_ = m.git.RemoveWorktree(name, true) // Rollback
				return nil, fmt.Errorf("failed to get project root: %w", cwdErr)
			}
			contextMgr := context.NewManager(projectRoot, false)
			result, copyErr := contextMgr.CopyContextFiles(worktreePath, roleInfo.ContextFiles)
			if copyErr != nil {
				_ = m.git.RemoveWorktree(name, true) // Rollback
				return nil, fmt.Errorf("failed to copy context files: %w", copyErr)
			}
			// Log warnings but don't fail for missing context files
			_ = result // Result contains Copied, Skipped, and Errors for logging if needed
		}

		// Generate CLAUDE.md with role system prompt
		if genErr := m.generateClaudeMD(worktreePath, roleInfo); genErr != nil {
			_ = m.git.RemoveWorktree(name, true) // Rollback
			return nil, fmt.Errorf("failed to generate CLAUDE.md: %w", genErr)
		}
	}

	// 5. Build service environment and check health
	var serviceEnv map[string]string
	if m.serviceInjector != nil {
		serviceEnv = m.serviceInjector.BuildEnvironment()

		// Log warnings for unhealthy services
		for _, warning := range m.serviceInjector.CheckServiceHealth() {
			fmt.Printf("Warning: %s\n", warning)
		}
	}

	// 6. Create container with service injection (rollback worktree on failure)
	containerOpts := docker.AgentContainerOptions{
		ServiceEnv: serviceEnv,
	}
	containerID, err := m.docker.CreateAgentContainerWithOptions(name, worktreePath, containerOpts)
	if err != nil {
		_ = m.git.RemoveWorktree(name, true) // Rollback
		return nil, fmt.Errorf("failed to create container: %w", err)
	}

	// 7. Start container (rollback both on failure)
	if err := m.docker.StartContainer(containerID); err != nil {
		_ = m.docker.RemoveContainer(containerID) // Rollback
		_ = m.git.RemoveWorktree(name, true)      // Rollback
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	// 8. Setup container (install Claude Code CLI)
	if err := m.docker.SetupContainer(containerID); err != nil {
		_ = m.docker.StopContainer(containerID)   // Rollback
		_ = m.docker.RemoveContainer(containerID) // Rollback
		_ = m.git.RemoveWorktree(name, true)      // Rollback
		return nil, fmt.Errorf("failed to setup container: %w", err)
	}

	// 9. Create state entry
	agent := &Agent{
		Name:          name,
		ContainerID:   containerID,
		ContainerName: fmt.Sprintf("tanuki-%s", name),
		Branch:        m.git.GetBranchName(name),
		WorktreePath:  worktreePath,
		Status:        state.StatusIdle,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	// Store role information if role was assigned
	if roleInfo != nil {
		agent.Role = roleInfo.Name
		agent.AllowedTools = roleInfo.AllowedTools
		agent.DisallowedTools = roleInfo.DisallowedTools
	}

	if err := m.state.SetAgent(agent); err != nil {
		// Rollback everything
		_ = m.docker.StopContainer(containerID)
		_ = m.docker.RemoveContainer(containerID)
		_ = m.git.RemoveWorktree(name, true)
		return nil, fmt.Errorf("failed to save state: %w", err)
	}

	return agent, nil
}

// Remove deletes an agent and cleans up all associated resources.
// By default, this fails if the agent is currently working unless Force is true.
func (m *Manager) Remove(name string, opts RemoveOptions) error {
	agent, err := m.state.GetAgent(name)
	if err != nil {
		return fmt.Errorf("%w: %q", ErrAgentNotFound, name)
	}

	// Check if working
	if agent.Status == state.StatusWorking && !opts.Force {
		return fmt.Errorf("%w: use --force to remove anyway", ErrAgentWorking)
	}

	// Stop and remove container (ignore errors - best effort)
	_ = m.docker.StopContainer(agent.ContainerID)
	_ = m.docker.RemoveContainer(agent.ContainerID)

	// Remove worktree and optionally branch
	if err := m.git.RemoveWorktree(name, !opts.KeepBranch); err != nil {
		// Continue with state removal even if git cleanup fails
		_ = m.state.RemoveAgent(name)
		return fmt.Errorf("failed to remove worktree: %w", err)
	}

	// Remove from state
	return m.state.RemoveAgent(name)
}

// Stop stops an agent's container without removing it.
func (m *Manager) Stop(name string) error {
	agent, err := m.state.GetAgent(name)
	if err != nil {
		return fmt.Errorf("%w: %q", ErrAgentNotFound, name)
	}

	if err := m.docker.StopContainer(agent.ContainerID); err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}

	// Update state
	agent.Status = state.StatusStopped
	agent.UpdatedAt = time.Now()
	return m.state.SetAgent(agent)
}

// Start starts a stopped agent's container.
func (m *Manager) Start(name string) error {
	agent, err := m.state.GetAgent(name)
	if err != nil {
		return fmt.Errorf("%w: %q", ErrAgentNotFound, name)
	}

	if err := m.docker.StartContainer(agent.ContainerID); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	// Update state
	agent.Status = state.StatusIdle
	agent.UpdatedAt = time.Now()
	return m.state.SetAgent(agent)
}

// Get returns information about a specific agent.
func (m *Manager) Get(name string) (*Agent, error) {
	agent, err := m.state.GetAgent(name)
	if err != nil {
		return nil, fmt.Errorf("%w: %q", ErrAgentNotFound, name)
	}
	return agent, nil
}

// List returns all agents.
func (m *Manager) List() ([]*Agent, error) {
	agents, err := m.state.ListAgents()
	if err != nil {
		return nil, fmt.Errorf("failed to list agents: %w", err)
	}
	return agents, nil
}

// Status returns detailed status information about an agent.
// This aggregates information from the container, git worktree, and state.
func (m *Manager) Status(name string) (*Status, error) {
	agent, err := m.state.GetAgent(name)
	if err != nil {
		return nil, fmt.Errorf("%w: %q", ErrAgentNotFound, name)
	}

	status := &Status{
		Name:     agent.Name,
		Status:   string(agent.Status),
		Branch:   agent.Branch,
		LastTask: agent.LastTask,
		Uptime:   time.Since(agent.CreatedAt),
	}

	// Get container status
	containerInfo, err := m.docker.InspectContainer(agent.ContainerID)
	if err == nil && containerInfo != nil {
		isRunning := containerInfo.State == "running"
		status.Container = ContainerStatus{
			ID:      agent.ContainerID[:12], // Short ID
			Running: isRunning,
			Memory:  "",
			CPU:     "",
		}

		// Get resource usage if container is running
		if isRunning {
			if resourceUsage, resErr := m.docker.GetResourceUsage(agent.ContainerID); resErr == nil && resourceUsage != nil {
				status.Container.Memory = resourceUsage.Memory
				status.Container.CPU = resourceUsage.CPU
			}
		}
	}

	// Get git status
	mainBranch, err := m.git.GetMainBranch()
	if err != nil {
		mainBranch = "main" // Fallback
	}

	diff, _ := m.git.GetDiff(name, mainBranch)
	gitStatus, _ := m.git.GetStatus(name)

	status.Git = GitStatus{
		Branch:     agent.Branch,
		HasChanges: len(diff) > 0 || len(gitStatus) > 0,
	}

	return status, nil
}

// Run executes a task in the agent's container using Claude Code.
// Supports both fire-and-forget and follow (streaming) modes.
func (m *Manager) Run(name string, prompt string, opts RunOptions) error {
	agent, err := m.state.GetAgent(name)
	if err != nil {
		return fmt.Errorf("%w: %q", ErrAgentNotFound, name)
	}

	// Check if agent is already working
	if agent.Status == state.StatusWorking {
		return errors.New("agent is already working on a task")
	}

	// Check container is ready
	if err := m.executor.CheckContainer(agent.ContainerID); err != nil {
		return fmt.Errorf("container not ready: %w", err)
	}

	// Build execute options
	execOpts := executor.ExecuteOptions{
		AllowedTools:    opts.AllowedTools,
		DisallowedTools: opts.DisallowedTools,
		MaxTurns:        opts.MaxTurns,
		Model:           opts.Model,
		SystemPrompt:    opts.SystemPrompt,
		WorkDir:         "/workspace",
	}

	// Apply defaults from config if not specified
	if len(execOpts.AllowedTools) == 0 {
		execOpts.AllowedTools = m.config.Defaults.AllowedTools
	}
	if execOpts.MaxTurns == 0 {
		execOpts.MaxTurns = m.config.Defaults.MaxTurns
	}
	if execOpts.Model == "" {
		execOpts.Model = m.config.Defaults.Model
	}

	// Update state to working
	agent.Status = state.StatusWorking
	agent.UpdatedAt = time.Now()
	agent.LastTask = &TaskInfo{
		Prompt:    prompt,
		StartedAt: time.Now(),
	}
	if err := m.state.SetAgent(agent); err != nil {
		return fmt.Errorf("failed to update state: %w", err)
	}

	// Execute based on mode
	var result *executor.ExecutionResult
	var execErr error

	output := opts.Output
	if output == nil {
		output = os.Stdout
	}

	if opts.Follow {
		// Blocking: stream output
		result, execErr = m.executor.RunFollow(agent.ContainerID, prompt, execOpts, output)
	} else {
		// Fire-and-forget: run and capture output
		result, execErr = m.executor.Run(agent.ContainerID, prompt, execOpts)
	}

	// Update state back to idle
	agent.Status = state.StatusIdle
	agent.UpdatedAt = time.Now()

	if result != nil {
		completedAt := result.CompletedAt
		agent.LastTask.CompletedAt = &completedAt
		agent.LastTask.SessionID = result.SessionID
	}

	if err := m.state.SetAgent(agent); err != nil {
		return fmt.Errorf("failed to update state after execution: %w", err)
	}

	return execErr
}

// IsWorking checks if an agent is currently executing a task.
func (m *Manager) IsWorking(name string) (bool, error) {
	agent, err := m.state.GetAgent(name)
	if err != nil {
		return false, fmt.Errorf("%w: %q", ErrAgentNotFound, name)
	}
	return agent.Status == "working", nil
}

// Reconcile synchronizes state with actual container and git status.
// This is useful for detecting stale state (e.g., containers removed externally).
func (m *Manager) Reconcile() error {
	agents, err := m.state.ListAgents()
	if err != nil {
		return fmt.Errorf("failed to list agents: %w", err)
	}

	for _, agent := range agents {
		updated := false

		// Check container existence
		exists := m.docker.ContainerExists(agent.ContainerID)
		running := m.docker.ContainerRunning(agent.ContainerID)

		if !exists {
			if agent.Status != state.StatusError {
				agent.Status = state.StatusError
				updated = true
			}
		} else if !running && agent.Status == state.StatusWorking {
			agent.Status = state.StatusStopped
			updated = true
		}

		if updated {
			agent.UpdatedAt = time.Now()
			if err := m.state.SetAgent(agent); err != nil {
				// Log error but continue with other agents
				continue
			}
		}
	}

	return nil
}

// validateAgentName checks if an agent name meets requirements:
// - At least 2 characters
// - At most 63 characters (DNS label limit)
// - Start with lowercase letter
// - Contain only lowercase letters, numbers, and hyphens
// - End with letter or number
func validateAgentName(name string) error {
	if len(name) < 2 {
		return errors.New("name must be at least 2 characters")
	}
	if len(name) > 63 {
		return errors.New("name must be at most 63 characters")
	}
	if !validNamePattern.MatchString(name) {
		return errors.New("name must start with a letter and contain only lowercase letters, numbers, and hyphens")
	}
	return nil
}

// generateClaudeMD creates a CLAUDE.md file in the worktree with role-specific instructions.
func (m *Manager) generateClaudeMD(worktreePath string, roleInfo *RoleInfo) error {
	claudeMDPath := filepath.Join(worktreePath, "CLAUDE.md")

	var content strings.Builder

	// Add role system prompt as agent instructions
	content.WriteString("# Agent Instructions\n\n")
	content.WriteString(roleInfo.SystemPrompt)
	content.WriteString("\n")

	// Add context file references if any (will be populated later by context manager)
	if len(roleInfo.ContextFiles) > 0 {
		content.WriteString("\n## Context Files\n\n")
		content.WriteString("Review these files for project context:\n\n")
		for _, file := range roleInfo.ContextFiles {
			content.WriteString(fmt.Sprintf("- %s\n", file))
		}
	}

	// Add service documentation if available
	if m.serviceInjector != nil {
		serviceDocs := m.serviceInjector.GenerateDocumentation()
		if serviceDocs != "" {
			content.WriteString(serviceDocs)
		}
	}

	return os.WriteFile(claudeMDPath, []byte(content.String()), 0600)
}
