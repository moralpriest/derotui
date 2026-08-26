// Copyright 2017-2026 DERO Project. All rights reserved.

package pages

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/atotto/clipboard"
	"github.com/deroproject/dero-wallet-cli/internal/ui/styles"
	"github.com/deroproject/dero-wallet-cli/internal/wallet"
)

// NamesModel is the registered names management page.
type NamesModel struct {
	names       []wallet.NameEntry
	cursor      int
	cancelled   bool
	register    bool
	transfer    bool
	transferAll bool
	copiedName  string
	loading     bool
	error       string
	flash       string
	flashGood   bool

	width  int
	height int
}

// NewNames creates a new names management page.
func NewNames() NamesModel {
	return NamesModel{cursor: 0}
}

// SetNames sets the list of registered names.
func (n *NamesModel) SetNames(names []wallet.NameEntry) {
	n.names = names
	n.loading = false
	if n.cursor >= len(n.names) && len(n.names) > 0 {
		n.cursor = len(n.names) - 1
	}
}

// SetLoading sets the loading state.
func (n *NamesModel) SetLoading(loading bool) {
	n.loading = loading
}

// SetError sets the error message.
func (n *NamesModel) SetError(err string) {
	n.error = err
	n.loading = false
}

// SetFlash sets a flash message.
func (n *NamesModel) SetFlash(msg string, good bool) {
	n.flash = msg
	n.flashGood = good
}

// Cancelled returns true if the user cancelled.
func (n NamesModel) Cancelled() bool { return n.cancelled }

// Names returns the list of registered names (for bulk operations).
func (n NamesModel) Names() []wallet.NameEntry { return n.names }

// WantRegister returns true if the user wants to register a new name.
func (n NamesModel) WantRegister() bool { return n.register }

// WantTransfer returns the selected name and true if the user wants to transfer.
func (n NamesModel) WantTransfer() (string, bool) {
	if n.transfer && n.cursor >= 0 && n.cursor < len(n.names) {
		return n.names[n.cursor].Name, true
	}
	return "", false
}

// WantTransferAll returns all names and true if the user wants to transfer all.
func (n NamesModel) WantTransferAll() ([]string, bool) {
	if !n.transferAll || len(n.names) == 0 {
		return nil, false
	}
	all := make([]string, len(n.names))
	for i, entry := range n.names {
		all[i] = entry.Name
	}
	return all, true
}

// ResetActions clears action flags.
func (n *NamesModel) ResetActions() {
	n.register = false
	n.transfer = false
	n.transferAll = false
}

// Refresh resets state for re-display.
func (n *NamesModel) Refresh() {
	n.cursor = 0
	n.copiedName = ""
	n.error = ""
	n.flash = ""
	n.cancelled = false
	n.loading = false
}

func (n NamesModel) Init() tea.Cmd { return nil }

func (n NamesModel) Update(msg tea.Msg) (NamesModel, tea.Cmd) {
	n.flash = ""
	n.error = ""

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, pageEscKeys):
			n.cancelled = true
			return n, nil

		case key.Matches(msg, key.NewBinding(key.WithKeys("up", "k"))):
			if n.cursor > 0 {
				n.cursor--
			}
			return n, nil

		case key.Matches(msg, key.NewBinding(key.WithKeys("down", "j"))):
			if n.cursor < len(n.names)-1 {
				n.cursor++
			}
			return n, nil

		case key.Matches(msg, key.NewBinding(key.WithKeys("r"))):
			if !n.loading {
				n.register = true
			}
			return n, nil

		case key.Matches(msg, key.NewBinding(key.WithKeys("t"))):
			if !n.loading && len(n.names) > 0 {
				n.transfer = true
			}
			return n, nil

		case key.Matches(msg, key.NewBinding(key.WithKeys("a"))):
			if !n.loading && len(n.names) > 0 {
				n.transferAll = true
			}
			return n, nil

		case key.Matches(msg, key.NewBinding(key.WithKeys("c"))):
			if !n.loading && len(n.names) > 0 && n.cursor >= 0 && n.cursor < len(n.names) {
				name := n.names[n.cursor].Name
				if clipboard.WriteAll(name) == nil {
					n.copiedName = name
				}
			}
			return n, nil
		}

	case tea.WindowSizeMsg:
		n.width = msg.Width
		n.height = msg.Height
		return n, nil
	}

	return n, nil
}

func (n NamesModel) View() string {
	var content string

	if n.loading {
		content = lipgloss.JoinVertical(lipgloss.Center,
			styles.TitleStyle.Render("Registered Names"),
			"",
			styles.MutedStyle.Render("Loading..."),
		)
	} else {
		var b strings.Builder
		if n.error != "" {
			b.WriteString(styles.ErrorStyle.
				Width(styles.Width - 8).
				Render("✗ " + n.error))
			b.WriteString("\n\n")
		}

		if len(n.names) == 0 {
			b.WriteString(styles.MutedStyle.Render("No registered names"))
			b.WriteString("\n\n")
		}

		for i, entry := range n.names {
			prefix := "  "
			nameStyle := styles.MutedStyle
			if i == n.cursor {
				prefix = lipgloss.NewStyle().Foreground(styles.ColorPrimary).Render("▸ ")
				nameStyle = lipgloss.NewStyle().Foreground(styles.ColorText).Bold(true)
			}

			nameDisplay := prefix + nameStyle.Render(entry.Name)
			if n.copiedName == entry.Name {
				nameDisplay += " " + styles.SuccessStyle.Render("(copied)")
			}
			b.WriteString(nameDisplay)
			b.WriteString("\n")
		}

		if n.flash != "" {
			style := styles.ErrorStyle
			if n.flashGood {
				style = styles.SuccessStyle
			}
			b.WriteString(style.Render(n.flash))
			b.WriteString("\n")
		}

		b.WriteString("\n")

		footer := fmt.Sprintf("[R]egister  %s  %s  %s  [C]opy  [Esc] Back",
			dimIf(len(n.names) == 0, "[T]ransfer"),
			dimIf(len(n.names) > 0, "↑↓ Select"),
			dimIf(len(n.names) == 0, "[A]ll Transfer"),
		)
		b.WriteString(styles.MutedStyle.Render(footer))

		content = lipgloss.JoinVertical(lipgloss.Center,
			styles.TitleStyle.Render("Registered Names"),
			"",
			b.String(),
		)
	}

	return styles.ThemedBoxStyle().
		Width(styles.Width).
		Align(lipgloss.Center).
		Padding(1, 4).
		Render(content)
}

func dimIf(cond bool, s string) string {
	if cond {
		return styles.MutedStyle.Render(s)
	}
	return s
}

// HandleMouse handles mouse events for the names page.
func (n NamesModel) HandleMouse(msg tea.MouseClickMsg, windowWidth, windowHeight int) NamesModel {
	row := msg.Mouse().Y
	// Compensate for box border, title, and blank line (~5 lines).
	titleOffset := 5
	if n.error != "" {
		titleOffset += 2
	}

	row = row - titleOffset
	if row < 0 || row >= len(n.names) {
		return n
	}
	n.cursor = row
	return n
}
