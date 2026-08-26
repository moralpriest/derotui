// Copyright 2017-2026 DERO Project. All rights reserved.

package pages

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/deroproject/dero-wallet-cli/internal/ui/components"
	"github.com/deroproject/dero-wallet-cli/internal/ui/styles"
)

// NameRegisterModel is the name registration form page.
type NameRegisterModel struct {
	nameInput  components.InputModel
	cancelled  bool
	confirmed  bool
	error      string
	processing bool

	width  int
	height int
}

// NewNameRegister creates a new name registration form.
func NewNameRegister() NameRegisterModel {
	m := NameRegisterModel{
		nameInput: components.NewInput("Name", "Enter a name (alphanumeric, . - _)", false),
	}
	m.nameInput.Focus()
	return m
}

// Cancelled returns true if the user cancelled.
func (n NameRegisterModel) Cancelled() bool { return n.cancelled }

// Confirmed returns true if the user confirmed.
func (n NameRegisterModel) Confirmed() bool { return n.confirmed }

// GetName returns the entered name.
func (n NameRegisterModel) GetName() string {
	return strings.TrimSpace(n.nameInput.Value())
}

// SetError sets the error message.
func (n *NameRegisterModel) SetError(err string) {
	n.error = err
	n.processing = false
}

// StartProcessing begins the registration processing state.
func (n *NameRegisterModel) StartProcessing() {
	n.processing = true
	n.error = ""
}

// Reset resets the form.
func (n *NameRegisterModel) Reset() {
	n.nameInput = components.NewInput("Name", "Enter a name (alphanumeric, . - _)", false)
	n.nameInput.Focus()
	n.cancelled = false
	n.confirmed = false
	n.error = ""
	n.processing = false
}

func (n NameRegisterModel) Init() tea.Cmd {
	return n.nameInput.Init()
}

func (n NameRegisterModel) Update(msg tea.Msg) (NameRegisterModel, tea.Cmd) {
	n.error = ""

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if key.Matches(msg, pageEscKeys) {
			if n.processing {
				return n, nil
			}
			n.cancelled = true
			return n, nil
		}

		if key.Matches(msg, key.NewBinding(key.WithKeys("enter"))) {
			if n.processing {
				return n, nil
			}
			name := n.GetName()
			if name == "" {
				n.error = "Name cannot be empty"
				return n, nil
			}
			if len(name) >= 64 {
				n.error = "Name must be less than 64 characters"
				return n, nil
			}
			for _, r := range name {
				if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
					(r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-') {
					n.error = "Name can only contain letters, digits, . _ -"
					return n, nil
				}
			}
			n.confirmed = true
			return n, nil
		}

	case tea.WindowSizeMsg:
		n.width = msg.Width
		n.height = msg.Height
		return n, nil
	}

	var cmd tea.Cmd
	n.nameInput, cmd = n.nameInput.Update(msg)
	return n, cmd
}

func (n NameRegisterModel) View() string {
	var content string

	if n.processing {
		content = lipgloss.JoinVertical(lipgloss.Center,
			styles.TitleStyle.Render("Register Name"),
			"",
			styles.MutedStyle.Render("Processing..."),
		)
	} else {
		var b strings.Builder
		b.WriteString(styles.MutedStyle.Render("Register a username for your wallet address"))
		b.WriteString("\n\n")
		b.WriteString(n.nameInput.View())
		b.WriteString("\n\n")

		if n.error != "" {
			b.WriteString(styles.ErrorStyle.
				Width(styles.Width - 8).
				Render("✗ " + n.error))
			b.WriteString("\n")
		}

		b.WriteString(styles.MutedStyle.Render("[Enter] Confirm  [Esc] Cancel"))

		content = lipgloss.JoinVertical(lipgloss.Center,
			styles.TitleStyle.Render("Register Name"),
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

// HandleMouse handles mouse events for the register page.
func (n NameRegisterModel) HandleMouse(msg tea.MouseClickMsg, windowWidth, windowHeight int) NameRegisterModel {
	return n
}
