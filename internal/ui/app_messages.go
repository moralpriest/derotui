// Copyright 2017-2026 DERO Project. All rights reserved.

package ui

import (
	"time"

	"github.com/deroproject/dero-wallet-cli/internal/config"
	daemonservice "github.com/deroproject/dero-wallet-cli/internal/services/daemon"
	"github.com/deroproject/dero-wallet-cli/internal/services/installer"
	minerservice "github.com/deroproject/dero-wallet-cli/internal/services/miner"
	"github.com/deroproject/dero-wallet-cli/internal/wallet"
)

const registrationConfirmTimeoutBlocks uint64 = 20

// tickMsg is sent periodically for updates.
type tickMsg time.Time

type daemonStatusEntry struct {
	isOnline        bool
	isSynced        bool
	isSyncing       bool
	isBootstrapping bool
	isHealthy       bool
	network         string
	address         string
	height          uint64
	peerHeight      int64
	syncProgress    float64
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

type daemonManagerMsg struct {
	snapshot daemonservice.Snapshot
	logs     []string
	info     wallet.DaemonInfo
	err      string
	source   string
}

type daemonInstallPreviewMsg struct {
	plan installer.Plan
	err  string
}

type daemonInstallApplyMsg struct {
	err         string
	userService bool
}

type daemonInstallApplySudoMsg struct {
	err string
}

// daemonUninstallMsg carries the result of a derod service uninstall.
type daemonUninstallMsg struct {
	err     string
	removed string // comma-separated list of removed components
}

// minerControlMsg carries the result of a start/stop miner command. The RPC
// backend is passed through the message (not assigned on the model inside the
// command closure) because Model.Update is a value receiver: a closure mutating
// m.rpcMiner would write to a discarded copy and the live model would never
// see the running miner.
type minerControlMsg struct {
	err   string
	miner minerservice.RPCBackend // non-nil when an RPC/engine miner was started or stopped
}

type minerStatsMsg struct {
	running    bool
	hashrate   uint64
	blocks     uint64
	minis      uint64
	rejected   uint64
	height     uint64
	difficulty uint64
	hashes     uint64
	uptime     time.Duration
	threads    int
	address    string
	status     string
	daemonHost string
}

// walletDataMsg is sent when a background transaction-refresh completes.
// The heavy GetTransactions call (Show_Transfers) runs off the UI thread.
type walletDataMsg struct {
	txs []wallet.TransactionInfo
	err string
}

// regPollMsg carries the result of a background registration-status poll.
type regPollMsg struct {
	txID     string
	status   string
	found    bool
	rejected bool
	err      string
}

// namesLoadedMsg carries the result of listing the wallet's registered names.
type namesLoadedMsg struct {
	names []wallet.NameEntry
	err   string
}

// nameRegisterResultMsg carries the result of a name registration.
type nameRegisterResultMsg struct {
	txID string
	err  string
}

// nameTransferResultMsg carries the result of a name transfer.
type nameTransferResultMsg struct {
	txID string
	err  string
}

// xswdDialogTimeoutMsg fires when an XSWD auth/permission dialog has been
// shown too long; the TUI dismisses it (denying the request) so it doesn't
// hang forever.
type xswdDialogTimeoutMsg struct{}

// tokensLoadedMsg carries tokens loaded wallet-locally (balances + tracked
// SCIDs). Metadata may still be missing; the scan hydrates it incrementally.
type tokensLoadedMsg struct {
	tokens []wallet.TokenInfo
	err    string
}

// tokenScanProgressMsg reports one step of the incremental token scan.
// candidates carries the full candidate list on every step; index is the
// number of candidates already checked; found holds tokens discovered in
// this step (balance > 0, metadata hydrated). A msg with candidates != nil
// and index == 0 starts a scan; index >= len(candidates) finishes it.
type tokenScanProgressMsg struct {
	id         int
	candidates []string
	index      int
	found      []wallet.TokenInfo
	zero       []string
	err        string
}

// tokenBalanceRefreshMsg reports re-checked balances for zero-balance
// candidates registered by an earlier scan. A freshly registered SCID only
// yields its balance after the wallet's next sync round, so pending
// candidates are re-checked periodically until they resolve or are removed.
type tokenBalanceRefreshMsg struct {
	found   []wallet.TokenInfo
	pending []string
}

type tokenMetadataMsg struct {
	tokens []wallet.TokenInfo
}

// tokenAddResultMsg carries result of adding a token.
type tokenAddResultMsg struct {
	scid string
	err  string
}

// tokenSendResultMsg carries result of token transfer.
type tokenSendResultMsg struct {
	txID   string
	err    string
	scid   string
	amount uint64
}

// tokenHistoryLoadedMsg carries token transaction history.
type tokenHistoryLoadedMsg struct {
	scid string
	txs  []wallet.TransactionInfo
	err  string
}
