package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version of FastShip",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("FastShip %s\n", version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
