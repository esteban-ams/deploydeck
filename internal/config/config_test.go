package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeConfigFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write config file: %v", err)
	}
	return path
}

func minimalConfig() string {
	return `
auth:
  webhook_secret: "test-secret"
services:
  myapp:
    compose_file: "docker-compose.yml"
    service_name: "myapp"
`
}

func TestLoad_Valid(t *testing.T) {
	path := writeConfigFile(t, minimalConfig())
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Auth.WebhookSecret != "test-secret" {
		t.Errorf("expected secret 'test-secret', got %q", cfg.Auth.WebhookSecret)
	}
	if len(cfg.Services) != 1 {
		t.Errorf("expected 1 service, got %d", len(cfg.Services))
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/config.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	path := writeConfigFile(t, "invalid: [yaml: {broken")
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestValidate_MissingSecret(t *testing.T) {
	path := writeConfigFile(t, `
services:
  myapp:
    compose_file: "docker-compose.yml"
    service_name: "myapp"
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing webhook_secret")
	}
}

func TestValidate_MissingServices(t *testing.T) {
	path := writeConfigFile(t, `
auth:
  webhook_secret: "secret"
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing services")
	}
}

func TestValidate_MissingComposeFile(t *testing.T) {
	path := writeConfigFile(t, `
auth:
  webhook_secret: "secret"
services:
  myapp:
    service_name: "myapp"
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing compose_file")
	}
}

func TestValidate_MissingServiceName(t *testing.T) {
	path := writeConfigFile(t, `
auth:
  webhook_secret: "secret"
services:
  myapp:
    compose_file: "docker-compose.yml"
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing service_name")
	}
}

func TestValidate_InvalidMode(t *testing.T) {
	path := writeConfigFile(t, `
auth:
  webhook_secret: "secret"
services:
  myapp:
    compose_file: "docker-compose.yml"
    service_name: "myapp"
    mode: "invalid"
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid mode")
	}
}

func TestDefaults_ServerPort(t *testing.T) {
	path := writeConfigFile(t, minimalConfig())
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Server.Port != 9000 {
		t.Errorf("expected default port 9000, got %d", cfg.Server.Port)
	}
}

func TestDefaults_ServerHost(t *testing.T) {
	path := writeConfigFile(t, minimalConfig())
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("expected default host '0.0.0.0', got %q", cfg.Server.Host)
	}
}

func TestDefaults_ModePull(t *testing.T) {
	path := writeConfigFile(t, minimalConfig())
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	svc := cfg.Services["myapp"]
	if svc.Mode != DeployModePull {
		t.Errorf("expected default mode 'pull', got %q", svc.Mode)
	}
}

func TestDefaults_TimeoutPull(t *testing.T) {
	path := writeConfigFile(t, minimalConfig())
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	svc := cfg.Services["myapp"]
	if svc.Timeout != 5*time.Minute {
		t.Errorf("expected default pull timeout 5m, got %v", svc.Timeout)
	}
}

func TestDefaults_TimeoutBuild(t *testing.T) {
	path := writeConfigFile(t, `
auth:
  webhook_secret: "secret"
services:
  myapp:
    compose_file: "docker-compose.yml"
    service_name: "myapp"
    mode: "build"
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	svc := cfg.Services["myapp"]
	if svc.Timeout != 10*time.Minute {
		t.Errorf("expected default build timeout 10m, got %v", svc.Timeout)
	}
}

func TestDefaults_BuildModeBranch(t *testing.T) {
	path := writeConfigFile(t, `
auth:
  webhook_secret: "secret"
services:
  myapp:
    compose_file: "docker-compose.yml"
    service_name: "myapp"
    mode: "build"
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	svc := cfg.Services["myapp"]
	if svc.Branch != "main" {
		t.Errorf("expected default branch 'main', got %q", svc.Branch)
	}
}

func TestDefaults_HealthCheck(t *testing.T) {
	path := writeConfigFile(t, minimalConfig())
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	svc := cfg.Services["myapp"]
	if svc.HealthCheck.Timeout != 30*time.Second {
		t.Errorf("expected health timeout 30s, got %v", svc.HealthCheck.Timeout)
	}
	if svc.HealthCheck.Interval != 2*time.Second {
		t.Errorf("expected health interval 2s, got %v", svc.HealthCheck.Interval)
	}
	if svc.HealthCheck.Retries != 10 {
		t.Errorf("expected health retries 10, got %d", svc.HealthCheck.Retries)
	}
}

func TestDefaults_KeepImages(t *testing.T) {
	path := writeConfigFile(t, minimalConfig())
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	svc := cfg.Services["myapp"]
	if svc.Rollback.KeepImages != 3 {
		t.Errorf("expected keep_images 3, got %d", svc.Rollback.KeepImages)
	}
}

func TestEnvOverrides(t *testing.T) {
	tests := []struct {
		name     string
		envKey   string
		envValue string
		check    func(*Config) bool
		desc     string
	}{
		{
			name:     "port",
			envKey:   "DEPLOYDECK_PORT",
			envValue: "8080",
			check:    func(c *Config) bool { return c.Server.Port == 8080 },
			desc:     "port should be 8080",
		},
		{
			name:     "host",
			envKey:   "DEPLOYDECK_HOST",
			envValue: "127.0.0.1",
			check:    func(c *Config) bool { return c.Server.Host == "127.0.0.1" },
			desc:     "host should be 127.0.0.1",
		},
		{
			name:     "webhook_secret",
			envKey:   "DEPLOYDECK_WEBHOOK_SECRET",
			envValue: "env-secret",
			check:    func(c *Config) bool { return c.Auth.WebhookSecret == "env-secret" },
			desc:     "webhook_secret should be env-secret",
		},
		{
			name:     "log_level",
			envKey:   "DEPLOYDECK_LOG_LEVEL",
			envValue: "debug",
			check:    func(c *Config) bool { return c.Logging.Level == "debug" },
			desc:     "log_level should be debug",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeConfigFile(t, minimalConfig())
			os.Setenv(tt.envKey, tt.envValue)
			defer os.Unsetenv(tt.envKey)

			cfg, err := Load(path)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tt.check(cfg) {
				t.Errorf("env override failed: %s", tt.desc)
			}
		})
	}
}

func TestResolveTokenFiles_FromEnv(t *testing.T) {
	path := writeConfigFile(t, `
auth:
  webhook_secret: "secret"
services:
  myapp:
    compose_file: "docker-compose.yml"
    service_name: "myapp"
    mode: "build"
`)
	os.Setenv("DEPLOYDECK_CLONE_TOKEN", "env-token")
	defer os.Unsetenv("DEPLOYDECK_CLONE_TOKEN")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	svc := cfg.Services["myapp"]
	if svc.CloneToken != "env-token" {
		t.Errorf("expected clone_token 'env-token', got %q", svc.CloneToken)
	}
}

func TestResolveTokenFiles_ExplicitTakesPrecedence(t *testing.T) {
	path := writeConfigFile(t, `
auth:
  webhook_secret: "secret"
services:
  myapp:
    compose_file: "docker-compose.yml"
    service_name: "myapp"
    mode: "build"
    clone_token: "explicit-token"
`)
	os.Setenv("DEPLOYDECK_CLONE_TOKEN", "env-token")
	defer os.Unsetenv("DEPLOYDECK_CLONE_TOKEN")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	svc := cfg.Services["myapp"]
	if svc.CloneToken != "explicit-token" {
		t.Errorf("expected clone_token 'explicit-token', got %q", svc.CloneToken)
	}
}

func TestValidate_IPWhitelist_ValidEntries(t *testing.T) {
	t.Parallel()
	path := writeConfigFile(t, `
auth:
  webhook_secret: "secret"
server:
  ip_whitelist:
    - "10.0.0.1"
    - "192.168.1.0/24"
    - "::1"
services:
  myapp:
    compose_file: "docker-compose.yml"
    service_name: "myapp"
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error for valid ip_whitelist: %v", err)
	}
	if len(cfg.Server.IPWhitelist) != 3 {
		t.Errorf("expected 3 whitelist entries, got %d", len(cfg.Server.IPWhitelist))
	}
}

func TestValidate_IPWhitelist_InvalidEntry(t *testing.T) {
	t.Parallel()
	path := writeConfigFile(t, `
auth:
  webhook_secret: "secret"
server:
  ip_whitelist:
    - "not-an-ip"
services:
  myapp:
    compose_file: "docker-compose.yml"
    service_name: "myapp"
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid ip_whitelist entry")
	}
}

func TestValidate_IPWhitelist_EmptyIsAllowed(t *testing.T) {
	t.Parallel()
	// An absent ip_whitelist section must not trigger a validation error.
	path := writeConfigFile(t, minimalConfig())
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error with no ip_whitelist: %v", err)
	}
	if len(cfg.Server.IPWhitelist) != 0 {
		t.Errorf("expected empty whitelist, got %v", cfg.Server.IPWhitelist)
	}
}

func TestResolveTokenFiles_FromFile(t *testing.T) {
	tokenFile := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(tokenFile, []byte("file-token\n"), 0644); err != nil {
		t.Fatalf("write token file: %v", err)
	}

	configContent := `
auth:
  webhook_secret: "secret"
services:
  myapp:
    compose_file: "docker-compose.yml"
    service_name: "myapp"
    mode: "build"
    clone_token_file: "` + tokenFile + `"
`
	path := writeConfigFile(t, configContent)

	// Unset env var to ensure file takes precedence
	os.Unsetenv("DEPLOYDECK_CLONE_TOKEN")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	svc := cfg.Services["myapp"]
	if svc.CloneToken != "file-token" {
		t.Errorf("expected clone_token 'file-token', got %q", svc.CloneToken)
	}
}
