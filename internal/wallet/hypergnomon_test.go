// Copyright 2017-2026 DERO Project. All rights reserved.

package wallet

import (
	"strings"
	"testing"
	"time"

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

func TestNameFromCatalogVals(t *testing.T) {
	if got := nameFromCatalogVals(map[string]string{"name": "Art NFA"}); got != "Art NFA" {
		t.Fatalf("name key: %q", got)
	}
	if got := nameFromCatalogVals(map[string]string{"metadata": `{"name":"Cool NFT","description":"x"}`}); got != "Cool NFT" {
		t.Fatalf("g45 metadata: %q", got)
	}
	if got := nameFromCatalogVals(map[string]string{"var_header_name": "vault.tela", "name": "ignored"}); got != "vault.tela" {
		t.Fatalf("header wins: %q", got)
	}
	if got := LookupSCName("", ""); got != "" {
		t.Fatalf("empty lookup: %q", got)
	}
}

// TestHyperGnomonProgressCached verifies Progress() is served from cached
// atomics refreshed by the background poller — not from a synchronous bbolt
// owners-bucket scan. This is the startup-lag fix: the UI thread must never
// block on a full owners-bucket scan inside Update, and Progress must stay
// cheap (atomic reads only) even against a 50k-SCID owners bucket.
func TestHyperGnomonProgressCached(t *testing.T) {
	store, err := hgstorage.NewBBoltDB(t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}

	batch := hgnative.NewWriteBatch()
	batch.AddAddrSCID("dero1qwallet", strings.Repeat("a", 64), 100)
	batch.AddAddrSCID("dero1qwallet", strings.Repeat("b", 64), 200)
	batch.AddOwner(strings.Repeat("a", 64), "dero1qwallet")
	batch.AddOwner(strings.Repeat("b", 64), "dero1qwallet")
	batch.LastHeight = 200
	if err := store.Inner().FlushBatch(batch); err != nil {
		t.Fatal(err)
	}

	h := &HyperGnomon{store: store, stop: make(chan struct{}), pollDone: make(chan struct{})}
	go h.pollProgress()

	// The poller samples immediately on start; wait for the first cached
	// sample to land, then assert Progress serves the cached values.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if h.cachedScids.Load() == 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	scids, last, chain, _ := h.Progress()
	if scids != 2 {
		t.Fatalf("cached progress: got scids=%d, want 2", scids)
	}
	if last != 0 || chain != 0 {
		t.Fatalf("no indexer attached: got last=%d chain=%d, want 0/0", last, chain)
	}

	// Progress must stay cheap: 1000 consecutive calls in a tight loop must
	// return identical values (atomic reads only, no store access) — the
	// pre-fix implementation walked the whole owners bucket per call.
	firstScids, firstLast, firstChain, firstStatus := h.Progress()
	for i := 0; i < 1000; i++ {
		gotScids, gotLast, gotChain, gotStatus := h.Progress()
		if gotScids != firstScids || gotLast != firstLast || gotChain != firstChain || gotStatus != firstStatus {
			t.Fatalf("call %d: (%d,%d,%d,%q) != first (%d,%d,%d,%q)",
				i, gotScids, gotLast, gotChain, gotStatus, firstScids, firstLast, firstChain, firstStatus)
		}
	}

	// Closing the store must not panic Progress: cached values remain
	// readable (the poller stops refreshing them).
	_ = store.Close()

	// Close must end the poller promptly (pollDone closes).
	close(h.stop)
	select {
	case <-h.pollDone:
	case <-time.After(5 * time.Second):
		t.Fatal("poller did not exit after Close")
	}

	// Progress stays safe after the poller exits (cached values remain).
	_, _, _, _ = h.Progress()
}

// TestHyperGnomonCountCached verifies Count() is served from the same cached
// atomics as Progress() — it must never scan the owners bucket on the UI
// thread.
func TestHyperGnomonCountCached(t *testing.T) {
	store, err := hgstorage.NewBBoltDB(t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}

	batch := hgnative.NewWriteBatch()
	batch.AddAddrSCID("dero1qwallet", strings.Repeat("a", 64), 100)
	batch.AddOwner(strings.Repeat("a", 64), "dero1qwallet")
	batch.LastHeight = 100
	if err := store.Inner().FlushBatch(batch); err != nil {
		t.Fatal(err)
	}

	h := &HyperGnomon{store: store, stop: make(chan struct{}), pollDone: make(chan struct{})}
	defer close(h.stop)
	go h.pollProgress()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if h.cachedScids.Load() == 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if got := h.Count(); got != 1 {
		t.Fatalf("cached count: got %d, want 1", got)
	}

	// A nil receiver is safe and stays at zero.
	var nilH *HyperGnomon
	if got := nilH.Count(); got != 0 {
		t.Fatalf("nil receiver count: got %d, want 0", got)
	}
}

// TestHyperGnomonCloseStopsPoller verifies Close ends the background poller
// promptly (pollDone closes) and that the poller never revives a closed
// instance by re-sampling after the store handle is gone.
func TestHyperGnomonCloseStopsPoller(t *testing.T) {
	store, err := hgstorage.NewBBoltDB(t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}

	h := &HyperGnomon{store: store, stop: make(chan struct{}), pollDone: make(chan struct{})}
	go h.pollProgress()

	// Close must end the poller promptly (pollDone closes) and clear the
	// running state.
	h.Close()
	select {
	case <-h.pollDone:
	case <-time.After(5 * time.Second):
		t.Fatal("poller did not exit after Close")
	}
	if h.IsRunning() {
		t.Fatal("instance still running after Close")
	}

	// Close is idempotent.
	h.Close()

	// A poller over an instance with no store and no index exits promptly
	// (nothing to sample, nothing to revive).
	empty := &HyperGnomon{stop: make(chan struct{}), pollDone: make(chan struct{})}
	go empty.pollProgress()
	close(empty.stop)
	select {
	case <-empty.pollDone:
	case <-time.After(5 * time.Second):
		t.Fatal("poller did not exit for a closed instance")
	}
}
