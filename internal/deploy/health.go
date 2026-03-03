package deploy

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/esteban-ams/fastship/internal/config"
)

// HealthChecker performs HTTP health checks on deployed services
type HealthChecker struct {
	client *http.Client
}

// NewHealthChecker creates a new health checker
func NewHealthChecker() *HealthChecker {
	return &HealthChecker{
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// Wait polls the health check URL until the service is healthy or timeout is reached
func (h *HealthChecker) Wait(ctx context.Context, cfg config.HealthCheckConfig) error {
	if !cfg.Enabled {
		return nil
	}

	if cfg.URL == "" {
		return fmt.Errorf("health check is enabled but 'url' is not set: add 'health_check.url' to the service config (e.g. http://localhost:8080/health)")
	}

	deadline := time.Now().Add(cfg.Timeout)
	attempt := 0

	for {
		attempt++

		// Check if we've exceeded the timeout
		if time.Now().After(deadline) {
			return fmt.Errorf("health check timed out after %v (%d attempts): %q did not return a 2xx response — check that the service started correctly",
				cfg.Timeout, attempt, cfg.URL)
		}

		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Perform health check
		if err := h.check(ctx, cfg.URL); err == nil {
			// Health check passed
			return nil
		}

		// Health check failed, wait before retry
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(cfg.Interval):
			// Continue to next attempt
		}

		// Check if we've exceeded max retries
		if attempt >= cfg.Retries {
			return fmt.Errorf("health check failed after %d attempts (interval: %v): %q did not return a 2xx response — check the service logs with 'docker compose logs'",
				attempt, cfg.Interval, cfg.URL)
		}
	}
}

// check performs a single health check request
func (h *HealthChecker) check(ctx context.Context, url string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return fmt.Errorf("health check request to %q failed: %w", url, err)
	}
	defer resp.Body.Close()

	// Consider 2xx status codes as healthy
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	return fmt.Errorf("health check URL %q returned unhealthy status %d (expected 2xx)", url, resp.StatusCode)
}
