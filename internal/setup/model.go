package setup

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Model is a one-shot bubbletea model that shows PATH setup instructions
// the first time pyvm is launched without the shim on the user's PATH.
type Model struct {
	width       int
	height      int
	shimPath    string
	shellConfig string
}

// New builds the setup model, detecting the user's shell automatically.
func New() Model {
	homeDir, _ := os.UserHomeDir()
	shimPath := filepath.Join(homeDir, ".pyvm", "shim")

	shellConfig := "~/.bashrc"
	if runtime.GOOS == "windows" {
		shellConfig = "PATH environment variable"
	} else if shell := os.Getenv("SHELL"); strings.Contains(shell, "zsh") {
		shellConfig = "~/.zshrc"
	} else if strings.Contains(os.Getenv("SHELL"), "fish") {
		shellConfig = "~/.config/fish/config.fish"
	}

	return Model{
		shimPath:    shimPath,
		shellConfig: shellConfig,
		width:       80,
		height:      24,
	}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter", " ":
			return m, tea.Quit
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

func (m Model) View() string {
	brandColour := lipgloss.Color("#F7C948")

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(brandColour).
		MarginBottom(1).
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(brandColour).
		PaddingBottom(1)

	boxStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(brandColour).
		Padding(1, 2).
		Width(min(m.width-4, 80))

	highlight := lipgloss.NewStyle().Foreground(brandColour).Bold(true)
	footer := lipgloss.NewStyle().Foreground(lipgloss.Color("#626262")).MarginTop(1)

	title := titleStyle.Render("PyVM — First-Time Setup")

	var body string
	if runtime.GOOS == "windows" {
		body = fmt.Sprintf(
			"To use PyVM, add the shim directory to your PATH:\n\n%s\n\nRun this in Command Prompt:\n\n%s\n\nThen restart your terminal.",
			highlight.Render(m.shimPath),
			highlight.Render(fmt.Sprintf(`setx PATH "%%PATH%%;%s"`, m.shimPath)),
		)
	} else {
		shellConfigFile := "~/.bashrc"
		if strings.Contains(os.Getenv("SHELL"), "zsh") {
			shellConfigFile = "~/.zshrc"
		} else if strings.Contains(os.Getenv("SHELL"), "fish") {
			shellConfigFile = "~/.config/fish/config.fish"
		}

		var fishExtra string
		if strings.Contains(os.Getenv("SHELL"), "fish") {
			fishExtra = fmt.Sprintf("\nFor fish, use:\n\n%s\n",
				highlight.Render(fmt.Sprintf(
					"fish_add_path %s", m.shimPath)))
		}

		body = fmt.Sprintf(
			"To use PyVM, add the shim directory to your PATH:\n\n%s\n\n"+
				"Option 1 — add it automatically:\n\n%s\n\n"+
				"Option 2 — paste this line into %s:\n\n%s\n%s\n"+
				"Then restart your terminal or run:\n\n%s",
			highlight.Render(m.shimPath),
			highlight.Render(fmt.Sprintf(
				`echo 'export PATH="$HOME/.pyvm/shim:$PATH"' >> %s`, shellConfigFile)),
			shellConfigFile,
			highlight.Render(`export PATH="$HOME/.pyvm/shim:$PATH"`),
			fishExtra,
			highlight.Render(fmt.Sprintf("source %s", shellConfigFile)),
		)
	}

	box := boxStyle.Render(body)
	foot := footer.Render("Press Enter to continue…")

	paddingTop := max(0, (m.height-lipgloss.Height(title)-lipgloss.Height(box)-lipgloss.Height(foot)-4)/2)
	return strings.Repeat("\n", paddingTop) +
		lipgloss.JoinVertical(lipgloss.Center, title, box, foot)
}

// IsShimInPath is duplicated here so the setup package doesn't import utils
// (avoiding an import cycle when main imports both).
func IsShimInPath() bool {
	homeDir, _ := os.UserHomeDir()
	shimDir := filepath.Join(homeDir, ".pyvm", "shim")
	for _, entry := range strings.Split(os.Getenv("PATH"), string(os.PathListSeparator)) {
		if entry == shimDir {
			return true
		}
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
