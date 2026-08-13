// Copyright 2017-2026 DERO Project. All rights reserved.

package daemon

import (
	"os"
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
	if settings.FastSync && !dataDirHasData(settings.DataDir) {
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

func dataDirHasData(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	return len(entries) > 0
}
