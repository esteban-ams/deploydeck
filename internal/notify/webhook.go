package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// WebhookNotifier sends deployment notifications as a JSON POST to any HTTP endpoint.
type WebhookNotifier struct {
	url     string
	method  string
	headers map[string]string
}

// NewWebhookNotifier creates a WebhookNotifier. If method is empty, "POST" is used.
func NewWebhookNotifier(url, method string, headers map[string]string) *WebhookNotifier {
	if method == "" {
		method = http.MethodPost
	}
	return &WebhookNotifier{url: url, method: method, headers: headers}
}

// Name implements Notifier.
func (w *WebhookNotifier) Name() string { return "webhook" }

// webhookPayload is the JSON body sent to the generic webhook endpoint.
type webhookPayload struct {
	Service      string  `json:"service"`
	Status       string  `json:"status"`
	Image        string  `json:"image"`
	DurationS    float64 `json:"duration_s"`
	DeploymentID string  `json:"deployment_id"`
}

// Send posts the deployment outcome as JSON to the configured URL.
func (w *WebhookNotifier) Send(ctx context.Context, event Event) error {
	payload := webhookPayload{
		Service:      event.Service,
		Status:       event.Status,
		Image:        event.Image,
		DurationS:    event.Duration.Seconds(),
		DeploymentID: event.DeployID,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("webhook: marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, w.method, w.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("webhook: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range w.headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("webhook: send request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook: unexpected status %d", resp.StatusCode)
	}
	return nil
}
