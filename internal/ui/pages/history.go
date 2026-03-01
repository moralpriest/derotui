// Copyright 2017-2026 DERO Project. All rights reserved.

package pages

import (
	"fmt"
	"log"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/deroproject/dero-wallet-cli/internal/ui/styles"
)

var (
	historyExportKeys = key.NewBinding(key.WithKeys("e", "E"))
	historyPgUpKeys   = key.NewBinding(key.WithKeys("pgup"))
	historyPgDownKeys = key.NewBinding(key.WithKeys("pgdown"))
	historyHomeKeys   = key.NewBinding(key.WithKeys("home"))
	historyEndKeys    = key.NewBinding(key.WithKeys("end"))
)

// HistoryModel represents the history page with a custom table renderer
type HistoryModel struct {
	Transactions  []Transaction
	cursor        int
	offset        int
	wantDetails   bool
	wantExport    bool
	exportMessage string
	exportSuccess bool
	lastClickRow  int
	lastClickAt   time.Time
}

// NewHistory creates a new history page
func NewHistory() HistoryModel {
	return HistoryModel{
		Transactions: []Transaction{},
		cursor:       0,
		offset:       0,
		lastClickRow: -1,
	}
}

// Init initializes the history page
func (h HistoryModel) Init() tea.Cmd {
	return nil
}

// Update handles events
func (h HistoryModel) Update(msg tea.Msg) (HistoryModel, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, pageEnterKeys):
			if len(h.Transactions) > 0 {
				log.Printf("[DEBUG History] Enter pressed - WantDetails for tx %d", h.cursor)
				h.wantDetails = true
			}
		case key.Matches(msg, historyExportKeys):
			if len(h.Transactions) > 0 {
				log.Printf("[DEBUG History] Export pressed - WantExport")
				h.wantExport = true
			}
		case key.Matches(msg, pageUpKeys):
			h.cursorUp()
		case key.Matches(msg, pageDownKeys):
			h.cursorDown()
		case key.Matches(msg, historyPgUpKeys):
			h.cursorPgUp()
		case key.Matches(msg, historyPgDownKeys):
			h.cursorPgDown()
		case key.Matches(msg, historyHomeKeys):
			h.cursor = 0
			h.offset = 0
		case key.Matches(msg, historyEndKeys):
			h.cursor = len(h.Transactions) - 1
			h.offset = max(0, len(h.Transactions)-10)
		}
	}

	return h, cmd
}

func (h *HistoryModel) cursorUp() {
	if h.cursor > 0 {
		h.cursor--
		if h.cursor < h.offset {
			h.offset = h.cursor
		}
	}
}

func (h *HistoryModel) cursorDown() {
	if h.cursor < len(h.Transactions)-1 {
		h.cursor++
		if h.cursor >= h.offset+10 {
			h.offset = h.cursor - 9
		}
	}
}

func (h *HistoryModel) cursorPgUp() {
	h.cursor -= 10
	if h.cursor < 0 {
		h.cursor = 0
	}
	h.offset = h.cursor
	if h.offset > 0 {
		h.offset = max(0, h.offset-9)
	}
}

func (h *HistoryModel) cursorPgDown() {
	h.cursor += 10
	if h.cursor >= len(h.Transactions) {
		h.cursor = len(h.Transactions) - 1
	}
	h.offset = max(0, h.cursor-9)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// View renders the history page
func (h HistoryModel) View() string {
	if len(h.Transactions) == 0 {
		return styles.MutedStyle.Render("No transactions yet")
	}

	// Build the view
	var sections []string

	// Header (contains column headers)
	header := h.renderHeader()
	sections = append(sections, header)

	// Rows
	rows := h.renderRows()
	sections = append(sections, rows)

	// Pagination info
	pageInfo := styles.MutedStyle.Render(
		fmt.Sprintf("Showing %d-%d of %d", h.cursor+1, h.cursor+1, len(h.Transactions)),
	)
	sections = append(sections, pageInfo)

	// Export message (if any)
	if h.exportMessage != "" {
		var msgStyle lipgloss.Style
		if h.exportSuccess {
			msgStyle = styles.SuccessStyle
		} else {
			msgStyle = styles.ErrorStyle
		}
		sections = append(sections, msgStyle.Render(h.exportMessage))
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (h HistoryModel) renderHeader() string {
	// Column headers in muted gray
	selectorCol := lipgloss.NewStyle().Width(2).Render("")
	dateCol := lipgloss.NewStyle().Width(12).Bold(true).Foreground(styles.ColorMuted).Render("Date")
	blockCol := lipgloss.NewStyle().Width(10).Bold(true).Foreground(styles.ColorMuted).Render("Block")
	typeCol := lipgloss.NewStyle().Width(10).Bold(true).Foreground(styles.ColorMuted).Render("Type")
	amountCol := lipgloss.NewStyle().Width(18).Bold(true).Foreground(styles.ColorMuted).Render("Amount")

	headerRow := lipgloss.JoinHorizontal(lipgloss.Left, selectorCol, dateCol, blockCol, typeCol, amountCol)
	separator := styles.StyledSeparator(52)

	return lipgloss.JoinVertical(lipgloss.Left, headerRow, separator)
}

func (h HistoryModel) renderRows() string {
	var rows []string

	endIdx := h.offset + 10
	if endIdx > len(h.Transactions) {
		endIdx = len(h.Transactions)
	}

	for i := h.offset; i < endIdx; i++ {
		row := h.renderRow(i)
		rows = append(rows, row)
	}

	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func (h HistoryModel) renderRow(idx int) string {
	tx := h.Transactions[idx]
	isSelected := idx == h.cursor

	// Date
	dateStr := tx.Timestamp

	// Block
	blockStr := histUint64ToStr(tx.Height)

	// Type text (without color for now)
	var typeStr string
	if tx.Coinbase {
		typeStr = "⛏ MINED"
	} else if tx.Amount >= 0 {
		typeStr = "↓ IN"
	} else {
		typeStr = "↑ OUT"
	}

	// Amount text
	amountStr := formatTxAmount(tx.Amount)

	// Selection indicator
	var selectorStr string
	if isSelected {
		selectorStr = "▸"
	} else {
		selectorStr = " "
	}

	if isSelected {
		// Selected row: uniform styling across all columns
		rowStyle := lipgloss.NewStyle().
			Background(styles.ColorPrimary).
			Foreground(styles.ColorText).
			Bold(true)

		selectorCol := rowStyle.Copy().Width(2).Render(selectorStr)
		dateCol := rowStyle.Copy().Width(12).Render(dateStr)
		blockCol := rowStyle.Copy().Width(10).Render(blockStr)
		typeCol := rowStyle.Copy().Width(10).Render(typeStr)
		amountCol := rowStyle.Copy().Width(18).Render(amountStr)

		return lipgloss.JoinHorizontal(lipgloss.Left, selectorCol, dateCol, blockCol, typeCol, amountCol)
	}

	// Non-selected row: apply type/amount colors
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

// SetTransactions sets the transaction list
func (h *HistoryModel) SetTransactions(txs []Transaction) {
	h.Transactions = txs
	h.cursor = 0
	h.offset = 0
	h.lastClickRow = -1
	h.lastClickAt = time.Time{}
}

// SelectedTx returns the selected transaction
func (h HistoryModel) SelectedTx() *Transaction {
	if h.cursor >= 0 && h.cursor < len(h.Transactions) {
		return &h.Transactions[h.cursor]
	}
	return nil
}

// WantDetails returns true if user wants to see details
func (h HistoryModel) WantDetails() bool {
	return h.wantDetails
}

// WantExport returns true if user wants to export history
func (h HistoryModel) WantExport() bool {
	return h.wantExport
}

// SetExportMessage sets the export result message
func (h *HistoryModel) SetExportMessage(msg string, success bool) {
	h.exportMessage = msg
	h.exportSuccess = success
}

// ClearExportMessage clears the export message
func (h *HistoryModel) ClearExportMessage() {
	h.exportMessage = ""
}

// ResetActions resets action flags
func (h *HistoryModel) ResetActions() {
	h.wantDetails = false
	h.wantExport = false
}

// HandleMouse handles mouse events on the history page
func (h HistoryModel) HandleMouse(msg tea.MouseClickMsg, windowWidth, windowHeight int) HistoryModel {
	// Calculate centered position
	boxWidth := styles.Width
	boxX := (windowWidth - boxWidth) / 2
	boxY := (windowHeight - styles.SmallBoxHeight) / 2

	// Adjust to content area (after title)
	relX := msg.Mouse().X - boxX - 4
	relY := msg.Mouse().Y - boxY - 3

	switch msg.Button {
	case tea.MouseLeft:
		// Table header is at y=0-1, rows start at y=2
		// Each row is 1 line
		if relX >= 0 && relX < 60 && relY >= 2 && relY < 2+10 {
			if len(h.Transactions) > 0 {
				rowIndex := relY - 2 + h.offset
				if rowIndex < len(h.Transactions) {
					h.cursor = rowIndex
					now := time.Now()
					if h.lastClickRow == rowIndex && now.Sub(h.lastClickAt) <= 500*time.Millisecond {
						h.wantDetails = true
					}
					h.lastClickRow = rowIndex
					h.lastClickAt = now
				}
			}
		}

	case tea.MouseWheelUp:
		h.cursorUp()

	case tea.MouseWheelDown:
		h.cursorDown()
	}

	return h
}

func formatTxAmount(amount int64) string {
	negative := amount < 0
	if negative {
		amount = -amount
	}
	whole := amount / 100000
	frac := amount % 100000

	result := histIntToStr64(whole) + "." + histPadLeft64(uint64(frac), 5)
	if negative {
		result = "-" + result
	} else {
		result = "+" + result
	}
	return result
}

func histIntToStr64(n int64) string {
	return styles.IntToStr(n)
}

func histPadLeft64(n uint64, width int) string {
	s := styles.UintToStr(n)
	for len(s) < width {
		s = "0" + s
	}
	return s
}

func histUint64ToStr(n uint64) string {
	return styles.UintToStr(n)
}
