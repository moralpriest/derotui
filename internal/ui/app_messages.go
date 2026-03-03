// Copyright 2017-2026 DERO Project. All rights reserved.

package ui

import (
	"time"

	"github.com/deroproject/dero-wallet-cli/internal/config"
	"github.com/deroproject/dero-wallet-cli/internal/wallet"
)

const registrationConfirmTimeoutBlocks uint64 = 20

// tickMsg is sent periodically for updates.
type tickMsg time.Time

type daemonStatusEntry struct {
	isOnline   bool
	isSynced   bool
	isHealthy  bool
	network    string
	address    string
	height     uint64
	topoHeight int64
}

// daemonStatusMsg carries daemon status info.
type daemonStatusMsg struct {
	daemons []daemonStatusEntry
}

// startupCheckMsg carries the last wallet path if found.
type startupCheckMsg struct {
	lastWallet string
}

type debugToggleResultMsg struct {
	enabled bool
	logPath string
	open    bool
	err     error
}

// daemonConnectMsg carries the result of a daemon connection attempt.
type daemonConnectMsg struct {
	address string
	err     error
	testnet bool   // daemon's network mode (from RPC response)
	network string // "Simulator" if simulator mode, empty otherwise
}

type walletOpenedMsg struct {
	wallet *wallet.Wallet
	err    error
}

type walletCreatedMsg struct {
	wallet *wallet.Wallet
	seed   string
	file   string
	err    error
}

type walletRestoredMsg struct {
	wallet *wallet.Wallet
	file   string
	err    error
}

type transferResultMsg struct {
	txID         string
	err          string
	amountAtomic uint64
	destination  string
}

type registrationResultMsg struct {
	txID              string
	err               string
	alreadyRegistered bool
}

type passwordChangedMsg struct {
	err error
}

// walletDaemonConnectedMsg is sent when async daemon connection completes.
type walletDaemonConnectedMsg struct {
	connected     bool
	network       config.WalletNetwork // The network type we connected to
	daemonAddress string               // The daemon address we connected to
	err           string               // Error message if connection failed
}

// networkRequiredMsg is sent when wallet needs network selection.
type networkRequiredMsg struct {
	file     string
	password string
}
