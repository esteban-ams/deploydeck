package deploy

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/esteban-ams/deploydeck/internal/config"
	"github.com/esteban-ams/deploydeck/internal/docker"
	"github.com/esteban-ams/deploydeck/internal/git"
	"github.com/esteban-ams/deploydeck/internal/storage"
)

// Engine orchestrates deployments.
type Engine struct {
	mu           sync.Mutex
	serviceLocks map[string]*sync.Mutex
	store        storage.Storage
	dockerClient *docker.Client
	gitClient    *git.Client
	healthChecker *HealthChecker
	config        *config.Config
}

// NewEngine creates a new deployment engine backed by the given store.
func NewEngine(cfg *config.Config, store storage.Storage) *Engine {
	return &Engine{
		serviceLocks:  make(map[string]*sync.Mutex),
		store:         store,
		dockerClient:  docker.NewClient(),
		gitClient:     git.NewClient(),
		healthChecker: NewHealthChecker(),
		config:        cfg,
	}
}

// DeployOptions holds deployment options.
type DeployOptions struct {
	Image    string
	Tag      string
	CloneURL string // for build mode: repo clone URL
	Branch   string // for build mode: branch name
	Commit   string // for build mode: commit SHA
}

// Deploy initiates a deployment for a service.
// Returns the deployment record immediately and performs deployment asynchronously.
func (e *Engine) Deploy(ctx context.Context, serviceName string, opts DeployOptions) (*storage.Deployment, error) {
	// Check if service exists in config
	svcCfg, ok := e.config.Services[serviceName]
	if !ok {
		return nil, fmt.Errorf("service %s not found in configuration", serviceName)
	}

	// Get or create service lock
	svcLock := e.getServiceLock(serviceName)

	// Create deployment record
	deployment := &storage.Deployment{
		ID:        generateDeploymentID(),
		Service:   serviceName,
		Status:    storage.StatusPending,
		Mode:      string(svcCfg.Mode),
		Image:     opts.Image,
		StartedAt: time.Now(),
	}

	if err := e.store.Save(deployment); err != nil {
		return nil, fmt.Errorf("save deployment record for service %q: %w", serviceName, err)
	}

	// Start deployment in background
	go func() {
		// Lock this service to serialise deployments
		svcLock.Lock()
		defer svcLock.Unlock()

		// Create a timeout context for the entire deployment operation
		deployCtx, cancel := context.WithTimeout(context.Background(), svcCfg.Timeout)
		defer cancel()

		e.executeDeploy(deployCtx, deployment, svcCfg, opts)
	}()

	return deployment, nil
}

// executeDeploy performs the actual deployment.
func (e *Engine) executeDeploy(ctx context.Context, deployment *storage.Deployment, svcCfg config.ServiceConfig, opts DeployOptions) {
	// Update status to running
	e.updateDeployment(deployment.ID, func(d *storage.Deployment) {
		d.Status = storage.StatusRunning
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
				rollbackTag := fmt.Sprintf("%s:rollback-%d", deployment.Service, time.Now().Unix())
				tagErr := e.dockerClient.TagImage(ctx, currentImage, rollbackTag)

				// Persist PreviousImage and (if tagging succeeded) RollbackTag
				// immediately so that SQLite retains rollback info across restarts.
				e.updateDeployment(deployment.ID, func(d *storage.Deployment) {
					d.PreviousImage = currentImage
					if tagErr == nil {
						d.RollbackTag = rollbackTag
					}
				})

				if tagErr != nil {
					log.Printf("Warning: could not tag rollback image for %s: %v", deployment.Service, tagErr)
				} else {
					log.Printf("Saved current image for rollback: %s", currentImage)
					log.Printf("Tagged rollback image: %s -> %s", currentImage, rollbackTag)
				}
			}
		}
	}

	// Refresh local copy so rollback logic below sees the persisted values.
	current, err := e.store.Get(deployment.ID)
	if err != nil {
		log.Printf("Warning: could not re-read deployment %s from store: %v", deployment.ID, err)
		current = deployment
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
			e.handleDeploymentFailure(current, "clone", err, svcCfg)
			return
		}

		// Build image
		log.Printf("Building image for service %s", deployment.Service)
		if err := e.dockerClient.ComposeBuild(ctx, dockerOpts); err != nil {
			e.handleDeploymentFailure(current, "build", err, svcCfg)
			return
		}

	default:
		// Pull mode (default)
		log.Printf("Pulling image for service %s", deployment.Service)
		if err := e.dockerClient.ComposePull(ctx, dockerOpts); err != nil {
			e.handleDeploymentFailure(current, "pull", err, svcCfg)
			return
		}
	}

	// Step 3: Deploy (docker compose up -d) — same for both modes
	log.Printf("Deploying service %s", deployment.Service)
	if err := e.dockerClient.ComposeUp(ctx, dockerOpts); err != nil {
		e.handleDeploymentFailure(current, "up", err, svcCfg)
		return
	}

	// Step 4: Health check — same for both modes
	if svcCfg.HealthCheck.Enabled {
		log.Printf("Running health check for service %s", deployment.Service)
		if err := e.healthChecker.Wait(ctx, svcCfg.HealthCheck); err != nil {
			e.handleDeploymentFailure(current, "health_check", err, svcCfg)
			return
		}
		log.Printf("Health check passed for service %s", deployment.Service)
	}

	// Step 5: Mark deployment as successful
	now := time.Now()
	e.updateDeployment(deployment.ID, func(d *storage.Deployment) {
		d.Status = storage.StatusSuccess
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

// handleDeploymentFailure handles a failed deployment.
func (e *Engine) handleDeploymentFailure(deployment *storage.Deployment, phase string, err error, svcCfg config.ServiceConfig) {
	errMsg := fmt.Sprintf("service %q: deployment failed at %s phase: %v", deployment.Service, phase, err)
	log.Printf("Deployment %s failed: %s", deployment.ID, errMsg)

	finalStatus := storage.StatusFailed

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
			finalStatus = storage.StatusRolledBack
		}
	}

	now := time.Now()
	e.updateDeployment(deployment.ID, func(d *storage.Deployment) {
		d.Status = finalStatus
		d.ErrorMessage = errMsg
		d.CompletedAt = &now
	})
}

// rollback reverts to the previously tagged rollback image.
func (e *Engine) rollback(ctx context.Context, deployment *storage.Deployment, svcCfg config.ServiceConfig) error {
	dockerOpts := docker.ComposeOptions{
		ComposeFile: svcCfg.ComposeFile,
		Service:     svcCfg.ServiceName,
		WorkingDir:  svcCfg.WorkingDir,
		Env:         svcCfg.Env,
	}

	// If we have a rollback tag, restore it as the original image before bringing up
	if deployment.RollbackTag != "" {
		if err := e.dockerClient.TagImage(ctx, deployment.RollbackTag, deployment.PreviousImage); err != nil {
			return fmt.Errorf("failed to restore rollback image tag %q to %q: %w", deployment.RollbackTag, deployment.PreviousImage, err)
		}
		log.Printf("Restored rollback image: %s -> %s", deployment.RollbackTag, deployment.PreviousImage)
	}

	if err := e.dockerClient.ComposeUp(ctx, dockerOpts); err != nil {
		return fmt.Errorf("docker compose up failed during rollback of service %q: %w", deployment.Service, err)
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

// GetDeployment retrieves a deployment by ID.
func (e *Engine) GetDeployment(id string) (*storage.Deployment, error) {
	return e.store.Get(id)
}

// ListDeployments returns all deployments ordered by start time descending.
func (e *Engine) ListDeployments() []*storage.Deployment {
	deployments, err := e.store.List()
	if err != nil {
		log.Printf("Warning: could not list deployments from store: %v", err)
		return nil
	}
	return deployments
}

// getServiceLock returns or creates a mutex for a service.
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

// updateDeployment applies fn to the stored deployment and logs a warning on error.
func (e *Engine) updateDeployment(id string, fn func(*storage.Deployment)) {
	if err := e.store.Update(id, fn); err != nil {
		log.Printf("Warning: could not update deployment %q in store: %v", id, err)
	}
}

// generateDeploymentID generates a unique deployment ID based on nanosecond time.
func generateDeploymentID() string {
	return fmt.Sprintf("dep_%d", time.Now().UnixNano())
}
