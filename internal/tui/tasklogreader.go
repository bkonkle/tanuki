package tui

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// TaskLogReader reads task execution logs from disk files.
type TaskLogReader struct {
	projectRoot string
	logFilePath string
}

// NewTaskLogReader creates a new task log reader.
// The logFilePath should be relative to the project root.
func NewTaskLogReader(projectRoot, logFilePath string) *TaskLogReader {
	return &TaskLogReader{
		projectRoot: projectRoot,
		logFilePath: logFilePath,
	}
}

// LoadLogs reads the entire log file content.
// Returns the content as a string, or an error if the file cannot be read.
func (r *TaskLogReader) LoadLogs() (string, error) {
	if r.logFilePath == "" {
		return "", fmt.Errorf("no log file path specified")
	}

	fullPath := r.getFullPath()

	// Check if file exists
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return "", fmt.Errorf("log file not found: %s", r.logFilePath)
	}

	// Read file content
	content, err := os.ReadFile(fullPath) //nolint:gosec // Path is constructed from trusted project config
	if err != nil {
		return "", fmt.Errorf("read log file: %w", err)
	}

	return string(content), nil
}

// GetLastN returns the last N lines from the log file.
// This is more efficient than loading the entire file for large logs.
func (r *TaskLogReader) GetLastN(n int) (string, error) {
	if r.logFilePath == "" {
		return "", fmt.Errorf("no log file path specified")
	}

	fullPath := r.getFullPath()

	// Check if file exists
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return "", fmt.Errorf("log file not found: %s", r.logFilePath)
	}

	// Open file
	file, err := os.Open(fullPath) //nolint:gosec // Path is constructed from trusted project config
	if err != nil {
		return "", fmt.Errorf("open log file: %w", err)
	}
	defer func() { _ = file.Close() }()

	// Read all lines into a buffer
	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("scan log file: %w", err)
	}

	// Get last N lines
	start := 0
	if len(lines) > n {
		start = len(lines) - n
	}

	lastLines := lines[start:]
	return strings.Join(lastLines, "\n"), nil
}

// GetLineCount returns the total number of lines in the log file.
func (r *TaskLogReader) GetLineCount() (int, error) {
	if r.logFilePath == "" {
		return 0, fmt.Errorf("no log file path specified")
	}

	fullPath := r.getFullPath()

	// Check if file exists
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return 0, fmt.Errorf("log file not found: %s", r.logFilePath)
	}

	// Open file
	file, err := os.Open(fullPath) //nolint:gosec // Path is constructed from trusted project config
	if err != nil {
		return 0, fmt.Errorf("open log file: %w", err)
	}
	defer func() { _ = file.Close() }()

	// Count lines
	count := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		count++
	}

	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("scan log file: %w", err)
	}

	return count, nil
}

// Exists checks if the log file exists.
func (r *TaskLogReader) Exists() bool {
	if r.logFilePath == "" {
		return false
	}

	fullPath := r.getFullPath()
	_, err := os.Stat(fullPath)
	return err == nil
}

// GetFilePath returns the relative log file path.
func (r *TaskLogReader) GetFilePath() string {
	return r.logFilePath
}

// getFullPath returns the absolute path to the log file.
func (r *TaskLogReader) getFullPath() string {
	if filepath.IsAbs(r.logFilePath) {
		return filepath.Clean(r.logFilePath)
	}
	return filepath.Clean(filepath.Join(r.projectRoot, r.logFilePath))
}
