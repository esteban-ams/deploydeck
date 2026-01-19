package webhook

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/esteban-ams/fastship/internal/config"
	"github.com/esteban-ams/fastship/internal/deploy"
	"github.com/labstack/echo/v4"
)

// Handler handles webhook HTTP requests
type Handler struct {
	verifier *Verifier
	engine   *deploy.Engine
	config   *config.Config
	startTime time.Time
}

// NewHandler creates a new webhook handler
func NewHandler(cfg *config.Config, engine *deploy.Engine) *Handler {
	return &Handler{
		verifier:  NewVerifier(cfg.Auth.WebhookSecret),
		engine:    engine,
		config:    cfg,
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

// HandleDeploy handles POST /api/deploy/:service
func (h *Handler) HandleDeploy(c echo.Context) error {
	serviceName := c.Param("service")

	// Verify the service exists
	if _, ok := h.config.Services[serviceName]; !ok {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": fmt.Sprintf("service %s not found", serviceName),
		})
	}

	// Read request body
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "failed to read request body",
		})
	}

	// Verify webhook signature
	headers := make(map[string]string)
	for key, values := range c.Request().Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	if err := h.verifier.Verify(headers, body); err != nil {
		log.Printf("Authentication failed: %v", err)
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "authentication failed",
		})
	}

	// Parse request body
	var req DeployRequest
	if len(body) > 0 {
		if err := json.Unmarshal(body, &req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "invalid request body",
			})
		}
	}

	// Initiate deployment
	deployment, err := h.engine.Deploy(c.Request().Context(), serviceName, deploy.DeployOptions{
		Image: req.Image,
		Tag:   req.Tag,
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
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
			"error": fmt.Sprintf("service %s not found", serviceName),
		})
	}

	// Read request body for authentication
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "failed to read request body",
		})
	}

	// Verify webhook signature
	headers := make(map[string]string)
	for key, values := range c.Request().Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	if err := h.verifier.Verify(headers, body); err != nil {
		log.Printf("Authentication failed: %v", err)
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "authentication failed",
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
	Image       string  `json:"image,omitempty"`
	StartedAt   string  `json:"started_at"`
	CompletedAt *string `json:"completed_at,omitempty"`
	Error       string  `json:"error,omitempty"`
}

// HandleListDeployments handles GET /api/deployments
func (h *Handler) HandleListDeployments(c echo.Context) error {
	deployments := h.engine.ListDeployments()

	info := make([]DeploymentInfo, 0, len(deployments))
	for _, d := range deployments {
		item := DeploymentInfo{
			ID:        d.ID,
			Service:   d.Service,
			Status:    string(d.Status),
			Image:     d.Image,
			StartedAt: d.StartedAt.Format(time.RFC3339),
			Error:     d.ErrorMessage,
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
		Version: "1.0.0",
		Uptime:  uptime.String(),
	})
}
