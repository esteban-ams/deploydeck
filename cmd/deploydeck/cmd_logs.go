package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"
)

var logsFollow bool

var logsCmd = &cobra.Command{
	Use:   "logs <service>",
	Short: "Fetch logs for the latest deployment of a service",
	Long: `Print the logs for the most recent deployment of the named service.

With --follow, the command polls every 2 seconds and prints new lines as they
appear. Polling stops automatically when the deployment reaches a terminal state
(success, failed, or rolled_back).`,
	Args: cobra.ExactArgs(1),
	RunE: runLogs,
}

func init() {
	logsCmd.Flags().BoolVarP(&logsFollow, "follow", "f", false, "Poll for new log lines until the deployment completes")
	rootCmd.AddCommand(logsCmd)
}

// logsResponse mirrors the server-side DeploymentLogsResponse type.
type logsResponse struct {
	DeploymentID string   `json:"deployment_id"`
	Service      string   `json:"service"`
	Status       string   `json:"status"`
	Logs         []string `json:"logs"`
}

// deploymentsListResponse mirrors the wrapper returned by GET /api/deployments.
type deploymentsListResponse struct {
	Deployments []deploymentInfo `json:"deployments"`
}

// deploymentInfo mirrors the server-side DeploymentInfo type.
type deploymentInfo struct {
	ID          string  `json:"id"`
	Service     string  `json:"service"`
	Status      string  `json:"status"`
	Mode        string  `json:"mode,omitempty"`
	Image       string  `json:"image,omitempty"`
	RollbackTag string  `json:"rollback_tag,omitempty"`
	StartedAt   string  `json:"started_at"`
	CompletedAt *string `json:"completed_at,omitempty"`
	Error       string  `json:"error,omitempty"`
}

// isTerminal returns true when the status string represents a final deployment state.
func isTerminal(status string) bool {
	switch status {
	case "success", "failed", "rolled_back":
		return true
	}
	return false
}

func runLogs(cmd *cobra.Command, args []string) error {
	service := args[0]
	c := newClient(serverURL, serverSecret)

	// Resolve the latest deployment ID for this service.
	deploymentID, err := latestDeploymentID(c, service)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Logs failed: %v\n", err)
		os.Exit(1)
	}

	printed := 0

	fetchAndPrint := func() (string, error) {
		resp, err := c.get("/api/deployments/" + deploymentID + "/logs")
		if err != nil {
			return "", fmt.Errorf("fetch logs: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			msg := errorMessage(resp)
			return "", fmt.Errorf("%s (%d)", msg, resp.StatusCode)
		}

		var result logsResponse
		if err := decodeJSON(resp, &result); err != nil {
			return "", err
		}

		// Print only the lines we have not yet shown.
		for _, line := range result.Logs[printed:] {
			fmt.Println(line)
		}
		printed = len(result.Logs)

		return result.Status, nil
	}

	status, err := fetchAndPrint()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Logs failed: %v\n", err)
		os.Exit(1)
	}

	if !logsFollow || isTerminal(status) {
		return nil
	}

	// Poll until a terminal state is reached.
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		status, err = fetchAndPrint()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Logs failed: %v\n", err)
			os.Exit(1)
		}
		if isTerminal(status) {
			break
		}
	}

	return nil
}

// latestDeploymentID calls GET /api/deployments and returns the ID of the most
// recent deployment for the given service.
func latestDeploymentID(c *client, service string) (string, error) {
	resp, err := c.get("/api/deployments")
	if err != nil {
		return "", fmt.Errorf("list deployments: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("list deployments: %s (%d)", errorMessage(resp), resp.StatusCode)
	}

	var result deploymentsListResponse
	if err := decodeJSON(resp, &result); err != nil {
		return "", fmt.Errorf("list deployments: %w", err)
	}

	// The server returns deployments ordered by started_at descending, so the
	// first match for this service is the most recent.
	for _, d := range result.Deployments {
		if d.Service == service {
			return d.ID, nil
		}
	}

	return "", fmt.Errorf("no deployments found for service %q", service)
}
