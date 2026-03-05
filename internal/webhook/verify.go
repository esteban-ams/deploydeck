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
	AuthMethodGitHub     AuthMethod = "github"      // X-Hub-Signature-256
	AuthMethodGitLab     AuthMethod = "gitlab"      // X-GitLab-Token
	AuthMethodDeployDeck AuthMethod = "deploydeck"  // X-DeployDeck-Secret
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
// - DeployDeck: X-DeployDeck-Secret with either HMAC or simple secret
func (v *Verifier) Verify(headers map[string]string, body []byte) (AuthMethod, error) {
	// Try GitHub-style HMAC signature
	if sig := headers["X-Hub-Signature-256"]; sig != "" {
		if v.verifyHMAC(body, sig) {
			return AuthMethodGitHub, nil
		}
		return "", fmt.Errorf("GitHub HMAC signature verification failed (X-Hub-Signature-256): " +
			"ensure the webhook secret in DeployDeck matches the secret configured in your GitHub repository settings")
	}

	// Try GitLab token
	if token := headers["X-GitLab-Token"]; token != "" {
		if token == v.secret {
			return AuthMethodGitLab, nil
		}
		return "", fmt.Errorf("GitLab token verification failed (X-GitLab-Token): " +
			"ensure the token in your GitLab webhook settings matches auth.webhook_secret in config.yaml")
	}

	// Try DeployDeck secret (supports both HMAC and simple secret)
	if secret := headers["X-DeployDeck-Secret"]; secret != "" {
		// Check if it's an HMAC signature (starts with "sha256=")
		if strings.HasPrefix(secret, "sha256=") {
			if v.verifyHMAC(body, secret) {
				return AuthMethodDeployDeck, nil
			}
			return "", fmt.Errorf("DeployDeck HMAC signature verification failed (X-DeployDeck-Secret): " +
				"the sha256= signature does not match; ensure the request body was not modified in transit " +
				"and the signing secret matches auth.webhook_secret in config.yaml")
		}
		// Simple secret comparison
		if secret == v.secret {
			return AuthMethodDeployDeck, nil
		}
		return "", fmt.Errorf("DeployDeck secret verification failed (X-DeployDeck-Secret): " +
			"the provided secret does not match auth.webhook_secret in config.yaml")
	}

	return "", fmt.Errorf("no authentication header found: provide one of " +
		"X-Hub-Signature-256 (GitHub), X-GitLab-Token (GitLab), or X-DeployDeck-Secret (DeployDeck)")
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
