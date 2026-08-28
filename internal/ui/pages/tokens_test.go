// Copyright 2017-2026 DERO Project. All rights reserved.

package pages

import (
	"strings"
	"testing"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"github.com/deroproject/dero-wallet-cli/internal/wallet"
)

func TestTokenTableRowAlignment(t *testing.T) {
	ticker, name, bal, scid := tokenCells(wallet.TokenInfo{
		SCID:     "d74d1bb9968e3947a9bd40c5a9bdf598135f6b07a93bc98ded1fefa6ddd36bf5",
		Name:     "Dero Seals Token",
		Ticker:   "DST",
		Decimals: 5,
		Balance:  1000000,
	})
	if ticker != "DST" {
		t.Fatalf("ticker = %q", ticker)
	}
	if name != "Dero Seals Token" {
		t.Fatalf("name = %q", name)
	}
	if bal != "10" {
		t.Fatalf("balance = %q, want 10", bal)
	}
	if scid != "d74d1bb9...ddd36bf5" {
		t.Fatalf("scid = %q", scid)
	}
}

func TestTokensTableNavigation(t *testing.T) {
	m := NewTokens()
	m.SetTokens([]wallet.TokenInfo{
		{SCID: "1111111111111111111111111111111111111111111111111111111111111111", Balance: 1},
		{SCID: "2222222222222222222222222222222222222222222222222222222222222222", Balance: 2},
		{SCID: "3333333333333333333333333333333333333333333333333333333333333333", Balance: 3},
	})
	if m.cursor != 0 {
		t.Fatalf("expected cursor at 0, got %d", m.cursor)
	}
	m, _ = m.Update(tea.KeyPressMsg{Text: "down"})
	if m.cursor != 1 {
		t.Fatalf("expected cursor at 1 after down, got %d", m.cursor)
	}
	m, _ = m.Update(tea.KeyPressMsg{Text: "k"})
	if m.cursor != 0 {
		t.Fatalf("expected cursor at 0 after 'k', got %d", m.cursor)
	}
	m, _ = m.Update(tea.KeyPressMsg{Text: "d"})
	if scid, ok := m.WantRemove(); !ok {
		t.Fatal("expected 'd' to request removal")
	} else if scid != "1111111111111111111111111111111111111111111111111111111111111111" {
		t.Fatalf("expected removal for cursor 0 SCID, got %q", scid)
	}
}

func TestTokensHasSingleScanAction(t *testing.T) {
	m := NewTokens()
	m, _ = m.Update(tea.KeyPressMsg{Text: "n"})
	if m.WantRescan() || m.WantResetScan() {
		t.Fatal("new scan key should no longer trigger a scan action")
	}
}

func TestTokensViewRendersTable(t *testing.T) {
	m := NewTokens()
	m.SetTokens([]wallet.TokenInfo{
		{SCID: "1111111111111111111111111111111111111111111111111111111111111111", Name: "Alpha", Ticker: "ALPHA", Balance: 7},
	})
	v := m.View()
	for _, want := range []string{"Ticker", "Name", "Balance", "SCID", "ALPHA", "Alpha", "7"} {
		if !containsStr(v, want) {
			t.Fatalf("missing %q in view: %q", want, v)
		}
	}
	if containsStr(v, "[N]ew scan") {
		t.Fatalf("unexpected duplicate scan mode in view: %q", v)
	}
}

func TestTokensViewShowsTableWhileScanning(t *testing.T) {
	m := NewTokens()
	m.SetTokens([]wallet.TokenInfo{
		{SCID: "1111111111111111111111111111111111111111111111111111111111111111", Ticker: "ALPHA", Name: "Alpha", Balance: 7},
	})
	m.SetScanning(true, "Checking 3/10 — 1 token found")
	v := m.View()
	if !containsStr(v, "ALPHA") || !containsStr(v, "Alpha") {
		t.Fatalf("scanning should not hide ticker/name: %q", v)
	}
}

func TestTokensTitleCentered(t *testing.T) {
	m := NewTokens()
	m.SetTokens([]wallet.TokenInfo{
		{SCID: "1111111111111111111111111111111111111111111111111111111111111111", Ticker: "ALPHA", Name: "Alpha", Balance: 7},
	})
	plain := stripANSI(m.View())
	var titleLine, headerLine string
	for _, line := range strings.Split(plain, "\n") {
		if titleLine == "" && strings.Contains(line, "Tokens") && !strings.Contains(line, "No tokens") {
			titleLine = strings.TrimRightFunc(line, unicode.IsSpace)
		}
		if strings.Contains(line, "Ticker") && strings.Contains(line, "Name") {
			headerLine = strings.TrimRightFunc(line, unicode.IsSpace)
		}
	}
	if titleLine == "" || headerLine == "" {
		t.Fatalf("missing title or header:\n%s", plain)
	}
	titleAt := strings.Index(titleLine, "Tokens")
	if titleAt < 8 {
		t.Fatalf("title not centered, index %d in %q", titleAt, titleLine)
	}
	headerAt := strings.Index(headerLine, "Ticker")
	if headerAt > titleAt {
		t.Fatalf("Ticker should sit left of centered title, header=%d title=%d\n%s", headerAt, titleAt, plain)
	}
}

func containsStr(s, sub string) bool {
	return strings.Contains(s, sub)
}
