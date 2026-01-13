---
id: TANK-036
title: Workload Balancing
status: todo
priority: medium
estimate: M
depends_on: [TANK-030, TANK-006]
workstream: B
phase: 3
---

# Workload Balancing

## Summary

Implement workload balancing strategies for distributing tasks across agents. The balancer decides which agent should receive a task based on role matching, current workload, and agent availability.

## Acceptance Criteria

- [ ] Role-based agent matching (task role → agent role)
- [ ] Least-loaded agent selection within role
- [ ] Agent availability tracking (idle, working, stopped)
- [ ] Configurable agents per role
- [ ] Get idle agents by role
- [ ] Workload statistics per agent
- [ ] Unit tests with 80%+ coverage

## Technical Details

### Balancer Interface

```go
// internal/task/balancer.go
package task

import (
    "fmt"
    "sort"
    "sync"
)

// Balancer handles task-to-agent assignment
type Balancer struct {
    mu        sync.RWMutex
    workloads map[string]int // agentName -> active task count
}

// NewBalancer creates a new workload balancer
func NewBalancer() *Balancer {
    return &Balancer{
        workloads: make(map[string]int),
    }
}

// Agent represents an agent for balancing purposes
type Agent struct {
    Name   string
    Role   string
    Status string // idle, working, stopped
}

// AssignTask selects the best agent for a task
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
```

### Workload Tracking

```go
// TrackAssignment records that an agent was assigned a task
func (b *Balancer) TrackAssignment(agentName string) {
    b.mu.Lock()
    defer b.mu.Unlock()

    b.workloads[agentName]++
}

// TrackCompletion records that an agent completed a task
func (b *Balancer) TrackCompletion(agentName string) {
    b.mu.Lock()
    defer b.mu.Unlock()

    if b.workloads[agentName] > 0 {
        b.workloads[agentName]--
    }
}

// GetWorkload returns current task count for an agent
func (b *Balancer) GetWorkload(agentName string) int {
    b.mu.RLock()
    defer b.mu.RUnlock()

    return b.workloads[agentName]
}

// GetTotalWorkload returns total tasks across all agents
func (b *Balancer) GetTotalWorkload() int {
    b.mu.RLock()
    defer b.mu.RUnlock()

    total := 0
    for _, count := range b.workloads {
        total += count
    }
    return total
}

// ResetWorkload clears workload tracking for an agent
func (b *Balancer) ResetWorkload(agentName string) {
    b.mu.Lock()
    defer b.mu.Unlock()

    delete(b.workloads, agentName)
}
```

### Agent Queries

```go
// GetIdleAgents returns idle agents, optionally filtered by role
func (b *Balancer) GetIdleAgents(agents []*Agent, role string) []*Agent {
    var result []*Agent

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

// GetAgentsByRole returns all agents with a specific role
func (b *Balancer) GetAgentsByRole(agents []*Agent, role string) []*Agent {
    var result []*Agent

    for _, ag := range agents {
        if ag.Role == role {
            result = append(result, ag)
        }
    }

    return result
}

// GetRolesNeeded returns roles that have pending tasks but no agents
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
```

### Balancing Strategies

```go
// Strategy defines how to select an agent
type Strategy int

const (
    // StrategyLeastLoaded selects agent with fewest active tasks
    StrategyLeastLoaded Strategy = iota

    // StrategyRoundRobin cycles through agents
    StrategyRoundRobin

    // StrategyRandom selects randomly
    StrategyRandom
)

// BalancerWithStrategy supports different balancing strategies
type BalancerWithStrategy struct {
    *Balancer
    strategy   Strategy
    roundRobin map[string]int // role -> last index
}

func NewBalancerWithStrategy(strategy Strategy) *BalancerWithStrategy {
    return &BalancerWithStrategy{
        Balancer:   NewBalancer(),
        strategy:   strategy,
        roundRobin: make(map[string]int),
    }
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
    // Use time-based seed in real implementation
    return agents[0]
}
```

### Statistics

```go
// Stats returns workload statistics
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

type BalancerStats struct {
    TotalTasks     int
    AgentWorkloads map[string]int
    MaxWorkload    int
    BusiestAgent   string
    AvgWorkload    float64
}
```

### Integration Example

```go
// Example usage in project orchestrator
func (o *Orchestrator) assignPendingTasks() {
    tasks := o.taskMgr.GetPending()
    agents, _ := o.agentMgr.List()

    for _, t := range tasks {
        // Skip blocked tasks
        if o.resolver.IsBlocked(t.ID) {
            continue
        }

        // Find best agent
        agent, err := o.balancer.AssignTask(t, agents)
        if err != nil {
            log.Printf("Cannot assign %s: %v", t.ID, err)
            continue
        }

        // Assign and track
        o.taskMgr.Assign(t.ID, agent.Name)
        o.balancer.TrackAssignment(agent.Name)

        // Update agent status (for next iteration)
        for i, ag := range agents {
            if ag.Name == agent.Name {
                agents[i].Status = "working"
                break
            }
        }

        log.Printf("Assigned %s to %s", t.ID, agent.Name)
    }
}
```

## Testing

### Unit Tests

```go
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

func TestBalancer_GetRolesNeeded(t *testing.T) {
    b := NewBalancer()

    tasks := []*Task{
        {ID: "T1", Role: "backend", Status: StatusPending},
        {ID: "T2", Role: "frontend", Status: StatusPending},
        {ID: "T3", Role: "qa", Status: StatusPending},
    }

    agents := []*Agent{
        {Name: "be-1", Role: "backend"},
    }

    needed := b.GetRolesNeeded(tasks, agents)

    // Should need frontend and qa
    if len(needed) != 2 {
        t.Errorf("GetRolesNeeded() = %v, want [frontend, qa]", needed)
    }
}
```

## Error Handling

| Scenario | Behavior |
|----------|----------|
| No agents for role | Return error with role name |
| No idle agents | Return error |
| Nil task | Return error |
| Empty agent list | Return error |

## Out of Scope

- Agent capacity limits
- Task priority in assignment
- Geographic/resource affinity
- Agent health monitoring
- Automatic agent scaling

## Notes

The balancer is stateless regarding agent status - it receives the current agent list each time. Workload tracking is maintained separately and should be synchronized with actual task completion events.

The least-loaded strategy works well for homogeneous tasks. For varying task complexity, consider adding task weights.
