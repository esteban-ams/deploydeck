package deploy

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/esteban-ams/fastship/internal/config"
	"github.com/esteban-ams/fastship/internal/docker"
)

// Status represents the deployment status
type Status string

const (
	StatusPending     Status = "pending"
	StatusRunning     Status = "running"
	StatusSuccess     Status = "success"
	StatusFailed      Status = "failed"
	StatusRolledBack  Status = "rolled_back"
)

// Deployment represents a single deployment
type Deployment struct {
	ID            string
	Service       string
	Status        Status
	Image         string
	PreviousImage string
	StartedAt     time.Time
	CompletedAt   *time.Time
	ErrorMessage  string
}

// Engine orchestrates deployments
type Engine struct {
	mu            sync.Mutex
	serviceLocks  map[string]*sync.Mutex
	deployments   map[string]*Deployment
	dockerClient  *docker.Client
	healthChecker *HealthChecker
	config        *config.Config
}

// NewEngine creates a new deployment engine
func NewEngine(cfg *config.Config) *Engine {
	return &Engine{
		serviceLocks:  make(map[string]*sync.Mutex),
		deployments:   make(map[string]*Deployment),
		dockerClient:  docker.NewClient(),
		healthChecker: NewHealthChecker(),
		config:        cfg,
	}
}

// DeployOptions holds deployment options
type DeployOptions struct {
	Image string
	Tag   string
}

// Deploy initiates a deployment for a service
// Returns the deployment ID immediately and performs deployment asynchronously
func (e *Engine) Deploy(ctx context.Context, serviceName string, opts DeployOptions) (*Deployment, error) {
	// Check if service exists in config
	svcCfg, ok := e.config.Services[serviceName]
	if !ok {
		return nil, fmt.Errorf("service %s not found in configuration", serviceName)
	}

	// Get or create service lock
	svcLock := e.getServiceLock(serviceName)

	// Create deployment record
	deployment := &Deployment{
		ID:        generateDeploymentID(),
		Service:   serviceName,
		Status:    StatusPending,
		Image:     opts.Image,
		StartedAt: time.Now(),
	}

	e.mu.Lock()
	e.deployments[deployment.ID] = deployment
	e.mu.Unlock()

	// Start deployment in background
	go func() {
		// Lock this service to serialize deployments
		svcLock.Lock()
		defer svcLock.Unlock()

		e.executeDeploy(context.Background(), deployment, svcCfg)
	}()

	return deployment, nil
}

// executeDeploy performs the actual deployment
func (e *Engine) executeDeploy(ctx context.Context, deployment *Deployment, svcCfg config.ServiceConfig) {
	// Update status to running
	e.updateDeployment(deployment.ID, func(d *Deployment) {
		d.Status = StatusRunning
	})

	log.Printf("Starting deployment %s for service %s", deployment.ID, deployment.Service)

	// Step 1: Get current image for rollback
	dockerOpts := docker.ComposeOptions{
		ComposeFile: svcCfg.ComposeFile,
		Service:     svcCfg.ServiceName,
		WorkingDir:  svcCfg.WorkingDir,
		Env:         svcCfg.Env,
	}

	containerName, err := e.dockerClient.GetContainerName(ctx, dockerOpts)
	if err != nil {
		log.Printf("Warning: could not get container name for %s: %v", deployment.Service, err)
	} else {
		currentImage, err := e.dockerClient.GetCurrentImage(ctx, containerName)
		if err != nil {
			log.Printf("Warning: could not get current image for %s: %v", deployment.Service, err)
		} else {
			deployment.PreviousImage = currentImage
			log.Printf("Saved current image for rollback: %s", currentImage)
		}
	}

	// Step 2: Pull new image
	log.Printf("Pulling image for service %s", deployment.Service)
	if err := e.dockerClient.ComposePull(ctx, dockerOpts); err != nil {
		e.handleDeploymentFailure(deployment, "pull", err, svcCfg)
		return
	}

	// Step 3: Deploy (docker compose up -d)
	log.Printf("Deploying service %s", deployment.Service)
	if err := e.dockerClient.ComposeUp(ctx, dockerOpts); err != nil {
		e.handleDeploymentFailure(deployment, "up", err, svcCfg)
		return
	}

	// Step 4: Health check
	if svcCfg.HealthCheck.Enabled {
		log.Printf("Running health check for service %s", deployment.Service)
		if err := e.healthChecker.Wait(ctx, svcCfg.HealthCheck); err != nil {
			e.handleDeploymentFailure(deployment, "health_check", err, svcCfg)
			return
		}
		log.Printf("Health check passed for service %s", deployment.Service)
	}

	// Step 5: Mark deployment as successful
	now := time.Now()
	e.updateDeployment(deployment.ID, func(d *Deployment) {
		d.Status = StatusSuccess
		d.CompletedAt = &now
	})

	log.Printf("Deployment %s completed successfully", deployment.ID)
}

// handleDeploymentFailure handles a failed deployment
func (e *Engine) handleDeploymentFailure(deployment *Deployment, phase string, err error, svcCfg config.ServiceConfig) {
	errMsg := fmt.Sprintf("deployment failed at %s phase: %v", phase, err)
	log.Printf("Deployment %s failed: %s", deployment.ID, errMsg)

	// Attempt rollback if enabled and we have a previous image
	if svcCfg.Rollback.Enabled && deployment.PreviousImage != "" {
		log.Printf("Attempting rollback for deployment %s", deployment.ID)
		if err := e.rollback(context.Background(), deployment, svcCfg); err != nil {
			errMsg = fmt.Sprintf("%s; rollback failed: %v", errMsg, err)
			log.Printf("Rollback failed for deployment %s: %v", deployment.ID, err)
		} else {
			log.Printf("Rollback successful for deployment %s", deployment.ID)
		}
	}

	now := time.Now()
	e.updateDeployment(deployment.ID, func(d *Deployment) {
		d.Status = StatusFailed
		d.ErrorMessage = errMsg
		d.CompletedAt = &now
	})
}

// rollback reverts to the previous image
func (e *Engine) rollback(ctx context.Context, deployment *Deployment, svcCfg config.ServiceConfig) error {
	// For simplicity, we'll just restart the service
	// In a real implementation, you might want to explicitly set the image
	dockerOpts := docker.ComposeOptions{
		ComposeFile: svcCfg.ComposeFile,
		Service:     svcCfg.ServiceName,
		WorkingDir:  svcCfg.WorkingDir,
		Env:         svcCfg.Env,
	}

	if err := e.dockerClient.ComposeUp(ctx, dockerOpts); err != nil {
		return fmt.Errorf("rollback compose up failed: %w", err)
	}

	deployment.Status = StatusRolledBack
	return nil
}

// GetDeployment retrieves a deployment by ID
func (e *Engine) GetDeployment(id string) (*Deployment, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	deployment, ok := e.deployments[id]
	if !ok {
		return nil, fmt.Errorf("deployment %s not found", id)
	}

	return deployment, nil
}

// ListDeployments returns all deployments
func (e *Engine) ListDeployments() []*Deployment {
	e.mu.Lock()
	defer e.mu.Unlock()

	deployments := make([]*Deployment, 0, len(e.deployments))
	for _, d := range e.deployments {
		deployments = append(deployments, d)
	}

	return deployments
}

// getServiceLock returns or creates a mutex for a service
func (e *Engine) getServiceLock(serviceName string) *sync.Mutex {
	e.mu.Lock()
	defer e.mu.Unlock()

	lock, ok := e.serviceLocks[serviceName]
	if !ok {
		lock = &sync.Mutex{}
		e.serviceLocks[serviceName] = lock
	}

	return lock
}

// updateDeployment updates a deployment with a function
func (e *Engine) updateDeployment(id string, fn func(*Deployment)) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if deployment, ok := e.deployments[id]; ok {
		fn(deployment)
	}
}

// generateDeploymentID generates a unique deployment ID
func generateDeploymentID() string {
	// Simple implementation - in production you might want something more robust
	return fmt.Sprintf("dep_%d", time.Now().UnixNano())
}
