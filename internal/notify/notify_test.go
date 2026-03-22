package notify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// captureServer starts an httptest.Server that records the last request body
// and returns the given status code. Caller must close the server.
func captureServer(t *testing.T, statusCode int) (*httptest.Server, *[]byte) {
	t.Helper()
	var captured []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("captureServer: read body: %v", err)
		}
		captured = data
		w.WriteHeader(statusCode)
	}))
	return srv, &captured
}

var testEvent = Event{
	Service:   "myapp",
	Status:    "success",
	Image:     "nginx:alpine",
	Duration:  45 * time.Second,
	DeployID:  "dep_123",
	ServerURL: "http://localhost:9000",
}

// --- SlackNotifier ---

func TestSlackNotifier_SendSuccess(t *testing.T) {
	t.Parallel()
	srv, captured := captureServer(t, http.StatusOK)
	defer srv.Close()

	n := NewSlackNotifier(srv.URL)
	if err := n.Send(context.Background(), testEvent); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(*captured, &payload); err != nil {
		t.Fatalf("response body is not valid JSON: %v", err)
	}
	blocks, ok := payload["blocks"].([]any)
	if !ok || len(blocks) == 0 {
		t.Fatal("expected non-empty 'blocks' array in Slack payload")
	}
}

func TestSlackNotifier_SendIncludesLogsLink(t *testing.T) {
	t.Parallel()
	srv, captured := captureServer(t, http.StatusOK)
	defer srv.Close()

	n := NewSlackNotifier(srv.URL)
	if err := n.Send(context.Background(), testEvent); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	body := string(*captured)
	wantURL := "http://localhost:9000/api/deployments/dep_123/logs"
	if !containsString(body, wantURL) {
		t.Errorf("expected logs URL %q in Slack payload, got: %s", wantURL, body)
	}
}

func TestSlackNotifier_SendNoLogsLinkWhenServerURLEmpty(t *testing.T) {
	t.Parallel()
	srv, captured := captureServer(t, http.StatusOK)
	defer srv.Close()

	event := testEvent
	event.ServerURL = ""

	n := NewSlackNotifier(srv.URL)
	if err := n.Send(context.Background(), event); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	body := string(*captured)
	if containsString(body, "/api/deployments/") {
		t.Errorf("expected no logs link when ServerURL is empty, got: %s", body)
	}
}

func TestSlackNotifier_ReturnsErrorOnNon2xx(t *testing.T) {
	t.Parallel()
	srv, _ := captureServer(t, http.StatusInternalServerError)
	defer srv.Close()

	n := NewSlackNotifier(srv.URL)
	if err := n.Send(context.Background(), testEvent); err == nil {
		t.Fatal("expected error for non-2xx status, got nil")
	}
}

func TestSlackNotifier_Name(t *testing.T) {
	t.Parallel()
	n := NewSlackNotifier("http://example.com")
	if n.Name() != "slack" {
		t.Errorf("expected name %q, got %q", "slack", n.Name())
	}
}

// --- DiscordNotifier ---

func TestDiscordNotifier_SendSuccess(t *testing.T) {
	t.Parallel()
	srv, captured := captureServer(t, http.StatusNoContent)
	defer srv.Close()

	n := NewDiscordNotifier(srv.URL)
	if err := n.Send(context.Background(), testEvent); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(*captured, &payload); err != nil {
		t.Fatalf("response body is not valid JSON: %v", err)
	}
	embeds, ok := payload["embeds"].([]any)
	if !ok || len(embeds) == 0 {
		t.Fatal("expected non-empty 'embeds' array in Discord payload")
	}
}

func TestDiscordNotifier_EmbedColorByStatus(t *testing.T) {
	t.Parallel()
	cases := []struct {
		status string
		color  float64 // JSON numbers decode to float64
	}{
		{"success", float64(0x2ecc71)},
		{"failed", float64(0xe74c3c)},
		{"rolled_back", float64(0xe67e22)},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.status, func(t *testing.T) {
			t.Parallel()
			srv, captured := captureServer(t, http.StatusNoContent)
			defer srv.Close()

			event := testEvent
			event.Status = tc.status
			n := NewDiscordNotifier(srv.URL)
			if err := n.Send(context.Background(), event); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			var payload map[string]any
			if err := json.Unmarshal(*captured, &payload); err != nil {
				t.Fatalf("invalid JSON: %v", err)
			}
			embeds := payload["embeds"].([]any)
			embed := embeds[0].(map[string]any)
			if embed["color"] != tc.color {
				t.Errorf("status %q: expected color %v, got %v", tc.status, tc.color, embed["color"])
			}
		})
	}
}

func TestDiscordNotifier_ReturnsErrorOnNon2xx(t *testing.T) {
	t.Parallel()
	srv, _ := captureServer(t, http.StatusBadRequest)
	defer srv.Close()

	n := NewDiscordNotifier(srv.URL)
	if err := n.Send(context.Background(), testEvent); err == nil {
		t.Fatal("expected error for non-2xx status, got nil")
	}
}

func TestDiscordNotifier_Name(t *testing.T) {
	t.Parallel()
	n := NewDiscordNotifier("http://example.com")
	if n.Name() != "discord" {
		t.Errorf("expected name %q, got %q", "discord", n.Name())
	}
}

// --- WebhookNotifier ---

func TestWebhookNotifier_SendSuccess(t *testing.T) {
	t.Parallel()
	srv, captured := captureServer(t, http.StatusOK)
	defer srv.Close()

	n := NewWebhookNotifier(srv.URL, http.MethodPost, nil)
	if err := n.Send(context.Background(), testEvent); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var payload webhookPayload
	if err := json.Unmarshal(*captured, &payload); err != nil {
		t.Fatalf("response body is not valid JSON: %v", err)
	}
	if payload.Service != "myapp" {
		t.Errorf("expected service %q, got %q", "myapp", payload.Service)
	}
	if payload.Status != "success" {
		t.Errorf("expected status %q, got %q", "success", payload.Status)
	}
	if payload.DeploymentID != "dep_123" {
		t.Errorf("expected deployment_id %q, got %q", "dep_123", payload.DeploymentID)
	}
	if payload.DurationS != 45.0 {
		t.Errorf("expected duration_s 45.0, got %f", payload.DurationS)
	}
}

func TestWebhookNotifier_CustomHeaders(t *testing.T) {
	t.Parallel()
	var gotHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("X-Custom-Token")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	n := NewWebhookNotifier(srv.URL, http.MethodPost, map[string]string{
		"X-Custom-Token": "secret-token",
	})
	if err := n.Send(context.Background(), testEvent); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotHeader != "secret-token" {
		t.Errorf("expected header value %q, got %q", "secret-token", gotHeader)
	}
}

func TestWebhookNotifier_DefaultsToPost(t *testing.T) {
	t.Parallel()
	var gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Empty method should default to POST.
	n := NewWebhookNotifier(srv.URL, "", nil)
	if err := n.Send(context.Background(), testEvent); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("expected method POST, got %q", gotMethod)
	}
}

func TestWebhookNotifier_ReturnsErrorOnNon2xx(t *testing.T) {
	t.Parallel()
	srv, _ := captureServer(t, http.StatusUnauthorized)
	defer srv.Close()

	n := NewWebhookNotifier(srv.URL, http.MethodPost, nil)
	if err := n.Send(context.Background(), testEvent); err == nil {
		t.Fatal("expected error for non-2xx status, got nil")
	}
}

func TestWebhookNotifier_Name(t *testing.T) {
	t.Parallel()
	n := NewWebhookNotifier("http://example.com", "", nil)
	if n.Name() != "webhook" {
		t.Errorf("expected name %q, got %q", "webhook", n.Name())
	}
}

// --- Dispatcher ---

func TestDispatcher_FiresAllNotifiers(t *testing.T) {
	t.Parallel()

	var slackCalled, discordCalled atomic.Bool

	slackSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slackCalled.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	defer slackSrv.Close()

	discordSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		discordCalled.Store(true)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer discordSrv.Close()

	d := NewDispatcher(
		NewSlackNotifier(slackSrv.URL),
		NewDiscordNotifier(discordSrv.URL),
	)
	d.Notify(testEvent)

	// Give goroutines time to complete.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if slackCalled.Load() && discordCalled.Load() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !slackCalled.Load() {
		t.Error("Slack notifier was not called")
	}
	if !discordCalled.Load() {
		t.Error("Discord notifier was not called")
	}
}

func TestDispatcher_IsNonBlocking(t *testing.T) {
	t.Parallel()

	// Use a server that deliberately delays longer than any reasonable synchronous call.
	slowSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer slowSrv.Close()

	d := NewDispatcher(NewWebhookNotifier(slowSrv.URL, http.MethodPost, nil))

	start := time.Now()
	d.Notify(testEvent)
	elapsed := time.Since(start)

	// Notify must return well before the slow server responds.
	if elapsed > 100*time.Millisecond {
		t.Errorf("Notify blocked for %v; expected near-instant return", elapsed)
	}
}

func TestDispatcher_NoopWhenEmpty(t *testing.T) {
	t.Parallel()
	d := NewDispatcher()
	// Must not panic and must return immediately.
	d.Notify(testEvent)
}

// containsString reports whether s contains substr.
func containsString(s, substr string) bool {
	return strings.Contains(s, substr)
}
