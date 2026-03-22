package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// statusEmoji returns the emoji prefix for a given deployment status.
func statusEmoji(status string) string {
	switch status {
	case "success":
		return "✅"
	case "failed":
		return "❌"
	case "rolled_back":
		return "🔄"
	default:
		return "ℹ️"
	}
}

// statusLabel returns a human-readable label for a given deployment status.
func statusLabel(status string) string {
	switch status {
	case "success":
		return "Deploy succeeded"
	case "failed":
		return "Deploy failed"
	case "rolled_back":
		return "Deploy rolled back"
	default:
		return "Deploy " + status
	}
}

// SlackNotifier sends deployment notifications via a Slack Incoming Webhook.
type SlackNotifier struct {
	webhookURL string
}

// NewSlackNotifier creates a SlackNotifier that posts to the given webhook URL.
func NewSlackNotifier(webhookURL string) *SlackNotifier {
	return &SlackNotifier{webhookURL: webhookURL}
}

// Name implements Notifier.
func (s *SlackNotifier) Name() string { return "slack" }

// Send posts a Block Kit message to the Slack webhook URL.
func (s *SlackNotifier) Send(ctx context.Context, event Event) error {
	header := fmt.Sprintf("%s %s — %s", statusEmoji(event.Status), statusLabel(event.Status), event.Service)

	fields := []map[string]any{
		{"type": "mrkdwn", "text": fmt.Sprintf("*Image*\n%s", event.Image)},
		{"type": "mrkdwn", "text": fmt.Sprintf("*Duration*\n%s", event.Duration.Round(1e6).String())},
		{"type": "mrkdwn", "text": fmt.Sprintf("*Deployment ID*\n%s", event.DeployID)},
	}

	if event.ServerURL != "" {
		logsURL := fmt.Sprintf("%s/api/deployments/%s/logs", event.ServerURL, event.DeployID)
		fields = append(fields, map[string]any{
			"type": "mrkdwn",
			"text": fmt.Sprintf("*Logs*\n<%s|View logs>", logsURL),
		})
	}

	payload := map[string]any{
		"blocks": []map[string]any{
			{
				"type": "header",
				"text": map[string]any{
					"type": "plain_text",
					"text": header,
				},
			},
			{
				"type":   "section",
				"fields": fields,
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("slack: marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("slack: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("slack: send request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("slack: unexpected status %d", resp.StatusCode)
	}
	return nil
}
