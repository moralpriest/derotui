// Copyright 2017-2026 DERO Project. All rights reserved.

package pages

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/deroproject/dero-wallet-cli/internal/ui/components"
	"github.com/deroproject/dero-wallet-cli/internal/ui/styles"
)

// NameTransferModel is the name transfer form page.
type NameTransferModel struct {
	name        string
	transferAll bool
	nameCount   int
	names       []string // all names when transferAll is set
	ownerInput  components.InputModel
	cancelled   bool
	confirmed   bool
	error       string
	processing  bool

	width  int
	height int
}

// NewNameTransfer creates a new name transfer form.
func NewNameTransfer() NameTransferModel {
	m := NameTransferModel{
		ownerInput: components.NewInput("New Owner", "Enter DERO address or username", false),
	}
	m.ownerInput.Focus()
	return m
}

// SetName sets the name to transfer (pre-filled from the names list).
func (n *NameTransferModel) SetName(name string) {
	n.name = name
	n.transferAll = false
	n.nameCount = 0
}

// SetTransferAll configures the form for bulk transfer of all owned names.
func (n *NameTransferModel) SetTransferAll(names []string) {
	n.transferAll = true
	n.nameCount = len(names)
	n.names = names
	n.name = ""
}

// GetAllNames returns the names to transfer when transferAll is set.
func (n NameTransferModel) GetAllNames() []string {
	return n.names
}

// IsTransferAll returns true if this is a bulk transfer-all operation.
func (n NameTransferModel) IsTransferAll() bool {
	return n.transferAll
}

// Cancelled returns true if the user cancelled.
func (n NameTransferModel) Cancelled() bool { return n.cancelled }

// Confirmed returns true if the user confirmed.
func (n NameTransferModel) Confirmed() bool { return n.confirmed }

// GetName returns the name being transferred.
func (n NameTransferModel) GetName() string {
	return n.name
}

// GetNewOwner returns the entered owner address.
func (n NameTransferModel) GetNewOwner() string {
	return strings.TrimSpace(n.ownerInput.Value())
}

// SetError sets the error message.
func (n *NameTransferModel) SetError(err string) {
	n.error = err
	n.processing = false
}

// StartProcessing begins the transfer processing state.
func (n *NameTransferModel) StartProcessing() {
	n.processing = true
	n.error = ""
}

// Reset resets the form.
func (n *NameTransferModel) Reset() {
	n.name = ""
	n.transferAll = false
	n.nameCount = 0
	n.names = nil
	n.ownerInput = components.NewInput("New Owner", "Enter DERO address or username", false)
	n.ownerInput.Focus()
	n.cancelled = false
	n.confirmed = false
	n.error = ""
	n.processing = false
}

func (n NameTransferModel) Init() tea.Cmd {
	return n.ownerInput.Init()
}

func (n NameTransferModel) Update(msg tea.Msg) (NameTransferModel, tea.Cmd) {
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
			owner := n.GetNewOwner()
			if owner == "" {
				n.error = "New owner address cannot be empty"
				return n, nil
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
	n.ownerInput, cmd = n.ownerInput.Update(msg)
	return n, cmd
}

func (n NameTransferModel) View() string {
	var content string

	if n.processing {
		content = lipgloss.JoinVertical(lipgloss.Center,
			styles.TitleStyle.Render("Transfer Name"),
			"",
			styles.MutedStyle.Render("Processing..."),
		)
	} else {
		var b strings.Builder
		if n.transferAll {
			b.WriteString(styles.MutedStyle.Render(fmt.Sprintf("Transferring all %d registered names", n.nameCount)))
			b.WriteString("\n\n")
		} else if n.name != "" {
			b.WriteString(styles.MutedStyle.Render("Transferring: "))
			b.WriteString(lipgloss.NewStyle().Foreground(styles.ColorPrimary).Bold(true).Render(n.name))
			b.WriteString("\n\n")
		}

		b.WriteString(n.ownerInput.View())
		b.WriteString("\n\n")

		if n.error != "" {
			b.WriteString(styles.ErrorStyle.
				Width(styles.Width - 8).
				Render("✗ " + n.error))
			b.WriteString("\n")
		}

		b.WriteString(styles.MutedStyle.Render("[Enter] Confirm  [Esc] Cancel"))

		content = lipgloss.JoinVertical(lipgloss.Center,
			styles.TitleStyle.Render("Transfer Name"),
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

// HandleMouse handles mouse events for the transfer page.
func (n NameTransferModel) HandleMouse(msg tea.MouseClickMsg, windowWidth, windowHeight int) NameTransferModel {
	return n
}
