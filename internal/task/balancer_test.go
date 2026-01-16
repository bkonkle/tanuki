package task

import (
	"sync"
	"testing"
)

func TestBalancer_AssignTask(t *testing.T) {
	b := NewBalancer()

	task := &Task{ID: "T1", Workstream: "api"}
	agents := []*Agent{
		{Name: "api-1", Workstream: "api", Status: "idle"},
		{Name: "api-2", Workstream: "api", Status: "idle"},
		{Name: "web-1", Workstream: "web", Status: "idle"},
	}

	agent, err := b.AssignTask(task, agents)

	if err != nil {
		t.Fatalf("AssignTask() error: %v", err)
	}

	if agent.Workstream != "api" {
		t.Errorf("Selected agent workstream = %s, want api", agent.Workstream)
	}
}

func TestBalancer_AssignTaskNilTask(t *testing.T) {
	b := NewBalancer()

	agents := []*Agent{
		{Name: "api-1", Workstream: "api", Status: "idle"},
	}

	_, err := b.AssignTask(nil, agents)
	if err == nil {
		t.Error("AssignTask(nil) should return error")
	}
}

func TestBalancer_LeastLoaded(t *testing.T) {
	b := NewBalancer()

	// Pre-load one agent
	b.TrackAssignment("api-1")
	b.TrackAssignment("api-1")

	task := &Task{ID: "T1", Workstream: "api"}
	agents := []*Agent{
		{Name: "api-1", Workstream: "api", Status: "idle"},
		{Name: "api-2", Workstream: "api", Status: "idle"},
	}

	agent, err := b.AssignTask(task, agents)

	if err != nil {
		t.Fatalf("AssignTask() error: %v", err)
	}

	// Should select api-2 (less loaded)
	if agent.Name != "api-2" {
		t.Errorf("Selected %s, want api-2 (least loaded)", agent.Name)
	}
}

func TestBalancer_NoIdleAgents(t *testing.T) {
	b := NewBalancer()

	task := &Task{ID: "T1", Workstream: "api"}
	agents := []*Agent{
		{Name: "api-1", Workstream: "api", Status: "working"},
		{Name: "api-2", Workstream: "api", Status: "working"},
	}

	_, err := b.AssignTask(task, agents)

	if err == nil {
		t.Error("AssignTask() should error when no idle agents")
	}
}

func TestBalancer_NoMatchingWorkstream(t *testing.T) {
	b := NewBalancer()

	task := &Task{ID: "T1", Workstream: "api"}
	agents := []*Agent{
		{Name: "web-1", Workstream: "web", Status: "idle"},
	}

	_, err := b.AssignTask(task, agents)

	if err == nil {
		t.Error("AssignTask() should error when no matching workstream")
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
		{Name: "a1", Workstream: "api", Status: "idle"},
		{Name: "a2", Workstream: "api", Status: "working"},
		{Name: "a3", Workstream: "web", Status: "idle"},
	}

	// All idle
	idle := b.GetIdleAgents(agents, "")
	if len(idle) != 2 {
		t.Errorf("GetIdleAgents('') = %d, want 2", len(idle))
	}

	// Idle api only
	idleAPI := b.GetIdleAgents(agents, "api")
	if len(idleAPI) != 1 {
		t.Errorf("GetIdleAgents('api') = %d, want 1", len(idleAPI))
	}
}

func TestBalancer_GetAgentsByWorkstream(t *testing.T) {
	b := NewBalancer()

	agents := []*Agent{
		{Name: "a1", Workstream: "api", Status: "idle"},
		{Name: "a2", Workstream: "api", Status: "working"},
		{Name: "a3", Workstream: "web", Status: "idle"},
	}

	api := b.GetAgentsByWorkstream(agents, "api")
	if len(api) != 2 {
		t.Errorf("GetAgentsByWorkstream('api') = %d, want 2", len(api))
	}

	web := b.GetAgentsByWorkstream(agents, "web")
	if len(web) != 1 {
		t.Errorf("GetAgentsByWorkstream('web') = %d, want 1", len(web))
	}
}

func TestBalancer_GetWorkstreamsNeeded(t *testing.T) {
	b := NewBalancer()

	tasks := []*Task{
		{ID: "T1", Workstream: "api", Status: StatusPending},
		{ID: "T2", Workstream: "web", Status: StatusPending},
		{ID: "T3", Workstream: "qa", Status: StatusPending},
		{ID: "T4", Workstream: "api", Status: StatusComplete}, // Should be ignored
	}

	agents := []*Agent{
		{Name: "api-1", Workstream: "api"},
	}

	needed := b.GetWorkstreamsNeeded(tasks, agents)

	// Should need web and qa
	if len(needed) != 2 {
		t.Errorf("GetWorkstreamsNeeded() = %v, want 2 workstreams", needed)
	}

	// Verify workstreams
	needMap := make(map[string]bool)
	for _, ws := range needed {
		needMap[ws] = true
	}

	if !needMap["web"] {
		t.Error("Should need web workstream")
	}
	if !needMap["qa"] {
		t.Error("Should need qa workstream")
	}
	if needMap["api"] {
		t.Error("Should not need api workstream (already have agent)")
	}
}

func TestBalancer_GetWorkstreamsNeededWithBlocked(t *testing.T) {
	b := NewBalancer()

	tasks := []*Task{
		{ID: "T1", Workstream: "api", Status: StatusBlocked}, // Should be included
	}

	agents := []*Agent{}

	needed := b.GetWorkstreamsNeeded(tasks, agents)

	if len(needed) != 1 || needed[0] != "api" {
		t.Errorf("GetWorkstreamsNeeded() = %v, want [api]", needed)
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

func TestBalancer_ConcurrentAccess(_ *testing.T) {
	b := NewBalancer()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)

		go func(_ int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				b.TrackAssignment("agent-1")
			}
		}(i)

		go func(_ int) {
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

	task := &Task{ID: "T1", Workstream: "api"}
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

	b.TrackAssignment("api-1")
	b.TrackAssignment("api-1")

	task := &Task{ID: "T1", Workstream: "api"}
	agents := []*Agent{
		{Name: "api-1", Workstream: "api", Status: "idle"},
		{Name: "api-2", Workstream: "api", Status: "idle"},
	}

	agent, err := b.AssignTaskWithStrategy(task, agents)

	if err != nil {
		t.Fatalf("AssignTaskWithStrategy() error: %v", err)
	}

	if agent.Name != "api-2" {
		t.Errorf("Selected %s, want api-2 (least loaded)", agent.Name)
	}
}

func TestBalancerWithStrategy_RoundRobin(t *testing.T) {
	b := NewBalancerWithStrategy(StrategyRoundRobin)

	task := &Task{ID: "T1", Workstream: "api"}
	agents := []*Agent{
		{Name: "api-1", Workstream: "api", Status: "idle"},
		{Name: "api-2", Workstream: "api", Status: "idle"},
		{Name: "api-3", Workstream: "api", Status: "idle"},
	}

	// Should cycle through agents
	names := make([]string, 3)
	for i := 0; i < 3; i++ {
		agent, err := b.AssignTaskWithStrategy(task, agents)
		if err != nil {
			t.Fatalf("AssignTaskWithStrategy failed: %v", err)
		}
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

	task := &Task{ID: "T1", Workstream: "api"}
	agents := []*Agent{
		{Name: "api-1", Workstream: "api", Status: "idle"},
	}

	agent, err := b.AssignTaskWithStrategy(task, agents)

	if err != nil {
		t.Fatalf("AssignTaskWithStrategy() error: %v", err)
	}

	if agent.Name != "api-1" {
		t.Errorf("Selected %s, want api-1", agent.Name)
	}
}

func TestBalancerWithStrategy_NilTask(t *testing.T) {
	b := NewBalancerWithStrategy(StrategyLeastLoaded)

	agents := []*Agent{
		{Name: "api-1", Workstream: "api", Status: "idle"},
	}

	_, err := b.AssignTaskWithStrategy(nil, agents)
	if err == nil {
		t.Error("AssignTaskWithStrategy(nil) should return error")
	}
}

func TestBalancerWithStrategy_NoMatchingWorkstream(t *testing.T) {
	b := NewBalancerWithStrategy(StrategyLeastLoaded)

	task := &Task{ID: "T1", Workstream: "api"}
	agents := []*Agent{
		{Name: "web-1", Workstream: "web", Status: "idle"},
	}

	_, err := b.AssignTaskWithStrategy(task, agents)

	if err == nil {
		t.Error("AssignTaskWithStrategy() should error when no matching workstream")
	}
}

func TestBalancerWithStrategy_NoIdleAgents(t *testing.T) {
	b := NewBalancerWithStrategy(StrategyLeastLoaded)

	task := &Task{ID: "T1", Workstream: "api"}
	agents := []*Agent{
		{Name: "api-1", Workstream: "api", Status: "working"},
	}

	_, err := b.AssignTaskWithStrategy(task, agents)

	if err == nil {
		t.Error("AssignTaskWithStrategy() should error when no idle agents")
	}
}

func TestBalancerWithStrategy_SelectRoundRobinEmpty(t *testing.T) {
	b := NewBalancerWithStrategy(StrategyRoundRobin)

	result := b.selectRoundRobin([]*Agent{}, "api")

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
