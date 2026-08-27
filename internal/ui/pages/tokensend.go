// Copyright 2017-2026 DERO Project. All rights reserved.

package pages

import (
	"strings"
	"time"
	"unicode/utf8"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/atotto/clipboard"
	"github.com/deroproject/dero-wallet-cli/internal/ui/components"
	"github.com/deroproject/dero-wallet-cli/internal/ui/styles"
	"github.com/deroproject/dero-wallet-cli/internal/wallet"
	"github.com/deroproject/derohe/globals"
	"github.com/deroproject/derohe/rpc"
)

// TokenSendModel is the token transfer form.
type TokenSendModel struct {
	scid        string
	ticker      string
	decimals    uint64
	balance     uint64
	deroBalance uint64

	addressInput    components.InputModel
	amountInput     components.InputModel
	ringsizeIndex   int
	focusIndex      int
	confirmed       bool
	cancelled       bool
	err             string
	processing      bool
	processingStart time.Time
	resultReady     bool
	resultTxID      string
	addressStatus   FieldStatus
	amountStatus    FieldStatus
	simulator       bool
	isIntegrated    bool
}

// NewTokenSend creates a token send form.
func NewTokenSend() TokenSendModel {
	m := TokenSendModel{
		addressInput:  components.NewInput("", "Recipient address or username", false),
		amountInput:   components.NewInput("", "0.00000", false),
		ringsizeIndex: 3,
	}
	m.addressInput.SetCharLimit(MaxAddressLength)
	m.amountInput.SetCharFilter(isNumericChar)
	m.addressInput.Focus()
	return m
}

func (s TokenSendModel) Init() tea.Cmd { return s.addressInput.Init() }

// SetToken sets the token being sent.
func (s *TokenSendModel) SetToken(scid, ticker string, decimals, balance, deroBalance uint64) {
	s.scid = scid
	s.ticker = ticker
	s.decimals = decimals
	s.balance = balance
	s.deroBalance = deroBalance
}

// SetSimulator sets simulator flag.
func (s *TokenSendModel) SetSimulator(v bool) { s.simulator = v; s.ringsizeIndex = 3 }

// SetBalance updates balances.
func (s *TokenSendModel) SetBalance(balance, deroBalance uint64) {
	s.balance = balance
	s.deroBalance = deroBalance
}

func (s *TokenSendModel) StartProcessing() {
	s.processing = true
	s.confirmed = false
	s.processingStart = time.Now()
	s.resultReady = false
	s.resultTxID = ""
	s.blurAllTokenSend()
}

func (s *TokenSendModel) SetSuccess(txID string) { s.resultReady = true; s.resultTxID = txID }

func (s TokenSendModel) IsMinimumDurationElapsed() bool {
	if !s.processing {
		return true
	}
	return time.Since(s.processingStart) >= MinimumVisibleDuration
}

func (s TokenSendModel) ShouldComplete() bool { return s.resultReady && s.IsMinimumDurationElapsed() }
func (s TokenSendModel) IsProcessing() bool   { return s.processing }
func (s TokenSendModel) ProcessingMinDurationCmd() tea.Cmd {
	return tea.Tick(MinimumVisibleDuration, func(t time.Time) tea.Msg { return processingMinDurationMsg(t) })
}

func (s TokenSendModel) Confirmed() bool    { return s.confirmed }
func (s TokenSendModel) Cancelled() bool    { return s.cancelled }
func (s TokenSendModel) GetSCID() string    { return s.scid }
func (s TokenSendModel) GetAddress() string { return strings.TrimSpace(s.addressInput.Value()) }
func (s TokenSendModel) GetAmount() uint64 {
	v := strings.TrimSpace(s.amountInput.Value())
	amt, _ := wallet.ParseTokenAmount(v, s.decimals)
	return amt
}
func (s TokenSendModel) GetRingsize() uint64 { return RingsizeOptions[s.ringsizeIndex].Value }
func (s *TokenSendModel) SetError(err string) {
	s.err = err
	s.confirmed = false
	s.processing = false
	s.resultReady = false
}
func (s *TokenSendModel) Reset() {
	s.addressInput.Reset()
	s.amountInput.Reset()
	s.ringsizeIndex = 3
	s.focusIndex = 0
	s.confirmed = false
	s.cancelled = false
	s.err = ""
	s.processing = false
	s.resultReady = false
	s.resultTxID = ""
	s.addressStatus = FieldEmpty
	s.amountStatus = FieldEmpty
	s.isIntegrated = false
	s.addressInput.Focus()
}

func (s *TokenSendModel) nextFocusTokenSend() {
	s.blurAllTokenSend()
	s.focusIndex = (s.focusIndex + 1) % 4
}
func (s *TokenSendModel) prevFocusTokenSend() {
	s.blurAllTokenSend()
	s.focusIndex = (s.focusIndex - 1 + 4) % 4
}
func (s *TokenSendModel) blurAllTokenSend() {
	s.addressInput.Blur()
	s.amountInput.Blur()
}
func (s *TokenSendModel) nextRingsizeTokenSend() {
	s.ringsizeIndex = (s.ringsizeIndex + 1) % len(RingsizeOptions)
}
func (s *TokenSendModel) prevRingsizeTokenSend() {
	s.ringsizeIndex = (s.ringsizeIndex - 1 + len(RingsizeOptions)) % len(RingsizeOptions)
}
func (s *TokenSendModel) focusCmdTokenSend() tea.Cmd {
	switch s.focusIndex {
	case 0:
		return s.addressInput.Focus()
	case 1:
		return s.amountInput.Focus()
	}
	return nil
}

func (s *TokenSendModel) validateAddressTokenSend() {
	addrStr := strings.TrimSpace(s.addressInput.Value())
	if addrStr == "" {
		s.addressStatus = FieldEmpty
		s.isIntegrated = false
		return
	}
	addr, err := globals.ParseValidateAddress(addrStr)
	if err != nil {
		if wallet.IsValidUsernameCandidate(addrStr) {
			s.addressStatus = FieldValid
			s.isIntegrated = false
			return
		}
		s.addressStatus = FieldInvalid
		s.isIntegrated = false
		return
	}
	s.isIntegrated = addr.IsIntegratedAddress()
	if s.isIntegrated {
		// Validate integrated args
		if err := addr.Arguments.Validate_Arguments(); err != nil {
			s.addressStatus = FieldInvalid
			return
		}
		// Check for RPC_VALUE_TRANSFER etc. is okay.
		_ = addr.Arguments.Has(rpc.RPC_DESTINATION_PORT, rpc.DataUint64)
	}
	s.addressStatus = FieldValid
}

func (s *TokenSendModel) validateAmountTokenSend() {
	v := strings.TrimSpace(s.amountInput.Value())
	if v == "" {
		s.amountStatus = FieldEmpty
		return
	}
	amt, err := wallet.ParseTokenAmount(v, s.decimals)
	if err != nil || amt == 0 || amt > s.balance {
		s.amountStatus = FieldInvalid
		return
	}
	s.amountStatus = FieldValid
}

func (s TokenSendModel) validateTokenSend() string {
	addr := strings.TrimSpace(s.addressInput.Value())
	if addr == "" {
		return "Recipient address is required"
	}
	if _, err := globals.ParseValidateAddress(addr); err != nil && !wallet.IsValidUsernameCandidate(addr) {
		return "Invalid DERO address or username"
	}
	v := strings.TrimSpace(s.amountInput.Value())
	if v == "" {
		return "Amount is required"
	}
	amt, err := wallet.ParseTokenAmount(v, s.decimals)
	if err != nil || amt == 0 {
		return "Invalid amount"
	}
	if amt > s.balance {
		return "Insufficient token balance"
	}
	if s.deroBalance < 2000 {
		return "Insufficient DERO for fee"
	}
	return ""
}

func (s *TokenSendModel) enforceAmountLimitTokenSend(prev string) {
	v := strings.TrimSpace(s.amountInput.Value())
	if v == "" {
		return
	}
	parts := strings.Split(v, ".")
	if len(parts) == 2 && uint64(len(parts[1])) > s.decimals {
		s.amountInput.SetValue(prev)
		return
	}
	amt, err := wallet.ParseTokenAmount(v, s.decimals)
	if err != nil {
		s.amountInput.SetValue(prev)
		return
	}
	if amt > s.balance {
		s.amountInput.SetValue(prev)
	}
}

func (s TokenSendModel) Update(msg tea.Msg) (TokenSendModel, tea.Cmd) {
	var cmd tea.Cmd
	if s.processing {
		switch msg.(type) {
		case processingMinDurationMsg:
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
			s.validateAddressTokenSend()
			s.validateAmountTokenSend()
			s.nextFocusTokenSend()
			return s, s.focusCmdTokenSend()
		case key.Matches(msg, pagePrevFieldKeys):
			s.validateAddressTokenSend()
			s.validateAmountTokenSend()
			s.prevFocusTokenSend()
			return s, s.focusCmdTokenSend()
		case key.Matches(msg, key.NewBinding(key.WithKeys("left"))):
			if s.focusIndex == 2 {
				s.prevRingsizeTokenSend()
				return s, nil
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("right"))):
			if s.focusIndex == 2 {
				s.nextRingsizeTokenSend()
				return s, nil
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+v"))):
			if s.focusIndex == 0 {
				if text, err := clipboard.ReadAll(); err == nil && text != "" {
					s.addressInput.SetValue(strings.TrimSpace(text))
					s.validateAddressTokenSend()
				}
				return s, nil
			}
		case key.Matches(msg, pageEnterKeys):
			if err := s.validateTokenSend(); err == "" {
				s.confirmed = true
				return s, nil
			} else if s.focusIndex == 3 {
				s.err = err
				return s, nil
			}
			s.nextFocusTokenSend()
			return s, s.focusCmdTokenSend()
		}
	}
	switch s.focusIndex {
	case 0:
		s.addressInput, cmd = s.addressInput.Update(msg)
	case 1:
		prev := s.amountInput.Value()
		s.amountInput, cmd = s.amountInput.Update(msg)
		s.enforceAmountLimitTokenSend(prev)
	}
	s.validateAddressTokenSend()
	s.validateAmountTokenSend()
	s.err = ""
	return s, cmd
}

func (s TokenSendModel) View() string {
	containerWidth := styles.InputWidth + 4
	contentWidth := containerWidth - 4

	buildHeaderRow := func(leftPlain, rightPlain string, leftStyled, rightStyled string) string {
		gap := contentWidth - utf8.RuneCountInString(leftPlain) - utf8.RuneCountInString(rightPlain)
		if gap < 1 {
			gap = 1
		}
		return leftStyled + strings.Repeat(" ", gap) + rightStyled
	}

	// Token header
	tokenLabel := "Token: " + safeSCIDLabel(s.scid)
	if s.ticker != "" {
		tokenLabel = "Token: " + s.ticker + " (" + safeSCIDLabel(s.scid) + ")"
	}
	tokenHeader := styles.TitleStyle.Render(tokenLabel)
	tokenBalPlain := "Balance: " + wallet.FormatTokenAmount(s.balance, s.decimals)
	tokenBal := styles.BalanceStyle.Render(tokenBalPlain)
	headerRow := buildHeaderRow(tokenLabel, tokenBalPlain, tokenHeader, tokenBal)

	// Address
	addressLabelPlain := "⎘ RECIPIENT"
	addressLabel := styles.TitleStyle.Render(addressLabelPlain)
	addrStatusPlain, addrStatusStyled := "", ""
	switch s.addressStatus {
	case FieldValid:
		addrStatusPlain = " ✓"
		addrStatusStyled = lipgloss.NewStyle().Foreground(styles.ColorSuccess).Render(" ✓")
	case FieldInvalid:
		addrStatusPlain = " ✗"
		addrStatusStyled = styles.ErrorStyle.Render(" ✗")
	}
	addressRow := buildHeaderRow(addressLabelPlain+addrStatusPlain, "Ctrl+V Paste", addressLabel+addrStatusStyled, styles.MutedStyle.Render("Ctrl+V Paste"))
	addressBox := s.addressInput.View()

	// Amount
	amountLabelPlain := styles.BalanceGlyph + " AMOUNT"
	amountLabel := styles.TitleStyle.Render(amountLabelPlain)
	amtStatusPlain, amtStatusStyled := "", ""
	switch s.amountStatus {
	case FieldValid:
		amtStatusPlain = " ✓"
		amtStatusStyled = lipgloss.NewStyle().Foreground(styles.ColorSuccess).Render(" ✓")
	case FieldInvalid:
		amtStatusPlain = " ✗"
		amtStatusStyled = styles.ErrorStyle.Render(" ✗")
	}
	balTextPlain := "Avail: " + wallet.FormatTokenAmount(s.balance, s.decimals)
	balText := styles.MutedStyle.Render("Avail: ") + styles.BalanceStyle.Render(wallet.FormatTokenAmount(s.balance, s.decimals))
	amountRow := buildHeaderRow(amountLabelPlain+amtStatusPlain, balTextPlain, amountLabel+amtStatusStyled, balText)
	amountBox := s.amountInput.View()

	// Ringsize
	ringsizeText := "◌ RING SIZE: " + RingsizeOptions[s.ringsizeIndex].Label
	var ringsizeDisplay string
	if s.focusIndex == 2 {
		ringsizeDisplay = lipgloss.NewStyle().Foreground(styles.ColorPrimary).Bold(true).Render("← " + ringsizeText + " →")
	} else {
		ringsizeDisplay = styles.MutedStyle.Render(ringsizeText)
	}
	ringsizeRow := lipgloss.NewStyle().Width(containerWidth - 4).Align(lipgloss.Center).Render(ringsizeDisplay)

	// Fee note
	feeNote := styles.MutedStyle.Render("Fee paid in DERO • DERO balance: " + formatAtomic(s.deroBalance))
	feeRow := lipgloss.NewStyle().Width(containerWidth - 4).Align(lipgloss.Center).Render(feeNote)

	buttonText := "Send Token"
	if s.processing {
		buttonText = "Sending..."
	}
	var sendButton string
	if s.processing {
		sendButton = lipgloss.NewStyle().Background(styles.ColorBorder).Foreground(styles.ColorMuted).Padding(0, 6).Width(28).Align(lipgloss.Center).Render(buttonText)
	} else if s.focusIndex == 3 {
		sendButton = lipgloss.NewStyle().Background(styles.ColorPrimary).Foreground(styles.ColorSecondary).Bold(true).Padding(0, 6).Width(28).Align(lipgloss.Center).Render(buttonText)
	} else {
		sendButton = lipgloss.NewStyle().Background(styles.ColorBorder).Foreground(styles.ColorText).Padding(0, 6).Width(28).Align(lipgloss.Center).Render(buttonText)
	}
	sendButton = lipgloss.NewStyle().Width(containerWidth - 4).Align(lipgloss.Center).Render(sendButton)

	hints := styles.MutedStyle.Render("Tab Next • ←/→ Ring • Enter Send • Esc Cancel")
	hints = lipgloss.NewStyle().Width(containerWidth - 4).Align(lipgloss.Center).Render(hints)

	var errorView string
	if s.err != "" {
		errorView = styles.ErrorStyle.Render("✗ " + s.err)
		errorView = lipgloss.NewStyle().Width(containerWidth - 4).Align(lipgloss.Center).Render(errorView)
	}

	sections := []string{
		headerRow,
		"",
		addressRow,
		addressBox,
		"",
		amountRow,
		amountBox,
		"",
		ringsizeRow,
		"",
		feeRow,
		"",
		sendButton,
		"",
	}
	if errorView != "" {
		sections = append(sections, errorView)
	}
	sections = append(sections, hints)
	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// HandleMouse for token send
func (s TokenSendModel) HandleMouse(msg tea.MouseClickMsg, windowWidth, windowHeight int) TokenSendModel {
	boxWidth := styles.Width
	boxHeight := styles.MediumBoxHeight
	boxX := (windowWidth - boxWidth) / 2
	boxY := (windowHeight - boxHeight) / 2
	relX := msg.Mouse().X - boxX - 4
	relY := msg.Mouse().Y - boxY - 3
	if s.processing {
		return s
	}
	switch msg.Button {
	case tea.MouseLeft:
		if relY >= 3 && relY <= 4 {
			s.blurAllTokenSend()
			s.focusIndex = 0
			s.addressInput.Focus()
			return s
		}
		if relY >= 6 && relY <= 7 {
			s.blurAllTokenSend()
			s.focusIndex = 1
			s.amountInput.Focus()
			return s
		}
		if relY == 8 {
			centerX := boxWidth / 2
			if relX < centerX-10 {
				s.prevRingsizeTokenSend()
			} else if relX > centerX+10 {
				s.nextRingsizeTokenSend()
			}
			s.focusIndex = 2
			return s
		}
		if relY == 11 {
			s.focusIndex = 3
			if err := s.validateTokenSend(); err != "" {
				s.err = err
				return s
			}
			s.confirmed = true
			return s
		}
	case tea.MouseWheelUp:
		if s.focusIndex > 0 {
			s.prevFocusTokenSend()
		}
	case tea.MouseWheelDown:
		if s.focusIndex < 3 {
			s.nextFocusTokenSend()
		}
	}
	return s
}
