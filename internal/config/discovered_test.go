// Copyright 2017-2026 DERO Project. All rights reserved.

package config

import (
	"path/filepath"
	"testing"
)

func TestMergeAndClearDiscoveredSCIDs(t *testing.T) {
	// Use a scratch config file so the test never touches ~/.derotui.json.
	cfgPath := filepath.Join(t.TempDir(), "config.json")
	prev := configPathFn
	configPathFn = func() string { return cfgPath }
	defer func() { configPathFn = prev }()

	wallet := filepath.Join(t.TempDir(), "wallet.db")

	if err := MergeDiscoveredSCIDs(wallet, []DiscoveredSCID{
		{SCID: "aaaa", Source: "wallet", LastCheckedHeight: 10},
		{SCID: "bbbb", Source: "wallet", LastCheckedHeight: 11},
	}); err != nil {
		t.Fatal(err)
	}
	if got := len(GetDiscoveredSCIDs(wallet)); got != 2 {
		t.Fatalf("after merge: got %d discovered SCIDs, want 2", got)
	}

	// Clear resets the wallet's cache back to empty.
	ClearDiscoveredSCIDs(wallet)
	if got := len(GetDiscoveredSCIDs(wallet)); got != 0 {
		t.Fatalf("after clear: got %d discovered SCIDs, want 0", got)
	}

	// Clearing one wallet must not affect another.
	if err := MergeDiscoveredSCIDs(wallet, []DiscoveredSCID{{SCID: "cccc", Source: "wallet"}}); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(t.TempDir(), "other.db")
	if err := MergeDiscoveredSCIDs(other, []DiscoveredSCID{{SCID: "dddd", Source: "wallet"}}); err != nil {
		t.Fatal(err)
	}
	ClearDiscoveredSCIDs(wallet)
	if got := len(GetDiscoveredSCIDs(other)); got != 1 {
		t.Fatalf("other wallet cache was cleared too: got %d, want 1", got)
	}
}
