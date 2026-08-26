// Copyright 2017-2026 DERO Project. All rights reserved.

package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/deroproject/dero-wallet-cli/internal/ui/pages"
	"github.com/deroproject/dero-wallet-cli/internal/wallet"
)

// TestPaletteMinerOpensFromDashboard verifies the global "/" palette can
// select /miner and transition to PageMiner with the correct return page.
func TestPaletteMinerOpensFromDashboard(t *testing.T) {
	m := NewModel()
	m.page = PageMain

	// Open palette with wallet open
	m.palette.Open(true)

	// Use exported fields to select /miner (index 2 in filtered list: /open, /close, /miner)
	m.palette.Selected = 2

	// Send Enter
	result, _ := m.Update(tea.KeyPressMsg{Text: "enter"})
	got := result.(Model)

	// Should transition to PageMiner
	if got.page != PageMiner {
		t.Fatalf("expected PageMiner, got page %v", got.page)
	}
	// Return page should be PageMain (the dashboard)
	if got.minerReturnPage != PageMain {
		t.Fatalf("expected minerReturnPage=PageMain, got %v", got.minerReturnPage)
	}
	// Palette should be closed
	if got.palette.IsOpen() {
		t.Fatal("palette should be closed after selection")
	}
}

// TestWantDonateNilWallet verifies the dashboard Donate action does not panic
// when no wallet is open (the old code dereferenced m.wallet outside the
// nil-guard).
func TestWantDonateNilWallet(t *testing.T) {
	m := NewModel()
	m.page = PageMain
	m.wallet = nil

	// "d" triggers dashboardWantDonate via the dashboard update loop.
	result, _ := m.Update(tea.KeyPressMsg{Text: "d"})
	got := result.(Model)

	if got.page != PageMain {
		t.Fatalf("expected to stay on PageMain with no wallet open, got page %v", got.page)
	}
	if got.wallet != nil {
		t.Fatal("wallet should still be nil")
	}
	if !strings.Contains(got.dashboard.View(), "No wallet open") {
		t.Fatalf("expected flash explaining wallet must be open, got view: %q", got.dashboard.View())
	}
}

// TestXSWDTimeoutDismissesDialog verifies an auth dialog is auto-dismissed
// (denied) after the server-side timeout fires.
func TestXSWDTimeoutDismissesDialog(t *testing.T) {
	m := NewModel()
	m.page = PageXSWDAuth
	m.xswdPrevPage = PageMain
	resp := make(chan bool, 1)
	m.xswdAuthCh = resp
	m.xswdAuth = pages.NewXSWDAuth("Test App", "test desc", "https://example.com", "app-1")

	result, _ := m.Update(xswdDialogTimeoutMsg{})
	got := result.(Model)

	if got.page != PageMain {
		t.Fatalf("expected dialog to be dismissed back to PageMain, got %v", got.page)
	}
	if got.xswdAuthCh != nil {
		t.Fatal("expected auth response channel to be cleared after timeout")
	}
	select {
	case result := <-resp:
		if result {
			t.Fatal("expected timed-out auth request to be denied")
		}
	default:
		t.Fatal("expected the auth response channel to receive a denial")
	}
}

// TestXSWDPermTimeoutDismissesDialog verifies a permission dialog is also
// auto-dismissed (denied) on timeout.
func TestXSWDPermTimeoutDismissesDialog(t *testing.T) {
	m := NewModel()
	m.page = PageXSWDPerm
	m.xswdPrevPage = PageMain
	resp := make(chan int, 1)
	m.xswdPermCh = resp
	m.xswdPerm = pages.NewXSWDPerm("Test App", "getbalance")

	result, _ := m.Update(xswdDialogTimeoutMsg{})
	got := result.(Model)

	if got.page != PageMain {
		t.Fatalf("expected dialog to be dismissed back to PageMain, got %v", got.page)
	}
	if got.xswdPermCh != nil {
		t.Fatal("expected perm response channel to be cleared after timeout")
	}
	select {
	case result := <-resp:
		if result != wallet.XSWDPermDeny {
			t.Fatalf("expected timed-out perm request to be denied, got %d", result)
		}
	default:
		t.Fatal("expected the perm response channel to receive a denial")
	}
}
