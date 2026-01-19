package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// AuthMethod represents different webhook authentication methods
type AuthMethod string

const (
	AuthMethodGitHub   AuthMethod = "github"    // X-Hub-Signature-256
	AuthMethodGitLab   AuthMethod = "gitlab"    // X-GitLab-Token
	AuthMethodFastShip AuthMethod = "fastship"  // X-FastShip-Secret
)

// Verifier handles webhook authentication
type Verifier struct {
	secret string
}

// NewVerifier creates a new webhook verifier
func NewVerifier(secret string) *Verifier {
	return &Verifier{secret: secret}
}

// Verify checks the authenticity of a webhook request
// It supports multiple authentication methods:
// - GitHub: X-Hub-Signature-256 with HMAC-SHA256
// - GitLab: X-GitLab-Token with simple token comparison
// - FastShip: X-FastShip-Secret with either HMAC or simple secret
func (v *Verifier) Verify(headers map[string]string, body []byte) error {
	// Try GitHub-style HMAC signature
	if sig := headers["X-Hub-Signature-256"]; sig != "" {
		if v.verifyHMAC(body, sig) {
			return nil
		}
		return fmt.Errorf("invalid GitHub signature")
	}

	// Try GitLab token
	if token := headers["X-GitLab-Token"]; token != "" {
		if token == v.secret {
			return nil
		}
		return fmt.Errorf("invalid GitLab token")
	}

	// Try FastShip secret (supports both HMAC and simple secret)
	if secret := headers["X-FastShip-Secret"]; secret != "" {
		// Check if it's an HMAC signature (starts with "sha256=")
		if strings.HasPrefix(secret, "sha256=") {
			if v.verifyHMAC(body, secret) {
				return nil
			}
			return fmt.Errorf("invalid FastShip HMAC signature")
		}
		// Simple secret comparison
		if secret == v.secret {
			return nil
		}
		return fmt.Errorf("invalid FastShip secret")
	}

	return fmt.Errorf("no authentication header found")
}

// verifyHMAC verifies an HMAC-SHA256 signature
// Signature format: "sha256=<hex_encoded_hmac>"
func (v *Verifier) verifyHMAC(body []byte, signature string) bool {
	// Remove "sha256=" prefix if present
	signature = strings.TrimPrefix(signature, "sha256=")

	// Compute expected HMAC
	mac := hmac.New(sha256.New, []byte(v.secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))

	// Use constant-time comparison to prevent timing attacks
	return hmac.Equal([]byte(expected), []byte(signature))
}
