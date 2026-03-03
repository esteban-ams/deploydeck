// Package ratelimit provides a per-IP rate limiting middleware for Echo.
// It uses a token-bucket algorithm via golang.org/x/time/rate, creating one
// limiter per remote IP address. Stale limiters are cleaned up periodically to
// prevent unbounded memory growth.
package ratelimit

import (
	"net/http"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"golang.org/x/time/rate"
)

// entry holds a limiter and the last time it was used.
type entry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// Limiter manages per-IP token-bucket limiters.
type Limiter struct {
	mu      sync.Mutex
	entries map[string]*entry
	r       rate.Limit
	burst   int
}

// NewLimiter creates a Limiter that allows requestsPerMinute requests per IP
// with a given burst size.
func NewLimiter(requestsPerMinute, burst int) *Limiter {
	l := &Limiter{
		entries: make(map[string]*entry),
		r:       rate.Limit(float64(requestsPerMinute) / 60.0),
		burst:   burst,
	}
	go l.cleanupLoop()
	return l
}

// allow returns true if the given IP is within its rate limit.
func (l *Limiter) allow(ip string) bool {
	l.mu.Lock()
	e, ok := l.entries[ip]
	if !ok {
		e = &entry{
			limiter: rate.NewLimiter(l.r, l.burst),
		}
		l.entries[ip] = e
	}
	e.lastSeen = time.Now()
	allowed := e.limiter.Allow()
	l.mu.Unlock()
	return allowed
}

// cleanupLoop removes limiters that have not been used for more than 5 minutes.
// It runs every minute in the background for the lifetime of the Limiter.
func (l *Limiter) cleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		l.cleanup(5 * time.Minute)
	}
}

// cleanup evicts entries not seen within ttl. Exported for testing.
func (l *Limiter) cleanup(ttl time.Duration) {
	cutoff := time.Now().Add(-ttl)
	l.mu.Lock()
	for ip, e := range l.entries {
		if e.lastSeen.Before(cutoff) {
			delete(l.entries, ip)
		}
	}
	l.mu.Unlock()
}

// entryCount returns the number of active IP entries. Used in tests.
func (l *Limiter) entryCount() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.entries)
}

// Middleware returns an Echo middleware that enforces the rate limit.
// Requests that exceed the limit receive HTTP 429 with a JSON error body.
func (l *Limiter) Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ip := c.RealIP()
			if !l.allow(ip) {
				return c.JSON(http.StatusTooManyRequests, map[string]string{
					"error": "rate limit exceeded — try again later",
				})
			}
			return next(c)
		}
	}
}
