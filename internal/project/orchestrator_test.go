package project

import (
	"context"
	"testing"
	"time"

	"github.com/bkonkle/tanuki/internal/agent"
	"github.com/bkonkle/tanuki/internal/task"
)

// Mock implementations for testing

type mockTaskManager struct {
	tasks map[string]*task.Task
}

func newMockTaskManager() *mockTaskManager {
	return &mockTaskManager{
		tasks: make(map[string]*task.Task),
	}
}

func (m *mockTaskManager) addTask(t *task.Task) {
	m.tasks[t.ID] = t
}

func (m *mockTaskManager) Scan() ([]*task.Task, error) {
	var tasks []*task.Task
	for _, t := range m.tasks {
		tasks = append(tasks, t)
	}
	return tasks, nil
}

func (m *mockTaskManager) Get(id string) (*task.Task, error) {
	t, ok := m.tasks[id]
	if !ok {
		return nil, &task.ValidationError{Message: "not found"}
	}
	return t, nil
}

func (m *mockTaskManager) GetByRole(role string) []*task.Task {
	var tasks []*task.Task
	for _, t := range m.tasks {
		if t.Role == role {
			tasks = append(tasks, t)
		}
	}
	return tasks
}

func (m *mockTaskManager) GetByStatus(status task.Status) []*task.Task {
	var tasks []*task.Task
	for _, t := range m.tasks {
		if t.Status == status {
			tasks = append(tasks, t)
		}
	}
	return tasks
}

func (m *mockTaskManager) GetPending() []*task.Task {
	return m.GetByStatus(task.StatusPending)
}

func (m *mockTaskManager) UpdateStatus(id string, status task.Status) error {
	t, ok := m.tasks[id]
	if !ok {
		return &task.ValidationError{Message: "not found"}
	}
	t.Status = status
	return nil
}

func (m *mockTaskManager) Assign(id string, agentName string) error {
	t, ok := m.tasks[id]
	if !ok {
		return &task.ValidationError{Message: "not found"}
	}
	t.AssignedTo = agentName
	t.Status = task.StatusAssigned
	return nil
}

func (m *mockTaskManager) Unassign(id string) error {
	t, ok := m.tasks[id]
	if !ok {
		return &task.ValidationError{Message: "not found"}
	}
	t.AssignedTo = ""
	return nil
}

func (m *mockTaskManager) IsBlocked(id string) (bool, error) {
	return false, nil
}

func (m *mockTaskManager) Stats() *TaskStats {
	stats := &TaskStats{
		ByStatus:   make(map[task.Status]int),
		ByRole:     make(map[string]int),
		ByPriority: make(map[task.Priority]int),
	}
	for _, t := range m.tasks {
		stats.Total++
		stats.ByStatus[t.Status]++
		stats.ByRole[t.Role]++
		stats.ByPriority[t.Priority]++
	}
	return stats
}

type mockAgentManager struct {
	agents map[string]*agent.Agent
}

func newMockAgentManager() *mockAgentManager {
	return &mockAgentManager{
		agents: make(map[string]*agent.Agent),
	}
}

func (m *mockAgentManager) addAgent(ag *agent.Agent) {
	m.agents[ag.Name] = ag
}

func (m *mockAgentManager) Spawn(name string, opts agent.SpawnOptions) (*agent.Agent, error) {
	ag := &agent.Agent{
		Name:   name,
		Role:   opts.Role,
		Status: "idle",
	}
	m.agents[name] = ag
	return ag, nil
}

func (m *mockAgentManager) Get(name string) (*agent.Agent, error) {
	ag, ok := m.agents[name]
	if !ok {
		return nil, agent.ErrAgentNotFound
	}
	return ag, nil
}

func (m *mockAgentManager) List() ([]*agent.Agent, error) {
	var agents []*agent.Agent
	for _, ag := range m.agents {
		agents = append(agents, ag)
	}
	return agents, nil
}

func (m *mockAgentManager) Start(name string) error {
	ag, ok := m.agents[name]
	if !ok {
		return agent.ErrAgentNotFound
	}
	ag.Status = "idle"
	return nil
}

func (m *mockAgentManager) Stop(name string) error {
	ag, ok := m.agents[name]
	if !ok {
		return agent.ErrAgentNotFound
	}
	ag.Status = "stopped"
	return nil
}

func (m *mockAgentManager) Remove(name string, opts agent.RemoveOptions) error {
	delete(m.agents, name)
	return nil
}

func (m *mockAgentManager) Run(name string, prompt string, opts agent.RunOptions) error {
	return nil
}

type mockTaskQueue struct {
	tasks map[string]*task.Task
}

func newMockTaskQueue() *mockTaskQueue {
	return &mockTaskQueue{
		tasks: make(map[string]*task.Task),
	}
}

func (m *mockTaskQueue) Enqueue(t *task.Task) error {
	m.tasks[t.ID] = t
	return nil
}

func (m *mockTaskQueue) Dequeue(role string) (*task.Task, error) {
	for id, t := range m.tasks {
		if t.Role == role {
			delete(m.tasks, id)
			return t, nil
		}
	}
	return nil, &task.ValidationError{Message: "no tasks for role"}
}

func (m *mockTaskQueue) Peek(role string) (*task.Task, error) {
	for _, t := range m.tasks {
		if t.Role == role {
			return t, nil
		}
	}
	return nil, &task.ValidationError{Message: "no tasks for role"}
}

func (m *mockTaskQueue) Size() int {
	return len(m.tasks)
}

func (m *mockTaskQueue) SizeByRole(role string) int {
	count := 0
	for _, t := range m.tasks {
		if t.Role == role {
			count++
		}
	}
	return count
}

func (m *mockTaskQueue) Contains(taskID string) bool {
	_, ok := m.tasks[taskID]
	return ok
}

func (m *mockTaskQueue) Clear() {
	m.tasks = make(map[string]*task.Task)
}

// Tests

func TestNewOrchestrator(t *testing.T) {
	taskMgr := newMockTaskManager()
	agentMgr := newMockAgentManager()
	queue := newMockTaskQueue()
	config := DefaultOrchestratorConfig()

	orch := NewOrchestrator(taskMgr, agentMgr, queue, config)

	if orch == nil {
		t.Fatal("NewOrchestrator() returned nil")
	}

	if orch.status != StatusStopped {
		t.Errorf("Initial status = %v, want stopped", orch.status)
	}
}

func TestOrchestrator_StartWithNoTasks(t *testing.T) {
	taskMgr := newMockTaskManager()
	agentMgr := newMockAgentManager()
	queue := newMockTaskQueue()
	config := DefaultOrchestratorConfig()

	orch := NewOrchestrator(taskMgr, agentMgr, queue, config)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := orch.Start(ctx)
	if err == nil {
		t.Error("Start() should error when no tasks")
	}
}

func TestOrchestrator_StartAlreadyRunning(t *testing.T) {
	taskMgr := newMockTaskManager()
	taskMgr.addTask(&task.Task{ID: "T1", Role: "backend", Status: task.StatusPending})
	agentMgr := newMockAgentManager()
	queue := newMockTaskQueue()
	config := DefaultOrchestratorConfig()
	config.StopWhenComplete = true

	orch := NewOrchestrator(taskMgr, agentMgr, queue, config)
	orch.status = StatusRunning

	ctx := context.Background()
	err := orch.Start(ctx)
	if err == nil {
		t.Error("Start() should error when already running")
	}
}

func TestOrchestrator_GetProgress(t *testing.T) {
	taskMgr := newMockTaskManager()
	taskMgr.addTask(&task.Task{ID: "T1", Role: "backend", Status: task.StatusComplete})
	taskMgr.addTask(&task.Task{ID: "T2", Role: "backend", Status: task.StatusPending})
	taskMgr.addTask(&task.Task{ID: "T3", Role: "frontend", Status: task.StatusInProgress})
	agentMgr := newMockAgentManager()
	queue := newMockTaskQueue()
	config := DefaultOrchestratorConfig()

	orch := NewOrchestrator(taskMgr, agentMgr, queue, config)

	progress := orch.GetProgress()

	if progress.Total != 3 {
		t.Errorf("Total = %d, want 3", progress.Total)
	}

	if progress.Complete != 1 {
		t.Errorf("Complete = %d, want 1", progress.Complete)
	}

	if progress.InProgress != 1 {
		t.Errorf("InProgress = %d, want 1", progress.InProgress)
	}

	if progress.Pending != 1 {
		t.Errorf("Pending = %d, want 1", progress.Pending)
	}

	// ~33% complete
	if progress.Percentage < 30 || progress.Percentage > 35 {
		t.Errorf("Percentage = %f, want ~33", progress.Percentage)
	}
}

func TestOrchestrator_Status(t *testing.T) {
	taskMgr := newMockTaskManager()
	taskMgr.addTask(&task.Task{ID: "T1", Role: "backend", Status: task.StatusPending})
	agentMgr := newMockAgentManager()
	agentMgr.addAgent(&agent.Agent{Name: "be-1", Role: "backend", Status: "idle"})
	queue := newMockTaskQueue()
	config := DefaultOrchestratorConfig()

	orch := NewOrchestrator(taskMgr, agentMgr, queue, config)
	orch.status = StatusRunning
	orch.started = time.Now().Add(-5 * time.Minute)

	status := orch.Status()

	if status.Status != StatusRunning {
		t.Errorf("Status = %v, want running", status.Status)
	}

	if status.AgentCount != 1 {
		t.Errorf("AgentCount = %d, want 1", status.AgentCount)
	}

	if status.IdleAgents != 1 {
		t.Errorf("IdleAgents = %d, want 1", status.IdleAgents)
	}
}

func TestOrchestrator_StopNotRunning(t *testing.T) {
	taskMgr := newMockTaskManager()
	agentMgr := newMockAgentManager()
	queue := newMockTaskQueue()
	config := DefaultOrchestratorConfig()

	orch := NewOrchestrator(taskMgr, agentMgr, queue, config)

	err := orch.Stop()
	if err == nil {
		t.Error("Stop() should error when not running")
	}
}

func TestOrchestrator_IsComplete(t *testing.T) {
	tests := []struct {
		name     string
		tasks    []*task.Task
		complete bool
	}{
		{
			name: "all complete",
			tasks: []*task.Task{
				{ID: "T1", Status: task.StatusComplete},
				{ID: "T2", Status: task.StatusComplete},
			},
			complete: true,
		},
		{
			name: "some pending",
			tasks: []*task.Task{
				{ID: "T1", Status: task.StatusComplete},
				{ID: "T2", Status: task.StatusPending},
			},
			complete: false,
		},
		{
			name: "some in progress",
			tasks: []*task.Task{
				{ID: "T1", Status: task.StatusComplete},
				{ID: "T2", Status: task.StatusInProgress},
			},
			complete: false,
		},
		{
			name: "failed tasks count as complete",
			tasks: []*task.Task{
				{ID: "T1", Status: task.StatusComplete},
				{ID: "T2", Status: task.StatusFailed},
			},
			complete: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskMgr := newMockTaskManager()
			for _, tsk := range tt.tasks {
				taskMgr.addTask(tsk)
			}
			agentMgr := newMockAgentManager()
			queue := newMockTaskQueue()
			config := DefaultOrchestratorConfig()

			orch := NewOrchestrator(taskMgr, agentMgr, queue, config)

			if orch.isComplete() != tt.complete {
				t.Errorf("isComplete() = %v, want %v", orch.isComplete(), tt.complete)
			}
		})
	}
}

func TestDefaultOrchestratorConfig(t *testing.T) {
	config := DefaultOrchestratorConfig()

	if config.PollInterval != 10*time.Second {
		t.Errorf("PollInterval = %v, want 10s", config.PollInterval)
	}

	if config.MaxAgentsPerRole != 1 {
		t.Errorf("MaxAgentsPerRole = %d, want 1", config.MaxAgentsPerRole)
	}

	if !config.AutoSpawnAgents {
		t.Error("AutoSpawnAgents should default to true")
	}

	if config.StopWhenComplete {
		t.Error("StopWhenComplete should default to false")
	}
}

func TestOrchestrator_Events(t *testing.T) {
	taskMgr := newMockTaskManager()
	agentMgr := newMockAgentManager()
	queue := newMockTaskQueue()
	config := DefaultOrchestratorConfig()

	orch := NewOrchestrator(taskMgr, agentMgr, queue, config)

	events := orch.Events()
	if events == nil {
		t.Error("Events() returned nil")
	}
}

func TestCountIdleAgents(t *testing.T) {
	agents := []*agent.Agent{
		{Name: "a1", Status: "idle"},
		{Name: "a2", Status: "working"},
		{Name: "a3", Status: "idle"},
		{Name: "a4", Status: "stopped"},
	}

	count := countIdleAgents(agents)
	if count != 2 {
		t.Errorf("countIdleAgents() = %d, want 2", count)
	}
}

func TestCountIdleAgents_Empty(t *testing.T) {
	count := countIdleAgents(nil)
	if count != 0 {
		t.Errorf("countIdleAgents(nil) = %d, want 0", count)
	}
}

func TestOrchestrator_HandleEvent_TaskCompleted(t *testing.T) {
	taskMgr := newMockTaskManager()
	taskMgr.addTask(&task.Task{
		ID:         "T1",
		Role:       "backend",
		Status:     task.StatusInProgress,
		AssignedTo: "be-1",
	})
	agentMgr := newMockAgentManager()
	queue := newMockTaskQueue()
	config := DefaultOrchestratorConfig()

	orch := NewOrchestrator(taskMgr, agentMgr, queue, config)

	event := task.Event{
		Type:      task.EventTaskCompleted,
		TaskID:    "T1",
		AgentName: "be-1",
	}

	orch.handleEvent(context.Background(), event)

	// Task should be unassigned
	tsk, _ := taskMgr.Get("T1")
	if tsk.AssignedTo != "" {
		t.Errorf("Task should be unassigned after completion")
	}
}
