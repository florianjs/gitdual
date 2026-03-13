package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize GitDual in the current repository",
	Long:  `Initialize GitDual configuration for the current git repository.`,
	Run: func(cmd *cobra.Command, args []string) {
		force, _ := cmd.Flags().GetBool("force")

		configPath := ".gitdual.yml"
		gitignorePath := ".gitignore"
		stateDir := ".gitdual"

		if _, err := os.Stat(configPath); err == nil && !force {
			fmt.Println("GitDual already initialized. Use --force to overwrite.")
			return
		}

		configContent := `version: 1

remotes:
  private: ""
  public: ""

exclude:
  folders:
    - docs/
    - notes/
  files:
    - "*.internal.md"
    - .env*

commit:
  public_message: "auto"
`

		if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating config: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✓ Created %s\n", configPath)

		if err := os.MkdirAll(stateDir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating state directory: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✓ Created %s/\n", stateDir)

		if err := addToGitignore(gitignorePath, []string{configPath, stateDir + "/"}); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not update .gitignore: %v\n", err)
		} else {
			fmt.Printf("✓ Updated %s\n", gitignorePath)
		}

		fmt.Println("\nGitDual initialized! Edit .gitdual.yml to configure your remotes.")
	},
}

func addToGitignore(gitignorePath string, entries []string) error {
	var existingContent string
	var existingEntries map[string]bool = make(map[string]bool)

	file, err := os.Open(gitignorePath)
	if err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			existingContent += line + "\n"
			if line != "" && !strings.HasPrefix(line, "#") {
				existingEntries[line] = true
			}
		}
	}

	var newEntries []string
	for _, entry := range entries {
		if !existingEntries[entry] {
			newEntries = append(newEntries, entry)
		}
	}

	if len(newEntries) == 0 {
		return nil
	}

	var content string
	if existingContent != "" {
		content = existingContent
		if !strings.HasSuffix(existingContent, "\n\n") {
			content += "\n"
		}
	}

	content += "\n# GitDual\n"
	for _, entry := range newEntries {
		content += entry + "\n"
	}

	return os.WriteFile(gitignorePath, []byte(content), 0644)
}

func init() {
	initCmd.Flags().BoolP("force", "f", false, "Overwrite existing configuration")
	rootCmd.AddCommand(initCmd)
}
