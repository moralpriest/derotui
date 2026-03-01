// Copyright 2017-2026 DERO Project. All rights reserved.

package pages

import (
	"fmt"
	"log"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/atotto/clipboard"
	"github.com/deroproject/dero-wallet-cli/internal/ui/styles"
)

var (
	txDetailsCopyTxIDKeys        = key.NewBinding(key.WithKeys("t"))
	txDetailsCopyHeightKeys      = key.NewBinding(key.WithKeys("h"))
	txDetailsCopySenderKeys      = key.NewBinding(key.WithKeys("s"))
	txDetailsCopyDestinationKeys = key.NewBinding(key.WithKeys("d"))
	txDetailsCopyProofKeys       = key.NewBinding(key.WithKeys("p"))
	txDetailsCopyMessageKeys     = key.NewBinding(key.WithKeys("m"))
	txDetailsCopyPortKeys        = key.NewBinding(key.WithKeys("i"))
)

// TxDetailsModel represents the transaction details page
type TxDetailsModel struct {
	tx          Transaction
	cancelled   bool
	copyMessage string // flash message for copy confirmation
}

// NewTxDetails creates a new transaction details page
func NewTxDetails() TxDetailsModel {
	return TxDetailsModel{}
}

// Init initializes the transaction details page
func (t TxDetailsModel) Init() tea.Cmd {
	return nil
}

// Update handles events
func (t TxDetailsModel) Update(msg tea.Msg) (TxDetailsModel, tea.Cmd) {
	// Clear copy message on any key
	t.copyMessage = ""

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, pageEscKeys):
			log.Printf("[DEBUG txdetails] Closed transaction details")
			t.cancelled = true
		case key.Matches(msg, txDetailsCopyTxIDKeys):
			// Copy TxID
			if t.tx.TxID != "" {
				clipboard.WriteAll(t.tx.TxID)
				t.copyMessage = "TxID copied!"
				log.Printf("[DEBUG txdetails] Copied TxID")
			} else {
				t.copyMessage = "No TxID to copy"
			}
		case key.Matches(msg, txDetailsCopyHeightKeys):
			// Copy Block Height
			clipboard.WriteAll(fmt.Sprintf("%d", t.tx.Height))
			t.copyMessage = "Block height copied!"
			log.Printf("[DEBUG txdetails] Copied block height")
		case key.Matches(msg, txDetailsCopySenderKeys):
			// Copy Sender
			if t.tx.Sender != "" {
				clipboard.WriteAll(t.tx.Sender)
				t.copyMessage = "Sender copied!"
				log.Printf("[DEBUG txdetails] Copied sender")
			} else {
				t.copyMessage = "No sender to copy"
			}
		case key.Matches(msg, txDetailsCopyDestinationKeys):
			// Copy Destination
			if t.tx.Destination != "" {
				clipboard.WriteAll(t.tx.Destination)
				t.copyMessage = "Destination copied!"
				log.Printf("[DEBUG txdetails] Copied destination")
			} else {
				t.copyMessage = "No destination to copy"
			}
		case key.Matches(msg, txDetailsCopyProofKeys):
			// Copy Proof
			if t.tx.Proof != "" {
				clipboard.WriteAll(t.tx.Proof)
				t.copyMessage = "Proof copied!"
				log.Printf("[DEBUG txdetails] Copied proof")
			} else {
				t.copyMessage = "No proof to copy"
			}
		case key.Matches(msg, txDetailsCopyMessageKeys):
			// Copy Message
			if t.tx.Message != "" {
				clipboard.WriteAll(t.tx.Message)
				t.copyMessage = "Message copied!"
				log.Printf("[DEBUG txdetails] Copied message")
			} else {
				t.copyMessage = "No message to copy"
			}
		case key.Matches(msg, txDetailsCopyPortKeys):
			// Copy Destination/Source Port (Payment ID)
			if t.tx.DestinationPort != 0 {
				clipboard.WriteAll(fmt.Sprintf("%d", t.tx.DestinationPort))
				t.copyMessage = "Destination Port copied!"
				log.Printf("[DEBUG txdetails] Copied destination port")
			} else if t.tx.SourcePort != 0 {
				clipboard.WriteAll(fmt.Sprintf("%d", t.tx.SourcePort))
				t.copyMessage = "Source Port copied!"
				log.Printf("[DEBUG txdetails] Copied source port")
			} else {
				t.copyMessage = "No port to copy"
			}
		}
	}
	return t, nil
}

// View renders the transaction details page
func (t TxDetailsModel) View() string {
	// Constants for layout
	const totalWidth = 68
	const labelWidth = 14

	// Helper functions
	padLabel := func(label string) string {
		for len(label) < labelWidth {
			label += " "
		}
		return label
	}

	sectionHeader := func(title string) string {
		prefix := "── "
		suffix := " "
		lineLen := totalWidth - len(prefix) - len(title) - len(suffix)
		if lineLen < 0 {
			lineLen = 0
		}
		line := styles.Separator(lineLen)
		return styles.MutedStyle.Render(prefix + title + suffix + line)
	}

	row := func(label, value string) string {
		return lipgloss.NewStyle().Width(totalWidth).Render(
			styles.MutedStyle.Render(padLabel(label)) + value,
		)
	}

	// Type display
	var typeStr string
	if t.tx.Coinbase {
		typeStr = styles.TxInStyle.Render("⛏ Miner Reward")
	} else if t.tx.Incoming {
		typeStr = styles.TxInStyle.Render("↓ Incoming")
	} else {
		typeStr = styles.TxOutStyle.Render("↑ Outgoing")
	}

	// Status display
	var statusStr string
	switch t.tx.Status {
	case 0:
		statusStr = styles.SuccessStyle.Render("Confirmed")
	case 1:
		statusStr = styles.WarningStyle.Render("Spent")
	default:
		statusStr = styles.MutedStyle.Render("Unknown")
	}

	// Amount display
	var amountStr string
	if t.tx.Coinbase || t.tx.Incoming {
		amountStr = styles.TxInStyle.Render("+" + txdFormatAtomic(uint64(t.tx.Amount)) + " DERO")
	} else {
		amountStr = styles.TxOutStyle.Render(txdFormatAmount(t.tx.Amount) + " DERO")
	}

	// Fee display
	var feeStr string
	if t.tx.Fee > 0 {
		feeStr = styles.TextStyle.Render(txdFormatAtomic(t.tx.Fee) + " DERO")
	} else {
		feeStr = styles.MutedStyle.Render("-")
	}

	// Burn display
	var burnStr string
	if t.tx.Burn > 0 {
		burnStr = styles.WarningStyle.Render(txdFormatAtomic(t.tx.Burn) + " DERO")
	} else {
		burnStr = styles.MutedStyle.Render("-")
	}

	// Block info
	heightStr := styles.TextStyle.Render(fmt.Sprintf("%d", t.tx.Height))
	topoStr := styles.TextStyle.Render(fmt.Sprintf("%d", t.tx.TopoHeight))
	dateStr := styles.TextStyle.Render(t.tx.Timestamp)

	// TxID
	var txIDStr string
	if t.tx.TxID != "" {
		txIDStr = styles.AccentStyle.Render(txdTruncate(t.tx.TxID, 52))
	} else {
		txIDStr = styles.MutedStyle.Render("-")
	}

	// Block Hash
	var blockHashStr string
	if t.tx.BlockHash != "" {
		blockHashStr = styles.TextStyle.Render(txdTruncate(t.tx.BlockHash, 52))
	} else {
		blockHashStr = styles.MutedStyle.Render("-")
	}

	// Sender
	var senderStr string
	if t.tx.Sender != "" {
		senderStr = styles.TextStyle.Render(txdTruncate(t.tx.Sender, 52))
	} else {
		senderStr = styles.MutedStyle.Render("-")
	}

	// Destination
	var destStr string
	if t.tx.Destination != "" {
		destStr = styles.TextStyle.Render(txdTruncate(t.tx.Destination, 52))
	} else {
		destStr = styles.MutedStyle.Render("-")
	}

	// Proof
	var proofStr string
	if t.tx.Proof != "" {
		proofStr = styles.TextStyle.Render(txdTruncate(t.tx.Proof, 52))
	} else {
		proofStr = styles.MutedStyle.Render("-")
	}

	// Handle Message separately with proper wrapping
	var messageRow string
	if t.tx.Message != "" {
		messageRow = styles.MutedStyle.Render(padLabel("Message:")) + txdFormatMessage(t.tx.Message, labelWidth, totalWidth)
	} else {
		messageRow = row("Message:", styles.MutedStyle.Render("-"))
	}

	// Build content with section headers
	txRows := []string{
		sectionHeader("Overview"),
		row("Type:", typeStr),
		row("Status:", statusStr),
		row("Amount:", amountStr),
		row("Fee:", feeStr),
		row("Burn:", burnStr),
		"",
		sectionHeader("Block"),
		row("Height:", heightStr),
		row("Topo:", topoStr),
		row("Date/Time:", dateStr),
		"",
		sectionHeader("Transaction"),
		row("TxID:", txIDStr),
		row("Block Hash:", blockHashStr),
		row("Sender:", senderStr),
	}

	// Show Destination for all transactions
	txRows = append(txRows, row("Destination:", destStr))

	txRows = append(txRows, row("Proof:", proofStr), messageRow)

	content := lipgloss.JoinVertical(lipgloss.Left, txRows...)

	// Destination/Source Port info (payment ID) shown only when non-zero
	if t.tx.DestinationPort != 0 || t.tx.SourcePort != 0 {
		portRows := []string{"", sectionHeader("Ports (Payment ID)")}

		if t.tx.DestinationPort != 0 {
			destPortStr := fmt.Sprintf("%d", t.tx.DestinationPort)
			portRows = append(portRows, row("Dest Port:", styles.AccentStyle.Render(destPortStr)))
		}

		if t.tx.SourcePort != 0 {
			srcPortStr := fmt.Sprintf("%d", t.tx.SourcePort)
			portRows = append(portRows, row("Source Port:", styles.TextStyle.Render(srcPortStr)))
		}

		content = lipgloss.JoinVertical(lipgloss.Left, content, lipgloss.JoinVertical(lipgloss.Left, portRows...))
	}

	// Copy message (flash feedback)
	if t.copyMessage != "" {
		content = lipgloss.JoinVertical(lipgloss.Left,
			content,
			"",
			styles.SuccessStyle.Render("✓ "+t.copyMessage),
		)
	}

	// Footer hints
	footer := "T TxID • H Height • S Sender • D Dest • P Proof • M Msg"
	if t.tx.DestinationPort != 0 || t.tx.SourcePort != 0 {
		footer += " • I Port"
	}
	footer += " • Esc"
	content = lipgloss.JoinVertical(lipgloss.Left, content, "", styles.MutedStyle.Render(footer))

	return content
}

// SetTransaction sets the transaction to display
func (t *TxDetailsModel) SetTransaction(tx Transaction) {
	t.tx = tx
	t.cancelled = false
	t.copyMessage = ""
}

// Cancelled returns true if the user wants to close the details view
func (t TxDetailsModel) Cancelled() bool {
	return t.cancelled
}

// Reset resets the details page state
func (t *TxDetailsModel) Reset() {
	t.cancelled = false
	t.copyMessage = ""
}

// HandleMouse handles mouse events on the transaction details page
func (t TxDetailsModel) HandleMouse(msg tea.MouseClickMsg, windowWidth, windowHeight int) TxDetailsModel {
	// Calculate centered position
	boxWidth := styles.Width
	boxX := (windowWidth - boxWidth) / 2
	boxY := (windowHeight - styles.MediumBoxHeight) / 2

	// Adjust to content area
	relX := msg.Mouse().X - boxX - 4
	relY := msg.Mouse().Y - boxY - 3

	// Clickable area for values (after the label)
	const labelWidth = 14
	const valueStartX = labelWidth

	switch msg.Button {
	case tea.MouseLeft:
		// Check if clicking on copyable rows
		// TxID is at row 13
		if relX >= valueStartX && relY == 13 {
			if t.tx.TxID != "" {
				clipboard.WriteAll(t.tx.TxID)
				t.copyMessage = "TxID copied!"
			}
			return t
		}

		// Sender is at row 15
		if relX >= valueStartX && relY == 15 {
			if t.tx.Sender != "" {
				clipboard.WriteAll(t.tx.Sender)
				t.copyMessage = "Sender copied!"
			}
			return t
		}

		// Destination is at row 16
		if relX >= valueStartX && relY == 16 {
			if t.tx.Destination != "" {
				clipboard.WriteAll(t.tx.Destination)
				t.copyMessage = "Destination copied!"
			}
			return t
		}

		// Proof is at row 17
		if relX >= valueStartX && relY == 17 {
			if t.tx.Proof != "" {
				clipboard.WriteAll(t.tx.Proof)
				t.copyMessage = "Proof copied!"
			}
			return t
		}

		// Click on help area to cancel (Esc)
		if relY >= 22 {
			t.cancelled = true
		}

	case tea.MouseWheelUp, tea.MouseWheelDown:
		// Could implement scrolling for long content
		// For now, do nothing
	}

	return t
}

// Helper functions

// txdChunkSplit breaks a long string into fixed-size chunks
func txdChunkSplit(s string, chunkSize int) []string {
	var chunks []string
	for len(s) > 0 {
		size := chunkSize
		if size > len(s) {
			size = len(s)
		}
		chunks = append(chunks, s[:size])
		s = s[size:]
	}
	return chunks
}

func txdFormatAmount(amount int64) string {
	negative := amount < 0
	if negative {
		amount = -amount
	}
	whole := amount / 100000
	frac := amount % 100000

	result := txdIntToStr(whole) + "." + txdPadLeft(uint64(frac), 5)
	if negative {
		result = "-" + result
	}
	return result
}

func txdFormatAtomic(atomic uint64) string {
	whole := atomic / 100000
	frac := atomic % 100000
	return txdIntToStr(int64(whole)) + "." + txdPadLeft(frac, 5)
}

func txdIntToStr(n int64) string {
	return styles.IntToStr(n)
}

func txdPadLeft(n uint64, width int) string {
	s := styles.UintToStr(n)
	for len(s) < width {
		s = "0" + s
	}
	return s
}

func txdTruncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	half := (maxLen - 3) / 2
	return s[:half] + "..." + s[len(s)-half:]
}

// txdFormatMessage formats a message with proper wrapping and indentation
// labelWidth is the width of the label column for alignment
// availableWidth is the total available width for the content area
func txdFormatMessage(msg string, labelWidth int, availableWidth int) string {
	if msg == "" {
		return styles.MutedStyle.Render("-")
	}

	// Calculate content width (available space after label)
	contentWidth := availableWidth - labelWidth - 1 // -1 for safety margin
	if contentWidth < 20 {
		contentWidth = 40 // minimum reasonable width
	}

	// Wrap the text
	words := strings.Fields(msg)
	if len(words) == 0 {
		return styles.TextStyle.Render(msg)
	}

	var lines []string
	var currentLine strings.Builder

	for _, word := range words {
		// Check if word is too long - force-break it
		if len(word) > contentWidth {
			// Flush current line first if it has content
			if currentLine.Len() > 0 {
				lines = append(lines, currentLine.String())
				currentLine.Reset()
			}

			// Split long word into chunks
			chunks := txdChunkSplit(word, contentWidth)
			for i, chunk := range chunks {
				if i < len(chunks)-1 {
					// All but last chunk go on their own line
					lines = append(lines, chunk)
				} else {
					// Last chunk goes into currentLine for potential continuation
					currentLine.WriteString(chunk)
				}
			}
		} else if currentLine.Len() == 0 {
			// First word on this line
			currentLine.WriteString(word)
		} else if currentLine.Len()+1+len(word) <= contentWidth {
			// Word fits on current line
			currentLine.WriteString(" ")
			currentLine.WriteString(word)
		} else {
			// Word doesn't fit, start new line
			lines = append(lines, currentLine.String())
			currentLine.Reset()
			currentLine.WriteString(word)
		}
	}
	if currentLine.Len() > 0 {
		lines = append(lines, currentLine.String())
	}

	// Build result with proper indentation
	if len(lines) == 0 {
		return styles.TextStyle.Render(msg)
	}

	// First line is already positioned after the label
	var result strings.Builder
	result.WriteString(styles.TextStyle.Render(lines[0]))

	// Continuation lines need indentation to align with first line of text
	indent := strings.Repeat(" ", labelWidth)
	for i := 1; i < len(lines); i++ {
		result.WriteString("\n")
		result.WriteString(indent)
		result.WriteString(styles.TextStyle.Render(lines[i]))
	}

	return result.String()
}
