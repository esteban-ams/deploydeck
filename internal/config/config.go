package config

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// DeployMode represents how a service is deployed
type DeployMode string

const (
	DeployModePull  DeployMode = "pull"
	DeployModeBuild DeployMode = "build"
)

// Config represents the complete FastShip configuration
type Config struct {
	Server    ServerConfig             `yaml:"server"`
	Auth      AuthConfig               `yaml:"auth"`
	Dashboard DashboardConfig          `yaml:"dashboard"`
	Logging   LoggingConfig            `yaml:"logging"`
	Services  map[string]ServiceConfig `yaml:"services"`
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Port int    `yaml:"port"`
	Host string `yaml:"host"`
	TLS  struct {
		Enabled  bool   `yaml:"enabled"`
		CertFile string `yaml:"cert_file"`
		KeyFile  string `yaml:"key_file"`
	} `yaml:"tls"`
}

// AuthConfig holds webhook authentication settings
type AuthConfig struct {
	WebhookSecret string `yaml:"webhook_secret"`
}

// DashboardConfig holds web dashboard settings
type DashboardConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level  string `yaml:"level"`  // debug, info, warn, error
	Format string `yaml:"format"` // json, text
}

// ServiceConfig represents a single deployable service
type ServiceConfig struct {
	ComposeFile    string            `yaml:"compose_file"`
	ServiceName    string            `yaml:"service_name"`
	WorkingDir     string            `yaml:"working_dir"`
	Mode           DeployMode        `yaml:"mode"`              // "pull" (default) or "build"
	Branch         string            `yaml:"branch"`            // branch filter for build mode (default "main")
	Repo           string            `yaml:"repo"`              // fallback clone URL if webhook payload lacks it
	CloneToken     string            `yaml:"clone_token"`       // auth token for private repos
	CloneTokenFile string            `yaml:"clone_token_file"`  // path to file containing token (Docker secrets)
	Timeout        time.Duration     `yaml:"timeout"`           // overall deployment timeout
	PruneAfterBuild bool            `yaml:"prune_after_build"` // clean Docker build cache after successful build
	HealthCheck    HealthCheckConfig `yaml:"health_check"`
	Rollback       RollbackConfig    `yaml:"rollback"`
	Env            map[string]string `yaml:"env"`
}

// HealthCheckConfig holds health check settings
type HealthCheckConfig struct {
	Enabled  bool          `yaml:"enabled"`
	URL      string        `yaml:"url"`
	Timeout  time.Duration `yaml:"timeout"`
	Interval time.Duration `yaml:"interval"`
	Retries  int           `yaml:"retries"`
}

// RollbackConfig holds rollback settings
type RollbackConfig struct {
	Enabled    bool `yaml:"enabled"`
	KeepImages int  `yaml:"keep_images"`
}

// Load reads and parses the configuration file
func Load(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	// Apply environment variable overrides
	applyEnvOverrides(&cfg)

	// Resolve clone tokens from files and env var
	resolveTokenFiles(&cfg)

	// Apply defaults
	applyDefaults(&cfg)

	// Validate configuration
	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &cfg, nil
}

// applyEnvOverrides applies environment variable overrides
func applyEnvOverrides(cfg *Config) {
	if port := os.Getenv("FASTSHIP_PORT"); port != "" {
		fmt.Sscanf(port, "%d", &cfg.Server.Port)
	}
	if host := os.Getenv("FASTSHIP_HOST"); host != "" {
		cfg.Server.Host = host
	}
	if secret := os.Getenv("FASTSHIP_WEBHOOK_SECRET"); secret != "" {
		cfg.Auth.WebhookSecret = secret
	}
	if logLevel := os.Getenv("FASTSHIP_LOG_LEVEL"); logLevel != "" {
		cfg.Logging.Level = logLevel
	}
}

// resolveTokenFiles resolves clone tokens from files and env var fallback.
// Priority: clone_token (YAML) > clone_token_file > FASTSHIP_CLONE_TOKEN (env)
func resolveTokenFiles(cfg *Config) {
	envToken := os.Getenv("FASTSHIP_CLONE_TOKEN")

	for name, svc := range cfg.Services {
		if svc.CloneToken != "" {
			continue
		}

		// Try reading from file
		if svc.CloneTokenFile != "" {
			data, err := os.ReadFile(svc.CloneTokenFile)
			if err != nil {
				log.Printf("Warning: could not read clone_token_file for service %s: %v", name, err)
			} else if token := strings.TrimSpace(string(data)); token != "" {
				svc.CloneToken = token
				cfg.Services[name] = svc
				continue
			}
		}

		// Fall back to env var
		if envToken != "" {
			svc.CloneToken = envToken
			cfg.Services[name] = svc
		}
	}
}

// applyDefaults sets default values for missing configuration
func applyDefaults(cfg *Config) {
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 9000
	}
	if cfg.Server.Host == "" {
		cfg.Server.Host = "0.0.0.0"
	}
	if cfg.Logging.Level == "" {
		cfg.Logging.Level = "info"
	}
	if cfg.Logging.Format == "" {
		cfg.Logging.Format = "text"
	}

	// Apply defaults for each service
	for name, svc := range cfg.Services {
		if svc.Mode == "" {
			svc.Mode = DeployModePull
		}
		if svc.Mode == DeployModeBuild && svc.Branch == "" {
			svc.Branch = "main"
		}
		if svc.Timeout == 0 {
			switch svc.Mode {
			case DeployModeBuild:
				svc.Timeout = 10 * time.Minute
			default:
				svc.Timeout = 5 * time.Minute
			}
		}
		if svc.WorkingDir == "" {
			// Use directory of compose file as working dir
			svc.WorkingDir = "."
		}
		if svc.HealthCheck.Timeout == 0 {
			svc.HealthCheck.Timeout = 30 * time.Second
		}
		if svc.HealthCheck.Interval == 0 {
			svc.HealthCheck.Interval = 2 * time.Second
		}
		if svc.HealthCheck.Retries == 0 {
			svc.HealthCheck.Retries = 10
		}
		if svc.Rollback.KeepImages == 0 {
			svc.Rollback.KeepImages = 3
		}
		cfg.Services[name] = svc
	}
}

// validate checks that the configuration is valid
func validate(cfg *Config) error {
	if cfg.Auth.WebhookSecret == "" {
		return fmt.Errorf("auth.webhook_secret is required")
	}

	if len(cfg.Services) == 0 {
		return fmt.Errorf("at least one service must be configured")
	}

	for name, svc := range cfg.Services {
		if svc.ComposeFile == "" {
			return fmt.Errorf("service %s: compose_file is required", name)
		}
		if svc.ServiceName == "" {
			return fmt.Errorf("service %s: service_name is required", name)
		}
		if svc.Mode != DeployModePull && svc.Mode != DeployModeBuild {
			return fmt.Errorf("service %s: mode must be 'pull' or 'build', got '%s'", name, svc.Mode)
		}
	}

	return nil
}
