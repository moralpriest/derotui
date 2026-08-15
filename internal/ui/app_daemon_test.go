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
