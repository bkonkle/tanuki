package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// mockAgentProvider is a mock implementation of AgentProvider for testing.
type mockAgentProvider struct {
	agents      []*AgentInfo
	listErr     error
	stopErr     error
	startErr    error
	stopCalled  string
	startCalled string
}

func (m *mockAgentProvider) ListAgents() ([]*AgentInfo, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.agents, nil
}

func (m *mockAgentProvider) StopAgent(name string) error {
	m.stopCalled = name
	return m.stopErr
}

func (m *mockAgentProvider) StartAgent(name string) error {
	m.startCalled = name
	return m.startErr
}

// mockTaskProvider is a mock implementation of TaskProvider for testing.
type mockTaskProvider struct {
	tasks   []*TaskInfo
	listErr error
}

func (m *mockTaskProvider) ListTasks() ([]*TaskInfo, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.tasks, nil
}

// assertModel is a test helper that asserts the tea.Model is a Model and returns it.
func assertModel(t *testing.T, teaModel tea.Model) Model {
	t.Helper()
	m, ok := teaModel.(Model)
	if !ok {
		t.Fatal("expected Model type from Update")
	}
	return m
}

func TestNewModel(t *testing.T) {
	agentProvider := &mockAgentProvider{}
	taskProvider := &mockTaskProvider{}

	model := NewModel(agentProvider, taskProvider)

	if model.activePane != PaneAgents {
		t.Errorf("expected activePane to be PaneAgents, got %v", model.activePane)
	}
	if !model.logFollow {
		t.Error("expected logFollow to be true by default")
	}
	if model.statusFilter != "all" {
		t.Errorf("expected statusFilter to be 'all', got %s", model.statusFilter)
	}
	if model.roleFilter != "all" {
		t.Errorf("expected roleFilter to be 'all', got %s", model.roleFilter)
	}
	if model.maxLogs != 1000 {
		t.Errorf("expected maxLogs to be 1000, got %d", model.maxLogs)
	}
}

func TestModelUpdate_WindowResize(t *testing.T) {
	model := NewModel(nil, nil)

	newModel, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	m := assertModel(t, newModel)

	if m.width != 100 {
		t.Errorf("expected width 100, got %d", m.width)
	}
	if m.height != 50 {
		t.Errorf("expected height 50, got %d", m.height)
	}
}

func TestModelUpdate_TabNavigation(t *testing.T) {
	model := NewModel(nil, nil)

	// Tab should cycle through panes
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
	m := assertModel(t, newModel)
	if m.activePane != PaneTasks {
		t.Errorf("expected PaneTasks after first tab, got %v", m.activePane)
	}

	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = assertModel(t, newModel)
	if m.activePane != PaneLogs {
		t.Errorf("expected PaneLogs after second tab, got %v", m.activePane)
	}

	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = assertModel(t, newModel)
	if m.activePane != PaneAgents {
		t.Errorf("expected PaneAgents after third tab, got %v", m.activePane)
	}
}

func TestModelUpdate_ShiftTabNavigation(t *testing.T) {
	model := NewModel(nil, nil)

	// Shift+Tab should cycle backwards
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	m := assertModel(t, newModel)
	if m.activePane != PaneLogs {
		t.Errorf("expected PaneLogs after shift+tab, got %v", m.activePane)
	}
}

func TestModelUpdate_UpDownNavigation(t *testing.T) {
	agents := []*AgentInfo{
		{Name: "agent1", Status: "idle"},
		{Name: "agent2", Status: "working"},
		{Name: "agent3", Status: "stopped"},
	}

	agentProvider := &mockAgentProvider{agents: agents}
	model := NewModel(agentProvider, nil)
	model.agents = agents

	// Move down
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	m := assertModel(t, newModel)
	if m.agentCursor != 1 {
		t.Errorf("expected cursor at 1 after down, got %d", m.agentCursor)
	}

	// Move down again
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = assertModel(t, newModel)
	if m.agentCursor != 2 {
		t.Errorf("expected cursor at 2 after second down, got %d", m.agentCursor)
	}

	// Move down at bottom should stay
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = assertModel(t, newModel)
	if m.agentCursor != 2 {
		t.Errorf("expected cursor to stay at 2, got %d", m.agentCursor)
	}

	// Move up
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = assertModel(t, newModel)
	if m.agentCursor != 1 {
		t.Errorf("expected cursor at 1 after up, got %d", m.agentCursor)
	}
}

func TestModelUpdate_JKNavigation(t *testing.T) {
	agents := []*AgentInfo{
		{Name: "agent1", Status: "idle"},
		{Name: "agent2", Status: "working"},
	}

	agentProvider := &mockAgentProvider{agents: agents}
	model := NewModel(agentProvider, nil)
	model.agents = agents

	// j should move down
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m := assertModel(t, newModel)
	if m.agentCursor != 1 {
		t.Errorf("expected cursor at 1 after j, got %d", m.agentCursor)
	}

	// k should move up
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m = assertModel(t, newModel)
	if m.agentCursor != 0 {
		t.Errorf("expected cursor at 0 after k, got %d", m.agentCursor)
	}
}

func TestModelUpdate_HelpToggle(t *testing.T) {
	model := NewModel(nil, nil)

	// Toggle help on
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m := assertModel(t, newModel)
	if !m.showHelp {
		t.Error("expected showHelp to be true after ?")
	}

	// Toggle help off
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = assertModel(t, newModel)
	if m.showHelp {
		t.Error("expected showHelp to be false after second ?")
	}
}

func TestModelUpdate_AgentsRefreshed(t *testing.T) {
	model := NewModel(nil, nil)
	model.width = 100
	model.height = 50

	agents := []*AgentInfo{
		{Name: "agent1", Status: "idle"},
		{Name: "agent2", Status: "working"},
	}

	newModel, _ := model.Update(agentsRefreshedMsg{agents: agents})
	m := assertModel(t, newModel)

	if len(m.agents) != 2 {
		t.Errorf("expected 2 agents, got %d", len(m.agents))
	}
	if m.agents[0].Name != "agent1" {
		t.Errorf("expected first agent name 'agent1', got %s", m.agents[0].Name)
	}
}

func TestModelUpdate_TasksRefreshed(t *testing.T) {
	model := NewModel(nil, nil)
	model.width = 100
	model.height = 50

	tasks := []*TaskInfo{
		{ID: "TASK-001", Title: "Test Task", Status: "pending"},
	}

	newModel, _ := model.Update(tasksRefreshedMsg{tasks: tasks})
	m := assertModel(t, newModel)

	if len(m.tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(m.tasks))
	}
}

func TestModelUpdate_FollowToggle(t *testing.T) {
	model := NewModel(nil, nil)
	model.activePane = PaneLogs
	model.logFollow = true

	// Toggle follow off
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	m := assertModel(t, newModel)
	if m.logFollow {
		t.Error("expected logFollow to be false after f")
	}

	// Toggle follow on
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	m = assertModel(t, newModel)
	if !m.logFollow {
		t.Error("expected logFollow to be true after second f")
	}
}

func TestModelUpdate_PauseToggle(t *testing.T) {
	model := NewModel(nil, nil)
	model.activePane = PaneLogs

	// Toggle pause on
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	m := assertModel(t, newModel)
	if !m.logPaused {
		t.Error("expected logPaused to be true after p")
	}

	// Toggle pause off
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	m = assertModel(t, newModel)
	if m.logPaused {
		t.Error("expected logPaused to be false after second p")
	}
}

func TestModelUpdate_ClearLogs(t *testing.T) {
	model := NewModel(nil, nil)
	model.activePane = PaneLogs
	model.logs = []LogLine{
		{Content: "test log", Timestamp: time.Now()},
	}

	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m := assertModel(t, newModel)

	if len(m.logs) != 0 {
		t.Errorf("expected 0 logs after clear, got %d", len(m.logs))
	}
}

func TestModel_FilteredTasks(t *testing.T) {
	model := NewModel(nil, nil)
	model.tasks = []*TaskInfo{
		{ID: "TASK-001", Status: "pending", Role: "backend"},
		{ID: "TASK-002", Status: "in_progress", Role: "backend"},
		{ID: "TASK-003", Status: "complete", Role: "frontend"},
		{ID: "TASK-004", Status: "pending", Role: "frontend"},
	}

	// No filter - all tasks
	filtered := model.filteredTasks()
	if len(filtered) != 4 {
		t.Errorf("expected 4 tasks with no filter, got %d", len(filtered))
	}

	// Status filter
	model.statusFilter = "pending"
	filtered = model.filteredTasks()
	if len(filtered) != 2 {
		t.Errorf("expected 2 pending tasks, got %d", len(filtered))
	}

	// Role filter
	model.statusFilter = "all"
	model.roleFilter = "backend"
	filtered = model.filteredTasks()
	if len(filtered) != 2 {
		t.Errorf("expected 2 backend tasks, got %d", len(filtered))
	}

	// Combined filter
	model.statusFilter = "pending"
	model.roleFilter = "frontend"
	filtered = model.filteredTasks()
	if len(filtered) != 1 {
		t.Errorf("expected 1 pending frontend task, got %d", len(filtered))
	}
}

func TestModel_CycleStatusFilter(t *testing.T) {
	model := NewModel(nil, nil)

	expectedOrder := []string{"all", "pending", "in_progress", "complete", "failed", "blocked", "all"}

	for i, expected := range expectedOrder {
		if model.statusFilter != expected {
			t.Errorf("step %d: expected filter %s, got %s", i, expected, model.statusFilter)
		}
		model.cycleStatusFilter()
	}
}

func TestModel_AddLogLine(t *testing.T) {
	model := NewModel(nil, nil)
	model.height = 20

	// Add a log line
	line := LogLine{
		Content:   "test log",
		Timestamp: time.Now(),
		Level:     "info",
	}
	model.AddLogLine(line)

	if len(model.logs) != 1 {
		t.Errorf("expected 1 log, got %d", len(model.logs))
	}
	if model.logs[0].Content != "test log" {
		t.Errorf("expected content 'test log', got %s", model.logs[0].Content)
	}
}

func TestModel_AddLogLine_MaxBuffer(t *testing.T) {
	model := NewModel(nil, nil)
	model.height = 20

	// Add more than max logs (1000)
	for i := 0; i < 1005; i++ {
		model.AddLogLine(LogLine{Content: "log", Timestamp: time.Now()})
	}

	if len(model.logs) != 1000 {
		t.Errorf("expected 1000 logs (max), got %d", len(model.logs))
	}
}

func TestModel_View_Loading(t *testing.T) {
	model := NewModel(nil, nil)
	// Width and height are 0

	view := model.View()
	if view != "Loading..." {
		t.Errorf("expected 'Loading...' view, got %s", view)
	}
}

func TestModel_View_WithDimensions(t *testing.T) {
	model := NewModel(nil, nil)
	model.width = 100
	model.height = 40

	view := model.View()

	// Should contain title
	if len(view) == 0 {
		t.Error("expected non-empty view")
	}
}

func TestModel_View_Help(t *testing.T) {
	model := NewModel(nil, nil)
	model.width = 100
	model.height = 40
	model.showHelp = true

	view := model.View()

	// Should contain help text
	if len(view) == 0 {
		t.Error("expected non-empty help view")
	}
}

func TestModel_GetActivePane(t *testing.T) {
	model := NewModel(nil, nil)

	if model.GetActivePane() != PaneAgents {
		t.Errorf("expected PaneAgents, got %v", model.GetActivePane())
	}

	model.activePane = PaneTasks
	if model.GetActivePane() != PaneTasks {
		t.Errorf("expected PaneTasks, got %v", model.GetActivePane())
	}
}

func TestModel_GetAgentCursor(t *testing.T) {
	model := NewModel(nil, nil)
	model.agentCursor = 5

	if model.GetAgentCursor() != 5 {
		t.Errorf("expected cursor 5, got %d", model.GetAgentCursor())
	}
}

func TestModel_GetTaskCursor(t *testing.T) {
	model := NewModel(nil, nil)
	model.taskCursor = 3

	if model.GetTaskCursor() != 3 {
		t.Errorf("expected cursor 3, got %d", model.GetTaskCursor())
	}
}

func TestModel_SetAgents(t *testing.T) {
	model := NewModel(nil, nil)
	agents := []*AgentInfo{
		{Name: "test-agent", Status: "idle"},
	}

	model.SetAgents(agents)

	if len(model.agents) != 1 {
		t.Errorf("expected 1 agent, got %d", len(model.agents))
	}
}

func TestModel_SetTasks(t *testing.T) {
	model := NewModel(nil, nil)
	tasks := []*TaskInfo{
		{ID: "TASK-001", Title: "Test"},
	}

	model.SetTasks(tasks)

	if len(model.tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(model.tasks))
	}
}

func TestModel_ShowTaskDetails(t *testing.T) {
	model := NewModel(nil, nil)
	model.width = 100
	model.height = 40
	model.activePane = PaneTasks
	model.tasks = []*TaskInfo{
		{ID: "TASK-001", Title: "Test Task", Status: "pending"},
	}

	// Press Enter to show task details
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m := assertModel(t, newModel)

	if !m.showTaskDetails {
		t.Error("expected showTaskDetails to be true after Enter")
	}
	if m.taskDetailsModal == nil {
		t.Error("expected taskDetailsModal to be set")
	}
}

func TestModel_CloseTaskDetails(t *testing.T) {
	model := NewModel(nil, nil)
	model.width = 100
	model.height = 40
	model.showTaskDetails = true
	model.taskDetailsModal = NewTaskDetailsModal(
		&TaskInfo{ID: "TASK-001", Title: "Test"},
		100, 40)

	// Press Esc to close modal
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m := assertModel(t, newModel)

	if m.showTaskDetails {
		t.Error("expected showTaskDetails to be false after Esc")
	}
}

func TestModel_NoTaskDetailsWhenEmpty(t *testing.T) {
	model := NewModel(nil, nil)
	model.width = 100
	model.height = 40
	model.activePane = PaneTasks
	model.tasks = []*TaskInfo{} // Empty task list

	// Press Enter with no tasks
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m := assertModel(t, newModel)

	if m.showTaskDetails {
		t.Error("expected showTaskDetails to remain false with no tasks")
	}
}

func TestModel_TaskDetailsWithFilter(t *testing.T) {
	model := NewModel(nil, nil)
	model.width = 100
	model.height = 40
	model.activePane = PaneTasks
	model.tasks = []*TaskInfo{
		{ID: "TASK-001", Status: "pending", Role: "backend"},
		{ID: "TASK-002", Status: "in_progress", Role: "frontend"},
	}
	model.statusFilter = "in_progress"

	// Cursor should be on first filtered task (TASK-002)
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m := assertModel(t, newModel)

	if !m.showTaskDetails {
		t.Error("expected showTaskDetails to be true")
	}
	if m.taskDetailsModal.task.ID != "TASK-002" {
		t.Errorf("expected modal to show TASK-002, got %s", m.taskDetailsModal.task.ID)
	}
}

func TestModel_LogLineMsg(t *testing.T) {
	model := NewModel(nil, nil)
	model.height = 20

	// Send a log line message
	line := LogLine{
		Content:   "test log",
		Timestamp: time.Now(),
		Level:     "info",
	}

	newModel, _ := model.Update(logLineMsg{line: line})
	m := assertModel(t, newModel)

	if len(m.logs) != 1 {
		t.Errorf("expected 1 log after logLineMsg, got %d", len(m.logs))
	}
	if m.logs[0].Content != "test log" {
		t.Errorf("expected content 'test log', got %s", m.logs[0].Content)
	}
}

func TestModel_LogLineMsg_WhenPaused(t *testing.T) {
	model := NewModel(nil, nil)
	model.height = 20
	model.logPaused = true

	// Send a log line message while paused
	line := LogLine{
		Content:   "test log",
		Timestamp: time.Now(),
		Level:     "info",
	}

	newModel, _ := model.Update(logLineMsg{line: line})
	m := assertModel(t, newModel)

	// Log should not be added when paused
	if len(m.logs) != 0 {
		t.Errorf("expected 0 logs when paused, got %d", len(m.logs))
	}
}

func TestModel_ToggleLogFollow(t *testing.T) {
	model := NewModel(nil, nil)
	model.activePane = PaneLogs
	model.logFollow = true

	// Press 'f' to toggle follow
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	m := assertModel(t, newModel)

	if m.logFollow {
		t.Error("expected logFollow to be false after toggle")
	}

	// Press 'f' again to toggle back
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	m = assertModel(t, newModel)

	if !m.logFollow {
		t.Error("expected logFollow to be true after second toggle")
	}
}

func TestModel_ToggleLogPause(t *testing.T) {
	model := NewModel(nil, nil)
	model.activePane = PaneLogs
	model.logPaused = false

	// Press 'p' to pause
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	m := assertModel(t, newModel)

	if !m.logPaused {
		t.Error("expected logPaused to be true after toggle")
	}
}

func TestModel_ClearLogs(t *testing.T) {
	model := NewModel(nil, nil)
	model.height = 20
	model.activePane = PaneLogs

	// Add some logs
	for i := 0; i < 10; i++ {
		model.AddLogLine(LogLine{Content: "test", Timestamp: time.Now()})
	}

	if len(model.logs) != 10 {
		t.Fatalf("setup failed: expected 10 logs, got %d", len(model.logs))
	}

	// Press 'c' to clear
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m := assertModel(t, newModel)

	if len(m.logs) != 0 {
		t.Errorf("expected 0 logs after clear, got %d", len(m.logs))
	}
	if m.logOffset != 0 {
		t.Errorf("expected logOffset 0 after clear, got %d", m.logOffset)
	}
}

func TestModel_SwitchLogAgent(t *testing.T) {
	model := NewModel(nil, nil)

	// Switch to a new agent
	model.switchLogAgent("test-agent")

	if model.selectedAgent != "test-agent" {
		t.Errorf("expected selectedAgent 'test-agent', got %s", model.selectedAgent)
	}
	if !model.logFollow {
		t.Error("expected logFollow to be reset to true")
	}
	if len(model.logs) != 0 {
		t.Errorf("expected logs to be cleared, got %d logs", len(model.logs))
	}
	if model.logOffset != 0 {
		t.Errorf("expected logOffset to be 0, got %d", model.logOffset)
	}
}
