// Copyright 2017-2026 DERO Project. All rights reserved.

package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/deroproject/dero-wallet-cli/internal/config"
	daemonservice "github.com/deroproject/dero-wallet-cli/internal/services/daemon"
	"github.com/deroproject/dero-wallet-cli/internal/ui/pages"
	"github.com/deroproject/dero-wallet-cli/internal/wallet"
)

func TestPreferredDaemonAddressStickyWins(t *testing.T) {
	m := NewModel()
	m.stickyDaemonAddress = "dero.geek.cloud:10102"
	m.Opts.DaemonAddress = "other.example:9999"
	m.lastWalletDaemon = "last.example:8888"

	if got := m.preferredDaemonAddress(); got != "dero.geek.cloud:10102" {
		t.Fatalf("expected sticky daemon, got %q", got)
	}
}

func TestPreferredDaemonAddressOptsFallback(t *testing.T) {
	m := NewModel()
	m.Opts.DaemonAddress = "cli.example:7777"
	m.lastWalletDaemon = "last.example:8888"

	if got := m.preferredDaemonAddress(); got != "cli.example:7777" {
		t.Fatalf("expected CLI daemon, got %q", got)
	}
}

func TestPreferredDaemonAddressLastWalletFallback(t *testing.T) {
	m := NewModel()
	m.lastWalletDaemon = "last.example:8888"

	if got := m.preferredDaemonAddress(); got != "last.example:8888" {
		t.Fatalf("expected last wallet daemon, got %q", got)
	}
}

func TestPreferredDaemonAddressEmptyWhenNone(t *testing.T) {
	m := NewModel()

	if got := m.preferredDaemonAddress(); got != "" {
		t.Fatalf("expected empty preferred daemon, got %q", got)
	}
}

func TestPreferredDaemonAddressIgnoresNotConnectedSentinel(t *testing.T) {
	m := NewModel()
	m.Opts.DaemonAddress = "Not connected"
	m.lastWalletDaemon = "Not connected"

	if got := m.preferredDaemonAddress(); got != "" {
		t.Fatalf("expected empty preferred daemon when only sentinel values present, got %q", got)
	}
}

func TestRPCToGetwork(t *testing.T) {
	m := NewModel()

	tests := []struct {
		input    string
		expected string
	}{
		{"node.dero.live:10102", "node.dero.live:10100"},
		{"127.0.0.1:40402", "127.0.0.1:40400"},
		{"localhost:20000", "localhost:20000"},
		{"example.com:10102", "example.com:10100"},
		{"custom:12345", "custom:12343"}, // port-2 fallback
	}

	for _, tc := range tests {
		got := m.rpcToGetwork(tc.input)
		if got != tc.expected {
			t.Fatalf("rpcToGetwork(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestGetworkHostPrefersStickyDaemon(t *testing.T) {
	m := NewModel()
	m.stickyDaemonAddress = "sticky.example:10102"
	m.Opts.DaemonAddress = "cli.example:10102"

	got := m.getworkHost()
	if got != "sticky.example:10100" {
		t.Fatalf("expected sticky daemon getwork host, got %q", got)
	}
}

func TestGetworkHostFallsBackToOpts(t *testing.T) {
	m := NewModel()
	m.Opts.DaemonAddress = "cli.example:40402"

	got := m.getworkHost()
	if got != "cli.example:40400" {
		t.Fatalf("expected CLI daemon getwork host, got %q", got)
	}
}

func TestGetworkHostEmptyWhenNoDaemon(t *testing.T) {
	m := NewModel()

	got := m.getworkHost()
	// With no daemon configured, falls back to default daemon settings (mainnet)
	if got != "127.0.0.1:10100" {
		t.Fatalf("expected default mainnet getwork host, got %q", got)
	}
}

func TestMinerDaemonNetworkPrefersEmbedded(t *testing.T) {
	_ = NewModel()
	m := NewModel()

	// Without embedded daemon, uses daemon settings default (mainnet)
	got := m.minerDaemonNetwork()
	if got != "mainnet" {
		t.Fatalf("expected mainnet from defaults, got %q", got)
	}
}

func TestValidateMiningAddressForDaemonMainnetOnTestnet(t *testing.T) {
	_ = NewModel()

	// Mainnet address (dero1...) on testnet should fail
	err := validateMiningAddressForDaemon("dero1abc123", "testnet")
	if err == nil {
		t.Fatal("expected error for mainnet address on testnet")
	}
}

func TestValidateMiningAddressForDaemonTestnetOnMainnet(t *testing.T) {
	_ = NewModel()

	// Testnet address (deto1...) on mainnet should fail
	err := validateMiningAddressForDaemon("deto1abc123", "mainnet")
	if err == nil {
		t.Fatal("expected error for testnet address on mainnet")
	}
}

func TestValidateMiningAddressForDaemonMatch(t *testing.T) {
	_ = NewModel()

	// Mainnet address on mainnet should pass
	err := validateMiningAddressForDaemon("dero1abc123", "mainnet")
	if err != nil {
		t.Fatalf("expected no error for matching mainnet address, got %v", err)
	}

	// Testnet address on testnet should pass
	err = validateMiningAddressForDaemon("deto1abc123", "testnet")
	if err != nil {
		t.Fatalf("expected no error for matching testnet address, got %v", err)
	}
}

// A daemon detected on a local port that the app did NOT start must stay
// labeled "External Local" and must not be marked as managed.
func TestApplyDaemonManagerMsgExternalStaysExternal(t *testing.T) {
	m := NewModel()
	msg := daemonManagerMsg{
		source: "External Local",
		snapshot: daemonservice.Snapshot{
			Running: true,
			Managed: false,
			PID:     4242,
			RPCBind: "127.0.0.1:10102",
		},
		info: wallet.DaemonInfo{IsOnline: true, IsHealthy: true, IsSynced: true, Network: "mainnet", Height: 100},
	}
	m.applyDaemonManagerMsg(msg)

	snap := m.daemonStatus.Snapshot
	if snap.Source != "External Local" {
		t.Fatalf("expected source External Local, got %q", snap.Source)
	}
	if snap.Managed {
		t.Fatal("external daemon must not be marked managed")
	}
	if !snap.Running {
		t.Fatal("external daemon should be marked running")
	}
	if snap.PID != 4242 {
		t.Fatalf("expected PID 4242 to be preserved, got %d", snap.PID)
	}
}

// A daemon started by the app's manager stays "Managed Local" even when the
// detector reported a generic source (e.g. right after Start).
func TestApplyDaemonManagerMsgManagedStaysManaged(t *testing.T) {
	m := NewModel()
	msg := daemonManagerMsg{
		source: "",
		snapshot: daemonservice.Snapshot{
			Running: true,
			Managed: true,
			PID:     123,
			RPCBind: "127.0.0.1:10102",
		},
		info: wallet.DaemonInfo{IsOnline: true, IsHealthy: true, IsSynced: true},
	}
	m.applyDaemonManagerMsg(msg)

	snap := m.daemonStatus.Snapshot
	if snap.Source != "Managed Local" {
		t.Fatalf("expected source Managed Local, got %q", snap.Source)
	}
	if !snap.Managed {
		t.Fatal("managed daemon should stay marked managed")
	}
	if !snap.Running {
		t.Fatal("managed daemon should be marked running")
	}
}

// A systemd-managed derod keeps its own label and is not claimed as managed.
func TestApplyDaemonManagerMsgSystemDStaysSystemDaemon(t *testing.T) {
	m := NewModel()
	msg := daemonManagerMsg{
		source: "System Daemon",
		snapshot: daemonservice.Snapshot{
			Running: true,
			Managed: false,
		},
		info: wallet.DaemonInfo{IsOnline: true, IsHealthy: true, IsSynced: true},
	}
	m.applyDaemonManagerMsg(msg)

	snap := m.daemonStatus.Snapshot
	if snap.Source != "System Daemon" {
		t.Fatalf("expected source System Daemon, got %q", snap.Source)
	}
	if snap.Managed {
		t.Fatal("system daemon must not be marked managed")
	}
	if !snap.Running {
		t.Fatal("system daemon should be marked running")
	}
}

// Opening settings for an external daemon must show the running node's real
// config (from its command line), not the app's saved template.
func TestDaemonSettingsForSnapshotExternalUsesRealConfig(t *testing.T) {
	base := config.DaemonSettings{
		Mode:       "embedded",
		DataDir:    "/home/user/.derotui",
		RPCBind:    "127.0.0.1:10102",
		BinaryPath: "/home/user/.derotui/derod",
	}
	snap := pages.DaemonStatusSnapshot{
		Source:  "External Local",
		Network: "mainnet",
		RPCBind: "0.0.0.0:10102",
		LaunchArgs: []string{
			"/opt/dero/derod",
			"--data-dir=/opt/dero/data",
			"--rpc-bind=0.0.0.0:10102",
			"--getwork-bind=0.0.0.0:10100",
			"--node-tag=my-node",
		},
	}

	got := daemonSettingsForSnapshot(base, snap)

	if got.Mode != "external" {
		t.Errorf("Mode = %q, want external", got.Mode)
	}
	if got.BinaryPath != "/opt/dero/derod" {
		t.Errorf("BinaryPath = %q, want /opt/dero/derod", got.BinaryPath)
	}
	if got.DataDir != "/opt/dero/data" {
		t.Errorf("DataDir = %q, want /opt/dero/data", got.DataDir)
	}
	if got.RPCBind != "0.0.0.0:10102" {
		t.Errorf("RPCBind = %q", got.RPCBind)
	}
	if got.GetWorkBind != "0.0.0.0:10100" {
		t.Errorf("GetWorkBind = %q", got.GetWorkBind)
	}
	if got.NodeTag != "my-node" {
		t.Errorf("NodeTag = %q", got.NodeTag)
	}
}

// Settings for a managed daemon must not be touched by the snapshot.
func TestDaemonSettingsForSnapshotManagedUnchanged(t *testing.T) {
	base := config.DaemonSettings{Mode: "embedded", DataDir: "/home/user/.derotui"}
	snap := pages.DaemonStatusSnapshot{
		Source:     "Managed Local",
		DataDir:    "/somewhere/else",
		LaunchArgs: []string{"/opt/dero/derod", "--data-dir=/opt/dero/data"},
	}

	got := daemonSettingsForSnapshot(base, snap)
	if got.DataDir != "/home/user/.derotui" {
		t.Errorf("DataDir should stay at base for managed daemon, got %q", got.DataDir)
	}
}

func TestTailFileLinesReturnsLastNonEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "derod.log")
	content := ""
	for i := 1; i <= 205; i++ {
		content += "line " + string(rune('0'+i%10)) + "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	lines := tailFileLines(path, maxExternalLogLines)
	if len(lines) != maxExternalLogLines {
		t.Fatalf("expected %d lines, got %d", maxExternalLogLines, len(lines))
	}
	if lines[0] == "" || lines[len(lines)-1] == "" {
		t.Fatal("empty lines should be trimmed")
	}
	if got := tailFileLines(filepath.Join(dir, "missing.log"), maxExternalLogLines); got != nil {
		t.Fatalf("expected nil for missing file, got %v", got)
	}
}

// TestWalletDaemonRetryDoesNotForceDashboard verifies a failed background
// daemon reconnection (the periodic offline auto-retry) does not yank the user
// back to the dashboard. This was the "backs out by itself" bug: the retry
// loop fires every few seconds while offline and used to set m.page=PageMain.
func TestWalletDaemonRetryDoesNotForceDashboard(t *testing.T) {
	m := NewModel()
	// Wallet open but offline, user navigated into a sub-page.
	m.page = PageSend
	m.daemonRetryAfter = initialDaemonRetryInterval

	result, _ := m.Update(walletDaemonConnectedMsg{connected: false, err: "connection refused"})
	got := result.(Model)

	if got.page != PageSend {
		t.Fatalf("failed daemon retry navigated away from PageSend: got page %v", got.page)
	}
	if got.dashboard.IsConnecting {
		t.Fatal("connecting indicator should be cleared after a failed retry")
	}
}

func TestApplyPruneConversionDeferredWhenTopoUnknown(t *testing.T) {
	m := Model{}
	s := config.DaemonSettings{PruneHistory: "50000"}
	if !m.applyPruneConversion(&s) {
		t.Fatal("expected defer when topo is 0")
	}
	if s.PruneHistory != "50000" {
		t.Fatalf("keep-last must stay unchanged when deferred, got %q", s.PruneHistory)
	}
}

func TestApplyPruneConversionRewritesAbsoluteCut(t *testing.T) {
	m := Model{}
	m.daemonStatus.Snapshot.TopoHeight = 600000
	s := config.DaemonSettings{PruneHistory: "50000"}
	if m.applyPruneConversion(&s) {
		t.Fatal("expected conversion, not defer")
	}
	if s.PruneHistory != "550000" {
		t.Fatalf("cut = topo-keep = 550000, got %q", s.PruneHistory)
	}
}

func TestApplyPruneConversionFullProfileNoOp(t *testing.T) {
	m := Model{}
	s := config.DaemonSettings{PruneHistory: ""}
	if m.applyPruneConversion(&s) {
		t.Fatal("full profile is not deferred")
	}
	if s.PruneHistory != "" {
		t.Fatalf("full profile must stay empty, got %q", s.PruneHistory)
	}
}
