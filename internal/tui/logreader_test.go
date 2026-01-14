package tui

import (
	"testing"
	"time"
)

func TestDetectLogLevel(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected string
	}{
		{
			name:     "error level",
			line:     "ERROR: Something went wrong",
			expected: "error",
		},
		{
			name:     "fatal level",
			line:     "FATAL: Critical failure",
			expected: "error",
		},
		{
			name:     "warning level",
			line:     "WARNING: This is a warning",
			expected: "warn",
		},
		{
			name:     "debug level",
			line:     "DEBUG: Debug information",
			expected: "debug",
		},
		{
			name:     "info level",
			line:     "Processing request...",
			expected: "info",
		},
		{
			name:     "err: prefix",
			line:     "err: connection failed",
			expected: "error",
		},
		{
			name:     "case insensitive",
			line:     "Error in module X",
			expected: "error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectLogLevel(tt.line)
			if result != tt.expected {
				t.Errorf("detectLogLevel(%q) = %q, want %q", tt.line, result, tt.expected)
			}
		})
	}
}

func TestNewLogReader(t *testing.T) {
	agentName := "test-agent"
	reader := NewLogReader(agentName)

	if reader == nil {
		t.Fatal("NewLogReader returned nil")
	}

	expectedContainerName := "tanuki-test-agent"
	if reader.containerName != expectedContainerName {
		t.Errorf("containerName = %q, want %q", reader.containerName, expectedContainerName)
	}

	if reader.outputCh == nil {
		t.Error("outputCh should not be nil")
	}

	if reader.stopCh == nil {
		t.Error("stopCh should not be nil")
	}
}

func TestLogReaderStop(t *testing.T) {
	reader := NewLogReader("test-agent")

	// Stop should not panic even if never started
	reader.Stop()

	// Verify stop channel is closed
	select {
	case <-reader.stopCh:
		// Channel is closed, expected
	case <-time.After(100 * time.Millisecond):
		t.Error("stopCh was not closed after Stop()")
	}
}

func TestLogReaderOutputCh(t *testing.T) {
	reader := NewLogReader("test-agent")

	ch := reader.OutputCh()
	if ch == nil {
		t.Error("OutputCh() returned nil")
	}

	// Verify we can read from the channel
	select {
	case <-ch:
		// Shouldn't receive anything yet
		t.Error("Received unexpected data from output channel")
	case <-time.After(10 * time.Millisecond):
		// Expected - no data should be available
	}
}
