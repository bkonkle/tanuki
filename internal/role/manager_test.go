package role

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestNewManager(t *testing.T) {
	projectRoot := "/tmp/test-project"
	m := NewManager(projectRoot)

	if m.projectRoot != projectRoot {
		t.Errorf("projectRoot = %q, want %q", m.projectRoot, projectRoot)
	}

	expectedRolesDir := filepath.Join(projectRoot, ".tanuki", "roles")
	if m.rolesDir != expectedRolesDir {
		t.Errorf("rolesDir = %q, want %q", m.rolesDir, expectedRolesDir)
	}
}

func TestFileManager_GetBuiltinRoles(t *testing.T) {
	m := NewManager("/tmp/test")
	roles := m.GetBuiltinRoles()

	if len(roles) != 6 {
		t.Errorf("GetBuiltinRoles() returned %d roles, want 6", len(roles))
	}

	// All builtin roles should be valid
	for _, role := range roles {
		if err := role.Validate(); err != nil {
			t.Errorf("Builtin role %q is invalid: %v", role.Name, err)
		}
	}
}

func TestFileManager_Validate(t *testing.T) {
	m := NewManager("/tmp/test")

	validRole := &Role{
		Name:         "test",
		Description:  "Test role",
		SystemPrompt: "You are a test role",
	}

	if err := m.Validate(validRole); err != nil {
		t.Errorf("Validate() unexpected error for valid role: %v", err)
	}

	invalidRole := &Role{
		Description:  "Missing name",
		SystemPrompt: "You are a test role",
	}

	if err := m.Validate(invalidRole); err == nil {
		t.Error("Validate() expected error for invalid role but got nil")
	}
}

func TestFileManager_List_BuiltinOnly(t *testing.T) {
	// Create temp directory for testing
	tmpDir := t.TempDir()
	projectRoot := filepath.Join(tmpDir, "project")

	m := NewManager(projectRoot)

	// No project roles directory exists
	roles, err := m.List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	// Should return only builtin roles
	if len(roles) != 6 {
		t.Errorf("List() returned %d roles, want 6 (builtin only)", len(roles))
	}
}

func TestFileManager_List_WithProjectRoles(t *testing.T) {
	// Create temp directory for testing
	tmpDir := t.TempDir()
	projectRoot := filepath.Join(tmpDir, "project")
	rolesDir := filepath.Join(projectRoot, ".tanuki", "roles")

	// Create roles directory
	if err := os.MkdirAll(rolesDir, 0755); err != nil {
		t.Fatalf("Failed to create roles directory: %v", err)
	}

	// Create a custom role
	customRole := &Role{
		Name:         "custom",
		Description:  "Custom test role",
		SystemPrompt: "You are a custom role",
		AllowedTools: []string{"Read", "Grep"},
	}

	data, err := yaml.Marshal(customRole)
	if err != nil {
		t.Fatalf("Failed to marshal custom role: %v", err)
	}

	rolePath := filepath.Join(rolesDir, "custom.yaml")
	if err := os.WriteFile(rolePath, data, 0644); err != nil {
		t.Fatalf("Failed to write custom role: %v", err)
	}

	// Create manager and list roles
	m := NewManager(projectRoot)
	roles, err := m.List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	// Should return builtin + custom roles
	if len(roles) != 7 {
		t.Errorf("List() returned %d roles, want 7 (6 builtin + 1 custom)", len(roles))
	}

	// Verify custom role is in the list
	foundCustom := false
	for _, role := range roles {
		if role.Name == "custom" {
			foundCustom = true
			if role.Description != "Custom test role" {
				t.Errorf("Custom role description = %q, want %q", role.Description, "Custom test role")
			}
		}
	}

	if !foundCustom {
		t.Error("Custom role not found in list")
	}
}

func TestFileManager_Get_Builtin(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	role, err := m.Get("backend")
	if err != nil {
		t.Fatalf("Get(\"backend\") error: %v", err)
	}

	if role.Name != "backend" {
		t.Errorf("Role name = %q, want %q", role.Name, "backend")
	}

	if role.SystemPrompt == "" {
		t.Error("Backend role system prompt is empty")
	}
}

func TestFileManager_Get_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	_, err := m.Get("nonexistent")
	if err == nil {
		t.Error("Get(\"nonexistent\") expected error but got nil")
	}
}

func TestFileManager_Get_ProjectOverridesBuiltin(t *testing.T) {
	// Create temp directory for testing
	tmpDir := t.TempDir()
	projectRoot := filepath.Join(tmpDir, "project")
	rolesDir := filepath.Join(projectRoot, ".tanuki", "roles")

	// Create roles directory
	if err := os.MkdirAll(rolesDir, 0755); err != nil {
		t.Fatalf("Failed to create roles directory: %v", err)
	}

	// Create a project role that overrides the builtin backend role
	customBackend := &Role{
		Name:         "backend",
		Description:  "Custom backend role",
		SystemPrompt: "Custom backend prompt",
		AllowedTools: []string{"Read"},
	}

	data, err := yaml.Marshal(customBackend)
	if err != nil {
		t.Fatalf("Failed to marshal custom role: %v", err)
	}

	rolePath := filepath.Join(rolesDir, "backend.yaml")
	if err := os.WriteFile(rolePath, data, 0644); err != nil {
		t.Fatalf("Failed to write custom role: %v", err)
	}

	// Get the backend role
	m := NewManager(projectRoot)
	role, err := m.Get("backend")
	if err != nil {
		t.Fatalf("Get(\"backend\") error: %v", err)
	}

	// Should return the custom role, not the builtin
	if role.Description != "Custom backend role" {
		t.Errorf("Role description = %q, want %q", role.Description, "Custom backend role")
	}

	if role.SystemPrompt != "Custom backend prompt" {
		t.Errorf("Role system prompt = %q, want %q", role.SystemPrompt, "Custom backend prompt")
	}

	if role.Builtin {
		t.Error("Role should not be marked as builtin")
	}
}

func TestFileManager_LoadFromFile(t *testing.T) {
	tmpDir := t.TempDir()
	projectRoot := filepath.Join(tmpDir, "project")

	// Create a role file
	role := &Role{
		Name:         "test",
		Description:  "Test role",
		SystemPrompt: "You are a test role",
		AllowedTools: []string{"Read", "Write"},
	}

	data, err := yaml.Marshal(role)
	if err != nil {
		t.Fatalf("Failed to marshal role: %v", err)
	}

	rolePath := filepath.Join(tmpDir, "test.yaml")
	if err := os.WriteFile(rolePath, data, 0644); err != nil {
		t.Fatalf("Failed to write role file: %v", err)
	}

	// Load the role
	m := NewManager(projectRoot)
	loaded, err := m.LoadFromFile(rolePath)
	if err != nil {
		t.Fatalf("LoadFromFile() error: %v", err)
	}

	if loaded.Name != "test" {
		t.Errorf("Loaded role name = %q, want %q", loaded.Name, "test")
	}

	if loaded.Builtin {
		t.Error("Loaded role should not be marked as builtin")
	}
}

func TestFileManager_LoadFromFile_WithSystemPromptFile(t *testing.T) {
	tmpDir := t.TempDir()
	projectRoot := filepath.Join(tmpDir, "project")

	// Create a system prompt file
	promptContent := "This is a custom system prompt from a file."
	promptPath := filepath.Join(projectRoot, "prompts", "custom.md")
	if err := os.MkdirAll(filepath.Dir(promptPath), 0755); err != nil {
		t.Fatalf("Failed to create prompts directory: %v", err)
	}
	if err := os.WriteFile(promptPath, []byte(promptContent), 0644); err != nil {
		t.Fatalf("Failed to write prompt file: %v", err)
	}

	// Create a role file with system_prompt_file
	role := &Role{
		Name:             "test",
		Description:      "Test role",
		SystemPromptFile: "prompts/custom.md",
		AllowedTools:     []string{"Read"},
	}

	data, err := yaml.Marshal(role)
	if err != nil {
		t.Fatalf("Failed to marshal role: %v", err)
	}

	rolePath := filepath.Join(tmpDir, "test.yaml")
	if err := os.WriteFile(rolePath, data, 0644); err != nil {
		t.Fatalf("Failed to write role file: %v", err)
	}

	// Load the role
	m := NewManager(projectRoot)
	loaded, err := m.LoadFromFile(rolePath)
	if err != nil {
		t.Fatalf("LoadFromFile() error: %v", err)
	}

	// SystemPrompt should be populated from the file
	if loaded.SystemPrompt != promptContent {
		t.Errorf("SystemPrompt = %q, want %q", loaded.SystemPrompt, promptContent)
	}
}

func TestFileManager_LoadFromFile_Invalid(t *testing.T) {
	tmpDir := t.TempDir()
	projectRoot := filepath.Join(tmpDir, "project")

	// Create an invalid role file (missing required fields)
	invalidRole := &Role{
		Name: "invalid",
		// Missing Description and SystemPrompt
	}

	data, err := yaml.Marshal(invalidRole)
	if err != nil {
		t.Fatalf("Failed to marshal role: %v", err)
	}

	rolePath := filepath.Join(tmpDir, "invalid.yaml")
	if err := os.WriteFile(rolePath, data, 0644); err != nil {
		t.Fatalf("Failed to write role file: %v", err)
	}

	// Load should fail validation
	m := NewManager(projectRoot)
	_, err = m.LoadFromFile(rolePath)
	if err == nil {
		t.Error("LoadFromFile() expected error for invalid role but got nil")
	}
}

func TestFileManager_InitRoles(t *testing.T) {
	tmpDir := t.TempDir()
	projectRoot := filepath.Join(tmpDir, "project")
	rolesDir := filepath.Join(projectRoot, ".tanuki", "roles")

	m := NewManager(projectRoot)

	// Initialize roles
	if err := m.InitRoles(); err != nil {
		t.Fatalf("InitRoles() error: %v", err)
	}

	// Check that directory was created
	if _, err := os.Stat(rolesDir); err != nil {
		t.Errorf("Roles directory was not created: %v", err)
	}

	// Check that builtin role files were created
	builtinRoles := []string{"backend", "frontend", "qa", "docs", "devops", "fullstack"}
	for _, name := range builtinRoles {
		rolePath := filepath.Join(rolesDir, name+".yaml")
		if _, err := os.Stat(rolePath); err != nil {
			t.Errorf("Builtin role file %q was not created: %v", name, err)
		}

		// Verify file can be loaded and is valid
		role, err := m.LoadFromFile(rolePath)
		if err != nil {
			t.Errorf("Failed to load initialized role %q: %v", name, err)
		}
		if role.Name != name {
			t.Errorf("Role name = %q, want %q", role.Name, name)
		}
	}
}

func TestFileManager_InitRoles_DoesNotOverwrite(t *testing.T) {
	tmpDir := t.TempDir()
	projectRoot := filepath.Join(tmpDir, "project")
	rolesDir := filepath.Join(projectRoot, ".tanuki", "roles")

	// Create roles directory
	if err := os.MkdirAll(rolesDir, 0755); err != nil {
		t.Fatalf("Failed to create roles directory: %v", err)
	}

	// Create a custom backend role
	customContent := []byte("name: backend\ndescription: Custom\nsystem_prompt: Custom prompt\n")
	backendPath := filepath.Join(rolesDir, "backend.yaml")
	if err := os.WriteFile(backendPath, customContent, 0644); err != nil {
		t.Fatalf("Failed to write custom backend role: %v", err)
	}

	m := NewManager(projectRoot)

	// Initialize roles
	if err := m.InitRoles(); err != nil {
		t.Fatalf("InitRoles() error: %v", err)
	}

	// Read the backend role file
	content, err := os.ReadFile(backendPath)
	if err != nil {
		t.Fatalf("Failed to read backend role file: %v", err)
	}

	// Should still have custom content
	if string(content) != string(customContent) {
		t.Error("InitRoles() overwrote existing role file")
	}
}
