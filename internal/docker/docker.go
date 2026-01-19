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
		return fmt.Errorf("docker compose pull failed: %w: %s", err, stderr.String())
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
		return fmt.Errorf("docker compose up failed: %w: %s", err, stderr.String())
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
		return "", fmt.Errorf("get current image failed: %w: %s", err, stderr.String())
	}

	image := strings.TrimSpace(stdout.String())
	if image == "" {
		return "", fmt.Errorf("container %s not found or has no image", containerName)
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
		return "", fmt.Errorf("get container name failed: %w: %s", err, stderr.String())
	}

	containerID := strings.TrimSpace(stdout.String())
	if containerID == "" {
		return "", fmt.Errorf("no container found for service %s", opts.Service)
	}

	// Get container name from ID
	cmd = exec.CommandContext(ctx, "docker", "inspect",
		"-f", "{{.Name}}", containerID)

	stdout.Reset()
	stderr.Reset()
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("get container name from ID failed: %w: %s", err, stderr.String())
	}

	name := strings.TrimSpace(stdout.String())
	// Remove leading slash from container name
	name = strings.TrimPrefix(name, "/")

	return name, nil
}

// formatEnv converts a map of environment variables to []string format
func formatEnv(env map[string]string) []string {
	result := make([]string, 0, len(env))
	for k, v := range env {
		result = append(result, fmt.Sprintf("%s=%s", k, v))
	}
	return result
}
