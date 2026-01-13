package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Version != "1" {
		t.Errorf("expected version '1', got '%s'", cfg.Version)
	}

	if cfg.Image.Name != "bkonkle/tanuki" {
		t.Errorf("expected image name 'bkonkle/tanuki', got '%s'", cfg.Image.Name)
	}

	if cfg.Image.Tag != "latest" {
		t.Errorf("expected image tag 'latest', got '%s'", cfg.Image.Tag)
	}

	if cfg.Defaults.MaxTurns != 50 {
		t.Errorf("expected max_turns 50, got %d", cfg.Defaults.MaxTurns)
	}

	if len(cfg.Defaults.AllowedTools) != 6 {
		t.Errorf("expected 6 allowed tools, got %d", len(cfg.Defaults.AllowedTools))
	}

	if cfg.Git.BranchPrefix != "tanuki/" {
		t.Errorf("expected branch_prefix 'tanuki/', got '%s'", cfg.Git.BranchPrefix)
	}

	if cfg.Network.Name != "tanuki-net" {
		t.Errorf("expected network name 'tanuki-net', got '%s'", cfg.Network.Name)
	}
}

func TestWriteAndLoad(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "tanuki-config-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Write default config
	configPath := filepath.Join(tmpDir, "tanuki.yaml")
	if err := WriteDefault(configPath); err != nil {
		t.Fatalf("failed to write default config: %v", err)
	}

	// Verify file exists
	if !Exists(configPath) {
		t.Fatal("config file should exist after writing")
	}

	// Load the config
	loader := NewLoader()
	cfg, err := loader.LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Verify loaded values match defaults
	defaults := DefaultConfig()
	if cfg.Version != defaults.Version {
		t.Errorf("loaded version '%s' != default '%s'", cfg.Version, defaults.Version)
	}

	if cfg.Image.Name != defaults.Image.Name {
		t.Errorf("loaded image name '%s' != default '%s'", cfg.Image.Name, defaults.Image.Name)
	}
}

func TestLoadWithOverrides(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "tanuki-config-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Write default config
	configPath := filepath.Join(tmpDir, "tanuki.yaml")
	if err := WriteDefault(configPath); err != nil {
		t.Fatalf("failed to write default config: %v", err)
	}

	// Load with overrides
	loader := NewLoader()
	loader.SetOverride("defaults.max_turns", 100)
	loader.SetOverride("git.auto_push", true)

	cfg, err := loader.LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Verify overrides applied
	if cfg.Defaults.MaxTurns != 100 {
		t.Errorf("expected max_turns override to be 100, got %d", cfg.Defaults.MaxTurns)
	}

	if !cfg.Git.AutoPush {
		t.Error("expected auto_push override to be true")
	}
}

func TestValidation(t *testing.T) {
	tests := []struct {
		name        string
		modify      func(*Config)
		expectError bool
		errorField  string
	}{
		{
			name:        "valid config",
			modify:      func(c *Config) {},
			expectError: false,
		},
		{
			name: "invalid version",
			modify: func(c *Config) {
				c.Version = "2"
			},
			expectError: true,
			errorField:  "Version",
		},
		{
			name: "max_turns too low",
			modify: func(c *Config) {
				c.Defaults.MaxTurns = 0
			},
			expectError: true,
			errorField:  "MaxTurns",
		},
		{
			name: "max_turns too high",
			modify: func(c *Config) {
				c.Defaults.MaxTurns = 1001
			},
			expectError: true,
			errorField:  "MaxTurns",
		},
		{
			name: "missing model",
			modify: func(c *Config) {
				c.Defaults.Model = ""
			},
			expectError: true,
			errorField:  "Model",
		},
		{
			name: "missing network name",
			modify: func(c *Config) {
				c.Network.Name = ""
			},
			expectError: true,
			errorField:  "Name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			tt.modify(cfg)

			loader := NewLoader()
			err := loader.Validate(cfg)

			if tt.expectError && err == nil {
				t.Errorf("expected validation error for %s, got nil", tt.errorField)
			}

			if !tt.expectError && err != nil {
				t.Errorf("unexpected validation error: %v", err)
			}
		})
	}
}

func TestValidationErrorMessages(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Version = "2" // Invalid

	loader := NewLoader()
	err := loader.Validate(cfg)

	if err == nil {
		t.Fatal("expected validation error")
	}

	errs, ok := err.(ValidationErrors)
	if !ok {
		t.Fatalf("expected ValidationErrors, got %T", err)
	}

	if len(errs) == 0 {
		t.Fatal("expected at least one validation error")
	}

	// Check that error message is user-friendly
	errMsg := errs[0].Message
	if errMsg == "" {
		t.Error("validation error message should not be empty")
	}
}

func TestConfigMerging(t *testing.T) {
	// Create temp directory structure
	tmpDir, err := os.MkdirTemp("", "tanuki-config-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Save current dir and change to temp dir
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create project config with different values
	projectConfig := `version: "1"
image:
  name: "custom/image"
  tag: "v1.0"
defaults:
  allowed_tools:
    - Read
    - Write
  max_turns: 75
  model: "claude-opus-4-20250514"
  resources:
    memory: "8g"
    cpus: "4"
git:
  branch_prefix: "custom/"
  auto_push: true
network:
  name: "custom-net"
`

	if err := os.WriteFile("tanuki.yaml", []byte(projectConfig), 0644); err != nil {
		t.Fatalf("failed to write project config: %v", err)
	}

	// Load config
	cfg, err := Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Verify project config values were loaded
	if cfg.Image.Name != "custom/image" {
		t.Errorf("expected image name 'custom/image', got '%s'", cfg.Image.Name)
	}

	if cfg.Defaults.MaxTurns != 75 {
		t.Errorf("expected max_turns 75, got %d", cfg.Defaults.MaxTurns)
	}

	if cfg.Git.BranchPrefix != "custom/" {
		t.Errorf("expected branch_prefix 'custom/', got '%s'", cfg.Git.BranchPrefix)
	}

	if !cfg.Git.AutoPush {
		t.Error("expected auto_push to be true")
	}
}

func TestFindProjectConfig(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "tanuki-config-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Save current dir and change to temp dir
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Test no config found
	loader := NewLoader()
	if path := loader.findProjectConfig(); path != "" {
		t.Errorf("expected no config found, got '%s'", path)
	}

	// Create tanuki.yaml
	if err := WriteDefault("tanuki.yaml"); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	if path := loader.findProjectConfig(); path != "tanuki.yaml" {
		t.Errorf("expected 'tanuki.yaml', got '%s'", path)
	}

	// Remove tanuki.yaml and create .tanuki/config/tanuki.yaml
	os.Remove("tanuki.yaml")
	os.MkdirAll(".tanuki/config", 0755)
	if err := WriteDefault(".tanuki/config/tanuki.yaml"); err != nil {
		t.Fatalf("failed to write alt config: %v", err)
	}

	if path := loader.findProjectConfig(); path != filepath.Join(".tanuki", "config", "tanuki.yaml") {
		t.Errorf("expected '.tanuki/config/tanuki.yaml', got '%s'", path)
	}
}

func TestGlobalConfigPath(t *testing.T) {
	path := GlobalConfigPath()

	if path == "" {
		t.Skip("could not determine home directory")
	}

	// Should contain .config/tanuki
	if !filepath.IsAbs(path) {
		t.Errorf("expected absolute path, got '%s'", path)
	}
}

func TestBuildConfig(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "tanuki-config-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Save current dir and change to temp dir
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Config with build instead of image name
	buildConfig := `version: "1"
image:
  build:
    context: "."
    dockerfile: "Dockerfile.tanuki"
defaults:
  allowed_tools:
    - Read
  max_turns: 50
  model: "claude-sonnet-4-5-20250514"
  resources:
    memory: "4g"
    cpus: "2"
git:
  branch_prefix: "tanuki/"
network:
  name: "tanuki-net"
`

	if err := os.WriteFile("tanuki.yaml", []byte(buildConfig), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Image.Build == nil {
		t.Fatal("expected Build config to be set")
	}

	if cfg.Image.Build.Context != "." {
		t.Errorf("expected build context '.', got '%s'", cfg.Image.Build.Context)
	}

	if cfg.Image.Build.Dockerfile != "Dockerfile.tanuki" {
		t.Errorf("expected dockerfile 'Dockerfile.tanuki', got '%s'", cfg.Image.Build.Dockerfile)
	}
}
