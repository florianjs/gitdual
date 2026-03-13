package cli

import (
	"fmt"
	"os"

	"github.com/florianjs/gitdual/internal/config"
	"github.com/florianjs/gitdual/internal/git"
	"github.com/florianjs/gitdual/internal/sync"
	"github.com/spf13/cobra"
)

var pushCmd = &cobra.Command{
	Use:   "push [remote] [message]",
	Short: "Push to remotes",
	Long: `Push changes to remotes with optional commit message.

Remotes:
  - private: Push with full commit history
  - public:  Push with single clean commit (excludes filtered files)
  - all:     Push to both (default)

Examples:
  gitdual push                              # Push to all remotes
  gitdual push public                       # Push clean commit to public
  gitdual push private "fix: bug"           # Push to private with message
  gitdual push public "feat: new feature"   # Push clean commit to public
  gitdual push all "release v1.0"           # Push to both with message`,
	Args: cobra.MaximumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		remote := "all"
		message := ""

		if len(args) >= 1 {
			remote = args[0]
			if remote != "public" && remote != "private" && remote != "all" {
				fmt.Fprintf(os.Stderr, "Error: invalid remote '%s'. Use: public, private, or all\n", remote)
				os.Exit(1)
			}
		}

		if len(args) >= 2 {
			message = args[1]
		}

		dryRun, _ := cmd.Flags().GetBool("dry-run")
		force, _ := cmd.Flags().GetBool("force")

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
			fmt.Printf("[DRY-RUN] ")
		}

		if force {
			fmt.Println("Warning: force push enabled")
		}

		switch remote {
		case "private":
			fmt.Printf("Pushing to private remote (full history)")
			if message != "" {
				fmt.Printf(" with message: %q", message)
			}
			fmt.Println()
			n, err := sm.PushPrivate(dryRun, message)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("  Pushed %d commit(s) to private\n", n)

		case "public":
			fmt.Printf("Pushing to public remote (clean commit)")
			if message != "" {
				fmt.Printf(" with message: %q", message)
			} else {
				fmt.Printf(" with default message")
			}
			fmt.Println()
			n, err := sm.PushPublic(dryRun, message, force)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("  Pushed %d commit(s) to public\n", n)

		case "all":
			fmt.Printf("Pushing to all remotes")
			if message != "" {
				fmt.Printf(" with message: %q", message)
			}
			fmt.Println()
			private, public, err := sm.PushWithMessage(dryRun, message)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("  Pushed %d commit(s) to private, %d to public\n", private, public)
		}
	},
}

func init() {
	pushCmd.Flags().BoolP("dry-run", "d", false, "Preview changes without pushing")
	pushCmd.Flags().BoolP("force", "f", false, "Force push")
	rootCmd.AddCommand(pushCmd)
}
