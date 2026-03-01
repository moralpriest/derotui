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
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/atotto/clipboard"

	"github.com/deroproject/dero-wallet-cli/internal/config"
	derolog "github.com/deroproject/dero-wallet-cli/internal/log"
	"github.com/deroproject/dero-wallet-cli/internal/ui/pages"
	"github.com/deroproject/dero-wallet-cli/internal/wallet"
	"github.com/deroproject/derohe/globals"
	"github.com/deroproject/derohe/walletapi"
)

// connectToDaemon returns a command that connects to the daemon.
func (m *Model) connectToDaemon(address string) tea.Cmd {
	return func() (result tea.Msg) {
		// Recover from any panics in the walletapi library
		defer func() {
			if r := recover(); r != nil {
				result = daemonConnectMsg{address: address, err: fmt.Errorf("connection failed (internal error): %v", r)}
			}
		}()

		normalizedAddress, err := wallet.NormalizeDaemonAddress(address)
		if err != nil {
			return daemonConnectMsg{address: address, err: err}
		}

		// Query daemon info to detect network type
		info := wallet.GetDaemonInfo(context.Background(), normalizedAddress)
		if !info.IsHealthy {
			return daemonConnectMsg{address: normalizedAddress, err: fmt.Errorf("daemon at %s is not responding correctly", normalizedAddress)}
		}

		// Auto-switch network to match daemon BEFORE connecting
		// This prevents "Mainnet/TestNet is different between wallet/daemon" error
		globals.Arguments["--testnet"] = info.Testnet
		globals.InitNetwork() // Re-initialize network config to apply the change

		// Update the global daemon endpoint
		walletapi.Daemon_Endpoint_Active = normalizedAddress
		walletapi.SetDaemonAddress(normalizedAddress)

		// Try to connect via WebSocket
		err = walletapi.Connect(normalizedAddress)
		return daemonConnectMsg{
			address: normalizedAddress,
			err:     err,
			testnet: info.Testnet,
			network: info.Network,
		}
	}
}

func (m *Model) storeRestoreSeed(seed string) tea.Cmd {
	m.Opts.ElectrumSeed = seed
	return nil
}

// toggleDebugLoggingCmd returns a command that toggles debug logging on/off.
// It performs side effects in the command and returns a message that mutates model state in Update.
func (m Model) toggleDebugLoggingCmd(openConsole bool) tea.Cmd {
	enable := !m.Opts.Debug
	return func() tea.Msg {
		if !enable {
			// Disable debug logging
			log.SetOutput(io.Discard)
			derolog.SetOutput(io.Discard)
			CloseLogFile()
			return debugToggleResultMsg{enabled: false}
		}

		cwd, err := os.Getwd()
		if err != nil {
			cwd = "."
		}
		logPath := filepath.Join(cwd, "derotui-debug.log")
		logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
		if err != nil {
			return debugToggleResultMsg{err: err}
		}

		DebugLogPath = logPath
		configureDebugLogging(logFile)

		return debugToggleResultMsg{enabled: true, logPath: logPath, open: openConsole}
	}
}

func (m *Model) updateWalletInfo() {
	if m.wallet == nil {
		return
	}

	info := m.wallet.GetInfo()

	// Update dashboard with all wallet info
	m.dashboard.SetBalance(info.Balance, info.LockedBalance)
	m.dashboard.SetAddress(info.Address)
	m.dashboard.SetWalletInfo(
		m.wallet.GetFileName(),
		info.Network,
		info.IsOnline,
		info.IsSynced,
		info.IsRegistered,
		info.DaemonAddress,
		info.Height,
		info.DaemonHeight,
	)

	if m.pendingRegTxID != "" {
		if info.IsRegistered {
			m.clearPendingRegistration()
			m.dashboard.SetFlashMessage("Wallet registration confirmed", true)
		} else {
			status := "pending"
			if m.pendingRegStatus != "" {
				status = m.pendingRegStatus
			}
			if info.DaemonAddress != "" && info.DaemonAddress != "Not connected" {
				txStatus, err := wallet.GetTxStatus(context.Background(), info.DaemonAddress, m.pendingRegTxID)
				if err == nil && txStatus.Found {
					status = txStatus.Status
					m.pendingRegStatus = status
					if txStatus.Rejected {
						m.clearPendingRegistration()
						m.dashboard.SetFlashMessage("Registration rejected by daemon. Press [G] to retry.", false)
					}
				}
			}

			if m.pendingRegTxID != "" {
				if m.pendingRegHeight == 0 {
					m.pendingRegHeight = info.DaemonHeight
				}
				if info.DaemonHeight > 0 && m.pendingRegHeight > 0 && info.DaemonHeight >= m.pendingRegHeight+registrationConfirmTimeoutBlocks {
					m.clearPendingRegistration()
					m.dashboard.SetFlashMessage("Registration still unconfirmed after 20 blocks. Press [G] to retry.", false)
				} else {
					m.dashboard.SetRegistrationPending(m.pendingRegTxID, status)
				}
			}
		}
	} else {
		m.dashboard.SetRegistrationPending("", "")
	}

	// Update send balance
	m.send.SetBalance(info.Balance)

	// Update transactions
	txs := m.wallet.GetTransactions(50)
	var pageTxs []pages.Transaction
	for _, tx := range txs {
		pageTxs = append(pageTxs, pages.Transaction{
			TxID:            tx.TxID,
			Amount:          tx.Amount,
			Height:          tx.Height,
			TopoHeight:      tx.TopoHeight,
			Timestamp:       wallet.FormatTimestamp(tx.Timestamp),
			Coinbase:        tx.Coinbase,
			Incoming:        tx.Incoming,
			Fee:             tx.Fee,
			BlockHash:       tx.BlockHash,
			Proof:           tx.Proof,
			Sender:          tx.Sender,
			Destination:     tx.Destination,
			Burn:            tx.Burn,
			DestinationPort: tx.DestinationPort,
			SourcePort:      tx.SourcePort,
			Status:          tx.Status,
			Message:         tx.Message,
		})
	}
	pageTxs = m.mergePendingOutgoing(pageTxs)
	m.dashboard.SetRecentTxs(pageTxs)
	m.history.SetTransactions(pageTxs)

	if m.page == PageMain && !info.IsRegistered && !m.regHintShown {
		m.dashboard.SetFlashMessage("Wallet not registered. Press [G] to register.", false)
		m.regHintShown = true
	}

	// Update log entries if debug is enabled
	if m.Opts.Debug {
		m.updateDashboardLogEntries()
	}
}

// updateDashboardLogEntries updates the global debug log entries (filtered for high-signal).
func (m *Model) updateDashboardLogEntries() {
	buffer := GetLogBuffer()
	if buffer == nil {
		return
	}

	entries := buffer.GetEntries()

	// Filter to high-signal entries only
	var filtered []derolog.LogEntry
	for _, entry := range entries {
		if IsHighSignal(entry) {
			filtered = append(filtered, entry)
		}
	}

	m.debugLogEntries = filtered
	m.clampDebugScrollForHeight(m.height)
}

func (m *Model) copyAddress() {
	if m.wallet != nil {
		info := m.wallet.GetInfo()
		clipboard.WriteAll(info.Address)
	}
}

func (m *Model) clearPendingRegistration() {
	m.pendingRegTxID = ""
	m.pendingRegStatus = ""
	m.pendingRegHeight = 0
	m.dashboard.SetRegistrationPending("", "")
}

// PreFlightNetworkCheck ensures we know the wallet's network before opening.
// Returns true if we can proceed, false if we need to show network selection.
func (m *Model) PreFlightNetworkCheck(file string) bool {
	// Check CLI flags first
	if m.Opts.ExplicitSimulator || m.Opts.ExplicitTestnet {
		return true
	}

	// If user explicitly selected a daemon via /connect, that network is authoritative
	if m.stickyDaemonAddress != "" {
		return true
	}

	// Check saved network
	savedNetwork := config.GetWalletNetwork(file)
	return savedNetwork != ""
}

// Commands
func (m *Model) tryOpenWallet(file, password string) tea.Cmd {
	// Check if CLI flags explicitly set network
	cliOverride := m.Opts.ExplicitTestnet || m.Opts.ExplicitSimulator

	// Check if we have saved network info for this wallet
	savedNetwork := config.GetWalletNetwork(file)

	// If no CLI override and no saved network, need to prompt user
	if !cliOverride && savedNetwork == "" {
		return func() tea.Msg {
			return networkRequiredMsg{file: file, password: password}
		}
	}

	// Use helper to get effective network (same logic as UI displays)
	testnet, simulator := m.getEffectiveNetwork(file)

	return func() tea.Msg {
		w, err := wallet.Open(file, password, testnet, simulator)
		return walletOpenedMsg{wallet: w, err: err}
	}
}

// openWalletWithNetwork opens a wallet with an explicitly selected network.
func (m *Model) openWalletWithNetwork(file, password string, network pages.NetworkSelection) tea.Cmd {
	testnet := false
	simulator := false

	switch network {
	case pages.NetworkTestnet:
		testnet = true
	case pages.NetworkSimulator:
		simulator = true
	}

	return func() tea.Msg {
		w, err := wallet.Open(file, password, testnet, simulator)
		return walletOpenedMsg{wallet: w, err: err}
	}
}

func (m *Model) createWallet(filename, password string) tea.Cmd {
	testnet, simulator := m.getCreateRestoreNetwork()

	return func() tea.Msg {
		w, seed, err := wallet.Create(filename, password, testnet, simulator)
		return walletCreatedMsg{wallet: w, seed: seed, file: filename, err: err}
	}
}

func (m *Model) restoreWallet(filename, password, seed string) tea.Cmd {
	testnet, simulator := m.getCreateRestoreNetwork()

	return func() tea.Msg {
		w, err := wallet.Restore(filename, password, seed, testnet, simulator)
		return walletRestoredMsg{wallet: w, file: filename, err: err}
	}
}

func (m *Model) restoreWalletFromKey(filename, password, hexKey string) tea.Cmd {
	testnet, simulator := m.getCreateRestoreNetwork()

	return func() tea.Msg {
		w, err := wallet.RestoreFromKey(filename, password, hexKey, testnet, simulator)
		return walletRestoredMsg{wallet: w, file: filename, err: err}
	}
}

func (m *Model) executeTransfer() tea.Cmd {
	// Capture send parameters at closure creation time, not execution time.
	// This prevents race conditions if m.send is modified before the closure runs.
	w := m.wallet
	dest := m.send.GetAddress()
	amount := m.send.GetAmount()
	paymentID := m.send.GetPaymentID()
	ringsize := m.send.GetRingsize()
	message := m.send.GetMessage()

	return func() tea.Msg {
		if w == nil {
			return transferResultMsg{err: "wallet not open"}
		}

		result := w.Transfer(wallet.TransferParams{
			Destination: dest,
			Amount:      amount,
			PaymentID:   paymentID,
			Ringsize:    ringsize,
			Message:     message,
		})

		return transferResultMsg{txID: result.TxID, err: result.Error, amountAtomic: amount, destination: dest}
	}
}

const pendingOutgoingTTL = 10 * time.Minute

func (m *Model) addPendingOutgoingTx(txID string, amountAtomic uint64, destination string) {
	if txID == "" || amountAtomic == 0 {
		return
	}
	if m.pendingOutgoing == nil {
		m.pendingOutgoing = make(map[string]pendingOutgoingTx)
	}
	if _, exists := m.pendingOutgoing[txID]; exists {
		return
	}

	now := time.Now()
	m.pendingOutgoing[txID] = pendingOutgoingTx{
		tx: pages.Transaction{
			TxID:        txID,
			Amount:      -int64(amountAtomic),
			Timestamp:   wallet.FormatTimestamp(now.Unix()),
			Incoming:    false,
			Coinbase:    false,
			Destination: destination,
			Status:      2,
		},
		addedAt: now,
	}
}

func (m *Model) mergePendingOutgoing(base []pages.Transaction) []pages.Transaction {
	if len(m.pendingOutgoing) == 0 {
		return base
	}

	now := time.Now()
	existing := make(map[string]struct{}, len(base))
	for _, tx := range base {
		if tx.TxID != "" {
			existing[tx.TxID] = struct{}{}
		}
	}

	pending := make([]pendingOutgoingTx, 0, len(m.pendingOutgoing))
	for txID, item := range m.pendingOutgoing {
		if _, found := existing[txID]; found {
			delete(m.pendingOutgoing, txID)
			continue
		}
		if now.Sub(item.addedAt) > pendingOutgoingTTL {
			delete(m.pendingOutgoing, txID)
			continue
		}
		pending = append(pending, item)
	}

	if len(pending) == 0 {
		return base
	}

	sort.Slice(pending, func(i, j int) bool {
		return pending[i].addedAt.After(pending[j].addedAt)
	})

	merged := make([]pages.Transaction, 0, len(pending)+len(base))
	for _, item := range pending {
		merged = append(merged, item.tx)
	}
	merged = append(merged, base...)
	return merged
}

func (m *Model) registerWalletCmd() tea.Cmd {
	w := m.wallet

	return func() tea.Msg {
		if w == nil {
			return registrationResultMsg{err: "wallet not open"}
		}

		info := w.GetInfo()
		if info.IsRegistered {
			return registrationResultMsg{alreadyRegistered: true}
		}

		txID, err := w.Register()
		if err != nil {
			return registrationResultMsg{err: err.Error()}
		}

		return registrationResultMsg{txID: txID}
	}
}

func (m *Model) changeWalletPassword(currentPass, newPass string) tea.Cmd {
	return func() tea.Msg {
		if m.wallet == nil {
			return passwordChangedMsg{err: fmt.Errorf("wallet not open")}
		}

		// Verify current password using the wallet's built-in check
		if !m.wallet.CheckPassword(currentPass) {
			return passwordChangedMsg{err: fmt.Errorf("current password is incorrect")}
		}

		// Change the password
		err := m.wallet.ChangePassword(newPass)
		if err != nil {
			return passwordChangedMsg{err: fmt.Errorf("failed to change password: %w", err)}
		}

		return passwordChangedMsg{err: nil}
	}
}

// connectWalletToDaemonAsync connects the wallet to daemon asynchronously.
func (m *Model) connectWalletToDaemonAsync() tea.Cmd {
	// Capture values to avoid race conditions.
	// Only use sticky daemon as an explicit override selected by user (/connect).
	// Do not force the welcome page cached daemon here, because it may be a
	// different network than the wallet being opened.
	w := m.wallet
	knownHealthy := false
	knownAddress := ""
	if m.stickyDaemonAddress != "" {
		knownHealthy = m.stickyDaemonHealthy
		knownAddress = m.stickyDaemonAddress
	} else if m.Opts.DaemonAddress != "" {
		// If we already have a daemon endpoint selected (CLI/autodetect/fallback),
		// prefer it to avoid retrying localhost first.
		knownAddress = m.Opts.DaemonAddress
		if info := wallet.GetDaemonInfo(context.Background(), knownAddress); info.IsHealthy {
			knownHealthy = true
		}
	}
	return func() tea.Msg {
		if w == nil {
			return walletDaemonConnectedMsg{connected: false, err: "Wallet not open"}
		}
		if m.Opts.Offline {
			return walletDaemonConnectedMsg{connected: false, err: "Offline mode enabled"}
		}

		// Guard wallet websocket connection with timeout to prevent indefinitely
		// stuck "connecting" state on platforms where Connect may hang.
		type connectResult struct {
			connected bool
			errMsg    string
		}
		resultCh := make(chan connectResult, 1)
		go func() {
			connected, errMsg := w.ConnectToLocalDaemonFast(knownHealthy, knownAddress)
			resultCh <- connectResult{connected: connected, errMsg: errMsg}
		}()

		var connected bool
		var errMsg string
		select {
		case res := <-resultCh:
			connected = res.connected
			errMsg = res.errMsg
		case <-time.After(12 * time.Second):
			connected = false
			errMsg = "daemon websocket connect timed out"
		}

		// Get network type and daemon address from wallet after connection
		var network config.WalletNetwork
		var daemonAddr string
		if connected {
			daemonAddr = w.GetDaemonAddress()
			switch w.GetNetworkType() {
			case "simulator":
				network = config.NetworkSimulator
			case "testnet":
				network = config.NetworkTestnet
			default:
				network = config.NetworkMainnet
			}
		}
		return walletDaemonConnectedMsg{connected: connected, network: network, daemonAddress: daemonAddr, err: errMsg}
	}
}

func (m *Model) getCreateRestoreNetwork() (testnet bool, simulator bool) {
	// Priority: CLI flags (explicit) > pending explicit UI selection > mainnet default.
	if m.Opts.ExplicitTestnet {
		return true, false
	}
	if m.Opts.ExplicitSimulator {
		return false, true
	}

	switch m.pendingNetwork {
	case pages.NetworkTestnet:
		return true, false
	case pages.NetworkSimulator:
		return false, true
	case pages.NetworkMainnet:
		return false, false
	}

	// No explicit network known - return mainnet as safe default.
	return false, false
}

// getEffectiveNetwork returns the effective network for wallet operations.
// Priority: CLI flags (explicit) > saved wallet config > current daemon > prompt user
// IMPORTANT: Never silently fall back to m.Opts.Testnet unless explicitly passed via CLI.
func (m *Model) getEffectiveNetwork(file string) (testnet, simulator bool) {
	// 1) Explicit CLI flags have highest priority
	if m.Opts.ExplicitTestnet {
		return true, false
	}
	if m.Opts.ExplicitSimulator {
		return false, true
	}

	// 2) Saved wallet network config (authoritative for this wallet)
	savedNetwork := config.GetWalletNetwork(file)
	if savedNetwork != "" {
		switch savedNetwork {
		case config.NetworkTestnet:
			return true, false
		case config.NetworkSimulator:
			return false, true
		case config.NetworkMainnet:
			return false, false
		}
	}

	// 3) Current daemon (what welcome shows) - only if healthy
	if m.cachedDaemonAddress != "" {
		if info := wallet.GetDaemonInfo(context.Background(), m.cachedDaemonAddress); info.IsHealthy {
			return info.Testnet, info.Network == "Simulator"
		}
	}

	// 4) No network known - return mainnet as safe default (user will be prompted if needed)
	// NEVER return m.Opts.Testnet here as that can carry stale state
	return false, false
}

// startXSWDCmd returns a command that starts the XSWD server in the background.
func (m *Model) startXSWDCmd() tea.Cmd {
	return func() tea.Msg {
		if m.wallet == nil || m.program == nil {
			return wallet.XSWDStartedMsg{Err: fmt.Errorf("wallet or program not ready")}
		}
		bridge := wallet.StartXSWD(m.wallet.GetDisk(), m.program)
		return wallet.XSWDStartedMsg{Bridge: bridge}
	}
}

func (m *Model) tickCmd() tea.Cmd {
	return tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}
