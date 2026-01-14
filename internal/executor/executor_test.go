package executor

import (
	"bytes"
	"strings"
	"testing"

	"github.com/bkonkle/tanuki/internal/docker"
)

// Mock Docker manager for testing
type mockDockerManager struct {
	execFn             func(containerID string, cmd []string, opts docker.ExecOptions) error
	execWithOutputFn   func(containerID string, cmd []string) (string, error)
	containerRunningFn func(containerID string) bool
}

func (m *mockDockerManager) Exec(containerID string, cmd []string, opts docker.ExecOptions) error {
	if m.execFn != nil {
		return m.execFn(containerID, cmd, opts)
	}
	return nil
}

func (m *mockDockerManager) ExecWithOutput(containerID string, cmd []string) (string, error) {
	if m.execWithOutputFn != nil {
		return m.execWithOutputFn(containerID, cmd)
	}
	return "", nil
}

func (m *mockDockerManager) ContainerRunning(containerID string) bool {
	if m.containerRunningFn != nil {
		return m.containerRunningFn(containerID)
	}
	return true
}

func TestNewExecutor(t *testing.T) {
	docker := &mockDockerManager{}
	executor := NewExecutor(docker)

	if executor == nil {
		t.Fatal("NewExecutor returned nil")
	}
	if executor.docker == nil {
		t.Fatal("executor.docker is nil")
	}
}

func TestBuildCommand(t *testing.T) {
	docker := &mockDockerManager{}
	executor := NewExecutor(docker)

	opts := ExecuteOptions{
		AllowedTools:    []string{"Read", "Write", "Edit"},
		DisallowedTools: []string{"Bash"},
		MaxTurns:        50,
		Model:           "claude-sonnet-4-5-20250514",
		SystemPrompt:    "Test prompt",
	}

	cmd := executor.buildCommand("test task", opts)

	// Verify core command structure
	if len(cmd) < 3 {
		t.Fatalf("command too short: %v", cmd)
	}
	if cmd[0] != "claude" {
		t.Errorf("expected first arg 'claude', got %q", cmd[0])
	}
	if cmd[1] != "-p" {
		t.Errorf("expected second arg '-p', got %q", cmd[1])
	}
	if cmd[2] != "test task" {
		t.Errorf("expected third arg 'test task', got %q", cmd[2])
	}

	// Verify flags are present
	cmdStr := strings.Join(cmd, " ")
	if !strings.Contains(cmdStr, "--output-format stream-json") {
		t.Error("missing --output-format flag")
	}
	if !strings.Contains(cmdStr, "--allowedTools Read,Write,Edit") {
		t.Error("missing or incorrect --allowedTools flag")
	}
	if !strings.Contains(cmdStr, "--disallowedTools Bash") {
		t.Error("missing or incorrect --disallowedTools flag")
	}
	if !strings.Contains(cmdStr, "--max-turns 50") {
		t.Error("missing --max-turns flag")
	}
	if !strings.Contains(cmdStr, "--model claude-sonnet-4-5-20250514") {
		t.Error("missing --model flag")
	}
	if !strings.Contains(cmdStr, "--append-system-prompt Test prompt") {
		t.Error("missing --append-system-prompt flag")
	}
}

func TestRun(t *testing.T) {
	outputCalled := false
	docker := &mockDockerManager{
		execWithOutputFn: func(containerID string, cmd []string) (string, error) {
			outputCalled = true
			// Simulate stream-json output with session ID
			return `{"type":"session_start","session_id":"test-123"}
{"type":"content","content":"Hello"}`, nil
		},
	}
	executor := NewExecutor(docker)

	opts := ExecuteOptions{
		AllowedTools: []string{"Read", "Write"},
	}

	result, err := executor.Run("container-123", "test prompt", opts)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if !outputCalled {
		t.Error("ExecWithOutput was not called")
	}

	if result.SessionID != "test-123" {
		t.Errorf("expected session ID 'test-123', got %q", result.SessionID)
	}

	if result.StartedAt.IsZero() {
		t.Error("StartedAt not set")
	}
	if result.CompletedAt.IsZero() {
		t.Error("CompletedAt not set")
	}
}

func TestRunFollow(t *testing.T) {
	execCalled := false
	docker := &mockDockerManager{
		execFn: func(containerID string, cmd []string, opts docker.ExecOptions) error {
			execCalled = true
			// Simulate writing to stdout
			if opts.Stdout != nil {
				opts.Stdout.Write([]byte(`{"type":"session_start","session_id":"test-456"}
{"type":"content","content":"Hello world"}`))
			}
			return nil
		},
	}
	executor := NewExecutor(docker)

	opts := ExecuteOptions{
		AllowedTools: []string{"Read", "Write"},
	}

	var output bytes.Buffer
	result, err := executor.RunFollow("container-123", "test prompt", opts, &output)
	if err != nil {
		t.Fatalf("RunFollow failed: %v", err)
	}

	if !execCalled {
		t.Error("Exec was not called")
	}

	if result.SessionID != "test-456" {
		t.Errorf("expected session ID 'test-456', got %q", result.SessionID)
	}

	outputStr := output.String()
	if !strings.Contains(outputStr, "Hello world") {
		t.Errorf("output not captured: %q", outputStr)
	}
}

func TestExtractSessionID(t *testing.T) {
	docker := &mockDockerManager{}
	executor := NewExecutor(docker)

	tests := []struct {
		name     string
		output   string
		expected string
	}{
		{
			name:     "valid session ID",
			output:   `{"type":"session_start","session_id":"abc-123"}`,
			expected: "abc-123",
		},
		{
			name: "multiline with session ID",
			output: `{"type":"status","message":"Starting"}
{"type":"session_start","session_id":"xyz-789"}
{"type":"content","content":"Done"}`,
			expected: "xyz-789",
		},
		{
			name:     "no session ID",
			output:   `{"type":"content","content":"Hello"}`,
			expected: "",
		},
		{
			name:     "invalid JSON",
			output:   "not json",
			expected: "",
		},
		{
			name:     "empty output",
			output:   "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := executor.extractSessionID(tt.output)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestParseCommand(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "simple command",
			input:    "npm test",
			expected: []string{"npm", "test"},
		},
		{
			name:     "command with flags",
			input:    "npm run test --coverage",
			expected: []string{"npm", "run", "test", "--coverage"},
		},
		{
			name:     "command with quoted string",
			input:    `echo "hello world"`,
			expected: []string{"echo", "hello world"},
		},
		{
			name:     "command with single quotes",
			input:    "echo 'hello world'",
			expected: []string{"echo", "hello world"},
		},
		{
			name:     "complex command",
			input:    `git commit -m "feat: add new feature"`,
			expected: []string{"git", "commit", "-m", "feat: add new feature"},
		},
		{
			name:     "empty string",
			input:    "",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseCommand(tt.input)
			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d args, got %d: %v", len(tt.expected), len(result), result)
			}
			for i, arg := range result {
				if arg != tt.expected[i] {
					t.Errorf("arg %d: expected %q, got %q", i, tt.expected[i], arg)
				}
			}
		})
	}
}

func TestVerifyClaudeInstalled(t *testing.T) {
	docker := &mockDockerManager{
		execWithOutputFn: func(containerID string, cmd []string) (string, error) {
			if cmd[0] == "which" && cmd[1] == "claude" {
				return "/usr/local/bin/claude\n", nil
			}
			return "", nil
		},
	}
	executor := NewExecutor(docker)

	err := executor.VerifyClaudeInstalled("container-123")
	if err != nil {
		t.Errorf("VerifyClaudeInstalled failed: %v", err)
	}
}

func TestVerifyClaudeInstalled_NotFound(t *testing.T) {
	docker := &mockDockerManager{
		execWithOutputFn: func(containerID string, cmd []string) (string, error) {
			return "", ErrClaudeNotFound
		},
	}
	executor := NewExecutor(docker)

	err := executor.VerifyClaudeInstalled("container-123")
	if err != ErrClaudeNotFound {
		t.Errorf("expected ErrClaudeNotFound, got %v", err)
	}
}

func TestIsRunning(t *testing.T) {
	docker := &mockDockerManager{
		execWithOutputFn: func(containerID string, cmd []string) (string, error) {
			// Simulate pgrep finding claude processes
			return "12345\n67890\n", nil
		},
	}
	executor := NewExecutor(docker)

	running, err := executor.IsRunning("container-123")
	if err != nil {
		t.Fatalf("IsRunning failed: %v", err)
	}
	if !running {
		t.Error("expected IsRunning to return true")
	}
}

func TestIsRunning_NotRunning(t *testing.T) {
	docker := &mockDockerManager{
		execWithOutputFn: func(containerID string, cmd []string) (string, error) {
			// Simulate pgrep not finding any processes
			return "", nil
		},
		containerRunningFn: func(containerID string) bool {
			return true
		},
	}
	executor := NewExecutor(docker)

	running, err := executor.IsRunning("container-123")
	if err != nil {
		t.Fatalf("IsRunning failed: %v", err)
	}
	if running {
		t.Error("expected IsRunning to return false")
	}
}

func TestRun_ContainerNotRunning(t *testing.T) {
	docker := &mockDockerManager{
		containerRunningFn: func(containerID string) bool {
			return false
		},
	}
	executor := NewExecutor(docker)

	opts := ExecuteOptions{}
	_, err := executor.Run("container-123", "test prompt", opts)
	if err == nil {
		t.Fatal("expected error when container not running")
	}
	if !strings.Contains(err.Error(), "not running") {
		t.Errorf("unexpected error message: %v", err)
	}
}
