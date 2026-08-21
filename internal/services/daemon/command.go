// Copyright 2017-2026 DERO Project. All rights reserved.

package daemon

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/deroproject/dero-wallet-cli/internal/config"
)

// BuildArgs builds derod launch arguments from saved settings.
func BuildArgs(settings config.DaemonSettings) []string {
	args := make([]string, 0, 24)
	if settings.Network == string(config.NetworkTestnet) {
		args = append(args, "--testnet")
	}
	if settings.Debug {
		args = append(args, "--debug")
	}
	if settings.FastSync && !dataDirHasData(settings.DataDir, settings.Network) {
		args = append(args, "--fastsync")
	}
	if settings.TimeIsInSync {
		args = append(args, "--timeisinsync")
	}
	if settings.SyncNode {
		args = append(args, "--sync-node")
	}
	appendKV := func(flag, value string) {
		value = strings.TrimSpace(value)
		if value != "" {
			args = append(args, flag+"="+value)
		}
	}
	appendKV("--data-dir", settings.DataDir)
	appendKV("--rpc-bind", settings.RPCBind)
	appendKV("--p2p-bind", settings.P2PBind)
	appendKV("--getwork-bind", settings.GetWorkBind)
	appendKV("--socks-proxy", settings.SocksProxy)
	appendKV("--integrator-address", settings.IntegratorAddress)
	appendKV("--node-tag", settings.NodeTag)
	appendKV("--min-peers", settings.MinPeers)
	appendKV("--max-peers", settings.MaxPeers)
	appendKV("--log-dir", settings.LogDir)
	appendKV("--clog-level", settings.ConsoleLogLevel)
	appendKV("--flog-level", settings.FileLogLevel)
	if strings.TrimSpace(settings.PruneHistory) != "50" {
		appendKV("--prune-history", settings.PruneHistory)
	}
	for _, node := range settings.PriorityNodes {
		appendKV("--add-priority-node", node)
	}
	for _, node := range settings.ExclusiveNodes {
		appendKV("--add-exclusive-node", node)
	}
	return args
}

// dataDirHasData reports whether derod already has chain data for the
// given network under dataDir. The config dir itself (~/.derotui) always
// contains wallets/config, which must NOT count as chain data — otherwise
// --fastsync is skipped on a first start and the daemon does a full sync.
func dataDirHasData(dir, network string) bool {
	network = strings.ToLower(strings.TrimSpace(network))
	var names []string
	switch network {
	case "testnet":
		names = []string{"testnet", "testnet_simulator"}
	case "simulator":
		names = []string{"mainnet_simulator"}
	default:
		names = []string{"mainnet", "mainnet_simulator"}
	}
	for _, name := range names {
		if chainDirLooksPopulated(filepath.Join(dir, name)) {
			return true
		}
	}
	base := strings.ToLower(filepath.Base(dir))
	for _, name := range names {
		if base == name && chainDirLooksPopulated(dir) {
			return true
		}
	}
	return false
}

func chainDirLooksPopulated(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		n := strings.ToLower(e.Name())
		if strings.HasPrefix(n, ".") {
			continue
		}
		if strings.HasSuffix(n, ".log") || strings.HasSuffix(n, ".lock") {
			continue
		}
		return true
	}
	return false
}
