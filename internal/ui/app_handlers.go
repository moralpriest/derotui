// Copyright 2017-2026 DERO Project. All rights reserved.

package ui

import (
	"context"
	"os"
	"path/filepath"

	"charm.land/bubbles/v2/filepicker"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/deroproject/dero-wallet-cli/internal/config"
	derolog "github.com/deroproject/dero-wallet-cli/internal/log"
	"github.com/deroproject/dero-wallet-cli/internal/ui/pages"
	"github.com/deroproject/dero-wallet-cli/internal/ui/styles"
	"github.com/deroproject/dero-wallet-cli/internal/wallet"
)

func (m *Model) handleWelcomeAction() tea.Cmd {
	action := m.welcome.Action()
	m.welcome.ResetAction()

	// Clear the input field when any valid action is triggered (except preview)
	if action != pages.ActionNone && action != pages.ActionPreviewTheme {
		m.welcome.ResetInput()
	}

	switch action {
	case pages.ActionOpen:
		m.isCreating = false
		m.isRestoringFromSeed = false
		m.isRestoringFromKey = false
		// Reinitialize file picker in current directory
		fp := filepicker.New()
		fp.AllowedTypes = []string{".db"}
		fp.CurrentDirectory, _ = os.Getwd()
		fp.ShowHidden = false
		fp.SetHeight(10)
		fp.KeyMap.Back = key.NewBinding(key.WithKeys("h", "backspace", "left"), key.WithHelp("h", "back"))
		applyFilePickerTheme(&fp)
		m.filePicker = fp
		m.page = PageFilePicker
		return tea.Batch(m.filePicker.Init(), m.setWindowTitleCmd())

	case pages.ActionCreate:
		m.page = PagePassword
		m.password = pages.NewPassword(pages.PasswordModeCreate)
		m.password.SetVersion(Version)
		m.isCreating = true
		m.isRestoringFromSeed = false
		m.isRestoringFromKey = false
		return tea.Batch(m.password.Init(), m.setWindowTitleCmd())

	case pages.ActionRestoreSeed:
		m.seed = pages.NewSeed(pages.SeedModeInput, "")
		m.page = PageSeed
		m.isCreating = false
		m.isRestoringFromSeed = true
		m.isRestoringFromKey = false
		return tea.Batch(m.seed.Init(), m.setWindowTitleCmd())

	case pages.ActionRestoreKey:
		m.keyInput = pages.NewKeyInput()
		m.page = PageKeyInput
		m.isCreating = false
		m.isRestoringFromSeed = false
		m.isRestoringFromKey = true
		return tea.Batch(m.keyInput.Init(), m.setWindowTitleCmd())

	case pages.ActionConnectDaemon:
		// Use current daemon status (what welcome shows) to pre-populate the connect form
		testnet := m.Opts.Testnet
		simulator := m.Opts.Simulator
		if m.cachedDaemonAddress != "" {
			if info := wallet.GetDaemonInfo(context.Background(), m.cachedDaemonAddress); info.IsHealthy {
				testnet = info.Testnet
				simulator = info.Network == "Simulator"
			}
		}
		m.daemon = pages.NewDaemon(testnet, simulator)
		m.page = PageDaemon
		return tea.Batch(m.daemon.Init(), m.setWindowTitleCmd())

	case pages.ActionSwitchNetwork:
		network := "Mainnet"
		switch m.welcome.Network {
		case "Simulator":
			network = "Simulator"
		case "Testnet":
			network = "Testnet"
		default:
			network = "Mainnet"
		}

		nextTestnet := false
		nextSimulator := false
		switch network {
		case "Testnet":
			nextTestnet = true
		case "Simulator":
			nextSimulator = true
		}

		m.daemon = pages.NewDaemon(nextTestnet, nextSimulator)
		m.page = PageDaemon
		return tea.Batch(m.daemon.Init(), m.setWindowTitleCmd())

	case pages.ActionDebug:
		if m.debugEnabled {
			m.debugConsoleOpen = true
			m.debugAutoFollow = true
			m.clampDebugScrollForHeight(m.height)
			m.updateDashboardLogEntries()
			return nil
		}
		if m.Opts.Debug {
			m.debugEnabled = true
			m.debugConsoleOpen = true
			m.debugAutoFollow = true
			m.dashboard.SetDebugEnabled(true)
			m.clampDebugScrollForHeight(m.height)
			m.updateDashboardLogEntries()
			return nil
		}
		return m.toggleDebugLoggingCmd(true)

	case pages.ActionExit:
		return tea.Quit

	case pages.ActionPreviewTheme:
		selectedTheme := m.welcome.SelectedTheme()
		if selectedTheme != "" {
			// Preview the theme (don't save to config yet)
			styles.ApplyTheme(selectedTheme)
			applyFilePickerTheme(&m.filePicker)
		}
		return nil

	case pages.ActionSetTheme:
		selectedTheme := m.welcome.SelectedTheme()
		if selectedTheme != "" {
			// Apply the theme
			if err := styles.ApplyTheme(selectedTheme); err != nil {
				m.welcome.SetError("Failed to apply theme: " + err.Error())
			} else {
				applyFilePickerTheme(&m.filePicker)
				// Save to config
				if err := config.SetTheme(selectedTheme); err != nil {
					m.welcome.SetError("Theme applied but failed to save: " + err.Error())
				} else {
					m.welcome.SetSuccess("Theme changed to " + styles.GetThemeName(selectedTheme))
				}
			}
		}
		return nil
	}

	return nil
}

func (m *Model) handlePasswordAction() tea.Cmd {
	if m.password.Cancelled() {
		// If changing password, go back to dashboard
		if m.isChangingPassword {
			m.password.Reset()
			m.isChangingPassword = false
			m.page = PageMain
			return m.setWindowTitleCmd()
		}
		// Otherwise go to welcome
		m.welcome.ResetInput()
		m.page = PageWelcome
		m.password.Reset()
		m.pendingNetwork = pages.NetworkNone
		m.pendingCreateRestore = ""
		m.pendingPassword = ""
		return tea.Batch(m.checkDaemonStatus(), m.setWindowTitleCmd())
	}

	if m.password.Confirmed() {
		// Clear confirmed immediately to prevent re-triggering on subsequent messages
		m.password.ClearConfirmed()

		password := m.password.Password()
		walletName := m.password.WalletName()

		if m.isChangingPassword {
			currentPass := m.password.CurrentPassword()
			return m.changeWalletPassword(currentPass, password)
		} else if m.isCreating || m.isRestoringFromSeed || m.isRestoringFromKey {
			// For create/restore, check if we need to prompt for network first
			// Only skip if explicit CLI flags are provided
			needsNetworkPrompt := !m.Opts.ExplicitTestnet && !m.Opts.ExplicitSimulator

			if needsNetworkPrompt {
				derolog.Info("wallet", "create_restore.network_prompt", "Showing network selection before create/restore")
				m.pendingPassword = password
				m.pendingNetwork = pages.NetworkMainnet // Default selection in UI
				m.pendingCreateRestore = walletName
				m.network = pages.NewNetwork(walletName)
				m.page = PageNetwork
				return m.network.Init()
			}

			// No network prompt needed - use CLI flags or default to mainnet
			if m.isCreating {
				derolog.Info("wallet", "create.start", "Creating new wallet", "name", walletName)
				return m.createWallet(walletName, password)
			} else if m.isRestoringFromSeed {
				derolog.Info("wallet", "restore.seed.start", "Restoring wallet from seed", "name", walletName)
				return m.restoreWallet(walletName, password, m.Opts.ElectrumSeed)
			} else {
				derolog.Info("wallet", "restore.key.start", "Restoring wallet from key", "name", walletName)
				return m.restoreWalletFromKey(walletName, password, m.pendingKey)
			}
		} else {
			if m.pendingNetwork != pages.NetworkNone {
				selection := m.pendingNetwork
				m.pendingNetwork = pages.NetworkNone
				derolog.Info("wallet", "open.with_pending_network", "Opening wallet with previously selected network")
				return m.openWalletWithNetwork(m.walletFile, password, selection)
			}

			// Pre-flight network check before opening wallet
			if !m.PreFlightNetworkCheck(m.walletFile) {
				derolog.Info("wallet", "open.network_unknown", "Network unknown, showing selection")
				m.pendingPassword = password
				m.network = pages.NewNetwork(m.walletFile)
				m.page = PageNetwork
				return m.network.Init()
			}
			derolog.Info("wallet", "open.start", "Opening wallet", "file", filepath.Base(m.walletFile))
			return m.tryOpenWallet(m.walletFile, password)
		}
	}

	return nil
}

func (m *Model) handleNetworkAction() tea.Cmd {
	if m.network.Cancelled() {
		m.network.Reset()
		m.pendingPassword = ""
		m.pendingNetwork = pages.NetworkNone
		m.pendingCreateRestore = ""
		// Go back to appropriate page based on what we were doing
		if m.isCreating || m.isRestoringFromSeed || m.isRestoringFromKey {
			// Go back to password page for create/restore
			m.page = PagePassword
			return m.password.Init()
		}
		// For open wallet flow, go back to welcome
		m.welcome.ResetInput()
		m.page = PageWelcome
		return m.checkDaemonStatus()
	}

	if m.network.Confirmed() {
		selection := m.network.Selection()
		password := m.pendingPassword
		file := m.walletFile
		createRestoreName := m.pendingCreateRestore

		// Clear pending state
		m.pendingPassword = ""
		m.pendingCreateRestore = ""
		m.pendingNetwork = selection
		m.network.Reset()

		// Save the selected network for this wallet
		var networkConfig config.WalletNetwork
		var networkName string
		switch selection {
		case pages.NetworkMainnet:
			networkConfig = config.NetworkMainnet
			networkName = "mainnet"
		case pages.NetworkTestnet:
			networkConfig = config.NetworkTestnet
			networkName = "testnet"
		case pages.NetworkSimulator:
			networkConfig = config.NetworkSimulator
			networkName = "simulator"
		}

		// Check if we're in create/restore flow (no existing wallet file)
		isCreateRestoreFlow := m.isCreating || m.isRestoringFromSeed || m.isRestoringFromKey

		if isCreateRestoreFlow {
			// For create/restore, use explicit selection captured above.
			derolog.Info("wallet", "create_restore.network_selected", "Selected network for create/restore", "network", networkName)

			// Now proceed with create/restore
			walletName := createRestoreName
			if walletName == "" {
				walletName = m.password.WalletName()
			}

			if m.isCreating {
				derolog.Info("wallet", "create.start", "Creating new wallet", "name", walletName)
				return m.createWallet(walletName, password)
			} else if m.isRestoringFromSeed {
				derolog.Info("wallet", "restore.seed.start", "Restoring wallet from seed", "name", walletName)
				return m.restoreWallet(walletName, password, m.Opts.ElectrumSeed)
			} else {
				derolog.Info("wallet", "restore.key.start", "Restoring wallet from key", "name", walletName)
				return m.restoreWalletFromKey(walletName, password, m.pendingKey)
			}
		}

		// Existing wallet flow - save network and open
		if file != "" {
			if err := config.SetWalletNetwork(file, networkConfig); err != nil {
				derolog.Warn("wallet", "config.network_save_failed", "Failed saving wallet network", "error", err.Error(), "file", file, "network", networkName)
			} else {
				derolog.Info("wallet", "network.saved", "Saved network for wallet", "network", networkName, "file", filepath.Base(file))
			}
		}

		// If no password (auto-open flow), redirect to password page
		if password == "" {
			m.password = pages.NewPassword(pages.PasswordModeUnlock)
			m.password.SetVersion(Version)
			m.password.SetWalletFile(file)
			m.page = PagePassword
			return tea.Batch(m.password.Init(), m.setWindowTitleCmd())
		}

		// Open wallet with selected network
		derolog.Info("wallet", "open.with_network", "Opening wallet with network", "network", networkName)
		return tea.Batch(m.openWalletWithNetwork(file, password, selection), m.setWindowTitleCmd())
	}

	return nil
}

func (m *Model) handleDaemonAction() tea.Cmd {
	if m.daemon.Cancelled() {
		m.daemon.Reset()
		m.welcome.ResetInput()
		m.page = PageWelcome
		return tea.Batch(m.checkDaemonStatus(), m.setWindowTitleCmd())
	}

	if m.daemon.Confirmed() {
		address := m.daemon.Address()
		derolog.Info("daemon", "connect.start", "Connecting to daemon", "address", address)

		// Reset confirmed flag to prevent repeated connection attempts
		m.daemon.ResetConfirmed()

		// Return a command to connect asynchronously
		return m.connectToDaemon(address)
	}

	return nil
}

func (m *Model) handleSeedAction() tea.Cmd {
	if m.seed.Cancelled() {
		// If wallet is already open, go back to main page
		if m.wallet != nil {
			m.page = PageMain
			return m.setWindowTitleCmd()
		}
		// Otherwise go to welcome
		m.welcome.ResetInput()
		m.page = PageWelcome
		return tea.Batch(m.checkDaemonStatus(), m.setWindowTitleCmd())
	}

	if m.seed.Confirmed() {
		if m.isRestoringFromSeed {
			// Restoring - need password first
			seed := m.seed.GetSeed()
			m.page = PagePassword
			m.password = pages.NewPassword(pages.PasswordModeCreate)
			m.password.SetVersion(Version)
			return tea.Batch(m.password.Init(), m.storeRestoreSeed(seed), m.setWindowTitleCmd())
		}

		// Displaying seed (after creation or from dashboard)
		m.page = PageMain
		m.updateWalletInfo()
		// Note: m.tickCmd() is already running from Init(); do not add duplicate tickers
		return m.setWindowTitleCmd()
	}

	return nil
}

func (m *Model) handleKeyInputAction() tea.Cmd {
	if m.keyInput.Cancelled() {
		// If displaying key from an open wallet, go back to main
		if m.keyInput.IsDisplayMode() && m.wallet != nil {
			m.page = PageMain
			return m.setWindowTitleCmd()
		}
		// Otherwise go to welcome (restoring from key was cancelled)
		m.welcome.ResetInput()
		m.page = PageWelcome
		m.keyInput.Reset()
		m.isRestoringFromKey = false
		return tea.Batch(m.checkDaemonStatus(), m.setWindowTitleCmd())
	}

	if m.keyInput.Confirmed() {
		// If displaying key, go back to main
		if m.keyInput.IsDisplayMode() {
			m.page = PageMain
			return m.setWindowTitleCmd()
		}
		// Restoring from key - store the key and go to password page
		m.pendingKey = m.keyInput.GetKey()
		m.page = PagePassword
		m.password = pages.NewPassword(pages.PasswordModeCreate)
		m.password.SetVersion(Version)
		return tea.Batch(m.password.Init(), m.setWindowTitleCmd())
	}

	return nil
}

func (m *Model) handleDashboard(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd

	m.dashboard, cmd = m.dashboard.Update(msg)

	// Handle dashboard actions
	if m.dashboard.WantSend() {
		m.dashboard.ResetActions()
		if m.wallet != nil {
			info := m.wallet.GetInfo()
			if !info.IsRegistered {
				m.dashboard.SetFlashMessage("Register wallet first ([G])", false)
				return cmd
			}
		}
		// Set simulator flag before reset so ringsize defaults correctly
		if m.wallet != nil {
			m.send.SetSimulator(m.wallet.IsSimulator())
		}
		m.send.Reset() // Clear any leftover state from previous use
		// Set balance before entering send page
		if m.wallet != nil {
			info := m.wallet.GetInfo()
			m.send.SetBalance(info.Balance)
		}
		m.page = PageSend
		return tea.Batch(m.send.Init(), m.setWindowTitleCmd())
	}
	if m.dashboard.WantCopy() {
		m.dashboard.ResetActions()
		m.copyAddress()
	}
	if m.dashboard.WantViewQR() {
		m.dashboard.ResetActions()
		if m.wallet != nil {
			info := m.wallet.GetInfo()
			m.qrcode = pages.NewQRCode(info.Address)
			m.qrReturnPage = PageMain
			m.page = PageQRCode
			return m.setWindowTitleCmd()
		}
	}
	if m.dashboard.WantViewSeed() {
		m.dashboard.ResetActions()
		if m.wallet != nil {
			seed := m.wallet.GetSeed()
			m.seed = pages.NewSeed(pages.SeedModeDisplay, seed)
			m.page = PageSeed
			return m.setWindowTitleCmd()
		}
	}
	if m.dashboard.WantViewKey() {
		m.dashboard.ResetActions()
		if m.wallet != nil {
			hexKey := m.wallet.GetHexKey()
			m.keyInput = pages.NewKeyInputDisplay(hexKey)
			m.page = PageKeyInput
			return m.setWindowTitleCmd()
		}
	}
	if m.dashboard.WantHistory() {
		m.dashboard.ResetActions()
		m.page = PageHistory
		return m.setWindowTitleCmd()
	}
	if m.dashboard.WantRegister() {
		m.dashboard.ResetActions()
		if m.wallet == nil {
			m.dashboard.SetFlashMessage("Wallet not open", false)
			return cmd
		}
		info := m.wallet.GetInfo()
		if info.IsRegistered {
			m.clearPendingRegistration()
			m.dashboard.SetFlashMessage("Wallet is already registered", true)
			return cmd
		}
		m.clearPendingRegistration()
		m.dashboard.SetRegistering(true)
		m.dashboard.SetFlashMessage("Registration in progress...", true)
		return tea.Batch(m.registerWalletCmd(), cmd)
	}
	if m.dashboard.WantChangePassword() {
		m.dashboard.ResetActions()
		m.password = pages.NewPassword(pages.PasswordModeChange)
		m.password.SetVersion(Version)
		m.isChangingPassword = true
		m.isCreating = false
		m.isRestoringFromSeed = false
		m.isRestoringFromKey = false
		m.page = PagePassword
		return tea.Batch(m.password.Init(), m.setWindowTitleCmd())
	}
	if m.dashboard.WantXSWD() {
		m.dashboard.ResetActions()
		// Toggle XSWD server
		if m.xswdBridge != nil {
			// Stop XSWD
			derolog.Info("xswd", "stop.request", "Stopping XSWD server")
			m.xswdBridge.Stop()
			m.xswdBridge = nil
			m.dashboard.SetXSWDRunning(false)
			m.dashboard.SetFlashMessage("XSWD server stopped", true)
		} else {
			// Start XSWD
			derolog.Info("xswd", "start.request", "Starting XSWD server")
			m.dashboard.SetXSWDRunning(true) // Optimistic - will correct if fails
			m.dashboard.SetFlashMessage("XSWD server starting...", true)
			return tea.Batch(m.startXSWDCmd(), cmd)
		}
	}
	if m.dashboard.WantIntegratedAddr() {
		m.dashboard.ResetActions()
		if m.wallet != nil {
			info := m.wallet.GetInfo()
			m.integratedAddr = pages.NewIntegratedAddr(info.Address)
			m.page = PageIntegratedAddr
			return tea.Batch(m.integratedAddr.Init(), m.setWindowTitleCmd())
		}
	}
	if m.dashboard.WantDonate() {
		m.dashboard.ResetActions()
		if m.wallet != nil {
			info := m.wallet.GetInfo()
			if !info.IsRegistered {
				m.dashboard.SetFlashMessage("Register wallet first ([G])", false)
				return cmd
			}
		}
		m.send = pages.NewSend()
		m.send.SetSimulator(m.wallet.IsSimulator())
		m.send.SetBalance(m.wallet.GetInfo().Balance)
		m.send.SetAddress(donationAddress)
		m.page = PageSend
		return tea.Batch(m.send.Init(), m.setWindowTitleCmd())
	}
	return cmd
}
