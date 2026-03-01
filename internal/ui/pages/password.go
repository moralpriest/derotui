// Copyright 2017-2026 DERO Project. All rights reserved.

package pages

import (
	"log"
	"os"
	"strings"
	"unicode"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/deroproject/dero-wallet-cli/internal/ui/components"
	"github.com/deroproject/dero-wallet-cli/internal/ui/styles"
)

var (
	passwordRevealKeys = key.NewBinding(key.WithKeys("f2"))
)

// PasswordMode represents the password screen mode
type PasswordMode int

const (
	PasswordModeUnlock PasswordMode = iota
	PasswordModeCreate
	PasswordModeConfirm
	PasswordModeChange
)

// PasswordModel represents the password screen
type PasswordModel struct {
	Mode            PasswordMode
	walletNameInput components.InputModel // For create mode: wallet filename
	currentInput    components.InputModel // For change mode: current password
	input           components.InputModel // Main password input (or new password in change mode)
	confirmInput    components.InputModel
	password        string
	currentPassword string // Snapshot of current password for change mode
	walletName      string // Snapshot of wallet name for create mode
	confirmed       bool
	cancelled       bool
	error           string
	focusField      int // 0=name/current/input, 1=input/confirm, 2=confirm (for create/change mode)
	walletFile      string
	Version         string // Version string to display in frame
}

// NewPassword creates a new password screen
func NewPassword(mode PasswordMode) PasswordModel {
	m := PasswordModel{
		Mode:       mode,
		focusField: 0,
	}

	if mode == PasswordModeChange {
		m.currentInput = components.NewInput("Current Password", "Enter current password...", true)
		m.input = components.NewInput("New Password", "Enter new password...", true)
		m.confirmInput = components.NewInput("Confirm New Password", "Confirm new password...", true)
		m.currentInput.Focus()
	} else if mode == PasswordModeCreate {
		m.walletNameInput = components.NewInput("Wallet Name", "wallet.db", false)
		m.walletNameInput.SetValue("wallet.db")
		m.input = components.NewInput("Password", "Enter password...", true)
		m.confirmInput = components.NewInput("Confirm Password", "Confirm password...", true)
		m.walletNameInput.Focus()
	} else {
		m.input = components.NewInput("Password", "Enter password...", true)
		m.input.Focus()
	}

	return m
}

// Init initializes the password screen
func (p PasswordModel) Init() tea.Cmd {
	if p.Mode == PasswordModeChange {
		return p.currentInput.Init()
	}
	if p.Mode == PasswordModeCreate {
		return p.walletNameInput.Init()
	}
	return p.input.Init()
}

// Update handles events
func (p PasswordModel) Update(msg tea.Msg) (PasswordModel, tea.Cmd) {
	var cmd tea.Cmd

	// Store error at start - preserve it unless explicitly cleared
	currentError := p.error

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		// Handle escape first
		if key.Matches(msg, pageEscKeys) {
			log.Printf("[DEBUG password] Cancelled password entry (mode=%d)", p.Mode)
			p.cancelled = true
			return p, nil
		}

		switch {
		case key.Matches(msg, pageTabKeys):
			return p.handleTab()

		case key.Matches(msg, pageEnterKeys):
			return p.handleEnter()

		case key.Matches(msg, passwordRevealKeys):
			// Toggle reveal on all password inputs
			p.currentInput.ToggleReveal()
			p.input.ToggleReveal()
			p.confirmInput.ToggleReveal()
			log.Printf("[DEBUG password] Toggled password reveal")
			return p, nil
		}

		// Other key press - update focused input and clear error (user is typing)
		cmd = p.updateFocusedInput(msg)
		p.error = "" // Clear error when typing
		return p, cmd
	}

	// Non-KeyMsg (like cursor blink) - update focused input but preserve error
	cmd = p.updateFocusedInput(msg)
	p.error = currentError // Preserve error

	return p, cmd
}

// handleTab cycles through input fields
func (p PasswordModel) handleTab() (PasswordModel, tea.Cmd) {
	var cmd tea.Cmd

	if p.Mode == PasswordModeChange {
		// 3 fields: current(0) -> new(1) -> confirm(2) -> current(0)
		p.blurAll()
		p.focusField = (p.focusField + 1) % 3
		switch p.focusField {
		case 0:
			cmd = p.currentInput.Focus()
		case 1:
			cmd = p.input.Focus()
		case 2:
			cmd = p.confirmInput.Focus()
		}
	} else if p.Mode == PasswordModeCreate {
		// 3 fields: name(0) -> password(1) -> confirm(2) -> name(0)
		p.blurAll()
		p.focusField = (p.focusField + 1) % 3
		switch p.focusField {
		case 0:
			cmd = p.walletNameInput.Focus()
		case 1:
			cmd = p.input.Focus()
		case 2:
			cmd = p.confirmInput.Focus()
		}
	}
	// Unlock mode has only 1 field, Tab does nothing

	return p, cmd
}

// handleEnter validates and confirms or moves to next field
func (p PasswordModel) handleEnter() (PasswordModel, tea.Cmd) {
	var cmd tea.Cmd

	if p.Mode == PasswordModeChange {
		// 3 fields: must be on confirm field to submit
		if p.focusField < 2 {
			// Move to next field
			p.blurAll()
			p.focusField++
			if p.focusField == 1 {
				cmd = p.input.Focus()
			} else {
				cmd = p.confirmInput.Focus()
			}
			return p, cmd
		}
		// On confirm field - validate all
		if len(p.currentInput.Value()) < 1 {
			p.error = "Current password cannot be empty"
			return p, nil
		}
		if len(p.input.Value()) < 1 {
			p.error = "New password cannot be empty"
			return p, nil
		}
		if p.input.Value() != p.confirmInput.Value() {
			p.error = "New passwords do not match"
			return p, nil
		}
		if p.currentInput.Value() == p.input.Value() {
			p.error = "New password must be different from current"
			return p, nil
		}
		p.password = p.input.Value()
		p.currentPassword = p.currentInput.Value() // Store snapshot of current password
		p.confirmed = true
		log.Printf("[DEBUG password] Password change confirmed")
		return p, nil

	} else if p.Mode == PasswordModeCreate {
		// 3 fields: name(0) -> password(1) -> confirm(2)
		if p.focusField < 2 {
			// Move to next field
			p.blurAll()
			p.focusField++
			if p.focusField == 1 {
				cmd = p.input.Focus()
			} else {
				cmd = p.confirmInput.Focus()
			}
			return p, cmd
		}
		// On confirm field - validate all fields
		// Validate wallet name
		walletName := strings.TrimSpace(p.walletNameInput.Value())
		if walletName == "" {
			p.error = "Wallet name cannot be empty"
			return p, nil
		}
		// Check for invalid filename characters
		invalidChars := `/\:*?"<>|`
		for _, c := range invalidChars {
			if strings.ContainsRune(walletName, c) {
				p.error = "Wallet name contains invalid characters"
				return p, nil
			}
		}
		// Auto-append .db if not present
		if !strings.HasSuffix(strings.ToLower(walletName), ".db") {
			walletName = walletName + ".db"
		}
		// Check if file already exists
		if _, err := os.Stat(walletName); err == nil {
			p.error = "A wallet with this name already exists"
			return p, nil
		}
		// Validate passwords
		if p.input.Value() != p.confirmInput.Value() {
			p.error = "Passwords do not match"
			return p, nil
		}
		if len(p.input.Value()) < 1 {
			p.error = "Password cannot be empty"
			return p, nil
		}
		p.walletName = walletName
		p.password = p.input.Value()
		p.confirmed = true
		log.Printf("[DEBUG password] Wallet creation confirmed: %s", walletName)
		return p, nil

	} else {
		// Unlock mode
		if len(p.input.Value()) < 1 {
			p.error = "Password cannot be empty"
			return p, nil
		}
		p.password = p.input.Value()
		p.confirmed = true
		log.Printf("[DEBUG password] Password submitted for unlock")
		return p, nil
	}
}

// blurAll blurs all input fields
func (p *PasswordModel) blurAll() {
	p.walletNameInput.Blur()
	p.currentInput.Blur()
	p.input.Blur()
	p.confirmInput.Blur()
}

// updateFocusedInput updates the currently focused input
func (p *PasswordModel) updateFocusedInput(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd

	if p.Mode == PasswordModeChange {
		switch p.focusField {
		case 0:
			p.currentInput, cmd = p.currentInput.Update(msg)
		case 1:
			p.input, cmd = p.input.Update(msg)
		case 2:
			p.confirmInput, cmd = p.confirmInput.Update(msg)
		}
	} else if p.Mode == PasswordModeCreate {
		switch p.focusField {
		case 0:
			p.walletNameInput, cmd = p.walletNameInput.Update(msg)
		case 1:
			p.input, cmd = p.input.Update(msg)
		case 2:
			p.confirmInput, cmd = p.confirmInput.Update(msg)
		}
	} else {
		p.input, cmd = p.input.Update(msg)
	}

	return cmd
}

// View renders the password screen
func (p PasswordModel) View() string {
	// Logo
	logo := styles.Logo()

	// Title
	var title string
	switch p.Mode {
	case PasswordModeUnlock:
		title = "Unlock Wallet"
	case PasswordModeCreate:
		title = "Create Wallet Password"
	case PasswordModeConfirm:
		title = "Confirm Password"
	case PasswordModeChange:
		title = "Change Wallet Password"
	}

	titleStyled := styles.TitleStyle.Render(title)

	// Subtitle
	var subtitle string
	switch p.Mode {
	case PasswordModeUnlock:
		if p.walletFile != "" {
			subtitle = p.walletFile
		} else {
			subtitle = "Enter your wallet password to continue"
		}
	case PasswordModeCreate:
		subtitle = "Create a strong password for your wallet"
	case PasswordModeChange:
		subtitle = "Enter your current password, then create a new one"
	}
	subtitleStyled := styles.MutedStyle.Render(subtitle)

	// Input(s)
	var inputView string
	if p.Mode == PasswordModeChange {
		inputView = p.currentInput.View() + "\n" + p.input.View()
		// Add password complexity indicator below new password
		complexity, complexityStyle := p.getPasswordComplexity(p.input.Value())
		if complexity != "" {
			inputView += "\n" + complexityStyle.Render("  Strength: "+complexity)
		}
		inputView += "\n" + p.confirmInput.View()
	} else if p.Mode == PasswordModeCreate {
		// Show wallet name input first, then password fields
		inputView = p.walletNameInput.View() + "\n" + p.input.View()
		// Add password complexity indicator
		complexity, complexityStyle := p.getPasswordComplexity(p.input.Value())
		if complexity != "" {
			inputView += "\n" + complexityStyle.Render("  Strength: "+complexity)
		}
		inputView += "\n" + p.confirmInput.View()
	} else {
		inputView = p.input.View()
	}

	// Error
	var errorView string
	if p.error != "" {
		errorView = "\n" + styles.ErrorStyle.Render("✗ "+p.error)
	}

	// Help
	var help string
	if p.Mode == PasswordModeCreate || p.Mode == PasswordModeChange {
		help = "Tab • F2 Reveal • Enter Confirm • Esc Cancel"
	} else {
		help = "F2 Reveal • Enter Confirm • Esc Cancel"
	}
	helpStyled := styles.MutedStyle.Render(help)

	// Compose
	content := lipgloss.JoinVertical(lipgloss.Center,
		logo,
		"",
		titleStyled,
		subtitleStyled,
		"",
		inputView,
		errorView,
		"",
		helpStyled,
	)

	// Build custom frame with embedded version in top border
	boxWidth := styles.Width
	horizontalPadding := 4
	innerWidth := boxWidth - 2 - (horizontalPadding * 2) // 80 - 2 - 8 = 70

	versionStr := "v" + p.Version

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

// getPasswordComplexity returns the complexity label and style for a password
func (p PasswordModel) getPasswordComplexity(password string) (string, lipgloss.Style) {
	if len(password) == 0 {
		return "", lipgloss.NewStyle()
	}

	var hasUpper, hasLower, hasNumber, hasSymbol bool
	for _, c := range password {
		switch {
		case unicode.IsUpper(c):
			hasUpper = true
		case unicode.IsLower(c):
			hasLower = true
		case unicode.IsNumber(c):
			hasNumber = true
		case unicode.IsPunct(c) || unicode.IsSymbol(c):
			hasSymbol = true
		}
	}

	mixCount := 0
	if hasUpper {
		mixCount++
	}
	if hasLower {
		mixCount++
	}
	if hasNumber {
		mixCount++
	}
	if hasSymbol {
		mixCount++
	}

	length := len(password)

	// Determine complexity
	if length >= 12 && mixCount >= 3 {
		return "Very Strong", styles.AccentStyle
	}
	if length >= 8 && mixCount >= 2 {
		return "Strong", styles.SuccessStyle
	}
	if length >= 6 {
		return "Fair", styles.WarningStyle
	}
	return "Weak", styles.ErrorStyle
}

// Password returns the entered password
func (p PasswordModel) Password() string {
	return p.password
}

// CurrentPassword returns the current password (for change mode)
func (p PasswordModel) CurrentPassword() string {
	return p.currentPassword
}

// WalletName returns the wallet filename (for create mode)
func (p PasswordModel) WalletName() string {
	return p.walletName
}

// Confirmed returns whether password was confirmed
func (p PasswordModel) Confirmed() bool {
	return p.confirmed
}

// Cancelled returns whether user cancelled
func (p PasswordModel) Cancelled() bool {
	return p.cancelled
}

// SetError sets an error message
func (p *PasswordModel) SetError(err string) {
	p.error = err
	p.confirmed = false
}

// ClearConfirmed clears the confirmed flag to prevent re-triggering
func (p *PasswordModel) ClearConfirmed() {
	p.confirmed = false
}

// SetWalletFile sets the wallet file path to display
func (p *PasswordModel) SetWalletFile(path string) {
	p.walletFile = path
}

// SetVersion sets the version string to display in the frame
func (p *PasswordModel) SetVersion(version string) {
	p.Version = version
}

// Reset resets the password screen
func (p *PasswordModel) Reset() {
	p.walletNameInput.Reset()
	if p.Mode == PasswordModeCreate {
		p.walletNameInput.SetValue("wallet.db")
	}
	p.currentInput.Reset()
	p.input.Reset()
	p.confirmInput.Reset()
	p.password = ""
	p.currentPassword = ""
	p.walletName = ""
	p.confirmed = false
	p.cancelled = false
	p.error = ""
	p.focusField = 0
	if p.Mode == PasswordModeChange {
		p.currentInput.Focus()
	} else if p.Mode == PasswordModeCreate {
		p.walletNameInput.Focus()
	} else {
		p.input.Focus()
	}
}

// HandleMouse handles mouse events on the password screen
func (p PasswordModel) HandleMouse(msg tea.MouseClickMsg, windowWidth, windowHeight int) PasswordModel {
	// Calculate centered position
	boxWidth := styles.Width
	boxX := (windowWidth - boxWidth) / 2
	boxY := (windowHeight - styles.MediumBoxHeight) / 2

	// Adjust to content area (ignore X for now, used for future enhancements)
	_ = msg.X - boxX - 4
	relY := msg.Mouse().Y - boxY - 8 // After logo and title

	switch msg.Button {
	case tea.MouseLeft:
		// Field height varies by mode
		if p.Mode == PasswordModeUnlock {
			// Single password field at y=0-1
			if relY >= 0 && relY <= 2 {
				p.blurAll()
				p.focusField = 0
				p.input.Focus()
				return p
			}
			// Confirm button at y=4
			if relY == 4 {
				if len(p.input.Value()) < 1 {
					p.error = "Password cannot be empty"
					return p
				}
				p.password = p.input.Value()
				p.confirmed = true
				return p
			}
		} else if p.Mode == PasswordModeCreate {
			// Wallet name: y=0-1
			if relY >= 0 && relY <= 1 {
				p.blurAll()
				p.focusField = 0
				p.walletNameInput.Focus()
				return p
			}
			// Password: y=3-4
			if relY >= 3 && relY <= 4 {
				p.blurAll()
				p.focusField = 1
				p.input.Focus()
				return p
			}
			// Confirm: y=6-7
			if relY >= 6 && relY <= 7 {
				p.blurAll()
				p.focusField = 2
				p.confirmInput.Focus()
				return p
			}
			// Confirm button at y=9
			if relY == 9 {
				newP, _ := p.handleEnter()
				return newP
			}
		} else if p.Mode == PasswordModeChange {
			// Current password: y=0-1
			if relY >= 0 && relY <= 1 {
				p.blurAll()
				p.focusField = 0
				p.currentInput.Focus()
				return p
			}
			// New password: y=3-4
			if relY >= 3 && relY <= 4 {
				p.blurAll()
				p.focusField = 1
				p.input.Focus()
				return p
			}
			// Confirm: y=6-7
			if relY >= 6 && relY <= 7 {
				p.blurAll()
				p.focusField = 2
				p.confirmInput.Focus()
				return p
			}
			// Confirm button at y=9
			if relY == 9 {
				newP, _ := p.handleEnter()
				return newP
			}
		}

	case tea.MouseWheelUp:
		// Tab through fields with scroll wheel - cycle up
		p.blurAll()
		if p.Mode == PasswordModeUnlock {
			// Only one field
			return p
		}
		// Cycle through 3 fields
		p.focusField = (p.focusField - 1 + 3) % 3
		p.updateFocusFromField()

	case tea.MouseWheelDown:
		// Tab through fields with scroll wheel - cycle down
		p.blurAll()
		if p.Mode == PasswordModeUnlock {
			// Only one field
			return p
		}
		p.focusField = (p.focusField + 1) % 3
		p.updateFocusFromField()
	}

	return p
}

// updateFocusFromField focuses the appropriate input based on focusField
func (p *PasswordModel) updateFocusFromField() {
	if p.Mode == PasswordModeChange {
		switch p.focusField {
		case 0:
			p.currentInput.Focus()
		case 1:
			p.input.Focus()
		case 2:
			p.confirmInput.Focus()
		}
	} else if p.Mode == PasswordModeCreate {
		switch p.focusField {
		case 0:
			p.walletNameInput.Focus()
		case 1:
			p.input.Focus()
		case 2:
			p.confirmInput.Focus()
		}
	} else {
		p.input.Focus()
	}
}
