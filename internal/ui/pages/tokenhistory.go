// Copyright 2017-2026 DERO Project. All rights reserved.

package pages

import (
	"fmt"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/deroproject/dero-wallet-cli/internal/ui/styles"
)

// TokenHistoryModel shows history for a specific token.
type TokenHistoryModel struct {
	SCID         string
	Ticker       string
	Decimals     uint64
	Transactions []Transaction
	cursor       int
	offset       int
	wantDetails  bool
	cancelled    bool
}

func NewTokenHistory() TokenHistoryModel { return TokenHistoryModel{} }

func (m *TokenHistoryModel) SetToken(scid, ticker string, decimals uint64) {
	m.SCID = scid
	m.Ticker = ticker
	m.Decimals = decimals
}

func (m *TokenHistoryModel) SetTransactions(txs []Transaction) {
	m.Transactions = txs
	m.cursor = 0
	m.offset = 0
}

func (m TokenHistoryModel) Cancelled() bool   { return m.cancelled }
func (m TokenHistoryModel) WantDetails() bool { return m.wantDetails }
func (m TokenHistoryModel) SelectedTx() *Transaction {
	if m.cursor >= 0 && m.cursor < len(m.Transactions) {
		return &m.Transactions[m.cursor]
	}
	return nil
}
func (m *TokenHistoryModel) ResetActions() { m.wantDetails = false }
func (m *TokenHistoryModel) Reset() {
	m.cancelled = false
	m.wantDetails = false
	m.cursor = 0
	m.offset = 0
}
func (m TokenHistoryModel) Init() tea.Cmd { return nil }

func (m TokenHistoryModel) Update(msg tea.Msg) (TokenHistoryModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, pageEscKeys):
			m.cancelled = true
			return m, nil
		case key.Matches(msg, pageEnterKeys):
			if len(m.Transactions) > 0 {
				m.wantDetails = true
			}
		case key.Matches(msg, pageUpKeys):
			if m.cursor > 0 {
				m.cursor--
				if m.cursor < m.offset {
					m.offset = m.cursor
				}
			}
		case key.Matches(msg, pageDownKeys):
			if m.cursor < len(m.Transactions)-1 {
				m.cursor++
				if m.cursor >= m.offset+10 {
					m.offset = m.cursor - 9
				}
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("pgup"))):
			m.cursor -= 10
			if m.cursor < 0 {
				m.cursor = 0
			}
			m.offset = m.cursor
		case key.Matches(msg, key.NewBinding(key.WithKeys("pgdown"))):
			m.cursor += 10
			if m.cursor >= len(m.Transactions) {
				m.cursor = len(m.Transactions) - 1
			}
			if m.cursor >= 0 {
				m.offset = max(0, m.cursor-9)
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("home"))):
			m.cursor = 0
			m.offset = 0
		case key.Matches(msg, key.NewBinding(key.WithKeys("end"))):
			m.cursor = len(m.Transactions) - 1
			m.offset = max(0, len(m.Transactions)-10)
		}
	}
	return m, nil
}

func (m TokenHistoryModel) View() string {
	title := "Token History"
	if m.Ticker != "" {
		title = "Token History: " + m.Ticker
	} else if m.SCID != "" {
		title = "Token History: " + safeSCIDLabel(m.SCID)
	}
	if len(m.Transactions) == 0 {
		body := lipgloss.JoinVertical(lipgloss.Center,
			styles.TitleStyle.Render(title),
			"",
			styles.MutedStyle.Render("No transactions for this token"),
			"",
			styles.MutedStyle.Render("[Esc] Back"),
		)
		return styles.ThemedBoxStyle().Width(styles.Width).Align(lipgloss.Center).Padding(1, 4).Render(body)
	}
	var sections []string
	sections = append(sections, styles.TitleStyle.Render(title))
	sections = append(sections, "")
	// Reuse history rendering logic but with token-specific amount formatting.
	header := m.renderHeaderToken()
	sections = append(sections, header)
	rows := m.renderRowsToken()
	sections = append(sections, rows)
	pageInfo := styles.MutedStyle.Render(fmt.Sprintf("Showing %d of %d  •  [Esc] Back  [Enter] Details", m.cursor+1, len(m.Transactions)))
	sections = append(sections, pageInfo)
	content := lipgloss.JoinVertical(lipgloss.Left, sections...)
	return styles.ThemedBoxStyle().Width(styles.Width).Align(lipgloss.Center).Padding(1, 4).Render(content)
}

func (m TokenHistoryModel) renderHeaderToken() string {
	selectorCol := lipgloss.NewStyle().Width(2).Render("")
	dateCol := lipgloss.NewStyle().Width(12).Bold(true).Foreground(styles.ColorMuted).Render("Date")
	blockCol := lipgloss.NewStyle().Width(10).Bold(true).Foreground(styles.ColorMuted).Render("Block")
	typeCol := lipgloss.NewStyle().Width(10).Bold(true).Foreground(styles.ColorMuted).Render("Type")
	amountCol := lipgloss.NewStyle().Width(18).Bold(true).Foreground(styles.ColorMuted).Render("Amount")
	headerRow := lipgloss.JoinHorizontal(lipgloss.Left, selectorCol, dateCol, blockCol, typeCol, amountCol)
	sep := styles.StyledSeparator(52)
	return lipgloss.JoinVertical(lipgloss.Left, headerRow, sep)
}

func (m TokenHistoryModel) renderRowsToken() string {
	var rows []string
	endIdx := m.offset + 10
	if endIdx > len(m.Transactions) {
		endIdx = len(m.Transactions)
	}
	for i := m.offset; i < endIdx; i++ {
		rows = append(rows, m.renderRowToken(i))
	}
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func (m TokenHistoryModel) renderRowToken(idx int) string {
	tx := m.Transactions[idx]
	isSelected := idx == m.cursor
	dateStr := tx.Timestamp
	blockStr := histUint64ToStr(tx.Height)
	var typeStr string
	if tx.Coinbase {
		typeStr = "⛏ MINED"
	} else if tx.Amount >= 0 {
		typeStr = "↓ IN"
	} else {
		typeStr = "↑ OUT"
	}
	amountStr := formatTxAmountToken(tx.Amount, m.Decimals)
	selectorStr := " "
	if isSelected {
		selectorStr = "▸"
	}
	if isSelected {
		rowStyle := lipgloss.NewStyle().Background(styles.ColorPrimary).Foreground(styles.ColorText).Bold(true)
		selectorCol := rowStyle.Copy().Width(2).Render(selectorStr)
		dateCol := rowStyle.Copy().Width(12).Render(dateStr)
		blockCol := rowStyle.Copy().Width(10).Render(blockStr)
		typeCol := rowStyle.Copy().Width(10).Render(typeStr)
		amountCol := rowStyle.Copy().Width(18).Render(amountStr)
		return lipgloss.JoinHorizontal(lipgloss.Left, selectorCol, dateCol, blockCol, typeCol, amountCol)
	}
	var typeStyle, amountStyle lipgloss.Style
	if tx.Coinbase || tx.Amount >= 0 {
		typeStyle = styles.TxInStyle
		amountStyle = styles.TxInStyle
	} else {
		typeStyle = styles.TxOutStyle
		amountStyle = styles.TxOutStyle
	}
	selectorCol := lipgloss.NewStyle().Width(2).Render(selectorStr)
	dateCol := lipgloss.NewStyle().Width(12).Render(dateStr)
	blockCol := lipgloss.NewStyle().Width(10).Render(blockStr)
	typeCol := lipgloss.NewStyle().Width(10).Render(typeStyle.Render(typeStr))
	amountCol := lipgloss.NewStyle().Width(18).Render(amountStyle.Render(amountStr))
	return lipgloss.JoinHorizontal(lipgloss.Left, selectorCol, dateCol, blockCol, typeCol, amountCol)
}

func formatTxAmountToken(amount int64, decimals uint64) string {
	if decimals == 0 {
		if amount >= 0 {
			return "+" + fmt.Sprintf("%d", amount)
		}
		return fmt.Sprintf("%d", amount)
	}
	neg := amount < 0
	if neg {
		amount = -amount
	}
	divisor := uint64(1)
	for i := uint64(0); i < decimals; i++ {
		divisor *= 10
	}
	whole := uint64(amount) / divisor
	frac := uint64(amount) % divisor
	fracStr := fmt.Sprintf("%0*d", decimals, frac)
	fracStr = stringsTrimRight(fracStr, "0")
	if fracStr == "" {
		fracStr = "0"
	}
	s := fmt.Sprintf("%d.%s", whole, fracStr)
	if neg {
		return "-" + s
	}
	return "+" + s
}

func stringsTrimRight(s, cutset string) string {
	// Avoid importing strings just for TrimRight.
	for len(s) > 0 && len(cutset) == 1 && s[len(s)-1] == cutset[0] {
		s = s[:len(s)-1]
	}
	return s
}

func (m TokenHistoryModel) HandleMouse(msg tea.MouseClickMsg, windowWidth, windowHeight int) TokenHistoryModel {
	boxWidth := styles.Width
	boxX := (windowWidth - boxWidth) / 2
	boxY := (windowHeight - styles.SmallBoxHeight) / 2
	relX := msg.Mouse().X - boxX - 4
	relY := msg.Mouse().Y - boxY - 3
	switch msg.Button {
	case tea.MouseLeft:
		if relX >= 0 && relX < 60 && relY >= 2 && relY < 2+10 {
			if len(m.Transactions) > 0 {
				rowIndex := relY - 2 + m.offset
				if rowIndex < len(m.Transactions) {
					m.cursor = rowIndex
				}
			}
		}
	case tea.MouseWheelUp:
		if m.cursor > 0 {
			m.cursor--
			if m.cursor < m.offset {
				m.offset = m.cursor
			}
		}
	case tea.MouseWheelDown:
		if m.cursor < len(m.Transactions)-1 {
			m.cursor++
			if m.cursor >= m.offset+10 {
				m.offset = m.cursor - 9
			}
		}
	}
	return m
}
