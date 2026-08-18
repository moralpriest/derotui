// Copyright 2017-2026 DERO Project. All rights reserved.

package ui

import "testing"

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
