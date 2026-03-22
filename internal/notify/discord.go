package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// discordColor returns the embed color integer for a given deployment status.
func discordColor(status string) int {
	switch status {
	case "success":
		return 0x2ecc71 // green
	case "failed":
		return 0xe74c3c // red
	case "rolled_back":
		return 0xe67e22 // orange
	default:
		return 0x95a5a6 // grey
	}
}

// DiscordNotifier sends deployment notifications via a Discord webhook.
type DiscordNotifier struct {
	webhookURL string
}

// NewDiscordNotifier creates a DiscordNotifier that posts to the given webhook URL.
func NewDiscordNotifier(webhookURL string) *DiscordNotifier {
	return &DiscordNotifier{webhookURL: webhookURL}
}

// Name implements Notifier.
func (d *DiscordNotifier) Name() string { return "discord" }

// Send posts an embed to the Discord webhook URL.
func (d *DiscordNotifier) Send(ctx context.Context, event Event) error {
	title := fmt.Sprintf("%s %s — %s", statusEmoji(event.Status), statusLabel(event.Status), event.Service)

	fields := []map[string]any{
		{"name": "Service", "value": event.Service, "inline": true},
		{"name": "Image", "value": event.Image, "inline": true},
		{"name": "Duration", "value": event.Duration.Round(1e6).String(), "inline": true},
		{"name": "Deployment ID", "value": event.DeployID, "inline": false},
	}

	embed := map[string]any{
		"title":  title,
		"color":  discordColor(event.Status),
		"fields": fields,
	}

	payload := map[string]any{
		"embeds": []map[string]any{embed},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("discord: marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("discord: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("discord: send request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("discord: unexpected status %d", resp.StatusCode)
	}
	return nil
}
