// Copyright 2017-2026 DERO Project. All rights reserved.

package ui

import (
	"context"
	"testing"

	"github.com/deroproject/dero-wallet-cli/internal/wallet"
)

// setMockReference installs a stub network-tip fetcher so tests don't hit the
// network, and restores the real one afterwards.
func setMockReference(t *testing.T, fn func(network string) (uint64, bool)) {
	t.Helper()
	orig := fetchNetworkReferenceHeight
	fetchNetworkReferenceHeight = func(_ context.Context, network string) (uint64, bool) {
		return fn(network)
	}
	t.Cleanup(func() { fetchNetworkReferenceHeight = orig })
}

// clearReferenceCache empties the tip-height cache so a mocked fetcher always
// runs.
func clearReferenceCache() {
	referenceHeightMu.Lock()
	defer referenceHeightMu.Unlock()
	for k := range referenceHeightCache {
		delete(referenceHeightCache, k)
	}
}

func TestClassifyDaemonSyncOffline(t *testing.T) {
	clearReferenceCache()
	setMockReference(t, func(network string) (uint64, bool) {
		return 0, false
	})

	info := classifyDaemonSync(wallet.DaemonInfo{IsOnline: false, IsHealthy: false}, "mainnet", 0)
	if info.IsSynced || info.IsSyncing || info.IsBootstrapping {
		t.Fatal("offline daemon must not be reported synced/syncing/bootstrapping")
	}
	if info.PeerHeight != 0 || info.SyncProgress != 0 {
		t.Fatalf("offline daemon must zero peer height/progress, got %d/%f", info.PeerHeight, info.SyncProgress)
	}
}

func TestClassifyDaemonSyncPeerHeightSyncing(t *testing.T) {
	clearReferenceCache()
	setMockReference(t, func(network string) (uint64, bool) {
		return 0, false
	})

	info := classifyDaemonSync(wallet.DaemonInfo{IsOnline: true, IsHealthy: true, Height: 532}, "mainnet", 7_414_000)
	if !info.IsSyncing {
		t.Fatal("expected IsSyncing when height below peer height")
	}
	if info.IsSynced || info.IsBootstrapping {
		t.Fatal("node catching up must not be synced or bootstrapping")
	}
	if info.PeerHeight != 7_414_000 {
		t.Fatalf("expected peer height to be kept, got %d", info.PeerHeight)
	}
	if info.SyncProgress < 0.007 || info.SyncProgress > 0.008 {
		t.Fatalf("expected tiny sync progress, got %f", info.SyncProgress)
	}
}

func TestClassifyDaemonSyncPeerHeightSynced(t *testing.T) {
	clearReferenceCache()
	setMockReference(t, func(network string) (uint64, bool) {
		return 0, false
	})

	info := classifyDaemonSync(wallet.DaemonInfo{IsOnline: true, IsHealthy: true, Height: 7_414_000}, "mainnet", 7_414_000)
	if !info.IsSynced || info.IsSyncing {
		t.Fatal("expected synced at peer height")
	}
	if info.IsBootstrapping {
		t.Fatal("synced node must not be bootstrapping")
	}
	if info.SyncProgress != 100 {
		t.Fatalf("expected 100%% progress, got %f", info.SyncProgress)
	}
}

func TestClassifyDaemonSyncBootstrappingWhenTopoAhead(t *testing.T) {
	clearReferenceCache()
	setMockReference(t, func(network string) (uint64, bool) {
		return 0, false
	})

	// During initial sync DERO runs topoheight ahead of height.
	info := classifyDaemonSync(wallet.DaemonInfo{IsOnline: true, IsHealthy: true, Height: 532, TopoHeight: 533}, "mainnet", 7_414_000)
	if !info.IsBootstrapping {
		t.Fatal("expected bootstrapping when topoheight differs from height")
	}
	if !info.IsSyncing {
		t.Fatal("expected syncing alongside bootstrapping")
	}
}

func TestClassifyDaemonSyncNoReferenceStaysHonest(t *testing.T) {
	clearReferenceCache()
	setMockReference(t, func(network string) (uint64, bool) {
		return 0, false
	})

	info := classifyDaemonSync(wallet.DaemonInfo{IsOnline: true, IsHealthy: true, Height: 532}, "mainnet", 0)
	if info.IsSynced || info.IsSyncing {
		t.Fatal("node with no reference must not be reported synced/syncing")
	}
	if info.PeerHeight != 0 {
		t.Fatal("peer height must stay zero when no reference is reachable")
	}
}

func TestClassifyDaemonSyncReferenceHeightFallback(t *testing.T) {
	clearReferenceCache()
	setMockReference(t, func(network string) (uint64, bool) {
		if network == "testnet" {
			return 5_000_000, true
		}
		return 0, false
	})

	info := classifyDaemonSync(wallet.DaemonInfo{IsOnline: true, IsHealthy: true, Height: 1_000_000}, "testnet", 0)
	if !info.IsSyncing {
		t.Fatal("expected syncing from network reference height")
	}
	if info.PeerHeight != 5_000_000 {
		t.Fatalf("expected reference as peer height, got %d", info.PeerHeight)
	}
	if info.SyncProgress != 20 {
		t.Fatalf("expected 20%% progress, got %f", info.SyncProgress)
	}
}

func TestClassifyDaemonSyncSimulatorAlwaysSynced(t *testing.T) {
	clearReferenceCache()
	setMockReference(t, func(network string) (uint64, bool) {
		return 0, false
	})

	info := classifyDaemonSync(wallet.DaemonInfo{IsOnline: true, IsHealthy: true, Height: 10}, "simulator", 0)
	if !info.IsSynced {
		t.Fatal("online simulator must be synced")
	}
	if info.IsSyncing || info.IsBootstrapping {
		t.Fatal("simulator must not be syncing/bootstrapping")
	}
	if info.SyncProgress != 100 {
		t.Fatalf("expected 100%% progress for simulator, got %f", info.SyncProgress)
	}
}

func TestSyncProgressRatioClamping(t *testing.T) {
	if got := syncProgressRatio(0, 100); got != 0 {
		t.Fatalf("zero height -> 0, got %f", got)
	}
	if got := syncProgressRatio(100, 0); got != 0 {
		t.Fatalf("zero target -> 0, got %f", got)
	}
	if got := syncProgressRatio(50, 100); got != 50 {
		t.Fatalf("expected 50, got %f", got)
	}
	if got := syncProgressRatio(200, 100); got != 100 {
		t.Fatalf("height above target clamps to 100, got %f", got)
	}
}
