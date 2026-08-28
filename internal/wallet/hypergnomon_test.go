// Copyright 2017-2026 DERO Project. All rights reserved.

package wallet

import (
	"strings"
	"testing"

	hgstorage "github.com/hypergnomon/hypergnomon/pkg/gnomes/storage"
	hgnative "github.com/hypergnomon/hypergnomon/storage"
)

// TestHyperGnomonSCIDsForAddress verifies that SCIDsForAddress reads the
// addr_scids reverse index built by HyperGnomon: only the SCIDs the queried
// address actually interacted with, not every SCID in the index.
func TestHyperGnomonSCIDsForAddress(t *testing.T) {
	store, err := hgstorage.NewBBoltDB(t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	batch := hgnative.NewWriteBatch()
	batch.AddAddrSCID("dero1qwallet", strings.Repeat("a", 64), 100)
	batch.AddAddrSCID("dero1qwallet", strings.Repeat("b", 64), 200)
	batch.AddAddrSCID("dero1qother", strings.Repeat("c", 64), 300)
	batch.LastHeight = 300
	if err := store.Inner().FlushBatch(batch); err != nil {
		t.Fatal(err)
	}

	h := &HyperGnomon{store: store}

	got := h.SCIDsForAddress("dero1qwallet")
	if len(got) != 2 {
		t.Fatalf("wallet address: got %d SCIDs, want 2 (%v)", len(got), got)
	}
	seen := map[string]bool{}
	for _, scid := range got {
		seen[scid] = true
	}
	if !seen[strings.Repeat("a", 64)] || !seen[strings.Repeat("b", 64)] {
		t.Fatalf("wallet address: got %v, want a.. and b.. SCIDs", got)
	}

	// A different address must only see its own interactions.
	other := h.SCIDsForAddress("dero1qother")
	if len(other) != 1 || other[0] != strings.Repeat("c", 64) {
		t.Fatalf("other address: got %v, want only c.. SCID", other)
	}

	// An address with no interactions gets nil, not an error.
	if got := h.SCIDsForAddress("dero1qstranger"); got != nil {
		t.Fatalf("stranger address: got %v, want nil", got)
	}

	// Whitespace is tolerated.
	if got := h.SCIDsForAddress("  dero1qother  "); len(got) != 1 {
		t.Fatalf("padded address: got %v, want 1 SCID", got)
	}

	// Nil receiver and closed-store safety.
	var nilH *HyperGnomon
	if got := nilH.SCIDsForAddress("dero1qwallet"); got != nil {
		t.Fatalf("nil receiver: got %v, want nil", got)
	}
	_ = store.Close()
	if got := h.SCIDsForAddress("dero1qwallet"); got != nil {
		t.Fatalf("closed store: got %v, want nil", got)
	}
}
