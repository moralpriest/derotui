// Copyright 2017-2026 DERO Project. All rights reserved.

package miner

import (
	"os"
	"testing"
	"time"
)

// TestEngineMinerLiveAgainstPublicNode mines against a real public DERO
// getwork endpoint for a few seconds and asserts a nonzero hashrate. It is
// the pre-PR smoke test for the engine-backed /miner path and is gated
// behind LIVE_MINE=1 so the default suite stays hermetic.
//
// Usage:
//
//	LIVE_MINE=1 go test -run TestEngineMinerLiveAgainstPublicNode -v ./internal/services/miner/
func TestEngineMinerLiveAgainstPublicNode(t *testing.T) {
	if os.Getenv("LIVE_MINE") == "" {
		t.Skip("set LIVE_MINE=1 to mine against a public DERO node")
	}
	m := NewEngineMiner(EngineMinerConfig{
		Address:    validMainnetWallet,
		Threads:    2,
		DaemonHost: "node.derofoundation.org:10100",
	})
	if err := m.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer m.Stop()

	deadline := time.Now().Add(15 * time.Second)
	var peak uint64
	for time.Now().Before(deadline) {
		if got := m.GetHashrate(); got > peak {
			peak = got
		}
		if peak > 0 {
			t.Logf("hashrate=%d H/s blocks=%d running=%v", m.GetHashrate(), m.GetBlocks(), m.IsRunning())
			time.Sleep(2 * time.Second)
			continue
		}
		time.Sleep(200 * time.Millisecond)
	}
	if peak == 0 {
		t.Fatal("hashrate stayed 0 for 15s against a live node; engine is not mining")
	}
	t.Logf("peak hashrate: %d H/s — engine mines against a real DERO daemon", peak)
}
