package webhook

import (
	"testing"
)

func TestParseGitHubPush_Valid(t *testing.T) {
	body := []byte(`{
		"ref": "refs/heads/main",
		"after": "abc123",
		"repository": {
			"clone_url": "https://github.com/user/repo.git"
		}
	}`)

	event, err := ParseGitHubPush(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event.Branch != "main" {
		t.Errorf("expected branch 'main', got %q", event.Branch)
	}
	if event.CloneURL != "https://github.com/user/repo.git" {
		t.Errorf("expected clone URL, got %q", event.CloneURL)
	}
	if event.Commit != "abc123" {
		t.Errorf("expected commit 'abc123', got %q", event.Commit)
	}
}

func TestParseGitHubPush_MissingRef(t *testing.T) {
	body := []byte(`{
		"repository": {
			"clone_url": "https://github.com/user/repo.git"
		}
	}`)

	_, err := ParseGitHubPush(body)
	if err == nil {
		t.Fatal("expected error for missing ref")
	}
}

func TestParseGitHubPush_MissingCloneURL(t *testing.T) {
	body := []byte(`{
		"ref": "refs/heads/main",
		"repository": {}
	}`)

	_, err := ParseGitHubPush(body)
	if err == nil {
		t.Fatal("expected error for missing clone_url")
	}
}

func TestParseGitHubPush_InvalidJSON(t *testing.T) {
	_, err := ParseGitHubPush([]byte(`not json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseGitLabPush_Valid(t *testing.T) {
	body := []byte(`{
		"ref": "refs/heads/develop",
		"after": "def456",
		"project": {
			"http_url": "https://gitlab.com/user/repo.git"
		}
	}`)

	event, err := ParseGitLabPush(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event.Branch != "develop" {
		t.Errorf("expected branch 'develop', got %q", event.Branch)
	}
	if event.CloneURL != "https://gitlab.com/user/repo.git" {
		t.Errorf("expected clone URL, got %q", event.CloneURL)
	}
	if event.Commit != "def456" {
		t.Errorf("expected commit 'def456', got %q", event.Commit)
	}
}

func TestParseGitLabPush_MissingRef(t *testing.T) {
	body := []byte(`{
		"project": {
			"http_url": "https://gitlab.com/user/repo.git"
		}
	}`)

	_, err := ParseGitLabPush(body)
	if err == nil {
		t.Fatal("expected error for missing ref")
	}
}

func TestParseGitLabPush_MissingHttpURL(t *testing.T) {
	body := []byte(`{
		"ref": "refs/heads/main",
		"project": {}
	}`)

	_, err := ParseGitLabPush(body)
	if err == nil {
		t.Fatal("expected error for missing http_url")
	}
}

func TestParseGitLabPush_InvalidJSON(t *testing.T) {
	_, err := ParseGitLabPush([]byte(`{broken`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestExtractBranch(t *testing.T) {
	tests := []struct {
		ref      string
		expected string
	}{
		{"refs/heads/main", "main"},
		{"refs/heads/develop", "develop"},
		{"refs/heads/feature/foo", "feature/foo"},
		{"refs/heads/release/v1.0", "release/v1.0"},
		{"main", "main"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.ref, func(t *testing.T) {
			got := extractBranch(tt.ref)
			if got != tt.expected {
				t.Errorf("extractBranch(%q) = %q, want %q", tt.ref, got, tt.expected)
			}
		})
	}
}
