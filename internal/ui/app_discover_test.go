// Copyright 2017-2026 DERO Project. All rights reserved.

package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/deroproject/dero-wallet-cli/internal/wallet"
)

// TestDiscoverEscClosesDetailBeforeLeavingPage is the regression test for the
// stuck dApp-info popup: the app-level global Esc handler used to navigate
// away from PageDiscover on the FIRST Esc, leaving the detail popup open so
// re-entering Discover showed it again. Esc must now be handled by the page
// model: first Esc closes the popup, second Esc leaves the page.
func TestDiscoverEscClosesDetailBeforeLeavingPage(t *testing.T) {
	m := NewModel()
	m.discover.SetCatalog(
		[]wallet.CatalogEntry{{SCID: "aa", Class: "TELA-INDEX-1", Name: "vault.tela", DURL: "vault.tela"}},
		nil, nil, false,
	)
	m.page = PageDiscover

	// I opens the dApp info popup.
	next, _ := m.Update(tea.KeyPressMsg{Text: "i"})
	m = next.(Model)
	if !m.discover.DetailOpen() {
		t.Fatal("I should open the detail popup")
	}

	// First Esc closes the popup and stays on Discover.
	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = next.(Model)
	if m.discover.DetailOpen() {
		t.Fatal("first Esc should close the detail popup")
	}
	if m.page != PageDiscover {
		t.Fatalf("first Esc should stay on Discover, got page %d", m.page)
	}

	// Second Esc (no popup open) leaves the page.
	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = next.(Model)
	if m.page == PageDiscover {
		t.Fatal("second Esc should leave the Discover page")
	}
}
