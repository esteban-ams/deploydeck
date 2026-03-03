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
	RateLimit RateLimitConfig          `yaml:"rate_limit"`
	Dashboard DashboardConfig          `yaml:"dashboard"`
	Logging   LoggingConfig            `yaml:"logging"`
	Services  map[string]ServiceConfig `yaml:"services"`
}

// RateLimitConfig holds rate limiting configuration for webhook endpoints
type RateLimitConfig struct {
	// Enabled controls whether rate limiting is active (default: true)
	Enabled bool `yaml:"enabled"`
	// RequestsPerMinute is the max number of requests per IP per minute (default: 10)
	RequestsPerMinute int `yaml:"requests_per_minute"`
	// BurstSize is the maximum burst above the steady rate (default: 5)
	BurstSize int `yaml:"burst_size"`
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
		return nil, fmt.Errorf("cannot read config file %q: %w (run 'cp config.example.yaml config.yaml' to create one)", configPath, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("cannot parse config file %q as YAML: %w", configPath, err)
	}

	// Apply environment variable overrides
	applyEnvOverrides(&cfg)

	// Resolve clone tokens from files and env var
	resolveTokenFiles(&cfg)

	// Apply defaults
	applyDefaults(&cfg)

	// Validate configuration
	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("invalid configuration in %q: %w", configPath, err)
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
	if rps := os.Getenv("FASTSHIP_RATE_LIMIT_RPM"); rps != "" {
		fmt.Sscanf(rps, "%d", &cfg.RateLimit.RequestsPerMinute)
	}
	if burst := os.Getenv("FASTSHIP_RATE_LIMIT_BURST"); burst != "" {
		fmt.Sscanf(burst, "%d", &cfg.RateLimit.BurstSize)
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
				log.Printf("Warning: service %q: cannot read clone_token_file %q: %v (falling back to FASTSHIP_CLONE_TOKEN env var)", name, svc.CloneTokenFile, err)
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

	// Rate limiting defaults: enabled with 10 req/min and burst of 5.
	// Enabled defaults to true when the rate_limit section is absent or
	// when enabled is not explicitly set to false.
	if !cfg.RateLimit.Enabled && cfg.RateLimit.RequestsPerMinute == 0 {
		// Section was omitted entirely — apply full defaults with enabled=true.
		cfg.RateLimit.Enabled = true
	}
	if cfg.RateLimit.RequestsPerMinute == 0 {
		cfg.RateLimit.RequestsPerMinute = 10
	}
	if cfg.RateLimit.BurstSize == 0 {
		cfg.RateLimit.BurstSize = 5
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
		return fmt.Errorf("auth.webhook_secret is required: set it in config.yaml or via the FASTSHIP_WEBHOOK_SECRET environment variable")
	}

	if len(cfg.Services) == 0 {
		return fmt.Errorf("no services defined: add at least one service under the 'services' key in config.yaml")
	}

	for name, svc := range cfg.Services {
		if svc.ComposeFile == "" {
			return fmt.Errorf("service %q: 'compose_file' is required (e.g. compose_file: /opt/myapp/docker-compose.yml)", name)
		}
		if svc.ServiceName == "" {
			return fmt.Errorf("service %q: 'service_name' is required — this must match the service name in your Docker Compose file", name)
		}
		if svc.Mode != DeployModePull && svc.Mode != DeployModeBuild {
			return fmt.Errorf("service %q: invalid mode %q — must be 'pull' (deploy a pre-built image) or 'build' (clone and build on this server)", name, svc.Mode)
		}
	}

	return nil
}
