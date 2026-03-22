package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

// serverURLDefault returns the default server URL, preferring the
// DEPLOYDECK_SERVER environment variable when set.
func serverURLDefault() string {
	if v := os.Getenv("DEPLOYDECK_SERVER"); v != "" {
		return v
	}
	return "http://localhost:9000"
}

// serverSecretDefault returns the auth secret from the DEPLOYDECK_SECRET
// environment variable, or an empty string when not set.
func serverSecretDefault() string {
	return os.Getenv("DEPLOYDECK_SECRET")
}

// client is a thin HTTP client for the DeployDeck API.
type client struct {
	server string
	secret string
	http   *http.Client
}

// newClient creates a client that targets the given server URL.
// All requests carry X-DeployDeck-Secret when secret is non-empty.
func newClient(server, secret string) *client {
	return &client{
		server: server,
		secret: secret,
		http:   &http.Client{Timeout: 30 * time.Second},
	}
}

// post sends a POST request with the given body marshalled as JSON.
func (c *client) post(path string, body any) (*http.Response, error) {
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request body: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, c.server+path, bytes.NewReader(buf))
	if err != nil {
		return nil, fmt.Errorf("build POST request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	c.setAuth(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("POST %s: %w", path, err)
	}
	return resp, nil
}

// get sends a GET request.
func (c *client) get(path string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, c.server+path, nil)
	if err != nil {
		return nil, fmt.Errorf("build GET request: %w", err)
	}
	c.setAuth(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", path, err)
	}
	return resp, nil
}

// setAuth adds X-DeployDeck-Secret when a secret is configured.
func (c *client) setAuth(req *http.Request) {
	if c.secret != "" {
		req.Header.Set("X-DeployDeck-Secret", c.secret)
	}
}

// decodeJSON reads the response body into v, closing the body when done.
func decodeJSON(resp *http.Response, v any) error {
	defer resp.Body.Close() //nolint:errcheck
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

// errorMessage extracts the "error" field from a non-2xx response body.
// It falls back to a generic message when the body cannot be decoded.
func errorMessage(resp *http.Response) string {
	defer resp.Body.Close() //nolint:errcheck
	var body struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err == nil && body.Error != "" {
		return body.Error
	}
	return fmt.Sprintf("server returned %s", resp.Status)
}
