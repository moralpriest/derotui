// Copyright 2017-2026 DERO Project. All rights reserved.

package pages

import (
	"log"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/deroproject/dero-wallet-cli/internal/config"
	"github.com/deroproject/dero-wallet-cli/internal/ui/styles"
)

var (
	welcomeUpKeys   = key.NewBinding(key.WithKeys("up", "ctrl+p"))
	welcomeDownKeys = key.NewBinding(key.WithKeys("down", "ctrl+n"))
)

// WelcomeAction represents user selection
type WelcomeAction int

const (
	ActionNone WelcomeAction = iota
	ActionOpen
	ActionCreate
	ActionRestoreSeed
	ActionRestoreKey
	ActionConnectDaemon
	ActionSwitchNetwork
	ActionDebug
	ActionExit
	ActionSetTheme
	ActionPreviewTheme
)

// Command represents a slash command
type Command struct {
	Name        string
	Description string
	Action      WelcomeAction
}

// DaemonStatusInfo represents one daemon status row on welcome page.
type DaemonStatusInfo struct {
	IsOnline    bool
	IsSynced    bool
	IsHealthy   bool
	Network     string
	Address     string
	BlockHeight uint64
	TopoHeight  int64
}

// WelcomeModel represents the welcome screen
type WelcomeModel struct {
	input           textinput.Model
	commands        []Command
	filtered        []Command
	selected        int
	action          WelcomeAction
	showMenu        bool
	inRestoreMenu   bool
	restoreOptions  []Command
	restoreSelected int
	inThemesMenu    bool
	themeOptions    []Command
	themeSelected   int
	selectedTheme   string // Store the selected theme ID
	previousTheme   string // Store theme before entering themes menu
	IsOnline        bool
	IsSynced        bool
	IsHealthy       bool
	Network         string
	DaemonAddress   string
	BlockHeight     uint64
	TopoHeight      int64
	Daemons         []DaemonStatusInfo
	Version         string
	errorMsg        string // Error message to display
	successMsg      string // Success message to display
}

// ActionRestore is used internally to trigger restore submenu
const ActionRestore WelcomeAction = 100

// ActionThemes is used internally to trigger themes submenu
const ActionThemes WelcomeAction = 101

// buildThemeOptions creates theme menu options from available themes
func buildThemeOptions() []Command {
	themes := styles.GetThemes()
	var options []Command
	for id, theme := range themes {
		options = append(options, Command{
			Name:        theme.Name,
			Description: id,
			Action:      ActionSetTheme,
		})
	}
	// Sort by theme name for consistent ordering
	for i := 0; i < len(options); i++ {
		for j := i + 1; j < len(options); j++ {
			if options[i].Name > options[j].Name {
				options[i], options[j] = options[j], options[i]
			}
		}
	}
	return options
}

// NewWelcome creates a new welcome screen
func NewWelcome() WelcomeModel {
	ti := textinput.New()
	ti.Placeholder = "Type / for commands..."
	ti.CharLimit = 64
	ti.SetWidth(40)
	ti.Focus()

	commands := []Command{
		{Name: "/open", Description: "Open an existing wallet", Action: ActionOpen},
		{Name: "/create", Description: "Create a new wallet", Action: ActionCreate},
		{Name: "/restore", Description: "Restore a wallet", Action: ActionRestore},
		{Name: "/themes", Description: "Change color theme", Action: ActionThemes},
		{Name: "/connect", Description: "Connect to a daemon", Action: ActionConnectDaemon},
		{Name: "/debug", Description: "Open debug console", Action: ActionDebug},
		{Name: "/exit", Description: "Exit the application", Action: ActionExit},
	}

	restoreOptions := []Command{
		{Name: "From Seed", Description: "25-word seed words", Action: ActionRestoreSeed},
		{Name: "From Key", Description: "64 character hex key", Action: ActionRestoreKey},
	}

	themeOpts := buildThemeOptions()

	return WelcomeModel{
		input:          ti,
		commands:       commands,
		restoreOptions: restoreOptions,
		themeOptions:   themeOpts,
		filtered:       []Command{},
		selected:       0,
		showMenu:       false,
	}
}

// Init initializes the welcome screen
func (w WelcomeModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles events
func (w WelcomeModel) Update(msg tea.Msg) (WelcomeModel, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		// Handle escape - close menu or go back from restore submenu
		if key.Matches(msg, pageEscKeys) {
			if w.inRestoreMenu {
				w.inRestoreMenu = false
				w.restoreSelected = 0
				w.showMenu = true
				return w, nil
			}
			if w.inThemesMenu {
				// Restore previous theme on escape
				if w.previousTheme != "" {
					w.selectedTheme = w.previousTheme
					w.action = ActionPreviewTheme
				}
				w.inThemesMenu = false
				w.themeSelected = 0
				w.showMenu = true
				return w, nil
			}
			if w.showMenu {
				w.showMenu = false
				w.input.SetValue("")
				w.filtered = []Command{}
				w.selected = 0
				return w, nil
			}
		}

		// Handle restore submenu navigation
		if w.inRestoreMenu {
			switch {
			case key.Matches(msg, welcomeUpKeys):
				if w.restoreSelected > 0 {
					w.restoreSelected--
				}
				return w, nil
			case key.Matches(msg, welcomeDownKeys):
				if w.restoreSelected < len(w.restoreOptions)-1 {
					w.restoreSelected++
				}
				return w, nil
			case key.Matches(msg, pageEnterKeys):
				action := w.restoreOptions[w.restoreSelected].Action
				log.Printf("[DEBUG Welcome] Restore submenu - selected action=%d", action)
				w.action = action
				return w, nil
			}
			return w, nil
		}

		// Handle themes submenu navigation
		if w.inThemesMenu {
			switch {
			case key.Matches(msg, welcomeUpKeys):
				if w.themeSelected > 0 {
					w.themeSelected--
					// Preview the theme
					w.selectedTheme = w.themeOptions[w.themeSelected].Description
					w.action = ActionPreviewTheme
				}
				return w, nil
			case key.Matches(msg, welcomeDownKeys):
				if w.themeSelected < len(w.themeOptions)-1 {
					w.themeSelected++
					// Preview the theme
					w.selectedTheme = w.themeOptions[w.themeSelected].Description
					w.action = ActionPreviewTheme
				}
				return w, nil
			case key.Matches(msg, pageEnterKeys):
				selectedTheme := w.themeOptions[w.themeSelected].Description
				w.selectedTheme = selectedTheme
				w.action = ActionSetTheme
				w.inThemesMenu = false
				return w, nil
			}
			return w, nil
		}

		// Handle navigation in menu
		if w.showMenu && len(w.filtered) > 0 {
			switch {
			case key.Matches(msg, welcomeUpKeys):
				if w.selected > 0 {
					w.selected--
				}
				return w, nil
			case key.Matches(msg, welcomeDownKeys):
				if w.selected < len(w.filtered)-1 {
					w.selected++
				}
				return w, nil
			case key.Matches(msg, pageTabKeys):
				// Autocomplete selected command
				if w.selected < len(w.filtered) {
					w.input.SetValue(w.filtered[w.selected].Name)
					w.input.SetCursor(len(w.filtered[w.selected].Name))
				}
				return w, nil
			case key.Matches(msg, pageEnterKeys):
				// Execute selected command
				if w.selected < len(w.filtered) {
					selectedAction := w.filtered[w.selected].Action
					log.Printf("[DEBUG Welcome] Menu - selected action=%d", selectedAction)
					if selectedAction == ActionRestore {
						w.inRestoreMenu = true
						w.restoreSelected = 0
						w.showMenu = false
						return w, nil
					}
					if selectedAction == ActionThemes {
						w.inThemesMenu = true
						w.themeSelected = 0
						w.showMenu = false
						// Store current theme before entering themes menu
						w.previousTheme = config.GetTheme()
						// Preview the first theme immediately
						if len(w.themeOptions) > 0 {
							w.selectedTheme = w.themeOptions[0].Description
							w.action = ActionPreviewTheme
						}
						return w, nil
					}
					log.Printf("[DEBUG Welcome] Menu selected action=%d (%s)", selectedAction, w.filtered[w.selected].Name)
					// Clear input immediately when command is selected
					w.input.SetValue("")
					w.input.Reset()
					w.showMenu = false
					w.filtered = []Command{}
					w.action = selectedAction
				}
				return w, nil
			}
		}

		// Handle enter without menu - check for exact command match
		if key.Matches(msg, pageEnterKeys) && !w.showMenu {
			val := strings.TrimSpace(w.input.Value())
			log.Printf("[DEBUG Welcome] Enter pressed")
			for _, c := range w.commands {
				if c.Name == val {
					log.Printf("[DEBUG Welcome] Direct command match: %s -> action=%d", c.Name, c.Action)
					if c.Action == ActionRestore {
						w.inRestoreMenu = true
						w.restoreSelected = 0
						return w, nil
					}
					if c.Action == ActionThemes {
						w.inThemesMenu = true
						w.themeSelected = 0
						// Store current theme before entering themes menu
						w.previousTheme = config.GetTheme()
						// Preview the first theme immediately
						if len(w.themeOptions) > 0 {
							w.selectedTheme = w.themeOptions[0].Description
							w.action = ActionPreviewTheme
						}
						return w, nil
					}
					// Clear input immediately when command is matched
					w.input.SetValue("")
					w.input.Reset()
					w.action = c.Action
					return w, nil
				}
			}
			log.Printf("[DEBUG Welcome] No command match")
		}

		// Quick-fix shortcuts (shown with welcome alert)
		if !w.inRestoreMenu && !w.showMenu && strings.TrimSpace(w.input.Value()) == "" && w.errorMsg != "" {
			switch {
			case key.Matches(msg, pageCopyKeys):
				w.action = ActionConnectDaemon
				return w, nil
			}
		}

	}

	// Don't update input when in restore submenu
	if w.inRestoreMenu {
		return w, nil
	}

	// Update text input
	prevValue := w.input.Value()
	w.input, cmd = w.input.Update(msg)
	newValue := w.input.Value()

	// Check if value changed
	if newValue != prevValue {
		// Filter commands based on input
		if strings.HasPrefix(newValue, "/") {
			w.showMenu = true
			w.filtered = w.filterCommands(newValue)
			w.selected = 0
			log.Printf("[DEBUG Welcome] Filtering commands for '%s', found %d matches", newValue, len(w.filtered))
		} else {
			w.showMenu = false
			w.filtered = []Command{}
			w.selected = 0
		}
	}

	return w, cmd
}

// filterCommands returns commands that match the input
func (w WelcomeModel) filterCommands(input string) []Command {
	var result []Command
	input = strings.ToLower(input)
	for _, c := range w.commands {
		if strings.HasPrefix(strings.ToLower(c.Name), input) {
			result = append(result, c)
		}
	}
	return result
}

// View renders the welcome screen
func (w WelcomeModel) View() string {
	logo := styles.Logo()
	subtitle := styles.MutedStyle.Render("Private. Secure. Decentralized.")

	metaRows := []string{}
	if len(w.Daemons) > 0 {
		for _, daemon := range w.Daemons {
			metaRows = append(metaRows, renderDaemonSummaryLine(daemon))
		}
	} else {
		metaRows = append(metaRows, renderDaemonSummaryLine(DaemonStatusInfo{
			IsOnline:    w.IsOnline,
			IsSynced:    w.IsSynced,
			IsHealthy:   w.IsHealthy,
			Network:     w.Network,
			Address:     w.DaemonAddress,
			BlockHeight: w.BlockHeight,
			TopoHeight:  w.TopoHeight,
		}))
	}

	metaStrip := lipgloss.JoinVertical(lipgloss.Center, metaRows...)

	// Input with styling
	inputStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.ColorPrimary).
		Padding(0, 1).
		Width(44)
	inputView := inputStyle.Render(w.input.View())

	// Command menu (shown when typing /)
	var menuView string
	if w.inRestoreMenu {
		// Show restore submenu
		menuStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(styles.ColorPrimary).
			Padding(0, 1).
			Width(44)

		title := styles.TitleStyle.Render("Restore Wallet")
		restoreLabels := make([]string, 0, len(w.restoreOptions))
		for _, c := range w.restoreOptions {
			restoreLabels = append(restoreLabels, c.Name)
		}
		restoreLabelWidth := maxLabelWidth(restoreLabels)

		var menuItems []string
		menuItems = append(menuItems, title, "")
		for i, c := range w.restoreOptions {
			namePadded := padLabel(c.Name, restoreLabelWidth)
			if i == w.restoreSelected {
				rowPlain := namePadded + " - " + c.Description
				item := styles.SelectedMenuItemStyle.Render("▸ ") + selectedMenuRow(rowPlain, 40)
				menuItems = append(menuItems, item)
			} else {
				// Unselected: no arrow + muted text
				desc := styles.MutedStyle.Render(" - " + c.Description)
				cmdName := styles.MutedStyle.Render(namePadded)
				item := "  " + cmdName + desc
				menuItems = append(menuItems, item)
			}
		}
		menuView = menuStyle.Render(lipgloss.JoinVertical(lipgloss.Left, menuItems...))
	} else if w.inThemesMenu {
		// Show themes submenu
		menuStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(styles.ColorPrimary).
			Padding(0, 1).
			Width(44)

		title := styles.TitleStyle.Render("Select Theme")
		themeLabels := make([]string, 0, len(w.themeOptions))
		for _, c := range w.themeOptions {
			themeLabels = append(themeLabels, c.Name)
		}
		themeLabelWidth := maxLabelWidth(themeLabels)

		currentTheme := config.GetTheme()

		var menuItems []string
		menuItems = append(menuItems, title, "")
		for i, c := range w.themeOptions {
			isCurrent := c.Description == currentTheme
			namePadded := padLabel(c.Name, themeLabelWidth)
			if i == w.themeSelected {
				var rowPlain string
				if isCurrent {
					rowPlain = namePadded + " ✓"
				} else {
					rowPlain = namePadded
				}
				item := styles.SelectedMenuItemStyle.Render("▸ ") + selectedMenuRow(rowPlain, 40)
				menuItems = append(menuItems, item)
			} else {
				// Unselected: no arrow + muted text
				cmdName := styles.MutedStyle.Render(namePadded)
				var item string
				if isCurrent {
					item = "  " + cmdName + styles.SuccessStyle.Render(" ✓")
				} else {
					item = "  " + cmdName
				}
				menuItems = append(menuItems, item)
			}
		}
		menuView = menuStyle.Render(lipgloss.JoinVertical(lipgloss.Left, menuItems...))
	} else if w.showMenu && len(w.filtered) > 0 {
		menuStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(styles.ColorBorder).
			Padding(0, 1).
			Width(44)

		filteredLabels := make([]string, 0, len(w.filtered))
		for _, c := range w.filtered {
			filteredLabels = append(filteredLabels, c.Name)
		}
		filteredLabelWidth := maxLabelWidth(filteredLabels)

		var menuItems []string
		menuItems = append(menuItems, styles.TitleStyle.Render("Slash Commands"), "")
		for i, c := range w.filtered {
			namePadded := padLabel(c.Name, filteredLabelWidth)
			if i == w.selected {
				rowPlain := namePadded + " - " + c.Description
				item := styles.SelectedMenuItemStyle.Render("▸ ") + selectedMenuRow(rowPlain, 40)
				menuItems = append(menuItems, item)
			} else {
				// Unselected: no arrow + muted text
				desc := styles.MutedStyle.Render(" - " + c.Description)
				cmdName := styles.MutedStyle.Render(namePadded)
				item := "  " + cmdName + desc
				menuItems = append(menuItems, item)
			}
		}
		menuView = menuStyle.Render(lipgloss.JoinVertical(lipgloss.Left, menuItems...))
	}

	// Help
	var help string
	if w.inRestoreMenu {
		help = styles.MutedStyle.Render("↑↓ Navigate • Enter Select • Esc Cancel")
	} else if w.inThemesMenu {
		help = styles.MutedStyle.Render("↑↓ Navigate • Enter Select • Esc Cancel • ✓ Current")
	} else if w.showMenu {
		help = styles.MutedStyle.Render("↑↓ Navigate • Tab Complete • Enter Select • Esc Cancel")
	} else {
		help = styles.MutedStyle.Render("Type / for Commands • F12 Debug")
	}

	// Compose
	elements := []string{
		logo,
		"",
		subtitle,
		"",
		metaStrip,
		"",
		inputView,
	}

	if menuView != "" {
		elements = append(elements, menuView)
	}

	elements = append(elements, "", help)

	alertLine := ""
	if w.errorMsg != "" {
		alertLine = "✗ " + w.errorMsg
		quickFix := styles.MutedStyle.Render("Quick fix: ") +
			styles.TextStyle.Render("[C]") + styles.MutedStyle.Render(" Connect daemon")
		elements = append(elements, "", alertLine, quickFix)
	} else if w.successMsg != "" {
		alertLine = "✓ " + w.successMsg
		elements = append(elements, "", alertLine)
	}

	content := lipgloss.JoinVertical(lipgloss.Center, elements...)
	if alertLine != "" {
		if w.errorMsg != "" {
			content = strings.Replace(content, alertLine, styles.ErrorStyle.Render(alertLine), 1)
		} else if w.successMsg != "" {
			successLine := "✓ " + w.successMsg
			content = strings.Replace(content, alertLine, styles.SuccessStyle.Render(successLine), 1)
		}
	}

	// Build custom frame with embedded version in top border
	boxWidth := styles.Width
	horizontalPadding := 4
	innerWidth := boxWidth - 2 - (horizontalPadding * 2) // 80 - 2 - 8 = 70

	versionStr := "v" + w.Version

	// Build top border with version embedded near the right corner
	// Format: ╭───────────────── v0.1.0 ──╮
	totalDashes := boxWidth - 4 - len(versionStr)
	leftDashes := totalDashes - 4 // Most dashes on left
	rightDashes := 4              // Just 4 dashes before corner

	// Build the styled top border
	cornerStyle := lipgloss.NewStyle().Foreground(styles.ColorBorder)
	dashStyle := lipgloss.NewStyle().Foreground(styles.ColorBorder)
	leftCorner := cornerStyle.Render("╭")
	rightCorner := cornerStyle.Render("╮")
	leftDashStr := dashStyle.Render(strings.Repeat("─", leftDashes))
	rightDashStr := dashStyle.Render(strings.Repeat("─", rightDashes))
	versionStyled := styles.MutedStyle.Render(versionStr)

	topBorder := leftCorner + leftDashStr + " " + versionStyled + " " + rightDashStr + rightCorner

	// Build side borders for content with padding and centering
	borderStyle := lipgloss.NewStyle().Foreground(styles.ColorBorder)
	paddingStr := strings.Repeat(" ", horizontalPadding)
	var framedLines []string
	contentLines := strings.Split(content, "\n")
	for _, line := range contentLines {
		visibleLen := lipgloss.Width(line)
		// Center the content within innerWidth
		if visibleLen < innerWidth {
			leftPad := (innerWidth - visibleLen) / 2
			rightPad := innerWidth - visibleLen - leftPad
			line = strings.Repeat(" ", leftPad) + line + strings.Repeat(" ", rightPad)
		}
		sideBorder := borderStyle.Render("│")
		framedLines = append(framedLines, sideBorder+paddingStr+line+paddingStr+sideBorder)
	}

	// Build bottom border
	bottomBorder := cornerStyle.Render("╰") + dashStyle.Render(strings.Repeat("─", boxWidth-2)) + cornerStyle.Render("╯")

	// Add top/bottom padding rows (2 rows each)
	sideBorder := borderStyle.Render("│")
	emptyContent := strings.Repeat(" ", boxWidth-2)
	paddingRow := sideBorder + emptyContent + sideBorder

	var allLines []string
	allLines = append(allLines, topBorder)
	for i := 0; i < 2; i++ { // Top padding
		allLines = append(allLines, paddingRow)
	}
	allLines = append(allLines, framedLines...)
	for i := 0; i < 2; i++ { // Bottom padding
		allLines = append(allLines, paddingRow)
	}
	allLines = append(allLines, bottomBorder)

	return strings.Join(allLines, "\n")
}

func renderDaemonSummaryLine(daemon DaemonStatusInfo) string {
	networkLabel := daemon.Network
	if networkLabel == "" {
		networkLabel = "Mainnet"
	}
	var networkStyled string
	switch networkLabel {
	case "Simulator":
		networkStyled = styles.SimulatorStyle.Render(networkLabel)
	case "Testnet":
		networkStyled = styles.TestnetStyle.Render(networkLabel)
	default:
		networkStyled = styles.SuccessStyle.Render(networkLabel)
	}

	addressLabel := "Not configured"
	if daemon.Address != "" {
		addressLabel = truncateWelcomeAddress(daemon.Address, 16)
	}

	var statusStyled string
	if daemon.IsOnline && daemon.IsHealthy {
		if daemon.IsSynced {
			statusStyled = styles.SuccessStyle.Render("●")
		} else {
			statusStyled = styles.WarningStyle.Render("●")
		}
	} else if daemon.IsOnline && !daemon.IsHealthy {
		statusStyled = styles.WarningStyle.Render("●")
	} else {
		statusStyled = styles.ErrorStyle.Render("●")
	}

	blockStyled := styles.MutedStyle.Render("-")
	if daemon.BlockHeight > 0 {
		blockStr := formatBlockHeight(daemon.BlockHeight)
		if daemon.IsOnline && daemon.IsHealthy && daemon.IsSynced {
			blockStyled = styles.SuccessStyle.Render(blockStr)
		} else if daemon.IsOnline {
			blockStyled = styles.WarningStyle.Render(blockStr)
		} else {
			blockStyled = styles.MutedStyle.Render(blockStr)
		}
	}

	return styles.MutedStyle.Render("Network:") + networkStyled +
		" " + statusStyled +
		"   " + styles.MutedStyle.Render("Daemon:") + styles.TextStyle.Render(addressLabel) +
		"   " + styles.MutedStyle.Render("Height:") + blockStyled
}

// Action returns the selected action
func (w WelcomeModel) Action() WelcomeAction {
	return w.action
}

// ResetAction clears the action
func (w *WelcomeModel) ResetAction() {
	w.action = ActionNone
}

// ResetInput clears the input field
func (w *WelcomeModel) ResetInput() {
	w.input.Reset()
	w.showMenu = false
	w.filtered = []Command{}
	w.selected = 0
}

// SetDaemonStatus sets the daemon connection status
func (w *WelcomeModel) SetDaemonStatus(isOnline bool, isSynced bool, isHealthy bool, network string, address string, blockHeight uint64, topoHeight int64) {
	w.IsOnline = isOnline
	w.IsSynced = isSynced
	w.IsHealthy = isHealthy
	w.Network = network
	w.DaemonAddress = address
	w.BlockHeight = blockHeight
	w.TopoHeight = topoHeight
	w.Daemons = []DaemonStatusInfo{{
		IsOnline:    isOnline,
		IsSynced:    isSynced,
		IsHealthy:   isHealthy,
		Network:     network,
		Address:     address,
		BlockHeight: blockHeight,
		TopoHeight:  topoHeight,
	}}
}

// SetDaemonStatuses sets one or more daemon status rows.
func (w *WelcomeModel) SetDaemonStatuses(daemons []DaemonStatusInfo) {
	w.Daemons = append([]DaemonStatusInfo(nil), daemons...)
	if len(w.Daemons) == 0 {
		w.IsOnline = false
		w.IsSynced = false
		w.IsHealthy = false
		w.Network = ""
		w.DaemonAddress = ""
		w.BlockHeight = 0
		w.TopoHeight = 0
		return
	}
	primary := w.Daemons[0]
	w.IsOnline = primary.IsOnline
	w.IsSynced = primary.IsSynced
	w.IsHealthy = primary.IsHealthy
	w.Network = primary.Network
	w.DaemonAddress = primary.Address
	w.BlockHeight = primary.BlockHeight
	w.TopoHeight = primary.TopoHeight
}

// SetError sets an error message to display on the welcome screen
func (w *WelcomeModel) SetError(msg string) {
	w.errorMsg = msg
}

// SetSuccess sets a success message to display on the welcome screen
func (w *WelcomeModel) SetSuccess(msg string) {
	w.successMsg = msg
}

// SelectedTheme returns the selected theme ID
func (w WelcomeModel) SelectedTheme() string {
	return w.selectedTheme
}

// HandleMouse handles mouse events on the welcome screen
func (w *WelcomeModel) HandleMouse(msg tea.MouseClickMsg, windowWidth, windowHeight int) bool {
	// Only handle left clicks
	if msg.Button != tea.MouseLeft {
		return false
	}

	// Calculate the centered box position
	boxWidth := styles.Width
	boxHeight := styles.MediumBoxHeight // Approximate height
	boxX := (windowWidth - boxWidth) / 2
	boxY := (windowHeight - boxHeight) / 2

	// Adjust coordinates to be relative to the box content area
	mouseX, mouseY := msg.Mouse().X, msg.Mouse().Y
	relX := mouseX - boxX - 4 // Account for padding
	relY := mouseY - boxY - 2 // Account for padding and logo

	// Check if in restore submenu
	if w.inRestoreMenu {
		// Menu items start around y=13, each item is 1 line
		menuStartY := 13
		itemIndex := relY - menuStartY
		if itemIndex >= 0 && itemIndex < len(w.restoreOptions) {
			w.restoreSelected = itemIndex
			if msg.Button == tea.MouseLeft {
				w.action = w.restoreOptions[itemIndex].Action
				return true
			}
		}
		return false
	}

	// Check if in themes submenu
	if w.inThemesMenu {
		// Menu items start around y=13, each item is 1 line
		menuStartY := 13
		itemIndex := relY - menuStartY
		if itemIndex >= 0 && itemIndex < len(w.themeOptions) {
			w.themeSelected = itemIndex
			if msg.Button == tea.MouseLeft {
				w.selectedTheme = w.themeOptions[itemIndex].Description
				w.action = ActionSetTheme
				return true
			}
		}
		return false
	}

	// Check if showing main command menu
	if w.showMenu && len(w.filtered) > 0 {
		// Menu items start around y=13, each item is 1 line
		menuStartY := 13
		itemIndex := relY - menuStartY
		if itemIndex >= 0 && itemIndex < len(w.filtered) {
			w.selected = itemIndex
			if msg.Button == tea.MouseLeft {
				selectedAction := w.filtered[itemIndex].Action
				if selectedAction == ActionRestore {
					w.inRestoreMenu = true
					w.restoreSelected = 0
					w.showMenu = false
				} else if selectedAction == ActionThemes {
					w.inThemesMenu = true
					w.themeSelected = 0
					w.showMenu = false
				} else {
					// Clear input immediately when command is selected via mouse
					w.input.SetValue("")
					w.input.Reset()
					w.showMenu = false
					w.filtered = []Command{}
					w.action = selectedAction
				}
				return true
			}
		}
		return false
	}

	// Check if clicking on input field
	// Input field is at approximately y=10-11
	if relY >= 10 && relY <= 11 && relX >= 0 && relX < 44 {
		// Focus input - handled by textinput component
		return false
	}

	return false
}

// formatBlockHeight formats block height for display
func formatBlockHeight(n uint64) string {
	return styles.UintToStr(n)
}

func truncateWelcomeAddress(addr string, max int) string {
	addr = stripDaemonScheme(addr)
	if max <= 3 || len(addr) <= max {
		return addr
	}
	return addr[:max-3] + "..."
}

func selectedMenuRow(text string, width int) string {
	return styles.SelectedRowStyle.Render(fitCellLeft(text, width))
}
