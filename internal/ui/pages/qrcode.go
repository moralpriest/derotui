// Copyright 2017-2026 DERO Project. All rights reserved.

package pages

import (
	"log"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/atotto/clipboard"
	"github.com/deroproject/dero-wallet-cli/internal/ui/styles"
	"github.com/skip2/go-qrcode"
)

// QRCodeModel represents the QR code display screen
type QRCodeModel struct {
	Address   string
	qrString  string
	copied    bool
	cancelled bool
	error     string
}

// NewQRCode creates a new QR code display screen
func NewQRCode(address string) QRCodeModel {
	m := QRCodeModel{
		Address: address,
	}
	m.generateQR()
	return m
}

// generateQR generates the QR code string representation
func (q *QRCodeModel) generateQR() {
	if q.Address == "" {
		q.error = "No address available"
		return
	}

	// Generate QR code with low recovery level for smaller size
	qr, err := qrcode.New(q.Address, qrcode.Low)
	if err != nil {
		q.error = "Failed to generate QR code"
		return
	}
	qr.DisableBorder = true

	// Convert to ASCII art string using Unicode blocks
	q.qrString = qrToString(qr)
}

// qrToString converts a QR code to a compact string using Unicode half blocks
func qrToString(qr *qrcode.QRCode) string {
	bitmap := qr.Bitmap()
	size := len(bitmap)

	var sb strings.Builder

	// Process two rows at a time using half blocks for compact display
	for y := 0; y < size; y += 2 {
		for x := 0; x < size; x++ {
			top := bitmap[y][x]
			bottom := false
			if y+1 < size {
				bottom = bitmap[y+1][x]
			}

			// Use Unicode half blocks for compact representation
			// true = black (QR module), false = white (background)
			if top && bottom {
				sb.WriteString("\u2588") // Full block (both black)
			} else if top && !bottom {
				sb.WriteString("\u2580") // Upper half block
			} else if !top && bottom {
				sb.WriteString("\u2584") // Lower half block
			} else {
				sb.WriteString(" ") // Space (both white)
			}
		}
		// Add newline after each row except the last one
		if y+2 < size {
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// Init initializes the QR code screen
func (q QRCodeModel) Init() tea.Cmd {
	return nil
}

// Update handles events
func (q QRCodeModel) Update(msg tea.Msg) (QRCodeModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, pageEscKeys), key.Matches(msg, pageEnterKeys):
			log.Printf("[DEBUG qrcode] Closed QR code view")
			q.cancelled = true
			return q, nil
		case key.Matches(msg, pageCopyKeys):
			if err := clipboard.WriteAll(q.Address); err == nil {
				q.copied = true
				log.Printf("[DEBUG qrcode] Copied address to clipboard")
			}
			return q, nil
		}
	}
	return q, nil
}

// View renders the QR code screen
func (q QRCodeModel) View() string {
	title := styles.TitleStyle.Render("◫ Wallet Address QR Code")

	// QR code content
	var content string
	if q.error != "" {
		content = styles.ErrorStyle.Render(q.error)
	} else {
		// Style the QR code with the accent color
		qrStyled := lipgloss.NewStyle().
			Foreground(styles.ColorAccent).
			Render(q.qrString)
		content = qrStyled
	}

	// Full address below QR code in a dedicated box (wallet-style)
	addressBoxWidth := styles.Width - 10
	if addressBoxWidth < 40 {
		addressBoxWidth = 40
	}
	addressLabel := "Address: "
	visibleAddr := q.Address
	maxAddrLen := addressBoxWidth - 10 - len(addressLabel)
	if maxAddrLen < 16 {
		maxAddrLen = 16
	}
	if len(visibleAddr) > maxAddrLen {
		visibleAddr = visibleAddr[:maxAddrLen-3] + "..."
	}
	addressLine := lipgloss.NewStyle().Foreground(styles.ColorAccent).Render("⎘ ") +
		styles.MutedStyle.Render(addressLabel) +
		styles.TextStyle.Render(visibleAddr)
	addressBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.ColorBorder).
		Padding(0, 1).
		Width(addressBoxWidth).
		Align(lipgloss.Left).
		Render(addressLine)

	// Copied message
	var copiedMsg string
	if q.copied {
		copiedMsg = styles.SuccessStyle.Render("✓ Copied to clipboard!")
	}

	// Help
	help := styles.MutedStyle.Render("C Copy Address • Enter/Esc Back")

	// Compose elements compactly to fit smaller terminals
	elements := []string{
		title,
		"",
		content,
		addressBox,
		help,
	}

	if copiedMsg != "" {
		elements = append(elements, copiedMsg)
	}

	composed := lipgloss.JoinVertical(lipgloss.Center, elements...)

	// Use minimal padding to avoid clipping at top in short terminals
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.ColorBorder).
		Width(styles.Width).
		Align(lipgloss.Center).
		Padding(0, 1).
		Render(composed)
}

// Cancelled returns whether user cancelled/dismissed the screen
func (q QRCodeModel) Cancelled() bool {
	return q.cancelled
}

// Reset resets the model state
func (q *QRCodeModel) Reset() {
	q.cancelled = false
	q.copied = false
}

// HandleMouse handles mouse events on the QR code page
func (q QRCodeModel) HandleMouse(msg tea.MouseClickMsg, windowWidth, windowHeight int) QRCodeModel {
	// Calculate centered position
	boxWidth := styles.Width
	boxX := (windowWidth - boxWidth) / 2
	boxY := (windowHeight - styles.MediumBoxHeight) / 2

	// Adjust to content area
	relX := msg.Mouse().X - boxX - 4
	relY := msg.Mouse().Y - boxY - 2 // After title, subtitle

	switch msg.Button {
	case tea.MouseLeft:
		// Copy button area: below QR code, around y=12-13
		if relY >= 12 && relY <= 13 && relX >= 10 && relX < 50 {
			if err := clipboard.WriteAll(q.Address); err == nil {
				q.copied = true
			}
			return q
		}
		// Back button area: y=15-16
		if relY >= 15 && relY <= 16 {
			q.cancelled = true
			return q
		}

	case tea.MouseWheelUp, tea.MouseWheelDown:
		// QR code doesn't scroll
	}

	return q
}
