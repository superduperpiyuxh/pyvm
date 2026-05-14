package styles

import "github.com/charmbracelet/lipgloss"

// AccentColor is the current accent hex. Call ApplyAccentColor to change it.
var AccentColor = "#F7C948"

// All styles are vars (not consts) so ApplyAccentColor can reassign them.
var (
	HighlightStyle lipgloss.Style
	SuccessStyle   lipgloss.Style
	ErrorStyle     lipgloss.Style
	AppStyle       lipgloss.Style
	TitleStyle     lipgloss.Style
	DocStyle       lipgloss.Style
	HelpStyle      func(strs ...string) string
)

// Themes are the preset colors shown in the TUI picker.
var Themes = []struct {
	Name  string
	Color string
}{
	{"Python yellow", "#F7C948"},
	{"Python blue", "#4B8BBE"},
	{"Dracula purple", "#BD93F9"},
	{"Nord frost", "#88C0D0"},
	{"Rose pink", "#FF79C6"},
	{"Catppuccin peach", "#FAB387"},
	{"Neon green", "#00FF88"},
	{"Monokai orange", "#FD971F"},
	{"Tokyo night blue", "#7AA2F7"},
	{"Gruvbox aqua", "#8EC07C"},
}

// ApplyAccentColor reassigns every style var to use the given hex color.
// Call this at startup (after loading config) and on every theme change.
func ApplyAccentColor(hex string) {
	AccentColor = hex
	accent := lipgloss.Color(hex)

	HighlightStyle = lipgloss.NewStyle().Foreground(accent)
	SuccessStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#8cdb2f"))
	ErrorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#f25d94"))

	AppStyle = lipgloss.NewStyle().
		Padding(1, 2).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(accent)

	TitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(accent).
		MarginBottom(1)

	DocStyle = lipgloss.NewStyle().Margin(1, 2)
	HelpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#626262")).Render
}

func init() { ApplyAccentColor(AccentColor) }
