// Copyright 2017-2026 DERO Project. All rights reserved.

package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/deroproject/dero-wallet-cli/internal/ui/pages"
	"github.com/deroproject/dero-wallet-cli/internal/wallet"
)

// TestNamesPageOpensRegister verifies pressing [R] on the names page opens the
// registration form.
func TestNamesPageOpensRegister(t *testing.T) {
	m := NewModel()
	m.page = PageNames
	m.names = pages.NewNames()
	m.names.SetNames([]wallet.NameEntry{{Name: "alice"}})

	result, _ := m.Update(tea.KeyPressMsg{Text: "r"})
	got := result.(Model)

	if got.page != PageNameRegister {
		t.Fatalf("expected PageNameRegister, got page %v", got.page)
	}
}

// TestNamesPageTransferSelected verifies pressing [T] pre-fills the transfer
// form with the selected name.
func TestNamesPageTransferSelected(t *testing.T) {
	m := NewModel()
	m.page = PageNames
	m.names = pages.NewNames()
	m.names.SetNames([]wallet.NameEntry{{Name: "alice"}, {Name: "bob"}})

	result, _ := m.Update(tea.KeyPressMsg{Text: "t"})
	got := result.(Model)

	if got.page != PageNameTransfer {
		t.Fatalf("expected PageNameTransfer, got page %v", got.page)
	}
	if got.nameTransfer.GetName() != "alice" {
		t.Fatalf("expected transfer of selected name alice, got %q", got.nameTransfer.GetName())
	}
}

// TestNamesPageTransferAll verifies pressing [A] opens the transfer form in
// transfer-all mode carrying every owned name.
func TestNamesPageTransferAll(t *testing.T) {
	m := NewModel()
	m.page = PageNames
	m.names = pages.NewNames()
	m.names.SetNames([]wallet.NameEntry{{Name: "alice"}, {Name: "bob"}})

	result, _ := m.Update(tea.KeyPressMsg{Text: "a"})
	got := result.(Model)

	if got.page != PageNameTransfer {
		t.Fatalf("expected PageNameTransfer, got page %v", got.page)
	}
	if !got.nameTransfer.IsTransferAll() {
		t.Fatal("expected transfer-all mode")
	}
	if got.nameTransfer.GetAllNames() == nil || len(got.nameTransfer.GetAllNames()) != 2 {
		t.Fatalf("expected 2 names for transfer-all, got %v", got.nameTransfer.GetAllNames())
	}
}

// TestNamesPageEscReturnsToDashboard verifies Esc closes the names page back
// to the dashboard.
func TestNamesPageEscReturnsToDashboard(t *testing.T) {
	m := NewModel()
	m.page = PageNames
	m.names = pages.NewNames()

	result, _ := m.Update(tea.KeyPressMsg{Text: "esc"})
	got := result.(Model)

	if got.page != PageMain {
		t.Fatalf("expected PageMain after Esc, got page %v", got.page)
	}
}

// TestNamesRegisterEscReturnsToList verifies Esc on the register form returns
// to the names list.
func TestNamesRegisterEscReturnsToList(t *testing.T) {
	m := NewModel()
	m.page = PageNameRegister
	m.nameRegister = pages.NewNameRegister()

	result, _ := m.Update(tea.KeyPressMsg{Text: "esc"})
	got := result.(Model)

	if got.page != PageNames {
		t.Fatalf("expected PageNames after Esc, got page %v", got.page)
	}
}
