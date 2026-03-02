// Copyright 2017-2026 DERO Project. All rights reserved.

package pages

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"
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

// isNumericChar returns true if the rune is a digit or decimal point
func isNumericChar(r rune) bool {
	return (r >= '0' && r <= '9') || r == '.'
}

func isValidUsernameCandidate(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" || strings.HasPrefix(name, "@") || len(name) > 64 {
		return false
	}

	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '_' || r == '-' {
			continue
		}
		return false
	}

	return true
}

// abs returns the absolute value of an integer
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// AtomicUnitsPerDERO is the number of atomic units in 1 DERO (5 decimal places)
const AtomicUnitsPerDERO = 100000

// MaxMessageBytes is the maximum bytes allowed in a transaction message
const MaxMessageBytes = 130

// RingsizeOptions represents the available anonymity set sizes
var RingsizeOptions = []struct {
	Value uint64
	Label string
}{
	{2, "2 (None)"},
	{4, "4 (Low)"},
	{8, "8 (Low)"},
	{16, "16 (Rec.)"},
	{32, "32 (Med.)"},
	{64, "64 (High)"},
	{128, "128 (High)"},
}

var (
	sendLeftKeys  = key.NewBinding(key.WithKeys("left"))
	sendRightKeys = key.NewBinding(key.WithKeys("right"))
	sendPasteKeys = key.NewBinding(key.WithKeys("ctrl+v"))
)

// FieldStatus represents the validation state of an input field
type FieldStatus int

const (
	FieldEmpty FieldStatus = iota
	FieldValid
	FieldInvalid
)

// MaxAddressLength is the maximum length for DERO addresses
// Standard: 66 chars, Integrated: up to 297 (mainnet) / 298 (testnet)
const MaxAddressLength = 300

// MinimumVisibleDuration is the minimum time to show the sending animation
const MinimumVisibleDuration = 600 * time.Millisecond

// SendModel represents the send page
type SendModel struct {
	addressInput      components.InputModel
	amountInput       components.InputModel
	messageInput      components.InputModel
	paymentIDInput    components.InputModel
	ringsizeIndex     int // Index into RingsizeOptions
	focusIndex        int
	confirmed         bool
	cancelled         bool
	error             string
	balance           uint64
	processing        bool        // true when transaction is being broadcast
	processingStart   time.Time   // when processing started
	resultReady       bool        // true when transfer result received
	resultTxID        string      // cached txid for after minimum duration
	addressStatus     FieldStatus // real-time address validation status
	amountStatus      FieldStatus // real-time amount validation status
	paymentIDStatus   FieldStatus // real-time payment ID validation status
	simulator         bool        // true if wallet is on simulator network
	isIntegrated      bool        // true if address has embedded payment ID
	integratedPort    uint64      // extracted destination port from integrated address
	integratedMsg     string      // extracted message from integrated address
	integratedAmount  uint64      // extracted amount from integrated address (RPC_VALUE_TRANSFER)
	lastParsedAddress string      // track last parsed address to detect changes
	msgManuallyEdited bool        // true if user manually edited the message
}

// processingMinDurationMsg fires once when the minimum send wait time has elapsed.
type processingMinDurationMsg time.Time

// NewSend creates a new send page
func NewSend() SendModel {
	m := SendModel{
		addressInput:   components.NewInput("", "Recipient address or username", false),
		amountInput:    components.NewInput("", "0.00000", false),
		messageInput:   components.NewInput("", "Message (optional)", false),
		paymentIDInput: components.NewInput("", "Destination Port / Payment ID (optional)", false),
		ringsizeIndex:  3, // Default to 16 (Recommended)
	}
	m.addressInput.SetCharLimit(MaxAddressLength)
	m.amountInput.SetCharFilter(isNumericChar)
	m.messageInput.SetCharLimit(MaxMessageBytes)
	m.paymentIDInput.SetCharLimit(20) // uint64 max is 20 digits
	m.paymentIDInput.SetCharFilter(func(r rune) bool {
		return r >= '0' && r <= '9'
	})
	m.addressInput.Focus()
	return m
}

// Init initializes the send page
func (s SendModel) Init() tea.Cmd {
	return s.addressInput.Init()
}

// StartProcessing begins the transaction broadcast
func (s *SendModel) StartProcessing() {
	s.processing = true
	s.confirmed = false // Clear confirmed to prevent re-triggering
	s.processingStart = time.Now()
	s.resultReady = false
	s.resultTxID = ""
	s.blurAll()
}

// SetSuccess marks result as ready, but processing continues until minimum duration
func (s *SendModel) SetSuccess(txID string) {
	s.resultReady = true
	s.resultTxID = txID
}

// IsMinimumDurationElapsed checks if the minimum visible animation time has passed
func (s SendModel) IsMinimumDurationElapsed() bool {
	if !s.processing {
		return true
	}
	return time.Since(s.processingStart) >= MinimumVisibleDuration
}

// ShouldComplete returns true when we can safely transition after minimum duration
func (s SendModel) ShouldComplete() bool {
	return s.resultReady && s.IsMinimumDurationElapsed()
}

// IsProcessing returns true if currently processing
func (s SendModel) IsProcessing() bool {
	return s.processing
}

// ProcessingMinDurationCmd returns a one-shot timer for minimum visible send state.
func (s SendModel) ProcessingMinDurationCmd() tea.Cmd {
	return tea.Tick(MinimumVisibleDuration, func(t time.Time) tea.Msg {
		return processingMinDurationMsg(t)
	})
}

// SetAddress pre-fills the recipient address
func (s *SendModel) SetAddress(addr string) {
	s.addressInput.SetValue(addr)
}

// SetMessage pre-fills the message
func (s *SendModel) SetMessage(msg string) {
	s.messageInput.SetValue(msg)
}

// Update handles events
func (s SendModel) Update(msg tea.Msg) (SendModel, tea.Cmd) {
	var cmd tea.Cmd

	// During processing, ignore interaction and wait for either
	// transfer result or minimum-duration timeout.
	if s.processing {
		switch msg.(type) {
		case processingMinDurationMsg:
			// Check if we should complete (result ready + min duration elapsed)
			if s.ShouldComplete() {
				s.processing = false
			}
		}
		return s, nil
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, pageEscKeys):
			s.cancelled = true
			return s, nil

		case key.Matches(msg, pageNextFieldKeys):
			s.validateAddress()
			s.validateAmount()
			s.validatePaymentID()
			s.nextFocus()
			return s, s.focusCmd()

		case key.Matches(msg, pagePrevFieldKeys):
			s.validateAddress()
			s.validateAmount()
			s.validatePaymentID()
			s.prevFocus()
			return s, s.focusCmd()

		case key.Matches(msg, sendLeftKeys):
			if s.focusIndex == 2 { // Ringsize selector
				s.prevRingsize()
				return s, nil
			}

		case key.Matches(msg, sendRightKeys):
			if s.focusIndex == 2 { // Ringsize selector
				s.nextRingsize()
				return s, nil
			}

		case key.Matches(msg, sendPasteKeys):
			if s.focusIndex == 0 { // Address field - paste from clipboard
				if text, err := clipboard.ReadAll(); err == nil && text != "" {
					s.addressInput.SetValue(strings.TrimSpace(text))
					s.validateAddress()
				}
				return s, nil
			}

		case key.Matches(msg, pageEnterKeys):
			if err := s.validate(); err == "" {
				s.confirmed = true
				return s, nil
			} else if s.focusIndex == 5 {
				s.error = err
				return s, nil
			}
			// Move to next field
			s.nextFocus()
			return s, s.focusCmd()
		}
	}

	// Update focused input
	switch s.focusIndex {
	case 0:
		s.addressInput, cmd = s.addressInput.Update(msg)
	case 1:
		prevAmount := s.amountInput.Value()
		s.amountInput, cmd = s.amountInput.Update(msg)
		s.enforceAmountLimit(prevAmount)
	case 3:
		prevMsg := s.messageInput.Value()
		s.messageInput, cmd = s.messageInput.Update(msg)
		// Track if user manually edited the message
		if s.messageInput.Value() != prevMsg {
			s.msgManuallyEdited = true
		}
	case 4:
		// Skip updating payment ID if integrated address (it's locked)
		if !s.isIntegrated {
			s.paymentIDInput, cmd = s.paymentIDInput.Update(msg)
		}
	}

	// Always validate to show real-time feedback
	s.validateAddress()
	s.validateAmount()
	s.validatePaymentID()
	s.error = "" // Clear error on typing

	return s, cmd
}

// View renders the send page
func (s SendModel) View() string {
	containerWidth := styles.InputWidth + 4
	contentWidth := containerWidth - 4

	buildHeaderRow := func(leftPlain, rightPlain string, leftStyled, rightStyled string) string {
		gap := contentWidth - utf8.RuneCountInString(leftPlain) - utf8.RuneCountInString(rightPlain)
		if gap < 1 {
			gap = 1
		}
		return leftStyled + strings.Repeat(" ", gap) + rightStyled
	}

	// Address section with validation indicator and paste button
	addressLabelPlain := "⎘ RECIPIENT"
	addressLabel := styles.TitleStyle.Render(addressLabelPlain)
	addressStatusPlain := ""
	var addressStatusStyled string
	switch s.addressStatus {
	case FieldValid:
		addressStatusPlain = " ✓"
		addressStatusStyled = lipgloss.NewStyle().Foreground(styles.ColorSuccess).Render(" ✓")
	case FieldInvalid:
		addressStatusPlain = " ✗"
		addressStatusStyled = styles.ErrorStyle.Render(" ✗")
	}
	addressLabelWithStatusPlain := addressLabelPlain + addressStatusPlain
	addressLabelWithStatus := addressLabel + addressStatusStyled
	addressLen := utf8.RuneCountInString(s.addressInput.Value())
	addressCounter := strconv.Itoa(addressLen) + "/" + strconv.Itoa(MaxAddressLength)
	var addressCounterStyled string
	if addressLen >= (MaxAddressLength*9)/10 {
		addressCounterStyled = lipgloss.NewStyle().Foreground(styles.ColorWarning).Render(addressCounter)
	} else {
		addressCounterStyled = styles.MutedStyle.Render(addressCounter)
	}
	// Show paste hint when address field is focused
	pasteHintPlain := "Ctrl+V Paste"
	var pasteHint string
	if s.focusIndex == 0 {
		pasteHint = lipgloss.NewStyle().Foreground(styles.ColorPrimary).Render(pasteHintPlain)
	} else {
		pasteHint = styles.MutedStyle.Render(pasteHintPlain)
	}
	rightInfoPlain := pasteHintPlain + "  " + addressCounter
	rightInfo := pasteHint + "  " + addressCounterStyled
	addressRow := buildHeaderRow(addressLabelWithStatusPlain, rightInfoPlain, addressLabelWithStatus, rightInfo)
	addressBox := s.addressInput.View()

	// Amount row with balance on right
	amountLabelPlain := styles.BalanceGlyph + " AMOUNT"
	amountLabel := styles.TitleStyle.Render(amountLabelPlain)
	amountStatusPlain := ""
	var amountStatusStyled string
	switch s.amountStatus {
	case FieldValid:
		amountStatusPlain = " ✓"
		amountStatusStyled = lipgloss.NewStyle().Foreground(styles.ColorSuccess).Render(" ✓")
	case FieldInvalid:
		amountStatusPlain = " ✗"
		amountStatusStyled = styles.ErrorStyle.Render(" ✗")
	}
	balanceText := styles.MutedStyle.Render("Avail: ") + styles.BalanceStyle.Render(styles.BalanceGlyph+" "+formatAtomic(s.balance))
	balanceTextPlain := "Avail: " + styles.BalanceGlyph + " " + formatAtomic(s.balance)
	amountLabelWithStatusPlain := amountLabelPlain + amountStatusPlain
	amountLabelWithStatus := amountLabel + amountStatusStyled
	amountRow := buildHeaderRow(amountLabelWithStatusPlain, balanceTextPlain, amountLabelWithStatus, balanceText)
	amountBox := s.amountInput.View()

	// Ring size row
	ringsizeText := "◌ RING SIZE: " + RingsizeOptions[s.ringsizeIndex].Label
	var ringsizeDisplay string
	if s.focusIndex == 2 {
		ringsizeDisplay = lipgloss.NewStyle().
			Foreground(styles.ColorPrimary).
			Bold(true).
			Render("← " + ringsizeText + " →")
	} else {
		ringsizeDisplay = styles.MutedStyle.Render(ringsizeText)
	}
	ringsizeRow := lipgloss.NewStyle().Width(containerWidth - 4).Align(lipgloss.Center).Render(ringsizeDisplay)

	// Message section with inline counter
	messageBytes := len(s.messageInput.Value())
	byteCounter := strconv.Itoa(messageBytes) + "/" + strconv.Itoa(MaxMessageBytes)
	var counterStyled string
	if messageBytes > MaxMessageBytes {
		counterStyled = styles.ErrorStyle.Render(byteCounter)
	} else {
		counterStyled = styles.MutedStyle.Render(byteCounter)
	}
	messageLabelPlain := "✉ MESSAGE"
	messageLabel := styles.TitleStyle.Render(messageLabelPlain)
	messageRow := buildHeaderRow(messageLabelPlain, byteCounter, messageLabel, counterStyled)
	messageBox := s.messageInput.View()

	// Destination Port / Payment ID section with validation indicator
	paymentLabelPlain := "◈ DESTINATION PORT / PAYMENT ID"
	paymentLabel := styles.TitleStyle.Render(paymentLabelPlain)
	paymentLen := utf8.RuneCountInString(s.paymentIDInput.Value())
	paymentCounter := strconv.Itoa(paymentLen) + "/20"
	var paymentCounterStyled string
	if paymentLen >= 18 {
		paymentCounterStyled = lipgloss.NewStyle().Foreground(styles.ColorWarning).Render(paymentCounter)
	} else {
		paymentCounterStyled = styles.MutedStyle.Render(paymentCounter)
	}
	paymentStatusPlain := ""
	var pidStatusStyled string
	switch s.paymentIDStatus {
	case FieldValid:
		// Only show "(from address)" if there's actually an extracted port
		if s.isIntegrated && s.integratedPort != 0 {
			paymentStatusPlain = " ✓ (from address)"
			pidStatusStyled = lipgloss.NewStyle().Foreground(styles.ColorSuccess).Render(" ✓ (from address)")
		} else {
			paymentStatusPlain = " ✓"
			pidStatusStyled = lipgloss.NewStyle().Foreground(styles.ColorSuccess).Render(" ✓")
		}
	case FieldInvalid:
		paymentStatusPlain = " ✗"
		pidStatusStyled = styles.ErrorStyle.Render(" ✗")
	}
	paymentLabelWithStatusPlain := paymentLabelPlain + paymentStatusPlain
	paymentLabelWithStatus := paymentLabel + pidStatusStyled
	paymentRow := buildHeaderRow(paymentLabelWithStatusPlain, paymentCounter, paymentLabelWithStatus, paymentCounterStyled)
	paymentBox := s.paymentIDInput.View()

	// Send button - dimmed when processing
	buttonText := "Send"
	if s.processing {
		buttonText = "Sending..."
	}

	var sendButton string
	if s.processing {
		// Dimmed button during processing
		sendButton = lipgloss.NewStyle().
			Background(styles.ColorBorder).
			Foreground(styles.ColorMuted).
			Padding(0, 6).
			Width(28).
			Align(lipgloss.Center).
			Render(buttonText)
	} else if s.focusIndex == 5 {
		sendButton = lipgloss.NewStyle().
			Background(styles.ColorPrimary).
			Foreground(styles.ColorSecondary).
			Bold(true).
			Padding(0, 6).
			Width(28).
			Align(lipgloss.Center).
			Render(buttonText)
	} else {
		sendButton = lipgloss.NewStyle().
			Background(styles.ColorBorder).
			Foreground(styles.ColorText).
			Padding(0, 6).
			Width(28).
			Align(lipgloss.Center).
			Render(buttonText)
	}
	sendButton = lipgloss.NewStyle().Width(containerWidth - 4).Align(lipgloss.Center).Render(sendButton)

	// Hints
	hints := styles.MutedStyle.Render("Ctrl+V Paste • Tab Next • ←/→ Ring • Enter Send • Esc Cancel")
	hints = lipgloss.NewStyle().Width(containerWidth - 4).Align(lipgloss.Center).Render(hints)

	// Error
	var errorView string
	if s.error != "" {
		errorView = styles.ErrorStyle.Render("✗ " + s.error)
		errorView = lipgloss.NewStyle().Width(containerWidth - 4).Align(lipgloss.Center).Render(errorView)
	}

	// Build sections
	sections := []string{
		addressRow,
		addressBox,
		"",
		amountRow,
		amountBox,
		"",
		ringsizeRow,
		"",
		messageRow,
		messageBox,
		"",
		paymentRow,
		paymentBox,
		"",
	}

	// Add send button (dimmed text when processing)
	sections = append(sections, sendButton)
	sections = append(sections, "")

	if errorView != "" {
		sections = append(sections, errorView)
	}

	sections = append(sections, hints)

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	// Return content without outer container to avoid border conflicts
	return content
}

func (s *SendModel) nextFocus() {
	s.blurAll()
	s.focusIndex = (s.focusIndex + 1) % 6
}

func (s *SendModel) prevFocus() {
	s.blurAll()
	s.focusIndex = (s.focusIndex - 1 + 6) % 6
}

func (s *SendModel) blurAll() {
	s.addressInput.Blur()
	s.amountInput.Blur()
	s.messageInput.Blur()
	s.paymentIDInput.Blur()
}

func (s *SendModel) nextRingsize() {
	s.ringsizeIndex = (s.ringsizeIndex + 1) % len(RingsizeOptions)
}

func (s *SendModel) prevRingsize() {
	s.ringsizeIndex = (s.ringsizeIndex - 1 + len(RingsizeOptions)) % len(RingsizeOptions)
}

func (s *SendModel) focusCmd() tea.Cmd {
	switch s.focusIndex {
	case 0:
		return s.addressInput.Focus()
	case 1:
		return s.amountInput.Focus()
	case 3:
		return s.messageInput.Focus()
	case 4:
		return s.paymentIDInput.Focus()
	}
	return nil
}

// validateAddress performs real-time validation of the recipient address
func (s *SendModel) validateAddress() {
	addrStr := strings.TrimSpace(s.addressInput.Value())
	if addrStr == "" {
		s.addressStatus = FieldEmpty
		s.isIntegrated = false
		s.integratedPort = 0
		s.integratedMsg = ""
		s.lastParsedAddress = ""
		return
	}

	addr, err := globals.ParseValidateAddress(addrStr)
	if err != nil {
		if isValidUsernameCandidate(addrStr) {
			s.addressStatus = FieldValid
			s.isIntegrated = false
			s.integratedPort = 0
			s.integratedMsg = ""
			s.integratedAmount = 0
			s.lastParsedAddress = ""
			s.paymentIDInput.Disabled = false
			return
		}

		s.addressStatus = FieldInvalid
		s.isIntegrated = false
		s.integratedPort = 0
		s.integratedMsg = ""
		s.integratedAmount = 0
		s.lastParsedAddress = ""
		s.paymentIDInput.Disabled = false
		return
	}

	// Check if this is a new address (changed since last validation)
	isNewAddress := addrStr != s.lastParsedAddress
	if isNewAddress {
		s.lastParsedAddress = addrStr
		s.msgManuallyEdited = false // Reset manual edit flag for new address
	}

	// Check for integrated address and extract embedded data
	if addr.IsIntegratedAddress() {
		s.isIntegrated = true
		// Lock the payment ID field for integrated addresses
		s.paymentIDInput.Disabled = true

		// Extract destination port (payment ID)
		if addr.Arguments.Has(rpc.RPC_DESTINATION_PORT, rpc.DataUint64) {
			s.integratedPort = addr.Arguments.Value(rpc.RPC_DESTINATION_PORT, rpc.DataUint64).(uint64)
			// Auto-fill the Payment ID field
			s.paymentIDInput.SetValue(strconv.FormatUint(s.integratedPort, 10))
			s.paymentIDStatus = FieldValid
		}

		// Extract embedded message/comment
		if addr.Arguments.Has(rpc.RPC_COMMENT, rpc.DataString) {
			s.integratedMsg = addr.Arguments.Value(rpc.RPC_COMMENT, rpc.DataString).(string)
			// Auto-fill message field only on address change and if user hasn't manually edited
			if isNewAddress && !s.msgManuallyEdited && s.messageInput.Value() == "" {
				s.messageInput.SetValue(s.integratedMsg)
			}
		}

		// Extract embedded amount (RPC_VALUE_TRANSFER)
		if addr.Arguments.Has(rpc.RPC_VALUE_TRANSFER, rpc.DataUint64) {
			s.integratedAmount = addr.Arguments.Value(rpc.RPC_VALUE_TRANSFER, rpc.DataUint64).(uint64)
			// Auto-fill amount field if it's currently empty
			if s.amountInput.Value() == "" {
				// Convert atomic amount to DERO decimal format
				whole := s.integratedAmount / AtomicUnitsPerDERO
				frac := s.integratedAmount % AtomicUnitsPerDERO
				amountStr := fmt.Sprintf("%d.%05d", whole, frac)
				// Trim trailing zeros for cleaner display
				amountStr = strings.TrimRight(amountStr, "0")
				if strings.HasSuffix(amountStr, ".") {
					amountStr += "0"
				}
				s.amountInput.SetValue(amountStr)
				s.amountStatus = FieldValid
			}
		}
	} else {
		// Save the current integrated port before clearing it
		prevIntegratedPort := s.integratedPort
		s.isIntegrated = false
		s.integratedPort = 0
		s.integratedMsg = ""
		s.integratedAmount = 0
		// Unlock the payment ID field for non-integrated addresses
		s.paymentIDInput.Disabled = false
		// Clear auto-filled payment ID when address is no longer integrated
		// but only if it matches the previously integrated port
		if s.paymentIDStatus == FieldValid {
			if pid, err := strconv.ParseUint(s.paymentIDInput.Value(), 10, 64); err == nil && pid == prevIntegratedPort && prevIntegratedPort != 0 {
				s.paymentIDInput.SetValue("")
				s.paymentIDStatus = FieldEmpty
			}
		}
	}

	s.addressStatus = FieldValid
}

func (s *SendModel) validatePaymentID() {
	// Skip validation if integrated address is being used
	if s.isIntegrated {
		s.paymentIDStatus = FieldValid
		return
	}

	pid := strings.TrimSpace(s.paymentIDInput.Value())
	if pid == "" {
		s.paymentIDStatus = FieldEmpty
		return
	}
	// Validate as uint64 (0-18446744073709551615)
	if _, err := strconv.ParseUint(pid, 10, 64); err != nil {
		s.paymentIDStatus = FieldInvalid
		return
	}
	s.paymentIDStatus = FieldValid
}

func (s *SendModel) validateAmount() {
	amountStr := strings.TrimSpace(s.amountInput.Value())
	if amountStr == "" {
		s.amountStatus = FieldEmpty
		return
	}
	atomic, err := parseAmountToAtomic(amountStr)
	if err != nil || atomic == 0 {
		s.amountStatus = FieldInvalid
		return
	}
	if atomic > s.balance {
		s.amountStatus = FieldInvalid
		return
	}
	s.amountStatus = FieldValid
}

func (s SendModel) validate() string {
	addr := strings.TrimSpace(s.addressInput.Value())
	if addr == "" {
		return "Recipient address is required"
	}
	if _, err := globals.ParseValidateAddress(addr); err != nil && !isValidUsernameCandidate(addr) {
		return "Invalid DERO address or username"
	}

	amountStr := strings.TrimSpace(s.amountInput.Value())
	if amountStr == "" {
		return "Amount is required"
	}
	atomic, err := parseAmountToAtomic(amountStr)
	if err != nil || atomic == 0 {
		return "Invalid amount"
	}

	if atomic > s.balance {
		return "Insufficient balance"
	}

	message := strings.TrimSpace(s.messageInput.Value())
	if len(message) > MaxMessageBytes {
		return "Message exceeds 130 byte limit"
	}

	paymentID := strings.TrimSpace(s.paymentIDInput.Value())
	if paymentID != "" {
		if _, err := strconv.ParseUint(paymentID, 10, 64); err != nil {
			return "Destination Port / Payment ID must be a valid uint64 number"
		}
	}

	return ""
}

func parseAmountToAtomic(amountStr string) (uint64, error) {
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
	if whole > (maxUint64-frac)/AtomicUnitsPerDERO {
		return 0, strconv.ErrRange
	}

	return whole*AtomicUnitsPerDERO + frac, nil
}

func (s *SendModel) enforceAmountLimit(previous string) {
	amountStr := strings.TrimSpace(s.amountInput.Value())
	if amountStr == "" {
		return
	}

	// Check decimal places - DERO uses 5 decimal places max
	parts := strings.Split(amountStr, ".")
	if len(parts) == 2 && len(parts[1]) > 5 {
		s.amountInput.SetValue(previous)
		return
	}

	atomic, err := parseAmountToAtomic(amountStr)
	if err != nil {
		s.amountInput.SetValue(previous)
		return
	}

	if atomic > s.balance {
		s.amountInput.SetValue(previous)
	}
}

func (s SendModel) Confirmed() bool {
	return s.confirmed
}

func (s SendModel) Cancelled() bool {
	return s.cancelled
}

func (s SendModel) GetAddress() string {
	return strings.TrimSpace(s.addressInput.Value())
}

func (s SendModel) GetAmount() uint64 {
	amountStr := strings.TrimSpace(s.amountInput.Value())
	atomic, _ := parseAmountToAtomic(amountStr)
	return atomic
}

func (s SendModel) GetPaymentID() uint64 {
	pidStr := strings.TrimSpace(s.paymentIDInput.Value())
	if pidStr == "" {
		return 0
	}
	pid, err := strconv.ParseUint(pidStr, 10, 64)
	if err != nil {
		return 0
	}
	return pid
}

func (s SendModel) GetRingsize() uint64 {
	return RingsizeOptions[s.ringsizeIndex].Value
}

func (s SendModel) GetMessage() string {
	return strings.TrimSpace(s.messageInput.Value())
}

func (s *SendModel) SetBalance(balance uint64) {
	s.balance = balance
}

func (s *SendModel) SetSimulator(simulator bool) {
	s.simulator = simulator
	// Ringsize 16 is the default for all networks
	s.ringsizeIndex = 3 // Index 3 = ringsize 16 (Recommended)
}

func (s *SendModel) SetError(err string) {
	s.error = err
	s.confirmed = false
	s.processing = false  // Stop animation so user can see the error
	s.resultReady = false // Clear any stale success state
}

func (s *SendModel) Reset() {
	s.addressInput.Reset()
	s.amountInput.Reset()
	s.messageInput.Reset()
	s.paymentIDInput.Reset()
	// Ringsize 16 is the default for all networks
	s.ringsizeIndex = 3 // Index 3 = ringsize 16 (Recommended)
	s.focusIndex = 0
	s.confirmed = false
	s.cancelled = false
	s.error = ""
	s.processing = false
	s.resultReady = false
	s.resultTxID = ""
	s.addressStatus = FieldEmpty
	s.amountStatus = FieldEmpty
	s.paymentIDStatus = FieldEmpty
	s.isIntegrated = false
	s.integratedPort = 0
	s.integratedMsg = ""
	s.integratedAmount = 0
	s.lastParsedAddress = ""
	s.msgManuallyEdited = false
	s.addressInput.Focus()
}

// HandleMouse handles mouse events on the send page
func (s SendModel) HandleMouse(msg tea.MouseClickMsg, windowWidth, windowHeight int) SendModel {
	// Calculate centered position
	boxWidth := styles.Width
	boxHeight := styles.MediumBoxHeight
	boxX := (windowWidth - boxWidth) / 2
	boxY := (windowHeight - boxHeight) / 2

	// Adjust to content area
	relX := msg.Mouse().X - boxX - 4
	relY := msg.Mouse().Y - boxY - 3 // After title

	// Block interaction during processing
	if s.processing {
		return s
	}

	switch msg.Button {
	case tea.MouseLeft:
		// Check each input field area
		// Address input: y=2-3
		if relY >= 2 && relY <= 3 {
			s.blurAll()
			s.focusIndex = 0
			s.addressInput.Focus()
			return s
		}

		// Amount input: y=5-6
		if relY >= 5 && relY <= 6 {
			s.blurAll()
			s.focusIndex = 1
			s.amountInput.Focus()
			return s
		}

		// Ringsize selector: y=7
		if relY == 7 {
			// Check if clicking on left/right arrows or center
			centerX := boxWidth / 2
			if relX < centerX-10 {
				s.prevRingsize()
			} else if relX > centerX+10 {
				s.nextRingsize()
			}
			s.focusIndex = 2
			return s
		}

		// Message input: y=9-10
		if relY >= 9 && relY <= 10 {
			s.blurAll()
			s.focusIndex = 3
			s.messageInput.Focus()
			return s
		}

		// Payment ID input: y=12-13
		if relY >= 12 && relY <= 13 {
			s.blurAll()
			s.focusIndex = 4
			s.paymentIDInput.Focus()
			return s
		}

		// Send button: y=15
		if relY == 15 {
			s.focusIndex = 5
			if err := s.validate(); err != "" {
				s.error = err
				return s
			}
			s.confirmed = true
			return s
		}

	case tea.MouseWheelUp:
		if s.focusIndex > 0 {
			s.validateAddress()
			s.validateAmount()
			s.validatePaymentID()
			s.prevFocus()
		}

	case tea.MouseWheelDown:
		if s.focusIndex < 5 {
			s.validateAddress()
			s.validateAmount()
			s.validatePaymentID()
			s.nextFocus()
		}
	}

	return s
}
