// Copyright 2017-2026 DERO Project. All rights reserved.

package pages

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/deroproject/dero-wallet-cli/internal/ui/components"
	"github.com/deroproject/dero-wallet-cli/internal/ui/styles"
	"github.com/deroproject/dero-wallet-cli/internal/wallet"
)

// TokensModel is the token/asset management page.
type TokensModel struct {
	tokens         []wallet.TokenInfo
	cursor         int
	loading        bool
	scanning       bool
	scanProgress   string
	err            string
	flash          string
	flashGood      bool
	adding         bool
	addInput       components.InputModel
	addError       string
	cancelled      bool
	wantSend       bool
	wantHistory    bool
	wantRemove     bool
	wantAdd        bool
	pendingAddSCID string
	selectedSCID   string
	lastClickRow   int
	lastClickAt    time.Time
	width          int
	height         int
}

// NewTokens creates a new tokens page.
func NewTokens() TokensModel {
	m := TokensModel{cursor: 0, lastClickRow: -1}
	m.addInput = components.NewInput("", "SCID (64 hex chars)", false)
	m.addInput.SetCharLimit(64)
	return m
}

// SetTokens sets the token list.
func (m *TokensModel) SetTokens(tokens []wallet.TokenInfo) {
	m.tokens = tokens
	m.loading = false
	m.err = ""
	if m.cursor >= len(m.tokens) && len(m.tokens) > 0 {
		m.cursor = len(m.tokens) - 1
	}
	if len(m.tokens) == 0 {
		m.cursor = 0
	}
}

// SetLoading sets loading state.
func (m *TokensModel) SetLoading(v bool) { m.loading = v }
func (m *TokensModel) SetScanning(v bool, progress string) {
	m.scanning = v
	m.scanProgress = progress
}

// SetError sets error.
func (m *TokensModel) SetError(s string) { m.err = s; m.loading = false }

// SetFlash sets flash message.
func (m *TokensModel) SetFlash(msg string, good bool) { m.flash = msg; m.flashGood = good }

// Cancelled reports if user cancelled.
func (m TokensModel) Cancelled() bool { return m.cancelled }

// WantSend reports if user wants to send the selected token.
func (m TokensModel) WantSend() (string, bool) {
	if m.wantSend && m.cursor >= 0 && m.cursor < len(m.tokens) {
		return m.tokens[m.cursor].SCID, true
	}
	if m.wantSend && m.selectedSCID != "" {
		return m.selectedSCID, true
	}
	return "", false
}

// WantHistory reports if user wants history for selected token.
func (m TokensModel) WantHistory() (string, bool) {
	if m.wantHistory && m.cursor >= 0 && m.cursor < len(m.tokens) {
		return m.tokens[m.cursor].SCID, true
	}
	return "", false
}

// WantRemove reports if user wants to remove selected token from tracking.
func (m TokensModel) WantRemove() (string, bool) {
	if m.wantRemove && m.cursor >= 0 && m.cursor < len(m.tokens) {
		return m.tokens[m.cursor].SCID, true
	}
	return "", false
}

// GetAddSCID returns the SCID being added (when confirmed).
func (m TokensModel) GetAddSCID() string { return strings.TrimSpace(m.addInput.Value()) }

// IsAdding reports if in add mode.
func (m TokensModel) IsAdding() bool { return m.adding }

// Tokens returns the token list.
func (m TokensModel) Tokens() []wallet.TokenInfo { return m.tokens }

// WantAdd reports if user confirmed adding a token.
func (m TokensModel) WantAdd() (string, bool) {
	if m.wantAdd {
		return m.pendingAddSCID, true
	}
	return "", false
}

// ResetActions clears action flags.
func (m *TokensModel) ResetActions() {
	m.wantSend = false
	m.wantHistory = false
	m.wantRemove = false
	m.wantAdd = false
	m.pendingAddSCID = ""
	m.selectedSCID = ""
}

// Reset clears state for re-display.
func (m *TokensModel) Reset() {
	m.cursor = 0
	m.err = ""
	m.flash = ""
	m.cancelled = false
	m.loading = false
	m.scanning = false
	m.scanProgress = ""
	m.adding = false
	m.addError = ""
	m.addInput.Reset()
	m.wantSend = false
	m.wantHistory = false
	m.wantRemove = false
	m.wantAdd = false
	m.pendingAddSCID = ""
	m.selectedSCID = ""
	m.lastClickRow = -1
}

func (m TokensModel) Init() tea.Cmd { return nil }

func (m TokensModel) Update(msg tea.Msg) (TokensModel, tea.Cmd) {
	var cmd tea.Cmd

	// Add mode has its own handling.
	if m.adding {
		switch msg := msg.(type) {
		case tea.KeyPressMsg:
			switch {
			case key.Matches(msg, pageEscKeys):
				m.adding = false
				m.addError = ""
				m.addInput.Reset()
				return m, nil
			case key.Matches(msg, pageEnterKeys):
				scid := strings.TrimSpace(m.addInput.Value())
				if err := wallet.ValidateSCID(scid); err != nil {
					m.addError = err.Error()
					return m, nil
				}
				m.wantAdd = true
				m.pendingAddSCID = scid
				m.adding = false
				m.addInput.Reset()
				return m, nil
			}
		}
		m.addInput, cmd = m.addInput.Update(msg)
		m.addError = ""
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, pageEscKeys):
			m.cancelled = true
			return m, nil
		case key.Matches(msg, key.NewBinding(key.WithKeys("up", "k"))):
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case key.Matches(msg, key.NewBinding(key.WithKeys("down", "j"))):
			if m.cursor < len(m.tokens)-1 {
				m.cursor++
			}
			return m, nil
		case key.Matches(msg, key.NewBinding(key.WithKeys("a"))):
			m.adding = true
			m.addError = ""
			m.addInput.Reset()
			return m, m.addInput.Focus()
		case key.Matches(msg, key.NewBinding(key.WithKeys("s"))):
			if len(m.tokens) > 0 && !m.loading {
				m.wantSend = true
			}
			return m, nil
		case key.Matches(msg, key.NewBinding(key.WithKeys("h"))):
			if len(m.tokens) > 0 && !m.loading {
				m.wantHistory = true
			}
			return m, nil
		case key.Matches(msg, key.NewBinding(key.WithKeys("d", "x"))):
			if len(m.tokens) > 0 && !m.loading {
				m.wantRemove = true
			}
			return m, nil
		case key.Matches(msg, pageEnterKeys):
			if len(m.tokens) > 0 && !m.loading {
				m.wantSend = true
			}
			return m, nil
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	}
	return m, nil
}

func (m TokensModel) View() string {
	var content string
	if m.loading {

		status := "Loading..."
		if m.scanning {
			status = "Scanning token contracts..."
			if m.scanProgress != "" {
				status += "\n" + m.scanProgress
			}
		}
		content = lipgloss.JoinVertical(lipgloss.Center,
			styles.TitleStyle.Render("Tokens"),
			"",
			styles.MutedStyle.Render(status),
		)
	} else if m.adding {
		b := strings.Builder{}
		b.WriteString(styles.TitleStyle.Render("Add Token"))
		b.WriteString("\n\n")
		b.WriteString(styles.MutedStyle.Render("Enter the token SCID (64 hex chars) to track:"))
		b.WriteString("\n\n")
		b.WriteString(m.addInput.View())
		b.WriteString("\n")
		if m.addError != "" {
			b.WriteString(styles.ErrorStyle.Render("✗ " + m.addError))
			b.WriteString("\n")
		}
		b.WriteString("\n")
		b.WriteString(styles.MutedStyle.Render("[Enter] Add  [Esc] Cancel"))
		content = lipgloss.JoinVertical(lipgloss.Center, b.String())
	} else {
		var b strings.Builder
		if m.scanning && m.scanProgress != "" {
			b.WriteString(styles.MutedStyle.Render("⟳ " + m.scanProgress))
			b.WriteString("\n\n")
		}
		if m.err != "" {
			b.WriteString(styles.ErrorStyle.Width(styles.Width - 8).Render("✗ " + m.err))
			b.WriteString("\n\n")
		}
		if len(m.tokens) == 0 {
			if m.scanning {
				b.WriteString(styles.WarningStyle.Render("Discovering tokens in the background..."))
				b.WriteString("\n")
			} else {
				b.WriteString(styles.MutedStyle.Render("No tokens found in this wallet"))
				b.WriteString("\n")
			}
			b.WriteString("\n")
			b.WriteString(styles.MutedStyle.Render("Press [A] to add a token SCID to track"))
			b.WriteString("\n\n")
		}
		for i, tok := range m.tokens {
			prefix := "  "
			nameStyle := styles.MutedStyle
			if i == m.cursor {
				prefix = lipgloss.NewStyle().Foreground(styles.ColorPrimary).Render("▸ ")
				nameStyle = lipgloss.NewStyle().Foreground(styles.ColorText).Bold(true)
			}
			displayName := safeSCIDLabel(tok.SCID)
			if tok.Ticker != "" {
				displayName = tok.Ticker
				if tok.Name != "" && tok.Name != tok.Ticker {
					displayName += " (" + tok.Name + ")"
				}
			} else if tok.Name != "" {
				displayName = tok.Name
			} else {
				displayName = safeSCIDLabel(tok.SCID)
			}
			balStr := wallet.FormatTokenAmount(tok.Balance, tok.Decimals)
			line := prefix + nameStyle.Render(displayName) + "  " + styles.BalanceStyle.Render(balStr)
			if tok.Ticker == "" && tok.Name == "" {
				line += "  " + styles.MutedStyle.Render(truncateSCID(tok.SCID, 16))
			}
			b.WriteString(line)
			b.WriteString("\n")
			// Second line: SCID truncated muted
			if tok.Ticker != "" || tok.Name != "" {
				scidLine := "    " + styles.MutedStyle.Render(truncateSCID(tok.SCID, 16))
				b.WriteString(scidLine)
				b.WriteString("\n")
			}
		}
		if m.flash != "" {
			style := styles.ErrorStyle
			if m.flashGood {
				style = styles.SuccessStyle
			}
			b.WriteString(style.Render(m.flash))
			b.WriteString("\n")
		}
		b.WriteString("\n")
		footer := fmt.Sprintf("[A]dd  %s  %s  %s  [Esc] Back",
			dimIf(len(m.tokens) == 0, "[S]end"),
			dimIf(len(m.tokens) == 0, "[H]istory"),
			dimIf(len(m.tokens) == 0, "[D]elete"),
		)
		b.WriteString(styles.MutedStyle.Render(footer))
		content = lipgloss.JoinVertical(lipgloss.Center,
			styles.TitleStyle.Render("Tokens"),
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

func truncateSCID(scid string, prefix int) string {
	if len(scid) <= prefix {
		return scid
	}
	return scid[:prefix] + "..."
}

func safeSCIDLabel(scid string) string {
	if len(scid) <= 16 {
		return scid
	}
	return scid[:8] + "..." + scid[len(scid)-8:]
}

// HandleMouse handles mouse for tokens page.
func (m TokensModel) HandleMouse(msg tea.MouseClickMsg, windowWidth, windowHeight int) TokensModel {
	if m.adding {
		return m
	}
	row := msg.Mouse().Y
	titleOffset := 5
	if m.err != "" {
		titleOffset += 2
	}
	row = row - titleOffset
	// Each token uses 1 or 2 lines; approximate as 2 lines per token for hit testing.
	idx := row / 2
	if idx < 0 || idx >= len(m.tokens) {
		return m
	}
	m.cursor = idx
	if msg.Button == tea.MouseLeft {
		now := time.Now()
		if m.lastClickRow == idx && now.Sub(m.lastClickAt) < 500*time.Millisecond {
			m.wantSend = true
		}
		m.lastClickRow = idx
		m.lastClickAt = now
	}
	return m
}
