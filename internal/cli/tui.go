package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/florianjs/gitdual/internal/config"
	"github.com/florianjs/gitdual/internal/git"
)

type state int

const (
	stateMenu state = iota
	stateExecuting
	stateSuccess
	stateError
	stateExclude
)

type tickMsg struct{}
type execCompleteMsg struct {
	output string
	err    error
}

var (
	gradientPurple = lipgloss.Color("#7C3AED")
	gradientPink   = lipgloss.Color("#EC4899")
	successGreen   = lipgloss.Color("#10B981")
	errorRed       = lipgloss.Color("#EF4444")
	warningYellow  = lipgloss.Color("#F59E0B")
	textGray       = lipgloss.Color("#9CA3AF")
	bgDark         = lipgloss.Color("#1F2937")

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(gradientPurple).
			Padding(0, 1)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(textGray).
			Italic(true)

	menuItemStyle = lipgloss.NewStyle().
			Padding(0, 2).
			Foreground(textGray)

	selectedStyle = lipgloss.NewStyle().
			Padding(0, 2).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(gradientPurple).
			Bold(true)

	iconStyle = lipgloss.NewStyle().
			Foreground(gradientPink).
			PaddingRight(1)

	statusBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(gradientPurple).
			Padding(1, 2).
			Margin(1, 0)

	progressStyle = lipgloss.NewStyle().
			Foreground(gradientPurple)

	successStyle = lipgloss.NewStyle().
			Foreground(successGreen).
			Bold(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(errorRed).
			Bold(true)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(gradientPurple).
			Padding(1, 3).
			Margin(1)

	helpStyle = lipgloss.NewStyle().
			Foreground(textGray).
			Padding(1, 0)
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

var progressBars = []string{
	"█                   ",
	"██                  ",
	"███                 ",
	"████                ",
	"█████               ",
	"██████              ",
	"███████             ",
	"████████            ",
	"█████████           ",
	"██████████          ",
	"███████████         ",
	"████████████        ",
	"█████████████       ",
	"██████████████      ",
	"███████████████     ",
	"████████████████    ",
	"█████████████████   ",
	"██████████████████  ",
	"███████████████████ ",
	"████████████████████",
}

type menuItem struct {
	title string
	desc  string
	icon  string
}

type mainModel struct {
	menuItems    []menuItem
	cursor       int
	state        state
	status       string
	output       string
	err          error
	width        int
	height       int
	spinnerFrame int
	progress     int
	ready        bool
	repoPath     string
	config       *config.Config
	excludeModel *excludeModel
}

func newMainModel() *mainModel {
	m := &mainModel{
		menuItems: []menuItem{
			{title: "Sync", desc: "Synchronize with remotes", icon: "🔄"},
			{title: "Push", desc: "Push to remotes", icon: "⬆️"},
			{title: "Pull", desc: "Pull from remotes", icon: "⬇️"},
			{title: "Exclude", desc: "Manage exclusions", icon: "🚫"},
			{title: "Status", desc: "Show current status", icon: "📊"},
			{title: "Init", desc: "Initialize GitDual", icon: "🚀"},
			{title: "Quit", desc: "Exit GitDual", icon: "👋"},
		},
		state:  stateMenu,
		status: "Ready to rock!",
		ready:  false,
	}

	repoPath, err := git.FindRepoRoot(".")
	if err == nil {
		m.repoPath = repoPath
		cfgPath, err := config.FindConfig(repoPath)
		if err == nil {
			cfg, err := config.LoadConfig(cfgPath)
			if err == nil {
				m.config = cfg
				m.ready = true
			}
		}
	}

	return m
}

func (m *mainModel) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(),
	)
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*80, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}

func (m *mainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.state == stateExclude && m.excludeModel != nil {
		newModel, cmd := m.excludeModel.Update(msg)
		if em, ok := newModel.(*excludeModel); ok {
			m.excludeModel = em
			if em.done {
				m.state = stateMenu
				m.excludeModel = nil
				return m, tickCmd()
			}
		}
		return m, cmd
	}

	switch msg := msg.(type) {
	case tickMsg:
		m.spinnerFrame = (m.spinnerFrame + 1) % len(spinnerFrames)
		if m.state == stateExecuting {
			m.progress = (m.progress + 1) % len(progressBars)
		}
		return m, tickCmd()

	case execCompleteMsg:
		m.output = msg.output
		m.err = msg.err
		if msg.err != nil {
			m.state = stateError
			m.status = "Command failed"
		} else {
			m.state = stateSuccess
			m.status = "Command completed successfully!"
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit

		case "q":
			if m.state == stateMenu {
				return m, tea.Quit
			}
			m.state = stateMenu
			m.status = "Ready"
			return m, nil

		case "esc":
			m.state = stateMenu
			m.status = "Ready"
			return m, nil

		case "up", "k":
			if m.state == stateMenu && m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.state == stateMenu && m.cursor < len(m.menuItems)-1 {
				m.cursor++
			}

		case "enter", " ":
			if m.state == stateMenu {
				return m.handleMenuSelection()
			} else if m.state == stateSuccess || m.state == stateError {
				m.state = stateMenu
				m.status = "Ready"
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	return m, nil
}

func (m *mainModel) handleMenuSelection() (tea.Model, tea.Cmd) {
	item := m.menuItems[m.cursor]

	switch item.title {
	case "Quit":
		return m, tea.Quit

	case "Exclude":
		m.state = stateExclude
		m.excludeModel = newExcludeModelEmbedded(m.repoPath, m.configPath(), m.config)
		return m, m.excludeModel.Init()

	case "Sync":
		m.state = stateExecuting
		m.status = "Synchronizing with remotes..."
		return m, m.executeCommand("sync")

	case "Push":
		m.state = stateExecuting
		m.status = "Pushing to remotes..."
		return m, m.executeCommand("push")

	case "Pull":
		m.state = stateExecuting
		m.status = "Pulling from remotes..."
		return m, m.executeCommand("pull")

	case "Status":
		m.state = stateExecuting
		m.status = "Checking status..."
		return m, m.executeCommand("status")

	case "Init":
		m.state = stateExecuting
		m.status = "Initializing GitDual..."
		return m, m.executeCommand("init")
	}

	return m, nil
}

func (m *mainModel) configPath() string {
	if m.repoPath != "" {
		cfgPath, _ := config.FindConfig(m.repoPath)
		return cfgPath
	}
	return ""
}

func (m *mainModel) executeCommand(cmdName string) tea.Cmd {
	return func() tea.Msg {
		c := exec.Command("gitdual", cmdName)
		c.Dir = m.repoPath
		output, err := c.CombinedOutput()
		return execCompleteMsg{
			output: string(output),
			err:    err,
		}
	}
}

func (m *mainModel) View() string {
	if m.state == stateExclude && m.excludeModel != nil {
		return m.excludeModel.View()
	}

	var b strings.Builder

	b.WriteString(renderHeader())
	b.WriteString("\n\n")

	switch m.state {
	case stateMenu:
		b.WriteString(m.renderMenu())
	case stateExecuting:
		b.WriteString(m.renderExecuting())
	case stateSuccess:
		b.WriteString(m.renderSuccess())
	case stateError:
		b.WriteString(m.renderError())
	}

	b.WriteString("\n")
	b.WriteString(m.renderStatusBar())

	content := b.String()
	return boxStyle.Render(content)
}

func renderHeader() string {
	title := titleStyle.Render("╔═══════════════════════════════════╗")
	title += "\n"
	title += titleStyle.Render("║         🌟 GitDual 🌟             ║")
	title += "\n"
	title += titleStyle.Render("╚═══════════════════════════════════╝")

	return lipgloss.JoinVertical(lipgloss.Center, title, subtitleStyle.Render("Dual remote management made easy"))
}

func (m *mainModel) renderMenu() string {
	var b strings.Builder

	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(gradientPink).Render("  Menu"))
	b.WriteString("\n\n")

	for i, item := range m.menuItems {
		icon := iconStyle.Render(item.icon)

		if m.cursor == i {
			line := fmt.Sprintf("%s %-10s %s", icon, item.title, item.desc)
			b.WriteString(selectedStyle.Render("  " + line))
		} else {
			line := fmt.Sprintf("%s %-10s %s", icon, item.title, item.desc)
			b.WriteString(menuItemStyle.Render("  " + line))
		}
		b.WriteString("\n")
	}

	return b.String()
}

func (m *mainModel) renderExecuting() string {
	var b strings.Builder

	spinner := lipgloss.NewStyle().
		Foreground(gradientPurple).
		Render(spinnerFrames[m.spinnerFrame])

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(gradientPink).
		Render("  ⚡ Executing")
	b.WriteString(title)
	b.WriteString("\n\n")

	b.WriteString(fmt.Sprintf("  %s %s\n\n", spinner, m.status))

	bar := progressBars[m.progress%len(progressBars)]
	progressDisplay := lipgloss.NewStyle().
		Foreground(gradientPurple).
		Render(bar)

	b.WriteString("  ")
	b.WriteString(progressDisplay)
	b.WriteString("\n\n")

	b.WriteString(helpStyle.Render("  Please wait... Press Esc to cancel"))

	return b.String()
}

func (m *mainModel) renderSuccess() string {
	var b strings.Builder

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(successGreen).
		Render("  ✅ Success!")
	b.WriteString(title)
	b.WriteString("\n\n")

	b.WriteString(lipgloss.NewStyle().Padding(0, 2).Render(m.status))
	b.WriteString("\n\n")

	if m.output != "" {
		outputBox := statusBoxStyle.Render(m.output)
		b.WriteString(outputBox)
		b.WriteString("\n")
	}

	b.WriteString(helpStyle.Render("  Press Enter to continue"))

	return b.String()
}

func (m *mainModel) renderError() string {
	var b strings.Builder

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(errorRed).
		Render("  ❌ Error")
	b.WriteString(title)
	b.WriteString("\n\n")

	if m.err != nil {
		b.WriteString(lipgloss.NewStyle().Padding(0, 2).Foreground(errorRed).Render(m.err.Error()))
		b.WriteString("\n\n")
	}

	if m.output != "" {
		outputBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(errorRed).
			Padding(1, 2).
			Render(m.output)
		b.WriteString(outputBox)
		b.WriteString("\n")
	}

	b.WriteString(helpStyle.Render("  Press Enter to continue"))

	return b.String()
}

func (m *mainModel) renderStatusBar() string {
	var status string

	if m.ready {
		status = lipgloss.NewStyle().
			Foreground(successGreen).
			Render("● GitDual Ready")
	} else {
		status = lipgloss.NewStyle().
			Foreground(warningYellow).
			Render("● Not initialized")
	}

	help := helpStyle.Render("↑/↓ Navigate | Enter Select | q Quit")

	return lipgloss.JoinHorizontal(lipgloss.Top, status, "  ", help)
}

func RunTUI() error {
	p := tea.NewProgram(newMainModel(), tea.WithAltScreen())
	_, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	return err
}
