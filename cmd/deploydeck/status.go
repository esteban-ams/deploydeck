package main

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the latest deployment status for each service",
	Long: `Call GET /api/deployments on the remote DeployDeck server and display
the most recent deployment per service in a table.`,
	RunE: runRemoteStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runRemoteStatus(cmd *cobra.Command, args []string) error {
	c := newClient(serverURL, serverSecret)

	resp, err := c.get("/api/deployments")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Status failed: %v\n", err)
		os.Exit(1)
	}

	if resp.StatusCode != 200 {
		msg := errorMessage(resp)
		fmt.Fprintf(os.Stderr, "Status failed: %s (%d)\n", msg, resp.StatusCode)
		os.Exit(1)
	}

	var result deploymentsListResponse
	if err := decodeJSON(resp, &result); err != nil {
		fmt.Fprintf(os.Stderr, "Status failed: %v\n", err)
		os.Exit(1)
	}

	// Keep only the most recent deployment per service. The server already
	// returns deployments in started_at descending order, so the first entry
	// per service name is the latest.
	seen := make(map[string]bool)
	var latest []deploymentInfo
	for _, d := range result.Deployments {
		if !seen[d.Service] {
			seen[d.Service] = true
			latest = append(latest, d)
		}
	}

	if len(latest) == 0 {
		fmt.Println("No deployments found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "SERVICE\tSTATUS\tIMAGE\tAGE")
	fmt.Fprintln(w, "-------\t------\t-----\t---")

	for _, d := range latest {
		age := formatAge(d.StartedAt)
		image := d.Image
		if image == "" {
			image = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", d.Service, d.Status, image, age)
	}
	w.Flush()

	return nil
}

// formatAge parses an RFC3339 timestamp and returns a human-readable age string
// such as "5s ago", "2m ago", or "1h ago".
func formatAge(ts string) string {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return ts
	}
	d := time.Since(t)

	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	default:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
}
