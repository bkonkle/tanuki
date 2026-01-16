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

func TestWorkstreamOrchestrator_SetWorkstreamConcurrency(t *testing.T) {
	orch := NewWorkstreamOrchestrator(nil, nil, DefaultWorkstreamConfig())

	orch.SetWorkstreamConcurrency("auth", 3)
	orch.SetWorkstreamConcurrency("payments", 2)
	orch.SetWorkstreamConcurrency("logging", 0) // Should become 1

	if orch.workstreamConcurrency["auth"] != 3 {
		t.Errorf("auth concurrency = %d, want 3", orch.workstreamConcurrency["auth"])
	}
	if orch.workstreamConcurrency["payments"] != 2 {
		t.Errorf("payments concurrency = %d, want 2", orch.workstreamConcurrency["payments"])
	}
	if orch.workstreamConcurrency["logging"] != 1 {
		t.Errorf("logging concurrency = %d, want 1 (clamped from 0)", orch.workstreamConcurrency["logging"])
	}
}

func TestWorkstreamOrchestrator_CanStartWorkstream(t *testing.T) {
	orch := NewWorkstreamOrchestrator(nil, nil, DefaultWorkstreamConfig())
	orch.SetWorkstreamConcurrency("auth", 2)

	// Should be able to start when no runners active
	if !orch.CanStartWorkstream("auth") {
		t.Error("should be able to start first workstream")
	}

	// Simulate starting a workstream
	orch.activeRunners["auth"] = 1

	// Should still be able to start (limit is 2)
	if !orch.CanStartWorkstream("auth") {
		t.Error("should be able to start second workstream")
	}

	// Simulate starting another
	orch.activeRunners["auth"] = 2

	// Should NOT be able to start (at limit)
	if orch.CanStartWorkstream("auth") {
		t.Error("should NOT be able to start third workstream (at limit)")
	}
}

func TestWorkstreamOrchestrator_ReleaseWorkstream(t *testing.T) {
	orch := NewWorkstreamOrchestrator(nil, nil, DefaultWorkstreamConfig())
	orch.activeRunners["auth"] = 2

	orch.ReleaseWorkstream("auth")
	if orch.activeRunners["auth"] != 1 {
		t.Errorf("activeRunners[auth] = %d, want 1", orch.activeRunners["auth"])
	}

	orch.ReleaseWorkstream("auth")
	if orch.activeRunners["auth"] != 0 {
		t.Errorf("activeRunners[auth] = %d, want 0", orch.activeRunners["auth"])
	}

	// Should not go below zero
	orch.ReleaseWorkstream("auth")
	if orch.activeRunners["auth"] != 0 {
		t.Errorf("activeRunners[auth] = %d, want 0 (should not go negative)", orch.activeRunners["auth"])
	}
}

func TestWorkstreamOrchestrator_GetActiveCount(t *testing.T) {
	orch := NewWorkstreamOrchestrator(nil, nil, DefaultWorkstreamConfig())

	if orch.GetActiveCount("auth") != 0 {
		t.Error("active count should be 0 initially")
	}

	orch.activeRunners["auth"] = 3

	if orch.GetActiveCount("auth") != 3 {
		t.Errorf("active count = %d, want 3", orch.GetActiveCount("auth"))
	}
}
