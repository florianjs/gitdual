package cli

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/florianjs/gitdual/internal/config"
	"github.com/florianjs/gitdual/internal/exclude"
	"github.com/spf13/cobra"
)

var excludeCmd = &cobra.Command{
	Use:   "exclude",
	Short: "Select files and folders to exclude from public remote",
	Long: `Open an interactive TUI to browse your project tree and select
files or folders that should be excluded from the public remote.

Navigation:
  ↑/k     Move up
  ↓/j     Move down
  Enter   Expand/collapse folder
  Space   Toggle exclusion
  s       Save changes
  q       Quit`,
	Run: func(cmd *cobra.Command, args []string) {
		repoPath, err := filepath.Abs(".")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		cfgPath, err := config.FindConfig(repoPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		cfg, err := config.LoadConfig(cfgPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		p := tea.NewProgram(
			newExcludeModel(repoPath, cfgPath, cfg),
			tea.WithAltScreen(),
		)
		_, err = p.Run()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

type fileNode struct {
	name     string
	path     string
	isDir    bool
	expanded bool
	selected bool
	children []*fileNode
}

type excludeModel struct {
	root       *fileNode
	flatNodes  []*fileNode
	cursor     int
	configPath string
	config     *config.Config
	status     string
	saved      bool
	width      int
	height     int
	embedded   bool
	done       bool
}

func newExcludeModel(repoPath, configPath string, cfg *config.Config) *excludeModel {
	root := buildFileTree(repoPath, cfg)
	flat := flattenTree(root, 0)

	return &excludeModel{
		root:       root,
		flatNodes:  flat,
		configPath: configPath,
		config:     cfg,
		status:     "Use Space to toggle exclusion, Enter to expand folders",
	}
}

func newExcludeModelEmbedded(repoPath, configPath string, cfg *config.Config) *excludeModel {
	m := newExcludeModel(repoPath, configPath, cfg)
	m.embedded = true
	return m
}

func buildFileTree(rootPath string, cfg *config.Config) *fileNode {
	absRoot, _ := filepath.Abs(rootPath)
	rootName := filepath.Base(absRoot)

	root := &fileNode{
		name:     rootName,
		path:     ".",
		isDir:    true,
		expanded: true,
	}

	excludedFolders := make(map[string]bool)
	for _, f := range cfg.Exclude.Folders {
		excludedFolders[strings.TrimSuffix(f, "/")] = true
	}
	excludedFiles := make(map[string]bool)
	for _, f := range cfg.Exclude.Files {
		excludedFiles[f] = true
	}

	matcher := exclude.NewMatcher(cfg.Exclude.PrivateSuffix)

	nodeMap := map[string]*fileNode{".": root}

	filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		relPath, _ := filepath.Rel(rootPath, path)
		if relPath == "." {
			return nil
		}

		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") && d.Name() != ".gitdual" {
				return fs.SkipDir
			}
			if d.Name() == ".git" {
				return fs.SkipDir
			}
		} else {
			if strings.HasPrefix(d.Name(), ".") {
				return nil
			}
		}

		if matcher.ShouldExclude(relPath) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}

		parentPath := filepath.Dir(relPath)
		if parentPath == "." {
			parentPath = "."
		}

		parent := nodeMap[parentPath]
		if parent == nil {
			return nil
		}

		node := &fileNode{
			name:  d.Name(),
			path:  relPath,
			isDir: d.IsDir(),
		}

		if d.IsDir() {
			node.expanded = false
			_, node.selected = excludedFolders[relPath]
			nodeMap[relPath] = node
		} else {
			_, node.selected = excludedFiles[relPath]
		}

		parent.children = append(parent.children, node)

		return nil
	})

	return root
}

func flattenTree(node *fileNode, depth int) []*fileNode {
	result := []*fileNode{node}

	if node.isDir && node.expanded {
		for _, child := range node.children {
			result = append(result, flattenTree(child, depth+1)...)
		}
	}

	return result
}

func rebuildFlat(m *excludeModel) {
	m.flatNodes = flattenTree(m.root, 0)
}

func (m *excludeModel) Init() tea.Cmd {
	return nil
}

func (m *excludeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	maxCursor := len(m.flatNodes) - 2
	if maxCursor < 0 {
		maxCursor = 0
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			if m.embedded {
				m.done = true
				return m, nil
			}
			return m, tea.Quit
		case "q":
			if m.saved {
				if m.embedded {
					m.done = true
					return m, nil
				}
				return m, tea.Quit
			}
			m.status = "Press 's' to save or 'Esc' to go back"
			return m, nil
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < maxCursor {
				m.cursor++
			}
		case "enter":
			nodeIdx := m.cursor + 1
			if nodeIdx < len(m.flatNodes) {
				node := m.flatNodes[nodeIdx]
				if node.isDir {
					node.expanded = !node.expanded
					rebuildFlat(m)
				}
			}
		case " ":
			nodeIdx := m.cursor + 1
			if nodeIdx < len(m.flatNodes) {
				node := m.flatNodes[nodeIdx]
				node.selected = !node.selected
				m.status = fmt.Sprintf("%s: %s", node.name, boolToStatus(node.selected))
			}
		case "s":
			m.saveConfig()
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	return m, nil
}

func boolToStatus(b bool) string {
	if b {
		return "EXCLUDED"
	}
	return "included"
}

func (m *excludeModel) saveConfig() {
	var folders, files []string

	collectExcluded(m.root, &folders, &files)

	m.config.Exclude.Folders = folders
	m.config.Exclude.Files = files

	data, err := configToYAML(m.config)
	if err != nil {
		m.status = fmt.Sprintf("Error: %v", err)
		return
	}

	err = os.WriteFile(m.configPath, data, 0644)
	if err != nil {
		m.status = fmt.Sprintf("Error saving: %v", err)
		return
	}

	m.saved = true
	m.status = "Configuration saved!"
}

func collectExcluded(node *fileNode, folders, files *[]string) {
	for _, child := range node.children {
		if child.selected {
			if child.isDir {
				*folders = append(*folders, child.path+"/")
			} else {
				*files = append(*files, child.path)
			}
		}
		collectExcluded(child, folders, files)
	}
}

func configToYAML(cfg *config.Config) ([]byte, error) {
	var b strings.Builder

	b.WriteString("# .gitdual.yml\n")
	b.WriteString(fmt.Sprintf("version: %d\n\n", cfg.Version))

	b.WriteString("remotes:\n")
	if cfg.Remotes.Private != "" {
		b.WriteString(fmt.Sprintf("  private: %s\n", cfg.Remotes.Private))
	}
	if cfg.Remotes.Public != "" {
		b.WriteString(fmt.Sprintf("  public: %s\n", cfg.Remotes.Public))
	}
	b.WriteString("\n")

	b.WriteString("exclude:\n")
	if len(cfg.Exclude.Folders) > 0 {
		b.WriteString("  folders:\n")
		for _, f := range cfg.Exclude.Folders {
			b.WriteString(fmt.Sprintf("    - %s\n", f))
		}
	} else {
		b.WriteString("  folders: []\n")
	}

	if len(cfg.Exclude.Files) > 0 {
		b.WriteString("  files:\n")
		for _, f := range cfg.Exclude.Files {
			b.WriteString(fmt.Sprintf("    - %s\n", f))
		}
	} else {
		b.WriteString("  files: []\n")
	}

	if len(cfg.Exclude.Branches) > 0 {
		b.WriteString("  branches:\n")
		for _, f := range cfg.Exclude.Branches {
			b.WriteString(fmt.Sprintf("    - %s\n", f))
		}
	} else {
		b.WriteString("  branches: []\n")
	}

	b.WriteString(fmt.Sprintf("  private_suffix: %q\n\n", cfg.Exclude.PrivateSuffix))

	b.WriteString("commit:\n")
	b.WriteString(fmt.Sprintf("  public_message: %q\n", cfg.Commit.PublicMessage))
	b.WriteString(fmt.Sprintf("  squash: %v\n", cfg.Commit.Squash))

	return []byte(b.String()), nil
}

func (m *excludeModel) View() string {
	var b strings.Builder

	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7C3AED")).
		Render("╔═══════════════════════════════════╗")
	header += "\n"
	header += lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7C3AED")).
		Render("║      🚫 Exclude Manager           ║")
	header += "\n"
	header += lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7C3AED")).
		Render("╚═══════════════════════════════════╝")

	b.WriteString(header)
	b.WriteString("\n\n")

	treeStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7C3AED")).
		Padding(1, 2).
		Width(m.width - 6).
		Height(m.height - 12)

	var treeContent strings.Builder
	startIdx := 1
	if len(m.flatNodes) <= 1 {
		treeContent.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF")).Render("  No files to display"))
		treeContent.WriteString("\n")
	} else {
		maxDisplay := len(m.flatNodes) - startIdx
		if m.height > 16 {
			available := m.height - 16
			if available > 0 && maxDisplay > available {
				maxDisplay = available
			}
		}

		for i := startIdx; i < len(m.flatNodes) && i-startIdx < maxDisplay; i++ {
			node := m.flatNodes[i]
			displayIdx := i - startIdx

			cursor := "  "
			if m.cursor == displayIdx {
				cursor = lipgloss.NewStyle().Foreground(lipgloss.Color("#7C3AED")).Render("▶ ")
			}

			depth := strings.Count(node.path, string(filepath.Separator))
			indent := strings.Repeat("  ", depth)

			icon := "📄 "
			if node.isDir {
				if node.expanded {
					icon = "📂 "
				} else {
					icon = "📁 "
				}
			}

			checkBox := "[ ] "
			if node.selected {
				checkBox = lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444")).Render("[✗] ")
			} else {
				checkBox = lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981")).Render("[ ] ")
			}

			name := node.name
			if node.isDir {
				name = lipgloss.NewStyle().Bold(true).Render(name)
			}

			if m.cursor == displayIdx {
				treeContent.WriteString(lipgloss.NewStyle().Background(lipgloss.Color("#2D1B69")).Render(fmt.Sprintf("%s%s%s%s%s", cursor, indent, icon, checkBox, name)))
			} else {
				treeContent.WriteString(fmt.Sprintf("%s%s%s%s%s", cursor, indent, icon, checkBox, name))
			}
			treeContent.WriteString("\n")
		}
	}

	b.WriteString(treeStyle.Render(treeContent.String()))
	b.WriteString("\n")

	excludedCount := countExcluded(m.root)
	statusPrefix := "●"
	if m.saved {
		statusPrefix = "✓"
	}

	statusColor := lipgloss.Color("#10B981")
	if strings.Contains(m.status, "Error") {
		statusColor = lipgloss.Color("#EF4444")
	} else if m.saved {
		statusColor = lipgloss.Color("#10B981")
	}

	statusText := m.status
	if excludedCount > 0 && !strings.Contains(m.status, "Error") {
		statusText = fmt.Sprintf("%s (%d item(s) selected for exclusion)", m.status, excludedCount)
	}

	b.WriteString(lipgloss.NewStyle().Foreground(statusColor).Render(fmt.Sprintf("%s %s", statusPrefix, statusText)))
	b.WriteString("\n\n")

	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#666666"))
	if m.embedded {
		b.WriteString(helpStyle.Render("↑/↓ Navigate | Enter Expand | Space Toggle | s Save | Esc Back"))
	} else {
		b.WriteString(helpStyle.Render("↑/↓ Navigate | Enter Expand | Space Toggle | s Save | Esc Quit"))
	}

	return b.String()
}

func countExcluded(node *fileNode) int {
	count := 0
	for _, child := range node.children {
		if child.selected {
			count++
		}
		count += countExcluded(child)
	}
	return count
}

func init() {
	rootCmd.AddCommand(excludeCmd)
}
