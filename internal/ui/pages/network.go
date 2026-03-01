// Copyright 2017-2026 DERO Project. All rights reserved.

package pages

import (
	"log"
	"path/filepath"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/deroproject/dero-wallet-cli/internal/ui/styles"
)

var (
	networkUpKeys        = key.NewBinding(key.WithKeys("up", "k"))
	networkDownKeys      = key.NewBinding(key.WithKeys("down", "j"))
	networkMainnetKeys   = key.NewBinding(key.WithKeys("1"))
	networkTestnetKeys   = key.NewBinding(key.WithKeys("2"))
	networkSimulatorKeys = key.NewBinding(key.WithKeys("3"))
)

// NetworkSelection represents the selected network
type NetworkSelection int

const (
	NetworkNone NetworkSelection = iota
	NetworkMainnet
	NetworkTestnet
	NetworkSimulator
)

// NetworkModel represents the network selection screen
type NetworkModel struct {
	selected   int // 0=Mainnet, 1=Testnet, 2=Simulator
	confirmed  bool
	cancelled  bool
	walletPath string
}

// WalletPath returns the wallet path/name stored in the model
func (n NetworkModel) WalletPath() string {
	return n.walletPath
}

// NewNetwork creates a new network selection screen
func NewNetwork(walletPath string) NetworkModel {
	return NetworkModel{
		selected:   0,
		walletPath: walletPath,
	}
}

// Init initializes the network screen
func (n NetworkModel) Init() tea.Cmd {
	return nil
}

// Update handles events
func (n NetworkModel) Update(msg tea.Msg) (NetworkModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, networkUpKeys):
			if n.selected > 0 {
				n.selected--
			}
		case key.Matches(msg, networkDownKeys):
			if n.selected < 2 {
				n.selected++
			}
		case key.Matches(msg, networkMainnetKeys):
			log.Printf("[DEBUG network] Selected Mainnet")
			n.selected = 0
			n.confirmed = true
		case key.Matches(msg, networkTestnetKeys):
			log.Printf("[DEBUG network] Selected Testnet")
			n.selected = 1
			n.confirmed = true
		case key.Matches(msg, networkSimulatorKeys):
			log.Printf("[DEBUG network] Selected Simulator")
			n.selected = 2
			n.confirmed = true
		case key.Matches(msg, pageEnterKeys):
			networks := []string{"Mainnet", "Testnet", "Simulator"}
			log.Printf("[DEBUG network] Confirmed selection: %s", networks[n.selected])
			n.confirmed = true
		case key.Matches(msg, pageEscKeys):
			log.Printf("[DEBUG network] Cancelled network selection")
			n.cancelled = true
		}
	}

	return n, nil
}

// View renders the network selection screen
func (n NetworkModel) View() string {
	logo := styles.Logo()
	title := styles.TitleStyle.Render("Select Wallet Network")

	walletName := filepath.Base(n.walletPath)
	walletLabel := styles.MutedStyle.Render("Wallet: ") + styles.TextStyle.Render(walletName)

	description := styles.MutedStyle.Render("Please select the network it was created on:")

	options := []string{"Mainnet", "Testnet", "Simulator"}

	var menuItems []string
	for i, opt := range options {
		if i == n.selected {
			item := styles.SelectedMenuItemStyle.Render("▸ ") + styles.TextStyle.Render(opt)
			menuItems = append(menuItems, item)
		} else {
			item := "  " + styles.MutedStyle.Render(opt)
			menuItems = append(menuItems, item)
		}
	}
	optionsView := lipgloss.JoinVertical(lipgloss.Left, menuItems...)

	note := styles.MutedStyle.Render("This will be saved for future opens.")
	help := styles.MutedStyle.Render("↑↓ Navigate • Enter Confirm • Esc Cancel")

	content := lipgloss.JoinVertical(lipgloss.Center,
		logo,
		"",
		title,
		"",
		walletLabel,
		"",
		description,
		"",
		optionsView,
		"",
		note,
		"",
		help,
	)

	return styles.ThemedBoxStyle().
		Width(styles.Width).
		Align(lipgloss.Center).
		Padding(2, 4).
		Render(content)
}

// Confirmed returns true if selection was confirmed
func (n NetworkModel) Confirmed() bool {
	return n.confirmed
}

// Cancelled returns true if selection was cancelled
func (n NetworkModel) Cancelled() bool {
	return n.cancelled
}

// Selection returns the selected network
func (n NetworkModel) Selection() NetworkSelection {
	switch n.selected {
	case 0:
		return NetworkMainnet
	case 1:
		return NetworkTestnet
	case 2:
		return NetworkSimulator
	default:
		return NetworkNone
	}
}

// Reset resets the model state
func (n *NetworkModel) Reset() {
	n.selected = 0
	n.confirmed = false
	n.cancelled = false
}

// SetSelection pre-selects a network
func (n *NetworkModel) SetSelection(network NetworkSelection) {
	switch network {
	case NetworkMainnet:
		n.selected = 0
	case NetworkTestnet:
		n.selected = 1
	case NetworkSimulator:
		n.selected = 2
	}
}

// HandleMouse handles mouse events on the network selection page
func (n NetworkModel) HandleMouse(msg tea.MouseClickMsg, windowWidth, windowHeight int) NetworkModel {
	// Calculate centered position
	boxWidth := styles.Width
	boxX := (windowWidth - boxWidth) / 2
	boxY := (windowHeight - styles.SmallBoxHeight) / 2

	// Adjust to content area
	relX := msg.Mouse().X - boxX - 4
	relY := msg.Mouse().Y - boxY - 10 // After logo, title, walletLabel, description

	switch msg.Button {
	case tea.MouseLeft:
		// Network options start at y=0, each is 1 line
		// Mainnet: y=0, Testnet: y=1, Simulator: y=2
		if relX >= 0 && relX < 50 {
			if relY >= 0 && relY <= 2 {
				n.selected = relY
				n.confirmed = true
				return n
			}
		}

		// Help area at bottom: y=11-12
		if relY >= 11 && relY <= 12 {
			n.cancelled = true
			return n
		}

	case tea.MouseWheelUp:
		if n.selected > 0 {
			n.selected--
		}

	case tea.MouseWheelDown:
		if n.selected < 2 {
			n.selected++
		}
	}

	return n
}
