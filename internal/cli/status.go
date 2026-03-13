package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current status",
	Long:  `Show the current synchronization status.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Status: Ready")
		fmt.Println("Private remote: not configured")
		fmt.Println("Public remote: not configured")
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
