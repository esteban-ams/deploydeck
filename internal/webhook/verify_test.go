package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func computeHMAC(secret, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestVerify_GitHubHMAC_Valid(t *testing.T) {
	v := NewVerifier("my-secret")
	body := []byte(`{"ref":"refs/heads/main"}`)
	sig := computeHMAC("my-secret", string(body))

	method, err := v.Verify(map[string]string{
		"X-Hub-Signature-256": sig,
	}, body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if method != AuthMethodGitHub {
		t.Errorf("expected auth method 'github', got %q", method)
	}
}

func TestVerify_GitHubHMAC_Invalid(t *testing.T) {
	v := NewVerifier("my-secret")
	body := []byte(`{"ref":"refs/heads/main"}`)

	_, err := v.Verify(map[string]string{
		"X-Hub-Signature-256": "sha256=invalid",
	}, body)
	if err == nil {
		t.Fatal("expected error for invalid GitHub HMAC")
	}
}

func TestVerify_GitLabToken_Valid(t *testing.T) {
	v := NewVerifier("my-secret")

	method, err := v.Verify(map[string]string{
		"X-GitLab-Token": "my-secret",
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if method != AuthMethodGitLab {
		t.Errorf("expected auth method 'gitlab', got %q", method)
	}
}

func TestVerify_GitLabToken_Invalid(t *testing.T) {
	v := NewVerifier("my-secret")

	_, err := v.Verify(map[string]string{
		"X-GitLab-Token": "wrong-secret",
	}, nil)
	if err == nil {
		t.Fatal("expected error for invalid GitLab token")
	}
}

func TestVerify_DeployDeckSecret_Valid(t *testing.T) {
	v := NewVerifier("my-secret")

	method, err := v.Verify(map[string]string{
		"X-DeployDeck-Secret": "my-secret",
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if method != AuthMethodDeployDeck {
		t.Errorf("expected auth method 'deploydeck', got %q", method)
	}
}

func TestVerify_DeployDeckSecret_Invalid(t *testing.T) {
	v := NewVerifier("my-secret")

	_, err := v.Verify(map[string]string{
		"X-DeployDeck-Secret": "wrong-secret",
	}, nil)
	if err == nil {
		t.Fatal("expected error for invalid DeployDeck secret")
	}
}

func TestVerify_DeployDeckHMAC_Valid(t *testing.T) {
	v := NewVerifier("my-secret")
	body := []byte(`{"image":"myapp:latest"}`)
	sig := computeHMAC("my-secret", string(body))

	method, err := v.Verify(map[string]string{
		"X-DeployDeck-Secret": sig,
	}, body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if method != AuthMethodDeployDeck {
		t.Errorf("expected auth method 'deploydeck', got %q", method)
	}
}

func TestVerify_DeployDeckHMAC_Invalid(t *testing.T) {
	v := NewVerifier("my-secret")

	_, err := v.Verify(map[string]string{
		"X-DeployDeck-Secret": "sha256=invalid",
	}, []byte(`body`))
	if err == nil {
		t.Fatal("expected error for invalid DeployDeck HMAC")
	}
}

func TestVerify_NoAuthHeader(t *testing.T) {
	v := NewVerifier("my-secret")

	_, err := v.Verify(map[string]string{}, nil)
	if err == nil {
		t.Fatal("expected error when no auth header present")
	}
}

func TestVerify_GitHubPriority(t *testing.T) {
	// When both GitHub and DeployDeck headers are present, GitHub should be checked first
	v := NewVerifier("my-secret")
	body := []byte(`test`)
	sig := computeHMAC("my-secret", string(body))

	method, err := v.Verify(map[string]string{
		"X-Hub-Signature-256": sig,
		"X-DeployDeck-Secret": "my-secret",
	}, body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if method != AuthMethodGitHub {
		t.Errorf("expected GitHub to take priority, got %q", method)
	}
}
