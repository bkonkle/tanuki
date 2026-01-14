package tui

import (
	"testing"
)

func TestAgentStatusIcon(t *testing.T) {
	tests := []struct {
		status   string
		expected string // We just check it's non-empty since the actual icon depends on styling
	}{
		{"idle", "●"},
		{"working", "◐"},
		{"stopped", "○"},
		{"error", "✗"},
		{"unknown", "?"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			icon := AgentStatusIcon(tt.status)
			if icon == "" {
				t.Errorf("expected non-empty icon for status %s", tt.status)
			}
		})
	}
}

func TestAgentStatusColor(t *testing.T) {
	tests := []struct {
		status string
	}{
		{"idle"},
		{"working"},
		{"stopped"},
		{"error"},
		{"unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			color := AgentStatusColor(tt.status)
			// Just verify it returns a color (non-empty)
			if color == "" {
				t.Errorf("expected non-empty color for status %s", tt.status)
			}
		})
	}
}

func TestTaskStatusIcon(t *testing.T) {
	tests := []struct {
		status string
	}{
		{"pending"},
		{"assigned"},
		{"in_progress"},
		{"review"},
		{"complete"},
		{"failed"},
		{"blocked"},
		{"unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			icon := TaskStatusIcon(tt.status)
			if icon == "" {
				t.Errorf("expected non-empty icon for status %s", tt.status)
			}
		})
	}
}

func TestTaskStatusColor(t *testing.T) {
	tests := []struct {
		status string
	}{
		{"pending"},
		{"assigned"},
		{"in_progress"},
		{"review"},
		{"complete"},
		{"failed"},
		{"blocked"},
		{"unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			color := TaskStatusColor(tt.status)
			if color == "" {
				t.Errorf("expected non-empty color for status %s", tt.status)
			}
		})
	}
}

func TestLogLevelColor(t *testing.T) {
	tests := []struct {
		level string
	}{
		{"error"},
		{"warn"},
		{"info"},
		{"unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			color := LogLevelColor(tt.level)
			if color == "" {
				t.Errorf("expected non-empty color for level %s", tt.level)
			}
		})
	}
}

func TestPaneBorder(t *testing.T) {
	// Test focused border returns a style
	focusedBorder := PaneBorder(true)
	// Just verify it returns a non-zero style
	rendered := focusedBorder.Render("test")
	if rendered == "" {
		t.Error("expected focused border to render content")
	}

	// Test unfocused border returns a style
	unfocusedBorder := PaneBorder(false)
	rendered = unfocusedBorder.Render("test")
	if rendered == "" {
		t.Error("expected unfocused border to render content")
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		max      int
		expected string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "he..."},
		{"hi", 10, "hi"},
		{"test", 4, "test"},
		{"test", 3, "tes"},
		{"hello", 3, "hel"},
		{"hello world this is long", 10, "hello w..."},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := Truncate(tt.input, tt.max)
			if result != tt.expected {
				t.Errorf("Truncate(%q, %d) = %q, expected %q", tt.input, tt.max, result, tt.expected)
			}
		})
	}
}

func TestStylesExist(t *testing.T) {
	// Verify all style variables are defined and non-nil
	styles := []struct {
		name  string
		value interface{}
	}{
		{"HeaderStyle", HeaderStyle},
		{"SelectedStyle", SelectedStyle},
		{"MutedStyle", MutedStyle},
		{"SuccessStyle", SuccessStyle},
		{"WarningStyle", WarningStyle},
		{"ErrorStyle", ErrorStyle},
		{"InfoStyle", InfoStyle},
		{"HelpStyle", HelpStyle},
		{"TitleStyle", TitleStyle},
		{"StatusBarStyle", StatusBarStyle},
	}

	for _, s := range styles {
		t.Run(s.name, func(t *testing.T) {
			if s.value == nil {
				t.Errorf("%s should not be nil", s.name)
			}
		})
	}
}

func TestColorsExist(t *testing.T) {
	// Verify all color variables are defined
	colors := []struct {
		name  string
		value interface{}
	}{
		{"ColorPrimary", ColorPrimary},
		{"ColorSecondary", ColorSecondary},
		{"ColorSuccess", ColorSuccess},
		{"ColorWarning", ColorWarning},
		{"ColorError", ColorError},
		{"ColorInfo", ColorInfo},
		{"ColorMuted", ColorMuted},
		{"ColorHighlight", ColorHighlight},
		{"ColorOrange", ColorOrange},
	}

	for _, c := range colors {
		t.Run(c.name, func(t *testing.T) {
			if c.value == nil {
				t.Errorf("%s should not be nil", c.name)
			}
		})
	}
}
