// Package state provides persistent state tracking for agents.
//
// State is stored in .tanuki/state/agents.json and tracks all agents,
// their container IDs, branches, status, and metadata. The state file
// is gitignored and survives CLI restarts.
package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Status represents the current state of an agent.
type Status string

const (
	// StatusCreating indicates the agent is being set up (worktree/container)
	StatusCreating Status = "creating"
	// StatusIdle indicates the agent is ready to accept tasks
	StatusIdle Status = "idle"
	// StatusWorking indicates the agent is currently executing a task
	StatusWorking Status = "working"
	// StatusStopped indicates the container is stopped but exists
	StatusStopped Status = "stopped"
	// StatusError indicates something went wrong
	StatusError Status = "error"
)

// State represents the complete agent state for a project.
type State struct {
	// Version is the state schema version
	Version string `json:"version"`

	// Project is the absolute path to the project root
	Project string `json:"project"`

	// Agents maps agent names to their state
	Agents map[string]*Agent `json:"agents"`
}

// Agent represents a single agent's state.
type Agent struct {
	// Name is the agent's unique identifier
	Name string `json:"name"`

	// ContainerID is the Docker container ID
	ContainerID string `json:"container_id"`

	// ContainerName is the Docker container name
	ContainerName string `json:"container_name"`

	// Branch is the Git branch name
	Branch string `json:"branch"`

	// WorktreePath is the path to the Git worktree
	WorktreePath string `json:"worktree_path"`

	// Status is the current agent status
	Status Status `json:"status"`

	// CreatedAt is when the agent was created
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when the agent was last updated
	UpdatedAt time.Time `json:"updated_at"`

	// LastTask contains information about the last task executed
	LastTask *TaskInfo `json:"last_task,omitempty"`

	// Role is the assigned role name (if any)
	Role string `json:"role,omitempty"`

	// AllowedTools is the list of tools this agent is allowed to use
	AllowedTools []string `json:"allowed_tools,omitempty"`

	// DisallowedTools is the list of tools this agent is not allowed to use
	DisallowedTools []string `json:"disallowed_tools,omitempty"`
}

// TaskInfo contains information about a task execution.
type TaskInfo struct {
	// Prompt is the task description
	Prompt string `json:"prompt"`

	// StartedAt is when the task started
	StartedAt time.Time `json:"started_at"`

	// CompletedAt is when the task completed (zero if still running)
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	// SessionID is the Claude Code session identifier
	SessionID string `json:"session_id"`

	// Workstream is the workstream this task belongs to
	Workstream string `json:"workstream,omitempty"`

	// TurnsUsed tracks conversation turns for context budget
	TurnsUsed int `json:"turns_used,omitempty"`

	// IterationsUsed tracks Ralph iterations for this task
	IterationsUsed int `json:"iterations_used,omitempty"`
}

// WorkstreamSession tracks context budget for a workstream.
type WorkstreamSession struct {
	// Workstream identifier
	Workstream string `json:"workstream"`

	// Role this workstream belongs to
	Role string `json:"role"`

	// AgentName assigned to this session
	AgentName string `json:"agent_name"`

	// TotalTurns used across all tasks in this session
	TotalTurns int `json:"total_turns"`

	// MaxTurns before context reset (from config)
	MaxTurns int `json:"max_turns"`

	// StartedAt is when this session started
	StartedAt time.Time `json:"started_at"`

	// Tasks completed in this session
	TasksCompleted []string `json:"tasks_completed,omitempty"`

	// CurrentTask being worked on
	CurrentTask string `json:"current_task,omitempty"`
}

// NeedsContextReset returns true if the session has exceeded its turn budget.
func (ws *WorkstreamSession) NeedsContextReset() bool {
	if ws.MaxTurns <= 0 {
		return false // No limit
	}
	return ws.TotalTurns >= ws.MaxTurns
}

// AddTurns increments the turn count.
func (ws *WorkstreamSession) AddTurns(turns int) {
	ws.TotalTurns += turns
}

// CompleteTask marks a task as completed in this session.
func (ws *WorkstreamSession) CompleteTask(taskID string) {
	ws.TasksCompleted = append(ws.TasksCompleted, taskID)
	ws.CurrentTask = ""
}

// Manager handles state persistence operations.
type Manager interface {
	// Load reads the state from disk
	Load() (*State, error)

	// Save writes the state to disk atomically
	Save(state *State) error

	// GetAgent retrieves a specific agent's state
	GetAgent(name string) (*Agent, error)

	// SetAgent updates or creates an agent's state
	SetAgent(agent *Agent) error

	// RemoveAgent deletes an agent from the state
	RemoveAgent(name string) error

	// ListAgents returns all agents
	ListAgents() ([]*Agent, error)

	// Reconcile checks actual container state and updates stale entries
	Reconcile() error
}

// FileStateManager implements Manager using a JSON file.
type FileStateManager struct {
	path    string
	mu      sync.RWMutex
	state   *State
	checker ContainerChecker
}

// ContainerChecker checks Docker container status.
// This interface allows mocking in tests.
type ContainerChecker interface {
	// ContainerStatus checks if a container exists and is running
	ContainerStatus(containerID string) (exists bool, running bool, err error)
}

// NewFileStateManager creates a new file-based state manager.
// The path should point to the agents.json file location.
func NewFileStateManager(path string, checker ContainerChecker) (*FileStateManager, error) {
	m := &FileStateManager{
		path:    path,
		checker: checker,
	}

	// Try to load existing state
	state, err := m.loadFromDisk()
	if err != nil {
		// If file doesn't exist, create new state
		if os.IsNotExist(err) {
			// Get project root (parent of .tanuki directory)
			projectPath := filepath.Dir(filepath.Dir(filepath.Dir(path)))
			state = &State{
				Version: "1",
				Project: projectPath,
				Agents:  make(map[string]*Agent),
			}
		} else {
			return nil, err
		}
	}

	m.state = state
	return m, nil
}

// Load returns the current state.
func (m *FileStateManager) Load() (*State, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return a copy to prevent external modifications
	stateCopy := *m.state
	stateCopy.Agents = make(map[string]*Agent, len(m.state.Agents))
	for k, v := range m.state.Agents {
		agentCopy := *v
		stateCopy.Agents[k] = &agentCopy
	}

	return &stateCopy, nil
}

// Save writes the state to disk atomically.
func (m *FileStateManager) Save(state *State) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Ensure directory exists
	dir := filepath.Dir(m.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	// Write to temp file first
	tmpPath := m.path + ".tmp"
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, m.path); err != nil {
		os.Remove(tmpPath) // Clean up temp file
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	m.state = state
	return nil
}

// GetAgent retrieves a specific agent's state.
func (m *FileStateManager) GetAgent(name string) (*Agent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	agent, exists := m.state.Agents[name]
	if !exists {
		return nil, fmt.Errorf("agent %q not found", name)
	}

	// Return a copy
	agentCopy := *agent
	return &agentCopy, nil
}

// SetAgent updates or creates an agent's state.
func (m *FileStateManager) SetAgent(agent *Agent) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	agent.UpdatedAt = time.Now()

	// If this is a new agent, set CreatedAt
	if _, exists := m.state.Agents[agent.Name]; !exists {
		agent.CreatedAt = agent.UpdatedAt
	}

	// Make a copy and store it
	agentCopy := *agent
	m.state.Agents[agent.Name] = &agentCopy

	// Save to disk
	return m.saveToDisk()
}

// RemoveAgent deletes an agent from the state.
func (m *FileStateManager) RemoveAgent(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.state.Agents[name]; !exists {
		return fmt.Errorf("agent %q not found", name)
	}

	delete(m.state.Agents, name)

	// Save to disk
	return m.saveToDisk()
}

// ListAgents returns all agents.
func (m *FileStateManager) ListAgents() ([]*Agent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	agents := make([]*Agent, 0, len(m.state.Agents))
	for _, agent := range m.state.Agents {
		agentCopy := *agent
		agents = append(agents, &agentCopy)
	}

	return agents, nil
}

// Reconcile checks actual container state and updates stale entries.
// This should be called periodically (e.g., on tanuki list) to detect
// containers that were removed or stopped outside of Tanuki.
func (m *FileStateManager) Reconcile() error {
	if m.checker == nil {
		return nil // No checker available, skip reconciliation
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	changed := false
	for _, agent := range m.state.Agents {
		if agent.ContainerID == "" {
			continue // No container to check
		}

		exists, running, err := m.checker.ContainerStatus(agent.ContainerID)
		if err != nil {
			// Log error but continue with other agents
			continue
		}

		oldStatus := agent.Status

		if !exists {
			// Container was removed
			agent.Status = StatusError
			changed = true
		} else if !running && (agent.Status == StatusWorking || agent.Status == StatusIdle) {
			// Container stopped unexpectedly
			agent.Status = StatusStopped
			changed = true
		} else if running && agent.Status == StatusStopped {
			// Container was restarted
			agent.Status = StatusIdle
			changed = true
		}

		if oldStatus != agent.Status {
			agent.UpdatedAt = time.Now()
		}
	}

	if changed {
		return m.saveToDisk()
	}

	return nil
}

// loadFromDisk reads the state file from disk.
func (m *FileStateManager) loadFromDisk() (*State, error) {
	data, err := os.ReadFile(m.path)
	if err != nil {
		return nil, err
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state: %w", err)
	}

	// Ensure Agents map is initialized
	if state.Agents == nil {
		state.Agents = make(map[string]*Agent)
	}

	return &state, nil
}

// saveToDisk writes the current state to disk (must be called with lock held).
func (m *FileStateManager) saveToDisk() error {
	// Ensure directory exists
	dir := filepath.Dir(m.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	// Write to temp file first
	tmpPath := m.path + ".tmp"
	data, err := json.MarshalIndent(m.state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, m.path); err != nil {
		os.Remove(tmpPath) // Clean up temp file
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// DefaultStatePath returns the default path for the state file.
func DefaultStatePath() string {
	return filepath.Join(".tanuki", "state", "agents.json")
}

// NewManager creates a new state manager with the default path.
// This is a convenience function for the most common use case.
func NewManager() (Manager, error) {
	return NewFileStateManager(DefaultStatePath(), nil)
}
