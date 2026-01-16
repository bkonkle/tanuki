// Package tools provides tool validation and filtering for Tanuki agents.
package tools

import (
	"fmt"
	"sort"
	"strings"

	"github.com/bkonkle/tanuki/internal/config"
)

// ValidTools lists all valid Claude Code tool names.
var ValidTools = []string{
	"Read",
	"Write",
	"Edit",
	"Bash",
	"Glob",
	"Grep",
	"TodoWrite",
	"Task",
	"WebFetch",
	"WebSearch",
}

// FilterOptions specifies tool filtering behavior.
type FilterOptions struct {
	// AllowedTools lists tools that are permitted. Empty means allow all.
	AllowedTools []string

	// DisallowedTools lists tools that are explicitly forbidden.
	DisallowedTools []string
}

// FilterResult contains the resolved tool lists and any validation errors.
type FilterResult struct {
	// AllowedTools is the final list of tools permitted for use.
	// Empty means no restrictions (allow all).
	AllowedTools []string

	// DisallowedTools is the final list of tools that are forbidden.
	DisallowedTools []string

	// Errors contains any validation errors encountered.
	Errors []error
}

// FilterTools resolves tool permissions based on workstream configuration and CLI overrides.
//
// Priority for allowed tools:
//  1. Explicit CLI override (opts.AllowedTools) - completely replaces workstream config
//  2. Workstream-based allowed tools (ws.AllowedTools)
//  3. No restrictions (empty list = allow all)
//
// Disallowed tools are additive:
//   - Workstream disallowed tools + CLI disallowed tools
//   - Both lists are combined and deduplicated
//
// Validation:
//   - All tool names must be in ValidTools
//   - Tools cannot appear in both allowed and disallowed lists
//   - Returns errors for invalid configurations but still returns a usable result
func FilterTools(ws *config.WorkstreamConfig, opts FilterOptions) *FilterResult {
	result := &FilterResult{
		Errors: []error{},
	}

	// Resolve allowed tools
	if len(opts.AllowedTools) > 0 {
		// CLI override - completely replaces workstream config
		result.AllowedTools = make([]string, len(opts.AllowedTools))
		copy(result.AllowedTools, opts.AllowedTools)
	} else if ws != nil && len(ws.AllowedTools) > 0 {
		// Use workstream-based restrictions
		result.AllowedTools = make([]string, len(ws.AllowedTools))
		copy(result.AllowedTools, ws.AllowedTools)
	}
	// else: no allowed tools specified = allow all (empty list)

	// Resolve disallowed tools (additive)
	disallowedSet := make(map[string]bool)

	// Add workstream-based disallowed tools
	if ws != nil {
		for _, tool := range ws.DisallowedTools {
			disallowedSet[tool] = true
		}
	}

	// Add CLI-specified disallowed tools
	for _, tool := range opts.DisallowedTools {
		disallowedSet[tool] = true
	}

	// Convert set to slice
	result.DisallowedTools = make([]string, 0, len(disallowedSet))
	for tool := range disallowedSet {
		result.DisallowedTools = append(result.DisallowedTools, tool)
	}

	// Sort for consistent output
	sort.Strings(result.AllowedTools)
	sort.Strings(result.DisallowedTools)

	// Validate tool names
	validToolSet := make(map[string]bool)
	for _, tool := range ValidTools {
		validToolSet[tool] = true
	}

	// Validate allowed tools
	for _, tool := range result.AllowedTools {
		if !validToolSet[tool] {
			result.Errors = append(result.Errors, &ToolError{
				Tool:    tool,
				Message: fmt.Sprintf("unknown tool in allowed_tools: %q (valid tools: %s)", tool, strings.Join(ValidTools, ", ")),
			})
		}
	}

	// Validate disallowed tools
	for _, tool := range result.DisallowedTools {
		if !validToolSet[tool] {
			result.Errors = append(result.Errors, &ToolError{
				Tool:    tool,
				Message: fmt.Sprintf("unknown tool in disallowed_tools: %q (valid tools: %s)", tool, strings.Join(ValidTools, ", ")),
			})
		}
	}

	// Check for conflicts (tool in both allowed and disallowed)
	if len(result.AllowedTools) > 0 {
		allowedSet := make(map[string]bool)
		for _, tool := range result.AllowedTools {
			allowedSet[tool] = true
		}

		for _, tool := range result.DisallowedTools {
			if allowedSet[tool] {
				result.Errors = append(result.Errors, &ToolError{
					Tool:    tool,
					Message: fmt.Sprintf("tool %q appears in both allowed and disallowed lists", tool),
				})
			}
		}
	}

	return result
}

// ToolError represents a tool validation error.
type ToolError struct {
	Tool    string
	Message string
}

func (e *ToolError) Error() string {
	return e.Message
}

// HasErrors returns true if the filter result contains validation errors.
func (r *FilterResult) HasErrors() bool {
	return len(r.Errors) > 0
}

// ErrorStrings returns all error messages as strings.
func (r *FilterResult) ErrorStrings() []string {
	msgs := make([]string, len(r.Errors))
	for i, err := range r.Errors {
		msgs[i] = err.Error()
	}
	return msgs
}

// IsValidTool checks if a tool name is valid.
func IsValidTool(tool string) bool {
	for _, valid := range ValidTools {
		if tool == valid {
			return true
		}
	}
	return false
}
