package docker

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Client handles Docker and Docker Compose operations
type Client struct{}

// NewClient creates a new Docker client
func NewClient() *Client {
	return &Client{}
}

// ComposeOptions holds options for docker compose commands
type ComposeOptions struct {
	ComposeFile string
	Service     string
	WorkingDir  string
	Env         map[string]string
}

// ComposePull executes 'docker compose pull' for a service
func (c *Client) ComposePull(ctx context.Context, opts ComposeOptions) error {
	args := []string{"compose"}

	if opts.ComposeFile != "" {
		args = append(args, "-f", opts.ComposeFile)
	}

	args = append(args, "pull", opts.Service)

	cmd := exec.CommandContext(ctx, "docker", args...)
	if opts.WorkingDir != "" {
		cmd.Dir = opts.WorkingDir
	}

	// Set environment variables
	if len(opts.Env) > 0 {
		cmd.Env = append(cmd.Environ(), formatEnv(opts.Env)...)
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker compose pull failed for service %q (compose file: %q): %w\nstderr: %s",
			opts.Service, opts.ComposeFile, err, stderr.String())
	}

	return nil
}

// ComposeBuild executes 'docker compose build' for a service
func (c *Client) ComposeBuild(ctx context.Context, opts ComposeOptions) error {
	args := []string{"compose"}

	if opts.ComposeFile != "" {
		args = append(args, "-f", opts.ComposeFile)
	}

	args = append(args, "build", opts.Service)

	cmd := exec.CommandContext(ctx, "docker", args...)
	if opts.WorkingDir != "" {
		cmd.Dir = opts.WorkingDir
	}

	// Set environment variables
	if len(opts.Env) > 0 {
		cmd.Env = append(cmd.Environ(), formatEnv(opts.Env)...)
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker compose build failed for service %q (compose file: %q): %w\nstderr: %s",
			opts.Service, opts.ComposeFile, err, stderr.String())
	}

	return nil
}

// ComposeUp executes 'docker compose up -d' for a service
func (c *Client) ComposeUp(ctx context.Context, opts ComposeOptions) error {
	args := []string{"compose"}

	if opts.ComposeFile != "" {
		args = append(args, "-f", opts.ComposeFile)
	}

	args = append(args, "up", "-d", opts.Service)

	cmd := exec.CommandContext(ctx, "docker", args...)
	if opts.WorkingDir != "" {
		cmd.Dir = opts.WorkingDir
	}

	// Set environment variables
	if len(opts.Env) > 0 {
		cmd.Env = append(cmd.Environ(), formatEnv(opts.Env)...)
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker compose up failed for service %q (compose file: %q): %w\nstderr: %s",
			opts.Service, opts.ComposeFile, err, stderr.String())
	}

	return nil
}

// GetCurrentImage returns the current image for a container
// This is used to save the current image before deployment for rollback
func (c *Client) GetCurrentImage(ctx context.Context, containerName string) (string, error) {
	cmd := exec.CommandContext(ctx, "docker", "inspect",
		"-f", "{{.Config.Image}}", containerName)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("docker inspect failed for container %q: %w\nstderr: %s", containerName, err, stderr.String())
	}

	image := strings.TrimSpace(stdout.String())
	if image == "" {
		return "", fmt.Errorf("container %q has no image configured (is it running?)", containerName)
	}

	return image, nil
}

// GetContainerName returns the container name for a compose service
func (c *Client) GetContainerName(ctx context.Context, opts ComposeOptions) (string, error) {
	args := []string{"compose"}

	if opts.ComposeFile != "" {
		args = append(args, "-f", opts.ComposeFile)
	}

	args = append(args, "ps", "-q", opts.Service)

	cmd := exec.CommandContext(ctx, "docker", args...)
	if opts.WorkingDir != "" {
		cmd.Dir = opts.WorkingDir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("docker compose ps failed for service %q (compose file: %q): %w\nstderr: %s",
			opts.Service, opts.ComposeFile, err, stderr.String())
	}

	containerID := strings.TrimSpace(stdout.String())
	if containerID == "" {
		return "", fmt.Errorf("no running container found for compose service %q — is it started?", opts.Service)
	}

	// Get container name from ID
	cmd = exec.CommandContext(ctx, "docker", "inspect",
		"-f", "{{.Name}}", containerID)

	stdout.Reset()
	stderr.Reset()
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("docker inspect failed for container ID %q: %w\nstderr: %s", containerID, err, stderr.String())
	}

	name := strings.TrimSpace(stdout.String())
	// Remove leading slash from container name
	name = strings.TrimPrefix(name, "/")

	return name, nil
}

// TagImage tags a Docker image with a new name.
func (c *Client) TagImage(ctx context.Context, source, target string) error {
	cmd := exec.CommandContext(ctx, "docker", "tag", source, target)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker tag %q -> %q failed: %w\nstderr: %s", source, target, err, stderr.String())
	}

	return nil
}

// RemoveImage removes a Docker image by name/tag.
func (c *Client) RemoveImage(ctx context.Context, image string) error {
	cmd := exec.CommandContext(ctx, "docker", "rmi", image)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker rmi %q failed: %w\nstderr: %s", image, err, stderr.String())
	}

	return nil
}

// ListImagesByFilter lists images matching a reference filter pattern.
func (c *Client) ListImagesByFilter(ctx context.Context, reference string) ([]string, error) {
	cmd := exec.CommandContext(ctx, "docker", "images",
		"--filter", fmt.Sprintf("reference=%s", reference),
		"--format", "{{.Repository}}:{{.Tag}}")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("docker images --filter reference=%q failed: %w\nstderr: %s", reference, err, stderr.String())
	}

	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	var images []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			images = append(images, line)
		}
	}

	return images, nil
}

// BuilderPrune removes unused Docker build cache.
func (c *Client) BuilderPrune(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "docker", "builder", "prune", "-f")

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker builder prune failed: %w\nstderr: %s", err, stderr.String())
	}

	return nil
}

// formatEnv converts a map of environment variables to []string format
func formatEnv(env map[string]string) []string {
	result := make([]string, 0, len(env))
	for k, v := range env {
		result = append(result, fmt.Sprintf("%s=%s", k, v))
	}
	return result
}
