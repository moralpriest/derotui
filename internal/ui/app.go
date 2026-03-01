// Copyright 2017-2026 DERO Project. All rights reserved.

package ui

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"charm.land/bubbles/v2/filepicker"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/deroproject/dero-wallet-cli/internal/config"
	derolog "github.com/deroproject/dero-wallet-cli/internal/log"
	"github.com/deroproject/dero-wallet-cli/internal/ui/pages"
	"github.com/deroproject/dero-wallet-cli/internal/ui/styles"
	"github.com/deroproject/dero-wallet-cli/internal/wallet"
)

func init() {
	// Disable logging by default - will be enabled by SetupLogging if --debug flag is set
	log.SetOutput(io.Discard)
}

// DebugLogPath is the path to the debug log file when --debug is enabled
var DebugLogPath string

const donationAddress = "deroi1qy8zrqrgqgcu6ayznw5zl9a50erdxgjd539rh3hz7qgu4zl4auqzkq9pvfp4x7p4235xzmntypuk7afqg3skgereypg8y6t9wd6zpuylnx8jqjfqwd5xzmrvypex2ur9de6zqf3qvahjqan9vaskup5n768"

const debugExpandedLogLines = 3

const (
	initialDaemonRetryInterval = 10 * time.Second
	maxDaemonRetryInterval     = 60 * time.Second
)

// SetupLogging configures debug logging when --debug flag is enabled
func SetupLogging(debug bool) {
	if !debug {
		log.SetOutput(io.Discard)
		derolog.SetOutput(io.Discard)
		CloseLogFile()
		return
	}

	// Get current working directory for absolute path
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	DebugLogPath = filepath.Join(cwd, "derotui-debug.log")

	logFile, err := os.OpenFile(DebugLogPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to open log file %s: %v\n", DebugLogPath, err)
		log.SetOutput(io.Discard)
		return
	}

	configureDebugLogging(logFile)

	derolog.Info("log", "init", "Logging initialized", "path", DebugLogPath)
	fmt.Printf("Debug log: %s\n", DebugLogPath)
}

func configureDebugLogging(logFile *os.File) {
	// Set up log buffer that captures entries for the debug console
	SetupLogBuffer(logFile)

	// Set up structured logging
	derolog.SetOutput(logFile)
	derolog.SetLevel(derolog.LevelDebug)
	derolog.RedirectStandardLog()
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
	PageIntegratedAddr // Generate integrated address
	PageXSWDAuth       // XSWD app authorization dialog
	PageXSWDPerm       // XSWD permission request dialog
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
	integratedAddr pages.IntegratedAddrModel

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
	xswdAuth     pages.XSWDAuthModel
	xswdPerm     pages.XSWDPermModel
	xswdPrevPage Page      // page to return to after dialog
	xswdAuthCh   chan bool // response channel for current auth request
	xswdPermCh   chan int  // response channel for current perm request

	// QR return page tracking
	qrReturnPage Page // page to return to after QR code view

	// Cached daemon status (from welcome page checks)
	cachedDaemonHealthy bool
	cachedDaemonAddress string

	// Sticky daemon selection set by explicit /connect.
	// This survives wallet transitions so create/open/restore can keep using
	// the exact daemon the user selected.
	stickyDaemonHealthy   bool
	stickyDaemonAddress   string
	stickyDaemonTestnet   bool
	stickyDaemonSimulator bool

	// Global debug console state (visible on all pages)
	debugEnabled     bool
	debugConsoleOpen bool
	debugAutoFollow  bool
	debugScrollStart int
	debugLastClickY  int
	debugLastClickAt time.Time
	debugLogEntries  []derolog.LogEntry
	regHintShown     bool
	pendingRegTxID   string
	pendingRegStatus string
	pendingRegHeight uint64
	pendingOutgoing  map[string]pendingOutgoingTx
	startupFlowSet   bool
	lastDaemonRetry  time.Time
	daemonRetryAfter time.Duration
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
		daemonRetryAfter: initialDaemonRetryInterval,
	}
	m.welcome.Version = Version
	m.password.SetVersion(Version)

	return m
}

// SetProgram sets the program reference for XSWD bridge message injection.
// This should be called immediately after creating the tea.Program but before Run().
func (m *Model) SetProgram(p wallet.MsgSender) {
	m.program = p
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
	daemonAddr := m.Opts.DaemonAddress
	testnet := m.Opts.Testnet
	simulator := m.Opts.Simulator
	implicitDaemon := daemonAddr == ""
	return func() tea.Msg {
		network := "Mainnet"
		if simulator {
			network = "Simulator"
		} else if testnet {
			network = "Testnet"
		}
		if daemonAddr == "" {
			if simulator {
				daemonAddr = wallet.DefaultSimulatorDaemon
			} else if testnet {
				daemonAddr = wallet.DefaultTestnetDaemon
			} else {
				daemonAddr = wallet.DefaultMainnetDaemon
			}
		}
		info := wallet.GetDaemonInfo(context.Background(), daemonAddr)
		if implicitDaemon && !simulator && !testnet && !info.IsHealthy {
			fallbackInfo := wallet.GetDaemonInfo(context.Background(), wallet.FallbackMainnetDaemon)
			if fallbackInfo.IsHealthy {
				daemonAddr = wallet.FallbackMainnetDaemon
				info = fallbackInfo
			}
		}
		if info.IsOnline {
			if info.Network == "Simulator" {
				network = "Simulator"
			} else if info.Testnet {
				network = "Testnet"
			} else {
				network = "Mainnet"
			}
		}
		return daemonStatusMsg{
			isOnline:   info.IsOnline,
			isSynced:   info.IsSynced,
			isHealthy:  info.IsHealthy,
			network:    network,
			address:    daemonAddr,
			height:     info.Height,
			topoHeight: info.TopoHeight,
		}
	}
}

// Update handles messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
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
			// Respond on any pending XSWD channels before quitting
			if m.xswdAuthCh != nil {
				m.xswdAuthCh <- false
				m.xswdAuthCh = nil
			}
			if m.xswdPermCh != nil {
				m.xswdPermCh <- wallet.XSWDPermDeny
				m.xswdPermCh = nil
			}
			if m.xswdBridge != nil {
				m.xswdBridge.Stop()
				m.xswdBridge = nil
				m.dashboard.SetXSWDRunning(false)
			}
			if m.wallet != nil {
				m.wallet.Close()
			}
			m.quitting = true
			return m, tea.Quit
		}

		// Q to quit only from main page (dashboard)
		if key.Matches(msg, key.NewBinding(key.WithKeys("q"))) && m.page == PageMain {
			// Respond on any pending XSWD channels before quitting
			if m.xswdAuthCh != nil {
				m.xswdAuthCh <- false
				m.xswdAuthCh = nil
			}
			if m.xswdPermCh != nil {
				m.xswdPermCh <- wallet.XSWDPermDeny
				m.xswdPermCh = nil
			}
			if m.xswdBridge != nil {
				m.xswdBridge.Stop()
				m.xswdBridge = nil
				m.dashboard.SetXSWDRunning(false)
			}
			if m.wallet != nil {
				m.wallet.Close()
			}
			m.quitting = true
			return m, tea.Quit
		}

		// Esc handling for different pages
		if key.Matches(msg, key.NewBinding(key.WithKeys("esc"))) {
			switch m.page {
			case PageMain:
				// Respond on any pending XSWD channels before closing wallet
				if m.xswdAuthCh != nil {
					m.xswdAuthCh <- false
					m.xswdAuthCh = nil
				}
				if m.xswdPermCh != nil {
					m.xswdPermCh <- wallet.XSWDPermDeny
					m.xswdPermCh = nil
				}
				// Stop XSWD server before closing wallet
				if m.xswdBridge != nil {
					m.xswdBridge.Stop()
					m.xswdBridge = nil
					m.dashboard.SetXSWDRunning(false)
				}
				// Close wallet and go back to welcome
				if m.wallet != nil {
					m.wallet.Close()
					m.wallet = nil
				}
				// Clear app-level cached state (keep global daemon endpoint for switch detection)
				m.cachedDaemonHealthy = false
				m.cachedDaemonAddress = ""
				m.regHintShown = false
				m.clearPendingRegistration()
				m.page = PageWelcome
				m.welcome = pages.NewWelcome()
				m.welcome.Version = Version
				return m, tea.Batch(m.checkDaemonStatus(), m.setWindowTitleCmd())
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
		// Update wallet info periodically
		if m.wallet != nil {
			m.updateWalletInfo()
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
		cmds = append(cmds, m.tickCmd())

	case daemonStatusMsg:
		// Update welcome screen with daemon status
		m.welcome.SetDaemonStatus(msg.isOnline, msg.isSynced, msg.isHealthy, msg.network, msg.address, msg.height, msg.topoHeight)
		// Cache daemon status for fast wallet connection
		m.cachedDaemonHealthy = msg.isOnline && msg.isHealthy
		m.cachedDaemonAddress = msg.address
		// If daemon is online and healthy, save the address
		if msg.isOnline && msg.isHealthy && m.Opts.DaemonAddress == "" {
			m.Opts.DaemonAddress = msg.address
		}
		if m.stickyDaemonAddress != "" && msg.address == m.stickyDaemonAddress {
			m.stickyDaemonHealthy = msg.isOnline && msg.isHealthy
		}

	case daemonConnectMsg:
		if msg.err != nil {
			m.daemon.SetError("Failed to connect: " + msg.err.Error())
		} else {
			// Success - update cached daemon address for display purposes only
			// Do NOT modify m.Opts.Testnet/Simulator as those are CLI flags that
			// should remain constant. Wallet network is determined by saved config.
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
			m.dashboard.SetConnecting(true) // Always show connecting until daemon connection completes
			m.page = PageMain
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
			m.updateWalletInfo()
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
		m.updateWalletInfo()

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
			m.updateWalletInfo() // Refresh balance/status now that we're connected
			// Update daemon address to match what wallet connected to
			// This ensures welcome screen shows correct daemon when wallet is closed
			if msg.daemonAddress != "" {
				m.Opts.DaemonAddress = msg.daemonAddress
				m.cachedDaemonAddress = msg.daemonAddress
				m.cachedDaemonHealthy = true
				if m.stickyDaemonAddress != "" {
					m.stickyDaemonAddress = msg.daemonAddress
					m.stickyDaemonHealthy = true
					m.stickyDaemonTestnet = msg.network == config.NetworkTestnet
					m.stickyDaemonSimulator = msg.network == config.NetworkSimulator
				}
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
			// Daemon connection failed - keep wallet OPEN and show dashboard offline
			// User can use /connect to retry or continue offline
			m.dashboard.SetFlashMessage(msg.err+" - Wallet opened offline. Use /connect to retry.", false)
			m.dashboard.SetConnecting(false)
			// Update wallet info to show offline status
			m.updateWalletInfo()
			// Stay on dashboard (don't close wallet or go back to welcome)
			m.page = PageMain
			cmds = append(cmds, m.setWindowTitleCmd())
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
		cmds = append(cmds, m.setWindowTitleCmd())

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
		cmds = append(cmds, m.setWindowTitleCmd())

	case wallet.XSWDStartedMsg:
		if msg.Err != nil {
			derolog.Error("xswd", "start.failed", "XSWD server failed to start", "error", msg.Err.Error())
			if m.page == PageMain {
				m.dashboard.SetXSWDRunning(false)
				m.dashboard.SetFlashMessage("XSWD: "+msg.Err.Error(), false)
			}
		} else {
			derolog.Info("xswd", "start.success", "XSWD server started")
			m.xswdBridge = msg.Bridge
			m.dashboard.SetXSWDRunning(true)
			m.dashboard.SetFlashMessage("XSWD server running", true)
		}

	case logUpdateMsg:
		// Update debug entries on all pages while enabled
		if m.Opts.Debug {
			m.updateDashboardLogEntries()
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
			m.updateWalletInfo()
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

	case PageIntegratedAddr:
		content = m.integratedAddr.View()

	case PageXSWDAuth:
		content = m.xswdAuth.View()

	case PageXSWDPerm:
		content = m.xswdPerm.View()
	}

	// Default dimensions if not yet received
	width, height := m.width, m.height
	if width == 0 {
		width = 100
	}
	if height == 0 {
		height = 40
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
	} else if m.page == PageQRCode || m.page == PageIntegratedAddr {
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

	contentWidth := 0
	for _, line := range contentLines {
		lineWidth := lipgloss.Width(line)
		if lineWidth > contentWidth {
			contentWidth = lineWidth
		}
	}

	versionStr := "v" + Version
	totalDashes := contentWidth - len(versionStr) - 2
	if totalDashes < 2 {
		totalDashes = 2
	}

	rightDashes := 4
	if totalDashes < rightDashes {
		rightDashes = totalDashes / 2
	}
	leftDashes := totalDashes - rightDashes

	cornerStyle := lipgloss.NewStyle().Foreground(styles.ColorBorder)
	dashStyle := lipgloss.NewStyle().Foreground(styles.ColorBorder)
	leftCorner := cornerStyle.Render("╭")
	rightCorner := cornerStyle.Render("╮")
	leftDashStr := dashStyle.Render(strings.Repeat("─", leftDashes))
	rightDashStr := dashStyle.Render(strings.Repeat("─", rightDashes))
	versionStyled := styles.MutedStyle.Render(versionStr)
	topBorder := leftCorner + leftDashStr + " " + versionStyled + " " + rightDashStr + rightCorner

	borderStyle := lipgloss.NewStyle().Foreground(styles.ColorBorder)
	sideBorder := borderStyle.Render("│")

	framedLines := make([]string, 0, len(contentLines)+2)
	framedLines = append(framedLines, topBorder)
	for _, line := range contentLines {
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
