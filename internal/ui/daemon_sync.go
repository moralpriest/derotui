// Copyright 2017-2026 DERO Project. All rights reserved.

package ui

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/deroproject/dero-wallet-cli/internal/wallet"
)

const (
	// referenceHeightTTL is how long a successful network-tip-height lookup
	// stays cached before being refreshed.
	referenceHeightTTL = 5 * time.Minute
	// referenceHeightErrTTL keeps failed lookups from being retried on every
	// 5-second status tick while the public node is down.
	referenceHeightErrTTL = 30 * time.Second
)

// fetchNetworkReferenceHeight returns the chain tip height of a well-known
// public node for the given network ("mainnet"/"testnet"). Simulator chains
// are local-only and need no reference. Injectable for tests.
var fetchNetworkReferenceHeight = func(ctx context.Context, network string) (uint64, bool) {
	switch strings.ToLower(strings.TrimSpace(network)) {
	case "mainnet":
		return fetchReferenceHeightFrom(wallet.FallbackMainnetDaemon)
	case "testnet":
		return fetchReferenceHeightFrom(wallet.FallbackTestnetDaemon)
	default:
		return 0, false
	}
}

func fetchReferenceHeightFrom(address string) (uint64, bool) {
	info := wallet.GetDaemonInfo(context.Background(), address)
	if !info.IsOnline || !info.IsHealthy || info.Height == 0 {
		return 0, false
	}
	return info.Height, true
}

type referenceHeightEntry struct {
	height uint64
	ok     bool
	at     time.Time
}

var (
	referenceHeightMu    sync.Mutex
	referenceHeightCache = map[string]referenceHeightEntry{}
)

// networkReferenceHeight returns the cached network tip height for a network,
// fetching it from a public node on a cache miss.
func networkReferenceHeight(network string) (uint64, bool) {
	net := strings.ToLower(strings.TrimSpace(network))

	referenceHeightMu.Lock()
	if entry, ok := referenceHeightCache[net]; ok {
		ttl := referenceHeightTTL
		if !entry.ok {
			ttl = referenceHeightErrTTL
		}
		if time.Since(entry.at) < ttl {
			referenceHeightMu.Unlock()
			return entry.height, entry.ok
		}
	}
	referenceHeightMu.Unlock()

	height, ok := fetchNetworkReferenceHeight(context.Background(), net)

	referenceHeightMu.Lock()
	referenceHeightCache[net] = referenceHeightEntry{height: height, ok: ok, at: time.Now()}
	referenceHeightMu.Unlock()
	return height, ok
}

// classifyDaemonSync derives the sync state (Synced / Syncing / Bootstrapping)
// for a daemon from peer height when available, falling back to a cached
// network tip height from a public node. When no reference is reachable the
// daemon is honestly reported as plain "online" (never falsely "synced").
func classifyDaemonSync(info wallet.DaemonInfo, network string, peerHeight int64) wallet.DaemonInfo {
	if !info.IsOnline || !info.IsHealthy {
		info.IsSynced = false
		info.IsSyncing = false
		info.IsBootstrapping = false
		info.IsFinalizingBootstrap = false
		info.PeerHeight = 0
		info.SyncProgress = 0
		return info
	}

	// A simulator is a local-only chain; an online simulator is by
	// definition up to date.
	if strings.EqualFold(strings.TrimSpace(network), "simulator") {
		info.IsSynced = true
		info.IsSyncing = false
		info.IsBootstrapping = false
		info.PeerHeight = 0
		info.SyncProgress = 100
		return info
	}

	// Right after start the chain reports height 0; that is "starting",
	// not mid-sync — keep the state line honest.
	if info.Height == 0 {
		info.IsSynced = false
		info.IsSyncing = false
		info.IsBootstrapping = false
		info.IsFinalizingBootstrap = false
		info.SyncProgress = 0
		if peerHeight > 0 {
			info.PeerHeight = peerHeight
		}
		return info
	}

	target := peerHeight
	if target <= 0 {
		if ref, ok := networkReferenceHeight(network); ok {
			target = int64(ref)
		}
	}

	if target > 0 {
		height := int64(info.Height)
		info.PeerHeight = target
		info.SyncProgress = syncProgressRatio(height, target)
		info.IsSyncing = height < target
		info.IsSynced = height >= target
		info.IsBootstrapping = info.TopoHeight > 0 && info.TopoHeight != height
		return info
	}

	// No peer/reference height available: keep bootstrapping signal (topo
	// running ahead of height) but never claim the node is synced.
	info.IsSynced = false
	info.IsSyncing = false
	info.PeerHeight = 0
	info.SyncProgress = 0
	info.IsBootstrapping = info.TopoHeight > 0 && info.TopoHeight != int64(info.Height)
	return info
}

// syncProgressRatio returns height/peerHeight as a percentage clamped to
// [0, 100].
func syncProgressRatio(height, target int64) float64 {
	if target <= 0 || height <= 0 {
		return 0
	}
	progress := float64(height) / float64(target) * 100
	if progress > 100 {
		progress = 100
	}
	if progress < 0 {
		progress = 0
	}
	return progress
}
