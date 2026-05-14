package styles

import "fmt"

// Item wraps a Python version for display inside a bubbles list.
// It implements the list.Item interface.
type Item struct {
	Name            string // "3.12.7"
	DescriptionText string // "cpython-3.12.7+... linux/amd64"
	Installed       bool
	Active          bool
}

func (i Item) Title() string {
	title := i.Name
	if i.Active {
		title = fmt.Sprintf("%s %s", title, SuccessStyle.Render("(active)"))
	}
	if i.Installed {
		title = fmt.Sprintf("%s %s", title, HighlightStyle.Render("(installed)"))
	}
	return title
}

func (i Item) Description() string { return i.DescriptionText }
func (i Item) FilterValue() string { return i.Name }
