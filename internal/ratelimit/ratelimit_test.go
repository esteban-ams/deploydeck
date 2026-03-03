package ratelimit

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
)

// okHandler is a trivial Echo handler that always returns 200.
func okHandler(c echo.Context) error {
	return c.String(http.StatusOK, "ok")
}

func TestAllow_WithinLimit(t *testing.T) {
	t.Parallel()
	// 60 req/min == 1/s, burst 5 → first 5 requests must be allowed immediately.
	l := NewLimiter(60, 5)
	for i := range 5 {
		if !l.allow("192.0.2.1") {
			t.Fatalf("request %d should have been allowed (within burst)", i+1)
		}
	}
}

func TestAllow_ExceedsLimit(t *testing.T) {
	t.Parallel()
	// 1 req/min, burst 1 → second request must be denied.
	l := NewLimiter(1, 1)
	if !l.allow("192.0.2.1") {
		t.Fatal("first request should be allowed")
	}
	if l.allow("192.0.2.1") {
		t.Fatal("second request should be denied (limit exceeded)")
	}
}

func TestAllow_DifferentIPsAreIndependent(t *testing.T) {
	t.Parallel()
	l := NewLimiter(1, 1)

	// First request from each IP should be allowed.
	if !l.allow("192.0.2.1") {
		t.Fatal("192.0.2.1 first request should be allowed")
	}
	if !l.allow("192.0.2.2") {
		t.Fatal("192.0.2.2 first request should be allowed")
	}

	// Second request from the same IP should be denied.
	if l.allow("192.0.2.1") {
		t.Fatal("192.0.2.1 second request should be denied")
	}
}

func TestCleanup_RemovesStaleEntries(t *testing.T) {
	t.Parallel()
	l := NewLimiter(60, 5)

	// Prime the limiter with two IPs.
	l.allow("192.0.2.1")
	l.allow("192.0.2.2")

	if l.entryCount() != 2 {
		t.Fatalf("expected 2 entries before cleanup, got %d", l.entryCount())
	}

	// Wind back lastSeen for one IP so it appears stale.
	l.mu.Lock()
	l.entries["192.0.2.1"].lastSeen = time.Now().Add(-10 * time.Minute)
	l.mu.Unlock()

	l.cleanup(5 * time.Minute)

	if l.entryCount() != 1 {
		t.Fatalf("expected 1 entry after cleanup, got %d", l.entryCount())
	}
	l.mu.Lock()
	_, stillPresent := l.entries["192.0.2.2"]
	l.mu.Unlock()
	if !stillPresent {
		t.Fatal("192.0.2.2 should still be present after cleanup")
	}
}

func TestMiddleware_AllowsRequestWithinLimit(t *testing.T) {
	t.Parallel()
	e := echo.New()
	l := NewLimiter(60, 5)
	mw := l.Middleware()

	req := httptest.NewRequest(http.MethodPost, "/api/deploy/myapp", nil)
	req.RemoteAddr = "192.0.2.10:1234"
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handler := mw(okHandler)
	if err := handler(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestMiddleware_Returns429WhenLimitExceeded(t *testing.T) {
	t.Parallel()
	e := echo.New()
	// 1 req/min, burst 1 → second request must be rejected.
	l := NewLimiter(1, 1)
	mw := l.Middleware()

	newRequest := func() (echo.Context, *httptest.ResponseRecorder) {
		req := httptest.NewRequest(http.MethodPost, "/api/deploy/myapp", nil)
		req.RemoteAddr = "192.0.2.20:5678"
		rec := httptest.NewRecorder()
		return e.NewContext(req, rec), rec
	}

	// First request: allowed.
	c1, rec1 := newRequest()
	if err := mw(okHandler)(c1); err != nil {
		t.Fatalf("first request unexpected error: %v", err)
	}
	if rec1.Code != http.StatusOK {
		t.Errorf("first request: expected 200, got %d", rec1.Code)
	}

	// Second request: rate limited.
	c2, rec2 := newRequest()
	if err := mw(okHandler)(c2); err != nil {
		t.Fatalf("second request unexpected error: %v", err)
	}
	if rec2.Code != http.StatusTooManyRequests {
		t.Errorf("second request: expected 429, got %d", rec2.Code)
	}

	// Verify the response body contains an "error" key.
	var body map[string]string
	if err := json.NewDecoder(rec2.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode 429 response body: %v", err)
	}
	if _, ok := body["error"]; !ok {
		t.Error("429 response body should contain an 'error' key")
	}
}

func TestMiddleware_DifferentIPsNotAffected(t *testing.T) {
	t.Parallel()
	e := echo.New()
	// burst 1 so we exhaust the first IP immediately
	l := NewLimiter(1, 1)
	mw := l.Middleware()

	makeReq := func(ip string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/deploy/myapp", nil)
		req.RemoteAddr = ip + ":1234"
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		_ = mw(okHandler)(c) //nolint — error returned as JSON, not Go error
		return rec
	}

	// Exhaust ip1.
	makeReq("10.0.0.1")

	// ip1 should now be rate limited.
	rec := makeReq("10.0.0.1")
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("ip1 second request: expected 429, got %d", rec.Code)
	}

	// ip2 should still be allowed.
	rec = makeReq("10.0.0.2")
	if rec.Code != http.StatusOK {
		t.Errorf("ip2 first request: expected 200, got %d", rec.Code)
	}
}
