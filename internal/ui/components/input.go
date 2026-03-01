// Copyright 2017-2026 DERO Project. All rights reserved.

package components

import (
	"unicode/utf8"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/deroproject/dero-wallet-cli/internal/ui/styles"
)

// CharFilterFunc is a function that returns true if the rune is allowed
type CharFilterFunc func(r rune) bool

// InputModel wraps a text input with styling
type InputModel struct {
	textinput.Model
	Label      string
	Error      string
	Focused    bool
	Password   bool
	Disabled   bool           // when true, input is read-only
	revealed   bool           // tracks if password is revealed
	charFilter CharFilterFunc // optional filter for allowed characters
}

// NewInput creates a new styled input
func NewInput(label, placeholder string, password bool) InputModel {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.CharLimit = 256
	ti.SetWidth(styles.InputWidth - 4)
	ti.Prompt = "" // Remove default prompt to prevent line wrapping

	if password {
		ti.EchoMode = textinput.EchoPassword
		ti.EchoCharacter = '•'
	}

	return InputModel{
		Model:    ti,
		Label:    label,
		Password: password,
	}
}

// Init initializes the input
func (i InputModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles input events
func (i InputModel) Update(msg tea.Msg) (InputModel, tea.Cmd) {
	// Ignore all key input when disabled
	if i.Disabled {
		// Still process blink commands for cursor animation, but ignore keys
		if _, ok := msg.(tea.KeyPressMsg); ok {
			return i, nil
		}
	}

	// Filter characters if a filter is set
	if i.charFilter != nil {
		// Character filtering temporarily disabled for v2 migration
		// TODO: Re-implement with v2 KeyPressMsg API
	}

	var cmd tea.Cmd
	i.Model, cmd = i.Model.Update(msg)
	return i, cmd
}

// View renders the input
func (i InputModel) View() string {
	label := styles.TextStyle.Render(i.Label)

	// Add visibility indicator for password fields
	if i.Password {
		if i.revealed {
			label += styles.MutedStyle.Render(" [Visible]")
		} else {
			label += styles.MutedStyle.Render(" [Hidden]")
		}
	}

	// Add locked indicator for disabled fields
	if i.Disabled {
		label += styles.MutedStyle.Render(" [Locked]")
	}

	var inputStyle = styles.InputStyle
	if i.Disabled {
		inputStyle = styles.DisabledInputStyle
	} else if i.Focused {
		inputStyle = styles.FocusedInputStyle
	}

	input := inputStyle.Render(i.Model.View())

	var result string
	if i.Label != "" {
		result = label + "\n" + input
	} else {
		result = input
	}

	if i.Error != "" {
		result += "\n" + styles.ErrorStyle.Render(i.Error)
	}

	return result
}

// Focus focuses the input
func (i *InputModel) Focus() tea.Cmd {
	i.Focused = true
	return i.Model.Focus()
}

// Blur removes focus from the input
func (i *InputModel) Blur() {
	i.Focused = false
	i.Model.Blur()
}

// Value returns the input value
func (i InputModel) Value() string {
	return i.Model.Value()
}

// SetValue sets the input value
func (i *InputModel) SetValue(v string) {
	if i.Model.CharLimit > 0 && utf8.RuneCountInString(v) > i.Model.CharLimit {
		runes := []rune(v)
		v = string(runes[:i.Model.CharLimit])
	}
	i.Model.SetValue(v)
}

// Reset clears the input
func (i *InputModel) Reset() {
	i.Model.Reset()
	i.Error = ""
	// Reset to hidden state for password fields
	if i.Password {
		i.revealed = false
		i.Model.EchoMode = textinput.EchoPassword
	}
}

// ToggleReveal toggles password visibility
func (i *InputModel) ToggleReveal() {
	if !i.Password {
		return // Only works for password fields
	}
	i.revealed = !i.revealed
	if i.revealed {
		i.Model.EchoMode = textinput.EchoNormal
	} else {
		i.Model.EchoMode = textinput.EchoPassword
	}
}

// SetCharLimit sets the character limit for the input
func (i *InputModel) SetCharLimit(limit int) {
	i.Model.CharLimit = limit
}

// SetCharFilter sets a filter function that blocks disallowed characters
func (i *InputModel) SetCharFilter(fn CharFilterFunc) {
	i.charFilter = fn
}
