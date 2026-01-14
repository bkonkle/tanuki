package task

import (
	"fmt"
	"sort"
	"sync"
)

// Balancer handles task-to-agent assignment based on role matching and workload.
// It tracks active task counts per agent and selects the least-loaded agent
// when assigning new tasks.
type Balancer struct {
	mu        sync.RWMutex
	workloads map[string]int // agentName -> active task count
}

// Agent represents an agent for balancing purposes.
// This is an interface-based type that doesn't depend on the concrete agent implementation.
type Agent struct {
	Name   string
	Role   string
	Status string // idle, working, stopped
}

// NewBalancer creates a new workload balancer.
func NewBalancer() *Balancer {
	return &Balancer{
		workloads: make(map[string]int),
	}
}

// AssignTask selects the best agent for a task based on role matching and workload.
// Returns an error if no suitable agent is available.
func (b *Balancer) AssignTask(t *Task, agents []*Agent) (*Agent, error) {
	if t == nil {
		return nil, fmt.Errorf("task is nil")
	}

	// Filter by role
	candidates := b.filterByRole(agents, t.Role)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no agents available for role %q", t.Role)
	}

	// Filter by availability
	available := b.filterAvailable(candidates)
	if len(available) == 0 {
		return nil, fmt.Errorf("no idle agents for role %q", t.Role)
	}

	// Select least loaded
	selected := b.selectLeastLoaded(available)

	return selected, nil
}

func (b *Balancer) filterByRole(agents []*Agent, role string) []*Agent {
	var result []*Agent
	for _, ag := range agents {
		if ag.Role == role {
			result = append(result, ag)
		}
	}
	return result
}

func (b *Balancer) filterAvailable(agents []*Agent) []*Agent {
	var result []*Agent
	for _, ag := range agents {
		if ag.Status == "idle" {
			result = append(result, ag)
		}
	}
	return result
}

func (b *Balancer) selectLeastLoaded(agents []*Agent) *Agent {
	if len(agents) == 0 {
		return nil
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	// Sort by workload (ascending)
	sort.Slice(agents, func(i, j int) bool {
		loadI := b.workloads[agents[i].Name]
		loadJ := b.workloads[agents[j].Name]
		return loadI < loadJ
	})

	return agents[0]
}

// TrackAssignment records that an agent was assigned a task.
func (b *Balancer) TrackAssignment(agentName string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.workloads[agentName]++
}

// TrackCompletion records that an agent completed a task.
func (b *Balancer) TrackCompletion(agentName string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.workloads[agentName] > 0 {
		b.workloads[agentName]--
	}
}

// GetWorkload returns current task count for an agent.
func (b *Balancer) GetWorkload(agentName string) int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return b.workloads[agentName]
}

// GetTotalWorkload returns total tasks across all agents.
func (b *Balancer) GetTotalWorkload() int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	total := 0
	for _, count := range b.workloads {
		total += count
	}
	return total
}

// ResetWorkload clears workload tracking for an agent.
func (b *Balancer) ResetWorkload(agentName string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	delete(b.workloads, agentName)
}

// GetIdleAgents returns idle agents, optionally filtered by role.
// If role is empty, returns all idle agents.
func (b *Balancer) GetIdleAgents(agents []*Agent, role string) []*Agent {
	result := make([]*Agent, 0, len(agents))

	for _, ag := range agents {
		if ag.Status != "idle" {
			continue
		}
		if role != "" && ag.Role != role {
			continue
		}
		result = append(result, ag)
	}

	return result
}

// GetAgentsByRole returns all agents with a specific role.
func (b *Balancer) GetAgentsByRole(agents []*Agent, role string) []*Agent {
	var result []*Agent

	for _, ag := range agents {
		if ag.Role == role {
			result = append(result, ag)
		}
	}

	return result
}

// GetRolesNeeded returns roles that have pending tasks but no agents.
func (b *Balancer) GetRolesNeeded(tasks []*Task, agents []*Agent) []string {
	// Roles with pending tasks
	taskRoles := make(map[string]bool)
	for _, t := range tasks {
		if t.Status == StatusPending || t.Status == StatusBlocked {
			taskRoles[t.Role] = true
		}
	}

	// Roles with agents
	agentRoles := make(map[string]bool)
	for _, ag := range agents {
		if ag.Role != "" {
			agentRoles[ag.Role] = true
		}
	}

	// Find missing roles
	var needed []string
	for role := range taskRoles {
		if !agentRoles[role] {
			needed = append(needed, role)
		}
	}

	return needed
}

// Stats returns workload statistics.
func (b *Balancer) Stats() *BalancerStats {
	b.mu.RLock()
	defer b.mu.RUnlock()

	stats := &BalancerStats{
		AgentWorkloads: make(map[string]int),
	}

	for agent, count := range b.workloads {
		stats.AgentWorkloads[agent] = count
		stats.TotalTasks += count
		if count > stats.MaxWorkload {
			stats.MaxWorkload = count
			stats.BusiestAgent = agent
		}
	}

	if len(b.workloads) > 0 {
		stats.AvgWorkload = float64(stats.TotalTasks) / float64(len(b.workloads))
	}

	return stats
}

// BalancerStats contains workload statistics.
type BalancerStats struct {
	TotalTasks     int
	AgentWorkloads map[string]int
	MaxWorkload    int
	BusiestAgent   string
	AvgWorkload    float64
}

// Strategy defines how to select an agent.
type Strategy int

const (
	// StrategyLeastLoaded selects agent with fewest active tasks.
	StrategyLeastLoaded Strategy = iota

	// StrategyRoundRobin cycles through agents.
	StrategyRoundRobin

	// StrategyRandom selects randomly.
	StrategyRandom
)

// BalancerWithStrategy supports different balancing strategies.
type BalancerWithStrategy struct {
	*Balancer
	strategy   Strategy
	roundRobin map[string]int // role -> last index
	mu         sync.Mutex     // for roundRobin state
}

// NewBalancerWithStrategy creates a balancer with a specific strategy.
func NewBalancerWithStrategy(strategy Strategy) *BalancerWithStrategy {
	return &BalancerWithStrategy{
		Balancer:   NewBalancer(),
		strategy:   strategy,
		roundRobin: make(map[string]int),
	}
}

// AssignTaskWithStrategy selects an agent using the configured strategy.
func (b *BalancerWithStrategy) AssignTaskWithStrategy(t *Task, agents []*Agent) (*Agent, error) {
	if t == nil {
		return nil, fmt.Errorf("task is nil")
	}

	// Filter by role
	candidates := b.filterByRole(agents, t.Role)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no agents available for role %q", t.Role)
	}

	// Filter by availability
	available := b.filterAvailable(candidates)
	if len(available) == 0 {
		return nil, fmt.Errorf("no idle agents for role %q", t.Role)
	}

	// Select based on strategy
	return b.selectAgent(available, t.Role), nil
}

func (b *BalancerWithStrategy) selectAgent(agents []*Agent, role string) *Agent {
	switch b.strategy {
	case StrategyRoundRobin:
		return b.selectRoundRobin(agents, role)
	case StrategyRandom:
		return b.selectRandom(agents)
	default:
		return b.selectLeastLoaded(agents)
	}
}

func (b *BalancerWithStrategy) selectRoundRobin(agents []*Agent, role string) *Agent {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(agents) == 0 {
		return nil
	}

	idx := b.roundRobin[role]
	selected := agents[idx%len(agents)]
	b.roundRobin[role] = idx + 1

	return selected
}

func (b *BalancerWithStrategy) selectRandom(agents []*Agent) *Agent {
	if len(agents) == 0 {
		return nil
	}
	// Simple implementation - in production would use proper random
	return agents[0]
}
