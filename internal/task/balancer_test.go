package task

import (
	"sync"
	"testing"
)

func TestBalancer_AssignTask(t *testing.T) {
	b := NewBalancer()

	task := &Task{ID: "T1", Role: "backend"}
	agents := []*Agent{
		{Name: "be-1", Role: "backend", Status: "idle"},
		{Name: "be-2", Role: "backend", Status: "idle"},
		{Name: "fe-1", Role: "frontend", Status: "idle"},
	}

	agent, err := b.AssignTask(task, agents)

	if err != nil {
		t.Fatalf("AssignTask() error: %v", err)
	}

	if agent.Role != "backend" {
		t.Errorf("Selected agent role = %s, want backend", agent.Role)
	}
}

func TestBalancer_AssignTaskNilTask(t *testing.T) {
	b := NewBalancer()

	agents := []*Agent{
		{Name: "be-1", Role: "backend", Status: "idle"},
	}

	_, err := b.AssignTask(nil, agents)
	if err == nil {
		t.Error("AssignTask(nil) should return error")
	}
}

func TestBalancer_LeastLoaded(t *testing.T) {
	b := NewBalancer()

	// Pre-load one agent
	b.TrackAssignment("be-1")
	b.TrackAssignment("be-1")

	task := &Task{ID: "T1", Role: "backend"}
	agents := []*Agent{
		{Name: "be-1", Role: "backend", Status: "idle"},
		{Name: "be-2", Role: "backend", Status: "idle"},
	}

	agent, err := b.AssignTask(task, agents)

	if err != nil {
		t.Fatalf("AssignTask() error: %v", err)
	}

	// Should select be-2 (less loaded)
	if agent.Name != "be-2" {
		t.Errorf("Selected %s, want be-2 (least loaded)", agent.Name)
	}
}

func TestBalancer_NoIdleAgents(t *testing.T) {
	b := NewBalancer()

	task := &Task{ID: "T1", Role: "backend"}
	agents := []*Agent{
		{Name: "be-1", Role: "backend", Status: "working"},
		{Name: "be-2", Role: "backend", Status: "working"},
	}

	_, err := b.AssignTask(task, agents)

	if err == nil {
		t.Error("AssignTask() should error when no idle agents")
	}
}

func TestBalancer_NoMatchingRole(t *testing.T) {
	b := NewBalancer()

	task := &Task{ID: "T1", Role: "backend"}
	agents := []*Agent{
		{Name: "fe-1", Role: "frontend", Status: "idle"},
	}

	_, err := b.AssignTask(task, agents)

	if err == nil {
		t.Error("AssignTask() should error when no matching role")
	}
}

func TestBalancer_WorkloadTracking(t *testing.T) {
	b := NewBalancer()

	b.TrackAssignment("agent-1")
	b.TrackAssignment("agent-1")
	b.TrackAssignment("agent-2")

	if b.GetWorkload("agent-1") != 2 {
		t.Errorf("Workload(agent-1) = %d, want 2", b.GetWorkload("agent-1"))
	}

	b.TrackCompletion("agent-1")

	if b.GetWorkload("agent-1") != 1 {
		t.Errorf("After completion, Workload(agent-1) = %d, want 1", b.GetWorkload("agent-1"))
	}
}

func TestBalancer_TrackCompletionDoesNotGoBelowZero(t *testing.T) {
	b := NewBalancer()

	b.TrackCompletion("agent-1") // Never assigned
	b.TrackCompletion("agent-1") // Still zero

	if b.GetWorkload("agent-1") != 0 {
		t.Errorf("Workload should not go below 0, got %d", b.GetWorkload("agent-1"))
	}
}

func TestBalancer_GetTotalWorkload(t *testing.T) {
	b := NewBalancer()

	b.TrackAssignment("agent-1")
	b.TrackAssignment("agent-1")
	b.TrackAssignment("agent-2")
	b.TrackAssignment("agent-3")

	total := b.GetTotalWorkload()
	if total != 4 {
		t.Errorf("GetTotalWorkload() = %d, want 4", total)
	}
}

func TestBalancer_ResetWorkload(t *testing.T) {
	b := NewBalancer()

	b.TrackAssignment("agent-1")
	b.TrackAssignment("agent-1")

	b.ResetWorkload("agent-1")

	if b.GetWorkload("agent-1") != 0 {
		t.Errorf("After reset, Workload(agent-1) = %d, want 0", b.GetWorkload("agent-1"))
	}
}

func TestBalancer_GetIdleAgents(t *testing.T) {
	b := NewBalancer()

	agents := []*Agent{
		{Name: "a1", Role: "backend", Status: "idle"},
		{Name: "a2", Role: "backend", Status: "working"},
		{Name: "a3", Role: "frontend", Status: "idle"},
	}

	// All idle
	idle := b.GetIdleAgents(agents, "")
	if len(idle) != 2 {
		t.Errorf("GetIdleAgents('') = %d, want 2", len(idle))
	}

	// Idle backend only
	idleBackend := b.GetIdleAgents(agents, "backend")
	if len(idleBackend) != 1 {
		t.Errorf("GetIdleAgents('backend') = %d, want 1", len(idleBackend))
	}
}

func TestBalancer_GetAgentsByRole(t *testing.T) {
	b := NewBalancer()

	agents := []*Agent{
		{Name: "a1", Role: "backend", Status: "idle"},
		{Name: "a2", Role: "backend", Status: "working"},
		{Name: "a3", Role: "frontend", Status: "idle"},
	}

	backend := b.GetAgentsByRole(agents, "backend")
	if len(backend) != 2 {
		t.Errorf("GetAgentsByRole('backend') = %d, want 2", len(backend))
	}

	frontend := b.GetAgentsByRole(agents, "frontend")
	if len(frontend) != 1 {
		t.Errorf("GetAgentsByRole('frontend') = %d, want 1", len(frontend))
	}
}

func TestBalancer_GetRolesNeeded(t *testing.T) {
	b := NewBalancer()

	tasks := []*Task{
		{ID: "T1", Role: "backend", Status: StatusPending},
		{ID: "T2", Role: "frontend", Status: StatusPending},
		{ID: "T3", Role: "qa", Status: StatusPending},
		{ID: "T4", Role: "backend", Status: StatusComplete}, // Should be ignored
	}

	agents := []*Agent{
		{Name: "be-1", Role: "backend"},
	}

	needed := b.GetRolesNeeded(tasks, agents)

	// Should need frontend and qa
	if len(needed) != 2 {
		t.Errorf("GetRolesNeeded() = %v, want 2 roles", needed)
	}

	// Verify roles
	needMap := make(map[string]bool)
	for _, r := range needed {
		needMap[r] = true
	}

	if !needMap["frontend"] {
		t.Error("Should need frontend role")
	}
	if !needMap["qa"] {
		t.Error("Should need qa role")
	}
	if needMap["backend"] {
		t.Error("Should not need backend role (already have agent)")
	}
}

func TestBalancer_GetRolesNeededWithBlocked(t *testing.T) {
	b := NewBalancer()

	tasks := []*Task{
		{ID: "T1", Role: "backend", Status: StatusBlocked}, // Should be included
	}

	agents := []*Agent{}

	needed := b.GetRolesNeeded(tasks, agents)

	if len(needed) != 1 || needed[0] != "backend" {
		t.Errorf("GetRolesNeeded() = %v, want [backend]", needed)
	}
}

func TestBalancer_Stats(t *testing.T) {
	b := NewBalancer()

	b.TrackAssignment("agent-1")
	b.TrackAssignment("agent-1")
	b.TrackAssignment("agent-2")

	stats := b.Stats()

	if stats.TotalTasks != 3 {
		t.Errorf("Stats.TotalTasks = %d, want 3", stats.TotalTasks)
	}

	if stats.AgentWorkloads["agent-1"] != 2 {
		t.Errorf("Stats.AgentWorkloads[agent-1] = %d, want 2", stats.AgentWorkloads["agent-1"])
	}

	if stats.MaxWorkload != 2 {
		t.Errorf("Stats.MaxWorkload = %d, want 2", stats.MaxWorkload)
	}

	if stats.BusiestAgent != "agent-1" {
		t.Errorf("Stats.BusiestAgent = %s, want agent-1", stats.BusiestAgent)
	}

	if stats.AvgWorkload != 1.5 {
		t.Errorf("Stats.AvgWorkload = %f, want 1.5", stats.AvgWorkload)
	}
}

func TestBalancer_StatsEmpty(t *testing.T) {
	b := NewBalancer()

	stats := b.Stats()

	if stats.TotalTasks != 0 {
		t.Errorf("Stats.TotalTasks = %d, want 0", stats.TotalTasks)
	}

	if stats.AvgWorkload != 0 {
		t.Errorf("Stats.AvgWorkload = %f, want 0", stats.AvgWorkload)
	}
}

func TestBalancer_ConcurrentAccess(t *testing.T) {
	b := NewBalancer()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)

		go func(id int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				b.TrackAssignment("agent-1")
			}
		}(i)

		go func(id int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				b.TrackCompletion("agent-1")
				b.GetWorkload("agent-1")
				b.GetTotalWorkload()
			}
		}(i)
	}

	wg.Wait()
}

func TestBalancer_EmptyAgentsList(t *testing.T) {
	b := NewBalancer()

	task := &Task{ID: "T1", Role: "backend"}
	agents := []*Agent{}

	_, err := b.AssignTask(task, agents)

	if err == nil {
		t.Error("AssignTask() should error with empty agents list")
	}
}

func TestBalancer_SelectLeastLoadedEmpty(t *testing.T) {
	b := NewBalancer()

	result := b.selectLeastLoaded([]*Agent{})

	if result != nil {
		t.Error("selectLeastLoaded([]) should return nil")
	}
}

// Tests for BalancerWithStrategy

func TestBalancerWithStrategy_LeastLoaded(t *testing.T) {
	b := NewBalancerWithStrategy(StrategyLeastLoaded)

	b.TrackAssignment("be-1")
	b.TrackAssignment("be-1")

	task := &Task{ID: "T1", Role: "backend"}
	agents := []*Agent{
		{Name: "be-1", Role: "backend", Status: "idle"},
		{Name: "be-2", Role: "backend", Status: "idle"},
	}

	agent, err := b.AssignTaskWithStrategy(task, agents)

	if err != nil {
		t.Fatalf("AssignTaskWithStrategy() error: %v", err)
	}

	if agent.Name != "be-2" {
		t.Errorf("Selected %s, want be-2 (least loaded)", agent.Name)
	}
}

func TestBalancerWithStrategy_RoundRobin(t *testing.T) {
	b := NewBalancerWithStrategy(StrategyRoundRobin)

	task := &Task{ID: "T1", Role: "backend"}
	agents := []*Agent{
		{Name: "be-1", Role: "backend", Status: "idle"},
		{Name: "be-2", Role: "backend", Status: "idle"},
		{Name: "be-3", Role: "backend", Status: "idle"},
	}

	// Should cycle through agents
	names := make([]string, 3)
	for i := 0; i < 3; i++ {
		agent, _ := b.AssignTaskWithStrategy(task, agents)
		names[i] = agent.Name
	}

	// Each should be different (round robin)
	seen := make(map[string]bool)
	for _, name := range names {
		if seen[name] {
			t.Errorf("Round robin selected same agent twice: %v", names)
			break
		}
		seen[name] = true
	}
}

func TestBalancerWithStrategy_Random(t *testing.T) {
	b := NewBalancerWithStrategy(StrategyRandom)

	task := &Task{ID: "T1", Role: "backend"}
	agents := []*Agent{
		{Name: "be-1", Role: "backend", Status: "idle"},
	}

	agent, err := b.AssignTaskWithStrategy(task, agents)

	if err != nil {
		t.Fatalf("AssignTaskWithStrategy() error: %v", err)
	}

	if agent.Name != "be-1" {
		t.Errorf("Selected %s, want be-1", agent.Name)
	}
}

func TestBalancerWithStrategy_NilTask(t *testing.T) {
	b := NewBalancerWithStrategy(StrategyLeastLoaded)

	agents := []*Agent{
		{Name: "be-1", Role: "backend", Status: "idle"},
	}

	_, err := b.AssignTaskWithStrategy(nil, agents)
	if err == nil {
		t.Error("AssignTaskWithStrategy(nil) should return error")
	}
}

func TestBalancerWithStrategy_NoMatchingRole(t *testing.T) {
	b := NewBalancerWithStrategy(StrategyLeastLoaded)

	task := &Task{ID: "T1", Role: "backend"}
	agents := []*Agent{
		{Name: "fe-1", Role: "frontend", Status: "idle"},
	}

	_, err := b.AssignTaskWithStrategy(task, agents)

	if err == nil {
		t.Error("AssignTaskWithStrategy() should error when no matching role")
	}
}

func TestBalancerWithStrategy_NoIdleAgents(t *testing.T) {
	b := NewBalancerWithStrategy(StrategyLeastLoaded)

	task := &Task{ID: "T1", Role: "backend"}
	agents := []*Agent{
		{Name: "be-1", Role: "backend", Status: "working"},
	}

	_, err := b.AssignTaskWithStrategy(task, agents)

	if err == nil {
		t.Error("AssignTaskWithStrategy() should error when no idle agents")
	}
}

func TestBalancerWithStrategy_SelectRoundRobinEmpty(t *testing.T) {
	b := NewBalancerWithStrategy(StrategyRoundRobin)

	result := b.selectRoundRobin([]*Agent{}, "backend")

	if result != nil {
		t.Error("selectRoundRobin([]) should return nil")
	}
}

func TestBalancerWithStrategy_SelectRandomEmpty(t *testing.T) {
	b := NewBalancerWithStrategy(StrategyRandom)

	result := b.selectRandom([]*Agent{})

	if result != nil {
		t.Error("selectRandom([]) should return nil")
	}
}
