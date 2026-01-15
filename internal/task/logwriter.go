package task

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	// LogsDir is the directory under .tanuki/ where task logs are stored
	LogsDir = "logs"
)

// LogWriter manages task execution log files.
// It creates and manages log files in the .tanuki/logs directory.
type LogWriter struct {
	projectRoot string
	logDir      string
}

// NewLogWriter creates a log writer for task execution logs.
// It ensures the log directory exists.
func NewLogWriter(projectRoot string) (*LogWriter, error) {
	logDir := filepath.Join(projectRoot, ".tanuki", LogsDir)

	// Ensure log directory exists
	if err := os.MkdirAll(logDir, 0750); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}

	return &LogWriter{
		projectRoot: projectRoot,
		logDir:      logDir,
	}, nil
}

// CreateTaskLogFile creates a new log file for task execution.
// Returns: file handle, relative path, error
func (w *LogWriter) CreateTaskLogFile(taskID string) (*os.File, string, error) {
	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("task-%s-%s.log", taskID, timestamp)
	fullPath := filepath.Join(w.logDir, filename)

	file, err := os.OpenFile(fullPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, "", fmt.Errorf("create task log file: %w", err)
	}

	// Return relative path from project root
	relativePath := filepath.Join(".tanuki", LogsDir, filename)
	return file, relativePath, nil
}

// CreateValidationLogFile creates a validation output log.
// Returns: file handle, relative path, error
func (w *LogWriter) CreateValidationLogFile(taskID string) (*os.File, string, error) {
	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("task-%s-%s-validate.log", taskID, timestamp)
	fullPath := filepath.Join(w.logDir, filename)

	file, err := os.OpenFile(fullPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, "", fmt.Errorf("create validation log file: %w", err)
	}

	// Return relative path from project root
	relativePath := filepath.Join(".tanuki", LogsDir, filename)
	return file, relativePath, nil
}

// GetLogPath returns the full path to a log file from a relative path.
func (w *LogWriter) GetLogPath(relativePath string) string {
	if filepath.IsAbs(relativePath) {
		return relativePath
	}
	return filepath.Join(w.projectRoot, relativePath)
}

// CleanOldLogs removes logs older than specified duration.
// This is useful for preventing log directory from growing unbounded.
func (w *LogWriter) CleanOldLogs(maxAge time.Duration) error {
	cutoff := time.Now().Add(-maxAge)

	entries, err := os.ReadDir(w.logDir)
	if err != nil {
		return fmt.Errorf("read log directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			logPath := filepath.Join(w.logDir, entry.Name())
			if err := os.Remove(logPath); err != nil {
				// Log but don't fail on individual file removal errors
				fmt.Printf("Warning: failed to remove old log %s: %v\n", entry.Name(), err)
			}
		}
	}

	return nil
}
