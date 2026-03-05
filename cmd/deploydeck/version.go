package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version of DeployDeck",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("DeployDeck %s\n", version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
