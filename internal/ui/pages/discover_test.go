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
