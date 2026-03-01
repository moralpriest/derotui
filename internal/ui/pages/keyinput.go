// Copyright 2017-2026 DERO Project. All rights reserved.

package pages

import (
	"log"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/atotto/clipboard"
	"github.com/deroproject/dero-wallet-cli/internal/ui/components"
	"github.com/deroproject/dero-wallet-cli/internal/ui/styles"
)

// KeyInputMode represents the key screen mode
type KeyInputMode int

const (
	KeyModeInput KeyInputMode = iota
	KeyModeDisplay
)

// KeyInputModel represents the hex key input screen
type KeyInputModel struct {
	mode      KeyInputMode
	input     components.InputModel
	hexKey    string
	confirmed bool
	cancelled bool
	error     string
	copied    bool
}

// NewKeyInput creates a new key input screen
func NewKeyInput() KeyInputModel {
	m := KeyInputModel{
		mode:  KeyModeInput,
		input: components.NewInput("Private Key", "Enter 64 character hex key...", false),
	}
	m.input.Focus()
	return m
}

// NewKeyInputDisplay creates a key display screen (read-only)
func NewKeyInputDisplay(hexKey string) KeyInputModel {
	return KeyInputModel{
		mode:   KeyModeDisplay,
		hexKey: hexKey,
		input:  components.NewInput("", "", false),
	}
}

// Init initializes the key input screen
func (k KeyInputModel) Init() tea.Cmd {
	return k.input.Init()
}

// Update handles events
func (k KeyInputModel) Update(msg tea.Msg) (KeyInputModel, tea.Cmd) {
	var cmd tea.Cmd

	// Store error at start - we'll preserve it unless explicitly cleared
	currentError := k.error

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if key.Matches(msg, pageEscKeys) {
			log.Printf("[DEBUG keyinput] Cancelled hex key %s", map[KeyInputMode]string{KeyModeInput: "input", KeyModeDisplay: "display"}[k.mode])
			k.cancelled = true
			return k, nil
		}

		// Display mode - handle copy and confirm
		if k.mode == KeyModeDisplay {
			if key.Matches(msg, pageCopyKeys) {
				if err := clipboard.WriteAll(k.hexKey); err == nil {
					k.copied = true
					log.Printf("[DEBUG keyinput] Copied hex key to clipboard")
				}
				return k, nil
			}
			if key.Matches(msg, pageEnterKeys) {
				k.confirmed = true
				log.Printf("[DEBUG keyinput] Confirmed hex key display")
			}
			return k, nil
		}

		// Input mode
		if key.Matches(msg, pageEnterKeys) {
			key := strings.TrimSpace(k.input.Value())
			if len(key) != 64 {
				k.error = "Key must be exactly 64 hexadecimal characters"
				log.Printf("[DEBUG keyinput] Validation failed: wrong length (%d)", len(key))
				return k, nil
			}
			// Validate hex
			for _, c := range key {
				if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
					k.error = "Key must contain only hexadecimal characters (0-9, a-f)"
					log.Printf("[DEBUG keyinput] Validation failed: invalid hex character")
					return k, nil
				}
			}
			k.confirmed = true
			log.Printf("[DEBUG keyinput] Hex key input confirmed")
			return k, nil
		}

		// For other key presses (typing), clear the error
		k.input, cmd = k.input.Update(msg)
		k.error = "" // User is typing, clear error
		return k, cmd
	}

	// Non-KeyMsg (like cursor blink) - update input but preserve error
	if k.mode == KeyModeInput {
		k.input, cmd = k.input.Update(msg)
		k.error = currentError
	}
	return k, cmd
}

// View renders the key input screen
func (k KeyInputModel) View() string {
	var title, subtitle string

	if k.mode == KeyModeDisplay {
		title = "🔐 Your Private Key 🔐"
		subtitle = "Store this 64-character key securely offline. You can use it to recover your wallet."
	} else {
		title = "Restore from Key"
		subtitle = "Enter your 64 character hexadecimal private key."
	}

	titleStyled := styles.TitleStyle.Render(title)
	subtitleStyled := styles.MutedStyle.Render(subtitle)

	// Content
	var content string
	if k.mode == KeyModeDisplay {
		// Display hex key in a box, split into two lines for readability
		var keyDisplay string
		if len(k.hexKey) >= 64 {
			keyDisplay = k.hexKey[:32] + "\n" + k.hexKey[32:64]
		} else {
			keyDisplay = k.hexKey
		}
		content = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(styles.ColorBorder).
			Padding(1, 2).
			Width(56).
			Align(lipgloss.Center).
			Foreground(styles.ColorAccent).
			Bold(true).
			Render(keyDisplay)
	} else {
		content = k.input.View()
	}

	// Error
	var errorView string
	if k.error != "" {
		errorView = styles.ErrorStyle.Render("✗ " + k.error)
	}

	// Warning
	warning := styles.WarningStyle.Render("⚠ Never share your private key with anyone!")

	// Copied message
	var copiedMsg string
	if k.copied {
		copiedMsg = styles.SuccessStyle.Render("✓ Copied to clipboard!")
	}

	// Help
	var help string
	if k.mode == KeyModeDisplay {
		help = "C Copy • Enter Continue • Esc Back"
	} else {
		help = "Enter Confirm • Esc Cancel"
	}
	helpStyled := styles.MutedStyle.Render(help)

	// Compose - only show logo for display mode, not restore mode
	var elements []string
	if k.mode == KeyModeDisplay {
		elements = []string{
			styles.Logo(),
			"",
			titleStyled,
			"",
			subtitleStyled,
			"",
			content,
		}
	} else {
		elements = []string{
			titleStyled,
			"",
			subtitleStyled,
			"",
			content,
		}
	}

	if errorView != "" {
		elements = append(elements, "", errorView)
	}

	elements = append(elements, "", warning)
	if copiedMsg != "" {
		elements = append(elements, "", copiedMsg)
	}
	elements = append(elements, "", helpStyled)

	composed := lipgloss.JoinVertical(lipgloss.Left, elements...)

	return styles.ThemedBoxStyle().
		Width(styles.Width).
		Align(lipgloss.Center).
		Padding(2, 4).
		Render(composed)
}

// Confirmed returns whether user confirmed
func (k KeyInputModel) Confirmed() bool {
	return k.confirmed
}

// Cancelled returns whether user cancelled
func (k KeyInputModel) Cancelled() bool {
	return k.cancelled
}

// GetKey returns the entered key
func (k KeyInputModel) GetKey() string {
	return strings.TrimSpace(k.input.Value())
}

// Reset resets the input
func (k *KeyInputModel) Reset() {
	k.confirmed = false
	k.cancelled = false
	k.error = ""
	if k.mode == KeyModeInput {
		k.input.Reset()
		k.input.Focus()
	}
}

// IsDisplayMode returns true if showing a key (not inputting)
func (k KeyInputModel) IsDisplayMode() bool {
	return k.mode == KeyModeDisplay
}

// HandleMouse handles mouse events on the key input screen
func (k KeyInputModel) HandleMouse(msg tea.MouseClickMsg, windowWidth, windowHeight int) KeyInputModel {
	// Calculate centered position
	boxWidth := styles.Width
	boxX := (windowWidth - boxWidth) / 2
	boxY := (windowHeight - styles.MediumBoxHeight) / 2

	// Adjust to content area
	relX := msg.Mouse().X - boxX - 4
	relY := msg.Mouse().Y - boxY - 7 // After logo, title, subtitle

	switch msg.Button {
	case tea.MouseLeft:
		if k.mode == KeyModeDisplay {
			// Copy button area: y=5 (below key display)
			if relY >= 5 && relY <= 6 && relX >= 10 && relX < 50 {
				if err := clipboard.WriteAll(k.hexKey); err == nil {
					k.copied = true
				}
				return k
			}
			// Continue button area: y=9
			if relY >= 9 && relY <= 10 {
				k.confirmed = true
				return k
			}
		} else {
			// Input mode: click on input field
			// Input is at y=0-1
			if relY >= 0 && relY <= 1 {
				k.input.Focus()
				return k
			}
			// Confirm button at y=4
			if relY >= 4 && relY <= 5 {
				key := strings.TrimSpace(k.input.Value())
				if len(key) != 64 {
					k.error = "Key must be exactly 64 hexadecimal characters"
					return k
				}
				// Validate hex
				for _, c := range key {
					if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
						k.error = "Key must contain only hexadecimal characters (0-9, a-f)"
						return k
					}
				}
				k.confirmed = true
				return k
			}
		}

	case tea.MouseWheelUp, tea.MouseWheelDown:
		// No scrollable content
	}

	return k
}
