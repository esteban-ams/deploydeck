package ipwhitelist

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

// okHandler is a trivial Echo handler that always returns 200.
func okHandler(c echo.Context) error {
	return c.String(http.StatusOK, "ok")
}

// mustNew calls New and fails the test on error.
func mustNew(t *testing.T, entries []string) *Whitelist {
	t.Helper()
	wl, err := New(entries)
	if err != nil {
		t.Fatalf("New(%v): unexpected error: %v", entries, err)
	}
	return wl
}

// ---- New / parsing ----

func TestNew_EmptyList(t *testing.T) {
	t.Parallel()
	wl := mustNew(t, nil)
	if len(wl.nets) != 0 {
		t.Errorf("expected 0 nets, got %d", len(wl.nets))
	}
}

func TestNew_PlainIPv4(t *testing.T) {
	t.Parallel()
	mustNew(t, []string{"10.0.0.1"})
}

func TestNew_PlainIPv6(t *testing.T) {
	t.Parallel()
	mustNew(t, []string{"::1"})
}

func TestNew_CIDRv4(t *testing.T) {
	t.Parallel()
	mustNew(t, []string{"192.168.1.0/24"})
}

func TestNew_CIDRv6(t *testing.T) {
	t.Parallel()
	mustNew(t, []string{"2001:db8::/32"})
}

func TestNew_Mixed(t *testing.T) {
	t.Parallel()
	mustNew(t, []string{"10.0.0.1", "192.168.1.0/24", "::1"})
}

func TestNew_InvalidEntry(t *testing.T) {
	t.Parallel()
	_, err := New([]string{"not-an-ip"})
	if err == nil {
		t.Fatal("expected error for invalid entry")
	}
}

func TestNew_InvalidCIDR(t *testing.T) {
	t.Parallel()
	// "999.999.999.999/24" is not a valid CIDR or plain IP.
	_, err := New([]string{"999.999.999.999/24"})
	if err == nil {
		t.Fatal("expected error for invalid CIDR")
	}
}

// ---- allows ----

func TestAllows_EmptyListPermitsAll(t *testing.T) {
	t.Parallel()
	wl := mustNew(t, nil)
	if !wl.allows("203.0.113.1") {
		t.Error("empty whitelist should allow any IP")
	}
}

func TestAllows_ExactIPv4Match(t *testing.T) {
	t.Parallel()
	wl := mustNew(t, []string{"10.0.0.1"})
	if !wl.allows("10.0.0.1") {
		t.Error("10.0.0.1 should be allowed")
	}
	if wl.allows("10.0.0.2") {
		t.Error("10.0.0.2 should not be allowed")
	}
}

func TestAllows_IPv4CIDR(t *testing.T) {
	t.Parallel()
	wl := mustNew(t, []string{"192.168.1.0/24"})

	allowed := []string{"192.168.1.1", "192.168.1.100", "192.168.1.254"}
	for _, ip := range allowed {
		if !wl.allows(ip) {
			t.Errorf("%s should be allowed by 192.168.1.0/24", ip)
		}
	}

	denied := []string{"192.168.2.1", "10.0.0.1", "8.8.8.8"}
	for _, ip := range denied {
		if wl.allows(ip) {
			t.Errorf("%s should not be allowed by 192.168.1.0/24", ip)
		}
	}
}

func TestAllows_MultipleEntries(t *testing.T) {
	t.Parallel()
	wl := mustNew(t, []string{"10.0.0.1", "192.168.0.0/16"})

	if !wl.allows("10.0.0.1") {
		t.Error("10.0.0.1 should be allowed")
	}
	if !wl.allows("192.168.5.99") {
		t.Error("192.168.5.99 should be allowed by 192.168.0.0/16")
	}
	if wl.allows("172.16.0.1") {
		t.Error("172.16.0.1 should not be allowed")
	}
}

func TestAllows_IPv6(t *testing.T) {
	t.Parallel()
	wl := mustNew(t, []string{"::1"})
	if !wl.allows("::1") {
		t.Error("::1 should be allowed")
	}
	if wl.allows("::2") {
		t.Error("::2 should not be allowed")
	}
}

func TestAllows_UnparseableIP(t *testing.T) {
	t.Parallel()
	wl := mustNew(t, []string{"10.0.0.1"})
	// An unparseable source IP should be denied, not panicked on.
	if wl.allows("not-an-ip") {
		t.Error("unparseable IP should be denied")
	}
}

// ---- Middleware ----

func makeEchoContext(t *testing.T, e *echo.Echo, remoteAddr string) (echo.Context, *httptest.ResponseRecorder) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/deploy/myapp", nil)
	req.RemoteAddr = remoteAddr + ":1234"
	rec := httptest.NewRecorder()
	return e.NewContext(req, rec), rec
}

func TestMiddleware_EmptyWhitelistAllowsAll(t *testing.T) {
	t.Parallel()
	e := echo.New()
	wl := mustNew(t, nil)
	mw := wl.Middleware()

	c, rec := makeEchoContext(t, e, "203.0.113.1")
	if err := mw(okHandler)(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestMiddleware_AllowedIP(t *testing.T) {
	t.Parallel()
	e := echo.New()
	wl := mustNew(t, []string{"10.0.0.1"})
	mw := wl.Middleware()

	c, rec := makeEchoContext(t, e, "10.0.0.1")
	if err := mw(okHandler)(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for allowed IP, got %d", rec.Code)
	}
}

func TestMiddleware_DeniedIP_Returns403(t *testing.T) {
	t.Parallel()
	e := echo.New()
	wl := mustNew(t, []string{"10.0.0.1"})
	mw := wl.Middleware()

	c, rec := makeEchoContext(t, e, "10.0.0.2")
	if err := mw(okHandler)(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for denied IP, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode 403 response body: %v", err)
	}
	if _, ok := body["error"]; !ok {
		t.Error("403 response body should contain an 'error' key")
	}
}

func TestMiddleware_AllowedByCIDR(t *testing.T) {
	t.Parallel()
	e := echo.New()
	wl := mustNew(t, []string{"192.168.1.0/24"})
	mw := wl.Middleware()

	allowed := []string{"192.168.1.1", "192.168.1.50"}
	for _, ip := range allowed {
		c, rec := makeEchoContext(t, e, ip)
		if err := mw(okHandler)(c); err != nil {
			t.Fatalf("%s: unexpected error: %v", ip, err)
		}
		if rec.Code != http.StatusOK {
			t.Errorf("%s: expected 200, got %d", ip, rec.Code)
		}
	}
}

func TestMiddleware_DeniedOutsideCIDR(t *testing.T) {
	t.Parallel()
	e := echo.New()
	wl := mustNew(t, []string{"192.168.1.0/24"})
	mw := wl.Middleware()

	c, rec := makeEchoContext(t, e, "192.168.2.1")
	if err := mw(okHandler)(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for IP outside CIDR, got %d", rec.Code)
	}
}

func TestMiddleware_TableDriven(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		entries    []string
		remoteAddr string
		wantStatus int
	}{
		{
			name:       "no whitelist allows any IP",
			entries:    nil,
			remoteAddr: "1.2.3.4",
			wantStatus: http.StatusOK,
		},
		{
			name:       "exact match allowed",
			entries:    []string{"10.0.0.5"},
			remoteAddr: "10.0.0.5",
			wantStatus: http.StatusOK,
		},
		{
			name:       "exact match denied",
			entries:    []string{"10.0.0.5"},
			remoteAddr: "10.0.0.6",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "CIDR match allowed",
			entries:    []string{"10.10.0.0/16"},
			remoteAddr: "10.10.255.1",
			wantStatus: http.StatusOK,
		},
		{
			name:       "CIDR match denied",
			entries:    []string{"10.10.0.0/16"},
			remoteAddr: "10.11.0.1",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "multiple entries second matches",
			entries:    []string{"10.0.0.1", "172.16.0.0/12"},
			remoteAddr: "172.20.0.50",
			wantStatus: http.StatusOK,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := echo.New()
			wl := mustNew(t, tc.entries)
			mw := wl.Middleware()

			c, rec := makeEchoContext(t, e, tc.remoteAddr)
			if err := mw(okHandler)(c); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if rec.Code != tc.wantStatus {
				t.Errorf("expected %d, got %d", tc.wantStatus, rec.Code)
			}
		})
	}
}
