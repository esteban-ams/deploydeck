package git

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
)

// Client handles Git operations
type Client struct{}

// NewClient creates a new Git client
func NewClient() *Client {
	return &Client{}
}

// CloneOptions holds options for git clone
type CloneOptions struct {
	URL        string // HTTPS clone URL
	Branch     string // branch to clone
	WorkingDir string // destination path
	Token      string // auth token for private repos
}

// Clone performs a shallow clone of the repository into WorkingDir.
// If WorkingDir already exists, it is removed first for a clean state.
func (c *Client) Clone(ctx context.Context, opts CloneOptions) error {
	// Remove existing directory for a clean clone
	if _, err := os.Stat(opts.WorkingDir); err == nil {
		if err := os.RemoveAll(opts.WorkingDir); err != nil {
			return fmt.Errorf("failed to clean working directory: %w", err)
		}
	}

	// Inject token into clone URL if provided
	cloneURL := opts.URL
	if opts.Token != "" {
		var err error
		cloneURL, err = injectToken(cloneURL, opts.Token)
		if err != nil {
			return fmt.Errorf("failed to inject auth token: %w", err)
		}
	}

	args := []string{"clone", "--depth", "1"}
	if opts.Branch != "" {
		args = append(args, "--branch", opts.Branch)
	}
	args = append(args, cloneURL, opts.WorkingDir)

	cmd := exec.CommandContext(ctx, "git", args...)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone failed: %w: %s", err, stderr.String())
	}

	return nil
}

// injectToken adds authentication to an HTTPS clone URL.
// GitHub: https://x-access-token:<token>@github.com/user/repo.git
// GitLab: https://oauth2:<token>@gitlab.com/user/repo.git
// Other:  https://token:<token>@host/user/repo.git
func injectToken(rawURL, token string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid clone URL: %w", err)
	}

	host := strings.ToLower(u.Hostname())

	var username string
	switch {
	case strings.Contains(host, "github"):
		username = "x-access-token"
	case strings.Contains(host, "gitlab"):
		username = "oauth2"
	default:
		username = "token"
	}

	u.User = url.UserPassword(username, token)
	return u.String(), nil
}
