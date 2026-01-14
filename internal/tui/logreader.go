package tui

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
)

// LogReader streams logs from a Docker container.
type LogReader struct {
	containerName string
	outputCh      chan LogLine
	stopCh        chan struct{}
	cmd           *exec.Cmd
}

// NewLogReader creates a new log reader for the specified agent.
func NewLogReader(agentName string) *LogReader {
	return &LogReader{
		containerName: fmt.Sprintf("tanuki-%s", agentName),
		outputCh:      make(chan LogLine, 100),
		stopCh:        make(chan struct{}),
	}
}

// Start begins streaming logs from the container.
func (r *LogReader) Start() error {
	// Use docker logs with follow, tail last 100 lines
	// #nosec G204 - containerName is constructed internally from agentName
	r.cmd = exec.Command("docker", "logs", "-f", "--tail", "100", r.containerName)

	stdout, err := r.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := r.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := r.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start logs command: %w", err)
	}

	// Read stdout in a goroutine
	go r.readOutput(stdout, "stdout")
	// Read stderr in a goroutine (Claude Code often writes here)
	go r.readOutput(stderr, "stderr")

	return nil
}

// readOutput reads lines from the given reader and sends them to the output channel.
func (r *LogReader) readOutput(reader io.Reader, _ string) {
	scanner := bufio.NewScanner(reader)

	// Set a larger buffer size for long lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		select {
		case <-r.stopCh:
			return
		default:
			text := scanner.Text()
			line := LogLine{
				Timestamp: time.Now(),
				Agent:     r.containerName,
				Content:   text,
				Level:     detectLogLevel(text),
			}

			// Non-blocking send to avoid goroutine leak
			select {
			case r.outputCh <- line:
			case <-r.stopCh:
				return
			default:
				// Channel full, drop old message
			}
		}
	}
}

// Stop stops the log reader and cleans up resources.
func (r *LogReader) Stop() {
	close(r.stopCh)
	if r.cmd != nil && r.cmd.Process != nil {
		_ = r.cmd.Process.Kill()
	}
}

// OutputCh returns the channel for receiving log lines.
func (r *LogReader) OutputCh() <-chan LogLine {
	return r.outputCh
}

// detectLogLevel attempts to detect the log level from the line content.
func detectLogLevel(line string) string {
	lower := strings.ToLower(line)

	// Check for error patterns
	if strings.Contains(lower, "error") ||
		strings.Contains(lower, "err:") ||
		strings.Contains(lower, "fatal") ||
		strings.Contains(lower, "panic") {
		return "error"
	}

	// Check for warning patterns
	if strings.Contains(lower, "warn") ||
		strings.Contains(lower, "warning") {
		return "warn"
	}

	// Check for debug patterns
	if strings.Contains(lower, "debug") ||
		strings.Contains(lower, "trace") {
		return "debug"
	}

	// Default to info
	return "info"
}
