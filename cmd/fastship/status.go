package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/esteban-ams/fastship/internal/config"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show configured services and their settings",
	RunE:  runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	fmt.Println()
	fmt.Printf("FastShip %s\n", version)
	fmt.Println()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "SERVICE\tMODE\tBRANCH\tHEALTH CHECK\tROLLBACK\tTIMEOUT")
	fmt.Fprintln(w, "-------\t----\t------\t------------\t--------\t-------")

	for name, svc := range cfg.Services {
		branch := "-"
		if svc.Mode == config.DeployModeBuild {
			branch = svc.Branch
		}

		healthCheck := "disabled"
		if svc.HealthCheck.Enabled {
			healthCheck = svc.HealthCheck.URL
		}

		rollback := "disabled"
		if svc.Rollback.Enabled {
			rollback = fmt.Sprintf("enabled (keep %d)", svc.Rollback.KeepImages)
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			name,
			svc.Mode,
			branch,
			healthCheck,
			rollback,
			svc.Timeout,
		)
	}
	w.Flush()

	fmt.Println()
	fmt.Printf("Server:  %s:%d\n", cfg.Server.Host, cfg.Server.Port)
	fmt.Printf("Config:  %s\n", configPath)
	fmt.Println()

	return nil
}
