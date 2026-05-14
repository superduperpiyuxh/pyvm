package model

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/user/pyvm/internal/styles"
	"github.com/user/pyvm/internal/utils"
)

// ColorAppliedMsg fires after a theme change so the TUI can re-render.
type ColorAppliedMsg struct{ Color string }

// Model is the root bubbletea model.
type Model struct {
	List              list.Model
	Versions          []utils.PythonVersion
	Err               error
	Loading           bool
	Spinner           spinner.Model
	CurrentTab        int // 0=Available  1=Installed  2=Themes
	InstallingVersion string
	Message           string
	MessageType       string
	InstalledTable    table.Model
	ConfirmingDelete  bool
	DeleteVersion     string

	// Themes tab
	ThemeCursor int
	ColorInput  textinput.Model
	InputActive bool
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(utils.FetchPythonVersions, m.Spinner.Tick)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case tea.KeyMsg:
		// Route all keys to the hex input when it is active.
		if m.InputActive {
			switch msg.String() {
			case "enter":
				hex := strings.TrimSpace(m.ColorInput.Value())
				if !strings.HasPrefix(hex, "#") {
					hex = "#" + hex
				}
				if utils.IsValidHex(hex) {
					return m, applyColor(hex)
				}
				m.Message = "Invalid hex — use #RRGGBB or #RGB"
				m.MessageType = "error"
				return m, nil
			case "esc":
				m.InputActive = false
				m.ColorInput.Blur()
				m.Message = ""
				return m, nil
			default:
				var cmd tea.Cmd
				m.ColorInput, cmd = m.ColorInput.Update(msg)
				return m, cmd
			}
		}

		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "tab":
			m.CurrentTab = (m.CurrentTab + 1) % 3
			m.Message = ""
			return m, nil

		case "up", "k":
			if m.CurrentTab == 2 {
				if m.ThemeCursor > 0 {
					m.ThemeCursor--
				}
				return m, nil
			}

		case "down", "j":
			if m.CurrentTab == 2 {
				if m.ThemeCursor < len(styles.Themes)-1 {
					m.ThemeCursor++
				}
				return m, nil
			}

		case "enter":
			if m.CurrentTab == 2 {
				return m, applyColor(styles.Themes[m.ThemeCursor].Color)
			}

		case "c":
			if m.CurrentTab == 2 {
				m.InputActive = true
				m.ColorInput.SetValue("")
				m.ColorInput.Focus()
				m.ColorInput.Placeholder = "#RRGGBB"
				m.Message = ""
				return m, textinput.Blink
			}

		case "i":
			if m.CurrentTab == 0 {
				sel, ok := m.List.SelectedItem().(styles.Item)
				if !ok {
					break
				}
				for _, v := range m.Versions {
					if v.Version == sel.Name && !v.Installed {
						m.Loading = true
						m.InstallingVersion = v.Version
						m.Message = ""
						return m, utils.DownloadAndInstall(v)
					}
				}
				m.Message = "Already installed — press u to activate."
				m.MessageType = "info"
			}

		case "u":
			sel, ok := m.List.SelectedItem().(styles.Item)
			if !ok {
				break
			}
			for _, v := range m.Versions {
				if v.Version == sel.Name && v.Installed {
					m.Loading = true
					m.Message = fmt.Sprintf("Switching to Python %s…", v.Version)
					return m, utils.SwitchVersion(v)
				}
			}
			m.Message = "Install this version first — press i."
			m.MessageType = "error"

		case "d":
			sel, ok := m.List.SelectedItem().(styles.Item)
			if !ok {
				break
			}
			for _, v := range m.Versions {
				if v.Version == sel.Name && v.Installed {
					if v.Active {
						m.Message = "Cannot delete the active version."
						m.MessageType = "error"
						return m, nil
					}
					m.ConfirmingDelete = true
					m.DeleteVersion = v.Version
					m.Message = fmt.Sprintf("Delete Python %s? Y to confirm, N to cancel.", v.Version)
					m.MessageType = "warning"
					return m, nil
				}
			}
			m.Message = "Not installed."
			m.MessageType = "error"

		case "y", "Y":
			if m.ConfirmingDelete {
				m.ConfirmingDelete = false
				m.Loading = true
				var toDelete utils.PythonVersion
				for _, v := range m.Versions {
					if v.Version == m.DeleteVersion {
						toDelete = v
						break
					}
				}
				return m, utils.DeleteVersion(toDelete)
			}

		case "n", "N":
			if m.ConfirmingDelete {
				m.ConfirmingDelete = false
				m.DeleteVersion = ""
				m.Message = "Delete cancelled."
				m.MessageType = "info"
			}

		case "r":
			m.Loading = true
			m.Message = ""
			return m, utils.FetchPythonVersions
		}

	case tea.WindowSizeMsg:
		h, v := styles.DocStyle.GetFrameSize()
		m.List.SetSize(msg.Width-h, msg.Height-v-6)
		m.InstalledTable.SetWidth(msg.Width - h)
		m.InstalledTable.SetHeight(msg.Height - v - 10)
		return m, nil

	case utils.ErrMsg:
		m.Err = msg
		m.Loading = false
		m.Message = msg.Error()
		m.MessageType = "error"
		return m, nil

	case utils.VersionsMsg:
		m.Versions = msg
		items := make([]list.Item, len(m.Versions))
		for i, v := range m.Versions {
			items[i] = styles.Item{
				Name: v.Version, DescriptionText: v.Filename,
				Installed: v.Installed, Active: v.Active,
			}
		}
		m.List.SetItems(items)
		m.Loading = false
		m.updateInstalledTable()
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.Spinner, cmd = m.Spinner.Update(msg)
		return m, cmd

	case utils.DownloadCompleteMsg:
		m.Loading = false
		m.InstallingVersion = ""
		for i, v := range m.Versions {
			if v.Version == msg.Version {
				m.Versions[i].Installed = true
				m.Versions[i].Path = msg.Path
			}
		}
		m.refreshListItems()
		m.updateInstalledTable()
		m.Message = fmt.Sprintf("Installed Python %s", msg.Version)
		m.MessageType = "success"
		return m, nil

	case utils.SwitchCompletedMsg:
		m.Loading = false
		for i := range m.Versions {
			m.Versions[i].Active = m.Versions[i].Version == msg.Version
		}
		m.refreshListItems()
		m.updateInstalledTable()
		if msg.ShimInPath {
			m.Message = fmt.Sprintf("Switched to Python %s!", msg.Version)
		} else {
			m.Message = "Switched! " + utils.GetShimPathInstructions()
		}
		m.MessageType = "success"
		return m, nil

	case utils.DeleteCompleteMsg:
		m.Loading = false
		for i, v := range m.Versions {
			if v.Version == msg.Version {
				m.Versions[i].Installed = false
				m.Versions[i].Path = ""
			}
		}
		m.refreshListItems()
		m.updateInstalledTable()
		m.Message = fmt.Sprintf("Deleted Python %s", msg.Version)
		m.MessageType = "success"
		return m, nil

	case ColorAppliedMsg:
		m.InputActive = false
		m.ColorInput.Blur()
		m.Message = fmt.Sprintf("Theme changed to %s", msg.Color)
		m.MessageType = "success"
		// Re-wire list delegate to new accent color
		accent := lipgloss.Color(styles.AccentColor)
		d := list.NewDefaultDelegate()
		d.Styles.SelectedTitle = d.Styles.SelectedTitle.
			Foreground(accent).BorderLeftForeground(accent)
		d.Styles.SelectedDesc = d.Styles.SelectedDesc.
			Foreground(lipgloss.AdaptiveColor{Light: "#555", Dark: "#999"}).
			BorderLeftForeground(accent)
		m.List.SetDelegate(d)
		return m, nil
	}

	newList, lCmd := m.List.Update(msg)
	m.List = newList
	cmds = append(cmds, lCmd)

	newTable, tCmd := m.InstalledTable.Update(msg)
	m.InstalledTable = newTable
	cmds = append(cmds, tCmd)

	return m, tea.Batch(cmds...)
}

func applyColor(hex string) tea.Cmd {
	return func() tea.Msg {
		styles.ApplyAccentColor(hex)
		cfg := utils.LoadConfig()
		cfg.AccentColor = hex
		utils.SaveConfig(cfg)
		return ColorAppliedMsg{Color: hex}
	}
}

func (m *Model) refreshListItems() {
	items := m.List.Items()
	for i, it := range items {
		item := it.(styles.Item)
		for _, v := range m.Versions {
			if v.Version == item.Name {
				item.Installed = v.Installed
				item.Active = v.Active
				items[i] = item
			}
		}
	}
	m.List.SetItems(items)
}

func (m *Model) updateInstalledTable() {
	var rows []table.Row
	for _, v := range m.Versions {
		if v.Installed {
			status := ""
			if v.Active {
				status = "active"
			}
			rows = append(rows, table.Row{v.Version, v.Path, status})
		}
	}
	m.InstalledTable.SetRows(rows)
}

func (m Model) View() string {
	if m.Err != nil {
		return fmt.Sprintf("Error: %s\n\nPress q to quit.", m.Err)
	}

	var parts []string
	parts = append(parts, styles.TitleStyle.Render("PyVM — Python Version Manager"))

	if !utils.IsShimInPath() {
		warn := lipgloss.NewStyle().
			Background(lipgloss.Color("#FFCC00")).
			Foreground(lipgloss.Color("#000")).Bold(true).Padding(1, 2)
		parts = append(parts, warn.Render("⚠  "+utils.GetShimPathInstructions()))
	}

	tabNames := []string{"Available", "Installed", "Themes"}
	tabBar := ""
	for i, name := range tabNames {
		if i == m.CurrentTab {
			tabBar += styles.HighlightStyle.Render("[ "+name+" ]") + " "
		} else {
			tabBar += fmt.Sprintf("[ %s ] ", name)
		}
	}
	parts = append(parts, tabBar)

	switch m.CurrentTab {
	case 0:
		parts = append(parts, m.List.View())
		if m.Loading {
			label := "Fetching versions…"
			if m.InstallingVersion != "" {
				label = fmt.Sprintf("Installing Python %s…", m.InstallingVersion)
			}
			parts = append(parts, fmt.Sprintf("%s %s", m.Spinner.View(), label))
		}
	case 1:
		parts = append(parts, m.InstalledTable.View())
	case 2:
		parts = append(parts, m.themesView())
	}

	if m.Message != "" {
		switch m.MessageType {
		case "success":
			parts = append(parts, styles.SuccessStyle.Render(m.Message))
		case "error", "warning":
			parts = append(parts, styles.ErrorStyle.Render(m.Message))
		default:
			parts = append(parts, m.Message)
		}
	}

	parts = append(parts, styles.HelpStyle(m.helpText()))
	return styles.AppStyle.Render(lipgloss.JoinVertical(lipgloss.Left, parts...))
}

func (m Model) themesView() string {
	accent := lipgloss.Color(styles.AccentColor)
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("#555"))

	currentBox := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(accent).Padding(0, 1).MarginBottom(1).
		Render(fmt.Sprintf("Current color: %s   %s",
			styles.HighlightStyle.Render(styles.AccentColor),
			lipgloss.NewStyle().Background(accent).Foreground(lipgloss.Color("#000")).
				Padding(0, 2).Render("  "),
		))

	var rows []string
	for i, theme := range styles.Themes {
		swatch := lipgloss.NewStyle().
			Background(lipgloss.Color(theme.Color)).Padding(0, 1).Render("  ")

		if i == m.ThemeCursor {
			nameStr := lipgloss.NewStyle().Foreground(accent).Bold(true).Render(theme.Name)
			rows = append(rows, fmt.Sprintf("  ▸ %s  %-22s  %s", swatch, nameStr, dim.Render(theme.Color)))
		} else {
			rows = append(rows, fmt.Sprintf("    %s  %-22s  %s", swatch, theme.Name, dim.Render(theme.Color)))
		}
	}

	customLine := "\n" + dim.Render("  Press c to type a custom hex color")
	if m.InputActive {
		customLine = fmt.Sprintf("\n  Custom hex: %s\n  %s",
			m.ColorInput.View(),
			dim.Render("Enter to apply · Esc to cancel"),
		)
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		currentBox,
		strings.Join(rows, "\n"),
		customLine,
	)
}

func (m Model) helpText() string {
	switch m.CurrentTab {
	case 0:
		return "\n[i] install  [u] use  [d] delete  [r] refresh  [tab] tabs  [q] quit"
	case 1:
		return "\n[u] use  [d] delete  [tab] tabs  [q] quit"
	case 2:
		if m.InputActive {
			return "\n[enter] apply  [esc] cancel"
		}
		return "\n[↑/↓] navigate  [enter] apply  [c] custom hex  [tab] tabs  [q] quit"
	}
	return ""
}
