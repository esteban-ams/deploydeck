// Package ipwhitelist provides an Echo middleware that restricts access to a
// set of allowed IP addresses and CIDR ranges. When the whitelist is empty all
// IPs are permitted, so existing deployments with no whitelist configured are
// unaffected.
package ipwhitelist

import (
	"fmt"
	"net"
	"net/http"

	"github.com/labstack/echo/v4"
)

// Whitelist holds the parsed allow-list used by the middleware.
type Whitelist struct {
	// nets holds pre-parsed CIDR ranges (single IPs are stored as /32 or /128).
	nets []*net.IPNet
}

// New builds a Whitelist from a slice of IP addresses and CIDR strings.
// Each entry must be either a plain IP (e.g. "10.0.0.1") or a CIDR range
// (e.g. "192.168.1.0/24"). Returns an error if any entry cannot be parsed.
// An empty entries slice produces a Whitelist that allows all IPs.
func New(entries []string) (*Whitelist, error) {
	wl := &Whitelist{
		nets: make([]*net.IPNet, 0, len(entries)),
	}
	for _, entry := range entries {
		// Try CIDR notation first.
		_, ipNet, err := net.ParseCIDR(entry)
		if err == nil {
			wl.nets = append(wl.nets, ipNet)
			continue
		}
		// Fall back to plain IP — wrap it as a host-only CIDR.
		ip := net.ParseIP(entry)
		if ip == nil {
			return nil, fmt.Errorf("ipwhitelist: %q is not a valid IP address or CIDR range", entry)
		}
		bits := 32
		if ip.To4() == nil {
			bits = 128
		}
		wl.nets = append(wl.nets, &net.IPNet{
			IP:   ip.Mask(net.CIDRMask(bits, bits)),
			Mask: net.CIDRMask(bits, bits),
		})
	}
	return wl, nil
}

// allows reports whether the given IP address is permitted by the whitelist.
// If the whitelist is empty every IP is allowed.
func (wl *Whitelist) allows(ip string) bool {
	if len(wl.nets) == 0 {
		return true
	}
	parsed := net.ParseIP(ip)
	if parsed == nil {
		// Unparseable source IP — deny to be safe.
		return false
	}
	for _, network := range wl.nets {
		if network.Contains(parsed) {
			return true
		}
	}
	return false
}

// Middleware returns an Echo middleware that rejects requests whose source IP
// is not in the whitelist with HTTP 403. When the whitelist is empty the
// middleware is a no-op pass-through.
func (wl *Whitelist) Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if len(wl.nets) == 0 {
				return next(c)
			}
			ip := c.RealIP()
			if !wl.allows(ip) {
				return c.JSON(http.StatusForbidden, map[string]string{
					"error": fmt.Sprintf("IP %q is not allowed", ip),
				})
			}
			return next(c)
		}
	}
}
