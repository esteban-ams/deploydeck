package webhook

import (
	"encoding/json"
	"fmt"
	"strings"
)

// PushEvent represents the relevant fields from a git push webhook payload
type PushEvent struct {
	CloneURL string // HTTPS clone URL
	Branch   string // branch name (without refs/heads/ prefix)
	Commit   string // commit SHA
}

// ParseGitHubPush extracts push event data from a GitHub webhook payload.
// GitHub push payload reference: https://docs.github.com/en/webhooks/webhook-events-and-payloads#push
func ParseGitHubPush(body []byte) (*PushEvent, error) {
	var payload struct {
		Ref        string `json:"ref"`
		After      string `json:"after"`
		Repository struct {
			CloneURL string `json:"clone_url"`
		} `json:"repository"`
	}

	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse GitHub push payload as JSON: %w", err)
	}

	if payload.Ref == "" || payload.Repository.CloneURL == "" {
		return nil, fmt.Errorf("GitHub push payload is missing required fields: 'ref' and 'repository.clone_url' must both be present; " +
			"ensure the webhook is sending a push event (not a ping or other event type)")
	}

	return &PushEvent{
		CloneURL: payload.Repository.CloneURL,
		Branch:   extractBranch(payload.Ref),
		Commit:   payload.After,
	}, nil
}

// ParseGitLabPush extracts push event data from a GitLab webhook payload.
// GitLab push payload reference: https://docs.gitlab.com/ee/user/project/integrations/webhook_events.html#push-events
func ParseGitLabPush(body []byte) (*PushEvent, error) {
	var payload struct {
		Ref     string `json:"ref"`
		After   string `json:"after"`
		Project struct {
			HttpURL string `json:"http_url"`
		} `json:"project"`
	}

	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse GitLab push payload as JSON: %w", err)
	}

	if payload.Ref == "" || payload.Project.HttpURL == "" {
		return nil, fmt.Errorf("GitLab push payload is missing required fields: 'ref' and 'project.http_url' must both be present; " +
			"ensure the webhook is sending a push event (not a tag or other event type)")
	}

	return &PushEvent{
		CloneURL: payload.Project.HttpURL,
		Branch:   extractBranch(payload.Ref),
		Commit:   payload.After,
	}, nil
}

// extractBranch converts "refs/heads/main" to "main"
func extractBranch(ref string) string {
	return strings.TrimPrefix(ref, "refs/heads/")
}
