package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/spf13/cobra"
)

var rollbackCmd = &cobra.Command{
	Use:   "rollback <service>",
	Short: "Roll back a service to its previous image",
	Long:  `Trigger a rollback for the named service by calling POST /api/rollback/:service.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runRollback,
}

func init() {
	rootCmd.AddCommand(rollbackCmd)
}

// rollbackResponse mirrors the server-side RollbackResponse type.
type rollbackResponse struct {
	Status       string `json:"status"`
	DeploymentID string `json:"deployment_id"`
	Service      string `json:"service"`
	Message      string `json:"message,omitempty"`
}

func runRollback(cmd *cobra.Command, args []string) error {
	service := args[0]
	c := newClient(serverURL, serverSecret)

	// POST with an empty body — auth is carried in the header.
	resp, err := c.post("/api/rollback/"+service, struct{}{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Rollback failed: %v\n", err)
		os.Exit(1)
	}

	if resp.StatusCode != http.StatusOK {
		msg := errorMessage(resp)
		fmt.Fprintf(os.Stderr, "Rollback failed: %s (%d)\n", msg, resp.StatusCode)
		os.Exit(1)
	}

	var result rollbackResponse
	if err := decodeJSON(resp, &result); err != nil {
		fmt.Fprintf(os.Stderr, "Rollback failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("deployment_id  %s\n", result.DeploymentID)
	fmt.Printf("status         %s\n", result.Status)
	fmt.Printf("service        %s\n", result.Service)
	if result.Message != "" {
		fmt.Printf("message        %s\n", result.Message)
	}
	return nil
}
