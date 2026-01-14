package agent

import (
	"testing"
	"time"
)

func TestBuildWorkstreamAgentName(t *testing.T) {
	tests := []struct {
		projectName string
		workstream  string
		want        string
	}{
		{"auth-feature", "oauth", "auth-feature-oauth"},
		{"api-refactor", "main", "api-refactor-main"},
		{"", "standalone", "standalone"},                    // Root task
		{"My Project", "Feature A", "my-project-feature-a"}, // Spaces converted
	}

	for _, tt := range tests {
		t.Run(tt.projectName+"/"+tt.workstream, func(t *testing.T) {
			got := buildWorkstreamAgentName(tt.projectName, tt.workstream)
			if got != tt.want {
				t.Errorf("buildWorkstreamAgentName(%q, %q) = %q, want %q",
					tt.projectName, tt.workstream, got, tt.want)
			}
		})
	}
}

func TestDefaultWorkstreamConfig(t *testing.T) {
	cfg := DefaultWorkstreamConfig()

	if cfg.PollInterval != 30*time.Second {
		t.Errorf("PollInterval = %v, want 30s", cfg.PollInterval)
	}
	if cfg.MaxWaitTime != 24*time.Hour {
		t.Errorf("MaxWaitTime = %v, want 24h", cfg.MaxWaitTime)
	}
	if cfg.MaxTurns != 50 {
		t.Errorf("MaxTurns = %d, want 50", cfg.MaxTurns)
	}
	if !cfg.Follow {
		t.Error("Follow should be true by default")
	}
}

func TestBuildWorktreeBranch(t *testing.T) {
	// Test that buildWorktreeBranch produces expected format
	branch := buildWorktreeBranch("auth-feature", "oauth")
	if branch != "tanuki/auth-feature-oauth" {
		t.Errorf("buildWorktreeBranch = %q, want tanuki/auth-feature-oauth", branch)
	}

	// Test with spaces (should be normalized)
	branch = buildWorktreeBranch("My Project", "Feature A")
	if branch != "tanuki/my-project-feature-a" {
		t.Errorf("buildWorktreeBranch = %q, want tanuki/my-project-feature-a", branch)
	}
}

func TestWorkstreamOrchestrator_SetRoleConcurrency(t *testing.T) {
	orch := NewWorkstreamOrchestrator(nil, nil, DefaultWorkstreamConfig())

	orch.SetRoleConcurrency("backend", 3)
	orch.SetRoleConcurrency("frontend", 2)
	orch.SetRoleConcurrency("qa", 0) // Should become 1

	if orch.roleConcurrency["backend"] != 3 {
		t.Errorf("backend concurrency = %d, want 3", orch.roleConcurrency["backend"])
	}
	if orch.roleConcurrency["frontend"] != 2 {
		t.Errorf("frontend concurrency = %d, want 2", orch.roleConcurrency["frontend"])
	}
	if orch.roleConcurrency["qa"] != 1 {
		t.Errorf("qa concurrency = %d, want 1 (clamped from 0)", orch.roleConcurrency["qa"])
	}
}

func TestWorkstreamOrchestrator_CanStartWorkstream(t *testing.T) {
	orch := NewWorkstreamOrchestrator(nil, nil, DefaultWorkstreamConfig())
	orch.SetRoleConcurrency("backend", 2)

	// Should be able to start when no runners active
	if !orch.CanStartWorkstream("backend") {
		t.Error("should be able to start first workstream")
	}

	// Simulate starting a workstream
	orch.activeRunners["backend"] = 1

	// Should still be able to start (limit is 2)
	if !orch.CanStartWorkstream("backend") {
		t.Error("should be able to start second workstream")
	}

	// Simulate starting another
	orch.activeRunners["backend"] = 2

	// Should NOT be able to start (at limit)
	if orch.CanStartWorkstream("backend") {
		t.Error("should NOT be able to start third workstream (at limit)")
	}
}

func TestWorkstreamOrchestrator_ReleaseWorkstream(t *testing.T) {
	orch := NewWorkstreamOrchestrator(nil, nil, DefaultWorkstreamConfig())
	orch.activeRunners["backend"] = 2

	orch.ReleaseWorkstream("backend")
	if orch.activeRunners["backend"] != 1 {
		t.Errorf("activeRunners[backend] = %d, want 1", orch.activeRunners["backend"])
	}

	orch.ReleaseWorkstream("backend")
	if orch.activeRunners["backend"] != 0 {
		t.Errorf("activeRunners[backend] = %d, want 0", orch.activeRunners["backend"])
	}

	// Should not go below zero
	orch.ReleaseWorkstream("backend")
	if orch.activeRunners["backend"] != 0 {
		t.Errorf("activeRunners[backend] = %d, want 0 (should not go negative)", orch.activeRunners["backend"])
	}
}

func TestWorkstreamOrchestrator_GetActiveCount(t *testing.T) {
	orch := NewWorkstreamOrchestrator(nil, nil, DefaultWorkstreamConfig())

	if orch.GetActiveCount("backend") != 0 {
		t.Error("active count should be 0 initially")
	}

	orch.activeRunners["backend"] = 3

	if orch.GetActiveCount("backend") != 3 {
		t.Errorf("active count = %d, want 3", orch.GetActiveCount("backend"))
	}
}
