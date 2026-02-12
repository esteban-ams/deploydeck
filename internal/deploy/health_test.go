package deploy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/esteban-ams/fastship/internal/config"
)

func TestHealthCheck_HealthyImmediately(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	hc := NewHealthChecker()
	err := hc.Wait(context.Background(), config.HealthCheckConfig{
		Enabled:  true,
		URL:      srv.URL,
		Timeout:  5 * time.Second,
		Interval: 100 * time.Millisecond,
		Retries:  3,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHealthCheck_HealthyAfterRetries(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	hc := NewHealthChecker()
	err := hc.Wait(context.Background(), config.HealthCheckConfig{
		Enabled:  true,
		URL:      srv.URL,
		Timeout:  5 * time.Second,
		Interval: 100 * time.Millisecond,
		Retries:  10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := attempts.Load()
	if got < 3 {
		t.Errorf("expected at least 3 attempts, got %d", got)
	}
}

func TestHealthCheck_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	hc := NewHealthChecker()
	err := hc.Wait(context.Background(), config.HealthCheckConfig{
		Enabled:  true,
		URL:      srv.URL,
		Timeout:  300 * time.Millisecond,
		Interval: 100 * time.Millisecond,
		Retries:  100,
	})
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestHealthCheck_ContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel after a short delay
	go func() {
		time.Sleep(150 * time.Millisecond)
		cancel()
	}()

	hc := NewHealthChecker()
	err := hc.Wait(ctx, config.HealthCheckConfig{
		Enabled:  true,
		URL:      srv.URL,
		Timeout:  10 * time.Second,
		Interval: 100 * time.Millisecond,
		Retries:  100,
	})
	if err == nil {
		t.Fatal("expected error from context cancellation")
	}
}

func TestHealthCheck_Disabled(t *testing.T) {
	hc := NewHealthChecker()
	err := hc.Wait(context.Background(), config.HealthCheckConfig{
		Enabled: false,
	})
	if err != nil {
		t.Fatalf("disabled health check should return nil, got: %v", err)
	}
}
