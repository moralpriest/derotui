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

const (
	tokColTicker = 10
	tokColName   = 22
	tokColBal    = 14
	tokColSCID   = 19
	tokensInner  = tokColTicker + tokColName + tokColBal + tokColSCID
	tokVisible   = 10
)

var (
	tokensUpKeys   = key.NewBinding(key.WithKeys("up", "k"))
	tokensDownKeys = key.NewBinding(key.WithKeys("down", "j"))
)

type TokensModel struct {
	tokens         []wallet.TokenInfo
	cursor         int
	offset         int
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
	wantRescan     bool
	wantResetScan  bool
	pendingAddSCID string
	selectedSCID   string
	lastClickRow   int
	lastClickAt    time.Time
	width          int
	height         int
}

func NewTokens() TokensModel {
	m := TokensModel{cursor: 0, lastClickRow: -1}
	m.addInput = components.NewInput("", "SCID (64 hex chars)", false)
	m.addInput.SetCharLimit(64)
	return m
}

func (m *TokensModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m *TokensModel) ClearCancelled() { m.cancelled = false }

func (m *TokensModel) SetTokens(tokens []wallet.TokenInfo) {
	if len(tokens) == 0 && len(m.tokens) > 0 {
		m.loading = false
		return
	}
	m.tokens = tokens
	m.loading = false
	m.err = ""
	if m.cursor >= len(m.tokens) && len(m.tokens) > 0 {
		m.cursor = len(m.tokens) - 1
	}
	if len(m.tokens) == 0 {
		m.cursor = 0
		m.offset = 0
	}
	m.clampOffset()
}

func (m *TokensModel) clampOffset() {
	vis := tokVisible
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+vis {
		m.offset = m.cursor - vis + 1
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

func tokenCells(tok wallet.TokenInfo) (ticker, name, bal, scid string) {
	ticker = tok.Ticker
	name = tok.Name
	if ticker == "" && name == "" {
		ticker = clip(safeSCIDLabel(tok.SCID), tokColTicker)
	}
	bal = wallet.FormatTokenAmount(tok.Balance, tok.Decimals)
	scid = safeSCIDLabel(tok.SCID)
	return clip(ticker, tokColTicker), clip(name, tokColName), clip(bal, tokColBal), clip(scid, tokColSCID)
}

func (m *TokensModel) SetLoading(v bool) { m.loading = v }
func (m *TokensModel) SetScanning(v bool, progress string) {
	m.scanning = v
	m.scanProgress = progress
}

func (m *TokensModel) SetError(s string) { m.err = s; m.loading = false }

func (m *TokensModel) SetFlash(msg string, good bool) { m.flash = msg; m.flashGood = good }

func (m TokensModel) Cancelled() bool { return m.cancelled }

func (m TokensModel) WantSend() (string, bool) {
	if m.wantSend && m.cursor >= 0 && m.cursor < len(m.tokens) {
		return m.tokens[m.cursor].SCID, true
	}
	if m.wantSend && m.selectedSCID != "" {
		return m.selectedSCID, true
	}
	return "", false
}

func (m TokensModel) WantHistory() (string, bool) {
	if m.wantHistory && m.cursor >= 0 && m.cursor < len(m.tokens) {
		return m.tokens[m.cursor].SCID, true
	}
	return "", false
}

func (m TokensModel) WantRemove() (string, bool) {
	if m.wantRemove && m.cursor >= 0 && m.cursor < len(m.tokens) {
		return m.tokens[m.cursor].SCID, true
	}
	return "", false
}

func (m TokensModel) GetAddSCID() string { return strings.TrimSpace(m.addInput.Value()) }

func (m TokensModel) IsAdding() bool { return m.adding }

func (m TokensModel) Tokens() []wallet.TokenInfo { return m.tokens }

func (m TokensModel) WantAdd() (string, bool) {
	if m.wantAdd {
		return m.pendingAddSCID, true
	}
	return "", false
}

func (m TokensModel) WantRescan() bool { return m.wantRescan }

func (m TokensModel) WantResetScan() bool { return false }

func (m *TokensModel) ResetActions() {
	m.wantSend = false
	m.wantHistory = false
	m.wantRemove = false
	m.wantAdd = false
	m.wantRescan = false
	m.wantResetScan = false
	m.pendingAddSCID = ""
	m.selectedSCID = ""
}

func (m *TokensModel) Reset() {
	m.cursor = 0
	m.offset = 0
	m.err = ""
	m.flash = ""
	m.cancelled = false
	m.loading = false
	m.scanning = false
	m.scanProgress = ""
	m.adding = false
	m.addError = ""
	m.addInput.Reset()
	m.ResetActions()
	m.lastClickRow = -1
}

func (m TokensModel) Init() tea.Cmd { return nil }

func (m *TokensModel) cursorUp() {
	if m.cursor > 0 {
		m.cursor--
		m.clampOffset()
	}
}

func (m *TokensModel) cursorDown() {
	if m.cursor < len(m.tokens)-1 {
		m.cursor++
		m.clampOffset()
	}
}

func (m TokensModel) Update(msg tea.Msg) (TokensModel, tea.Cmd) {
	var cmd tea.Cmd

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
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, pageEscKeys):
			m.cancelled = true
			return m, nil
		case key.Matches(msg, key.NewBinding(key.WithKeys("a"))):
			m.adding = true
			m.addError = ""
			m.addInput.Reset()
			return m, m.addInput.Focus()
		case key.Matches(msg, key.NewBinding(key.WithKeys("r", "R"))):
			m.wantRescan = true
			return m, nil
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
		case key.Matches(msg, tokensUpKeys):
			m.cursorUp()
			return m, nil
		case key.Matches(msg, tokensDownKeys):
			m.cursorDown()
			return m, nil
		}
	}
	return m, nil
}

func tokensTitle(text string) string {
	return styles.TitleStyle.Width(tokensInner).Align(lipgloss.Center).Render(text)
}

func (m TokensModel) View() string {
	var body strings.Builder
	if m.loading && len(m.tokens) == 0 && !m.adding {
		status := "Loading..."
		if m.scanning {
			status = "Scanning token contracts..."
			if m.scanProgress != "" {
				status += "\n" + m.scanProgress
			}
		}
		content := lipgloss.JoinVertical(lipgloss.Left,
			tokensTitle("Tokens"),
			"",
			styles.MutedStyle.Width(tokensInner).Align(lipgloss.Center).Render(status),
		)
		return tokensBox(content)
	}
	if m.adding {
		body.WriteString(styles.MutedStyle.Render("Enter the token SCID (64 hex chars) to track:"))
		body.WriteString("\n\n")
		body.WriteString(m.addInput.View())
		body.WriteString("\n")
		if m.addError != "" {
			body.WriteString(styles.ErrorStyle.Render("✗ " + m.addError))
			body.WriteString("\n")
		}
		body.WriteString("\n")
		body.WriteString(styles.MutedStyle.Render("[Enter] Add  [Esc] Cancel"))
		content := lipgloss.JoinVertical(lipgloss.Left, tokensTitle("Add Token"), "", body.String())
		return tokensBox(content)
	}

	if m.scanning && m.scanProgress != "" {
		body.WriteString(styles.MutedStyle.Render("⟳ " + m.scanProgress))
		body.WriteString("\n\n")
	}
	if m.err != "" {
		body.WriteString(styles.ErrorStyle.Width(tokensInner).Render("✗ " + m.err))
		body.WriteString("\n\n")
	}
	if len(m.tokens) == 0 {
		if m.scanning {
			body.WriteString(styles.WarningStyle.Render("Discovering tokens in the background..."))
			body.WriteString("\n")
		} else {
			body.WriteString(styles.MutedStyle.Render("No tokens found in this wallet"))
			body.WriteString("\n")
		}
		body.WriteString("\n")
		body.WriteString(styles.MutedStyle.Render("Press [A] to add a token SCID to track"))
		body.WriteString("\n\n")
	} else {
		body.WriteString(m.renderTable())
		body.WriteString("\n")
	}
	if m.flash != "" {
		style := styles.ErrorStyle
		if m.flashGood {
			style = styles.SuccessStyle
		}
		body.WriteString(style.Render(m.flash))
		body.WriteString("\n")
	}
	body.WriteString("\n")
	footer := fmt.Sprintf("[A]dd  %s  %s  %s  [R]escan  [Esc] Back",
		dimIf(len(m.tokens) == 0, "[S]end"),
		dimIf(len(m.tokens) == 0, "[H]istory"),
		dimIf(len(m.tokens) == 0, "[D]elete"),
	)
	body.WriteString(styles.MutedStyle.Render(footer))
	content := lipgloss.JoinVertical(lipgloss.Left, tokensTitle("Tokens"), "", body.String())
	return tokensBox(content)
}

func tokensBox(content string) string {
	return styles.ThemedBoxStyle().
		Width(styles.Width).
		Align(lipgloss.Left).
		Padding(1, 4).
		Render(content)
}

func (m TokensModel) renderTable() string {
	header := tokHeaderRow()
	sep := styles.StyledSeparator(tokensInner)
	end := m.offset + tokVisible
	if end > len(m.tokens) {
		end = len(m.tokens)
	}
	var rows []string
	for i := m.offset; i < end; i++ {
		rows = append(rows, tokRenderRow(m.tokens[i], i == m.cursor))
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, sep, lipgloss.JoinVertical(lipgloss.Left, rows...))
}

func tokHeaderRow() string {
	st := lipgloss.NewStyle().Bold(true).Foreground(styles.ColorMuted)
	return lipgloss.JoinHorizontal(lipgloss.Left,
		st.Width(tokColTicker).Render("Ticker"),
		st.Width(tokColName).Render("Name"),
		st.Width(tokColBal).Render("Balance"),
		st.Width(tokColSCID).Render("SCID"),
	)
}

func tokRenderRow(tok wallet.TokenInfo, selected bool) string {
	ticker, name, bal, scid := tokenCells(tok)
	if selected {
		st := lipgloss.NewStyle().
			Background(styles.ColorPrimary).
			Foreground(styles.ColorText).
			Bold(true)
		return lipgloss.JoinHorizontal(lipgloss.Left,
			st.Width(tokColTicker).Render(ticker),
			st.Width(tokColName).Render(name),
			st.Width(tokColBal).Render(bal),
			st.Width(tokColSCID).Render(scid),
		)
	}
	return lipgloss.JoinHorizontal(lipgloss.Left,
		lipgloss.NewStyle().Width(tokColTicker).Foreground(styles.ColorPrimary).Render(ticker),
		lipgloss.NewStyle().Width(tokColName).Foreground(styles.ColorText).Render(name),
		lipgloss.NewStyle().Width(tokColBal).Foreground(styles.ColorText).Render(bal),
		lipgloss.NewStyle().Width(tokColSCID).Foreground(styles.ColorMuted).Render(scid),
	)
}

func clip(s string, width int) string {
	if width <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= width {
		return s
	}
	if width == 1 {
		return string(r[:1])
	}
	return string(r[:width-1]) + "…"
}

func safeSCIDLabel(scid string) string {
	if len(scid) <= 16 {
		return scid
	}
	return scid[:8] + "..." + scid[len(scid)-8:]
}

func (m TokensModel) HandleMouse(msg tea.MouseClickMsg, windowWidth, windowHeight int) TokensModel {
	if m.adding || len(m.tokens) == 0 {
		return m
	}
	row := msg.Mouse().Y
	titleOffset := 6
	if m.err != "" {
		titleOffset += 2
	}
	if m.scanning && m.scanProgress != "" {
		titleOffset += 2
	}
	row = row - titleOffset + m.offset
	if row < 0 || row >= len(m.tokens) {
		return m
	}
	m.cursor = row
	m.clampOffset()
	if msg.Button == tea.MouseLeft {
		now := time.Now()
		if m.lastClickRow == row && now.Sub(m.lastClickAt) < 500*time.Millisecond {
			m.wantSend = true
		}
		m.lastClickRow = row
		m.lastClickAt = now
	}
	return m
}
