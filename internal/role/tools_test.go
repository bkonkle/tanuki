package role

import (
	"reflect"
	"testing"
)

func TestFilterTools(t *testing.T) {
	tests := []struct {
		name             string
		role             *Role
		opts             ToolFilterOptions
		wantAllowed      []string
		wantDisallowed   []string
		wantErrors       bool
		wantErrorCount   int
		wantErrorMessage string
	}{
		{
			name: "no restrictions",
			role: nil,
			opts: ToolFilterOptions{},
			wantAllowed:    nil,
			wantDisallowed: []string{}, // Empty slice, not nil
			wantErrors:     false,
		},
		{
			name: "use role allowed tools",
			role: &Role{
				AllowedTools: []string{"Read", "Grep", "Bash"},
			},
			opts:           ToolFilterOptions{},
			wantAllowed:    []string{"Bash", "Grep", "Read"}, // Sorted
			wantDisallowed: []string{}, // Empty slice, not nil
			wantErrors:     false,
		},
		{
			name: "use role disallowed tools",
			role: &Role{
				DisallowedTools: []string{"Write", "Edit"},
			},
			opts:           ToolFilterOptions{},
			wantAllowed:    nil,
			wantDisallowed: []string{"Edit", "Write"}, // Sorted
			wantErrors:     false,
		},
		{
			name: "use role allowed and disallowed",
			role: &Role{
				AllowedTools:    []string{"Read", "Grep"},
				DisallowedTools: []string{"Bash"},
			},
			opts:           ToolFilterOptions{},
			wantAllowed:    []string{"Grep", "Read"},
			wantDisallowed: []string{"Bash"},
			wantErrors:     false,
		},
		{
			name: "CLI override allowed tools",
			role: &Role{
				AllowedTools: []string{"Read"},
			},
			opts: ToolFilterOptions{
				AllowedTools: []string{"Read", "Write", "Edit"},
			},
			wantAllowed:    []string{"Edit", "Read", "Write"},
			wantDisallowed: []string{}, // Empty slice, not nil
			wantErrors:     false,
		},
		{
			name: "additive disallowed tools",
			role: &Role{
				DisallowedTools: []string{"Write"},
			},
			opts: ToolFilterOptions{
				DisallowedTools: []string{"Bash", "Edit"},
			},
			wantAllowed:    nil,
			wantDisallowed: []string{"Bash", "Edit", "Write"},
			wantErrors:     false,
		},
		{
			name: "deduplicate disallowed tools",
			role: &Role{
				DisallowedTools: []string{"Write", "Bash"},
			},
			opts: ToolFilterOptions{
				DisallowedTools: []string{"Bash", "Edit"},
			},
			wantAllowed:    nil,
			wantDisallowed: []string{"Bash", "Edit", "Write"},
			wantErrors:     false,
		},
		{
			name: "invalid tool in allowed list",
			role: &Role{
				AllowedTools: []string{"Read", "InvalidTool"},
			},
			opts:            ToolFilterOptions{},
			wantAllowed:     []string{"InvalidTool", "Read"},
			wantDisallowed:  []string{}, // Empty slice, not nil
			wantErrors:      true,
			wantErrorCount:  1,
			wantErrorMessage: "unknown tool in allowed_tools",
		},
		{
			name: "invalid tool in disallowed list",
			role: &Role{
				DisallowedTools: []string{"BadTool"},
			},
			opts:            ToolFilterOptions{},
			wantAllowed:     nil,
			wantDisallowed:  []string{"BadTool"},
			wantErrors:      true,
			wantErrorCount:  1,
			wantErrorMessage: "unknown tool in disallowed_tools",
		},
		{
			name: "tool in both allowed and disallowed",
			role: &Role{
				AllowedTools:    []string{"Read", "Write"},
				DisallowedTools: []string{"Write"},
			},
			opts:            ToolFilterOptions{},
			wantAllowed:     []string{"Read", "Write"},
			wantDisallowed:  []string{"Write"},
			wantErrors:      true,
			wantErrorCount:  1,
			wantErrorMessage: "appears in both allowed and disallowed",
		},
		{
			name: "multiple errors",
			role: &Role{
				AllowedTools:    []string{"Read", "BadTool1"},
				DisallowedTools: []string{"BadTool2"},
			},
			opts:           ToolFilterOptions{},
			wantAllowed:    []string{"BadTool1", "Read"},
			wantDisallowed: []string{"BadTool2"},
			wantErrors:     true,
			wantErrorCount: 2,
		},
		{
			name: "CLI override replaces role allowed",
			role: &Role{
				AllowedTools: []string{"Read", "Grep"},
			},
			opts: ToolFilterOptions{
				AllowedTools: []string{"Bash", "Write"},
			},
			wantAllowed:    []string{"Bash", "Write"},
			wantDisallowed: []string{}, // Empty slice, not nil
			wantErrors:     false,
		},
		{
			name: "CLI disallowed adds to role disallowed",
			role: &Role{
				DisallowedTools: []string{"Write"},
			},
			opts: ToolFilterOptions{
				DisallowedTools: []string{"Edit"},
			},
			wantAllowed:    nil,
			wantDisallowed: []string{"Edit", "Write"},
			wantErrors:     false,
		},
		{
			name: "CLI override with CLI disallowed",
			role: &Role{
				AllowedTools:    []string{"Read"},
				DisallowedTools: []string{"Write"},
			},
			opts: ToolFilterOptions{
				AllowedTools:    []string{"Read", "Write", "Edit"},
				DisallowedTools: []string{"Bash"},
			},
			wantAllowed:    []string{"Edit", "Read", "Write"},
			wantDisallowed: []string{"Bash", "Write"},
			wantErrors:     true, // Write appears in both lists
			wantErrorCount: 1,
			wantErrorMessage: "appears in both allowed and disallowed",
		},
		{
			name: "empty role with CLI options",
			role: &Role{},
			opts: ToolFilterOptions{
				AllowedTools:    []string{"Read", "Write"},
				DisallowedTools: []string{"Bash"},
			},
			wantAllowed:    []string{"Read", "Write"},
			wantDisallowed: []string{"Bash"},
			wantErrors:     false,
		},
		{
			name:           "nil role with CLI options",
			role:           nil,
			opts: ToolFilterOptions{
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
			result := FilterTools(tt.role, tt.opts)

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
						if contains(err.Error(), tt.wantErrorMessage) {
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
		found := false
		for _, vt := range ValidTools {
			if vt == tool {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("ValidTools missing expected tool: %q", tool)
		}
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
