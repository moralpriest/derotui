// Copyright 2017-2026 DERO Project. All rights reserved.

package ui

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"charm.land/bubbles/v2/filepicker"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/deroproject/dero-wallet-cli/internal/config"
	derolog "github.com/deroproject/dero-wallet-cli/internal/log"
	daemonservice "github.com/deroproject/dero-wallet-cli/internal/services/daemon"
	minerservice "github.com/deroproject/dero-wallet-cli/internal/services/miner"
	"github.com/deroproject/dero-wallet-cli/internal/ui/pages"
	"github.com/deroproject/dero-wallet-cli/internal/ui/styles"
	"github.com/deroproject/dero-wallet-cli/internal/wallet"
)

func init() {
	// Disable logging by default - will be enabled by SetupLogging if --debug flag is set
	log.SetOutput(io.Discard)
}

// DebugLogPath is the path to the unified log file (~/.derotui/derotui.log)
var DebugLogPath string

const donationAddress = "deroi1qy8zrqrgqgcu6ayznw5zl9a50erdxgjd539rh3hz7qgu4zl4auqzkq9pvfp4x7p4235xzmntypuk7afqg3skgereypg8y6t9wd6zpuylnx8jqjfqwd5xzmrvypex2ur9de6zqf3qvahjqan9vaskup5n768"

const debugExpandedLogLines = 3

const (
	initialDaemonRetryInterval = 3 * time.Second
	maxDaemonRetryInterval     = 20 * time.Second
	txRefreshIntervalOffline   = 30 * time.Second
)

// SetupLogging initializes logging to ~/.derotui/derotui.log. The log is
// always written (Info baseline, so navigation diagnostics survive without
// --debug); debug=true additionally records Debug-level entries.
func SetupLogging(debug bool) {
	path, err := derolog.Setup(debug)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to open log file: %v\n", err)
		return
	}

	DebugLogPath = path
	if debug {
		fmt.Printf("Debug log: %s\n", path)
	}
}

// diagLog records navigation diagnostics as Debug-level ui.nav entries in the
// unified log file. They are only captured when debug logging is enabled and
// never shown in the debug console (see IsHighSignal).
func diagLog(format string, args ...interface{}) {
	derolog.Debug("ui.nav", "diag", fmt.Sprintf(format, args...))
}

// Page represents the current page
type Page int

const (
	PageWelcome Page = iota
	PageFilePicker
	PagePassword
	PageNetwork // Network selection for unknown wallets
	PageSeed
	PageKeyInput
	PageQRCode
	PageMain
	PageSend
	PageHistory
	PageTxDetails
	PageDaemon
	PageDaemonStatus
	PageDaemonLogs
	PageDaemonSettings
	PageIntegratedAddr // Generate integrated address
	PageXSWDAuth       // XSWD app authorization dialog
	PageXSWDPerm       // XSWD permission request dialog
	PageMiner          // Embedded miner
	PageNames          // Registered names management
	PageNameRegister   // Register a new name
	PageNameTransfer   // Transfer name ownership
	PageTokens         // Token management
	PageTokenSend      // Send token
	PageTokenHistory   // Token history
	PageDiscover       // TELA / NFT / NFA catalog
	PageLogo           // Hex logo preview
)

// CLIOptions holds command line options
type CLIOptions struct {
	WalletFile    string
	Password      string
	Offline       bool
	OfflineFile   string
	RPCServer     bool
	RPCBind       string
	RPCLogin      string
	RPCPassChange bool
	Testnet       bool
	Simulator     bool
	// Explicit network flags passed by user via CLI.
	// These are immutable intent flags and should not be set by auto-detection.
	ExplicitTestnet   bool
	ExplicitSimulator bool
	DaemonAddress     string
	SocksProxy        string
	GenerateNew       bool
	RestoreSeed       bool
	ElectrumSeed      string
	Unlocked          bool
	Debug             bool
	DaemonOnly        bool
}

// Model is the main application model
type Model struct {
	// State
	page       Page
	width      int
	height     int
	quitting   bool
	walletFile string

	// Options
	Opts CLIOptions

	// Wallet
	wallet *wallet.Wallet

	// Components
	filePicker filepicker.Model

	// Pages
	welcome        pages.WelcomeModel
	password       pages.PasswordModel
	network        pages.NetworkModel // Network selection page
	seed           pages.SeedModel
	keyInput       pages.KeyInputModel
	qrcode         pages.QRCodeModel
	dashboard      pages.DashboardModel
	send           pages.SendModel
	history        pages.HistoryModel
	txDetails      pages.TxDetailsModel
	daemon         pages.DaemonModel
	daemonStatus   pages.DaemonStatusModel
	daemonLogs     pages.DaemonLogsModel
	daemonSettings pages.DaemonSettingsModel
	miner          pages.MinerModel
	integratedAddr pages.IntegratedAddrModel
	palette        pages.PaletteModel
	names          pages.NamesModel
	nameRegister   pages.NameRegisterModel
	nameTransfer   pages.NameTransferModel
	tokens         pages.TokensModel
	tokenSend      pages.TokenSendModel
	tokenHistory   pages.TokenHistoryModel
	discover       pages.DiscoverModel
	logo           pages.LogoModel

	// State flags
	isCreating           bool
	isRestoringFromSeed  bool
	isRestoringFromKey   bool
	isChangingPassword   bool
	pendingKey           string
	pendingPassword      string                 // Store password while selecting network
	pendingNetwork       pages.NetworkSelection // Store network while creating/restoring
	pendingCreateRestore string                 // Store wallet name while selecting network

	// XSWD
	program      wallet.MsgSender // tea.Program reference for XSWD message injection
	xswdBridge   *wallet.XSWDBridge
	xswdStarting bool // in-flight guard: a startXSWDCmd is running (same-batch double start)
	xswdAuth     pages.XSWDAuthModel
	xswdPerm     pages.XSWDPermModel
	xswdPrevPage Page      // page to return to after dialog
	xswdAuthCh   chan bool // response channel for current auth request
	xswdPermCh   chan int  // response channel for current perm request

	// QR return page tracking
	qrReturnPage Page // page to return to after QR code view

	// Miner return page tracking (dashboard vs welcome)
	minerReturnPage Page
	logoReturnPage  Page

	// Cached daemon status (from welcome page checks)
	cachedDaemonHealthy bool
	cachedDaemonAddress string
	cachedDaemonNetwork string

	// Sticky daemon selection set by explicit /connect.
	// This survives wallet transitions so create/open/restore can keep using
	// the exact daemon the user selected.
	stickyDaemonHealthy   bool
	stickyDaemonAddress   string
	stickyDaemonTestnet   bool
	stickyDaemonSimulator bool
	cliDaemonAddress      string
	lastWalletDaemon      string

	// Global debug console state (visible on all pages)
	debugEnabled           bool
	debugConsoleOpen       bool
	debugAutoFollow        bool
	debugScrollStart       int
	debugLastClickY        int
	debugLastClickAt       time.Time
	debugLogEntries        []derolog.LogEntry
	regHintShown           bool
	pendingRegTxID         string
	pendingRegStatus       string
	pendingRegHeight       uint64
	pendingOutgoing        map[string]pendingOutgoingTx
	startupFlowSet         bool
	lastDaemonRetry        time.Time
	daemonRetryAfter       time.Duration
	lastTxRefreshAt        time.Time
	tokenScanActive        bool
	discoverHydrating      bool
	discoverTried          map[string]bool
	discoverProbing        bool
	discoverOwnedDone      bool
	discoverRatingsLoading bool
	discoverCatalogLoading bool
	// Incremental token scan state (see tokenScanProgressMsg).
	tokenScanID         int
	tokenScanCandidates []string
	tokenScanPending    []string
	tokenRecheckActive  bool
	tokenScanFound      int
	tokenScanStartedAt  time.Time
	hyperCompleteLogged bool
	daemonManager       *daemonservice.Manager
	embeddedDaemon      *daemonservice.EmbeddedDaemon
	rpcMiner            minerservice.RPCBackend
	daemonManagedSince  time.Time
	lastEmbeddedError   string
	pendingPrune        bool
	pruneAppliedOnce    bool
	applyingPrune       bool
	hyperGnomon         *wallet.HyperGnomon
	hyperMu             *sync.Mutex
}

type pendingOutgoingTx struct {
	tx      pages.Transaction
	addedAt time.Time
}

// NewModel creates a new application model
func NewModel() Model {
	fp := filepicker.New()
	fp.AllowedTypes = []string{".db"}
	fp.CurrentDirectory = "."
	fp.ShowHidden = false
	fp.SetHeight(10)

	// Customize KeyMap to remove esc from Back (we handle it ourselves)
	fp.KeyMap.Back = key.NewBinding(key.WithKeys("h", "backspace", "left"), key.WithHelp("h", "back"))
	applyFilePickerTheme(&fp)

	m := Model{
		page:             PageWelcome,
		filePicker:       fp,
		welcome:          pages.NewWelcome(),
		password:         pages.NewPassword(pages.PasswordModeUnlock),
		network:          pages.NewNetwork(""),
		keyInput:         pages.NewKeyInput(),
		dashboard:        pages.NewDashboard(),
		send:             pages.NewSend(),
		history:          pages.NewHistory(),
		txDetails:        pages.NewTxDetails(),
		daemon:           pages.NewDaemon(false, false),
		daemonStatus:     pages.NewDaemonStatus(),
		daemonLogs:       pages.NewDaemonLogs(),
		daemonSettings:   pages.NewDaemonSettings(config.GetDaemonSettings()),
		miner:            pages.NewMiner(),
		palette:          pages.NewPalette(),
		names:            pages.NewNames(),
		nameRegister:     pages.NewNameRegister(),
		nameTransfer:     pages.NewNameTransfer(),
		tokens:           pages.NewTokens(),
		tokenSend:        pages.NewTokenSend(),
		tokenHistory:     pages.NewTokenHistory(),
		discover:         pages.NewDiscover(),
		logo:             pages.NewLogo(),
		daemonManager:    daemonservice.NewManager(),
		daemonRetryAfter: initialDaemonRetryInterval,
		hyperMu:          &sync.Mutex{},
	}
	m.welcome.Version = Version
	m.password.SetVersion(Version)
	m.embeddedDaemon = daemonservice.NewEmbeddedDaemon(nil)

	return m
}

// SetProgram sets the program reference for XSWD bridge message injection.
// This should be called immediately after creating the tea.Program but before Run().
func (m *Model) SetProgram(p wallet.MsgSender) {
	m.program = p
}

// SetCLIDaemonAddress records the daemon address provided via CLI flags.
// It is set before the program runs (Init's value receiver cannot persist
// model mutations) so shutdownSession can restore the CLI value after a
// /connect overwrites m.Opts.DaemonAddress.
func (m *Model) SetCLIDaemonAddress(addr string) {
	m.cliDaemonAddress = addr
}

func applyFilePickerTheme(fp *filepicker.Model) {
	s := filepicker.DefaultStyles()
	s.Cursor = lipgloss.NewStyle().Foreground(styles.ColorPrimary).Bold(true)
	s.Selected = lipgloss.NewStyle().Foreground(styles.ColorPrimary).Bold(true)
	s.Directory = lipgloss.NewStyle().Foreground(styles.ColorAccent)
	s.Symlink = lipgloss.NewStyle().Foreground(styles.ColorAccent)
	s.File = lipgloss.NewStyle().Foreground(styles.ColorText)
	s.Permission = lipgloss.NewStyle().Foreground(styles.ColorMuted)
	s.FileSize = lipgloss.NewStyle().Foreground(styles.ColorMuted)
	s.DisabledCursor = lipgloss.NewStyle().Foreground(styles.ColorMuted)
	s.DisabledFile = lipgloss.NewStyle().Foreground(styles.ColorMuted)
	s.DisabledSelected = lipgloss.NewStyle().Foreground(styles.ColorMuted)
	s.EmptyDirectory = lipgloss.NewStyle().Foreground(styles.ColorMuted)
	fp.Styles = s
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		m.filePicker.Init(),
		m.checkDaemonStatus(), // Check daemon on startup
		m.tickCmd(),           // Start periodic updates
		m.setWindowTitleCmd(), // Set initial window title
	}
	if !m.startupFlowSet {
		cmds = append(cmds, m.checkStartupWallet())
	}

	return tea.Batch(cmds...)
}

// checkStartupWallet checks for CLI wallet or last known wallet
func (m *Model) checkStartupWallet() tea.Cmd {
	return func() tea.Msg {
		// If wallet file specified via CLI, use that
		if m.Opts.WalletFile != "" {
			return startupCheckMsg{lastWallet: m.Opts.WalletFile}
		}
		// Check for last known wallet
		lastWallet := config.GetLastWallet()
		return startupCheckMsg{lastWallet: lastWallet}
	}
}

// checkDaemonStatus returns a command that checks daemon status
func (m *Model) checkDaemonStatus() tea.Cmd {
	// Capture values now to avoid closure issues with pointer receiver
	lastWalletDaemon := m.lastWalletDaemon
	daemonAddr := m.Opts.DaemonAddress
	testnet := m.Opts.Testnet
	simulator := m.Opts.Simulator
	return func() tea.Msg {
		statusFor := func(address string, defaultNetwork string) daemonStatusEntry {
			info := wallet.GetDaemonInfo(context.Background(), address)
			network := defaultNetwork
			if info.IsOnline {
				if info.Network == "Simulator" {
					network = "Simulator"
				} else if info.Testnet {
					network = "Testnet"
				} else {
					network = "Mainnet"
				}
			}
			info = classifyDaemonSync(info, network, 0)
			return daemonStatusEntry{
				isOnline:        info.IsOnline,
				isSynced:        info.IsSynced,
				isSyncing:       info.IsSyncing,
				isBootstrapping: info.IsBootstrapping,
				isHealthy:       info.IsHealthy,
				network:         network,
				address:         address,
				height:          info.Height,
				peerHeight:      info.PeerHeight,
				syncProgress:    info.SyncProgress,
			}
		}

		if lastWalletDaemon != "" {
			entry := statusFor(lastWalletDaemon, "Mainnet")
			return daemonStatusMsg{daemons: []daemonStatusEntry{entry}}
		}

		if daemonAddr != "" {
			entry := statusFor(daemonAddr, "Mainnet")
			return daemonStatusMsg{daemons: []daemonStatusEntry{entry}}
		}

		if simulator {
			entry := statusFor(wallet.DefaultSimulatorDaemon, "Simulator")
			return daemonStatusMsg{daemons: []daemonStatusEntry{entry}}
		}
		if testnet {
			entry := statusFor(wallet.DefaultTestnetDaemon, "Testnet")
			return daemonStatusMsg{daemons: []daemonStatusEntry{entry}}
		}

		candidates := []struct {
			address string
			network string
		}{
			{wallet.DefaultMainnetDaemon, "Mainnet"},
			{wallet.DefaultTestnetDaemon, "Testnet"},
			{wallet.DefaultSimulatorDaemon, "Simulator"},
		}

		daemons := make([]daemonStatusEntry, 0, len(candidates))
		for _, candidate := range candidates {
			entry := statusFor(candidate.address, candidate.network)
			if entry.isHealthy {
				daemons = append(daemons, entry)
			}
		}

		if len(daemons) > 0 {
			return daemonStatusMsg{daemons: daemons}
		}

		fallback := statusFor(wallet.FallbackMainnetDaemon, "Mainnet")
		if fallback.isHealthy {
			return daemonStatusMsg{daemons: []daemonStatusEntry{fallback}}
		}

		mainnetLocal := statusFor(wallet.DefaultMainnetDaemon, "Mainnet")
		return daemonStatusMsg{daemons: []daemonStatusEntry{mainnetLocal}}
	}
}

// Update handles messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Diagnostic: record every keypress while on a daemon page so an
	// unexpected back-out can be traced from ~/.derotui/derotui.log alone.
	switch m.page {
	case PageDaemonStatus, PageDaemonLogs, PageDaemonSettings:
		if kp, ok := msg.(tea.KeyPressMsg); ok {
			diagLog("key page=%d key=%q", m.page, kp.String())
		}
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.page == PageTokens {
			var cmd tea.Cmd
			m.tokens, cmd = m.tokens.Update(msg)
			return m, cmd
		}
		if m.page == PageDiscover {
			var cmd tea.Cmd
			m.discover, cmd = m.discover.Update(msg)
			return m, cmd
		}
		return m, nil

	case tea.MouseClickMsg:
		// Route mouse events to current page
		if handled := m.handleMouseEvent(msg); handled {
			return m, tea.Batch(cmds...)
		}

	case tea.KeyPressMsg:
		keyStr := msg.String()
		// Global quit - Ctrl+C works from any page
		if key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+c"))) {
			return m, m.shutdownSession(true)
		}

		// Global command palette: when open, it consumes all keys.
		if m.palette.IsOpen() {
			var paletteCmd tea.Cmd
			m.palette, paletteCmd = m.palette.Update(msg)
			cmds = append(cmds, paletteCmd)
			// Dispatch any command the user selected (palette closes itself).
			if m.palette.Action() != pages.ActionNone {
				cmds = append(cmds, m.handlePaletteAction())
			}
			return m, tea.Batch(cmds...)
		}

		// "/" opens the command palette on eligible pages.
		if keyStr == "/" && paletteEnabled(m.page) {
			m.palette.Open(m.wallet != nil)
			return m, tea.Batch(cmds...)
		}

		// Q to quit only from main page (dashboard)
		if key.Matches(msg, key.NewBinding(key.WithKeys("q"))) && m.page == PageMain {
			return m, m.shutdownSession(true)
		}

		// Esc handling for different pages
		if key.Matches(msg, key.NewBinding(key.WithKeys("esc"))) {
			switch m.page {
			case PageMain:
				closeCmd := m.shutdownSession(false)
				m.page = PageWelcome
				m.welcome = pages.NewWelcome()
				m.welcome.Version = Version
				return m, tea.Batch(closeCmd, m.checkDaemonStatus(), m.setWindowTitleCmd())
			case PageSend:
				// Go back to dashboard
				m.send.Reset()
				m.page = PageMain
				return m, m.setWindowTitleCmd()
			case PageHistory:
				// Go back to dashboard
				m.page = PageMain
				return m, m.setWindowTitleCmd()
			case PageTxDetails:
				// Go back to history
				m.txDetails.Reset()
				m.page = PageHistory
				return m, m.setWindowTitleCmd()
			case PageTokens:
				m.page = PageMain
				return m, m.setWindowTitleCmd()
			case PageDiscover:
				// The discover page owns Esc: first Esc closes the dApp info
				// popup, only a second Esc (no popup open) leaves the page.
				// Handle it in the page model so the popup can never trap the
				// user outside the page with the detail still open.
				var discCmd tea.Cmd
				m.discover, discCmd = m.discover.Update(msg)
				cmds = append(cmds, discCmd)
				if m.discover.Cancelled() {
					m.discover.ClearCancelled()
					m.page = PageMain
					if m.wallet == nil {
						m.page = PageWelcome
					}
					cmds = append(cmds, m.setWindowTitleCmd())
				}
				return m, tea.Batch(cmds...)
			case PageLogo:
				m.logo.ClearCancelled()
				m.page = m.logoReturnPage
				if m.page == PageLogo {
					m.page = PageWelcome
				}
				return m, m.setWindowTitleCmd()
			case PageTokenSend:
				m.tokenSend.Reset()
				m.page = PageTokens
				return m, m.setWindowTitleCmd()
			case PageTokenHistory:
				m.tokenHistory.Reset()
				m.page = PageTokens
				return m, m.setWindowTitleCmd()
			}
		}

		// Global debug console shortcuts (work on any page)
		// F12 opens debug console immediately when disabled, then toggles panel mode.
		if keyStr == "f12" || key.Matches(msg, key.NewBinding(key.WithKeys("f12"))) {
			if m.debugEnabled {
				m.debugConsoleOpen = !m.debugConsoleOpen
				return m, tea.Batch(cmds...)
			}
			if m.Opts.Debug {
				m.debugEnabled = true
				m.debugConsoleOpen = true
				m.debugAutoFollow = true
				m.dashboard.SetDebugEnabled(true)
				m.updateDashboardLogEntries()
				m.clampDebugScrollForHeight(m.height)
				return m, tea.Batch(cmds...)
			}
			cmds = append(cmds, m.toggleDebugLoggingCmd(true))
			return m, tea.Batch(cmds...)
		}
		// Debug log scrolling (only when panel is visible)
		if m.debugEnabled && m.debugConsoleOpen {
			visible := 3 // Fixed 3-line panel
			maxStart := m.maxDebugScrollStart(visible)

			switch keyStr {
			case "pgup":
				m.debugAutoFollow = false
				m.debugScrollStart -= visible
				if m.debugScrollStart < 0 {
					m.debugScrollStart = 0
				}
				return m, tea.Batch(cmds...)
			case "pgdown":
				m.debugScrollStart += visible
				if m.debugScrollStart >= maxStart {
					m.debugScrollStart = maxStart
					m.debugAutoFollow = true
				}
				return m, tea.Batch(cmds...)
			case "alt+up":
				m.debugAutoFollow = false
				m.debugScrollStart--
				if m.debugScrollStart < 0 {
					m.debugScrollStart = 0
				}
				return m, tea.Batch(cmds...)
			case "alt+down":
				m.debugScrollStart++
				if m.debugScrollStart >= maxStart {
					m.debugScrollStart = maxStart
					m.debugAutoFollow = true
				}
				return m, tea.Batch(cmds...)
			case "home":
				m.debugAutoFollow = false
				m.debugScrollStart = 0
				return m, tea.Batch(cmds...)
			case "end":
				m.debugAutoFollow = true
				m.debugScrollStart = maxStart
				return m, tea.Batch(cmds...)
			}
		}

	case tickMsg:
		if m.pendingPrune && !m.pruneAppliedOnce && !m.applyingPrune &&
			m.embeddedDaemon != nil && m.embeddedDaemon.IsRunning() &&
			m.daemonStatus.Snapshot.Running && m.daemonStatus.Snapshot.IsSynced &&
			m.daemonStatus.InstallPlan == nil && !m.daemonStatus.ConfirmingUninstall &&
			config.GetDaemonSettings().IsPruned() {
			ms := m.embeddedDaemon.MinerStatus()
			m.pruneAppliedOnce = true
			m.applyingPrune = true
			derolog.Info("daemon", "prune.auto_apply", "sync complete; restarting embedded daemon to apply pruning")
			cmds = append(cmds, m.applyPruneRestartCmd(ms.Running, ms.Address, ms.Threads))
		}
		if m.debugEnabled {
			m.updateDashboardLogEntries()
		}
		cmds = append(cmds, m.daemonTickCmd())
		cmds = append(cmds, m.minerStatsCmd()) // Update wallet info periodically
		if m.wallet != nil {
			// Silently re-check balances of scan candidates whose encrypted
			// balances had not synced yet: they appear as soon as the wallet's
			// sync round resolves them, no manual rescan needed.
			if m.page == PageTokens && len(m.tokenScanPending) > 0 && !m.tokenRecheckActive {
				m.tokenRecheckActive = true
				cmds = append(cmds, m.recheckTokenBalancesCmd(dedupeStrings(m.tokenScanPending)))
			}
			cmds = append(cmds, m.updateWalletInfo())
			// Update title to reflect current balance and sync status
			cmds = append(cmds, m.setWindowTitleCmd())

			// Auto-retry daemon connection for open wallet when offline.
			// This recovers from transient websocket failures without requiring
			// manual /connect after wallet open/restore.
			if !m.Opts.Offline && !m.dashboard.IsConnecting {
				info := m.wallet.GetInfo()
				now := time.Now()
				if !info.IsOnline && (m.lastDaemonRetry.IsZero() || now.Sub(m.lastDaemonRetry) >= m.daemonRetryAfter) {
					m.lastDaemonRetry = now
					m.dashboard.SetConnecting(true)
					cmds = append(cmds, m.connectWalletToDaemonAsync())
				}
			}
		} else if m.page == PageWelcome {
			// Refresh daemon status on welcome page
			cmds = append(cmds, m.checkDaemonStatus())
		}
		// HyperGnomon: keep HUD counts fresh and ensure indexer starts at launch
		// without requiring a wallet or an open token menu.
		m.updateHyperDashboard()
		if m.page == PageDiscover {
			cmds = append(cmds, m.maybeLoadDiscoverCatalog(), m.maybeProbeDiscover(), m.maybeHydrateDiscover(), m.maybeFetchDiscoverRatings())
		}
		if m.hyperGnomon == nil || !m.hyperGnomon.IsRunning() {
			if m.cachedDaemonHealthy && m.cachedDaemonAddress != "" {
				cmds = append(cmds, m.ensureHyperGnomonCmd(m.cachedDaemonAddress, m.cachedDaemonNetwork))
			} else if m.wallet != nil {
				endpoint := m.wallet.GetDaemonAddress()
				if endpoint != "" && endpoint != "Not connected" && !m.Opts.Offline {
					netLabel := "Mainnet"
					nt := strings.ToLower(m.wallet.GetNetworkType())
					if nt == "simulator" {
						netLabel = "Simulator"
					} else if nt == "testnet" {
						netLabel = "Testnet"
					}
					cmds = append(cmds, m.ensureHyperGnomonCmd(endpoint, netLabel))
				}
			}
		}
		cmds = append(cmds, m.tickCmd())

	case daemonStatusMsg:
		welcomeDaemons := make([]pages.DaemonStatusInfo, 0, len(msg.daemons))
		for _, daemon := range msg.daemons {
			welcomeDaemons = append(welcomeDaemons, pages.DaemonStatusInfo{
				IsOnline:        daemon.isOnline,
				IsSynced:        daemon.isSynced,
				IsSyncing:       daemon.isSyncing,
				IsBootstrapping: daemon.isBootstrapping,
				IsHealthy:       daemon.isHealthy,
				Network:         daemon.network,
				Address:         daemon.address,
				BlockHeight:     daemon.height,
				PeerHeight:      daemon.peerHeight,
				SyncProgress:    daemon.syncProgress,
			})
		}
		m.welcome.SetDaemonStatuses(welcomeDaemons)

		m.cachedDaemonHealthy = false
		m.cachedDaemonAddress = ""
		m.cachedDaemonNetwork = ""
		if len(msg.daemons) > 0 {
			primary := msg.daemons[0]
			m.cachedDaemonHealthy = primary.isOnline && primary.isHealthy
			m.cachedDaemonAddress = primary.address
			m.cachedDaemonNetwork = primary.network
		}

		m.fillIntegratorFallback()
		if len(msg.daemons) > 0 {
			primary := msg.daemons[0]
			if primary.isOnline && primary.isHealthy {
				cmds = append(cmds, m.ensureHyperGnomonCmd(primary.address, primary.network))
			}
		}

	case daemonManagerMsg:
		if msg.err != "" {
			m.lastEmbeddedError = msg.err
		} else if msg.snapshot.Running {
			m.lastEmbeddedError = ""
		}
		m.applyDaemonManagerMsg(msg)
		if msg.info.IsOnline && msg.info.IsHealthy {
			addr := msg.snapshot.RPCBind
			if addr == "" {
				addr = m.cachedDaemonAddress
			}
			if addr != "" {
				cmds = append(cmds, m.ensureHyperGnomonCmd(addr, msg.info.Network))
			}
		}

	case hyperStartedMsg:
		cmds = append(cmds, m.handleHyperStarted(msg))

	case daemonInstallPreviewMsg:
		m.daemonStatus.Downloading = false
		if msg.err != "" {
			m.daemonStatus.DownloadError = msg.err
		} else {
			// Show the plan and wait for explicit confirmation before touching
			// the system (systemd unit install is not reversible in the TUI).
			m.daemonStatus.SetInstallPlan(&msg.plan)
		}

	case daemonInstallApplyMsg:
		m.daemonStatus.ResetInstall()
		if msg.err != "" {
			m.daemonStatus.DownloadError = msg.err
		} else if msg.userService {
			m.daemonStatus.DownloadError = ""
			m.daemonStatus.InstallResult = "Installed the built-in daemon as a user service"
		} else {
			m.daemonStatus.DownloadError = ""
			m.daemonStatus.InstallResult = "Installed the built-in daemon as a service and started it"
		}

	case daemonInstallApplySudoMsg:
		if msg.err != "" {
			m.daemonStatus.DownloadError = msg.err
		} else {
			m.daemonStatus.DownloadError = ""
			m.daemonStatus.InstallResult = "Installed systemd unit with sudo and reloaded daemon manager"
		}

	case daemonUninstallMsg:
		m.daemonStatus.ResetUninstall()
		if msg.err != "" {
			m.daemonStatus.DownloadError = msg.err
		} else {
			m.daemonStatus.DownloadError = ""
			m.daemonStatus.InstallResult = "Reset complete \u2014 removed " + msg.removed
			cmds = append(cmds, m.daemonTickCmd())
		}

	case daemonConnectMsg:
		m.daemon.SetConnecting(false)
		if msg.err != nil {
			m.daemon.SetError("Failed to connect: " + msg.err.Error())
		} else {
			// Success - update cached daemon address for display purposes only
			// Do NOT modify m.Opts.Testnet/Simulator as those are CLI flags that
			// should remain constant. Wallet network is determined by saved config.
			m.lastWalletDaemon = ""
			m.Opts.DaemonAddress = msg.address
			m.cachedDaemonHealthy = true
			m.cachedDaemonAddress = msg.address
			m.stickyDaemonHealthy = true
			m.stickyDaemonAddress = msg.address
			m.stickyDaemonTestnet = msg.testnet
			m.stickyDaemonSimulator = msg.network == "Simulator"
			m.daemon.Reset()
			m.welcome.SetError("")
			m.welcome.ResetInput()
			m.page = PageWelcome
			cmds = append(cmds, m.checkDaemonStatus())
			cmds = append(cmds, m.setWindowTitleCmd())
		}

	case minerControlMsg:
		if msg.err != "" {
			m.miner.SetError(msg.err)
			m.miner.SetRunning(false)
			m.miner.SetStatus("")
		} else if msg.miner != nil {
			// RPC/engine miner path: the backend is delivered through the
			// message (a command closure cannot mutate the value-receiver
			// model). Store it on the live model so stats/stop can reach it.
			m.rpcMiner = msg.miner
			m.miner.SetRunning(msg.miner.IsRunning())
			m.miner.SetAddress(msg.miner.GetAddress())
			m.miner.SetDaemonHost(msg.miner.GetDaemonHost())
			if msg.miner.GetThreads() > 0 {
				m.miner.SetThreads(msg.miner.GetThreads())
			}
		} else if m.embeddedDaemon != nil {
			miner := m.embeddedDaemon.MinerStatus()
			m.miner.SetRunning(miner.Running)
			if miner.Threads > 0 {
				m.miner.SetThreads(miner.Threads)
			}
			m.miner.SetAddress(miner.Address)
		}
		if m.miner.Running {
			return m, tea.Tick(time.Millisecond*180, func(t time.Time) tea.Msg { return pages.SpinnerTickMsg{} })
		}

	case minerStatsMsg:
		m.miner.SetRunning(msg.running)
		m.miner.SetStats(msg.hashrate, msg.hashes, msg.minis, msg.blocks, msg.rejected, msg.height, msg.difficulty, msg.uptime)
		m.miner.SetThreads(msg.threads)
		m.miner.SetAddress(msg.address)
		m.miner.SetStatus(msg.status)
		m.miner.SetDaemonHost(msg.daemonHost)
		if msg.running {
			return m, tea.Tick(time.Millisecond*180, func(t time.Time) tea.Msg { return pages.SpinnerTickMsg{} })
		}

	case startupCheckMsg:
		// If we have a last wallet, check network first
		if msg.lastWallet != "" {
			// Normalize path to absolute for consistent lookups
			if absPath, err := filepath.Abs(msg.lastWallet); err == nil {
				m.walletFile = filepath.Clean(absPath)
			} else {
				m.walletFile = msg.lastWallet
			}

			// Pre-flight network check for auto-open
			if !m.PreFlightNetworkCheck(m.walletFile) {
				if m.Opts.Password != "" {
					m.pendingPassword = m.Opts.Password
				}
				derolog.Info("wallet", "startup.network_unknown", "Auto-open: Network unknown, showing selection", "wallet", derolog.TruncateAddress(msg.lastWallet))
				m.page = PageNetwork
				m.network = pages.NewNetwork(m.walletFile)
				cmds = append(cmds, m.network.Init())
			} else {
				derolog.Info("wallet", "startup.auto_open", "Auto-open: Network known", "wallet", derolog.TruncateAddress(msg.lastWallet))
				if m.Opts.Password != "" {
					cmds = append(cmds, m.tryOpenWallet(m.walletFile, m.Opts.Password))
				} else {
					m.page = PagePassword
					m.password = pages.NewPassword(pages.PasswordModeUnlock)
					m.password.SetVersion(Version)
					m.password.SetWalletFile(m.walletFile)
					cmds = append(cmds, m.password.Init())
					cmds = append(cmds, m.setWindowTitleCmd())
				}
			}
		}

	case walletOpenedMsg:
		if msg.err != nil {
			derolog.Error("wallet", "open.failed", "Failed to open wallet", "error", msg.err.Error())
			m.password.SetError(msg.err.Error())
		} else {
			// Keep lastWalletDaemon so preferredDaemonAddress can reuse it on
			// the next connect; do not wipe it here (was causing close/reopen
			// to fall back to the localhost daemon).
			network := "mainnet"
			if msg.wallet.IsTestnet() {
				network = "testnet"
			}
			if msg.wallet.IsSimulator() {
				network = "simulator"
			}
			derolog.Info("wallet", "open.success", "Wallet opened successfully", "network", network)
			m.wallet = msg.wallet
			m.regHintShown = false
			m.clearPendingRegistration()
			if err := config.SetLastWallet(m.walletFile); err != nil {
				derolog.Warn("wallet", "config.last_wallet_save_failed", "Failed saving last wallet", "error", err.Error(), "file", m.walletFile)
			}
			if err := config.SetLastMiningAddress(msg.wallet.GetInfo().Address); err != nil {
				derolog.Warn("wallet", "config.mining_address_save_failed", "Failed saving mining address", "error", err.Error())
			}
			// Save network based on wallet's actual network (not CLI flags)
			if msg.wallet.IsSimulator() {
				if err := config.SetWalletNetwork(m.walletFile, config.NetworkSimulator); err != nil {
					derolog.Warn("wallet", "config.network_save_failed", "Failed saving wallet network", "error", err.Error(), "file", m.walletFile, "network", string(config.NetworkSimulator))
				}
			} else if msg.wallet.IsTestnet() {
				if err := config.SetWalletNetwork(m.walletFile, config.NetworkTestnet); err != nil {
					derolog.Warn("wallet", "config.network_save_failed", "Failed saving wallet network", "error", err.Error(), "file", m.walletFile, "network", string(config.NetworkTestnet))
				}
			} else {
				if err := config.SetWalletNetwork(m.walletFile, config.NetworkMainnet); err != nil {
					derolog.Warn("wallet", "config.network_save_failed", "Failed saving wallet network", "error", err.Error(), "file", m.walletFile, "network", string(config.NetworkMainnet))
				}
			}
			// Clear wallet cache and app-level cached state
			// Note: Don't clear walletapi.Daemon_Endpoint_Active here - let ConnectToLocalDaemonFast detect the switch
			m.wallet.ClearDaemonAddress()
			m.cachedDaemonHealthy = false
			m.cachedDaemonAddress = ""
			m.lastDaemonRetry = time.Time{}
			m.daemonRetryAfter = initialDaemonRetryInterval
			m.lastTxRefreshAt = time.Time{}
			m.dashboard.SetConnecting(true) // Always show connecting until daemon connection completes
			m.dashboard.IndexerState = "scanning"
			m.page = PageMain
			// Token discovery starts automatically as soon as the wallet session
			// is connected; the Tokens page only displays the shared results.
			if addr := m.preferredDaemonAddress(); addr != "" {
				cmds = append(cmds, m.ensureHyperGnomonCmd(addr, network))
			}
			m.updateHyperDashboard()
			// Don't call updateWalletInfo() here - wait for daemon connection to complete
			// to avoid showing stale daemon address from previous wallet
			cmds = append(cmds, m.connectWalletToDaemonAsync()) // Connect async
			// Note: m.tickCmd() is already running from Init(); do not add duplicate tickers
			cmds = append(cmds, m.setWindowTitleCmd())
		}

	case walletCreatedMsg:
		if msg.err != nil {
			m.password.SetError(msg.err.Error())
		} else {
			m.wallet = msg.wallet
			m.regHintShown = false
			m.clearPendingRegistration()
			selectedNetwork := m.pendingNetwork
			// Clear create/restore flags
			m.isCreating = false
			m.isRestoringFromSeed = false
			m.isRestoringFromKey = false
			m.pendingNetwork = pages.NetworkNone
			m.pendingCreateRestore = ""
			// Normalize path
			if absPath, err := filepath.Abs(msg.file); err == nil {
				m.walletFile = filepath.Clean(absPath)
			} else {
				m.walletFile = msg.file
			}
			if err := config.SetLastWallet(m.walletFile); err != nil {
				derolog.Warn("wallet", "config.last_wallet_save_failed", "Failed saving last wallet", "error", err.Error(), "file", m.walletFile)
			}
			if err := config.SetLastMiningAddress(msg.wallet.GetInfo().Address); err != nil {
				derolog.Warn("wallet", "config.mining_address_save_failed", "Failed saving mining address", "error", err.Error())
			}
			// Save network using explicit create/restore selection when available.
			networkToSave := config.NetworkMainnet
			switch selectedNetwork {
			case pages.NetworkSimulator:
				networkToSave = config.NetworkSimulator
			case pages.NetworkTestnet:
				networkToSave = config.NetworkTestnet
			case pages.NetworkMainnet:
				networkToSave = config.NetworkMainnet
			default:
				if msg.wallet.IsSimulator() {
					networkToSave = config.NetworkSimulator
				} else if msg.wallet.IsTestnet() {
					networkToSave = config.NetworkTestnet
				}
			}
			if err := config.SetWalletNetwork(m.walletFile, networkToSave); err != nil {
				derolog.Warn("wallet", "config.network_save_failed", "Failed saving wallet network", "error", err.Error(), "file", m.walletFile, "network", string(networkToSave))
			}
			// Clear wallet cache and app-level cached state
			// Note: Don't clear walletapi.Daemon_Endpoint_Active here - let ConnectToLocalDaemonFast detect the switch
			m.wallet.ClearDaemonAddress()
			m.cachedDaemonHealthy = false
			m.cachedDaemonAddress = ""
			m.lastDaemonRetry = time.Time{}
			m.daemonRetryAfter = initialDaemonRetryInterval
			m.lastTxRefreshAt = time.Time{}
			m.dashboard.SetConnecting(true) // Always show connecting until daemon connection completes
			m.seed = pages.NewSeed(pages.SeedModeDisplay, msg.seed)
			m.page = PageSeed
			// Don't call updateWalletInfo() here - wait for daemon connection to complete
			cmds = append(cmds, m.connectWalletToDaemonAsync()) // Connect async in background
			cmds = append(cmds, m.setWindowTitleCmd())
		}

	case walletRestoredMsg:
		if msg.err != nil {
			m.seed.SetError(msg.err.Error())
		} else {
			m.wallet = msg.wallet
			m.regHintShown = false
			m.clearPendingRegistration()
			selectedNetwork := m.pendingNetwork
			// Clear create/restore flags
			m.isCreating = false
			m.isRestoringFromSeed = false
			m.isRestoringFromKey = false
			m.pendingNetwork = pages.NetworkNone
			m.pendingCreateRestore = ""
			// Normalize path
			if absPath, err := filepath.Abs(msg.file); err == nil {
				m.walletFile = filepath.Clean(absPath)
			} else {
				m.walletFile = msg.file
			}
			if err := config.SetLastWallet(m.walletFile); err != nil {
				derolog.Warn("wallet", "config.last_wallet_save_failed", "Failed saving last wallet", "error", err.Error(), "file", m.walletFile)
			}
			if err := config.SetLastMiningAddress(msg.wallet.GetInfo().Address); err != nil {
				derolog.Warn("wallet", "config.mining_address_save_failed", "Failed saving mining address", "error", err.Error())
			}
			// Save network using explicit create/restore selection when available.
			networkToSave := config.NetworkMainnet
			switch selectedNetwork {
			case pages.NetworkSimulator:
				networkToSave = config.NetworkSimulator
			case pages.NetworkTestnet:
				networkToSave = config.NetworkTestnet
			case pages.NetworkMainnet:
				networkToSave = config.NetworkMainnet
			default:
				if msg.wallet.IsSimulator() {
					networkToSave = config.NetworkSimulator
				} else if msg.wallet.IsTestnet() {
					networkToSave = config.NetworkTestnet
				}
			}
			if err := config.SetWalletNetwork(m.walletFile, networkToSave); err != nil {
				derolog.Warn("wallet", "config.network_save_failed", "Failed saving wallet network", "error", err.Error(), "file", m.walletFile, "network", string(networkToSave))
			}
			// Clear wallet cache and app-level cached state
			// Note: Don't clear walletapi.Daemon_Endpoint_Active here - let ConnectToLocalDaemonFast detect the switch
			m.wallet.ClearDaemonAddress()
			m.cachedDaemonHealthy = false
			m.cachedDaemonAddress = ""
			m.lastDaemonRetry = time.Time{}
			m.daemonRetryAfter = initialDaemonRetryInterval
			m.lastTxRefreshAt = time.Time{}
			m.dashboard.SetConnecting(true) // Always show connecting until daemon connection completes
			m.page = PageMain
			// Don't call updateWalletInfo() here - wait for daemon connection to complete
			cmds = append(cmds, m.connectWalletToDaemonAsync()) // Connect async
			// Note: m.tickCmd() is already running from Init(); do not add duplicate tickers
			cmds = append(cmds, m.setWindowTitleCmd())
		}

	case transferResultMsg:
		if msg.err != "" {
			derolog.Error("transfer", "failed", "Transfer failed", "error", msg.err)
			m.send.SetError(msg.err)
		} else if msg.txID == "" {
			// No error but no txID either - something went wrong
			derolog.Error("transfer", "failed", "Transfer failed: no transaction ID returned")
			m.send.SetError("Transfer failed: no transaction ID returned")
		} else {
			// Transfer successful - mark result ready (animation continues until min duration)
			derolog.Info("transfer", "success", "Transfer successful", "txid", derolog.TruncateID(msg.txID))
			m.send.SetSuccess(msg.txID)
			m.addPendingOutgoingTx(msg.txID, msg.amountAtomic, msg.destination)
			mergedTxs := m.mergePendingOutgoing(m.history.Transactions)
			m.dashboard.SetRecentTxs(mergedTxs)
			m.history.SetTransactions(mergedTxs)
		}

	case registrationResultMsg:
		m.dashboard.SetRegistering(false)
		if msg.err != "" {
			m.clearPendingRegistration()
			m.dashboard.SetFlashMessage("Registration failed: "+msg.err, false)
			break
		}
		if msg.alreadyRegistered {
			m.clearPendingRegistration()
			m.dashboard.SetFlashMessage("Wallet is already registered", true)
			cmds = append(cmds, m.updateWalletInfo())
			break
		}
		if msg.txID != "" {
			var startHeight uint64
			if m.wallet != nil {
				startHeight = m.wallet.GetInfo().DaemonHeight
			}
			m.regHintShown = true
			m.pendingRegTxID = msg.txID
			m.pendingRegStatus = "submitted"
			m.pendingRegHeight = startHeight
			m.dashboard.SetRegistrationPending(msg.txID, "submitted")
			m.dashboard.SetFlashMessage("Registration TX sent: "+msg.txID, true)
		} else {
			m.dashboard.SetFlashMessage("Registration transaction dispatched", true)
		}
		cmds = append(cmds, m.updateWalletInfo())

	case walletDataMsg:
		m.applyWalletData(msg)

	case regPollMsg:
		m.applyRegPoll(msg)

	case namesLoadedMsg:
		if m.page != PageNames {
			break
		}
		if msg.err != "" {
			m.names.SetError(msg.err)
		} else {
			m.names.SetNames(msg.names)
		}

	case nameRegisterResultMsg:
		if m.page != PageNameRegister {
			break
		}
		if msg.err != "" {
			m.nameRegister.SetError(msg.err)
		} else {
			m.nameRegister.Reset()
			m.names.SetFlash("Name registered successfully", true)
			m.page = PageNames
			m.names.SetLoading(true)
			cmds = append(cmds, m.setWindowTitleCmd(), m.loadNamesCmd())
		}

	case nameTransferResultMsg:
		if m.page != PageNameTransfer {
			break
		}
		if msg.err != "" {
			m.nameTransfer.SetError(msg.err)
		} else {
			m.nameTransfer.Reset()
			m.names.SetFlash(msg.txID+" successfully", true)
			m.page = PageNames
			m.names.SetLoading(true)
			cmds = append(cmds, m.setWindowTitleCmd(), m.loadNamesCmd())
		}

	case tokenScanProgressMsg:
		if msg.id != 0 && msg.id != m.tokenScanID {
			break
		}
		if msg.err != "" {
			m.tokenScanActive = false
			m.tokens.SetScanning(false, "")
			m.tokens.SetError(msg.err)
			break
		}
		if msg.candidates != nil {
			m.tokenScanCandidates = msg.candidates
			if msg.index == 0 {
				// Fresh scan: reset accumulated state.
				m.tokenScanFound = 0
				m.tokenScanPending = nil
			}
		}
		if len(msg.found) > 0 {
			m.tokenScanFound += len(msg.found)
			m.mergeDiscoveredTokens(msg.found)
			if cmd := m.hydrateTokenMetadataCmd(msg.found); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		if len(msg.zero) > 0 {
			m.tokenScanPending = append(m.tokenScanPending, msg.zero...)
		}
		total := len(m.tokenScanCandidates)
		if msg.index >= total {
			m.tokenScanActive = false
			m.tokens.SetScanning(false, "")
			if m.wallet != nil && total > 0 {
				height := m.wallet.GetInfo().DaemonHeight
				entries := make([]config.DiscoveredSCID, 0, len(m.tokens.Tokens()))
				for _, t := range m.tokens.Tokens() {
					if t.Balance > 0 {
						entries = append(entries, config.DiscoveredSCID{SCID: t.SCID, Source: "wallet", LastCheckedHeight: height, LastSeen: time.Now()})
					}
				}
				if len(entries) > 0 {
					_ = config.MergeDiscoveredSCIDs(m.walletFile, entries)
				}
			}
			m.tokens.SetFlash(fmt.Sprintf("Scan complete: %d %s with balance (%d candidates checked)",
				m.tokenScanFound, plural(m.tokenScanFound, "token"), total), true)
			ms := int64(0)
			if !m.tokenScanStartedAt.IsZero() {
				ms = time.Since(m.tokenScanStartedAt).Milliseconds()
			}
			derolog.Info("token", "scan.complete", "token scan finished",
				"found", fmt.Sprintf("%d", m.tokenScanFound),
				"candidates", fmt.Sprintf("%d", total),
				"duration_ms", fmt.Sprintf("%d", ms))
			m.updateHyperDashboard()
			break
		}
		m.tokens.SetScanning(true, fmt.Sprintf("Checking %d/%d — %d %s found",
			msg.index+1, total, m.tokenScanFound, plural(m.tokenScanFound, "token")))
		cmds = append(cmds, m.tokenScanStepCmd(m.tokenScanCandidates, msg.index))

	case tokenBalanceRefreshMsg:
		m.tokenRecheckActive = false
		m.tokenScanPending = msg.pending
		if len(msg.found) > 0 {
			m.mergeDiscoveredTokens(msg.found)
			if cmd := m.hydrateTokenMetadataCmd(msg.found); cmd != nil {
				cmds = append(cmds, cmd)
			}
			if m.wallet != nil {
				height := m.wallet.GetInfo().DaemonHeight
				entries := make([]config.DiscoveredSCID, 0, len(msg.found))
				for _, t := range msg.found {
					entries = append(entries, config.DiscoveredSCID{SCID: t.SCID, Source: "wallet", LastCheckedHeight: height, LastSeen: time.Now()})
				}
				_ = config.MergeDiscoveredSCIDs(m.walletFile, entries)
			}
			m.tokens.SetFlash(fmt.Sprintf("Synced: %d new %s appeared", len(msg.found), plural(len(msg.found), "token")), true)
		}

	case tokenMetadataMsg:
		m.mergeDiscoveredTokens(msg.tokens)

	case discoverNamesMsg:
		m.discoverHydrating = false
		m.discover.ApplyNames(msg.names)
		if cmd := m.maybeHydrateDiscover(); cmd != nil {
			cmds = append(cmds, cmd)
		}

	case telaLaunchMsg:
		note := msg.err
		if m.xswdBridge != nil && !m.xswdBridge.EpochRunning() {
			e := "EPOCH not connected"
			if err := m.xswdBridge.EpochError(); err != nil {
				e = "EPOCH: " + err.Error()
			}
			if note != "" {
				note = note + "; " + e
			} else {
				note = e
			}
		}
		m.discover.SetLaunchResult(msg.link, note)

	case discoverCatalogMsg:
		m.handleDiscoverCatalog(msg)

	case discoverRatingsMsg:
		m.discoverRatingsLoading = false
		m.discover.ApplyRatings(msg.ratings)
		if cmd := m.maybeFetchDiscoverRatings(); cmd != nil {
			cmds = append(cmds, cmd)
		}

	case discoverOwnedMsg:
		m.discoverProbing = false
		m.discoverOwnedDone = true
		m.discover.SetProbing(false)
		m.discover.SetOwned(msg.nft, msg.nfa)
		if cmd := m.maybeHydrateDiscover(); cmd != nil {
			cmds = append(cmds, cmd)
		}

	case tokensLoadedMsg:
		m.tokens.SetLoading(false)
		if msg.err != "" {
			if m.page == PageTokens {
				m.tokens.SetError(msg.err)
			}
		} else {
			m.mergeDiscoveredTokens(msg.tokens)
		}
	case tokenAddResultMsg:
		if msg.err != "" {
			m.tokens.SetError(msg.err)
			m.tokens.SetFlash(msg.err, false)
		} else {
			m.tokens.SetFlash("Token added: "+msg.scid[:8]+"...", true)
			m.tokens.SetLoading(false)
			m.tokenScanActive = true
			m.tokenScanFound = 0
			cmds = append(cmds, m.loadTokensCmd(), m.tokenScanStartCmd(false))
		}
	case tokenSendResultMsg:
		if msg.err != "" {
			m.tokenSend.SetError(msg.err)
		} else if msg.txID == "" {
			m.tokenSend.SetError("Transfer failed: no transaction ID")
		} else {
			m.tokenSend.SetSuccess(msg.txID)
		}
	case tokenHistoryLoadedMsg:
		if msg.err != "" {
			m.tokenHistory.SetTransactions(nil)
		} else {
			var txs []pages.Transaction
			for _, tx := range msg.txs {
				txs = append(txs, pages.Transaction{
					TxID:      tx.TxID,
					Amount:    tx.Amount,
					Height:    tx.Height,
					Timestamp: wallet.FormatTimestamp(tx.Timestamp),
					Incoming:  tx.Incoming,
					Message:   tx.Message,
				})
			}
			m.tokenHistory.SetTransactions(txs)
		}

	case passwordChangedMsg:
		if msg.err != nil {
			m.password.SetError(msg.err.Error())
			return m, nil // Return early - don't process further to avoid error being cleared
		} else {
			// Password changed successfully - go back to dashboard with flash message
			m.password.Reset()
			m.isChangingPassword = false
			m.page = PageMain
			m.dashboard.SetFlashMessage("Password changed successfully", true)
			cmds = append(cmds, m.setWindowTitleCmd())
		}

	case walletDaemonConnectedMsg:
		// Async daemon connection completed
		m.dashboard.SetConnecting(false)
		if msg.connected {
			m.lastDaemonRetry = time.Time{}
			m.daemonRetryAfter = initialDaemonRetryInterval
			cmds = append(cmds, m.updateWalletInfo()) // Refresh balance/status now that we're connected
			// Update daemon address to match what wallet connected to
			if msg.daemonAddress != "" {
				m.cachedDaemonAddress = msg.daemonAddress
				m.cachedDaemonHealthy = true
				m.cachedDaemonNetwork = string(msg.network)
			}
			addr := msg.daemonAddress
			if addr == "" {
				addr = m.cachedDaemonAddress
			}
			netLabel := string(msg.network)
			if netLabel == "" && m.wallet != nil {
				netLabel = m.wallet.GetNetworkType()
			}
			if addr != "" {
				cmds = append(cmds, m.ensureHyperGnomonCmd(addr, netLabel))
			}
			m.updateHyperDashboard()
			if m.wallet != nil && !m.tokenScanActive {
				m.tokenScanActive = true
				m.tokenScanFound = 0
				if m.page == PageTokens {
					m.tokens.SetScanning(true, "Preparing scan...")
					cmds = append(cmds, m.loadTokensCmd())
				}
				cmds = append(cmds, m.tokenScanStartCmd(false))
			}
			// Keep global debug state stable across page transitions.
			// Only sync dashboard indicator from current global state.
			m.dashboard.SetDebugEnabled(m.debugEnabled)
			// Note: XSWD no longer auto-starts - user must toggle it manually via dashboard
			if m.wallet != nil {
				info := m.wallet.GetInfo()
				if !info.IsRegistered && !m.regHintShown {
					m.dashboard.SetFlashMessage("Wallet not registered. Press [G] to register.", false)
					m.regHintShown = true
				}
			}
		} else if msg.err != "" {
			if m.daemonRetryAfter <= 0 {
				m.daemonRetryAfter = initialDaemonRetryInterval
			} else {
				m.daemonRetryAfter *= 2
				if m.daemonRetryAfter > maxDaemonRetryInterval {
					m.daemonRetryAfter = maxDaemonRetryInterval
				}
			}
			// Daemon connection failed - keep wallet OPEN and show offline.
			// Do NOT force-navigate back to the dashboard: this handler also
			// runs on the periodic auto-retry loop (every few seconds while
			// offline), so a page assignment here yanks the user off whatever
			// page they're on (send, history, seed, settings...) and is the
			// "backs out by itself" bug. The dashboard is already the landing
			// page when the initial connect fails; retries only refresh its
			// offline state and leave the current page alone.
			m.dashboard.SetFlashMessage(msg.err+" - Wallet opened offline. Use /connect to retry.", false)
			if m.wallet != nil {
				m.dashboard.SetWalletInfo(m.wallet.GetFileName(), m.wallet.GetNetworkType(), false, false, false, m.wallet.GetDaemonAddress(), 0, 0)
			}
			m.dashboard.SetConnecting(false)
			// Update wallet info to show offline status
			cmds = append(cmds, m.updateWalletInfo())
		}

	case networkRequiredMsg:
		// Wallet needs network selection - store password and show network page
		m.pendingPassword = msg.password
		// Normalize path
		if absPath, err := filepath.Abs(msg.file); err == nil {
			m.walletFile = filepath.Clean(absPath)
		} else {
			m.walletFile = msg.file
		}
		m.network = pages.NewNetwork(m.walletFile)
		m.page = PageNetwork

	case wallet.XSWDAuthRequest:
		derolog.Info("xswd", "auth.request", "XSWD auth request received", "app", msg.App.Name)
		// Reject if already handling an XSWD dialog
		if m.page == PageXSWDAuth || m.page == PageXSWDPerm {
			derolog.Warn("xswd", "auth.rejected", "Rejecting XSWD app (already in dialog)", "app", msg.App.Name)
			msg.Response <- false
			break
		}
		m.xswdPrevPage = m.page
		m.xswdAuthCh = msg.Response
		m.xswdAuth = pages.NewXSWDAuth(msg.App.Name, msg.App.Description, msg.App.URL, msg.App.ID)
		m.page = PageXSWDAuth
		cmds = append(cmds, m.setWindowTitleCmd(), m.xswdDialogTimeoutCmd())

	case wallet.XSWDPermissionRequest:
		derolog.Info("xswd", "perm.request", "XSWD permission request received", "app", msg.Perm.AppName, "method", msg.Perm.Method)
		// Reject if already handling an XSWD dialog
		if m.page == PageXSWDAuth || m.page == PageXSWDPerm {
			derolog.Warn("xswd", "perm.rejected", "Denying XSWD permission (already in dialog)", "app", msg.Perm.AppName)
			msg.Response <- wallet.XSWDPermDeny
			break
		}
		m.xswdPrevPage = m.page
		m.xswdPermCh = msg.Response
		m.xswdPerm = pages.NewXSWDPerm(msg.Perm.AppName, msg.Perm.Method)
		m.page = PageXSWDPerm
		cmds = append(cmds, m.setWindowTitleCmd(), m.xswdDialogTimeoutCmd())

	case xswdDialogTimeoutMsg:
		// Dismiss an XSWD dialog after the server-side timeout, denying the
		// request, so the UI never stays stuck on a dialog.
		switch m.page {
		case PageXSWDAuth:
			if m.xswdAuthCh != nil {
				m.xswdAuthCh <- false
				m.xswdAuthCh = nil
			}
			m.xswdAuth.Reset()
			m.page = m.xswdPrevPage
		case PageXSWDPerm:
			if m.xswdPermCh != nil {
				m.xswdPermCh <- wallet.XSWDPermDeny
				m.xswdPermCh = nil
			}
			m.xswdPerm.Reset()
			m.page = m.xswdPrevPage
		}
		cmds = append(cmds, m.setWindowTitleCmd())

	case wallet.XSWDStartedMsg:
		if msg.Err != nil {
			derolog.Error("xswd", "start.failed", "XSWD server failed to start", "error", msg.Err.Error())
			if m.page == PageMain {
				m.dashboard.SetXSWDRunning(false)
				m.dashboard.SetFlashMessage("XSWD: "+msg.Err.Error(), false)
			}
			if m.page == PageDiscover {
				m.discover.SetLaunchResult("", "XSWD: "+msg.Err.Error())
			}
		} else {
			derolog.Info("xswd", "start.success", "XSWD server started")
			m.xswdBridge = msg.Bridge
			m.dashboard.SetXSWDRunning(true)
			if msg.Bridge != nil && msg.Bridge.EpochRunning() {
				m.dashboard.SetFlashMessage("XSWD + EPOCH running", true)
			} else {
				why := "not connected"
				if msg.Bridge != nil {
					if err := msg.Bridge.EpochError(); err != nil {
						why = err.Error()
					}
				}
				m.dashboard.SetFlashMessage("XSWD running (EPOCH failed: "+why+")", false)
			}
		}

	case debugToggleResultMsg:
		if msg.err != nil {
			if m.page == PageWelcome {
				m.welcome.SetError("Debug: " + msg.err.Error())
			} else {
				m.dashboard.SetFlashMessage("Debug: "+msg.err.Error(), false)
			}
			break
		}

		m.Opts.Debug = msg.enabled
		m.debugEnabled = msg.enabled
		m.debugConsoleOpen = msg.open
		m.debugAutoFollow = msg.enabled
		m.debugScrollStart = 0
		m.debugLastClickY = -1
		m.debugLastClickAt = time.Time{}
		m.dashboard.SetDebugEnabled(msg.enabled)

		if msg.enabled {
			m.updateDashboardLogEntries()
			m.clampDebugScrollForHeight(m.height)
			if m.page == PageWelcome {
				m.welcome.SetError("")
			} else {
				m.dashboard.SetFlashMessage("Debug logging enabled: "+msg.logPath, true)
			}
		} else {
			m.debugLogEntries = nil
			m.debugConsoleOpen = false
			m.debugAutoFollow = false
			m.debugScrollStart = 0
			m.debugLastClickY = -1
			m.debugLastClickAt = time.Time{}
			if m.page == PageWelcome {
				m.welcome.SetError("")
			} else {
				m.dashboard.SetFlashMessage("Debug logging disabled", true)
			}
		}

	}

	// Route to current page
	before := m.page
	result, cmd := m.dispatchPage(msg, cmds)
	if result.page != before {
		// Diagnostic: every page transition is logged so an unexpected jump
		// back to welcome can be traced in the F12 debug console and the file.
		derolog.Info("ui", "page.transition", fmt.Sprintf("Page %d -> %d", before, result.page),
			"trigger", fmt.Sprintf("%T", msg), "page", fmt.Sprintf("%d", result.page))
	}
	return result, cmd
}

// shutdownSession unhooks session state immediately so Update can return.
// Wallet Close, XSWD Stop, and HyperGnomon Close run in the returned cmd —
// they block on derohe/bbolt and used to freeze the TUI when called inline.
func (m *Model) shutdownSession(quitting bool) tea.Cmd {
	if m.xswdAuthCh != nil {
		select {
		case m.xswdAuthCh <- false:
		default:
		}
		m.xswdAuthCh = nil
	}
	if m.xswdPermCh != nil {
		select {
		case m.xswdPermCh <- wallet.XSWDPermDeny:
		default:
		}
		m.xswdPermCh = nil
	}

	bridge := m.xswdBridge
	m.xswdBridge = nil
	if bridge != nil {
		m.dashboard.SetXSWDRunning(false)
	}

	w := m.wallet
	m.wallet = nil
	if w != nil && !quitting {
		m.lastWalletDaemon = w.GetDaemonAddress()
		m.cachedDaemonHealthy = false
		m.cachedDaemonAddress = ""
		m.cachedDaemonNetwork = ""
		m.Opts.DaemonAddress = m.cliDaemonAddress
		m.regHintShown = false
		m.clearPendingRegistration()
	}

	var hyper *wallet.HyperGnomon
	if quitting {
		m.quitting = true
		if m.hyperMu == nil {
			m.hyperMu = &sync.Mutex{}
		}
		m.hyperMu.Lock()
		hyper = m.hyperGnomon
		m.hyperGnomon = nil
		m.hyperMu.Unlock()
	}

	return func() tea.Msg {
		wallet.ShutdownTela()
		if bridge != nil {
			bridge.Stop()
		}
		if w != nil {
			w.Close()
		}
		if quitting {
			if hyper != nil {
				hyper.Close()
			}
			derolog.Close()
			return tea.Quit()
		}
		return nil
	}
}

// ensureHyperGnomonCmd returns a command that performs the expensive
// HyperGnomon startup off the UI thread and reports the result via
// hyperStartedMsg. Cheap pre-checks run synchronously so a healthy,
// matching indexer never spawns a command.
func (m *Model) ensureHyperGnomonCmd(endpoint, network string) tea.Cmd {
	if m.Opts.Offline {
		return nil
	}
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" || endpoint == "Not connected" {
		return nil
	}
	if strings.TrimSpace(network) == "" {
		network = "Mainnet"
	}
	normNet := strings.ToLower(strings.TrimSpace(network))
	if m.hyperMu == nil {
		m.hyperMu = &sync.Mutex{}
	}
	m.hyperMu.Lock()
	if m.hyperGnomon != nil && m.hyperGnomon.IsRunning() {
		if strings.EqualFold(m.hyperGnomon.Network(), normNet) && m.hyperGnomon.Endpoint() == endpoint {
			m.hyperMu.Unlock()
			m.stampHyperHUD(m.hyperGnomon)
			return nil
		}
		m.hyperGnomon.Close()
		m.hyperGnomon = nil
	}
	m.hyperMu.Unlock()

	return func() tea.Msg {
		start := time.Now()
		// Bound the whole startup: NewHyperGnomon opens bbolt with
		// Timeout: 0 (wait indefinitely for the file lock), so a second app
		// instance — or a stale test process — holding
		// ~/.derotui/hypergnomon/<net>/HYPERGNOMON.db would block forever.
		// A hung startup must surface as an error msg, not a frozen cmd.
		const startupTimeout = 15 * time.Second
		resultCh := make(chan hyperStartedMsg, 1)
		go func() {
			h, err := wallet.NewHyperGnomon(endpoint, normNet, "", 8)
			if err != nil {
				derolog.Warn("hypergnomon", "start.failed", "failed to start HyperGnomon", "error", err.Error(), "endpoint", endpoint, "network", normNet)
				resultCh <- hyperStartedMsg{err: err.Error(), network: normNet}
				return
			}
			derolog.Info("hypergnomon", "scan.start", "HyperGnomon started", "endpoint", endpoint, "network", normNet, "startup_ms", fmt.Sprintf("%d", time.Since(start).Milliseconds()))
			resultCh <- hyperStartedMsg{hyper: h, network: normNet}
		}()
		select {
		case msg := <-resultCh:
			return msg
		case <-time.After(startupTimeout):
			derolog.Warn("hypergnomon", "start.timeout", "HyperGnomon startup timed out (bbolt lock held or daemon unreachable)", "endpoint", endpoint, "network", normNet)
			return hyperStartedMsg{err: fmt.Sprintf("HyperGnomon startup timed out after %s (bbolt lock held or daemon unreachable)", startupTimeout), network: normNet}
		}
	}
}

// hyperStartedMsg handler: attach the freshly started indexer, stamp the HUD
// from its first cached sample, and kick the token scan if a wallet session
// is ready. Runs inside Update but only does cheap pointer assignment; the
// heavy startup already happened inside the command.
func (m *Model) handleHyperStarted(msg hyperStartedMsg) tea.Cmd {
	if msg.err != "" {
		return nil
	}
	if m.hyperMu == nil {
		m.hyperMu = &sync.Mutex{}
	}
	m.hyperMu.Lock()
	already := m.hyperGnomon != nil && m.hyperGnomon.IsRunning()
	if !already {
		m.hyperGnomon = msg.hyper
	}
	m.hyperMu.Unlock()
	if already {
		return nil
	}
	m.hyperCompleteLogged = false
	m.stampHyperHUD(msg.hyper)
	if m.wallet != nil && !m.tokenScanActive {
		m.tokenScanActive = true
		m.tokenScanFound = 0
		if m.page == PageTokens {
			m.tokens.SetScanning(true, "Preparing scan...")
			return tea.Batch(m.loadTokensCmd(), m.tokenScanStartCmd(false))
		}
		return m.tokenScanStartCmd(false)
	}
	return nil
}

func (m *Model) stampHyperHUD(h *wallet.HyperGnomon) {
	if h == nil {
		return
	}
	// Progress() is served from background-refreshed atomics (see
	// wallet.HyperGnomon.pollProgress), so stamping the HUD never performs a
	// synchronous bbolt owners-bucket scan inside Update.
	scids, last, chain, _ := h.Progress()
	state := "scanning"
	if chain > 0 && last+2 >= chain {
		state = "complete"
	}
	if state == "complete" && !m.hyperCompleteLogged {
		m.hyperCompleteLogged = true
		ms := int64(0)
		if !h.StartedAt().IsZero() {
			ms = time.Since(h.StartedAt()).Milliseconds()
		}
		derolog.Info("hypergnomon", "scan.complete", "HyperGnomon caught up",
			"scids", fmt.Sprintf("%d", scids),
			"last", fmt.Sprintf("%d", last),
			"chain", fmt.Sprintf("%d", chain),
			"duration_ms", fmt.Sprintf("%d", ms))
	}
	m.dashboard.SetIndexerProgress(scids, state)
}

func (m *Model) mergeDiscoveredTokens(tokens []wallet.TokenInfo) {
	existing := m.tokens.Tokens()
	bySCID := make(map[string]wallet.TokenInfo, len(existing)+len(tokens))
	for _, t := range existing {
		bySCID[strings.ToLower(t.SCID)] = t
	}
	for _, t := range tokens {
		key := strings.ToLower(t.SCID)
		cur, ok := bySCID[key]
		if !ok {
			bySCID[key] = t
			continue
		}
		if t.Name != "" {
			cur.Name = t.Name
		}
		if t.Ticker != "" {
			cur.Ticker = t.Ticker
		}
		if t.Decimals != 0 {
			cur.Decimals = t.Decimals
		}
		if t.Balance != 0 {
			cur.Balance = t.Balance
		}
		bySCID[key] = cur
	}
	merged := make([]wallet.TokenInfo, 0, len(bySCID))
	for _, v := range bySCID {
		merged = append(merged, v)
	}
	sort.Slice(merged, func(i, j int) bool {
		ki, kj := tokenSortKey(merged[i]), tokenSortKey(merged[j])
		if ki != kj {
			return ki < kj
		}
		return merged[i].SCID < merged[j].SCID
	})
	m.tokens.SetTokens(merged)
}

// tokenSortKey returns the primary display label used to order the token list.
func tokenSortKey(t wallet.TokenInfo) string {
	switch {
	case t.Ticker != "":
		return strings.ToLower(t.Ticker)
	case t.Name != "":
		return strings.ToLower(t.Name)
	default:
		return t.SCID
	}
}

// plural returns word with an "s" appended unless n == 1.
func plural(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}

// dedupeStrings returns s without duplicates, preserving order.
func dedupeStrings(s []string) []string {
	seen := make(map[string]bool, len(s))
	out := s[:0]
	for _, v := range s {
		if seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

func (m *Model) closeHyperGnomon() {
	if m.hyperMu == nil {
		m.hyperMu = &sync.Mutex{}
	}
	m.hyperMu.Lock()
	defer m.hyperMu.Unlock()
	if m.hyperGnomon != nil {
		m.hyperGnomon.Close()
		m.hyperGnomon = nil
	}
}

func (m *Model) hyperProgress() (scids int, lastHeight int64, chainHeight int64, status string) {
	if m.hyperMu == nil {
		m.hyperMu = &sync.Mutex{}
	}
	m.hyperMu.Lock()
	h := m.hyperGnomon
	m.hyperMu.Unlock()
	if h == nil {
		return 0, 0, 0, ""
	}
	return h.Progress()
}

func (m *Model) hyperSCIDs() []string {
	if m.hyperMu == nil {
		m.hyperMu = &sync.Mutex{}
	}
	m.hyperMu.Lock()
	h := m.hyperGnomon
	m.hyperMu.Unlock()
	if h == nil {
		return nil
	}
	return h.SCIDs()
}

type discoverNamesMsg struct {
	names map[string]string
}

type discoverRatingsMsg struct {
	ratings map[string]wallet.CatalogEntry
}

func (m *Model) maybeHydrateDiscover() tea.Cmd {
	if m.page != PageDiscover || m.discoverHydrating {
		return nil
	}
	need := m.discover.UnnamedVisible()
	if m.discoverTried == nil {
		m.discoverTried = map[string]bool{}
	}
	var scids []string
	for _, s := range need {
		if !m.discoverTried[s] {
			scids = append(scids, s)
		}
	}
	if len(scids) == 0 {
		return nil
	}
	endpoint := m.cachedDaemonAddress
	if m.hyperMu == nil {
		m.hyperMu = &sync.Mutex{}
	}
	m.hyperMu.Lock()
	h := m.hyperGnomon
	m.hyperMu.Unlock()
	if h != nil && h.Endpoint() != "" {
		endpoint = h.Endpoint()
	}
	if endpoint == "" {
		return nil
	}
	m.discoverHydrating = true
	for _, s := range scids {
		m.discoverTried[s] = true
	}
	return func() tea.Msg {
		names := map[string]string{}
		for _, s := range scids {
			if n := wallet.LookupSCName(endpoint, s); n != "" {
				names[s] = n
			}
		}
		return discoverNamesMsg{names: names}
	}
}

// maybeFetchDiscoverRatings returns a background command that enriches the
// visible TELA rows with TELA rating data. Ratings live in bbolt, and the
// indexer can hold its write lock for long stretches — so this MUST run off
// the UI thread (a synchronous per-row fetch in the tick path froze the app).
func (m *Model) maybeFetchDiscoverRatings() tea.Cmd {
	if m.page != PageDiscover || m.discoverRatingsLoading {
		return nil
	}
	m.hyperMu.Lock()
	h := m.hyperGnomon
	m.hyperMu.Unlock()
	if h == nil {
		return nil
	}
	scids := m.discover.VisibleSCIDs()
	if len(scids) == 0 {
		return nil
	}
	m.discoverRatingsLoading = true
	return func() tea.Msg {
		ratings := h.RatingsForSCIDs(scids)
		return discoverRatingsMsg{ratings: ratings}
	}
}

type discoverOwnedMsg struct {
	nft []wallet.CatalogEntry
	nfa []wallet.CatalogEntry
}

func (m *Model) maybeProbeDiscover() tea.Cmd {
	if m.page != PageDiscover || m.discoverProbing || m.discoverOwnedDone || m.wallet == nil {
		return nil
	}
	if m.hyperMu == nil {
		m.hyperMu = &sync.Mutex{}
	}
	m.hyperMu.Lock()
	h := m.hyperGnomon
	m.hyperMu.Unlock()
	if h == nil {
		return nil
	}
	addr := m.wallet.GetAddress()
	touched := m.hyperAddressSCIDs(addr)
	nftCands := wallet.FilterCatalogBySCIDs(h.Catalog("G45-NFT"), touched)
	nfaCands := wallet.FilterCatalogBySCIDs(h.Catalog("NFA"), touched)
	if len(nftCands)+len(nfaCands) == 0 {
		if m.discover.Classifying() {
			return nil
		}
		m.discoverOwnedDone = true
		m.discover.SetProbing(false)
		m.discover.SetOwned(nil, nil)
		return nil
	}
	endpoint := h.Endpoint()
	if endpoint == "" {
		endpoint = m.cachedDaemonAddress
	}
	w := m.wallet
	m.discoverProbing = true
	m.discover.SetProbing(true)
	return func() tea.Msg {
		keep := func(cands []wallet.CatalogEntry) []wallet.CatalogEntry {
			var out []wallet.CatalogEntry
			for _, e := range cands {
				if discoverAssetOwned(w, endpoint, e.SCID, addr) {
					out = append(out, e)
				}
			}
			return out
		}
		return discoverOwnedMsg{nft: keep(nftCands), nfa: keep(nfaCands)}
	}
}

func discoverAssetOwned(w *wallet.Wallet, endpoint, scid, addr string) bool {
	if w != nil {
		if bal, err := w.ProbeTokenBalance(scid); err == nil && bal > 0 {
			return true
		}
	}
	owner := wallet.LookupSCOwner(endpoint, scid)
	return owner != "" && strings.EqualFold(owner, addr)
}

// discoverCatalogMsg carries the result of the background catalog load.
type discoverCatalogMsg struct {
	tela        []wallet.CatalogEntry
	classifying bool
}

// maybeLoadDiscoverCatalog returns a background command that loads the TELA
// catalog off the UI thread. Catalog()/SCIDsByClass() each open bbolt read
// transactions — with the indexer holding write locks for long stretches
// (fastsync batches), a synchronous call in the tick path froze the whole UI
// and made Ctrl+C unresponsive. Results land via discoverCatalogMsg.
func (m *Model) maybeLoadDiscoverCatalog() tea.Cmd {
	if m.page != PageDiscover || m.discoverCatalogLoading {
		return nil
	}
	m.hyperMu.Lock()
	h := m.hyperGnomon
	m.hyperMu.Unlock()
	if h == nil {
		return nil
	}
	m.discoverCatalogLoading = true
	return func() tea.Msg {
		tela := wallet.FilterLaunchableTela(h.Catalog("TELA-INDEX-1"))
		classCount := len(h.SCIDsByClass("TELA-INDEX-1")) + len(h.SCIDsByClass("G45-NFT")) + len(h.SCIDsByClass("NFA"))
		classifying := h.Count() > 0 && classCount == 0
		return discoverCatalogMsg{tela: tela, classifying: classifying}
	}
}

// handleDiscoverCatalog merges the background catalog load into the page.
func (m *Model) handleDiscoverCatalog(msg discoverCatalogMsg) {
	m.discoverCatalogLoading = false
	m.discover.SetTela(msg.tela, msg.classifying, m.wallet == nil)
}

func (m *Model) hyperTokenLikeSCIDs() []string {
	if m.hyperMu == nil {
		m.hyperMu = &sync.Mutex{}
	}
	m.hyperMu.Lock()
	h := m.hyperGnomon
	m.hyperMu.Unlock()
	if h == nil {
		return nil
	}
	return h.TokenLikeSCIDs()
}

func (m *Model) hasHyperRunning() bool {
	if m.hyperMu == nil {
		m.hyperMu = &sync.Mutex{}
	}
	m.hyperMu.Lock()
	defer m.hyperMu.Unlock()
	return m.hyperGnomon != nil && m.hyperGnomon.IsRunning()
}

func (m *Model) updateHyperDashboard() {
	if m.hyperMu == nil {
		m.hyperMu = &sync.Mutex{}
	}
	m.hyperMu.Lock()
	h := m.hyperGnomon
	m.hyperMu.Unlock()
	if h == nil || !h.IsRunning() {
		return
	}
	m.stampHyperHUD(h)
}

// paletteEnabled reports whether the "/" command palette is available on the
// given page. It is disabled on pages with text inputs or already-interactive
// menus so "/" never gets stolen from typed input.
func paletteEnabled(page Page) bool {
	switch page {
	case PageMain, PageMiner, PageNames, PageTokens, PageTokenHistory, PageDiscover, PageDaemonStatus, PageDaemonLogs, PageDaemonSettings, PageHistory, PageTxDetails, PageQRCode, PageLogo:
		return true
	default:
		return false
	}
}

// overlayPage renders the command palette as a centered modal. The underlying
// page content is intentionally not composed underneath; the palette is a
// focused overlay that disappears when dismissed.
func overlayPage(_ string, overlay string, width, height int) string {
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, overlay)
}

// dispatchPage routes the current message to the active page model, collecting
// any resulting commands. It is extracted from Update to keep the message
// dispatch switch parallel to the page model switch.
func (m Model) dispatchPage(msg tea.Msg, cmds []tea.Cmd) (Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.page {
	case PageWelcome:
		m.welcome, cmd = m.welcome.Update(msg)
		cmds = append(cmds, cmd)
		cmds = append(cmds, m.handleWelcomeAction())

	case PageFilePicker:
		// Handle Esc to go back to welcome
		if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
			if keyMsg.String() == "esc" || keyMsg.String() == "escape" {
				m.welcome.ResetInput()
				m.page = PageWelcome
				return m, m.checkDaemonStatus()
			}
		}
		m.filePicker, cmd = m.filePicker.Update(msg)
		cmds = append(cmds, cmd)
		if didSelect, path := m.filePicker.DidSelectFile(msg); didSelect {
			// Normalize path
			if absPath, err := filepath.Abs(path); err == nil {
				m.walletFile = filepath.Clean(absPath)
			} else {
				m.walletFile = path
			}
			m.page = PagePassword
			m.password = pages.NewPassword(pages.PasswordModeUnlock)
			m.password.SetVersion(Version)
			m.password.SetWalletFile(m.walletFile)
			cmds = append(cmds, m.password.Init())
			cmds = append(cmds, m.setWindowTitleCmd())
		}

	case PagePassword:
		m.password, cmd = m.password.Update(msg)
		cmds = append(cmds, cmd)
		cmds = append(cmds, m.handlePasswordAction())

	case PageNetwork:
		m.network, cmd = m.network.Update(msg)
		cmds = append(cmds, cmd)
		cmds = append(cmds, m.handleNetworkAction())

	case PageSeed:
		m.seed, cmd = m.seed.Update(msg)
		cmds = append(cmds, cmd)
		cmds = append(cmds, m.handleSeedAction())

	case PageKeyInput:
		m.keyInput, cmd = m.keyInput.Update(msg)
		cmds = append(cmds, cmd)
		cmds = append(cmds, m.handleKeyInputAction())

	case PageLogo:
		m.logo, cmd = m.logo.Update(msg)
		cmds = append(cmds, cmd)
		if m.logo.Cancelled() {
			m.logo.ClearCancelled()
			m.page = m.logoReturnPage
			if m.page == PageLogo {
				m.page = PageWelcome
			}
			cmds = append(cmds, m.setWindowTitleCmd())
		}

	case PageQRCode:
		m.qrcode, cmd = m.qrcode.Update(msg)
		cmds = append(cmds, cmd)
		if m.qrcode.Cancelled() {
			m.qrcode.Reset()
			// Return to the page we came from.
			switch m.qrReturnPage {
			case PageIntegratedAddr:
				m.page = PageIntegratedAddr
			case PageWelcome:
				m.page = PageWelcome
			default:
				m.page = PageMain
			}
			m.qrReturnPage = PageMain // Reset to default
			cmds = append(cmds, m.setWindowTitleCmd())
		}

	case PageMain:
		cmds = append(cmds, m.handleDashboard(msg))

	case PageSend:
		m.send, cmd = m.send.Update(msg)
		cmds = append(cmds, cmd)
		if m.send.Cancelled() {
			m.send.Reset()
			m.page = PageMain
		}
		if m.send.Confirmed() {
			// Log transfer initiation (truncated address for privacy)
			addr := m.send.GetAddress()
			if len(addr) > 16 {
				addr = addr[:8] + "..." + addr[len(addr)-8:]
			}
			derolog.Debug("theme", "send.confirmed", "Send confirmed theme snapshot",
				"theme", styles.GetCurrentThemeID(),
				"border", fmt.Sprintf("%v", styles.ColorBorder),
				"primary", fmt.Sprintf("%v", styles.ColorPrimary))
			derolog.Info("transfer", "initiated", "Transfer initiated", "dest_truncated", addr, "amount_atomic", fmt.Sprintf("%d", m.send.GetAmount()))
			// Start processing animation and execute transfer
			m.send.StartProcessing()
			cmds = append(cmds, m.send.ProcessingMinDurationCmd())
			cmds = append(cmds, m.executeTransfer())
		}
		// Check if processing is complete (result received + minimum duration elapsed)
		if m.send.ShouldComplete() {
			m.send.Reset()
			m.page = PageMain
			cmds = append(cmds, m.updateWalletInfo())
			cmds = append(cmds, m.setWindowTitleCmd())
		}

	case PageHistory:
		// Clear export message on any key press (except 'e')
		if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
			if keyMsg.String() != "e" && keyMsg.String() != "E" {
				m.history.ClearExportMessage()
			}
		}
		m.history, cmd = m.history.Update(msg)
		cmds = append(cmds, cmd)
		// Handle details request
		if m.history.WantDetails() {
			m.history.ResetActions()
			if tx := m.history.SelectedTx(); tx != nil {
				m.txDetails.SetTransaction(*tx)
				m.page = PageTxDetails
			}
		}
		// Handle export request
		if m.history.WantExport() {
			m.history.ResetActions()
			if m.wallet != nil {
				count, err := m.wallet.ExportHistory("./history")
				if err != nil {
					m.history.SetExportMessage("Export failed: "+err.Error(), false)
				} else {
					m.history.SetExportMessage(fmt.Sprintf("Exported %d file(s) to ./history/", count), true)
				}
			}
		}

	case PageTxDetails:
		m.txDetails, cmd = m.txDetails.Update(msg)
		cmds = append(cmds, cmd)
		if m.txDetails.Cancelled() {
			m.txDetails.Reset()
			m.page = PageHistory
		}

	case PageDaemon:
		m.daemon, cmd = m.daemon.Update(msg)
		cmds = append(cmds, cmd)
		cmds = append(cmds, m.handleDaemonAction())

	case PageDaemonStatus:
		m.daemonStatus, cmd = m.daemonStatus.Update(msg)
		cmds = append(cmds, cmd)
		if m.daemonStatus.Cancelled() {
			// Diagnostic: record what message triggered the leave so a stray
			// input that bypasses the Y/N prompt can be traced (visible in the
			// F12 debug console / debug log).
			derolog.Info("daemon", "page.leave", "Leaving daemon page", "trigger", fmt.Sprintf("%T", msg))
			m.daemonStatus.ResetActions()
			m.page = PageWelcome
			cmds = append(cmds, m.setWindowTitleCmd())
		}
		if m.daemonStatus.WantStart() {
			m.daemonStatus.ResetActions()
			isEmbeddedMode := config.GetDaemonSettings().Mode == "embedded" || m.daemonStatus.Snapshot.Source == "Embedded"
			if !isEmbeddedMode {
				m.daemonManagedSince = time.Now()
			}
			cmds = append(cmds, m.startLocalDaemonCmd())
		}
		if m.daemonStatus.WantStop() {
			m.daemonStatus.ResetActions()
			cmds = append(cmds, m.stopLocalDaemonCmd())
		}
		if m.daemonStatus.WantRestart() {
			m.daemonStatus.ResetActions()
			cmds = append(cmds, m.restartLocalDaemonCmd())
		}
		if m.daemonStatus.WantLogs() {
			m.daemonStatus.ResetActions()
			m.page = PageDaemonLogs
			cmds = append(cmds, m.setWindowTitleCmd())
		}
		if m.daemonStatus.WantSettings() {
			m.daemonStatus.ResetActions()
			settings := daemonSettingsForSnapshot(config.GetDaemonSettings(), m.daemonStatus.Snapshot)
			if strings.TrimSpace(settings.IntegratorAddress) == "" && m.wallet != nil {
				if addr := strings.TrimSpace(m.wallet.GetInfo().Address); addr != "" {
					settings.IntegratorAddress = addr
				}
			}
			m.daemonSettings = pages.NewDaemonSettings(settings)
			m.page = PageDaemonSettings
			cmds = append(cmds, m.setWindowTitleCmd())
		}
		if m.daemonStatus.WantInstall() {
			m.daemonStatus.ResetActions()
			// Install registers the built-in daemon as a background service, so
			// it applies to embedded mode too — no external binary involved.
			if m.daemonStatus.Snapshot.Running || m.daemonStatus.Snapshot.Managed || m.daemonStatus.Snapshot.IsOnline {
				m.daemonStatus.DownloadError = "a daemon is already running; stop it before installing a service"
				m.daemonStatus.Downloading = false
				break
			}
			m.daemonStatus.Downloading = true
			m.daemonStatus.DownloadError = ""
			m.daemonStatus.InstallResult = ""
			cmds = append(cmds, m.daemonInstallPreviewCmd())
		}
		if m.daemonStatus.WantInstallApply() {
			m.daemonStatus.ResetActions()
			m.daemonStatus.Downloading = true
			m.daemonStatus.DownloadError = ""
			m.daemonStatus.InstallResult = ""
			if m.daemonStatus.InstallPlan != nil {
				plan := *m.daemonStatus.InstallPlan
				m.daemonStatus.ResetInstall()
				cmds = append(cmds, m.daemonInstallApplyCmd(plan))
			}
		}
		if m.daemonStatus.WantInstallDone() {
			m.daemonStatus.ResetActions()
			m.daemonStatus.ResetInstall()
		}
		if m.daemonStatus.WantUninstall() {
			m.daemonStatus.ResetActions()
			m.daemonStatus.ConfirmingUninstall = true
		}
		if m.daemonStatus.WantUninstallApply() {
			m.daemonStatus.ResetActions()
			m.daemonStatus.ResetUninstall()
			cmds = append(cmds, m.daemonUninstallCmd())
		}
		if m.daemonStatus.WantUninstallDone() {
			m.daemonStatus.ResetActions()
			m.daemonStatus.ResetUninstall()
		}

	case PageDaemonLogs:
		m.daemonLogs, cmd = m.daemonLogs.Update(msg)
		cmds = append(cmds, cmd)
		if m.daemonLogs.Cancelled() {
			m.daemonLogs.Reset()
			m.page = PageDaemonStatus
			cmds = append(cmds, m.setWindowTitleCmd())
		}

	case PageDaemonSettings:
		m.daemonSettings, cmd = m.daemonSettings.Update(msg)
		cmds = append(cmds, cmd)
		if m.daemonSettings.WantUseWallet() {
			m.daemonSettings.ResetFlags()
			if m.wallet != nil {
				settings := m.daemonSettings.Settings
				settings.IntegratorAddress = m.wallet.GetInfo().Address
				m.daemonSettings = pages.NewDaemonSettings(settings)
				m.daemonSettings.SetSuccess("Using current wallet address")
			} else {
				m.daemonSettings.SetError("Open a wallet first to use its address")
			}
		}
		if m.daemonSettings.Saved() {
			settings := m.daemonSettings.Settings
			if !settings.IsPruned() {
				m.pendingPrune = false
				m.pruneAppliedOnce = false
				m.applyingPrune = false
			}
			if err := config.SetDaemonSettings(settings); err != nil {
				m.daemonSettings.SetError("Failed to save settings: " + err.Error())
			} else {
				m.daemonSettings.SetSuccess("Daemon settings saved")
			}
			m.daemonSettings.ResetFlags()
		}
		if m.daemonSettings.Cancelled() {
			m.daemonSettings.ResetFlags()
			m.page = PageDaemonStatus
			cmds = append(cmds, m.setWindowTitleCmd())
		}

	case PageMiner:
		m.miner, cmd = m.miner.Update(msg)
		cmds = append(cmds, cmd)
		if m.miner.Cancelled() {
			m.miner.ResetActions()
			m.page = m.minerReturnPage
			cmds = append(cmds, m.setWindowTitleCmd())
		}
		if m.miner.WantStart() {
			m.miner.ResetActions()
			cmds = append(cmds, m.startMinerCmd())
		}
		if m.miner.WantStop() {
			m.miner.ResetActions()
			cmds = append(cmds, m.stopMinerCmd())
		}

	case PageIntegratedAddr:
		m.integratedAddr, cmd = m.integratedAddr.Update(msg)
		cmds = append(cmds, cmd)
		if m.integratedAddr.Cancelled() {
			m.integratedAddr.Reset()
			m.page = PageMain
			cmds = append(cmds, m.setWindowTitleCmd())
		}
		// Handle QR view request after successful generation
		if m.integratedAddr.WantViewQR() {
			m.integratedAddr.ResetActions()
			m.qrcode = pages.NewQRCode(m.integratedAddr.GeneratedAddress())
			m.qrReturnPage = PageIntegratedAddr
			m.page = PageQRCode
			cmds = append(cmds, m.setWindowTitleCmd())
		}

	case PageNames:
		m.names, cmd = m.names.Update(msg)
		cmds = append(cmds, cmd)
		if m.names.Cancelled() {
			m.names.Refresh()
			m.page = PageMain
			cmds = append(cmds, m.setWindowTitleCmd())
		}
		if m.names.WantRegister() {
			m.names.ResetActions()
			m.nameRegister = pages.NewNameRegister()
			m.page = PageNameRegister
			cmds = append(cmds, m.nameRegister.Init(), m.setWindowTitleCmd())
		}
		if name, ok := m.names.WantTransfer(); ok {
			m.names.ResetActions()
			m.nameTransfer = pages.NewNameTransfer()
			m.nameTransfer.SetName(name)
			m.page = PageNameTransfer
			cmds = append(cmds, m.nameTransfer.Init(), m.setWindowTitleCmd())
		}
		if _, ok := m.names.WantTransferAll(); ok {
			m.names.ResetActions()
			names := make([]string, 0, len(m.names.Names()))
			for _, entry := range m.names.Names() {
				names = append(names, entry.Name)
			}
			m.nameTransfer = pages.NewNameTransfer()
			m.nameTransfer.SetTransferAll(names)
			m.page = PageNameTransfer
			cmds = append(cmds, m.nameTransfer.Init(), m.setWindowTitleCmd())
		}

	case PageNameRegister:
		m.nameRegister, cmd = m.nameRegister.Update(msg)
		cmds = append(cmds, cmd)
		if m.nameRegister.Cancelled() {
			m.nameRegister.Reset()
			m.page = PageNames
			cmds = append(cmds, m.setWindowTitleCmd())
		}
		if m.nameRegister.Confirmed() {
			m.nameRegister.StartProcessing()
			name := m.nameRegister.GetName()
			cmds = append(cmds, m.registerNameCmd(name))
		}

	case PageNameTransfer:
		m.nameTransfer, cmd = m.nameTransfer.Update(msg)
		cmds = append(cmds, cmd)
		if m.nameTransfer.Cancelled() {
			m.nameTransfer.Reset()
			m.page = PageNames
			cmds = append(cmds, m.setWindowTitleCmd())
		}
		if m.nameTransfer.Confirmed() {
			m.nameTransfer.StartProcessing()
			newOwner := m.nameTransfer.GetNewOwner()
			if m.nameTransfer.IsTransferAll() {
				cmds = append(cmds, m.transferAllNamesCmd(m.nameTransfer.GetAllNames(), newOwner))
			} else {
				cmds = append(cmds, m.transferNameCmd(m.nameTransfer.GetName(), newOwner))
			}
		}

	case PageXSWDAuth:
		m.xswdAuth, cmd = m.xswdAuth.Update(msg)
		cmds = append(cmds, cmd)
		if m.xswdAuth.Confirmed() {
			result := m.xswdAuth.Accepted()
			if result {
				derolog.Info("xswd", "auth.accepted", "User accepted XSWD auth request")
			} else {
				derolog.Info("xswd", "auth.rejected", "User rejected XSWD auth request")
			}
			if m.xswdAuthCh != nil {
				m.xswdAuthCh <- result
				m.xswdAuthCh = nil
			}
			m.xswdAuth.Reset()
			m.page = m.xswdPrevPage
			cmds = append(cmds, m.setWindowTitleCmd())
		}

	case PageXSWDPerm:
		m.xswdPerm, cmd = m.xswdPerm.Update(msg)
		cmds = append(cmds, cmd)
		if m.xswdPerm.Confirmed() {
			result := m.xswdPerm.Result()
			permStr := fmt.Sprintf("%d", result)
			switch result {
			case wallet.XSWDPermAllow:
				permStr = "allow"
			case wallet.XSWDPermDeny:
				permStr = "deny"
			case wallet.XSWDPermAlwaysAllow:
				permStr = "always_allow"
			case wallet.XSWDPermAlwaysDeny:
				permStr = "always_deny"
			}
			derolog.Info("xswd", "perm.response", "User responded to permission request", "permission", permStr)
			if m.xswdPermCh != nil {
				m.xswdPermCh <- result
				m.xswdPermCh = nil
			}
			m.xswdPerm.Reset()
			m.page = m.xswdPrevPage
			cmds = append(cmds, m.setWindowTitleCmd())
		}

	case PageDiscover:
		m.discover, cmd = m.discover.Update(msg)
		cmds = append(cmds, cmd)
		cmds = append(cmds, m.maybeHydrateDiscover(), m.maybeFetchDiscoverRatings())
		if m.discover.Cancelled() {
			m.discover.ClearCancelled()
			m.page = PageMain
			if m.wallet == nil {
				m.page = PageWelcome
			}
			cmds = append(cmds, m.setWindowTitleCmd())
		}
		if scid, cancel, ok := m.discover.WantLaunch(); ok {
			m.discover.ResetActions()
			launch := m.launchTelaCmd(scid, cancel)
			if m.wallet != nil && (m.xswdBridge == nil || !m.xswdBridge.IsRunning()) {
				cmds = append(cmds, tea.Sequence(m.startXSWDCmd(), launch))
			} else {
				cmds = append(cmds, launch)
			}
		}

	case PageTokens:
		m.tokens, cmd = m.tokens.Update(msg)
		cmds = append(cmds, cmd)
		if m.tokens.Cancelled() {
			m.tokens.ClearCancelled()
			m.page = PageMain
			cmds = append(cmds, m.setWindowTitleCmd())
		}
		if m.tokens.WantRescan() {
			m.tokens.ResetActions()
			m.tokenScanActive = true
			m.tokenScanFound = 0
			m.tokens.SetScanning(true, "Scanning all indexed SCIDs...")
			cmds = append(cmds, m.tokenScanStartCmd(true))
		}
		if scid, ok := m.tokens.WantAdd(); ok {
			m.tokens.ResetActions()
			cmds = append(cmds, m.addTokenCmd(scid))
		}
		if scid, ok := m.tokens.WantSend(); ok {
			m.tokens.ResetActions()
			decimals := uint64(0)
			balance := uint64(0)
			ticker := ""
			for i := range m.tokens.Tokens() {
				if m.tokens.Tokens()[i].SCID == scid {
					decimals = m.tokens.Tokens()[i].Decimals
					balance = m.tokens.Tokens()[i].Balance
					ticker = m.tokens.Tokens()[i].Ticker
					break
				}
			}
			if balance == 0 {
				if b, err := m.wallet.GetTokenBalance(scid); err == nil {
					balance = b
				}
			}
			m.tokenSend = pages.NewTokenSend()
			m.tokenSend.SetToken(scid, ticker, decimals, balance, 0)
			if m.wallet != nil {
				info := m.wallet.GetInfo()
				m.tokenSend.SetSimulator(m.wallet.IsSimulator())
				m.tokenSend.SetBalance(balance, info.Balance)
			}
			m.page = PageTokenSend
			cmds = append(cmds, m.tokenSend.Init(), m.setWindowTitleCmd())
		}
		if scid, ok := m.tokens.WantHistory(); ok {
			m.tokens.ResetActions()
			ticker := ""
			decimals := uint64(0)
			for i := range m.tokens.Tokens() {
				if m.tokens.Tokens()[i].SCID == scid {
					ticker = m.tokens.Tokens()[i].Ticker
					decimals = m.tokens.Tokens()[i].Decimals
					break
				}
			}
			m.tokenHistory = pages.NewTokenHistory()
			m.tokenHistory.SetToken(scid, ticker, decimals)
			m.page = PageTokenHistory
			cmds = append(cmds, m.loadTokenHistoryCmd(scid), m.setWindowTitleCmd())
		}
		if scid, ok := m.tokens.WantRemove(); ok {
			m.tokens.ResetActions()
			if m.wallet != nil {
				if err := config.RemoveWalletToken(m.walletFile, scid); err == nil {
					label := scid
					if len(label) > 16 {
						label = label[:16] + "..."
					}
					m.tokens.SetFlash("Removed "+label, true)
					m.tokens.SetLoading(true)
					cmds = append(cmds, m.loadTokensCmd())
				}
			}
		}
	case PageTokenSend:
		m.tokenSend, cmd = m.tokenSend.Update(msg)
		cmds = append(cmds, cmd)
		if m.tokenSend.Cancelled() {
			m.tokenSend.Reset()
			m.page = PageTokens
			m.tokens.SetLoading(true)
			cmds = append(cmds, m.loadTokensCmd(), m.setWindowTitleCmd())
		}
		if m.tokenSend.Confirmed() {
			m.tokenSend.StartProcessing()
			cmds = append(cmds, m.tokenSend.ProcessingMinDurationCmd(), m.executeTokenTransfer())
		}
		if m.tokenSend.ShouldComplete() {
			m.tokenSend.Reset()
			m.page = PageTokens
			m.tokens.SetLoading(true)
			cmds = append(cmds, m.loadTokensCmd(), m.setWindowTitleCmd())
		}
	case PageTokenHistory:
		m.tokenHistory, cmd = m.tokenHistory.Update(msg)
		cmds = append(cmds, cmd)
		if m.tokenHistory.Cancelled() {
			m.tokenHistory.Reset()
			m.page = PageTokens
			cmds = append(cmds, m.setWindowTitleCmd())
		}
		if m.tokenHistory.WantDetails() {
			m.tokenHistory.ResetActions()
			if tx := m.tokenHistory.SelectedTx(); tx != nil {
				m.txDetails.SetTransaction(*tx)
				m.page = PageTxDetails
				cmds = append(cmds, m.setWindowTitleCmd())
			}
		}
	}

	return m, tea.Batch(cmds...)
}

// View renders the UI
func (m Model) View() tea.View {
	if m.quitting {
		return tea.NewView("Goodbye!\n")
	}

	var content string
	switch m.page {
	case PageWelcome:
		content = m.welcome.View()

	case PageFilePicker:
		content = m.renderFilePicker()

	case PagePassword:
		content = m.password.View()

	case PageNetwork:
		content = m.network.View()

	case PageSeed:
		content = m.seed.View()

	case PageKeyInput:
		content = m.keyInput.View()

	case PageLogo:
		content = m.logo.View()

	case PageQRCode:
		content = m.qrcode.View()

	case PageMain:
		content = m.renderDashboard()

	case PageSend:
		content = m.renderSend()

	case PageHistory:
		content = m.renderHistory()

	case PageTxDetails:
		content = m.renderTxDetails()

	case PageDaemon:
		content = m.daemon.View()

	case PageDaemonStatus:
		content = m.renderDaemonStatus()

	case PageDaemonLogs:
		content = m.daemonLogs.View()

	case PageDaemonSettings:
		content = m.daemonSettings.View()

	case PageMiner:
		content = m.miner.View()

	case PageNames:
		content = m.names.View()

	case PageNameRegister:
		content = m.nameRegister.View()

	case PageNameTransfer:
		content = m.nameTransfer.View()

	case PageIntegratedAddr:
		content = m.integratedAddr.View()

	case PageXSWDAuth:
		content = m.xswdAuth.View()

	case PageXSWDPerm:
		content = m.xswdPerm.View()

	case PageDiscover:
		content = m.discover.View()

	case PageTokens:
		content = m.tokens.View()

	case PageTokenSend:
		content = m.tokenSend.View()

	case PageTokenHistory:
		content = m.tokenHistory.View()
	}

	// Default dimensions if not yet received
	width, height := m.width, m.height
	if width == 0 {
		width = 100
	}
	if height == 0 {
		height = 40
	}

	// Overlay the command palette on top of the current page when open.
	if m.palette.IsOpen() {
		overlay := m.palette.View()
		content = overlayPage(content, overlay, width, height)
	}

	// Render debug UI when enabled:
	// - collapsed: always show 1-line strip
	// - expanded: show 3-line panel if it fits, otherwise fall back to strip
	var v tea.View
	if m.debugEnabled {
		// Expanded panel mode
		if m.debugConsoleOpen {
			requestedGapLines := 1 // Preferred empty line between main UI and debug panel

			// Keep content naturally compact and centered horizontally.
			mainContent := trimTrailingBlankLines(content)
			rawContentWidth := maxVisibleLineWidth(mainContent)
			mainContent = lipgloss.PlaceHorizontal(width, lipgloss.Center, mainContent)

			mainLines := lineCount(mainContent)
			logLines := debugExpandedLogLines
			consoleHeight := logLines + 4

			// If we are short by exactly one line, drop the spacer first before clipping main content.
			gapLines := requestedGapLines
			if mainLines+gapLines+consoleHeight > height {
				gapLines = 0
			}

			// Final clamp to avoid overflow in very small terminals.
			contentHeight := height - consoleHeight - gapLines
			if contentHeight < 0 {
				contentHeight = 0
			}
			if mainLines > contentHeight {
				mainContent = trimToLastLines(mainContent, contentHeight)
			}

			panelWidth := styles.Width
			if rawContentWidth > panelWidth {
				panelWidth = rawContentWidth
			}
			consoleOverlay := m.renderDebugConsoleOverlay(width, panelWidth, logLines)

			if contentHeight == 0 {
				v = tea.NewView(consoleOverlay)
			} else {
				if gapLines > 0 {
					v = tea.NewView(lipgloss.JoinVertical(lipgloss.Top, mainContent, "", consoleOverlay))
				} else {
					v = tea.NewView(lipgloss.JoinVertical(lipgloss.Top, mainContent, consoleOverlay))
				}
			}
		} else {
			// Collapsed strip mode
			contentHeight := height - 1
			if contentHeight < 1 {
				contentHeight = 1
			}
			mainContent := lipgloss.Place(width, contentHeight, lipgloss.Center, lipgloss.Center, content)
			strip := m.renderDebugStrip(width)
			v = tea.NewView(lipgloss.JoinVertical(lipgloss.Top, mainContent, strip))
		}
	} else if m.page == PageQRCode || m.page == PageIntegratedAddr || m.page == PageLogo {
		// QR code and integrated address pages are tall - place at top if terminal is too short
		v = tea.NewView(lipgloss.Place(width, height, lipgloss.Center, lipgloss.Top, content))
	} else if m.page == PageWelcome {
		// Welcome page is always placed at top to prevent logo cropping
		// The themes menu makes content tall, so top alignment ensures visibility
		v = tea.NewView(lipgloss.Place(width, height, lipgloss.Center, lipgloss.Top, content))
	} else {
		// Center content in full terminal
		v = tea.NewView(lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, content))
	}

	// Enable alt-screen and mouse support
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

// renderDebugConsoleOverlay renders the debug console as an overlay
func (m Model) renderDebugConsoleOverlay(termWidth int, preferredWidth int, logLineCount int) string {
	consoleWidth := preferredWidth
	if consoleWidth <= 0 {
		consoleWidth = styles.Width
	}
	maxAllowed := termWidth - 2
	if maxAllowed < 50 {
		maxAllowed = 50
	}
	if consoleWidth > maxAllowed {
		consoleWidth = maxAllowed
	}
	if consoleWidth < 50 {
		consoleWidth = 50
	}
	innerWidth := consoleWidth - 4 // minus left+right border and horizontal padding

	// Header bar with title and status (ASCII only to avoid rune-width ambiguity/wrapping)
	titleText := "o Debug Console"
	title := styles.TitleStyle.Render(titleText)
	statusText := "● LIVE"
	status := styles.SuccessStyle.Render(statusText)
	helpText := "F12 Collapse"
	leftPlain := titleText + "  " + statusText
	minGap := 2
	maxHelpWidth := utf8.RuneCountInString(leftPlain)
	maxHelpWidth = innerWidth - maxHelpWidth - minGap
	if maxHelpWidth < 0 {
		maxHelpWidth = 0
	}
	helpText = truncateRunes(helpText, maxHelpWidth)

	gapWidth := innerWidth - utf8.RuneCountInString(leftPlain) - utf8.RuneCountInString(helpText)
	if gapWidth < minGap {
		gapWidth = minGap
	}

	help := styles.MutedStyle.Render(helpText)
	headerLine := title + "  " + status + strings.Repeat(" ", gapWidth) + help

	separator := lipgloss.NewStyle().
		Foreground(styles.ColorBorder).
		Render(strings.Repeat("─", innerWidth))

	total := len(m.debugLogEntries)
	startIdx := 0
	if total > logLineCount {
		if m.debugAutoFollow {
			startIdx = total - logLineCount
		} else {
			startIdx = m.debugScrollStart
			maxStart := total - logLineCount
			if startIdx < 0 {
				startIdx = 0
			}
			if startIdx > maxStart {
				startIdx = maxStart
			}
		}
	}
	endIdx := startIdx + logLineCount
	if endIdx > total {
		endIdx = total
	}

	logColumnWidth := innerWidth - 2
	if logColumnWidth < 20 {
		logColumnWidth = 20
	}

	thumbStart := 0
	thumbSize := logLineCount
	if total > logLineCount {
		thumbSize = (logLineCount * logLineCount) / total
		if thumbSize < 1 {
			thumbSize = 1
		}
		trackSpan := logLineCount - thumbSize
		dataSpan := total - logLineCount
		if dataSpan > 0 && trackSpan > 0 {
			thumbStart = (startIdx * trackSpan) / dataSpan
		}
	}

	var logLines []string
	rowIdx := 0
	for i := startIdx; i < endIdx; i++ {
		entry := m.debugLogEntries[i]

		// Format using the improved formatter
		formatted := FormatLogEntry(entry, logColumnWidth)

		formatted = strings.ReplaceAll(formatted, "\n", " ")
		formatted = truncateRunes(formatted, logColumnWidth)
		visible := utf8.RuneCountInString(formatted)
		if visible < logColumnWidth {
			formatted += strings.Repeat(" ", logColumnWidth-visible)
		}

		// Color code by level
		var lineStyled string
		switch entry.Level {
		case derolog.LevelError:
			lineStyled = styles.ErrorStyle.Render(formatted)
		case derolog.LevelWarn:
			lineStyled = styles.WarningStyle.Render(formatted)
		default:
			lineStyled = lipgloss.NewStyle().Foreground(styles.ColorText).Render(formatted)
		}

		scrollChar := styles.MutedStyle.Render("│")
		if rowIdx >= thumbStart && rowIdx < thumbStart+thumbSize {
			scrollChar = lipgloss.NewStyle().Foreground(styles.ColorPrimary).Render("█")
		}
		row := lineStyled + " " + scrollChar
		logLines = append(logLines, row)
		rowIdx++
	}

	for len(logLines) < logLineCount {
		scrollChar := styles.MutedStyle.Render("│")
		if rowIdx >= thumbStart && rowIdx < thumbStart+thumbSize {
			scrollChar = lipgloss.NewStyle().Foreground(styles.ColorPrimary).Render("█")
		}
		row := strings.Repeat(" ", logColumnWidth) + " " + scrollChar
		logLines = append(logLines, row)
		rowIdx++
	}

	content := lipgloss.JoinVertical(lipgloss.Left, headerLine, separator)
	if logLineCount > 0 {
		logsContent := strings.Join(logLines, "\n")
		content = lipgloss.JoinVertical(lipgloss.Left, content, logsContent)
	}

	consoleStyle := themedBoxStyle().
		Background(lipgloss.Color("#0D0D0D")).
		Padding(0, 1).
		Width(consoleWidth)

	return lipgloss.PlaceHorizontal(termWidth, lipgloss.Center, consoleStyle.Render(content))
}

func (m Model) renderDebugStrip(termWidth int) string {
	stripWidth := termWidth - 2
	if stripWidth > styles.Width {
		stripWidth = styles.Width
	}
	if stripWidth < 20 {
		stripWidth = 20
	}

	// Show count of high-signal events
	eventCount := len(m.debugLogEntries)
	text := fmt.Sprintf("o Debug %d events  F12 Expand", eventCount)
	if len(text) > stripWidth {
		if stripWidth > 3 {
			text = text[:stripWidth-3] + "..."
		} else {
			text = text[:stripWidth]
		}
	}

	strip := lipgloss.NewStyle().
		Foreground(styles.ColorMuted).
		Background(lipgloss.Color("#0D0D0D")).
		Padding(0, 1).
		Width(stripWidth).
		Render(text)

	return lipgloss.PlaceHorizontal(termWidth, lipgloss.Center, strip)
}

func (m Model) maxDebugScrollStart(visible int) int {
	total := len(m.debugLogEntries)
	if total <= visible {
		return 0
	}
	return total - visible
}

func (m *Model) clampDebugScrollForHeight(height int) {
	visible := debugExpandedLogLines
	if visible <= 0 {
		m.debugScrollStart = 0
		return
	}
	maxStart := m.maxDebugScrollStart(visible)
	if m.debugScrollStart < 0 {
		m.debugScrollStart = 0
	}
	if m.debugScrollStart > maxStart {
		m.debugScrollStart = maxStart
	}
}

func (m Model) renderFilePicker() string {
	title := styles.TitleStyle.Render("Select Wallet File")
	picker := m.filePicker.View()
	help := styles.MutedStyle.Render("↑↓ Navigate • Enter Select • Esc Back")

	// Keep picker text left-aligned, but center the entire picker block.
	// Use visible line width (ANSI stripped) so centering works reliably.
	contentWidth := styles.Width - 8 // Account for box horizontal padding
	pickerWidth := maxVisibleLineWidth(picker) + 2
	if pickerWidth > contentWidth {
		pickerWidth = contentWidth
	}
	if pickerWidth < 20 {
		pickerWidth = 20
	}

	pickerBlock := lipgloss.NewStyle().
		Width(pickerWidth).
		Align(lipgloss.Left).
		Render(picker)

	centeredPicker := lipgloss.PlaceHorizontal(contentWidth, lipgloss.Center, pickerBlock)

	content := lipgloss.JoinVertical(lipgloss.Center,
		title,
		"",
		centeredPicker,
		"",
		help,
	)

	return themedBoxStyle().
		Width(styles.Width).
		Padding(2, 4).
		Render(content)
}

func maxVisibleLineWidth(s string) int {
	maxWidth := 0
	for _, line := range strings.Split(s, "\n") {
		w := visibleWidthANSI(line)
		if w > maxWidth {
			maxWidth = w
		}
	}
	return maxWidth
}

func trimToMaxLines(s string, maxLines int) string {
	if maxLines <= 0 {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) <= maxLines {
		return s
	}
	return strings.Join(lines[:maxLines], "\n")
}

func trimToLastLines(s string, maxLines int) string {
	if maxLines <= 0 {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) <= maxLines {
		return s
	}
	return strings.Join(lines[len(lines)-maxLines:], "\n")
}

func trimTrailingBlankLines(s string) string {
	lines := strings.Split(s, "\n")
	end := len(lines)
	for end > 0 {
		if strings.TrimSpace(lines[end-1]) != "" {
			break
		}
		end--
	}
	if end <= 0 {
		return ""
	}
	return strings.Join(lines[:end], "\n")
}

func truncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	r := []rune(s)
	return string(r[:max])
}

func lineCount(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

func visibleWidthANSI(s string) int {
	width := 0
	escSeen := false
	inCSI := false
	inOSC := false
	oscEscSeen := false
	for _, r := range s {
		if inOSC {
			if oscEscSeen {
				oscEscSeen = false
				if r == '\\' {
					inOSC = false
				}
				continue
			}
			if r == '\x1b' {
				oscEscSeen = true
				continue
			}
			if r == '\a' {
				inOSC = false
			}
			continue
		}

		if inCSI {
			if r >= '@' && r <= '~' {
				inCSI = false
			}
			continue
		}

		if escSeen {
			escSeen = false
			if r == '[' {
				inCSI = true
				continue
			}
			if r == ']' {
				inOSC = true
				continue
			}
			continue
		}

		if r == '\x1b' {
			escSeen = true
			continue
		}
		width++
	}
	return width
}

func (m Model) renderDashboard() string {
	// Dashboard renders everything itself (logo, wallet info, balance, actions, activity)
	content := m.dashboard.View()
	contentLines := strings.Split(content, "\n")

	// The outer frame must fit the design width (styles.Width), so the content
	// budget inside the frame is styles.Width - 2. Without this cap the frame
	// came out 2 columns wider than the box on every other page (the dashboard
	// View is padded to styles.Width before renderDashboard adds the borders),
	// so in an 80-column terminal the right border wrapped/collided.
	contentWidth := 0
	for _, line := range contentLines {
		lineWidth := lipgloss.Width(line)
		if lineWidth > contentWidth {
			contentWidth = lineWidth
		}
	}
	if contentWidth > styles.Width-2 {
		contentWidth = styles.Width - 2
	}

	// Brand on the left, version on the right.
	brandLabel := "deroTUI"
	versionStr := "v" + Version
	leftLabel := " " + brandLabel + " "
	rightLabel := " " + versionStr + " "
	totalDashes := contentWidth - lipgloss.Width(leftLabel) - lipgloss.Width(rightLabel)
	if totalDashes < 0 {
		totalDashes = 0
	}

	cornerStyle := lipgloss.NewStyle().Foreground(styles.ColorBorder)
	dashStyle := lipgloss.NewStyle().Foreground(styles.ColorBorder)
	leftCorner := cornerStyle.Render("╭")
	rightCorner := cornerStyle.Render("╮")
	dashStr := dashStyle.Render(strings.Repeat("─", totalDashes))

	brandStyled := styles.TitleStyle.Render(leftLabel)
	versionStyled := styles.MutedStyle.Render(rightLabel)
	topBorder := leftCorner + brandStyled + dashStr + versionStyled + rightCorner

	borderStyle := lipgloss.NewStyle().Foreground(styles.ColorBorder)
	sideBorder := borderStyle.Render("│")

	framedLines := make([]string, 0, len(contentLines)+2)
	framedLines = append(framedLines, topBorder)
	for _, line := range contentLines {
		// Overflow guard: a content line wider than the budget (e.g. a long
		// flash message) would push the side border past the frame. Truncate it
		// to the budget; ansi-aware truncation keeps colors intact.
		if w := lipgloss.Width(line); w > contentWidth {
			line = lipgloss.NewStyle().Inline(true).MaxWidth(contentWidth).Render(line)
		}
		pad := contentWidth - lipgloss.Width(line)
		if pad > 0 {
			line += strings.Repeat(" ", pad)
		}
		framedLines = append(framedLines, sideBorder+line+sideBorder)
	}

	bottomBorder := cornerStyle.Render("╰") + dashStyle.Render(strings.Repeat("─", contentWidth)) + cornerStyle.Render("╯")
	framedLines = append(framedLines, bottomBorder)

	return strings.Join(framedLines, "\n")
}

func (m Model) renderSend() string {
	title := styles.TitleStyle.Render("Send DERO")
	sendView := m.send.View()

	content := lipgloss.JoinVertical(lipgloss.Center,
		title,
		"",
		sendView,
	)

	return themedBoxStyle().
		Width(styles.Width).
		Align(lipgloss.Center).
		Padding(1, 4).
		Render(content)
}

func (m Model) renderHistory() string {
	title := styles.TitleStyle.Render("▤ Transaction History")
	historyView := m.history.View()
	help := styles.MutedStyle.Render("↑↓ Navigate • Enter Details • E Export • Esc Back")

	content := lipgloss.JoinVertical(lipgloss.Center,
		title,
		"",
		historyView,
		help,
	)

	return themedBoxStyle().
		Width(styles.Width).
		Align(lipgloss.Center).
		Padding(1, 4).
		Render(content)
}

func themedBoxStyle() lipgloss.Style {
	return styles.ThemedBoxStyle()
}

func (m Model) renderTxDetails() string {
	title := styles.TitleStyle.Render("Transaction Details")
	detailsView := m.txDetails.View()

	// Center title separately
	contentWidth := styles.Width - 10
	centeredTitle := lipgloss.NewStyle().Width(contentWidth).Align(lipgloss.Center).Render(title)

	// Details view is left-aligned
	leftAlignedDetails := lipgloss.NewStyle().Width(contentWidth).Align(lipgloss.Left).Render(detailsView)

	content := lipgloss.JoinVertical(lipgloss.Left,
		centeredTitle,
		leftAlignedDetails,
	)

	return themedBoxStyle().
		Width(styles.Width).
		Align(lipgloss.Left).
		Padding(1, 4).
		Render(content)
}

func (m Model) renderDaemonStatus() string {
	title := styles.TitleStyle.Render("Daemon Manager")
	daemonView := m.daemonStatus.View()

	contentWidth := styles.Width - 10
	centeredTitle := lipgloss.NewStyle().Width(contentWidth).Align(lipgloss.Center).Render(title)
	leftAlignedContent := lipgloss.NewStyle().Width(contentWidth).Align(lipgloss.Left).Render(daemonView)

	content := lipgloss.JoinVertical(lipgloss.Left,
		centeredTitle,
		"",
		leftAlignedContent,
	)

	return themedBoxStyle().
		Width(styles.Width).
		Align(lipgloss.Left).
		Padding(1, 4).
		Render(content)
}
