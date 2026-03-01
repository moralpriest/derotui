// Copyright 2017-2026 DERO Project. All rights reserved.

package pages

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"unicode/utf8"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/atotto/clipboard"
	"github.com/deroproject/dero-wallet-cli/internal/ui/components"
	"github.com/deroproject/dero-wallet-cli/internal/ui/styles"
	"github.com/deroproject/derohe/globals"
	"github.com/deroproject/derohe/rpc"
)

// IntegratedAddrModel represents the integrated address generation page
type IntegratedAddrModel struct {
	address      string
	portInput    components.InputModel
	messageInput components.InputModel
	amountInput  components.InputModel
	result       string
	error        string
	focusIndex   int
	cancelled    bool
	copied       bool
	generated    bool
	portStatus   FieldStatus
	amountStatus FieldStatus
	wantViewQR   bool // NEW: trigger QR view after generation
}

// NewIntegratedAddr creates a new integrated address page
func NewIntegratedAddr(address string) IntegratedAddrModel {
	m := IntegratedAddrModel{
		address:      address,
		portInput:    components.NewInput("", "Destination Port / Payment ID", false),
		messageInput: components.NewInput("", "Message", false),
		amountInput:  components.NewInput("", "0.00000", false),
		focusIndex:   0,
	}
	m.portInput.SetCharLimit(20)
	m.portInput.SetCharFilter(func(r rune) bool {
		return r >= '0' && r <= '9'
	})
	m.messageInput.SetCharLimit(130)
	m.amountInput.SetCharFilter(func(r rune) bool {
		return (r >= '0' && r <= '9') || r == '.'
	})
	m.portInput.Focus()
	return m
}

// Init initializes the integrated address page
func (i IntegratedAddrModel) Init() tea.Cmd {
	return i.portInput.Init()
}

// atomicUnitsPerDERO is the number of atomic units in 1 DERO (5 decimal places)
const atomicUnitsPerDERO = 100000

// maxDEROAmount is the maximum DERO amount (18.8 million DERO in atomic units)
// This represents the total supply of DERO in the blockchain
const maxDEROAmount uint64 = 18800000 * atomicUnitsPerDERO

// parseAmountToAtomic converts a decimal DERO amount string to atomic units
func parseIntegratedAmountToAtomic(amountStr string) (uint64, error) {
	amountStr = strings.TrimSpace(amountStr)
	if amountStr == "" {
		return 0, nil
	}

	parts := strings.Split(amountStr, ".")
	if len(parts) > 2 {
		return 0, strconv.ErrSyntax
	}

	whole, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil && parts[0] != "" {
		return 0, err
	}

	var frac uint64
	if len(parts) == 2 {
		fracStr := parts[1]
		if len(fracStr) > 5 {
			fracStr = fracStr[:5]
		}
		for len(fracStr) < 5 {
			fracStr += "0"
		}
		frac, err = strconv.ParseUint(fracStr, 10, 64)
		if err != nil {
			return 0, err
		}
	}

	maxUint64 := ^uint64(0)
	if whole > (maxUint64-frac)/atomicUnitsPerDERO {
		return 0, strconv.ErrRange
	}

	return whole*atomicUnitsPerDERO + frac, nil
}

// Update handles events
func (i IntegratedAddrModel) Update(msg tea.Msg) (IntegratedAddrModel, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, pageEnterKeys), key.Matches(msg, pageEscKeys), key.Matches(msg, pageNextFieldKeys), key.Matches(msg, pagePrevFieldKeys), key.Matches(msg, pageCopyKeys):
			// Keep current error state for command/navigation keys.
		default:
			i.error = ""
		}

		switch {
		case key.Matches(msg, pageEscKeys):
			i.cancelled = true
			return i, nil
		case key.Matches(msg, pageNextFieldKeys):
			i.nextFocus()
			return i, (&i).focusCmd()
		case key.Matches(msg, pagePrevFieldKeys):
			i.prevFocus()
			return i, (&i).focusCmd()
		case key.Matches(msg, pageEnterKeys):
			if i.focusIndex == 3 {
				if i.canGenerate() {
					// Generate button
					i.generate()
				}
				return i, nil
			}
		case key.Matches(msg, pageCopyKeys):
			if i.generated && i.result != "" {
				if err := clipboard.WriteAll(i.result); err == nil {
					i.copied = true
					log.Printf("[DEBUG IntegratedAddr] Copied integrated address to clipboard")
				}
			}
		// NEW: 'Y' key to view QR code of generated address
		case key.Matches(msg, key.NewBinding(key.WithKeys("y", "Y"))):
			if i.generated && i.result != "" {
				i.wantViewQR = true
			}
		}
	}

	// Update focused input
	switch i.focusIndex {
	case 0:
		prevPort := i.portInput.Value()
		i.portInput, cmd = i.portInput.Update(msg)
		i.enforcePortLimit(prevPort)
	case 1:
		prevMessage := i.messageInput.Value()
		i.messageInput, cmd = i.messageInput.Update(msg)
		i.enforceMessageByteLimit(prevMessage)
	case 2:
		prevAmount := i.amountInput.Value()
		i.amountInput, cmd = i.amountInput.Update(msg)
		i.enforceAmountLimit(prevAmount)
	}

	// Validate port and amount in real-time
	i.validatePort()
	i.validateAmount()

	return i, cmd
}

func (i *IntegratedAddrModel) validatePort() {
	portStr := strings.TrimSpace(i.portInput.Value())
	if portStr == "" {
		i.portStatus = FieldEmpty
		return
	}
	if _, err := strconv.ParseUint(portStr, 10, 64); err != nil {
		i.portStatus = FieldInvalid
		return
	}
	i.portStatus = FieldValid
}

func (i *IntegratedAddrModel) validateAmount() {
	amountStr := strings.TrimSpace(i.amountInput.Value())
	if amountStr == "" {
		i.amountStatus = FieldEmpty
		return
	}
	amount, err := parseIntegratedAmountToAtomic(amountStr)
	if err != nil {
		i.amountStatus = FieldInvalid
		return
	}
	if amount == 0 || amount > maxDEROAmount {
		i.amountStatus = FieldInvalid
		return
	}
	i.amountStatus = FieldValid
}

func (i *IntegratedAddrModel) enforcePortLimit(previous string) {
	portStr := strings.TrimSpace(i.portInput.Value())
	if portStr == "" {
		return
	}
	if _, err := strconv.ParseUint(portStr, 10, 64); err != nil {
		i.portInput.SetValue(previous)
		i.error = "Destination port must be a valid uint64 number"
	}
}

func (i *IntegratedAddrModel) enforceMessageByteLimit(previous string) {
	message := i.messageInput.Value()
	if len(message) <= MaxMessageBytes {
		return
	}
	i.messageInput.SetValue(previous)
	i.error = fmt.Sprintf("Message exceeds %d byte limit", MaxMessageBytes)
}

func (i *IntegratedAddrModel) enforceAmountLimit(previous string) {
	amountStr := strings.TrimSpace(i.amountInput.Value())
	if amountStr == "" {
		return
	}

	parts := strings.Split(amountStr, ".")
	if len(parts) == 2 && len(parts[1]) > 5 {
		i.amountInput.SetValue(previous)
		i.error = "Amount supports up to 5 decimal places"
		return
	}

	amount, err := parseIntegratedAmountToAtomic(amountStr)
	if err != nil {
		i.amountInput.SetValue(previous)
		i.error = "Invalid amount"
		return
	}

	if amount > maxDEROAmount {
		i.amountInput.SetValue(previous)
		i.error = "Amount exceeds maximum DERO supply"
	}
}

func (i IntegratedAddrModel) canGenerate() bool {
	portStr := strings.TrimSpace(i.portInput.Value())
	message := strings.TrimSpace(i.messageInput.Value())
	amountStr := strings.TrimSpace(i.amountInput.Value())

	if portStr == "" && message == "" && amountStr == "" {
		return false
	}

	if portStr != "" {
		if _, err := strconv.ParseUint(portStr, 10, 64); err != nil {
			return false
		}
	}

	if len(i.messageInput.Value()) > MaxMessageBytes {
		return false
	}

	if amountStr != "" {
		amount, err := parseIntegratedAmountToAtomic(amountStr)
		if err != nil || amount == 0 || amount > maxDEROAmount {
			return false
		}
	}

	return i.portStatus != FieldInvalid && i.amountStatus != FieldInvalid
}

func (i *IntegratedAddrModel) generate() {
	i.error = ""

	// Parse inputs
	var port uint64
	portStr := strings.TrimSpace(i.portInput.Value())
	message := strings.TrimSpace(i.messageInput.Value())
	amountStr := strings.TrimSpace(i.amountInput.Value())

	if portStr == "" && message == "" && amountStr == "" {
		i.error = "Please provide at least a port, message, or amount"
		return
	}

	if portStr != "" {
		var err error
		port, err = strconv.ParseUint(portStr, 10, 64)
		if err != nil {
			i.error = "Destination port must be a valid uint64 number"
			return
		}
	}
	if len(i.messageInput.Value()) > MaxMessageBytes {
		i.error = fmt.Sprintf("Message exceeds %d byte limit", MaxMessageBytes)
		return
	}

	var amount uint64
	if amountStr != "" {
		var err error
		amount, err = parseIntegratedAmountToAtomic(amountStr)
		if err != nil || amount == 0 {
			i.error = "Invalid amount"
			return
		}
		if amount > maxDEROAmount {
			i.error = "Amount exceeds maximum DERO supply"
			return
		}
	}

	// Create integrated address
	addr, err := globals.ParseValidateAddress(i.address)
	if err != nil {
		i.error = fmt.Sprintf("%v", err)
		return
	}

	// Build arguments
	var arguments rpc.Arguments
	if port != 0 {
		arguments = append(arguments, rpc.Argument{
			Name:     rpc.RPC_DESTINATION_PORT,
			DataType: rpc.DataUint64,
			Value:    port,
		})
	}
	if message != "" {
		arguments = append(arguments, rpc.Argument{
			Name:     rpc.RPC_COMMENT,
			DataType: rpc.DataString,
			Value:    message,
		})
	}
	if amount != 0 {
		arguments = append(arguments, rpc.Argument{
			Name:     rpc.RPC_VALUE_TRANSFER,
			DataType: rpc.DataUint64,
			Value:    amount,
		})
	}

	// Set arguments and encode
	addr.Arguments = arguments
	_, err = addr.MarshalText()
	if err != nil {
		i.error = fmt.Sprintf("%v", err)
		return
	}

	i.result = addr.String()
	i.error = ""
	i.generated = true
	i.copied = false
	i.wantViewQR = true // Auto-trigger QR view after successful generation
	log.Printf("[DEBUG IntegratedAddr] Generated integrated address (len=%d)", len(i.result))
}

func (i *IntegratedAddrModel) nextFocus() {
	i.blurAll()
	i.focusIndex = (i.focusIndex + 1) % 4
}

func (i *IntegratedAddrModel) prevFocus() {
	i.blurAll()
	i.focusIndex = (i.focusIndex - 1 + 4) % 4
}

func (i *IntegratedAddrModel) blurAll() {
	i.portInput.Blur()
	i.messageInput.Blur()
	i.amountInput.Blur()
}

func (i *IntegratedAddrModel) focusCmd() tea.Cmd {
	switch i.focusIndex {
	case 0:
		return i.portInput.Focus()
	case 1:
		return i.messageInput.Focus()
	case 2:
		return i.amountInput.Focus()
	}
	return nil
}

// View renders the integrated address page
func (i IntegratedAddrModel) View() string {
	containerWidth := styles.InputWidth + 4
	contentWidth := containerWidth - 4
	canGenerate := i.canGenerate()

	buildHeaderRow := func(leftPlain, rightPlain string, leftStyled, rightStyled string) string {
		gap := contentWidth - utf8.RuneCountInString(leftPlain) - utf8.RuneCountInString(rightPlain)
		if gap < 1 {
			gap = 1
		}
		return leftStyled + strings.Repeat(" ", gap) + rightStyled
	}

	labelStyle := styles.TitleStyle

	// Title
	title := styles.TitleStyle.Render("Payment Request")
	title = lipgloss.NewStyle().Width(contentWidth).Align(lipgloss.Center).Render(title)
	subtitle := styles.SubtitleStyle.Render("Create an address with embedded payment details")
	subtitle = lipgloss.NewStyle().Width(contentWidth).Align(lipgloss.Center).Render(subtitle)
	optsNote := styles.MutedStyle.Render("All fields are optional — provide at least one")
	optsNote = lipgloss.NewStyle().Width(contentWidth).Align(lipgloss.Center).Render(optsNote)

	// Port input section
	portLabelPrefix := "◎ DESTINATION PORT / PAYMENT ID "
	portLabelPlain := portLabelPrefix
	portLabel := labelStyle.Render(portLabelPlain)
	portLen := utf8.RuneCountInString(i.portInput.Value())
	portCounter := strconv.Itoa(portLen) + "/20"
	var portCounterStyled string
	if portLen >= 18 {
		portCounterStyled = lipgloss.NewStyle().Foreground(styles.ColorWarning).Render(portCounter)
	} else {
		portCounterStyled = styles.MutedStyle.Render(portCounter)
	}
	var portStatusStr string
	portStatusPlain := ""
	switch i.portStatus {
	case FieldValid:
		portStatusPlain = " ✓"
		portStatusStr = lipgloss.NewStyle().Foreground(styles.ColorSuccess).Render(" ✓")
	case FieldInvalid:
		portStatusPlain = " ✗"
		portStatusStr = styles.ErrorStyle.Render(" ✗")
	}
	portLabelWithStatusPlain := portLabelPlain + portStatusPlain
	portLabelWithStatus := portLabel + portStatusStr
	portRow := buildHeaderRow(portLabelWithStatusPlain, portCounter, portLabelWithStatus, portCounterStyled)
	portBox := i.portInput.View()

	// Message input section
	messageLabelPrefix := "✉ MESSAGE "
	messageLabelPlain := messageLabelPrefix
	messageLabel := labelStyle.Render(messageLabelPlain)
	messageBytes := len(i.messageInput.Value())
	messageCounter := strconv.Itoa(messageBytes) + "/" + strconv.Itoa(MaxMessageBytes)
	var messageCounterStyled string
	if messageBytes > MaxMessageBytes {
		messageCounterStyled = styles.ErrorStyle.Render(messageCounter)
	} else {
		messageCounterStyled = styles.MutedStyle.Render(messageCounter)
	}
	messageRow := buildHeaderRow(messageLabelPlain, messageCounter, messageLabel, messageCounterStyled)
	messageBox := i.messageInput.View()

	// Amount input section
	amountLabelPrefix := styles.BalanceGlyph + " AMOUNT "
	amountLabelPlain := amountLabelPrefix
	amountLabel := labelStyle.Render(amountLabelPlain)
	var amountStatusStr string
	amountStatusPlain := ""
	switch i.amountStatus {
	case FieldValid:
		amountStatusPlain = " ✓"
		amountStatusStr = lipgloss.NewStyle().Foreground(styles.ColorSuccess).Render(" ✓")
	case FieldInvalid:
		amountStatusPlain = " ✗"
		amountStatusStr = styles.ErrorStyle.Render(" ✗")
	}
	amountLabelWithStatusPlain := amountLabelPlain + amountStatusPlain
	amountLabelWithStatus := amountLabel + amountStatusStr
	amountRow := buildHeaderRow(amountLabelWithStatusPlain, "", amountLabelWithStatus, "")
	amountBox := i.amountInput.View()

	// Generate button styling matches Send page
	var generateBtn string
	buttonText := "Generate"
	if !canGenerate {
		generateBtn = lipgloss.NewStyle().
			Background(styles.ColorBorder).
			Foreground(styles.ColorMuted).
			Padding(0, 6).
			Width(28).
			Align(lipgloss.Center).
			Render(buttonText)
	} else if i.focusIndex == 3 {
		generateBtn = lipgloss.NewStyle().
			Background(styles.ColorPrimary).
			Foreground(styles.ColorSecondary).
			Bold(true).
			Padding(0, 6).
			Width(28).
			Align(lipgloss.Center).
			Render(buttonText)
	} else {
		generateBtn = lipgloss.NewStyle().
			Background(styles.ColorBorder).
			Foreground(styles.ColorText).
			Padding(0, 6).
			Width(28).
			Align(lipgloss.Center).
			Render(buttonText)
	}
	generateBtn = lipgloss.NewStyle().Width(contentWidth).Align(lipgloss.Center).Render(generateBtn)

	// Result section with full address display
	var resultSection string
	if i.generated {
		resultTitle := styles.TitleStyle.Render("⎘ PAYMENT REQUEST ADDRESS")

		// Clean and truncate address to fit in one line
		displayAddr := strings.ReplaceAll(i.result, "\n", "")
		displayAddr = strings.ReplaceAll(displayAddr, "\r", "")
		// Truncate to the same visible content width as text inputs.
		maxTextWidth := styles.InputWidth - 4
		if utf8.RuneCountInString(displayAddr) > maxTextWidth {
			runes := []rune(displayAddr)
			prefixLen := (maxTextWidth - 3) / 2
			suffixLen := maxTextWidth - 3 - prefixLen
			displayAddr = string(runes[:prefixLen]) + "..." + string(runes[len(runes)-suffixLen:])
		}

		// Pad address to fill the full width
		currentLen := utf8.RuneCountInString(displayAddr)
		if currentLen < maxTextWidth {
			displayAddr = displayAddr + strings.Repeat(" ", maxTextWidth-currentLen)
		}

		// Create result box style matching FocusedInputStyle but with left alignment
		resultBox := styles.FocusedInputStyle.Copy().
			Align(lipgloss.Left).
			Render(displayAddr)

		var copyMsg string
		if i.copied {
			copyMsg = styles.SuccessStyle.Render("✓ Copied to clipboard!")
		}

		resultSection = resultTitle + "\n" + resultBox

		if copyMsg != "" {
			resultSection = resultSection + "\n" + lipgloss.NewStyle().Width(contentWidth).Align(lipgloss.Center).Render(copyMsg)
		}
	}

	// Hints - update dynamically based on state
	var hintsText string
	if i.generated {
		hintsText = "Tab Next • Enter Generate • C Copy • Y QR Code • Esc Cancel"
	} else {
		hintsText = "Tab Next • Enter Generate • Esc Cancel"
	}
	hints := styles.MutedStyle.Render(hintsText)
	hints = lipgloss.NewStyle().Width(contentWidth).Align(lipgloss.Center).Render(hints)

	var errorView string
	if i.error != "" {
		errorView = styles.ErrorStyle.Render("✗ " + i.error)
		errorView = lipgloss.NewStyle().Width(contentWidth).Align(lipgloss.Center).Render(errorView)
	}

	// Build sections with tighter vertical rhythm
	sections := []string{
		title,
		subtitle,
		optsNote,
		"",
		portRow,
		portBox,
		messageRow,
		messageBox,
		amountRow,
		amountBox,
		"",
		generateBtn,
	}

	if resultSection != "" {
		sections = append(sections, "", "", resultSection)
	}

	if errorView != "" {
		sections = append(sections, errorView)
	}

	sections = append(sections, "", hints)

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	containerStyle := styles.ThemedBoxStyle()

	return containerStyle.
		Width(styles.Width).
		Align(lipgloss.Center).
		Render(content)
}

// Cancelled returns whether user cancelled
func (i IntegratedAddrModel) Cancelled() bool {
	return i.cancelled
}

// GeneratedAddress returns the generated integrated address
func (i IntegratedAddrModel) GeneratedAddress() string {
	return i.result
}

// WantViewQR returns true if user wants to view QR of generated address
func (i IntegratedAddrModel) WantViewQR() bool {
	return i.wantViewQR
}

// ResetActions resets action flags
func (i *IntegratedAddrModel) ResetActions() {
	i.wantViewQR = false
}

// Reset resets the model state
func (i *IntegratedAddrModel) Reset() {
	i.portInput.Reset()
	i.messageInput.Reset()
	i.amountInput.Reset()
	i.result = ""
	i.error = ""
	i.focusIndex = 0
	i.cancelled = false
	i.copied = false
	i.generated = false
	i.wantViewQR = false
	i.portStatus = FieldEmpty
	i.amountStatus = FieldEmpty
	i.portInput.Focus()
}

// HandleMouse handles mouse events
func (i IntegratedAddrModel) HandleMouse(msg tea.MouseClickMsg, windowWidth, windowHeight int) IntegratedAddrModel {
	if i.generated && msg.Button == tea.MouseLeft {
		// Use MediumBoxHeight (25) since Payment Request page is tall
		boxHeight := styles.MediumBoxHeight
		boxY := (windowHeight - boxHeight) / 2

		// Approximate result section position:
		// Title(1) + gap(1) + Port row(2) + Port input(2) + Message row(2) + Message input(2) +
		// Amount row(2) + Amount input(2) + gap(1) + Generate button(1) + gap(1) = ~16 rows before result
		// Result section starts around row 17, address box is ~4 rows tall
		resultStartY := 17
		resultEndY := resultStartY + 4

		relY := msg.Mouse().Y - boxY

		// Check if click is within result section area
		if relY >= resultStartY && relY <= resultEndY {
			if i.result != "" && !strings.HasPrefix(i.result, "Error:") {
				if err := clipboard.WriteAll(i.result); err == nil {
					i.copied = true
				}
			}
		}
	}
	return i
}
