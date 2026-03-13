package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/florianjs/gitdual/internal/config"
	"github.com/florianjs/gitdual/internal/git"
	"github.com/florianjs/gitdual/internal/sync"
	"github.com/spf13/cobra"
)

var pullCmd = &cobra.Command{
	Use:   "pull [remote]",
	Short: "Pull from remotes",
	Long: `Pull changes from remotes.

Remotes:
  - private: Pull full history from private remote (default)
  - public:  Pull collaborator changes from public remote into local working tree
  - all:     Pull from both (default)

Examples:
  gitdual pull              # Pull from both remotes
  gitdual pull private      # Pull from private remote only
  gitdual pull public       # Apply public remote changes locally`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		remote := "all"
		if len(args) >= 1 {
			remote = args[0]
			if remote != "public" && remote != "private" && remote != "all" {
				fmt.Fprintf(os.Stderr, "Error: invalid remote '%s'. Use: public, private, or all\n", remote)
				os.Exit(1)
			}
		}

		dryRun, _ := cmd.Flags().GetBool("dry-run")

		repoPath, err := git.FindRepoRoot(".")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		configPath, err := config.FindConfig(repoPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		cfg, err := config.LoadConfig(configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if err := cfg.Validate(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		repo, err := git.NewRepository(repoPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		mirror := git.NewWorkDir(cfg.PublicWorkDirPath(repoPath))
		sm := sync.NewSyncManager(repo, mirror, repoPath, cfg)

		if dryRun {
			fmt.Print("[DRY-RUN] ")
		}

		switch remote {
		case "private":
			fmt.Println("Pulling from private remote...")
			if err := sm.Pull(dryRun); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

		case "public":
			fmt.Println("Pulling from public remote...")
			conflicts, err := sm.PullPublic(dryRun)
			if err != nil {
				if len(conflicts) > 0 {
					fmt.Fprintf(os.Stderr, "Conflict: the following files have local modifications and were also changed upstream.\n")
					fmt.Fprintf(os.Stderr, "Resolve them manually, then run 'gitdual push private' to sync:\n")
					for _, f := range conflicts {
						fmt.Fprintf(os.Stderr, "  %s\n", f)
					}
				} else {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				}
				os.Exit(1)
			}

		case "all":
			fmt.Println("Pulling from all remotes...")
			conflicts, err := sm.PullPublic(dryRun)
			if err != nil {
				if len(conflicts) > 0 {
					fmt.Fprintf(os.Stderr, "Conflict: the following files have local modifications and were also changed upstream.\n")
					fmt.Fprintf(os.Stderr, "Resolve them manually, then run 'gitdual push private' to sync:\n  %s\n", strings.Join(conflicts, "\n  "))
				} else {
					fmt.Fprintf(os.Stderr, "Error pulling from public: %v\n", err)
				}
				os.Exit(1)
			}
			if cfg.Remotes.Private != "" {
				if err := sm.Pull(dryRun); err != nil {
					fmt.Fprintf(os.Stderr, "Error pulling from private: %v\n", err)
					os.Exit(1)
				}
			}
		}

		fmt.Println("Done.")
	},
}

func init() {
	pullCmd.Flags().BoolP("dry-run", "d", false, "Preview changes without applying them")
	rootCmd.AddCommand(pullCmd)
}
