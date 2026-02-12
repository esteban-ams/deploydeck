package deploy

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/esteban-ams/fastship/internal/config"
	"github.com/esteban-ams/fastship/internal/docker"
	"github.com/esteban-ams/fastship/internal/git"
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
	Mode          string
	Image         string
	PreviousImage string
	RollbackTag   string
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
	gitClient     *git.Client
	healthChecker *HealthChecker
	config        *config.Config
}

// NewEngine creates a new deployment engine
func NewEngine(cfg *config.Config) *Engine {
	return &Engine{
		serviceLocks:  make(map[string]*sync.Mutex),
		deployments:   make(map[string]*Deployment),
		dockerClient:  docker.NewClient(),
		gitClient:     git.NewClient(),
		healthChecker: NewHealthChecker(),
		config:        cfg,
	}
}

// DeployOptions holds deployment options
type DeployOptions struct {
	Image    string
	Tag      string
	CloneURL string // for build mode: repo clone URL
	Branch   string // for build mode: branch name
	Commit   string // for build mode: commit SHA
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
		Mode:      string(svcCfg.Mode),
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

		// Create a timeout context for the entire deployment operation
		ctx, cancel := context.WithTimeout(context.Background(), svcCfg.Timeout)
		defer cancel()

		e.executeDeploy(ctx, deployment, svcCfg, opts)
	}()

	return deployment, nil
}

// executeDeploy performs the actual deployment
func (e *Engine) executeDeploy(ctx context.Context, deployment *Deployment, svcCfg config.ServiceConfig, opts DeployOptions) {
	// Update status to running
	e.updateDeployment(deployment.ID, func(d *Deployment) {
		d.Status = StatusRunning
	})

	log.Printf("Starting deployment %s for service %s (mode: %s)", deployment.ID, deployment.Service, svcCfg.Mode)

	dockerOpts := docker.ComposeOptions{
		ComposeFile: svcCfg.ComposeFile,
		Service:     svcCfg.ServiceName,
		WorkingDir:  svcCfg.WorkingDir,
		Env:         svcCfg.Env,
	}

	// Step 1: Save current image and create rollback tag
	if svcCfg.Rollback.Enabled {
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

				// Tag current image as rollback snapshot
				rollbackTag := fmt.Sprintf("%s:rollback-%d", deployment.Service, time.Now().Unix())
				if err := e.dockerClient.TagImage(ctx, currentImage, rollbackTag); err != nil {
					log.Printf("Warning: could not tag rollback image for %s: %v", deployment.Service, err)
				} else {
					deployment.RollbackTag = rollbackTag
					log.Printf("Tagged rollback image: %s -> %s", currentImage, rollbackTag)
				}
			}
		}
	}

	// Step 2: Mode-specific operations
	switch svcCfg.Mode {
	case config.DeployModeBuild:
		// Clone repository
		log.Printf("Cloning repository for service %s (branch: %s)", deployment.Service, opts.Branch)
		cloneOpts := git.CloneOptions{
			URL:        opts.CloneURL,
			Branch:     opts.Branch,
			WorkingDir: svcCfg.WorkingDir,
			Token:      svcCfg.CloneToken,
		}
		if err := e.gitClient.Clone(ctx, cloneOpts); err != nil {
			e.handleDeploymentFailure(deployment, "clone", err, svcCfg)
			return
		}

		// Build image
		log.Printf("Building image for service %s", deployment.Service)
		if err := e.dockerClient.ComposeBuild(ctx, dockerOpts); err != nil {
			e.handleDeploymentFailure(deployment, "build", err, svcCfg)
			return
		}

	default:
		// Pull mode (default)
		log.Printf("Pulling image for service %s", deployment.Service)
		if err := e.dockerClient.ComposePull(ctx, dockerOpts); err != nil {
			e.handleDeploymentFailure(deployment, "pull", err, svcCfg)
			return
		}
	}

	// Step 3: Deploy (docker compose up -d) — same for both modes
	log.Printf("Deploying service %s", deployment.Service)
	if err := e.dockerClient.ComposeUp(ctx, dockerOpts); err != nil {
		e.handleDeploymentFailure(deployment, "up", err, svcCfg)
		return
	}

	// Step 4: Health check — same for both modes
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

	// Step 6: Clean up old rollback tags
	if svcCfg.Rollback.Enabled && svcCfg.Rollback.KeepImages > 0 {
		e.cleanupOldRollbackTags(ctx, deployment.Service, svcCfg.Rollback.KeepImages)
	}

	// Step 7: Auto-prune build cache if enabled (build mode only, non-blocking)
	if svcCfg.Mode == config.DeployModeBuild && svcCfg.PruneAfterBuild {
		log.Printf("Pruning Docker build cache for service %s", deployment.Service)
		if err := e.dockerClient.BuilderPrune(ctx); err != nil {
			log.Printf("Warning: build cache prune failed for %s: %v", deployment.Service, err)
		} else {
			log.Printf("Build cache pruned successfully for service %s", deployment.Service)
		}
	}
}

// handleDeploymentFailure handles a failed deployment
func (e *Engine) handleDeploymentFailure(deployment *Deployment, phase string, err error, svcCfg config.ServiceConfig) {
	errMsg := fmt.Sprintf("deployment failed at %s phase: %v", phase, err)
	log.Printf("Deployment %s failed: %s", deployment.ID, errMsg)

	finalStatus := StatusFailed

	// Attempt rollback if enabled and we have a previous image
	if svcCfg.Rollback.Enabled && deployment.PreviousImage != "" {
		log.Printf("Attempting rollback for deployment %s", deployment.ID)

		// Rollback gets its own timeout (not the expired deployment context)
		rollbackCtx, rollbackCancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer rollbackCancel()

		if rbErr := e.rollback(rollbackCtx, deployment, svcCfg); rbErr != nil {
			errMsg = fmt.Sprintf("%s; rollback failed: %v", errMsg, rbErr)
			log.Printf("Rollback failed for deployment %s: %v", deployment.ID, rbErr)
		} else {
			log.Printf("Rollback successful for deployment %s", deployment.ID)
			finalStatus = StatusRolledBack
		}
	}

	now := time.Now()
	e.updateDeployment(deployment.ID, func(d *Deployment) {
		d.Status = finalStatus
		d.ErrorMessage = errMsg
		d.CompletedAt = &now
	})
}

// rollback reverts to the previously tagged rollback image
func (e *Engine) rollback(ctx context.Context, deployment *Deployment, svcCfg config.ServiceConfig) error {
	dockerOpts := docker.ComposeOptions{
		ComposeFile: svcCfg.ComposeFile,
		Service:     svcCfg.ServiceName,
		WorkingDir:  svcCfg.WorkingDir,
		Env:         svcCfg.Env,
	}

	// If we have a rollback tag, restore it as the original image before bringing up
	if deployment.RollbackTag != "" {
		if err := e.dockerClient.TagImage(ctx, deployment.RollbackTag, deployment.PreviousImage); err != nil {
			return fmt.Errorf("rollback tag restore failed: %w", err)
		}
		log.Printf("Restored rollback image: %s -> %s", deployment.RollbackTag, deployment.PreviousImage)
	}

	if err := e.dockerClient.ComposeUp(ctx, dockerOpts); err != nil {
		return fmt.Errorf("rollback compose up failed: %w", err)
	}

	return nil
}

// cleanupOldRollbackTags removes old rollback image tags, keeping the most recent N.
func (e *Engine) cleanupOldRollbackTags(ctx context.Context, serviceName string, keepImages int) {
	if keepImages <= 0 {
		return
	}

	pattern := fmt.Sprintf("%s:rollback-*", serviceName)
	images, err := e.dockerClient.ListImagesByFilter(ctx, pattern)
	if err != nil {
		log.Printf("Warning: could not list rollback images for %s: %v", serviceName, err)
		return
	}

	// Docker lists images newest-first. Keep the first N, remove the rest.
	if len(images) <= keepImages {
		return
	}

	for _, img := range images[keepImages:] {
		if err := e.dockerClient.RemoveImage(ctx, img); err != nil {
			log.Printf("Warning: could not remove old rollback image %s: %v", img, err)
		} else {
			log.Printf("Removed old rollback image: %s", img)
		}
	}
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
