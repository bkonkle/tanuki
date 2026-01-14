package role

import (
	"testing"
)

func TestBuiltinRoles(t *testing.T) {
	roles := BuiltinRoles()

	if len(roles) != 6 {
		t.Errorf("BuiltinRoles() returned %d roles, want 6", len(roles))
	}

	// Check that all roles are valid
	roleNames := make(map[string]bool)
	for _, role := range roles {
		if err := role.Validate(); err != nil {
			t.Errorf("Builtin role %q failed validation: %v", role.Name, err)
		}

		// Check for duplicates
		if roleNames[role.Name] {
			t.Errorf("Duplicate role name: %q", role.Name)
		}
		roleNames[role.Name] = true

		// Verify builtin flag is set
		if !role.Builtin {
			t.Errorf("Builtin role %q should have Builtin=true", role.Name)
		}
	}

	// Verify expected roles exist
	expectedRoles := []string{"backend", "frontend", "qa", "docs", "devops", "fullstack"}
	for _, expected := range expectedRoles {
		if !roleNames[expected] {
			t.Errorf("Expected builtin role %q not found", expected)
		}
	}
}

func TestBackendRole(t *testing.T) {
	role, err := GetBuiltinRole("backend")
	if err != nil {
		t.Fatalf("GetBuiltinRole(\"backend\") error: %v", err)
	}

	if role.Name != "backend" {
		t.Errorf("Name = %q, want %q", role.Name, "backend")
	}

	if role.Description == "" {
		t.Error("Description is empty")
	}

	if role.SystemPrompt == "" {
		t.Error("SystemPrompt is empty")
	}

	if len(role.AllowedTools) == 0 {
		t.Error("AllowedTools is empty")
	}

	// Verify backend role includes essential tools
	hasRead := false
	hasWrite := false
	hasEdit := false
	for _, tool := range role.AllowedTools {
		switch tool {
		case "Read":
			hasRead = true
		case "Write":
			hasWrite = true
		case "Edit":
			hasEdit = true
		}
	}

	if !hasRead {
		t.Error("Backend role missing Read tool")
	}
	if !hasWrite {
		t.Error("Backend role missing Write tool")
	}
	if !hasEdit {
		t.Error("Backend role missing Edit tool")
	}

	if err := role.Validate(); err != nil {
		t.Errorf("Validate() error: %v", err)
	}
}

func TestFrontendRole(t *testing.T) {
	role, err := GetBuiltinRole("frontend")
	if err != nil {
		t.Fatalf("GetBuiltinRole(\"frontend\") error: %v", err)
	}

	if role.Name != "frontend" {
		t.Errorf("Name = %q, want %q", role.Name, "frontend")
	}

	if role.Description == "" {
		t.Error("Description is empty")
	}

	if role.SystemPrompt == "" {
		t.Error("SystemPrompt is empty")
	}

	if len(role.AllowedTools) == 0 {
		t.Error("AllowedTools is empty")
	}

	if err := role.Validate(); err != nil {
		t.Errorf("Validate() error: %v", err)
	}
}

func TestQARole(t *testing.T) {
	role, err := GetBuiltinRole("qa")
	if err != nil {
		t.Fatalf("GetBuiltinRole(\"qa\") error: %v", err)
	}

	if role.Name != "qa" {
		t.Errorf("Name = %q, want %q", role.Name, "qa")
	}

	if role.Description == "" {
		t.Error("Description is empty")
	}

	if role.SystemPrompt == "" {
		t.Error("SystemPrompt is empty")
	}

	if len(role.AllowedTools) == 0 {
		t.Error("AllowedTools is empty")
	}

	// QA role should NOT have Write or Edit tools
	for _, tool := range role.AllowedTools {
		if tool == "Write" || tool == "Edit" {
			t.Errorf("QA role should not have %q tool", tool)
		}
	}

	// QA role should explicitly disallow Write and Edit
	hasWriteDisallow := false
	hasEditDisallow := false
	for _, tool := range role.DisallowedTools {
		if tool == "Write" {
			hasWriteDisallow = true
		}
		if tool == "Edit" {
			hasEditDisallow = true
		}
	}

	if !hasWriteDisallow {
		t.Error("QA role should explicitly disallow Write tool")
	}
	if !hasEditDisallow {
		t.Error("QA role should explicitly disallow Edit tool")
	}

	if err := role.Validate(); err != nil {
		t.Errorf("Validate() error: %v", err)
	}
}

func TestDocsRole(t *testing.T) {
	role, err := GetBuiltinRole("docs")
	if err != nil {
		t.Fatalf("GetBuiltinRole(\"docs\") error: %v", err)
	}

	if role.Name != "docs" {
		t.Errorf("Name = %q, want %q", role.Name, "docs")
	}

	if role.Description == "" {
		t.Error("Description is empty")
	}

	if role.SystemPrompt == "" {
		t.Error("SystemPrompt is empty")
	}

	if len(role.AllowedTools) == 0 {
		t.Error("AllowedTools is empty")
	}

	// Docs role should have Write and Edit tools
	hasWrite := false
	hasEdit := false
	for _, tool := range role.AllowedTools {
		if tool == "Write" {
			hasWrite = true
		}
		if tool == "Edit" {
			hasEdit = true
		}
	}

	if !hasWrite {
		t.Error("Docs role should have Write tool")
	}
	if !hasEdit {
		t.Error("Docs role should have Edit tool")
	}

	if err := role.Validate(); err != nil {
		t.Errorf("Validate() error: %v", err)
	}
}

func TestDevOpsRole(t *testing.T) {
	role, err := GetBuiltinRole("devops")
	if err != nil {
		t.Fatalf("GetBuiltinRole(\"devops\") error: %v", err)
	}

	if role.Name != "devops" {
		t.Errorf("Name = %q, want %q", role.Name, "devops")
	}

	if role.Description == "" {
		t.Error("Description is empty")
	}

	if role.SystemPrompt == "" {
		t.Error("SystemPrompt is empty")
	}

	if len(role.AllowedTools) == 0 {
		t.Error("AllowedTools is empty")
	}

	// DevOps role should have full access
	hasRead := false
	hasWrite := false
	hasEdit := false
	hasBash := false
	for _, tool := range role.AllowedTools {
		switch tool {
		case "Read":
			hasRead = true
		case "Write":
			hasWrite = true
		case "Edit":
			hasEdit = true
		case "Bash":
			hasBash = true
		}
	}

	if !hasRead {
		t.Error("DevOps role missing Read tool")
	}
	if !hasWrite {
		t.Error("DevOps role missing Write tool")
	}
	if !hasEdit {
		t.Error("DevOps role missing Edit tool")
	}
	if !hasBash {
		t.Error("DevOps role missing Bash tool")
	}

	if err := role.Validate(); err != nil {
		t.Errorf("Validate() error: %v", err)
	}
}

func TestFullstackRole(t *testing.T) {
	role, err := GetBuiltinRole("fullstack")
	if err != nil {
		t.Fatalf("GetBuiltinRole(\"fullstack\") error: %v", err)
	}

	if role.Name != "fullstack" {
		t.Errorf("Name = %q, want %q", role.Name, "fullstack")
	}

	if role.Description == "" {
		t.Error("Description is empty")
	}

	if role.SystemPrompt == "" {
		t.Error("SystemPrompt is empty")
	}

	if len(role.AllowedTools) == 0 {
		t.Error("AllowedTools is empty")
	}

	// Fullstack role should have full access
	hasRead := false
	hasWrite := false
	hasEdit := false
	for _, tool := range role.AllowedTools {
		switch tool {
		case "Read":
			hasRead = true
		case "Write":
			hasWrite = true
		case "Edit":
			hasEdit = true
		}
	}

	if !hasRead {
		t.Error("Fullstack role missing Read tool")
	}
	if !hasWrite {
		t.Error("Fullstack role missing Write tool")
	}
	if !hasEdit {
		t.Error("Fullstack role missing Edit tool")
	}

	if err := role.Validate(); err != nil {
		t.Errorf("Validate() error: %v", err)
	}
}

func TestGetBuiltinRole(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"backend", false},
		{"frontend", false},
		{"qa", false},
		{"docs", false},
		{"devops", false},
		{"fullstack", false},
		{"nonexistent", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			role, err := GetBuiltinRole(tt.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetBuiltinRole(%q) error = %v, wantErr %v", tt.name, err, tt.wantErr)
				return
			}
			if !tt.wantErr && role == nil {
				t.Errorf("GetBuiltinRole(%q) returned nil role", tt.name)
			}
		})
	}
}

func TestListBuiltinRoleNames(t *testing.T) {
	names := ListBuiltinRoleNames()

	expected := []string{"backend", "frontend", "qa", "docs", "devops", "fullstack"}
	if len(names) != len(expected) {
		t.Errorf("ListBuiltinRoleNames() returned %d names, want %d", len(names), len(expected))
	}

	for _, name := range expected {
		found := false
		for _, n := range names {
			if n == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("ListBuiltinRoleNames() missing expected role: %s", name)
		}
	}
}
