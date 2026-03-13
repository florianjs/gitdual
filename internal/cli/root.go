package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "gitdual",
	Short: "Manage dual git remotes with ease",
	Long: `GitDual enables maintaining a single local repository
with both private and public remotes, automatically filtering
sensitive content and managing commit history.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := RunTUI(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func Execute() error {
	return rootCmd.Execute()
}
