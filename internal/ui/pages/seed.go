// Copyright 2017-2026 DERO Project. All rights reserved.

package pages

import (
	"log"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/atotto/clipboard"
	"github.com/deroproject/dero-wallet-cli/internal/ui/styles"
	"github.com/deroproject/dero-wallet-cli/internal/wallet"
)

var (
	seedDeleteKeys = key.NewBinding(key.WithKeys("backspace", "delete", "ctrl+h"))
)

// SeedMode represents the seed screen mode
type SeedMode int

const (
	SeedModeDisplay SeedMode = iota
	SeedModeInput
)

// SeedModel represents the seed screen
type SeedModel struct {
	Mode      SeedMode
	Seed      string
	textarea  textarea.Model
	confirmed bool
	cancelled bool
	error     string
	lastText  string // Track previous text for word completion detection
	copied    bool   // Track if seed was copied to clipboard
}

// NewSeed creates a new seed screen
func NewSeed(mode SeedMode, seed string) SeedModel {
	ta := textarea.New()
	ta.Placeholder = "Enter your 25-word seed words..."
	ta.CharLimit = 500
	ta.SetWidth(64)
	ta.SetHeight(5)
	ta.ShowLineNumbers = false

	// Style the textarea to match theme - remove all decorations
	ta.Prompt = ""
	taStyles := ta.Styles()
	taStyles.Focused.Base = lipgloss.NewStyle()
	taStyles.Focused.CursorLine = lipgloss.NewStyle()
	taStyles.Focused.Placeholder = lipgloss.NewStyle().Foreground(styles.ColorMuted)
	taStyles.Focused.Text = lipgloss.NewStyle().Foreground(styles.ColorText)
	taStyles.Focused.Prompt = lipgloss.NewStyle()
	taStyles.Focused.EndOfBuffer = lipgloss.NewStyle()
	taStyles.Blurred = taStyles.Focused
	ta.SetStyles(taStyles)

	if mode == SeedModeInput {
		ta.Focus()
	}

	return SeedModel{
		Mode:     mode,
		Seed:     seed,
		textarea: ta,
	}
}

// Init initializes the seed screen
func (s SeedModel) Init() tea.Cmd {
	if s.Mode == SeedModeInput {
		return textarea.Blink
	}
	return nil
}

// normalizeSeed cleans up seed input: allows spaces between words but removes empty lines
func normalizeSeed(input string) string {
	// Replace newlines and tabs with spaces
	input = strings.ReplaceAll(input, "\n", " ")
	input = strings.ReplaceAll(input, "\r", " ")
	input = strings.ReplaceAll(input, "\t", " ")
	// Collapse multiple spaces into one
	words := strings.Fields(input)
	return strings.Join(words, " ")
}

// getLastWord returns the last complete word from input (word before trailing space)
func getLastWord(input string) string {
	input = strings.TrimRight(input, " \t\n\r")
	words := strings.Fields(input)
	if len(words) == 0 {
		return ""
	}
	return words[len(words)-1]
}

// Update handles events
func (s SeedModel) Update(msg tea.Msg) (SeedModel, tea.Cmd) {
	var cmd tea.Cmd

	// Store error at start - we'll preserve it unless explicitly cleared
	currentError := s.error

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		// Handle escape first - always works
		if key.Matches(msg, pageEscKeys) {
			log.Printf("[DEBUG seed] Cancelled seed %s", map[SeedMode]string{SeedModeDisplay: "display", SeedModeInput: "input"}[s.Mode])
			s.cancelled = true
			return s, nil
		}

		// Handle copy in display mode
		if key.Matches(msg, pageCopyKeys) && s.Mode == SeedModeDisplay {
			if err := clipboard.WriteAll(s.Seed); err == nil {
				s.copied = true
				log.Printf("[DEBUG seed] Copied seed to clipboard")
			}
			return s, nil
		}

		// Handle enter for both display and input mode
		if key.Matches(msg, pageEnterKeys) {
			if s.Mode == SeedModeDisplay {
				s.confirmed = true
				log.Printf("[DEBUG seed] Confirmed seed display")
				return s, nil
			}
			// Input mode - validate and confirm
			seed := normalizeSeed(s.textarea.Value())
			if seed == "" {
				s.error = "Please enter your seed words"
				log.Printf("[DEBUG seed] Validation failed: empty seed")
				return s, nil
			}
			// Validate the seed using the wallet validation
			if err := wallet.ValidateSeed(seed); err != nil {
				s.error = err.Error()
				log.Printf("[DEBUG seed] Validation failed: %d words invalid", len(strings.Fields(seed)))
				return s, nil
			}
			s.Seed = seed
			s.confirmed = true
			log.Printf("[DEBUG seed] Seed input confirmed (25 words)")
			return s, nil
		}

		// For input mode, handle other key presses
		if s.Mode == SeedModeInput {
			prevText := s.textarea.Value()
			prevWords := strings.Fields(prevText)

			// If we have 25 words, only allow backspace/delete
			if len(prevWords) >= 25 {
				if !key.Matches(msg, seedDeleteKeys) {
					// Check if current text ends with space (meaning 25th word is complete)
					if strings.HasSuffix(prevText, " ") || len(prevWords) > 25 {
						s.error = "Maximum 25 words reached"
						return s, nil
					}
				}
			}

			// Pass to textarea
			s.textarea, cmd = s.textarea.Update(msg)
			newText := s.textarea.Value()

			// Check if space was added (word completed)
			if msg.Key().Code == tea.KeySpace && len(newText) > len(prevText) {
				lastWord := getLastWord(prevText)
				if lastWord != "" {
					// Validate the word
					if err := wallet.ValidateWord(lastWord); err != nil {
						s.error = err.Error()
					} else {
						s.error = "" // Valid word, clear error
					}
					// Check if we just completed the 25th word
					words := strings.Fields(newText)
					if len(words) == 25 {
						s.error = "" // Clear any error, we have exactly 25
					}
				}
			} else {
				// Other key press (not space) - clear error so user can type
				s.error = ""
			}

			s.lastText = newText
			return s, cmd
		}
	}

	// Non-KeyMsg (like cursor blink) - update textarea but preserve error
	if s.Mode == SeedModeInput {
		s.textarea, cmd = s.textarea.Update(msg)
		s.error = currentError // Preserve error
	}

	return s, cmd
}

// View renders the seed screen
func (s SeedModel) View() string {
	var title, subtitle string

	if s.Mode == SeedModeDisplay {
		title = "🔐 Your Recovery Seed 🔐"
		subtitle = "Write down these 25 words in order and store them safely offline. This seed phrase can recover your wallet."
	} else {
		title = "Restore Wallet"
		subtitle = "Enter your 25-word seed words to restore your wallet."
	}

	titleStyled := styles.TitleStyle.Render(title)
	subtitleStyled := styles.MutedStyle.Render(subtitle)

	// Content
	var content string
	if s.Mode == SeedModeDisplay {
		// Build seed words as cards in 5x5 grid
		words := strings.Fields(s.Seed)
		const numRows = 5
		const numCols = 5
		const cardWidth = 14
		const cardGap = " "

		cardBgStyle := lipgloss.NewStyle().Background(styles.ColorCardBg)

		// Helper to center text in a width
		centerText := func(text string, width int) string {
			totalPad := width - len(text)
			if totalPad < 0 {
				return text[:width]
			}
			leftPad := totalPad / 2
			rightPad := totalPad - leftPad
			return strings.Repeat(" ", leftPad) + text + strings.Repeat(" ", rightPad)
		}

		// Build card rows
		var rowBlocks []string
		for row := 0; row < numRows; row++ {
			var numLines []string
			var wordLines []string

			for col := 0; col < numCols; col++ {
				idx := row*numCols + col
				if idx < len(words) {
					// Number line (muted, centered)
					numStr := strconv.Itoa(idx + 1)
					paddedNum := centerText(numStr, cardWidth)
					numLine := cardBgStyle.Foreground(styles.ColorMuted).Render(paddedNum)

					// Word line (bold primary color to match title, centered)
					word := words[idx]
					paddedWord := centerText(word, cardWidth)
					wordLine := cardBgStyle.Foreground(styles.ColorPrimary).Bold(true).Render(paddedWord)

					numLines = append(numLines, numLine)
					wordLines = append(wordLines, wordLine)
				}
			}

			// Join the 5 cards in this row
			numRow := strings.Join(numLines, cardGap)
			wordRow := strings.Join(wordLines, cardGap)
			rowBlock := numRow + "\n" + wordRow
			rowBlocks = append(rowBlocks, rowBlock)
		}

		// Join all row blocks with blank lines between rows
		content = strings.Join(rowBlocks, "\n\n")
	} else {
		// Input mode
		content = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(styles.ColorPrimary).
			Padding(0, 1).
			Width(64).
			Render(s.textarea.View())
	}

	// Warning
	var warning string
	if s.Mode == SeedModeDisplay {
		warning = styles.WarningStyle.Render("⚠ Never share your seed words with anyone!")
	} else {
		warning = styles.WarningStyle.Render("⚠ Only enter your seed words on trusted devices!")
	}

	// Copied message
	var copiedMsg string
	if s.copied {
		copiedMsg = styles.SuccessStyle.Render("✓ Copied to clipboard!")
	}

	// Error
	var errorView string
	if s.error != "" {
		errorView = styles.ErrorStyle.Render("✗ " + s.error)
	}

	// Help
	var help string
	if s.Mode == SeedModeDisplay {
		help = "C Copy • Enter Continue • Esc Back"
	} else {
		help = "Enter Confirm • Esc Cancel"
	}
	helpStyled := styles.MutedStyle.Render(help)

	// Compose elements (no logo for cleaner seed display)
	elements := []string{
		titleStyled,
		"",
		subtitleStyled,
		"",
		content,
	}

	if warning != "" {
		elements = append(elements, "", warning)
	}
	if copiedMsg != "" {
		elements = append(elements, "", copiedMsg)
	}
	if errorView != "" {
		elements = append(elements, "", errorView)
	}
	elements = append(elements, "", helpStyled)

	composed := lipgloss.JoinVertical(lipgloss.Left, elements...)

	// Use reduced horizontal padding (1,2) for seed display to fit card grid
	if s.Mode == SeedModeDisplay {
		return styles.ThemedBoxStyle().
			Width(styles.Width).
			Align(lipgloss.Center).
			Padding(1, 2).
			Render(composed)
	}

	return styles.ThemedBoxStyle().
		Width(styles.Width).
		Align(lipgloss.Center).
		Padding(1, 4).
		Render(composed)
}

// Confirmed returns whether user confirmed
func (s SeedModel) Confirmed() bool {
	return s.confirmed
}

// Cancelled returns whether user cancelled
func (s SeedModel) Cancelled() bool {
	return s.cancelled
}

// GetSeed returns the seed
func (s SeedModel) GetSeed() string {
	return s.Seed
}

// SetError sets an error message
func (s *SeedModel) SetError(err string) {
	s.error = err
	s.confirmed = false
}

// HandleMouse handles mouse events on the seed screen
func (s SeedModel) HandleMouse(msg tea.MouseClickMsg, windowWidth, windowHeight int) SeedModel {
	// Calculate centered position
	boxY := (windowHeight - styles.MediumBoxHeight) / 2

	// Adjust to content area (section header + inner box)
	relY := msg.Mouse().Y - boxY - 3

	switch msg.Button {
	case tea.MouseLeft:
		if s.Mode == SeedModeDisplay {
			// Copy button area: around section header / box top area
			if relY >= 4 && relY <= 5 {
				if err := clipboard.WriteAll(s.Seed); err == nil {
					s.copied = true
				}
				return s
			}
			// Continue button area: around warning/help area
			if relY >= 11 && relY <= 12 {
				s.confirmed = true
				return s
			}
		} else {
			// Input mode: click on textarea to focus
			// Textarea is at y=0-4
			if relY >= 0 && relY <= 4 {
				s.textarea.Focus()
				return s
			}
			// Confirm button at y=7
			if relY >= 7 && relY <= 8 {
				seed := normalizeSeed(s.textarea.Value())
				if seed == "" {
					s.error = "Please enter your seed words"
					return s
				}
				if err := wallet.ValidateSeed(seed); err != nil {
					s.error = err.Error()
					return s
				}
				s.Seed = seed
				s.confirmed = true
				return s
			}
		}

	case tea.MouseWheelUp, tea.MouseWheelDown:
		// No scrollable content in seed display
	}

	return s
}
