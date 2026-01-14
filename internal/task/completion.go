package task

import (
	"context"
	"fmt"
	"log"
	"time"
)

// CompletionHandler handles task completion detection and validation.
type CompletionHandler struct {
	taskMgr   TaskManagerInterface
	validator *Validator
	events    chan<- Event
}

// TaskManagerInterface defines the methods needed by CompletionHandler.
// This allows for loose coupling with the actual TaskManager implementation.
type TaskManagerInterface interface {
	Get(id string) (*Task, error)
	UpdateStatus(id string, status Status) error
	Unassign(id string) error
}

// NewCompletionHandler creates a new completion handler.
func NewCompletionHandler(taskMgr TaskManagerInterface, workdir string, events chan<- Event) *CompletionHandler {
	return &CompletionHandler{
		taskMgr:   taskMgr,
		validator: NewValidator(workdir),
		events:    events,
	}
}

// HandleAgentOutput processes output from an agent working on a task.
// It validates the output against completion criteria and updates task status.
func (h *CompletionHandler) HandleAgentOutput(ctx context.Context, taskID, agentName, output string) error {
	t, err := h.taskMgr.Get(taskID)
	if err != nil {
		return fmt.Errorf("get task: %w", err)
	}

	result := h.validator.Validate(ctx, t, output)

	// Update task status
	if err := h.taskMgr.UpdateStatus(taskID, result.Status); err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	// Emit event
	h.emitEvent(result, agentName)

	// Handle based on result
	switch result.Status {
	case StatusComplete:
		log.Printf("Task %s completed by %s", taskID, agentName)
		if err := h.taskMgr.Unassign(taskID); err != nil {
			log.Printf("Warning: failed to unassign task %s: %v", taskID, err)
		}
		now := time.Now()
		t.CompletedAt = &now

	case StatusFailed:
		log.Printf("Task %s failed: %s", taskID, result.Message)
		// Keep assigned for retry or manual intervention

	case StatusReview:
		log.Printf("Task %s needs review: %s", taskID, result.Message)
		// Keep assigned, human will review

	case StatusInProgress:
		// Still working, no action needed
	}

	return nil
}

// emitEvent sends an event to the event channel if configured.
func (h *CompletionHandler) emitEvent(result *ValidationResult, agentName string) {
	if h.events == nil {
		return
	}

	eventType := statusToEventType(result.Status)
	h.events <- Event{
		Type:      eventType,
		TaskID:    result.Task.ID,
		AgentName: agentName,
		Message:   result.Message,
		Timestamp: time.Now(),
	}
}

// GetValidator returns the underlying validator for direct access if needed.
func (h *CompletionHandler) GetValidator() *Validator {
	return h.validator
}
