// Copyright 2017-2026 DERO Project. All rights reserved.

package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/deroproject/derohe/globals"
	"github.com/deroproject/derohe/walletapi"
	"golang.org/x/sys/unix"

	"github.com/deroproject/dero-wallet-cli/internal/config"
	"github.com/deroproject/dero-wallet-cli/internal/ui"
	"github.com/deroproject/dero-wallet-cli/internal/ui/styles"
	"github.com/deroproject/dero-wallet-cli/internal/wallet"
)

var (
	version   = "0.1.0"
	commit    = ""
	date      = ""
	buildType = "dev"
)

func main() {
	// Suppress DERO library console output to prevent TUI corruption
	// The library writes directly to stdout which breaks the Bubble Tea TUI
	globals.InitializeLog(io.Discard, io.Discard)

	// Initialize DERO lookup table for balance decryption (required)
	// This must be called before any wallet operations
	walletapi.Initialize_LookupTable(1, 1<<21) // ~32 MB RAM

	opts := parseFlags()

	// Setup logging based on --debug flag
	ui.SetupLogging(opts.Debug)

	// Load and apply saved theme
	savedTheme := config.GetTheme()
	styles.ApplyTheme(savedTheme)

	if !opts.Offline {
		// Check for running local daemons and prompt if multiple found
		// This runs before checkLastWallet so user can choose network first
		detectAndPromptDaemon(&opts)
	}

	// Auto-open last wallet if it matches the selected/detected network
	checkLastWallet(&opts)

	// Initialize globals.Arguments map with required keys for globals.Initialize()
	globals.Arguments = map[string]interface{}{}
	globals.Arguments["--testnet"] = opts.Testnet
	globals.Arguments["--simulator"] = opts.Simulator
	globals.Arguments["--debug"] = opts.Debug
	globals.Arguments["--offline"] = opts.Offline
	globals.Arguments["--offline-datafile"] = opts.OfflineFile
	globals.Arguments["--rpc-server"] = opts.RPCServer
	globals.Arguments["--rpc-bind"] = opts.RPCBind
	globals.Arguments["--rpc-login"] = opts.RPCLogin
	globals.Arguments["--allow-rpc-password-change"] = opts.RPCPassChange
	globals.Arguments["--unlocked"] = opts.Unlocked
	if opts.SocksProxy != "" {
		globals.Arguments["--socks-proxy"] = opts.SocksProxy
	} else {
		globals.Arguments["--socks-proxy"] = nil
	}

	// Initialize DERO globals (network config, proxy settings, etc.)
	globals.Initialize()

	fmt.Printf("DERO Wallet TUI v%s (%s/%s)\n", version, runtime.GOOS, runtime.GOARCH)
	fmt.Println("Copyright 2017-2026 DERO Project. All rights reserved.")
	fmt.Println()

	// Set daemon address - use CLI option or default to localhost
	daemonAddr := opts.DaemonAddress
	if daemonAddr == "" {
		if opts.Simulator {
			daemonAddr = wallet.DefaultSimulatorDaemon
		} else if opts.Testnet {
			daemonAddr = wallet.DefaultTestnetDaemon
			// Check if localhost testnet is available, fallback to remote if not
			if !wallet.IsDaemonHealthy(context.Background(), daemonAddr) {
				daemonAddr = wallet.FallbackTestnetDaemon
			}
		} else {
			daemonAddr = wallet.DefaultMainnetDaemon
			if !wallet.IsDaemonHealthy(context.Background(), daemonAddr) {
				daemonAddr = wallet.FallbackMainnetDaemon
			}
		}
	}
	if daemonAddr != "" {
		normalizedDaemon, err := wallet.NormalizeDaemonAddress(daemonAddr)
		if err != nil {
			fmt.Printf("Invalid daemon address %q: %v\n", daemonAddr, err)
			os.Exit(1)
		}
		daemonAddr = normalizedDaemon
		globals.Arguments["--daemon-address"] = daemonAddr
		// Set both endpoints for the walletapi
		walletapi.Daemon_Endpoint_Active = daemonAddr
		walletapi.SetDaemonAddress(daemonAddr)
	}

	// Start daemon connectivity maintenance in background only if daemon is healthy
	go func() {
		defer func() {
			if r := recover(); r != nil {
				// Silently recover - background connectivity failed but TUI continues
			}
		}()

		// Only start Keep_Connectivity if daemon is responding correctly
		if daemonAddr != "" && wallet.IsDaemonHealthy(context.Background(), daemonAddr) {
			walletapi.Keep_Connectivity()
		}
	}()

	m := ui.NewModel()
	m.Opts = opts
	m.SetStartupFlowSet(m.ApplyCLIStartupFlags())

	// Save original stdout/stderr file descriptors for Bubble Tea
	// Then redirect at OS level to /dev/null to prevent DERO library corruption
	origStdout, err := unix.Dup(int(os.Stdout.Fd()))
	if err != nil {
		fmt.Printf("Warning: failed to dup stdout: %v\n", err)
		origStdout = -1
	}
	origStderr, err := unix.Dup(int(os.Stderr.Fd()))
	if err != nil {
		fmt.Printf("Warning: failed to dup stderr: %v\n", err)
		origStderr = -1
	}

	// Open real /dev/tty for Bubble Tea output so color detection works.
	// os.NewFile on a duped fd can be detected as non-TTY and strip styles.
	ttyOut, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	if err != nil {
		ttyOut = os.Stdout
	} else {
		defer ttyOut.Close()
	}

	// Redirect stdout/stderr to /dev/null at OS level
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err == nil {
		unix.Dup2(int(devNull.Fd()), int(os.Stdout.Fd()))
		unix.Dup2(int(devNull.Fd()), int(os.Stderr.Fd()))
		devNull.Close()
	}

	// Restore stdout/stderr after TUI exits
	defer func() {
		if origStdout >= 0 {
			unix.Dup2(origStdout, int(os.Stdout.Fd()))
			unix.Close(origStdout)
		}
		if origStderr >= 0 {
			unix.Dup2(origStderr, int(os.Stderr.Fd()))
			unix.Close(origStderr)
		}
	}()

	// Pass the original stdout to Bubble Tea explicitly
	p := tea.NewProgram(&m, tea.WithOutput(ttyOut))

	// Store program reference for XSWD bridge message injection
	m.SetProgram(&programAdapter{p})

	// Setup signal handler to trigger graceful shutdown via tea.Program
	setupSignalHandler(p)

	if _, err := p.Run(); err != nil {
		// Restore stdout before printing
		if origStdout >= 0 {
			unix.Dup2(origStdout, int(os.Stdout.Fd()))
		}
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

func setupSignalHandler(p *tea.Program) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		// Trigger graceful shutdown of the tea.Program.
		// This allows the Model to handle cleanup (wallet.Close(), etc.)
		// before exiting, rather than using os.Exit which skips defers.
		p.Quit()
	}()
}

func parseFlags() ui.CLIOptions {
	opts := ui.CLIOptions{}

	flag.StringVar(&opts.WalletFile, "wallet-file", "", "Wallet file path")
	flag.StringVar(&opts.Password, "password", "", "Wallet password")
	flag.BoolVar(&opts.Offline, "offline", false, "Run in offline mode")
	flag.StringVar(&opts.OfflineFile, "offline-datafile", "getoutputs.bin", "Offline mode data file")
	flag.BoolVar(&opts.RPCServer, "rpc-server", false, "Enable RPC server")
	flag.StringVar(&opts.RPCBind, "rpc-bind", "127.0.0.1:20209", "RPC server bind address")
	flag.StringVar(&opts.RPCLogin, "rpc-login", "", "RPC server credentials (username:password)")
	flag.BoolVar(&opts.RPCPassChange, "allow-rpc-password-change", false, "Allow RPC password change")
	flag.BoolVar(&opts.Testnet, "testnet", false, "Use testnet")
	flag.BoolVar(&opts.Simulator, "simulator", false, "Connect to simulator (port 20000)")
	flag.StringVar(&opts.DaemonAddress, "daemon-address", "", "Daemon address (host:port or https://domain)")
	flag.StringVar(&opts.SocksProxy, "socks-proxy", "", "SOCKS proxy (ip:port)")
	flag.BoolVar(&opts.GenerateNew, "generate-new-wallet", false, "Generate new wallet")
	flag.BoolVar(&opts.RestoreSeed, "restore-deterministic-wallet", false, "Restore wallet from seed")
	flag.StringVar(&opts.ElectrumSeed, "electrum-seed", "", "Seed for wallet recovery")
	flag.BoolVar(&opts.Unlocked, "unlocked", false, "Keep wallet unlocked")
	flag.BoolVar(&opts.Debug, "debug", false, "Enable debug logging")

	flag.Parse()

	// Track whether network flags were explicitly provided by user.
	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "testnet":
			opts.ExplicitTestnet = true
		case "simulator":
			opts.ExplicitSimulator = true
		}
	})

	// Accept daemon address as positional argument (e.g., "derotui --testnet host:port")
	if opts.DaemonAddress == "" && flag.NArg() > 0 {
		arg := flag.Arg(0)
		// If it looks like a host:port or hostname, use it as daemon address
		if strings.Contains(arg, ":") || strings.Contains(arg, ".") {
			opts.DaemonAddress = arg
		}
	}

	return opts
}

// checkLastWallet auto-opens the last wallet if its network matches the current selection.
// Returns true if opts.WalletFile was set.
func checkLastWallet(opts *ui.CLIOptions) bool {
	// Skip if wallet file already specified
	if opts.WalletFile != "" {
		return false
	}

	// Check for last wallet
	lastWallet := config.GetLastWallet()
	if lastWallet == "" {
		return false
	}

	// Get saved network for this wallet
	savedNetwork := config.GetWalletNetwork(lastWallet)
	if savedNetwork == "" {
		return false // Network unknown, let TUI handle selection
	}

	// Set network flags based on saved wallet network to ensure consistency.
	// This also prevents the daemon detection prompt from appearing.
	if savedNetwork == config.NetworkTestnet {
		opts.Testnet = true
		opts.Simulator = false
	} else if savedNetwork == config.NetworkSimulator {
		opts.Simulator = true
		opts.Testnet = false
	} else if savedNetwork == config.NetworkMainnet {
		opts.Testnet = false
		opts.Simulator = false
	}

	// Check if last wallet's network matches current selection
	currentIsSimulator := opts.Simulator
	currentIsTestnet := opts.Testnet && !opts.Simulator
	currentIsMainnet := !opts.Testnet && !opts.Simulator

	walletMatchesNetwork := (savedNetwork == config.NetworkSimulator && currentIsSimulator) ||
		(savedNetwork == config.NetworkTestnet && currentIsTestnet) ||
		(savedNetwork == config.NetworkMainnet && currentIsMainnet)

	if !walletMatchesNetwork {
		return false // Last wallet is for different network
	}

	opts.WalletFile = lastWallet
	return true
}

// detectAndPromptDaemon checks for running local daemons and prompts user to choose if multiple found.
func detectAndPromptDaemon(opts *ui.CLIOptions) {
	// Skip if network already specified
	if opts.Testnet || opts.Simulator || opts.DaemonAddress != "" {
		return
	}

	daemons := []daemonInfo{
		{"Mainnet", wallet.DefaultMainnetDaemon, func() {}},
		{"Testnet", wallet.DefaultTestnetDaemon, func() { opts.Testnet = true }},
		{"Simulator", wallet.DefaultSimulatorDaemon, func() {
			opts.Simulator = true
			// Check if simulator is running in mainnet or testnet mode
			info := wallet.GetDaemonInfo(context.Background(), wallet.DefaultSimulatorDaemon)
			if info.Testnet {
				opts.Testnet = true
			}
		}},
	}

	// Check which daemons are running
	var available []daemonInfo
	for _, d := range daemons {
		if wallet.CheckDaemonFast(d.address) {
			available = append(available, d)
		}
	}

	// No prompt needed if 0 or 1 daemon running
	if len(available) <= 1 {
		if len(available) == 1 {
			available[0].setOpts()
		}
		return
	}

	// Multiple daemons - render styled prompt
	renderDaemonSelection(available, opts)
}

type daemonInfo struct {
	name    string
	address string
	setOpts func()
}

// daemonSelectModel is a bubbletea model for daemon selection
type daemonSelectModel struct {
	available     []daemonInfo
	selected      int
	chosen        bool
	width         int
	height        int
	lastClickTime time.Time
	lastClickIdx  int
}

// default dimensions for calculations before WindowSizeMsg is received
const defaultTermWidth = 80
const defaultTermHeight = 24

func (m daemonSelectModel) Init() tea.Cmd {
	return nil
}

func (m daemonSelectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.MouseClickMsg:
		if msg.Button == tea.MouseLeft {
			// Use current dimensions or defaults
			width := m.width
			if width == 0 {
				width = defaultTermWidth
			}
			height := m.height
			if height == 0 {
				height = defaultTermHeight
			}

			// Calculate the box position (centered)
			boxWidth := 60
			boxX := (width - boxWidth) / 2
			if boxX < 0 {
				boxX = 0
			}

			// Simplified X check: just ensure click is within the box horizontally
			// Box has 1 char border on each side, so inner width is boxWidth - 2
			mouseX, mouseY := msg.Mouse().X, msg.Mouse().Y
			if mouseX < boxX+1 || mouseX >= boxX+boxWidth-1 {
				return m, nil
			}

			// Calculate which row was clicked
			// The box always starts at Y=1 (after leading \n in View)
			// Box structure:
			// Y=1: Top border
			// Y=2: Top padding
			// Y=3-8: Logo (6 lines)
			// Y=9: blank
			// Y=10: "Multiple Daemons Detected" title
			// Y=11: blank
			// Y=12: "Select which network to connect to:" subtitle
			// Y=13: blank
			// Y=14+: Options start (1 line per option)
			// So options start at absolute Y=14

			optionsStartY := 14

			// Calculate which option was clicked
			relY := mouseY - optionsStartY
			if relY >= 0 && relY < len(m.available) {
				// Check for double-click (within 500ms on same option)
				now := time.Now()
				if relY == m.lastClickIdx && now.Sub(m.lastClickTime) < 500*time.Millisecond {
					m.selected = relY
					m.chosen = true
					return m, tea.Quit
				}
				m.selected = relY
				m.lastClickIdx = relY
				m.lastClickTime = now
			}
		}

	case tea.KeyPressMsg:
		switch msg.String() {
		case "up", "k":
			if m.selected > 0 {
				m.selected--
			}
		case "down", "j":
			if m.selected < len(m.available)-1 {
				m.selected++
			}
		case "enter":
			m.chosen = true
			return m, tea.Quit
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m daemonSelectModel) View() tea.View {
	// Use theme-aware styles from styles package
	// Box width minus padding (3 each side) minus border (1 each side)
	contentWidth := 52

	boxStyle := styles.ThemedBoxStyle().
		Padding(1, 3).
		Width(60)

	// Helper to center text within content width
	center := func(s string) string {
		return lipgloss.PlaceHorizontal(contentWidth, lipgloss.Center, s)
	}

	// Build content
	logo := styles.Logo()
	title := styles.TitleStyle.Render("Multiple Daemons Detected")
	subtitle := styles.MutedStyle.Render("Select which network to connect to:")

	var optionLines []string
	for i, d := range m.available {
		var line string
		addr := styles.MutedStyle.Render(fmt.Sprintf("(%s)", d.address))

		if i == m.selected {
			// Selected: arrow + bold white text (consistent with unified selection pattern)
			cursor := styles.SelectedMenuItemStyle.Render("▸ ")
			name := styles.TextStyle.Render(d.name)
			line = cursor + name + " " + addr
		} else {
			// Unselected: no arrow + muted text
			name := styles.MutedStyle.Render(d.name)
			line = "  " + name + " " + addr
		}

		optionLines = append(optionLines, line)
	}
	options := lipgloss.JoinVertical(lipgloss.Left, optionLines...)

	help := styles.MutedStyle.Render("↑/↓ Navigate • Enter/Double-click Select • q Quit")

	content := lipgloss.JoinVertical(lipgloss.Left,
		center(logo),
		"",
		center(title),
		"",
		center(subtitle),
		"",
		center(options),
		"",
		center(help),
	)

	// Use model's tracked width or default
	termWidth := m.width
	if termWidth == 0 {
		termWidth = defaultTermWidth
	}

	// Render and center the box
	box := boxStyle.Render(content)
	v := tea.NewView("\n" + lipgloss.PlaceHorizontal(termWidth, lipgloss.Center, box) + "\n")
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

// renderDaemonSelection displays a styled daemon selection prompt
func renderDaemonSelection(available []daemonInfo, opts *ui.CLIOptions) {
	m := daemonSelectModel{available: available}
	p := tea.NewProgram(m)

	finalModel, err := p.Run()
	if err != nil {
		os.Exit(0)
	}

	if result, ok := finalModel.(daemonSelectModel); ok {
		if result.chosen {
			available[result.selected].setOpts()
		} else {
			// User pressed q or ctrl+c
			os.Exit(0)
		}
	}
}

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `DERO Wallet TUI v%s

Usage: derotui [options]

Options:
`, version)
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Examples:
  derotui                                                    # Interactive mode
  derotui --wallet-file mywallet.db                          # Open specific wallet
  derotui --testnet                                          # Use testnet (local daemon)
  derotui --testnet testnet.dero.io:40402                    # Use remote testnet daemon
  derotui --daemon-address node.example.com:10102            # Use custom daemon
  derotui --simulator                                        # Connect to simulator
  derotui --rpc-server                                       # Enable RPC server

For more information, see https://github.com/deroproject/dero-wallet-cli
`)
	}
}

// programAdapter wraps *tea.Program to satisfy wallet.MsgSender interface.
// tea.Program.Send takes tea.Msg (a named interface{}) but MsgSender.Send
// takes interface{}, so we need this thin adapter for type compatibility.
type programAdapter struct {
	p *tea.Program
}

func (a *programAdapter) Send(msg interface{}) {
	a.p.Send(msg)
}
