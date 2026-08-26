// Copyright 2017-2026 DERO Project. All rights reserved.

package pages

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestPaletteOpenWithoutWallet(t *testing.T) {
	p := NewPalette()
	p.Open(false)

	if !p.IsOpen() {
		t.Fatal("palette should be open after Open()")
	}
	hasCreate := false
	hasClose := false
	for _, c := range p.commands {
		if c.Name == "/create" {
			hasCreate = true
		}
		if c.Name == "/close" {
			hasClose = true
		}
	}
	if !hasCreate {
		t.Fatal("expected /create when no wallet is open")
	}
	if hasClose {
		t.Fatal("/close should not exist when no wallet is open")
	}
	// Input should show "/"
	if p.input.Value() != "/" {
		t.Fatalf("expected input to show '/', got %q", p.input.Value())
	}
}

func TestPaletteOpenWithWallet(t *testing.T) {
	p := NewPalette()
	p.Open(true)

	hasCreate := false
	hasClose := false
	for _, c := range p.commands {
		if c.Name == "/create" {
			hasCreate = true
		}
		if c.Name == "/close" {
			hasClose = true
		}
	}
	if hasCreate {
		t.Fatal("/create should not exist when a wallet is open")
	}
	if !hasClose {
		t.Fatal("expected /close when a wallet is open")
	}
}

func TestPaletteFilterCommands(t *testing.T) {
	p := NewPalette()
	p.Open(false)

	p.Filtered = filterPaletteCommands(p.commands, "/m")
	if len(p.Filtered) != 1 || p.Filtered[0].Name != "/miner" {
		t.Fatalf("expected /miner only, got %+v", p.Filtered)
	}

	p.Filtered = filterPaletteCommands(p.commands, "/o")
	if len(p.Filtered) != 1 || p.Filtered[0].Name != "/open" {
		t.Fatalf("expected /open only, got %+v", p.Filtered)
	}
}

func TestPaletteClosePreservesAction(t *testing.T) {
	p := NewPalette()
	p.Open(false)

	// Simulate selecting /miner
	p.action = ActionMiner
	p.Close()

	// Action should be preserved for the app to dispatch
	if p.Action() != ActionMiner {
		t.Fatalf("Close() should preserve action, got %d", p.Action())
	}

	// But palette should be closed
	if p.IsOpen() {
		t.Fatal("palette should be closed")
	}
}

func TestPaletteEnterSelectsMiner(t *testing.T) {
	p := NewPalette()
	p.Open(true)

	// filtered: /open(0), /close(1), /miner(2), ...
	p.Selected = 2 // /miner

	// Simulate Enter key
	p, _ = p.Update(tea.KeyPressMsg{Text: "enter"})

	if p.Action() != ActionMiner {
		t.Fatalf("expected ActionMiner, got %d", p.Action())
	}
	if p.IsOpen() {
		t.Fatal("palette should be closed after selection")
	}
}
