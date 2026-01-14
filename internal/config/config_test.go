package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bkonkle/tanuki/internal/service"
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

	// Verify default services
	if cfg.Services == nil {
		t.Fatal("expected services to be configured")
	}

	if len(cfg.Services) != 2 {
		t.Errorf("expected 2 default services, got %d", len(cfg.Services))
	}

	if _, ok := cfg.Services["postgres"]; !ok {
		t.Error("expected postgres service to be configured")
	}

	if _, ok := cfg.Services["redis"]; !ok {
		t.Error("expected redis service to be configured")
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

func TestServiceConfiguration(t *testing.T) {
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

	// Config with services
	serviceConfig := `version: "1"
image:
  name: "bkonkle/tanuki"
  tag: "latest"
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
services:
  postgres:
    enabled: true
    image: "postgres:16"
    port: 5432
    environment:
      POSTGRES_USER: "tanuki"
      POSTGRES_PASSWORD: "tanuki"
      POSTGRES_DB: "tanuki_dev"
    volumes:
      - "tanuki-postgres:/var/lib/postgresql/data"
    healthcheck:
      command: ["pg_isready", "-U", "tanuki"]
      interval: "5s"
      timeout: "3s"
      retries: 5
  redis:
    enabled: true
    image: "redis:7"
    port: 6379
    volumes:
      - "tanuki-redis:/data"
    healthcheck:
      command: ["redis-cli", "ping"]
      interval: "5s"
      timeout: "3s"
      retries: 5
`

	if err := os.WriteFile("tanuki.yaml", []byte(serviceConfig), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Verify services loaded
	if cfg.Services == nil {
		t.Fatal("expected services to be configured")
	}

	if len(cfg.Services) != 2 {
		t.Errorf("expected 2 services, got %d", len(cfg.Services))
	}

	// Verify postgres config
	postgres, ok := cfg.Services["postgres"]
	if !ok {
		t.Fatal("expected postgres service")
	}

	if !postgres.Enabled {
		t.Error("expected postgres to be enabled")
	}

	if postgres.Image != "postgres:16" {
		t.Errorf("expected postgres image 'postgres:16', got '%s'", postgres.Image)
	}

	if postgres.Port != 5432 {
		t.Errorf("expected postgres port 5432, got %d", postgres.Port)
	}

	// Note: Viper lowercases map keys by default, so environment keys become lowercase
	expectedUser := postgres.Environment["POSTGRES_USER"]
	if expectedUser == "" {
		expectedUser = postgres.Environment["postgres_user"] // Viper lowercases keys
	}
	if expectedUser != "tanuki" {
		t.Errorf("expected POSTGRES_USER/postgres_user 'tanuki', got '%s'", expectedUser)
	}

	if postgres.Healthcheck == nil {
		t.Fatal("expected postgres healthcheck")
	}

	if len(postgres.Healthcheck.Command) != 3 {
		t.Errorf("expected healthcheck command length 3, got %d", len(postgres.Healthcheck.Command))
	}

	// Verify redis config
	redis, ok := cfg.Services["redis"]
	if !ok {
		t.Fatal("expected redis service")
	}

	if !redis.Enabled {
		t.Error("expected redis to be enabled")
	}

	if redis.Image != "redis:7" {
		t.Errorf("expected redis image 'redis:7', got '%s'", redis.Image)
	}
}

func TestServiceValidation(t *testing.T) {
	tests := []struct {
		name        string
		modify      func(*Config)
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid services",
			modify:      func(c *Config) {},
			expectError: false,
		},
		{
			name: "enabled service missing image",
			modify: func(c *Config) {
				c.Services["postgres"].Enabled = true
				c.Services["postgres"].Image = ""
			},
			expectError: true,
			errorMsg:    "image is required",
		},
		{
			name: "enabled service invalid port",
			modify: func(c *Config) {
				c.Services["postgres"].Enabled = true
				c.Services["postgres"].Port = 0
			},
			expectError: true,
			errorMsg:    "port must be greater than 0",
		},
		{
			name: "nil service config",
			modify: func(c *Config) {
				c.Services["postgres"] = nil
			},
			expectError: true,
			errorMsg:    "configuration is nil",
		},
		{
			name: "healthcheck with empty command",
			modify: func(c *Config) {
				c.Services["postgres"].Enabled = true
				c.Services["postgres"].Healthcheck = &service.HealthcheckConfig{
					Command:  []string{},
					Interval: "5s",
					Timeout:  "3s",
					Retries:  5,
				}
			},
			expectError: true,
			errorMsg:    "healthcheck command is required",
		},
		{
			name: "disabled service missing image (should pass)",
			modify: func(c *Config) {
				c.Services["postgres"].Enabled = false
				c.Services["postgres"].Image = ""
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			tt.modify(cfg)

			loader := NewLoader()
			err := loader.Validate(cfg)

			if tt.expectError && err == nil {
				t.Error("expected validation error, got nil")
			}

			if !tt.expectError && err != nil {
				t.Errorf("unexpected validation error: %v", err)
			}

			if tt.expectError && err != nil {
				errMsg := err.Error()
				if tt.errorMsg != "" && !contains(errMsg, tt.errorMsg) {
					t.Errorf("expected error message to contain '%s', got '%s'", tt.errorMsg, errMsg)
				}
			}
		})
	}
}

func TestServiceDefaults(t *testing.T) {
	// Test that default service configs are properly set up
	postgres := service.DefaultPostgresConfig()
	if postgres == nil {
		t.Fatal("expected postgres default config")
	}

	if postgres.Enabled {
		t.Error("expected default postgres to be disabled")
	}

	if postgres.Image != "postgres:16" {
		t.Errorf("expected postgres image 'postgres:16', got '%s'", postgres.Image)
	}

	if postgres.Port != 5432 {
		t.Errorf("expected postgres port 5432, got %d", postgres.Port)
	}

	redis := service.DefaultRedisConfig()
	if redis == nil {
		t.Fatal("expected redis default config")
	}

	if redis.Enabled {
		t.Error("expected default redis to be disabled")
	}

	if redis.Image != "redis:7" {
		t.Errorf("expected redis image 'redis:7', got '%s'", redis.Image)
	}

	if redis.Port != 6379 {
		t.Errorf("expected redis port 6379, got %d", redis.Port)
	}
}

// Helper function for string contains check
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || (len(s) > 0 && len(substr) > 0 && indexOf(s, substr) >= 0))
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
