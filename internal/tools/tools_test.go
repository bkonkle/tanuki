package tools

import (
	"reflect"
	"strings"
	"testing"

	"github.com/bkonkle/tanuki/internal/config"
)

func TestFilterTools(t *testing.T) {
	tests := []struct {
		name             string
		ws               *config.WorkstreamConfig
		opts             FilterOptions
		wantAllowed      []string
		wantDisallowed   []string
		wantErrors       bool
		wantErrorCount   int
		wantErrorMessage string
	}{
		{
			name:           "no restrictions",
			ws:             nil,
			opts:           FilterOptions{},
			wantAllowed:    nil,
			wantDisallowed: []string{}, // Empty slice, not nil
			wantErrors:     false,
		},
		{
			name: "use workstream allowed tools",
			ws: &config.WorkstreamConfig{
				AllowedTools: []string{"Read", "Grep", "Bash"},
			},
			opts:           FilterOptions{},
			wantAllowed:    []string{"Bash", "Grep", "Read"}, // Sorted
			wantDisallowed: []string{},                       // Empty slice, not nil
			wantErrors:     false,
		},
		{
			name: "use workstream disallowed tools",
			ws: &config.WorkstreamConfig{
				DisallowedTools: []string{"Write", "Edit"},
			},
			opts:           FilterOptions{},
			wantAllowed:    nil,
			wantDisallowed: []string{"Edit", "Write"}, // Sorted
			wantErrors:     false,
		},
		{
			name: "use workstream allowed and disallowed",
			ws: &config.WorkstreamConfig{
				AllowedTools:    []string{"Read", "Grep"},
				DisallowedTools: []string{"Bash"},
			},
			opts:           FilterOptions{},
			wantAllowed:    []string{"Grep", "Read"},
			wantDisallowed: []string{"Bash"},
			wantErrors:     false,
		},
		{
			name: "CLI override allowed tools",
			ws: &config.WorkstreamConfig{
				AllowedTools: []string{"Read"},
			},
			opts: FilterOptions{
				AllowedTools: []string{"Read", "Write", "Edit"},
			},
			wantAllowed:    []string{"Edit", "Read", "Write"},
			wantDisallowed: []string{}, // Empty slice, not nil
			wantErrors:     false,
		},
		{
			name: "additive disallowed tools",
			ws: &config.WorkstreamConfig{
				DisallowedTools: []string{"Write"},
			},
			opts: FilterOptions{
				DisallowedTools: []string{"Bash", "Edit"},
			},
			wantAllowed:    nil,
			wantDisallowed: []string{"Bash", "Edit", "Write"},
			wantErrors:     false,
		},
		{
			name: "deduplicate disallowed tools",
			ws: &config.WorkstreamConfig{
				DisallowedTools: []string{"Write", "Bash"},
			},
			opts: FilterOptions{
				DisallowedTools: []string{"Bash", "Edit"},
			},
			wantAllowed:    nil,
			wantDisallowed: []string{"Bash", "Edit", "Write"},
			wantErrors:     false,
		},
		{
			name: "invalid tool in allowed list",
			ws: &config.WorkstreamConfig{
				AllowedTools: []string{"Read", "InvalidTool"},
			},
			opts:             FilterOptions{},
			wantAllowed:      []string{"InvalidTool", "Read"},
			wantDisallowed:   []string{}, // Empty slice, not nil
			wantErrors:       true,
			wantErrorCount:   1,
			wantErrorMessage: "unknown tool in allowed_tools",
		},
		{
			name: "invalid tool in disallowed list",
			ws: &config.WorkstreamConfig{
				DisallowedTools: []string{"BadTool"},
			},
			opts:             FilterOptions{},
			wantAllowed:      nil,
			wantDisallowed:   []string{"BadTool"},
			wantErrors:       true,
			wantErrorCount:   1,
			wantErrorMessage: "unknown tool in disallowed_tools",
		},
		{
			name: "tool in both allowed and disallowed",
			ws: &config.WorkstreamConfig{
				AllowedTools:    []string{"Read", "Write"},
				DisallowedTools: []string{"Write"},
			},
			opts:             FilterOptions{},
			wantAllowed:      []string{"Read", "Write"},
			wantDisallowed:   []string{"Write"},
			wantErrors:       true,
			wantErrorCount:   1,
			wantErrorMessage: "appears in both allowed and disallowed",
		},
		{
			name: "multiple errors",
			ws: &config.WorkstreamConfig{
				AllowedTools:    []string{"Read", "BadTool1"},
				DisallowedTools: []string{"BadTool2"},
			},
			opts:           FilterOptions{},
			wantAllowed:    []string{"BadTool1", "Read"},
			wantDisallowed: []string{"BadTool2"},
			wantErrors:     true,
			wantErrorCount: 2,
		},
		{
			name: "CLI override replaces workstream allowed",
			ws: &config.WorkstreamConfig{
				AllowedTools: []string{"Read", "Grep"},
			},
			opts: FilterOptions{
				AllowedTools: []string{"Bash", "Write"},
			},
			wantAllowed:    []string{"Bash", "Write"},
			wantDisallowed: []string{}, // Empty slice, not nil
			wantErrors:     false,
		},
		{
			name: "CLI disallowed adds to workstream disallowed",
			ws: &config.WorkstreamConfig{
				DisallowedTools: []string{"Write"},
			},
			opts: FilterOptions{
				DisallowedTools: []string{"Edit"},
			},
			wantAllowed:    nil,
			wantDisallowed: []string{"Edit", "Write"},
			wantErrors:     false,
		},
		{
			name: "CLI override with CLI disallowed",
			ws: &config.WorkstreamConfig{
				AllowedTools:    []string{"Read"},
				DisallowedTools: []string{"Write"},
			},
			opts: FilterOptions{
				AllowedTools:    []string{"Read", "Write", "Edit"},
				DisallowedTools: []string{"Bash"},
			},
			wantAllowed:      []string{"Edit", "Read", "Write"},
			wantDisallowed:   []string{"Bash", "Write"},
			wantErrors:       true, // Write appears in both lists
			wantErrorCount:   1,
			wantErrorMessage: "appears in both allowed and disallowed",
		},
		{
			name: "empty workstream with CLI options",
			ws:   &config.WorkstreamConfig{},
			opts: FilterOptions{
				AllowedTools:    []string{"Read", "Write"},
				DisallowedTools: []string{"Bash"},
			},
			wantAllowed:    []string{"Read", "Write"},
			wantDisallowed: []string{"Bash"},
			wantErrors:     false,
		},
		{
			name: "nil workstream with CLI options",
			ws:   nil,
			opts: FilterOptions{
				AllowedTools:    []string{"Read"},
				DisallowedTools: []string{"Write"},
			},
			wantAllowed:    []string{"Read"},
			wantDisallowed: []string{"Write"},
			wantErrors:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FilterTools(tt.ws, tt.opts)

			// Check allowed tools
			if !reflect.DeepEqual(result.AllowedTools, tt.wantAllowed) {
				t.Errorf("AllowedTools = %v, want %v", result.AllowedTools, tt.wantAllowed)
			}

			// Check disallowed tools
			if !reflect.DeepEqual(result.DisallowedTools, tt.wantDisallowed) {
				t.Errorf("DisallowedTools = %v, want %v", result.DisallowedTools, tt.wantDisallowed)
			}

			// Check errors
			if result.HasErrors() != tt.wantErrors {
				t.Errorf("HasErrors() = %v, want %v", result.HasErrors(), tt.wantErrors)
			}

			if tt.wantErrors {
				if tt.wantErrorCount > 0 && len(result.Errors) != tt.wantErrorCount {
					t.Errorf("Error count = %d, want %d", len(result.Errors), tt.wantErrorCount)
				}

				if tt.wantErrorMessage != "" {
					found := false
					for _, err := range result.Errors {
						if strings.Contains(err.Error(), tt.wantErrorMessage) {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Expected error message containing %q, got errors: %v", tt.wantErrorMessage, result.ErrorStrings())
					}
				}
			}
		})
	}
}

func TestFilterResult_HasErrors(t *testing.T) {
	tests := []struct {
		name   string
		result *FilterResult
		want   bool
	}{
		{
			name: "no errors",
			result: &FilterResult{
				Errors: []error{},
			},
			want: false,
		},
		{
			name: "has errors",
			result: &FilterResult{
				Errors: []error{
					&ToolError{Tool: "Bad", Message: "test error"},
				},
			},
			want: true,
		},
		{
			name: "nil errors slice",
			result: &FilterResult{
				Errors: nil,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.HasErrors(); got != tt.want {
				t.Errorf("HasErrors() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilterResult_ErrorStrings(t *testing.T) {
	result := &FilterResult{
		Errors: []error{
			&ToolError{Tool: "Bad1", Message: "error 1"},
			&ToolError{Tool: "Bad2", Message: "error 2"},
		},
	}

	strs := result.ErrorStrings()
	if len(strs) != 2 {
		t.Errorf("ErrorStrings() length = %d, want 2", len(strs))
	}

	if strs[0] != "error 1" {
		t.Errorf("ErrorStrings()[0] = %q, want %q", strs[0], "error 1")
	}
	if strs[1] != "error 2" {
		t.Errorf("ErrorStrings()[1] = %q, want %q", strs[1], "error 2")
	}
}

func TestToolError_Error(t *testing.T) {
	err := &ToolError{
		Tool:    "BadTool",
		Message: "test error message",
	}

	if err.Error() != "test error message" {
		t.Errorf("Error() = %q, want %q", err.Error(), "test error message")
	}
}

func TestValidTools(t *testing.T) {
	// Verify ValidTools contains expected tools
	expectedTools := map[string]bool{
		"Read":      true,
		"Write":     true,
		"Edit":      true,
		"Bash":      true,
		"Glob":      true,
		"Grep":      true,
		"TodoWrite": true,
		"Task":      true,
		"WebFetch":  true,
		"WebSearch": true,
	}

	if len(ValidTools) != len(expectedTools) {
		t.Errorf("ValidTools length = %d, want %d", len(ValidTools), len(expectedTools))
	}

	for _, tool := range ValidTools {
		if !expectedTools[tool] {
			t.Errorf("ValidTools contains unexpected tool: %q", tool)
		}
	}

	for tool := range expectedTools {
		if !IsValidTool(tool) {
			t.Errorf("IsValidTool(%q) = false, want true", tool)
		}
	}
}

func TestIsValidTool(t *testing.T) {
	tests := []struct {
		tool string
		want bool
	}{
		{"Read", true},
		{"Write", true},
		{"InvalidTool", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.tool, func(t *testing.T) {
			if got := IsValidTool(tt.tool); got != tt.want {
				t.Errorf("IsValidTool(%q) = %v, want %v", tt.tool, got, tt.want)
			}
		})
	}
}
