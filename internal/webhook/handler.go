package webhook

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/esteban-ams/deploydeck/internal/config"
	"github.com/esteban-ams/deploydeck/internal/deploy"
	"github.com/labstack/echo/v4"
)

// Handler handles webhook HTTP requests
type Handler struct {
	verifier  *Verifier
	engine    *deploy.Engine
	config    *config.Config
	version   string
	startTime time.Time
}

// NewHandler creates a new webhook handler
func NewHandler(cfg *config.Config, engine *deploy.Engine, version string) *Handler {
	return &Handler{
		verifier:  NewVerifier(cfg.Auth.WebhookSecret),
		engine:    engine,
		config:    cfg,
		version:   version,
		startTime: time.Now(),
	}
}

// DeployRequest represents the request body for deployment
type DeployRequest struct {
	Image string `json:"image"`
	Tag   string `json:"tag"`
}

// DeployResponse represents the response for a deployment request
type DeployResponse struct {
	Status       string `json:"status"`
	DeploymentID string `json:"deployment_id"`
	Service      string `json:"service"`
}

// authHeaders extracts the three supported auth headers from an Echo request.
func authHeaders(c echo.Context) map[string]string {
	return map[string]string{
		"X-Hub-Signature-256": c.Request().Header.Get("X-Hub-Signature-256"),
		"X-GitLab-Token":      c.Request().Header.Get("X-GitLab-Token"),
		"X-DeployDeck-Secret": c.Request().Header.Get("X-DeployDeck-Secret"),
	}
}

// HandleDeploy handles POST /api/deploy/:service
func (h *Handler) HandleDeploy(c echo.Context) error {
	serviceName := c.Param("service")

	// Verify the service exists
	if _, ok := h.config.Services[serviceName]; !ok {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": fmt.Sprintf("service %q not found: check that it is defined under 'services' in config.yaml", serviceName),
		})
	}

	// Read request body
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "failed to read request body: the connection may have been interrupted",
		})
	}

	// Verify webhook signature
	authMethod, err := h.verifier.Verify(authHeaders(c), body)
	if err != nil {
		log.Printf("Authentication failed for service %q: %v", serviceName, err)
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": fmt.Sprintf("authentication failed for service %q: %v", serviceName, err),
		})
	}

	svcCfg := h.config.Services[serviceName]
	opts := deploy.DeployOptions{}

	if svcCfg.Mode == config.DeployModeBuild {
		// Build mode: parse webhook push payload to get clone info
		var pushEvent *PushEvent
		switch authMethod {
		case AuthMethodGitHub:
			pushEvent, _ = ParseGitHubPush(body)
		case AuthMethodGitLab:
			pushEvent, _ = ParseGitLabPush(body)
		case AuthMethodDeployDeck:
			// Try GitHub format first, then GitLab
			pushEvent, _ = ParseGitHubPush(body)
			if pushEvent == nil {
				pushEvent, _ = ParseGitLabPush(body)
			}
		}

		// Branch filter: only deploy if push is to configured branch
		if pushEvent != nil && pushEvent.Branch != svcCfg.Branch {
			log.Printf("Skipping deployment for %s: push to branch %q, service deploys branch %q", serviceName, pushEvent.Branch, svcCfg.Branch)
			return c.JSON(http.StatusOK, map[string]string{
				"status": "skipped",
				"reason": fmt.Sprintf("push to branch %q ignored: service %q only deploys branch %q", pushEvent.Branch, serviceName, svcCfg.Branch),
			})
		}

		// Determine clone URL: webhook payload takes precedence over config fallback
		if pushEvent != nil && pushEvent.CloneURL != "" {
			opts.CloneURL = pushEvent.CloneURL
			opts.Branch = pushEvent.Branch
			opts.Commit = pushEvent.Commit
		} else {
			opts.CloneURL = svcCfg.Repo
			opts.Branch = svcCfg.Branch
		}

		if opts.CloneURL == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": fmt.Sprintf("service %q uses build mode but no repository URL was found: "+
					"either send a GitHub/GitLab push webhook payload (which includes the clone URL) "+
					"or set 'repo' under service %q in config.yaml", serviceName, serviceName),
			})
		}
	} else {
		// Pull mode: parse standard deploy request
		var req DeployRequest
		if len(body) > 0 {
			if err := json.Unmarshal(body, &req); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{
					"error": fmt.Sprintf("failed to parse request body for service %q: expected JSON with optional 'image' and 'tag' fields, got: %v", serviceName, err),
				})
			}
		}
		opts.Image = req.Image
		opts.Tag = req.Tag
	}

	// Initiate deployment
	deployment, err := h.engine.Deploy(c.Request().Context(), serviceName, opts)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("failed to initiate deployment for service %q: %v", serviceName, err),
		})
	}

	log.Printf("Deployment %s initiated for service %s", deployment.ID, serviceName)

	return c.JSON(http.StatusOK, DeployResponse{
		Status:       string(deployment.Status),
		DeploymentID: deployment.ID,
		Service:      serviceName,
	})
}

// RollbackResponse represents the response for a rollback request
type RollbackResponse struct {
	Status       string `json:"status"`
	DeploymentID string `json:"deployment_id"`
	Service      string `json:"service"`
	Message      string `json:"message"`
}

// HandleRollback handles POST /api/rollback/:service
func (h *Handler) HandleRollback(c echo.Context) error {
	serviceName := c.Param("service")

	// Verify the service exists
	if _, ok := h.config.Services[serviceName]; !ok {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": fmt.Sprintf("service %q not found: check that it is defined under 'services' in config.yaml", serviceName),
		})
	}

	// Read request body for authentication
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "failed to read request body: the connection may have been interrupted",
		})
	}

	// Verify webhook signature
	if _, err := h.verifier.Verify(authHeaders(c), body); err != nil {
		log.Printf("Authentication failed for service %q rollback: %v", serviceName, err)
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": fmt.Sprintf("authentication failed for service %q rollback: %v", serviceName, err),
		})
	}

	return c.JSON(http.StatusOK, RollbackResponse{
		Status:  "not_implemented",
		Service: serviceName,
		Message: "Rollback functionality will be implemented with persistent storage",
	})
}

// DeploymentInfo represents deployment information in the response
type DeploymentInfo struct {
	ID          string  `json:"id"`
	Service     string  `json:"service"`
	Status      string  `json:"status"`
	Mode        string  `json:"mode,omitempty"`
	Image       string  `json:"image,omitempty"`
	RollbackTag string  `json:"rollback_tag,omitempty"`
	StartedAt   string  `json:"started_at"`
	CompletedAt *string `json:"completed_at,omitempty"`
	Error       string  `json:"error,omitempty"`
}

// HandleListDeployments handles GET /api/deployments
func (h *Handler) HandleListDeployments(c echo.Context) error {
	// GET requests carry no body; pass nil so HMAC methods fall through to
	// token-based checks (X-GitLab-Token or X-DeployDeck-Secret).
	if _, err := h.verifier.Verify(authHeaders(c), nil); err != nil {
		log.Printf("Authentication failed for GET /api/deployments: %v", err)
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": fmt.Sprintf("authentication failed: %v", err),
		})
	}

	deployments := h.engine.ListDeployments()

	info := make([]DeploymentInfo, 0, len(deployments))
	for _, d := range deployments {
		item := DeploymentInfo{
			ID:          d.ID,
			Service:     d.Service,
			Status:      string(d.Status),
			Mode:        d.Mode,
			Image:       d.Image,
			RollbackTag: d.RollbackTag,
			StartedAt:   d.StartedAt.Format(time.RFC3339),
			Error:       d.ErrorMessage,
		}
		if d.CompletedAt != nil {
			completedAt := d.CompletedAt.Format(time.RFC3339)
			item.CompletedAt = &completedAt
		}
		info = append(info, item)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"deployments": info,
	})
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
	Uptime  string `json:"uptime"`
}

// HandleHealth handles GET /api/health
func (h *Handler) HandleHealth(c echo.Context) error {
	uptime := time.Since(h.startTime)

	return c.JSON(http.StatusOK, HealthResponse{
		Status:  "healthy",
		Version: h.version,
		Uptime:  uptime.String(),
	})
}
