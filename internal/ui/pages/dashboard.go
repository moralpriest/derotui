// Copyright 2017-2026 DERO Project. All rights reserved.

package pages

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/paginator"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/deroproject/dero-wallet-cli/internal/ui/styles"
)

var (
	dashboardSendKeys           = key.NewBinding(key.WithKeys("s"))
	dashboardCopyKeys           = key.NewBinding(key.WithKeys("c"))
	dashboardViewQRKeys         = key.NewBinding(key.WithKeys("y"))
	dashboardHistoryKeys        = key.NewBinding(key.WithKeys("h"))
	dashboardRegisterKeys       = key.NewBinding(key.WithKeys("g"))
	dashboardViewSeedKeys       = key.NewBinding(key.WithKeys("v"))
	dashboardViewKeyKeys        = key.NewBinding(key.WithKeys("k"))
	dashboardChangePasswordKeys = key.NewBinding(key.WithKeys("p"))
	dashboardXSWDKeys           = key.NewBinding(key.WithKeys("x"))
	dashboardIntegratedAddrKeys = key.NewBinding(key.WithKeys("r"))
	dashboardDonateKeys         = key.NewBinding(key.WithKeys("d"))
	dashboardPrevPageKeys       = key.NewBinding(key.WithKeys("left", "["))
	dashboardNextPageKeys       = key.NewBinding(key.WithKeys("right", "]"))
	dashboardDownKeys           = key.NewBinding(key.WithKeys("down", "j"))
)

// DashboardModel represents the unified dashboard page
type DashboardModel struct {
	// Balance
	Balance       uint64
	LockedBalance uint64

	// Wallet info (for header section)
	WalletName    string
	Network       string
	IsOnline      bool
	IsSynced      bool
	IsRegistered  bool
	IsRegistering bool
	RegPending    bool
	RegTxID       string
	RegStatus     string
	IsConnecting  bool // true while async daemon connection is in progress
	DaemonAddress string
	Height        uint64
	DaemonHeight  uint64

	// Display
	Address   string
	RecentTxs []Transaction

	// Actions
	quickAction        int
	wantSend           bool
	wantCopy           bool
	wantViewQR         bool
	wantViewSeed       bool
	wantViewKey        bool
	wantHistory        bool
	wantRegister       bool
	wantChangePassword bool
	wantXSWD           bool
	wantIntegratedAddr bool // NEW: generate integrated address
	wantDonate         bool // NEW: donate to developer

	// XSWD status
	xswdRunning bool

	// Debug status (for indicator only, console is global)
	debugEnabled bool

	// Flash message
	flashMessage string
	flashSuccess bool

	// Paginator for Quick Actions
	paginator paginator.Model
}

// Page info
type pageInfo struct {
	actions []actionItem
}

type actionItem struct {
	key   string
	icon  string
	label string
}

var basePages = []pageInfo{
	{
		// Page 1: Core actions
		actions: []actionItem{
			{"S", "", "Send DERO"},
			{"C", "⎘", "Copy Address"},
			{"Y", "", "QR Code"},
			{"R", "", "Payment Request"},
			{"H", "", "History"},
		},
	},
	{
		// Page 2: Advanced actions
		actions: []actionItem{
			{"V", "✎", "View Seed"},
			{"K", "⚷", "View Hex Key"},
			{"P", "↻", "Change Password"},
			{"D", "♥", "Donate"},
			{"X", "⇄", "XSWD"},
		},
	},
}

var registerAction = actionItem{"G", "✎", "Register Wallet"}

// getPages returns pages with Register action only when wallet is unregistered
func (d DashboardModel) getPages() []pageInfo {
	// Create a copy of base pages
	pages := make([]pageInfo, len(basePages))
	copy(pages, basePages)

	// Add Register action only if wallet is unregistered
	if !d.IsRegistered {
		pages[0].actions = append([]actionItem{registerAction}, pages[0].actions...)
	}

	return pages
}

// Transaction represents a transaction for display
type Transaction struct {
	TxID            string
	Amount          int64 // positive = incoming, negative = outgoing
	Height          uint64
	TopoHeight      int64
	Timestamp       string // formatted for display
	Coinbase        bool   // true if miner reward
	Incoming        bool   // true if incoming transaction
	Fee             uint64
	BlockHash       string
	Proof           string // payment proof
	Sender          string
	Destination     string
	Burn            uint64
	DestinationPort uint64 // for SC calls
	SourcePort      uint64 // for SC calls
	Status          byte   // 0=confirmed, 1=spent, 2=unknown
	Message         string // transaction message/comment
}

// NewDashboard creates a new dashboard
func NewDashboard() DashboardModel {
	p := paginator.New()
	p.Type = paginator.Arabic
	p.PerPage = 1
	p.SetTotalPages(2) // 2 pages: Core and More

	return DashboardModel{
		quickAction: 0,
		paginator:   p,
	}
}

// Init initializes the dashboard
func (d DashboardModel) Init() tea.Cmd {
	return nil
}

// Update handles events
func (d DashboardModel) Update(msg tea.Msg) (DashboardModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		// Global shortcuts - work regardless of active tab
		case key.Matches(msg, dashboardSendKeys):
			d.wantSend = true
		case key.Matches(msg, dashboardCopyKeys):
			d.wantCopy = true
		case key.Matches(msg, dashboardViewQRKeys):
			d.wantViewQR = true
		case key.Matches(msg, dashboardHistoryKeys):
			d.wantHistory = true
		case key.Matches(msg, dashboardRegisterKeys):
			d.wantRegister = true
		case key.Matches(msg, dashboardViewSeedKeys):
			d.wantViewSeed = true
		case key.Matches(msg, dashboardViewKeyKeys):
			d.wantViewKey = true
		case key.Matches(msg, dashboardChangePasswordKeys):
			d.wantChangePassword = true
		case key.Matches(msg, dashboardXSWDKeys):
			d.wantXSWD = true
		case key.Matches(msg, dashboardIntegratedAddrKeys):
			d.wantIntegratedAddr = true
		case key.Matches(msg, dashboardDonateKeys):
			d.wantDonate = true

		// Page navigation with paginator
		case key.Matches(msg, dashboardPrevPageKeys):
			if d.paginator.Page > 0 {
				d.paginator.PrevPage()
				d.quickAction = 0
			}
		case key.Matches(msg, dashboardNextPageKeys):
			if d.paginator.Page < len(d.getPages())-1 {
				d.paginator.NextPage()
				d.quickAction = 0
			}
		case key.Matches(msg, pageTabKeys):
			// Cycle through pages
			if d.paginator.Page == 0 {
				d.paginator.NextPage()
			} else {
				d.paginator.PrevPage()
			}
			d.quickAction = 0

		// Navigation within page
		case key.Matches(msg, pageUpKeys):
			if d.quickAction > 0 {
				d.quickAction--
			}
		case key.Matches(msg, dashboardDownKeys):
			page := d.getPages()[d.paginator.Page]
			if d.quickAction < len(page.actions)-1 {
				d.quickAction++
			}

		// Execute action
		case key.Matches(msg, pageEnterKeys):
			page := d.getPages()[d.paginator.Page]
			if d.quickAction >= 0 && d.quickAction < len(page.actions) {
				action := page.actions[d.quickAction]
				switch action.key {
				case "S":
					d.wantSend = true
				case "C":
					d.wantCopy = true
				case "Y":
					d.wantViewQR = true
				case "H":
					d.wantHistory = true
				case "G":
					d.wantRegister = true
				case "V":
					d.wantViewSeed = true
				case "K":
					d.wantViewKey = true
				case "P":
					d.wantChangePassword = true
				case "R":
					d.wantIntegratedAddr = true
				case "D":
					d.wantDonate = true
				case "X":
					d.wantXSWD = true
				}
			}
		}
	}
	// Clear flash message on any key press
	d.flashMessage = ""
	return d, nil
}

// View renders the unified dashboard
func (d DashboardModel) View() string {
	// ═══════════════════════════════════════════════════════════════════════
	// HEADER SECTION: Logo + Wallet Info
	// ═══════════════════════════════════════════════════════════════════════
	logo := styles.Logo()

	// Top-right status HUD (right side of logo)
	var networkStyle lipgloss.Style
	networkLabel := d.Network
	if networkLabel == "" {
		networkLabel = "Unknown"
	}
	switch d.Network {
	case "Simulator":
		networkStyle = styles.SimulatorStyle.Copy().Bold(true)
	case "Testnet":
		networkStyle = styles.TestnetStyle.Copy().Bold(true)
	default:
		networkStyle = styles.SuccessStyle.Copy().Bold(true)
	}

	displayDaemon := truncateDaemonAddress(d.DaemonAddress, 24)
	if displayDaemon == "" {
		displayDaemon = "-"
	}

	var dotStyle lipgloss.Style
	if d.IsConnecting {
		dotStyle = styles.WarningStyle
	} else if d.IsOnline {
		dotStyle = styles.SuccessStyle
	} else {
		dotStyle = styles.ErrorStyle
	}

	// Height/status badge
	statusLabel := "OFFLINE"
	statusStyle := styles.ErrorStyle
	if d.IsOnline {
		if d.IsRegistering {
			statusLabel = "REGISTERING"
			statusStyle = styles.WarningStyle
		} else if d.RegPending {
			statusLabel = "PENDING"
			statusStyle = styles.WarningStyle
		} else if !d.IsRegistered {
			statusLabel = "UNREGISTERED"
			statusStyle = styles.ErrorStyle
		} else if d.IsSynced {
			statusLabel = "SYNCED"
			statusStyle = styles.SuccessStyle
		} else {
			statusLabel = "SYNCING"
			statusStyle = styles.WarningStyle
		}
	}

	walletLabel := d.WalletName
	if walletLabel == "" {
		walletLabel = "No wallet"
	}
	walletLabel = truncateMiddle(walletLabel, 24)

	networkLine := styles.MutedStyle.Render("Network: ") +
		networkStyle.Render(networkLabel) +
		" " +
		dotStyle.Render("●") +
		" " +
		statusStyle.Render("["+statusLabel+"]")

	heightLine := styles.MutedStyle.Render("Height:  ") + styles.TextStyle.Render(styles.UintToStr(d.Height))
	daemonLine := styles.MutedStyle.Render("Daemon:  ") + styles.TextStyle.Render(displayDaemon)
	walletLine := styles.MutedStyle.Render("Wallet:  ") + styles.TextStyle.Render(walletLabel)

	hudContent := lipgloss.JoinVertical(lipgloss.Left, networkLine, heightLine, daemonLine, walletLine)
	statusCard := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.ColorBorder).
		Padding(0, 1).
		Width(38).
		Render(hudContent)

	walletInfo := lipgloss.NewStyle().Render(statusCard)
	if d.RegPending && d.RegTxID != "" {
		regLine := styles.MutedStyle.Render("Reg TX ") + styles.WarningStyle.Render(truncateHash(d.RegTxID, 16))
		walletInfo = lipgloss.JoinVertical(lipgloss.Left, walletInfo, regLine)
	}

	// Join logo and wallet info horizontally
	header := lipgloss.JoinHorizontal(lipgloss.Top,
		logo,
		"   ",
		walletInfo,
	)

	// Center the header
	headerCentered := lipgloss.NewStyle().
		Width(styles.Width - 4).
		Render(header)

	// ═══════════════════════════════════════════════════════════════════════
	// BALANCE SECTION
	// ═══════════════════════════════════════════════════════════════════════
	balanceLabel := styles.MutedStyle.Render("Available Balance")

	// Format balance - all in neon purple
	whole := d.Balance / 100000
	frac := d.Balance % 100000
	wholeStr := formatWithCommas(whole)
	fracStr := dashPadLeft(frac, 5)
	balanceLinePlain := styles.BalanceGlyph + " " + wholeStr + "." + fracStr

	balanceLine := lipgloss.NewStyle().
		Foreground(styles.ColorPrimary).
		Bold(true).
		Render(balanceLinePlain)

	// Locked balance if any
	var lockedLine string
	lockedLinePlain := ""
	if d.LockedBalance > 0 {
		lockedLinePlain = "Locked: " + styles.BalanceGlyph + " " + formatAtomic(d.LockedBalance)
		lockedLine = styles.MutedStyle.Render("Locked: ") +
			styles.WarningStyle.Render(styles.BalanceGlyph+" "+formatAtomic(d.LockedBalance))
	}

	var balanceSection string
	if lockedLine != "" {
		balanceSection = lipgloss.JoinVertical(lipgloss.Center,
			balanceLabel,
			balanceLine,
			lockedLine,
		)
	} else {
		balanceSection = lipgloss.JoinVertical(lipgloss.Center,
			balanceLabel,
			balanceLine,
		)
	}

	balanceContentWidth := lipgloss.Width("Available Balance")
	if lipgloss.Width(balanceLinePlain) > balanceContentWidth {
		balanceContentWidth = lipgloss.Width(balanceLinePlain)
	}
	if lockedLinePlain != "" && lipgloss.Width(lockedLinePlain) > balanceContentWidth {
		balanceContentWidth = lipgloss.Width(lockedLinePlain)
	}

	balanceWidth := balanceContentWidth + 4
	maxBalanceWidth := styles.Width - 54
	if maxBalanceWidth < 48 {
		maxBalanceWidth = 48
	}
	if balanceWidth > maxBalanceWidth {
		balanceWidth = maxBalanceWidth
	}
	if balanceWidth < 36 {
		balanceWidth = 36
	}

	balanceCard := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.ColorBorder).
		Padding(0, 1).
		Width(balanceWidth).
		Align(lipgloss.Center).
		Render(balanceSection)

	balanceCentered := lipgloss.NewStyle().
		Width(styles.Width - 4).
		Align(lipgloss.Center).
		Render(balanceCard)

	addressWidth := styles.Width - 8

	// ═══════════════════════════════════════════════════════════════════════
	// QUICK ACTIONS + RECENT ACTIVITY (side by side)
	// Keep combined width aligned with address frame edges.
	// ═══════════════════════════════════════════════════════════════════════
	boxesGap := 2
	if (addressWidth-boxesGap)%2 != 0 {
		boxesGap = 3
	}
	leftBoxWidth := (addressWidth - boxesGap) / 2
	rightBoxWidth := leftBoxWidth

	// Quick Actions box with paginator
	page := d.getPages()[d.paginator.Page]

	// Build paginator header with bubbletea paginator dots
	paginatorView := d.paginator.View()
	paginatorHeaderPlain := "Quick Actions " + styles.IntToStr(int64(d.paginator.Page+1)) + "/" + styles.IntToStr(int64(len(d.getPages())))
	paginatorHeader := styles.TitleStyle.Render("Quick Actions") + " " + paginatorView

	// Build action list for current page
	actionLabels := make([]string, 0, len(page.actions))
	for _, action := range page.actions {
		actionLabels = append(actionLabels, action.label)
	}
	labelWidth := maxLabelWidth(actionLabels)

	var actionsLines []string
	actionsContentWidth := leftBoxWidth - 4
	for i, action := range page.actions {
		labelPadded := padLabel(action.label, labelWidth)
		labelForRender := labelPadded
		if action.key == "X" {
			labelForRender = strings.TrimRight(labelPadded, " ")
		}
		prefix := "[" + action.key + "] " + action.icon + " "
		rowBase := prefix + labelForRender
		rowPlain := fitCellLeft(rowBase, actionsContentWidth-2)
		if i == d.quickAction {
			selectedStyle := styles.SelectedRowStyle

			line := styles.SelectedMenuItemStyle.Render("▸ ")
			if action.key == "X" {
				base := rowBase + " "
				dotStyle := styles.MutedStyle.Copy().Background(styles.SelectedRowBackground()).Bold(true)
				if d.xswdRunning {
					dotStyle = styles.SuccessStyle.Copy().Background(styles.SelectedRowBackground()).Bold(true)
				}
				segment := selectedStyle.Render(base) + dotStyle.Render("●")
				line += segment + selectedStyle.Render(strings.Repeat(" ", maxInt(0, actionsContentWidth-2-len(rowBase)-2)))
			} else {
				line += selectedStyle.Render(rowPlain)
			}
			actionsLines = append(actionsLines, line)
		} else {
			// Unselected: aligned rows with muted text
			line := "  "
			if action.key == "X" {
				base := rowBase + " "
				if d.xswdRunning {
					line += styles.MutedStyle.Render(base) + styles.SuccessStyle.Render("●")
				} else {
					line += styles.MutedStyle.Render(base) + styles.MutedStyle.Render("●")
				}
				line += styles.MutedStyle.Render(strings.Repeat(" ", maxInt(0, actionsContentWidth-3-len(rowBase)-2)))
			} else {
				line += styles.MutedStyle.Render(fitCellLeft(rowBase, actionsContentWidth-3))
			}
			actionsLines = append(actionsLines, line)
		}
	}

	actionsContent := strings.Join(actionsLines, "\n")

	actionsBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.ColorBorder).
		Padding(0, 1).
		Width(leftBoxWidth).
		Render(centerRenderedLine(paginatorHeaderPlain, paginatorHeader, actionsContentWidth) + "\n" + actionsContent)

	// Recent Activity box
	recentTitlePlain := "Recent Activity"
	recentTitle := styles.TitleStyle.Render(recentTitlePlain)
	var recentLines []string
	recentContentWidth := rightBoxWidth - 4
	const amountColWidth = 11
	const dateColWidth = 10
	if len(d.RecentTxs) == 0 {
		plainLine := "No transactions yet"
		renderLine := styles.MutedStyle.Render(plainLine)
		recentLines = append(recentLines, centerRenderedLine(plainLine, renderLine, recentContentWidth))
	} else {
		for i, tx := range d.RecentTxs {
			if i >= 5 {
				break
			}
			var icon, iconGlyph, amountRaw string
			var amountStyle lipgloss.Style
			if tx.Coinbase {
				// Coinbase (miner reward) - green
				iconGlyph = "⛏"
				icon = styles.TxInStyle.Render(iconGlyph)
				amountRaw = "+" + formatAmount(tx.Amount)
				amountStyle = styles.TxInStyle
			} else if tx.Amount >= 0 {
				// Incoming - green
				iconGlyph = "↓"
				icon = styles.TxInStyle.Render(iconGlyph)
				amountRaw = "+" + formatAmount(tx.Amount)
				amountStyle = styles.TxInStyle
			} else {
				// Outgoing - red
				iconGlyph = "↑"
				icon = styles.TxOutStyle.Render(iconGlyph)
				amountRaw = formatAmount(tx.Amount)
				amountStyle = styles.TxOutStyle
			}

			amountColPlain := fitCellLeft(amountRaw, amountColWidth)
			dateColPlain := fitCellLeft(tx.Timestamp, dateColWidth)
			plainLine := iconGlyph + " " + amountColPlain + "  " + dateColPlain

			amountCol := amountStyle.Render(amountColPlain)
			dateCol := styles.MutedStyle.Render(dateColPlain)
			renderLine := icon + " " + amountCol + "  " + dateCol
			recentLines = append(recentLines, centerRenderedLine(plainLine, renderLine, recentContentWidth))
		}
	}
	recentContent := strings.Join(recentLines, "\n")

	recentBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.ColorBorder).
		Padding(0, 1).
		Width(rightBoxWidth).
		Render(centerRenderedLine(recentTitlePlain, recentTitle, recentContentWidth) + "\n" + recentContent)

	// Side by side
	boxes := lipgloss.JoinHorizontal(lipgloss.Top,
		actionsBox,
		strings.Repeat(" ", boxesGap),
		recentBox,
	)

	boxesCentered := lipgloss.NewStyle().
		Width(styles.Width - 4).
		Align(lipgloss.Center).
		Render(boxes)

	// ═══════════════════════════════════════════════════════════════════════
	// ADDRESS LINE
	// ═══════════════════════════════════════════════════════════════════════
	addressLabel := "Address: "
	addressText := lipgloss.NewStyle().Foreground(styles.ColorAccent).Render("⎘ ") +
		styles.MutedStyle.Render(addressLabel) +
		styles.TextStyle.Render(truncateMiddle(d.Address, addressWidth-6-len(addressLabel)))
	addrBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.ColorBorder).
		Padding(0, 1).
		Width(addressWidth).
		Render(addressText)

	addrLine := lipgloss.NewStyle().
		Width(styles.Width - 4).
		Align(lipgloss.Center).
		Render(addrBox)

	// ═══════════════════════════════════════════════════════════════════════
	// FOOTER HINTS
	// ═══════════════════════════════════════════════════════════════════════
	hints := []string{"↑↓ Navigate", "Tab/←/→ Tabs", "Enter Select", "C Copy", "Esc Close", "Q Quit"}
	footerContent := strings.Join(hints, " • ")
	footer := lipgloss.NewStyle().
		Width(styles.Width - 4).
		Align(lipgloss.Center).
		Render(styles.MutedStyle.Render(footerContent))

	// ═══════════════════════════════════════════════════════════════════════
	// FLASH MESSAGE (if any) - constrain width to fit in box
	// ═══════════════════════════════════════════════════════════════════════
	var flashView string
	if d.flashMessage != "" {
		if d.flashSuccess {
			flashView = styles.SuccessStyle.
				Width(styles.Width - 8). // Account for box padding
				Render("✓ " + d.flashMessage)
		} else {
			flashView = styles.ErrorStyle.
				Width(styles.Width - 8). // Account for box padding
				Render("✗ " + d.flashMessage)
		}
	}

	// ═══════════════════════════════════════════════════════════════════════
	// COMPOSE FULL VIEW
	// ═══════════════════════════════════════════════════════════════════════
	var content string
	if flashView != "" {
		content = lipgloss.JoinVertical(lipgloss.Left,
			headerCentered,
			"",
			flashView,
			"",
			balanceCentered,
			"",
			"",
			boxesCentered,
			"",
			addrLine,
			"",
			footer,
		)
	} else {
		content = lipgloss.JoinVertical(lipgloss.Left,
			headerCentered,
			"",
			balanceCentered,
			"",
			"",
			boxesCentered,
			"",
			addrLine,
			"",
			footer,
		)
	}

	return lipgloss.NewStyle().
		Padding(0, 2).
		Render(content)
}

// SetBalance updates the balance
func (d *DashboardModel) SetBalance(balance, locked uint64) {
	d.Balance = balance
	d.LockedBalance = locked
}

// SetAddress sets the wallet address
func (d *DashboardModel) SetAddress(addr string) {
	d.Address = addr
}

// SetRecentTxs sets recent transactions
func (d *DashboardModel) SetRecentTxs(txs []Transaction) {
	d.RecentTxs = txs
}

// SetWalletInfo sets wallet info for the header section
func (d *DashboardModel) SetWalletInfo(name, network string, isOnline, isSynced, isRegistered bool, daemonAddr string, height, daemonHeight uint64) {
	d.WalletName = name
	d.Network = network
	d.IsOnline = isOnline
	d.IsSynced = isSynced
	d.IsRegistered = isRegistered
	d.DaemonAddress = daemonAddr
	d.Height = height
	d.DaemonHeight = daemonHeight

	// Update paginator to reflect correct number of pages based on registration status
	d.paginator.SetTotalPages(len(d.getPages()))
}

// SetRegistering sets the registration in-progress state.
func (d *DashboardModel) SetRegistering(registering bool) {
	d.IsRegistering = registering
}

// SetRegistrationPending sets pending registration status and tx info.
func (d *DashboardModel) SetRegistrationPending(txID, status string) {
	d.RegTxID = txID
	d.RegStatus = status
	d.RegPending = txID != ""
}

// SetConnecting sets the connecting state
func (d *DashboardModel) SetConnecting(connecting bool) {
	d.IsConnecting = connecting
}

// WantSend returns true if user wants to send
func (d DashboardModel) WantSend() bool {
	return d.wantSend
}

// WantCopy returns true if user wants to copy address
func (d DashboardModel) WantCopy() bool {
	return d.wantCopy
}

// WantViewQR returns true if user wants to view address QR code
func (d DashboardModel) WantViewQR() bool {
	return d.wantViewQR
}

// WantViewSeed returns true if user wants to view seed
func (d DashboardModel) WantViewSeed() bool {
	return d.wantViewSeed
}

// WantViewKey returns true if user wants to view hex key
func (d DashboardModel) WantViewKey() bool {
	return d.wantViewKey
}

// WantHistory returns true if user wants to view history
func (d DashboardModel) WantHistory() bool {
	return d.wantHistory
}

// WantRegister returns true if user wants to register wallet.
func (d DashboardModel) WantRegister() bool {
	return d.wantRegister
}

// WantChangePassword returns true if user wants to change password
func (d DashboardModel) WantChangePassword() bool {
	return d.wantChangePassword
}

// WantXSWD returns true if user wants to toggle XSWD server
func (d DashboardModel) WantXSWD() bool {
	return d.wantXSWD
}

// WantIntegratedAddr returns true if user wants to generate integrated address
func (d DashboardModel) WantIntegratedAddr() bool {
	return d.wantIntegratedAddr
}

// WantDonate returns true if user wants to donate
func (d DashboardModel) WantDonate() bool {
	return d.wantDonate
}

// SetXSWDRunning sets the XSWD running status
func (d *DashboardModel) SetXSWDRunning(running bool) {
	d.xswdRunning = running
}

// SetDebugEnabled sets the debug enabled status (for indicator only)
func (d *DashboardModel) SetDebugEnabled(enabled bool) {
	d.debugEnabled = enabled
}

func truncateDaemonAddress(addr string, maxLen int) string {
	if len(addr) <= maxLen || maxLen < 4 {
		return addr
	}

	colon := strings.LastIndex(addr, ":")
	if colon > 0 {
		suffix := addr[colon:]
		if len(suffix) <= 8 {
			prefixLen := maxLen - len(suffix) - 3
			if prefixLen >= 4 {
				return addr[:prefixLen] + "..." + suffix
			}
		}
	}

	return addr[:maxLen-3] + "..."
}

func truncateHash(hash string, maxLen int) string {
	if len(hash) <= maxLen || maxLen < 10 {
		return hash
	}
	prefixLen := (maxLen - 3) / 2
	suffixLen := maxLen - 3 - prefixLen
	return hash[:prefixLen] + "..." + hash[len(hash)-suffixLen:]
}

func truncateMiddle(text string, maxLen int) string {
	if len(text) <= maxLen || maxLen < 10 {
		return text
	}
	prefixLen := (maxLen - 3) / 2
	suffixLen := maxLen - 3 - prefixLen
	return text[:prefixLen] + "..." + text[len(text)-suffixLen:]
}

func fitCellLeft(text string, width int) string {
	if width < 1 {
		return ""
	}
	if len(text) >= width {
		return text[:width]
	}
	return text + strings.Repeat(" ", width-len(text))
}

func centerRenderedLine(plainLine, renderedLine string, width int) string {
	if width <= 0 {
		return renderedLine
	}
	visible := lipgloss.Width(plainLine)
	if visible >= width {
		return renderedLine
	}
	padTotal := width - visible
	leftPad := padTotal / 2
	rightPad := padTotal - leftPad
	return strings.Repeat(" ", leftPad) + renderedLine + strings.Repeat(" ", rightPad)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ResetActions resets action flags
func (d *DashboardModel) ResetActions() {
	d.wantSend = false
	d.wantCopy = false
	d.wantViewQR = false
	d.wantViewSeed = false
	d.wantViewKey = false
	d.wantHistory = false
	d.wantRegister = false
	d.wantChangePassword = false
	d.wantXSWD = false
	d.wantIntegratedAddr = false
	d.wantDonate = false
	// Don't reset tab or selection - let user stay where they were
}

// HandleMouse handles mouse events on the dashboard
func (d DashboardModel) HandleMouse(msg tea.MouseClickMsg, windowWidth, windowHeight int) DashboardModel {
	// Calculate the centered content position
	contentWidth := styles.Width - 4
	contentX := (windowWidth - contentWidth) / 2
	contentY := (windowHeight - 30) / 2 // Approximate dashboard height

	addressWidth := styles.Width - 8
	boxesGap := 2
	if (addressWidth-boxesGap)%2 != 0 {
		boxesGap = 3
	}
	leftBoxWidth := (addressWidth - boxesGap) / 2
	rightBoxWidth := leftBoxWidth
	boxesTotalWidth := leftBoxWidth + boxesGap + rightBoxWidth
	boxesStartX := (contentWidth - boxesTotalWidth) / 2
	recentStartX := boxesStartX + leftBoxWidth + boxesGap

	// Adjust coordinates to be relative to content
	mouseX, mouseY := msg.Mouse().X, msg.Mouse().Y
	relX := mouseX - contentX
	relY := mouseY - contentY

	switch msg.Button {
	case tea.MouseLeft:
		// Quick actions box - action items start at y=15 (after paginator header)
		page := d.getPages()[d.paginator.Page]
		actionsTop := 15
		actionsBottom := actionsTop + len(page.actions)
		if relX >= boxesStartX && relX < boxesStartX+leftBoxWidth && relY >= actionsTop && relY < actionsBottom {
			actionIndex := relY - 15
			if actionIndex >= 0 && actionIndex < len(page.actions) {
				d.quickAction = actionIndex
				action := page.actions[actionIndex]
				switch action.key {
				case "S":
					d.wantSend = true
				case "C":
					d.wantCopy = true
				case "Y":
					d.wantViewQR = true
				case "H":
					d.wantHistory = true
				case "G":
					d.wantRegister = true
				case "V":
					d.wantViewSeed = true
				case "K":
					d.wantViewKey = true
				case "P":
					d.wantChangePassword = true
				case "R":
					d.wantIntegratedAddr = true
				case "D":
					d.wantDonate = true
				case "X":
					d.wantXSWD = true
				}
			}
		}

		// Recent Activity box is on the right side, around y=15-23
		if relX >= recentStartX && relX < recentStartX+rightBoxWidth && relY >= 15 && relY < 23 {
			// Click on a transaction row
			txIndex := relY - 15
			if txIndex >= 0 && txIndex < len(d.RecentTxs) && txIndex < 5 {
				// Could show transaction details here
				// For now, just navigate to history page
				d.wantHistory = true
			}
		}

	case tea.MouseWheelUp:
		// Navigate actions up within current page
		if d.quickAction > 0 {
			d.quickAction--
		}

	case tea.MouseWheelDown:
		// Navigate actions down within current page
		page := d.getPages()[d.paginator.Page]
		if d.quickAction < len(page.actions)-1 {
			d.quickAction++
		}
	}

	return d
}

// SetFlashMessage sets a flash message to display
func (d *DashboardModel) SetFlashMessage(message string, success bool) {
	d.flashMessage = message
	d.flashSuccess = success
}

// Helper functions

func formatAmount(amount int64) string {
	negative := amount < 0
	if negative {
		amount = -amount
	}
	whole := amount / 100000
	frac := amount % 100000

	result := dashIntToStr(whole) + "." + dashPadLeft(uint64(frac), 5)
	if negative {
		result = "-" + result
	}
	return result
}

func formatAtomic(atomic uint64) string {
	whole := atomic / 100000
	frac := atomic % 100000
	return dashIntToStr(int64(whole)) + "." + dashPadLeft(frac, 5)
}

func formatWithCommas(n uint64) string {
	if n == 0 {
		return "0"
	}
	str := ""
	for n > 0 {
		if str != "" {
			str = "," + str
		}
		if n < 1000 {
			str = dashUintToStr(n) + str
			break
		}
		str = dashPadLeft(n%1000, 3) + str
		n /= 1000
	}
	return str
}

func dashIntToStr(n int64) string {
	return styles.IntToStr(n)
}

func dashUintToStr(n uint64) string {
	return styles.UintToStr(n)
}

func dashPadLeft(n uint64, width int) string {
	s := styles.UintToStr(n)
	for len(s) < width {
		s = "0" + s
	}
	return s
}
