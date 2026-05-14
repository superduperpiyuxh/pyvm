package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/user/pyvm/internal/cli"
	"github.com/user/pyvm/internal/model"
	"github.com/user/pyvm/internal/setup"
	"github.com/user/pyvm/internal/styles"
	"github.com/user/pyvm/internal/utils"
)

func main() {
	// Load config first so accent color is applied before anything renders.
	cfg := utils.LoadConfig()
	styles.ApplyAccentColor(cfg.AccentColor)

	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "version") {
		fmt.Printf("pyvm %s\n", utils.GetVersion())
		os.Exit(0)
	}

	if err := utils.SetupShimDirectory(); err != nil {
		fmt.Printf("pyvm: setup error: %v\n", err)
		os.Exit(1)
	}

	if len(os.Args) > 1 {
		handleCommandLine()
	} else {
		launchTUI()
	}
}

func handleCommandLine() {
	command := os.Args[1]

	switch command {
	case "install":
		if len(os.Args) < 3 {
			fmt.Println("Usage: pyvm install <version>")
			os.Exit(1)
		}
		cli.InstallVersion(cleanVersion(os.Args[2]))

	case "use":
		if len(os.Args) < 3 {
			fmt.Println("Usage: pyvm use <version>")
			os.Exit(1)
		}
		cli.UseVersion(cleanVersion(os.Args[2]))

	case "list", "ls":
		cli.ListVersions()

	case "delete", "remove", "rm", "uninstall":
		if len(os.Args) < 3 {
			fmt.Println("Usage: pyvm delete <version>")
			os.Exit(1)
		}
		cli.DeleteVersion(cleanVersion(os.Args[2]))

	case "color", "theme":
		if len(os.Args) < 3 {
			cfg := utils.LoadConfig()
			fmt.Printf("Current accent color: %s\n", cfg.AccentColor)
			fmt.Println("Usage: pyvm color <#RRGGBB>")
			fmt.Println("Example: pyvm color '#4B8BBE'")
			fmt.Println("\nPreset themes:")
			for _, t := range styles.Themes {
				fmt.Printf("  %-22s  %s\n", t.Name, t.Color)
			}
			return
		}
		cli.SetColor(os.Args[2])

	case "which":
		if len(os.Args) < 3 {
			fmt.Println("Usage: pyvm which <tool>")
			os.Exit(1)
		}
		printWhich(os.Args[2])

	case "help", "--help", "-h":
		printHelp()

	default:
		fmt.Printf("pyvm: unknown command %q\n\n", command)
		printHelp()
		os.Exit(1)
	}
}

func launchTUI() {
	if !utils.IsShimInPath() {
		p := tea.NewProgram(setup.New(), tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	}

	accent := lipgloss.Color(styles.AccentColor)

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(accent)

	columns := []table.Column{
		{Title: "Version", Width: 12},
		{Title: "Path", Width: 50},
		{Title: "Status", Width: 10},
	}
	t := table.New(
		table.WithColumns(columns),
		table.WithRows([]table.Row{}),
		table.WithFocused(true),
		table.WithHeight(10),
	)
	ts := table.DefaultStyles()
	ts.Header = ts.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(accent).
		BorderBottom(true).Bold(true)
	ts.Selected = ts.Selected.
		Foreground(lipgloss.Color("#000000")).
		Background(accent)
	t.SetStyles(ts)

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(accent).BorderLeftForeground(accent)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(lipgloss.AdaptiveColor{Light: "#555", Dark: "#999"}).
		BorderLeftForeground(accent)

	l := list.New([]list.Item{}, delegate, 0, 0)
	l.Title = "Available Python Versions"
	l.Styles.Title = styles.TitleStyle
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)

	ti := textinput.New()
	ti.CharLimit = 10
	ti.Width = 12

	m := model.Model{
		List:           l,
		Loading:        true,
		Spinner:        s,
		InstalledTable: t,
		ColorInput:     ti,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

func cleanVersion(s string) string {
	s = strings.TrimPrefix(s, "python")
	s = strings.TrimPrefix(s, "-")
	return s
}

func printWhich(tool string) {
	homeDir, _ := os.UserHomeDir()
	shimPath := filepath.Join(homeDir, ".pyvm", "shim", tool)
	if runtime.GOOS == "windows" {
		shimPath += ".bat"
	}

	if _, err := os.Stat(shimPath); err == nil {
		fmt.Println(shimPath)
	} else {
		fmt.Printf("pyvm: '%s' not found in shim directory\n", tool)
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Print(`
PyVM — Python Version Manager

Usage:
  pyvm                        Interactive TUI
  pyvm install <version>      Install a Python version
  pyvm use <version>          Switch active version
  pyvm list                   List installed versions
  pyvm delete <version>       Remove an installed version
  pyvm color [#RRGGBB]        Get/set the accent color
  pyvm which <tool>           Show shim path for a tool
  pyvm version                Show pyvm version

Color examples:
  pyvm color '#4B8BBE'        Set Python blue
  pyvm color '#BD93F9'        Set Dracula purple
  pyvm color                  Show current color + all presets

TUI keybindings:
  tab           Cycle tabs (Available / Installed / Themes)

  In Themes tab:
    ↑/↓         Navigate presets
    enter       Apply selected preset
    c           Type a custom hex color

  In Available tab:
    i           Install selected version
    u           Activate selected version
    d           Delete selected version
    r           Refresh list
    /           Filter

  q             Quit

`)
}
