// Copyright 2017-2026 DERO Project. All rights reserved.

package pages

import (
	"log"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/deroproject/dero-wallet-cli/internal/ui/components"
	"github.com/deroproject/dero-wallet-cli/internal/ui/styles"
)

var (
	daemonPrevKeys = key.NewBinding(key.WithKeys("left", "up"))
	daemonNextKeys = key.NewBinding(key.WithKeys("right", "down"))
)

// DaemonNetwork is the selected daemon network in the connect form.
type DaemonNetwork int

const (
	DaemonNetworkMainnet DaemonNetwork = iota
	DaemonNetworkTestnet
	DaemonNetworkSimulator
)

// DaemonModel represents the daemon connection page
type DaemonModel struct {
	input     components.InputModel
	network   DaemonNetwork
	confirmed bool
	cancelled bool
	error     string
	address   string
}

// NewDaemon creates a new daemon connection page
func NewDaemon(testnet, simulator bool) DaemonModel {
	network := DaemonNetworkMainnet
	if simulator {
		network = DaemonNetworkSimulator
	} else if testnet {
		network = DaemonNetworkTestnet
	}

	m := DaemonModel{
		input:   components.NewInput("Daemon Address", "", false),
		network: network,
	}
	m.setPlaceholderForNetwork()
	m.input.Focus()

	return m
}

// Init initializes the daemon page
func (d DaemonModel) Init() tea.Cmd {
	return d.input.Init()
}

// Update handles events
func (d DaemonModel) Update(msg tea.Msg) (DaemonModel, tea.Cmd) {
	var cmd tea.Cmd

	// Store error at start - preserve it unless explicitly cleared
	currentError := d.error

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		// Handle escape first
		if key.Matches(msg, pageEscKeys) {
			log.Printf("[DEBUG daemon] Cancelled daemon connection")
			d.cancelled = true
			return d, nil
		}

		switch {
		case key.Matches(msg, daemonPrevKeys):
			d.selectPrevNetwork()
			d.setPlaceholderForNetwork()
			d.error = ""
			return d, nil
		case key.Matches(msg, daemonNextKeys):
			d.selectNextNetwork()
			d.setPlaceholderForNetwork()
			d.error = ""
			return d, nil
		case key.Matches(msg, pageTabKeys):
			d.cycleNextNetwork()
			d.setPlaceholderForNetwork()
			d.error = ""
			return d, nil
		case key.Matches(msg, pageShiftTabKeys):
			d.cyclePrevNetwork()
			d.setPlaceholderForNetwork()
			d.error = ""
			return d, nil
		case key.Matches(msg, pageEnterKeys):
			addr := d.input.Value()
			if addr == "" {
				addr = d.DefaultAddress()
			} else if !strings.Contains(addr, ":") {
				addr = addr + ":" + d.DefaultPort()
			}
			log.Printf("[DEBUG daemon] Confirmed daemon address: %s", addr)
			d.address = addr
			d.confirmed = true
			return d, nil
		}

		// Other key press - update input and clear error
		d.input, cmd = d.input.Update(msg)
		d.error = ""
		return d, cmd
	}

	// Non-KeyMsg - update input but preserve error
	d.input, cmd = d.input.Update(msg)
	d.error = currentError

	return d, cmd
}

// View renders the daemon connection page
func (d DaemonModel) View() string {
	// Logo
	logo := styles.Logo()

	// Title
	title := styles.TitleStyle.Render("Connect to Daemon")

	// Network radio options
	networksStyled := lipgloss.JoinHorizontal(lipgloss.Left,
		d.networkOption(DaemonNetworkMainnet, "Mainnet", styles.SuccessStyle, "10102"),
		styles.MutedStyle.Render("   "),
		d.networkOption(DaemonNetworkTestnet, "Testnet", styles.TestnetStyle, "40402"),
		styles.MutedStyle.Render("   "),
		d.networkOption(DaemonNetworkSimulator, "Simulator", styles.SimulatorStyle, "20000"),
	)

	// Input
	inputView := d.input.View()

	// Error - constrain width to fit in box
	var errorView string
	if d.error != "" {
		errorText := styles.ErrorStyle.
			Width(styles.Width - 8). // Account for box padding
			Render("✗ " + d.error)
		errorView = "\n" + errorText
	}

	// Help
	help := "←/→/Tab • Shift+Tab Network • Enter Connect • Esc Cancel"
	helpStyled := styles.MutedStyle.Render(help)

	// Compose
	content := lipgloss.JoinVertical(lipgloss.Center,
		logo,
		"",
		title,
		"",
		"",
		networksStyled,
		"",
		"",
		inputView,
		errorView,
		"",
		"",
		helpStyled,
	)

	return styles.ThemedBoxStyle().
		Width(styles.Width).
		Align(lipgloss.Center).
		Padding(2, 4).
		Render(content)
}

// Address returns the entered daemon address
func (d DaemonModel) Address() string {
	return d.address
}

// Confirmed returns whether connection was confirmed
func (d DaemonModel) Confirmed() bool {
	return d.confirmed
}

// Cancelled returns whether user cancelled
func (d DaemonModel) Cancelled() bool {
	return d.cancelled
}

// SetError sets an error message
func (d *DaemonModel) SetError(err string) {
	d.error = err
	d.confirmed = false
}

// Reset resets the daemon page
func (d *DaemonModel) Reset() {
	d.input.Reset()
	d.setPlaceholderForNetwork()
	d.confirmed = false
	d.cancelled = false
	d.error = ""
	d.address = ""
	d.input.Focus()
}

// ResetConfirmed resets only the confirmed flag (used when connection is in progress)
func (d *DaemonModel) ResetConfirmed() {
	d.confirmed = false
}

// HandleMouse handles mouse events on the daemon connection page
func (d DaemonModel) HandleMouse(msg tea.MouseClickMsg, windowWidth, windowHeight int) DaemonModel {
	// Calculate centered position
	boxWidth := styles.Width
	boxX := (windowWidth - boxWidth) / 2
	boxY := (windowHeight - styles.SmallBoxHeight) / 2

	// Adjust to content area
	relX := msg.Mouse().X - boxX - 4
	relY := msg.Mouse().Y - boxY - 10 // After logo, title, subtitle, network rows

	switch msg.Button {
	case tea.MouseLeft:
		// Network options row (single line with three options).
		if relY >= -4 && relY <= -3 {
			switch {
			case relX < 20:
				d.network = DaemonNetworkMainnet
			case relX < 40:
				d.network = DaemonNetworkTestnet
			default:
				d.network = DaemonNetworkSimulator
			}
			d.setPlaceholderForNetwork()
			d.input.Focus()
			return d
		}

		// Input field: y=0-1
		if relY >= 0 && relY <= 1 {
			d.input.Focus()
			return d
		}
		// Connect button: y=4 (after error space)
		if relY >= 4 && relY <= 5 && relX >= 15 && relX < 45 {
			addr := d.input.Value()
			if addr == "" {
				addr = d.DefaultAddress()
			} else if !strings.Contains(addr, ":") {
				addr = addr + ":" + d.DefaultPort()
			}
			d.address = addr
			d.confirmed = true
			return d
		}

	case tea.MouseWheelUp, tea.MouseWheelDown:
		// No scrollable content
	}

	return d
}

// SelectedNetwork returns the currently selected network.
func (d DaemonModel) SelectedNetwork() DaemonNetwork {
	return d.network
}

// DefaultPort returns the default port for the selected network.
func (d DaemonModel) DefaultPort() string {
	switch d.network {
	case DaemonNetworkTestnet:
		return "40402"
	case DaemonNetworkSimulator:
		return "20000"
	default:
		return "10102"
	}
}

// DefaultAddress returns the default localhost address for the selected network.
func (d DaemonModel) DefaultAddress() string {
	return "localhost:" + d.DefaultPort()
}

func (d *DaemonModel) setPlaceholderForNetwork() {
	d.input.Placeholder = d.DefaultAddress()
}

func (d *DaemonModel) selectPrevNetwork() {
	if d.network > DaemonNetworkMainnet {
		d.network--
	}
}

func (d *DaemonModel) selectNextNetwork() {
	if d.network < DaemonNetworkSimulator {
		d.network++
	}
}

func (d *DaemonModel) cyclePrevNetwork() {
	if d.network == DaemonNetworkMainnet {
		d.network = DaemonNetworkSimulator
		return
	}
	d.network--
}

func (d *DaemonModel) cycleNextNetwork() {
	if d.network == DaemonNetworkSimulator {
		d.network = DaemonNetworkMainnet
		return
	}
	d.network++
}

func (d DaemonModel) networkOption(network DaemonNetwork, label string, colorStyle lipgloss.Style, port string) string {
	radio := "○"
	if d.network == network {
		radio = "◉"
	}

	radioStyled := colorStyle.Bold(d.network == network).Render(radio)
	labelStyled := colorStyle.Bold(d.network == network).Render(label)
	portStyled := styles.MutedStyle.Render("(" + port + ")")

	return radioStyled + " " + labelStyled + " " + portStyled
}
