package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Synchronize with remotes",
	Long:  `Synchronize changes between private and public remotes.`,
	Run: func(cmd *cobra.Command, args []string) {
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		fmt.Printf("Syncing... (dry-run: %v)\n", dryRun)
	},
}

func init() {
	syncCmd.Flags().BoolP("dry-run", "d", false, "Perform a dry run without making changes")
	rootCmd.AddCommand(syncCmd)
}
