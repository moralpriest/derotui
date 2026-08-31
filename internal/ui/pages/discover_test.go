// Copyright 2017-2026 DERO Project. All rights reserved.

package pages

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/deroproject/dero-wallet-cli/internal/wallet"
)

func TestDiscoverTabsAndFilter(t *testing.T) {
	m := NewDiscover()
	m.SetCatalog(
		[]wallet.CatalogEntry{{SCID: "aa", Class: "TELA-INDEX-1", Name: "vault.tela", DURL: "vault.tela"}},
		[]wallet.CatalogEntry{{SCID: "bb", Class: "G45-NFT", Name: "Cool NFT"}},
		[]wallet.CatalogEntry{{SCID: "cc", Class: "NFA", Name: "Art NFA"}},
		false,
	)
	if m.tab != 0 || len(m.rows()) != 1 || m.rows()[0].Name != "vault.tela" {
		t.Fatalf("default tab TELA: tab=%d rows=%v", m.tab, m.rows())
	}
	m, _ = m.Update(tea.KeyPressMsg{Text: "2"})
	if m.tab != 1 || m.rows()[0].Class != "G45-NFT" {
		t.Fatalf("tab 2 NFT: tab=%d %+v", m.tab, m.rows())
	}
	m, _ = m.Update(tea.KeyPressMsg{Text: "3"})
	if m.tab != 2 || m.rows()[0].Class != "NFA" {
		t.Fatalf("tab 3 NFA: tab=%d %+v", m.tab, m.rows())
	}
	m, _ = m.Update(tea.KeyPressMsg{Text: "1"})
	m.filter.SetValue("vault")
	if len(m.rows()) != 1 {
		t.Fatalf("filter vault: %v", m.rows())
	}
	m.filter.SetValue("nope")
	if len(m.rows()) != 0 {
		t.Fatalf("filter nope should be empty")
	}
}

func TestDiscoverEmptyNamePlaceholder(t *testing.T) {
	m := NewDiscover()
	m.SetCatalog(nil, []wallet.CatalogEntry{{SCID: "bb", Class: "G45-NFT"}}, nil, false)
	m, _ = m.Update(tea.KeyPressMsg{Text: "2"})
	v := m.View()
	if !containsStr(v, "—") {
		t.Fatalf("expected dash for unnamed NFT, got %q", v)
	}
	if unnamed := m.UnnamedVisible(); len(unnamed) != 1 || unnamed[0] != "bb" {
		t.Fatalf("UnnamedVisible: %v", unnamed)
	}
	m.ApplyNames(map[string]string{"bb": "Cool NFT"})
	if m.rows()[0].Name != "Cool NFT" {
		t.Fatalf("ApplyNames: %+v", m.rows())
	}
	m.SetCatalog(nil, []wallet.CatalogEntry{{SCID: "bb", Class: "G45-NFT"}}, nil, false)
	if m.nft[0].Name != "Cool NFT" {
		t.Fatalf("SetCatalog should keep hydrated name, got %q", m.nft[0].Name)
	}
}

func TestDiscoverOwnedEmptyCopy(t *testing.T) {
	m := NewDiscover()
	m.SetTela(nil, false, true)
	m, _ = m.Update(tea.KeyPressMsg{Text: "2"})
	v := m.View()
	if !containsStr(v, "Open a wallet") {
		t.Fatalf("need wallet: %q", v)
	}
	m.SetTela(nil, false, false)
	m.SetProbing(true)
	v = m.View()
	if !containsStr(v, "Checking holdings") {
		t.Fatalf("probing: %q", v)
	}
	m.SetProbing(false)
	v = m.View()
	if !containsStr(v, "None owned") {
		t.Fatalf("none owned: %q", v)
	}
}

func TestDiscoverClassifyingEmpty(t *testing.T) {
	m := NewDiscover()
	m.SetCatalog(nil, nil, nil, true)
	v := m.View()
	if !containsStr(v, "Classifying") {
		t.Fatalf("expected classifying message, got %q", v)
	}
}

func TestDiscoverSortAZ(t *testing.T) {
	m := NewDiscover()
	m.SetCatalog([]wallet.CatalogEntry{
		{SCID: "cc", Class: "TELA-INDEX-1", Name: "zeta.tela", DURL: "zeta.tela"},
		{SCID: "aa", Class: "TELA-INDEX-1", Name: "alpha.tela", DURL: "alpha.tela"},
		{SCID: "bb", Class: "TELA-INDEX-1", Name: "mid.tela", DURL: "mid.tela"},
	}, nil, nil, false)
	rows := m.rows()
	if rows[0].Name != "alpha.tela" || rows[2].Name != "zeta.tela" {
		t.Fatalf("A-Z sort default: %v", rows)
	}
	// Toggle order -> Z-A
	m, _ = m.Update(tea.KeyPressMsg{Text: "o"})
	rows = m.rows()
	if rows[0].Name != "zeta.tela" || rows[2].Name != "alpha.tela" {
		t.Fatalf("Z-A after [O]: %v", rows)
	}
	// Back to ascending
	m, _ = m.Update(tea.KeyPressMsg{Text: "o"})
	if rows := m.rows(); rows[0].Name != "alpha.tela" {
		t.Fatalf("back to A-Z: %v", rows)
	}
}

func TestDiscoverSortRecent(t *testing.T) {
	m := NewDiscover()
	m.SetCatalog([]wallet.CatalogEntry{
		{SCID: "aa", Class: "TELA-INDEX-1", Name: "old.tela", InstallHeight: 100},
		{SCID: "bb", Class: "TELA-INDEX-1", Name: "new.tela", InstallHeight: 500},
		{SCID: "cc", Class: "TELA-INDEX-1", Name: "mid.tela", InstallHeight: 300},
	}, nil, nil, false)
	// Cycle sort: A-Z -> Recent (auto-switches to descending = newest first)
	m, _ = m.Update(tea.KeyPressMsg{Text: "s"})
	if discoverSortModes[m.sort] != "Recent" {
		t.Fatalf("sort mode after [S]: %s", discoverSortModes[m.sort])
	}
	if !m.descending {
		t.Fatal("Recent should default to descending (newest first)")
	}
	rows := m.rows()
	if rows[0].Name != "new.tela" || rows[2].Name != "old.tela" {
		t.Fatalf("Recent desc: %v", rows)
	}
	// Toggle order -> oldest first
	m, _ = m.Update(tea.KeyPressMsg{Text: "o"})
	rows = m.rows()
	if rows[0].Name != "old.tela" {
		t.Fatalf("Recent asc: %v", rows)
	}
}

func TestDiscoverSortSCID(t *testing.T) {
	m := NewDiscover()
	m.SetCatalog([]wallet.CatalogEntry{
		{SCID: "cccc", Class: "TELA-INDEX-1", Name: "b.tela"},
		{SCID: "aaaa", Class: "TELA-INDEX-1", Name: "a.tela"},
	}, nil, nil, false)
	// Cycle thrice: A-Z -> Recent (desc) -> Rating (desc) -> SCID (asc)
	m, _ = m.Update(tea.KeyPressMsg{Text: "s"})
	m, _ = m.Update(tea.KeyPressMsg{Text: "s"})
	m, _ = m.Update(tea.KeyPressMsg{Text: "s"})
	if discoverSortModes[m.sort] != "SCID" {
		t.Fatalf("sort mode: %s", discoverSortModes[m.sort])
	}
	if m.descending {
		t.Fatal("SCID should default to ascending")
	}
	rows := m.rows()
	if rows[0].SCID != "aaaa" || rows[1].SCID != "cccc" {
		t.Fatalf("SCID sort: %v", rows)
	}
	// Cycle wraps back to A-Z
	m, _ = m.Update(tea.KeyPressMsg{Text: "s"})
	if discoverSortModes[m.sort] != "A-Z" {
		t.Fatalf("sort wrap: %s", discoverSortModes[m.sort])
	}
}

func TestDiscoverSortRating(t *testing.T) {
	m := NewDiscover()
	m.SetCatalog([]wallet.CatalogEntry{
		{SCID: "aa", Class: "TELA-INDEX-1", Name: "low.tela", AvgRating: 3.1},
		{SCID: "bb", Class: "TELA-INDEX-1", Name: "top.tela", AvgRating: 9.2},
		{SCID: "cc", Class: "TELA-INDEX-1", Name: "mid.tela", AvgRating: 7.5},
		{SCID: "dd", Class: "TELA-INDEX-1", Name: "unrated.tela"},
	}, nil, nil, false)
	// Cycle sort twice: A-Z -> Recent -> Rating (auto-switches to descending = best first)
	m, _ = m.Update(tea.KeyPressMsg{Text: "s"})
	m, _ = m.Update(tea.KeyPressMsg{Text: "s"})
	if discoverSortModes[m.sort] != "Rating" {
		t.Fatalf("sort mode: %s", discoverSortModes[m.sort])
	}
	if !m.descending {
		t.Fatal("Rating should default to descending (best first)")
	}
	rows := m.rows()
	if rows[0].Name != "top.tela" || rows[2].Name != "low.tela" || rows[3].Name != "unrated.tela" {
		t.Fatalf("Rating desc: %v", rows)
	}
	// Toggle order -> worst first
	m, _ = m.Update(tea.KeyPressMsg{Text: "o"})
	rows = m.rows()
	if rows[0].Name != "unrated.tela" || rows[3].Name != "top.tela" {
		t.Fatalf("Rating asc: %v", rows)
	}
}

func TestDiscStars(t *testing.T) {
	cases := []struct {
		rating float64
		want   string
	}{
		{0.0, "—"},
		{-1.0, "—"},
		{9.9, "★★★★★"},
		{9.0, "★★★★★"}, // 4.5 rounds to 5
		{8.9, "★★★★☆"},
		{7.5, "★★★★☆"},
		{5.0, "★★★☆☆"},
		{3.1, "★★☆☆☆"},
		{1.0, "★☆☆☆☆"}, // 0.5 rounds up (half-up convention)
		{0.4, "☆☆☆☆☆"},
	}
	for _, tc := range cases {
		got := discStars(wallet.CatalogEntry{AvgRating: tc.rating})
		if got != tc.want {
			t.Fatalf("discStars(%v) = %q, want %q", tc.rating, got, tc.want)
		}
	}
}

func TestDiscoverDetailPopup(t *testing.T) {
	m := NewDiscover()
	m.SetCatalog([]wallet.CatalogEntry{{
		SCID: "aa", Class: "TELA-INDEX-1", Name: "vault.tela", DURL: "vault.tela",
		Desc: "Password vault", Version: "1.0.0", Tags: []string{"vault", "tool"},
		InstallHeight: 12345,
	}}, nil, nil, false)
	if m.detail {
		t.Fatal("detail should start closed")
	}
	m, _ = m.Update(tea.KeyPressMsg{Text: "i"})
	if !m.detail {
		t.Fatal("I should open detail")
	}
	v := m.View()
	if !containsStr(v, "vault.tela") || !containsStr(v, "Password vault") || !containsStr(v, "height 12345") || !containsStr(v, "vault, tool") {
		t.Fatalf("detail popup missing fields: %q", v)
	}
	// Esc closes detail, second Esc cancels page
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.detail {
		t.Fatal("Esc should close detail")
	}
	if m.Cancelled() {
		t.Fatal("first Esc should not cancel page")
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if !m.Cancelled() {
		t.Fatal("second Esc should cancel page")
	}
}

func TestDiscoverEnterLaunchesTela(t *testing.T) {
	m := NewDiscover()
	m.SetCatalog([]wallet.CatalogEntry{
		{SCID: "aa", Class: "TELA-INDEX-1", Name: "vault.tela", DURL: "vault.tela"},
	}, nil, nil, false)
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	scid, cancel, ok := m.WantLaunch()
	if !ok || scid != "aa" || cancel == nil {
		t.Fatalf("WantLaunch: ok=%v scid=%q cancel=%v", ok, scid, cancel)
	}
	if !m.launching || m.detail {
		t.Fatal("Enter should launch, not open detail")
	}
	if !containsStr(m.View(), "Opening vault.tela") {
		t.Fatalf("flash: %q", m.View())
	}
}

func TestDiscoverEnterOnNFTOpensDetail(t *testing.T) {
	m := NewDiscover()
	m.SetCatalog(nil, []wallet.CatalogEntry{{SCID: "bb", Class: "G45-NFT", Name: "Cool NFT"}}, nil, false)
	m, _ = m.Update(tea.KeyPressMsg{Text: "2"})
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if _, _, ok := m.WantLaunch(); ok {
		t.Fatal("NFT Enter should not launch")
	}
	if !m.detail {
		t.Fatal("NFT Enter should open detail")
	}
}

func TestDiscoverEscCancelsLaunch(t *testing.T) {
	m := NewDiscover()
	m.SetCatalog([]wallet.CatalogEntry{
		{SCID: "aa", Class: "TELA-INDEX-1", Name: "vault.tela", DURL: "vault.tela"},
	}, nil, nil, false)
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	_, cancel, _ := m.WantLaunch()
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.launching || m.Cancelled() {
		t.Fatal("Esc during launch should cancel clone, not leave page")
	}
	if cancel == nil || !cancel.Load() {
		t.Fatal("cancel flag")
	}
}

func TestDiscoverFilterMatchesDesc(t *testing.T) {
	m := NewDiscover()
	m.SetCatalog([]wallet.CatalogEntry{
		{SCID: "aa", Class: "TELA-INDEX-1", Name: "app1.tela", Desc: "password manager"},
		{SCID: "bb", Class: "TELA-INDEX-1", Name: "app2.tela", Desc: "mining pool"},
	}, nil, nil, false)
	m.filter.SetValue("password")
	rows := m.rows()
	if len(rows) != 1 || rows[0].Name != "app1.tela" {
		t.Fatalf("filter by desc: %v", rows)
	}
}

func TestDiscoverFooterShowsFilter(t *testing.T) {
	m := NewDiscover()
	if !containsStr(m.View(), "F Filter") {
		t.Fatalf("footer missing F Filter: %q", m.View())
	}
}
