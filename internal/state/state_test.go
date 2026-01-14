package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// mockContainerChecker implements ContainerChecker for testing.
type mockContainerChecker struct {
	containers map[string]containerStatus
}

type containerStatus struct {
	exists  bool
	running bool
}

func (m *mockContainerChecker) ContainerStatus(containerID string) (bool, bool, error) {
	status, ok := m.containers[containerID]
	if !ok {
		return false, false, nil
	}
	return status.exists, status.running, nil
}

func TestNewFileStateManager_NewState(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tanuki-state-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	statePath := filepath.Join(tmpDir, ".tanuki", "state", "agents.json")
	mgr, err := NewFileStateManager(statePath, nil)
	if err != nil {
		t.Fatalf("failed to create state manager: %v", err)
	}

	state, err := mgr.Load()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	if state.Version != "1" {
		t.Errorf("expected version '1', got '%s'", state.Version)
	}

	if len(state.Agents) != 0 {
		t.Errorf("expected empty agents map, got %d agents", len(state.Agents))
	}
}

func TestNewFileStateManager_ExistingState(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tanuki-state-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	statePath := filepath.Join(tmpDir, ".tanuki", "state", "agents.json")

	// Create existing state file
	existingState := &State{
		Version: "1",
		Project: "/test/project",
		Agents: map[string]*Agent{
			"test-agent": {
				Name:   "test-agent",
				Status: StatusIdle,
			},
		},
	}

	_ = os.MkdirAll(filepath.Dir(statePath), 0750)
	data, _ := json.Marshal(existingState)
	_ = os.WriteFile(statePath, data, 0600)

	// Load existing state
	mgr, err := NewFileStateManager(statePath, nil)
	if err != nil {
		t.Fatalf("failed to create state manager: %v", err)
	}

	state, err := mgr.Load()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	if len(state.Agents) != 1 {
		t.Errorf("expected 1 agent, got %d", len(state.Agents))
	}

	if agent, exists := state.Agents["test-agent"]; !exists {
		t.Error("expected test-agent to exist")
	} else if agent.Status != StatusIdle {
		t.Errorf("expected status idle, got %s", agent.Status)
	}
}

func TestSetAgent_NewAgent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tanuki-state-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	statePath := filepath.Join(tmpDir, ".tanuki", "state", "agents.json")
	mgr, err := NewFileStateManager(statePath, nil)
	if err != nil {
		t.Fatalf("failed to create state manager: %v", err)
	}

	agent := &Agent{
		Name:          "new-agent",
		ContainerID:   "abc123",
		ContainerName: "tanuki-new-agent",
		Branch:        "tanuki/new-agent",
		WorktreePath:  ".tanuki/worktrees/new-agent",
		Status:        StatusIdle,
	}

	if setErr := mgr.SetAgent(agent); setErr != nil {
		t.Fatalf("failed to set agent: %v", setErr)
	}

	// Verify agent was saved
	retrieved, err := mgr.GetAgent("new-agent")
	if err != nil {
		t.Fatalf("failed to get agent: %v", err)
	}

	if retrieved.Name != agent.Name {
		t.Errorf("expected name '%s', got '%s'", agent.Name, retrieved.Name)
	}

	if retrieved.ContainerID != agent.ContainerID {
		t.Errorf("expected container ID '%s', got '%s'", agent.ContainerID, retrieved.ContainerID)
	}

	// Verify CreatedAt was set
	if retrieved.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}

	// Verify UpdatedAt was set
	if retrieved.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be set")
	}
}

func TestSetAgent_UpdateExisting(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tanuki-state-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	statePath := filepath.Join(tmpDir, ".tanuki", "state", "agents.json")
	mgr, err := NewFileStateManager(statePath, nil)
	if err != nil {
		t.Fatalf("failed to create state manager: %v", err)
	}

	// Create initial agent
	agent := &Agent{
		Name:   "test-agent",
		Status: StatusIdle,
	}
	_ = mgr.SetAgent(agent)

	time.Sleep(10 * time.Millisecond)

	// Update agent
	agent.Status = StatusWorking
	if updateErr := mgr.SetAgent(agent); updateErr != nil {
		t.Fatalf("failed to update agent: %v", updateErr)
	}

	retrieved, err := mgr.GetAgent("test-agent")
	if err != nil {
		t.Fatalf("failed to get agent: %v", err)
	}

	if retrieved.Status != StatusWorking {
		t.Errorf("expected status working, got %s", retrieved.Status)
	}

	// CreatedAt should remain unchanged
	if retrieved.CreatedAt.After(retrieved.UpdatedAt) {
		t.Error("CreatedAt should be before UpdatedAt")
	}
}

func TestRemoveAgent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tanuki-state-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	statePath := filepath.Join(tmpDir, ".tanuki", "state", "agents.json")
	mgr, err := NewFileStateManager(statePath, nil)
	if err != nil {
		t.Fatalf("failed to create state manager: %v", err)
	}

	// Create agent
	agent := &Agent{
		Name:   "test-agent",
		Status: StatusIdle,
	}
	_ = mgr.SetAgent(agent)

	// Remove agent
	if removeErr := mgr.RemoveAgent("test-agent"); removeErr != nil {
		t.Fatalf("failed to remove agent: %v", removeErr)
	}

	// Verify agent is gone
	_, err = mgr.GetAgent("test-agent")
	if err == nil {
		t.Error("expected error when getting removed agent")
	}

	agents, _ := mgr.ListAgents()
	if len(agents) != 0 {
		t.Errorf("expected 0 agents, got %d", len(agents))
	}
}

func TestRemoveAgent_NotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tanuki-state-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	statePath := filepath.Join(tmpDir, ".tanuki", "state", "agents.json")
	mgr, err := NewFileStateManager(statePath, nil)
	if err != nil {
		t.Fatalf("failed to create state manager: %v", err)
	}

	err = mgr.RemoveAgent("nonexistent")
	if err == nil {
		t.Error("expected error when removing nonexistent agent")
	}
}

func TestListAgents(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tanuki-state-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	statePath := filepath.Join(tmpDir, ".tanuki", "state", "agents.json")
	mgr, err := NewFileStateManager(statePath, nil)
	if err != nil {
		t.Fatalf("failed to create state manager: %v", err)
	}

	// Create multiple agents
	for i, name := range []string{"agent-1", "agent-2", "agent-3"} {
		agent := &Agent{
			Name:        name,
			ContainerID: string(rune('a' + i)),
			Status:      StatusIdle,
		}
		_ = mgr.SetAgent(agent)
	}

	agents, err := mgr.ListAgents()
	if err != nil {
		t.Fatalf("failed to list agents: %v", err)
	}

	if len(agents) != 3 {
		t.Errorf("expected 3 agents, got %d", len(agents))
	}

	// Verify all agents are present
	names := make(map[string]bool)
	for _, agent := range agents {
		names[agent.Name] = true
	}

	for _, expected := range []string{"agent-1", "agent-2", "agent-3"} {
		if !names[expected] {
			t.Errorf("expected agent '%s' in list", expected)
		}
	}
}

func TestAtomicWrite(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tanuki-state-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	statePath := filepath.Join(tmpDir, ".tanuki", "state", "agents.json")
	mgr, err := NewFileStateManager(statePath, nil)
	if err != nil {
		t.Fatalf("failed to create state manager: %v", err)
	}

	// Set an agent
	agent := &Agent{
		Name:   "test-agent",
		Status: StatusIdle,
	}
	_ = mgr.SetAgent(agent)

	// Verify no .tmp file exists after successful write
	tmpPath := statePath + ".tmp"
	if _, err := os.Stat(tmpPath); err == nil {
		t.Error("expected temp file to be removed after atomic write")
	}

	// Verify state file exists
	if _, err := os.Stat(statePath); err != nil {
		t.Error("expected state file to exist")
	}
}

func TestReconcile_ContainerRemoved(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tanuki-state-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	statePath := filepath.Join(tmpDir, ".tanuki", "state", "agents.json")

	checker := &mockContainerChecker{
		containers: map[string]containerStatus{
			"abc123": {exists: false, running: false},
		},
	}

	mgr, err := NewFileStateManager(statePath, checker)
	if err != nil {
		t.Fatalf("failed to create state manager: %v", err)
	}

	// Create agent with container ID
	agent := &Agent{
		Name:        "test-agent",
		ContainerID: "abc123",
		Status:      StatusIdle,
	}
	_ = mgr.SetAgent(agent)

	// Reconcile should detect container is gone
	if err := mgr.Reconcile(); err != nil {
		t.Fatalf("failed to reconcile: %v", err)
	}

	retrieved, _ := mgr.GetAgent("test-agent")
	if retrieved.Status != StatusError {
		t.Errorf("expected status error after reconcile, got %s", retrieved.Status)
	}
}

func TestReconcile_ContainerStopped(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tanuki-state-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	statePath := filepath.Join(tmpDir, ".tanuki", "state", "agents.json")

	checker := &mockContainerChecker{
		containers: map[string]containerStatus{
			"abc123": {exists: true, running: false},
		},
	}

	mgr, err := NewFileStateManager(statePath, checker)
	if err != nil {
		t.Fatalf("failed to create state manager: %v", err)
	}

	// Create agent that thinks it's working
	agent := &Agent{
		Name:        "test-agent",
		ContainerID: "abc123",
		Status:      StatusWorking,
	}
	_ = mgr.SetAgent(agent)

	// Reconcile should detect container is stopped
	if err := mgr.Reconcile(); err != nil {
		t.Fatalf("failed to reconcile: %v", err)
	}

	retrieved, _ := mgr.GetAgent("test-agent")
	if retrieved.Status != StatusStopped {
		t.Errorf("expected status stopped after reconcile, got %s", retrieved.Status)
	}
}

func TestReconcile_ContainerRestarted(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tanuki-state-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	statePath := filepath.Join(tmpDir, ".tanuki", "state", "agents.json")

	checker := &mockContainerChecker{
		containers: map[string]containerStatus{
			"abc123": {exists: true, running: true},
		},
	}

	mgr, err := NewFileStateManager(statePath, checker)
	if err != nil {
		t.Fatalf("failed to create state manager: %v", err)
	}

	// Create agent that was stopped
	agent := &Agent{
		Name:        "test-agent",
		ContainerID: "abc123",
		Status:      StatusStopped,
	}
	_ = mgr.SetAgent(agent)

	// Reconcile should detect container is running again
	if err := mgr.Reconcile(); err != nil {
		t.Fatalf("failed to reconcile: %v", err)
	}

	retrieved, _ := mgr.GetAgent("test-agent")
	if retrieved.Status != StatusIdle {
		t.Errorf("expected status idle after reconcile, got %s", retrieved.Status)
	}
}

func TestReconcile_NoChecker(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tanuki-state-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	statePath := filepath.Join(tmpDir, ".tanuki", "state", "agents.json")
	mgr, err := NewFileStateManager(statePath, nil)
	if err != nil {
		t.Fatalf("failed to create state manager: %v", err)
	}

	// Reconcile with no checker should not error
	if err := mgr.Reconcile(); err != nil {
		t.Errorf("expected no error with nil checker, got: %v", err)
	}
}

func TestStatePersistence(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tanuki-state-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	statePath := filepath.Join(tmpDir, ".tanuki", "state", "agents.json")

	// Create manager and add agent
	mgr1, err := NewFileStateManager(statePath, nil)
	if err != nil {
		t.Fatalf("failed to create state manager: %v", err)
	}

	agent := &Agent{
		Name:        "persistent-agent",
		ContainerID: "xyz789",
		Status:      StatusIdle,
	}
	_ = mgr1.SetAgent(agent)

	// Create new manager (simulating CLI restart)
	mgr2, err := NewFileStateManager(statePath, nil)
	if err != nil {
		t.Fatalf("failed to create second state manager: %v", err)
	}

	// Verify agent persisted
	retrieved, err := mgr2.GetAgent("persistent-agent")
	if err != nil {
		t.Fatalf("failed to get agent from new manager: %v", err)
	}

	if retrieved.ContainerID != "xyz789" {
		t.Errorf("expected container ID 'xyz789', got '%s'", retrieved.ContainerID)
	}
}

func TestTaskInfo(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tanuki-state-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	statePath := filepath.Join(tmpDir, ".tanuki", "state", "agents.json")
	mgr, err := NewFileStateManager(statePath, nil)
	if err != nil {
		t.Fatalf("failed to create state manager: %v", err)
	}

	completedAt := time.Now()
	agent := &Agent{
		Name:   "test-agent",
		Status: StatusIdle,
		LastTask: &TaskInfo{
			Prompt:      "Test task",
			StartedAt:   time.Now().Add(-5 * time.Minute),
			CompletedAt: &completedAt,
			SessionID:   "session-123",
		},
	}

	_ = mgr.SetAgent(agent)

	retrieved, err := mgr.GetAgent("test-agent")
	if err != nil {
		t.Fatalf("failed to get agent: %v", err)
	}

	if retrieved.LastTask == nil {
		t.Fatal("expected LastTask to be set")
	}

	if retrieved.LastTask.Prompt != "Test task" {
		t.Errorf("expected prompt 'Test task', got '%s'", retrieved.LastTask.Prompt)
	}

	if retrieved.LastTask.SessionID != "session-123" {
		t.Errorf("expected session ID 'session-123', got '%s'", retrieved.LastTask.SessionID)
	}

	if retrieved.LastTask.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}
}

func TestDefaultStatePath(t *testing.T) {
	path := DefaultStatePath()
	expected := filepath.Join(".tanuki", "state", "agents.json")

	if path != expected {
		t.Errorf("expected path '%s', got '%s'", expected, path)
	}
}
