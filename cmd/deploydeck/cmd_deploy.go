package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/spf13/cobra"
)

var (
	deployImage string
	deployTag   string
)

var deployCmd = &cobra.Command{
	Use:   "deploy <service>",
	Short: "Trigger a deployment for a service",
	Long: `Trigger a deployment for the named service by calling POST /api/deploy/:service.

In pull mode, pass --image and --tag to specify which image to deploy.
In build mode, the server uses the repository configured in config.yaml.`,
	Args: cobra.ExactArgs(1),
	RunE: runDeploy,
}

func init() {
	deployCmd.Flags().StringVar(&deployImage, "image", "", "Image name to deploy (pull mode)")
	deployCmd.Flags().StringVar(&deployTag, "tag", "", "Image tag to deploy (pull mode)")
	rootCmd.AddCommand(deployCmd)
}

// deployRequest mirrors the server-side DeployRequest type.
type deployRequest struct {
	Image string `json:"image,omitempty"`
	Tag   string `json:"tag,omitempty"`
}

// deployResponse mirrors the server-side DeployResponse type.
type deployResponse struct {
	Status       string `json:"status"`
	DeploymentID string `json:"deployment_id"`
	Service      string `json:"service"`
}

func runDeploy(cmd *cobra.Command, args []string) error {
	service := args[0]
	c := newClient(serverURL, serverSecret)

	body := deployRequest{
		Image: deployImage,
		Tag:   deployTag,
	}

	resp, err := c.post("/api/deploy/"+service, body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Deploy failed: %v\n", err)
		os.Exit(1)
	}

	if resp.StatusCode != http.StatusOK {
		msg := errorMessage(resp)
		fmt.Fprintf(os.Stderr, "Deploy failed: %s (%d)\n", msg, resp.StatusCode)
		os.Exit(1)
	}

	var result deployResponse
	if err := decodeJSON(resp, &result); err != nil {
		fmt.Fprintf(os.Stderr, "Deploy failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("deployment_id  %s\n", result.DeploymentID)
	fmt.Printf("status         %s\n", result.Status)
	fmt.Printf("service        %s\n", result.Service)
	return nil
}
