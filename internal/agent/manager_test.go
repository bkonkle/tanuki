package agent

import (
	"errors"
	"io"
	"testing"
	"time"

	"github.com/bkonkle/tanuki/internal/config"
	"github.com/bkonkle/tanuki/internal/docker"
	"github.com/bkonkle/tanuki/internal/executor"
)

// Mock implementations for testing

type mockGitManager struct {
	createWorktreeFn   func(name string) (string, error)
	removeWorktreeFn   func(name string, deleteBranch bool) error
	getDiffFn          func(name string, baseBranch string) (string, error)
	getStatusFn        func(name string) (string, error)
	getCurrentBranchFn func() (string, error)
	getMainBranchFn    func() (string, error)
	worktreeExistsFn   func(name string) bool
	branchExistsFn     func(name string) bool
	getWorktreePathFn  func(name string) string
	getBranchNameFn    func(name string) string
}

func (m *mockGitManager) CreateWorktree(name string) (string, error) {
	if m.createWorktreeFn != nil {
		return m.createWorktreeFn(name)
	}
	return "/test/worktree/" + name, nil
}

func (m *mockGitManager) RemoveWorktree(name string, deleteBranch bool) error {
	if m.removeWorktreeFn != nil {
		return m.removeWorktreeFn(name, deleteBranch)
	}
	return nil
}

func (m *mockGitManager) GetDiff(name string, baseBranch string) (string, error) {
	if m.getDiffFn != nil {
		return m.getDiffFn(name, baseBranch)
	}
	return "", nil
}

func (m *mockGitManager) GetStatus(name string) (string, error) {
	if m.getStatusFn != nil {
		return m.getStatusFn(name)
	}
	return "", nil
}

func (m *mockGitManager) GetCurrentBranch() (string, error) {
	if m.getCurrentBranchFn != nil {
		return m.getCurrentBranchFn()
	}
	return "main", nil
}

func (m *mockGitManager) GetMainBranch() (string, error) {
	if m.getMainBranchFn != nil {
		return m.getMainBranchFn()
	}
	return "main", nil
}

func (m *mockGitManager) WorktreeExists(name string) bool {
	if m.worktreeExistsFn != nil {
		return m.worktreeExistsFn(name)
	}
	return false
}

func (m *mockGitManager) BranchExists(name string) bool {
	if m.branchExistsFn != nil {
		return m.branchExistsFn(name)
	}
	return false
}

func (m *mockGitManager) GetWorktreePath(name string) string {
	if m.getWorktreePathFn != nil {
		return m.getWorktreePathFn(name)
	}
	return "/test/worktree/" + name
}

func (m *mockGitManager) GetBranchName(name string) string {
	if m.getBranchNameFn != nil {
		return m.getBranchNameFn(name)
	}
	return "tanuki/" + name
}

type mockDockerManager struct {
	ensureNetworkFn                   func(name string) error
	createAgentContainerFn            func(name string, worktreePath string) (string, error)
	createAgentContainerWithOptionsFn func(name string, worktreePath string, opts docker.AgentContainerOptions) (string, error)
	startContainerFn                  func(containerID string) error
	stopContainerFn                   func(containerID string) error
	removeContainerFn                 func(containerID string) error
	containerExistsFn                 func(containerID string) bool
	containerRunningFn                func(containerID string) bool
	inspectContainerFn                func(containerID string) (*ContainerInfo, error)
	execWithOutputFn                  func(containerID string, cmd []string) (string, error)
	getResourceUsageFn                func(containerID string) (*ResourceUsage, error)
}

func (m *mockDockerManager) EnsureNetwork(name string) error {
	if m.ensureNetworkFn != nil {
		return m.ensureNetworkFn(name)
	}
	return nil
}

func (m *mockDockerManager) CreateAgentContainer(name string, worktreePath string) (string, error) {
	if m.createAgentContainerFn != nil {
		return m.createAgentContainerFn(name, worktreePath)
	}
	return "container-" + name, nil
}

func (m *mockDockerManager) CreateAgentContainerWithOptions(name string, worktreePath string, opts docker.AgentContainerOptions) (string, error) {
	if m.createAgentContainerWithOptionsFn != nil {
		return m.createAgentContainerWithOptionsFn(name, worktreePath, opts)
	}
	return "container-" + name, nil
}

func (m *mockDockerManager) StartContainer(containerID string) error {
	if m.startContainerFn != nil {
		return m.startContainerFn(containerID)
	}
	return nil
}

func (m *mockDockerManager) StopContainer(containerID string) error {
	if m.stopContainerFn != nil {
		return m.stopContainerFn(containerID)
	}
	return nil
}

func (m *mockDockerManager) RemoveContainer(containerID string) error {
	if m.removeContainerFn != nil {
		return m.removeContainerFn(containerID)
	}
	return nil
}

func (m *mockDockerManager) ContainerExists(containerID string) bool {
	if m.containerExistsFn != nil {
		return m.containerExistsFn(containerID)
	}
	return true
}

func (m *mockDockerManager) ContainerRunning(containerID string) bool {
	if m.containerRunningFn != nil {
		return m.containerRunningFn(containerID)
	}
	return true
}

func (m *mockDockerManager) InspectContainer(containerID string) (*ContainerInfo, error) {
	if m.inspectContainerFn != nil {
		return m.inspectContainerFn(containerID)
	}
	return &ContainerInfo{
		ID:      containerID,
		Name:    "tanuki-test",
		State:   "running",
		Status:  "Up 5 minutes",
		Image:   "bkonkle/tanuki:latest",
		Created: "2026-01-13T10:00:00Z",
	}, nil
}

func (m *mockDockerManager) ExecWithOutput(containerID string, cmd []string) (string, error) {
	if m.execWithOutputFn != nil {
		return m.execWithOutputFn(containerID, cmd)
	}
	return "", nil
}

func (m *mockDockerManager) GetResourceUsage(containerID string) (*ResourceUsage, error) {
	if m.getResourceUsageFn != nil {
		return m.getResourceUsageFn(containerID)
	}
	return &ResourceUsage{
		Memory: "50MB",
		CPU:    "1.5%",
	}, nil
}

type mockExecutor struct {
	runFn            func(containerID string, prompt string, opts executor.ExecuteOptions) (*executor.ExecutionResult, error)
	runFollowFn      func(containerID string, prompt string, opts executor.ExecuteOptions, output io.Writer) (*executor.ExecutionResult, error)
	checkContainerFn func(containerID string) error
}

func (m *mockExecutor) Run(containerID string, prompt string, opts executor.ExecuteOptions) (*executor.ExecutionResult, error) {
	if m.runFn != nil {
		return m.runFn(containerID, prompt, opts)
	}
	return &executor.ExecutionResult{
		SessionID:   "test-session-123",
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
	}, nil
}

func (m *mockExecutor) RunFollow(containerID string, prompt string, opts executor.ExecuteOptions, output io.Writer) (*executor.ExecutionResult, error) {
	if m.runFollowFn != nil {
		return m.runFollowFn(containerID, prompt, opts, output)
	}
	return &executor.ExecutionResult{
		SessionID:   "test-session-123",
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
	}, nil
}

func (m *mockExecutor) CheckContainer(containerID string) error {
	if m.checkContainerFn != nil {
		return m.checkContainerFn(containerID)
	}
	return nil
}

type mockStateManager struct {
	agents        map[string]*Agent
	loadFn        func() (*State, error)
	saveFn        func(state *State) error
	getAgentFn    func(name string) (*Agent, error)
	setAgentFn    func(agent *Agent) error
	removeAgentFn func(name string) error
	listAgentsFn  func() ([]*Agent, error)
}

func newMockStateManager() *mockStateManager {
	return &mockStateManager{
		agents: make(map[string]*Agent),
	}
}

func (m *mockStateManager) Load() (*State, error) {
	if m.loadFn != nil {
		return m.loadFn()
	}
	return &State{
		Version: "1",
		Project: "/test/project",
		Agents:  m.agents,
	}, nil
}

func (m *mockStateManager) Save(state *State) error {
	if m.saveFn != nil {
		return m.saveFn(state)
	}
	return nil
}

func (m *mockStateManager) GetAgent(name string) (*Agent, error) {
	if m.getAgentFn != nil {
		return m.getAgentFn(name)
	}
	agent, exists := m.agents[name]
	if !exists {
		return nil, errors.New("agent not found")
	}
	return agent, nil
}

func (m *mockStateManager) SetAgent(agent *Agent) error {
	if m.setAgentFn != nil {
		return m.setAgentFn(agent)
	}
	m.agents[agent.Name] = agent
	return nil
}

func (m *mockStateManager) RemoveAgent(name string) error {
	if m.removeAgentFn != nil {
		return m.removeAgentFn(name)
	}
	delete(m.agents, name)
	return nil
}

func (m *mockStateManager) ListAgents() ([]*Agent, error) {
	if m.listAgentsFn != nil {
		return m.listAgentsFn()
	}
	agents := make([]*Agent, 0, len(m.agents))
	for _, agent := range m.agents {
		agents = append(agents, agent)
	}
	return agents, nil
}

func testConfig() *config.Config {
	return config.DefaultConfig()
}

func TestNewManager(t *testing.T) {
	cfg := testConfig()
	git := &mockGitManager{}
	docker := &mockDockerManager{}
	state := newMockStateManager()
	executor := &mockExecutor{}

	manager, err := NewManager(cfg, git, docker, state, executor)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}
	if manager == nil {
		t.Fatal("NewManager returned nil")
	}
}

func TestNewManager_MissingDependencies(t *testing.T) {
	cfg := testConfig()
	git := &mockGitManager{}
	docker := &mockDockerManager{}
	state := newMockStateManager()
	executor := &mockExecutor{}

	tests := []struct {
		name     string
		cfg      *config.Config
		git      GitManager
		docker   DockerManager
		state    StateManager
		executor ClaudeExecutor
	}{
		{"nil config", nil, git, docker, state, executor},
		{"nil git", cfg, nil, docker, state, executor},
		{"nil docker", cfg, git, nil, state, executor},
		{"nil state", cfg, git, docker, nil, executor},
		{"nil executor", cfg, git, docker, state, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewManager(tt.cfg, tt.git, tt.docker, tt.state, tt.executor)
			if err == nil {
				t.Errorf("expected error for %s, got nil", tt.name)
			}
		})
	}
}

func TestSpawn(t *testing.T) {
	cfg := testConfig()
	git := &mockGitManager{}
	docker := &mockDockerManager{}
	state := newMockStateManager()

	executor := &mockExecutor{}
	manager, _ := NewManager(cfg, git, docker, state, executor)

	agent, err := manager.Spawn("test-agent", SpawnOptions{})
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	if agent.Name != "test-agent" {
		t.Errorf("expected agent name 'test-agent', got %q", agent.Name)
	}
	if agent.Status != "idle" {
		t.Errorf("expected status 'idle', got %q", agent.Status)
	}
	if agent.ContainerID != "container-test-agent" {
		t.Errorf("expected container ID 'container-test-agent', got %q", agent.ContainerID)
	}
}

func TestSpawn_InvalidName(t *testing.T) {
	cfg := testConfig()
	git := &mockGitManager{}
	docker := &mockDockerManager{}
	state := newMockStateManager()

	executor := &mockExecutor{}
	manager, _ := NewManager(cfg, git, docker, state, executor)

	invalidNames := []string{
		"A",          // Too short
		"Agent",      // Capital letter
		"agent_name", // Underscore
		"agent name", // Space
		"-agent",     // Starts with hyphen
		"agent-",     // Ends with hyphen
		"a",          // Single character
	}

	for _, name := range invalidNames {
		t.Run(name, func(t *testing.T) {
			_, err := manager.Spawn(name, SpawnOptions{})
			if err == nil {
				t.Errorf("expected error for invalid name %q, got nil", name)
			}
			if !errors.Is(err, ErrInvalidName) {
				t.Errorf("expected ErrInvalidName, got %v", err)
			}
		})
	}
}

func TestSpawn_AlreadyExists(t *testing.T) {
	cfg := testConfig()
	git := &mockGitManager{}
	docker := &mockDockerManager{}
	state := newMockStateManager()

	executor := &mockExecutor{}
	manager, _ := NewManager(cfg, git, docker, state, executor)

	// Create first agent
	_, err := manager.Spawn("test-agent", SpawnOptions{})
	if err != nil {
		t.Fatalf("first Spawn failed: %v", err)
	}

	// Try to create duplicate
	_, err = manager.Spawn("test-agent", SpawnOptions{})
	if err == nil {
		t.Fatal("expected error for duplicate agent, got nil")
	}
	if !errors.Is(err, ErrAgentExists) {
		t.Errorf("expected ErrAgentExists, got %v", err)
	}
}

func TestSpawn_WorktreeFailure_Rollback(t *testing.T) {
	cfg := testConfig()
	git := &mockGitManager{
		createWorktreeFn: func(name string) (string, error) {
			return "", errors.New("worktree creation failed")
		},
	}
	docker := &mockDockerManager{}
	state := newMockStateManager()

	executor := &mockExecutor{}
	manager, _ := NewManager(cfg, git, docker, state, executor)

	_, err := manager.Spawn("test-agent", SpawnOptions{})
	if err == nil {
		t.Fatal("expected error for worktree failure, got nil")
	}

	// Verify agent not in state
	_, err = state.GetAgent("test-agent")
	if err == nil {
		t.Error("expected agent not to be in state after rollback")
	}
}

func TestSpawn_ContainerFailure_Rollback(t *testing.T) {
	cfg := testConfig()

	worktreeRemoved := false
	git := &mockGitManager{
		removeWorktreeFn: func(name string, deleteBranch bool) error {
			worktreeRemoved = true
			return nil
		},
	}

	docker := &mockDockerManager{
		createAgentContainerWithOptionsFn: func(name string, worktreePath string, opts docker.AgentContainerOptions) (string, error) {
			return "", errors.New("container creation failed")
		},
	}
	state := newMockStateManager()

	executor := &mockExecutor{}
	manager, _ := NewManager(cfg, git, docker, state, executor)

	_, err := manager.Spawn("test-agent", SpawnOptions{})
	if err == nil {
		t.Fatal("expected error for container failure, got nil")
	}

	// Verify rollback occurred
	if !worktreeRemoved {
		t.Error("expected worktree to be removed during rollback")
	}

	// Verify agent not in state
	_, err = state.GetAgent("test-agent")
	if err == nil {
		t.Error("expected agent not to be in state after rollback")
	}
}

func TestRemove(t *testing.T) {
	cfg := testConfig()
	git := &mockGitManager{}
	docker := &mockDockerManager{}
	state := newMockStateManager()

	executor := &mockExecutor{}
	manager, _ := NewManager(cfg, git, docker, state, executor)

	// Create agent
	_, err := manager.Spawn("test-agent", SpawnOptions{})
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	// Remove agent
	err = manager.Remove("test-agent", RemoveOptions{})
	if err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	// Verify agent removed from state
	_, err = state.GetAgent("test-agent")
	if err == nil {
		t.Error("expected agent to be removed from state")
	}
}

func TestRemove_NotFound(t *testing.T) {
	cfg := testConfig()
	git := &mockGitManager{}
	docker := &mockDockerManager{}
	state := newMockStateManager()

	executor := &mockExecutor{}
	manager, _ := NewManager(cfg, git, docker, state, executor)

	err := manager.Remove("nonexistent", RemoveOptions{})
	if err == nil {
		t.Fatal("expected error for nonexistent agent, got nil")
	}
	if !errors.Is(err, ErrAgentNotFound) {
		t.Errorf("expected ErrAgentNotFound, got %v", err)
	}
}

func TestRemove_Working_WithoutForce(t *testing.T) {
	cfg := testConfig()
	git := &mockGitManager{}
	docker := &mockDockerManager{}
	state := newMockStateManager()

	executor := &mockExecutor{}
	manager, _ := NewManager(cfg, git, docker, state, executor)

	// Create agent and set to working
	agent, _ := manager.Spawn("test-agent", SpawnOptions{})
	agent.Status = "working"
	_ = state.SetAgent(agent)

	// Try to remove without force
	err := manager.Remove("test-agent", RemoveOptions{Force: false})
	if err == nil {
		t.Fatal("expected error for removing working agent, got nil")
	}
	if !errors.Is(err, ErrAgentWorking) {
		t.Errorf("expected ErrAgentWorking, got %v", err)
	}
}

func TestRemove_Working_WithForce(t *testing.T) {
	cfg := testConfig()
	git := &mockGitManager{}
	docker := &mockDockerManager{}
	state := newMockStateManager()

	executor := &mockExecutor{}
	manager, _ := NewManager(cfg, git, docker, state, executor)

	// Create agent and set to working
	agent, _ := manager.Spawn("test-agent", SpawnOptions{})
	agent.Status = "working"
	_ = state.SetAgent(agent)

	// Remove with force
	err := manager.Remove("test-agent", RemoveOptions{Force: true})
	if err != nil {
		t.Fatalf("Remove with force failed: %v", err)
	}
}

func TestStopStart(t *testing.T) {
	cfg := testConfig()
	git := &mockGitManager{}
	docker := &mockDockerManager{}
	state := newMockStateManager()

	executor := &mockExecutor{}
	manager, _ := NewManager(cfg, git, docker, state, executor)

	// Create agent
	agent, _ := manager.Spawn("test-agent", SpawnOptions{})

	// Stop agent
	err := manager.Stop("test-agent")
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// Verify status
	agent, _ = state.GetAgent("test-agent")
	if agent.Status != "stopped" {
		t.Errorf("expected status 'stopped', got %q", agent.Status)
	}

	// Start agent
	err = manager.Start("test-agent")
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Verify status
	agent, _ = state.GetAgent("test-agent")
	if agent.Status != "idle" {
		t.Errorf("expected status 'idle', got %q", agent.Status)
	}
}

func TestList(t *testing.T) {
	cfg := testConfig()
	git := &mockGitManager{}
	docker := &mockDockerManager{}
	state := newMockStateManager()

	executor := &mockExecutor{}
	manager, _ := NewManager(cfg, git, docker, state, executor)

	// Create multiple agents
	_, _ = manager.Spawn("agent-01", SpawnOptions{})
	_, _ = manager.Spawn("agent-02", SpawnOptions{})
	_, _ = manager.Spawn("agent-03", SpawnOptions{})

	agents, err := manager.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(agents) != 3 {
		t.Errorf("expected 3 agents, got %d", len(agents))
	}
}

func TestStatus(t *testing.T) {
	cfg := testConfig()
	git := &mockGitManager{}
	docker := &mockDockerManager{}
	state := newMockStateManager()

	executor := &mockExecutor{}
	manager, _ := NewManager(cfg, git, docker, state, executor)

	// Create agent
	_, err := manager.Spawn("test-agent", SpawnOptions{})
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	// Get status
	status, err := manager.Status("test-agent")
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}

	if status.Name != "test-agent" {
		t.Errorf("expected name 'test-agent', got %q", status.Name)
	}
	if status.Status != "idle" {
		t.Errorf("expected status 'idle', got %q", status.Status)
	}
	if !status.Container.Running {
		t.Error("expected container to be running")
	}
}

func TestReconcile(t *testing.T) {
	cfg := testConfig()
	git := &mockGitManager{}

	containerExists := true
	docker := &mockDockerManager{
		containerExistsFn: func(containerID string) bool {
			return containerExists
		},
		containerRunningFn: func(containerID string) bool {
			return false
		},
	}
	state := newMockStateManager()

	executor := &mockExecutor{}
	manager, _ := NewManager(cfg, git, docker, state, executor)

	// Create agent with working status
	agent, _ := manager.Spawn("test-agent", SpawnOptions{})
	agent.Status = "working"
	_ = state.SetAgent(agent)

	// Reconcile (container not running, should update status)
	err := manager.Reconcile()
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	// Check status updated
	agent, _ = state.GetAgent("test-agent")
	if agent.Status != "stopped" {
		t.Errorf("expected status 'stopped' after reconcile, got %q", agent.Status)
	}

	// Test with container removed
	containerExists = false
	err = manager.Reconcile()
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	agent, _ = state.GetAgent("test-agent")
	if string(agent.Status) != "error" {
		t.Errorf("expected status 'error' after reconcile, got %q", agent.Status)
	}
}

func TestValidateAgentName(t *testing.T) {
	tests := []struct {
		name  string
		valid bool
	}{
		{"test-agent", true},
		{"agent01", true},
		{"a1", true},
		{"my-agent-123", true},
		{"a", false},                      // Too short
		{"Agent", false},                  // Capital letter
		{"-agent", false},                 // Starts with hyphen
		{"agent-", false},                 // Ends with hyphen
		{"agent_name", false},             // Underscore
		{"agent name", false},             // Space
		{"agent.name", false},             // Dot
		{string(make([]byte, 64)), false}, // Too long
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAgentName(tt.name)
			if tt.valid && err != nil {
				t.Errorf("expected %q to be valid, got error: %v", tt.name, err)
			}
			if !tt.valid && err == nil {
				t.Errorf("expected %q to be invalid, got no error", tt.name)
			}
		})
	}
}

func TestIsWorking(t *testing.T) {
	cfg := testConfig()
	git := &mockGitManager{}
	docker := &mockDockerManager{}
	state := newMockStateManager()

	executor := &mockExecutor{}
	manager, _ := NewManager(cfg, git, docker, state, executor)

	// Create agent
	agent, _ := manager.Spawn("test-agent", SpawnOptions{})

	// Check not working
	working, err := manager.IsWorking("test-agent")
	if err != nil {
		t.Fatalf("IsWorking failed: %v", err)
	}
	if working {
		t.Error("expected agent not to be working")
	}

	// Set to working
	agent.Status = "working"
	_ = state.SetAgent(agent)

	// Check working
	working, err = manager.IsWorking("test-agent")
	if err != nil {
		t.Fatalf("IsWorking failed: %v", err)
	}
	if !working {
		t.Error("expected agent to be working")
	}
}

func TestRun(t *testing.T) {
	cfg := testConfig()
	git := &mockGitManager{}
	docker := &mockDockerManager{}
	state := newMockStateManager()

	executor := &mockExecutor{}
	manager, _ := NewManager(cfg, git, docker, state, executor)

	// Create agent
	agent, err := manager.Spawn("test-agent", SpawnOptions{})
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	// Run task
	err = manager.Run("test-agent", "test prompt", RunOptions{})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Verify agent returned to idle status
	agent, _ = state.GetAgent("test-agent")
	if agent.Status != "idle" {
		t.Errorf("expected status 'idle' after run, got %q", agent.Status)
	}

	// Verify last task was recorded
	if agent.LastTask == nil {
		t.Fatal("expected LastTask to be set")
	}
	if agent.LastTask.Prompt != "test prompt" {
		t.Errorf("expected prompt 'test prompt', got %q", agent.LastTask.Prompt)
	}
	if agent.LastTask.SessionID != "test-session-123" {
		t.Errorf("expected session ID 'test-session-123', got %q", agent.LastTask.SessionID)
	}
}

func TestAgent_UpdatedAt(t *testing.T) {
	cfg := testConfig()
	git := &mockGitManager{}
	docker := &mockDockerManager{}
	state := newMockStateManager()

	executor := &mockExecutor{}
	manager, _ := NewManager(cfg, git, docker, state, executor)

	// Create agent
	agent, _ := manager.Spawn("test-agent", SpawnOptions{})
	createdAt := agent.CreatedAt
	updatedAt := agent.UpdatedAt

	// Wait a moment
	time.Sleep(10 * time.Millisecond)

	// Stop agent (should update UpdatedAt)
	_ = manager.Stop("test-agent")
	agent, _ = state.GetAgent("test-agent")

	if agent.CreatedAt != createdAt {
		t.Error("CreatedAt should not change")
	}
	if agent.UpdatedAt == updatedAt {
		t.Error("UpdatedAt should be updated")
	}
}
