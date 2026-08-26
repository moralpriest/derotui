// Copyright 2017-2026 DERO Project. All rights reserved.

package daemon

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/deroproject/dero-wallet-cli/internal/config"
)

// ReadProcessArgs returns the argv of a running process by PID (from /proc),
// or nil if the process is gone or not readable.
func ReadProcessArgs(pid int) []string {
	if pid <= 0 {
		return nil
	}
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		return nil
	}
	var args []string
	for _, part := range strings.Split(string(data), "\x00") {
		part = strings.TrimSpace(part)
		if part != "" {
			args = append(args, part)
		}
	}
	return args
}

// FindDerodPID returns the PID of the derod process serving the given RPC
// address. It first tries socket->PID detection, then falls back to scanning
// /proc for a derod process whose command line references the port.
func FindDerodPID(rpcAddress string) int {
	if pid := FindPIDByAddress(rpcAddress); pid > 0 {
		return pid
	}
	_, port, err := net.SplitHostPort(rpcAddress)
	if err != nil {
		return 0
	}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		args := ReadProcessArgs(pid)
		if len(args) == 0 {
			continue
		}
		joined := strings.Join(args, " ")
		if !strings.Contains(strings.ToLower(joined), "derod") {
			continue
		}
		if strings.Contains(joined, port) {
			return pid
		}
	}
	return 0
}

// SettingsFromArgs overlays derod command-line arguments onto a base
// DaemonSettings, filling in every value the running process was launched
// with. Values not present on the command line keep their base value.
// Both "--flag=value" and "--flag value" forms are accepted.
func SettingsFromArgs(args []string, base config.DaemonSettings) config.DaemonSettings {
	s := base
	for i := 0; i < len(args); i++ {
		arg := strings.TrimSpace(args[i])
		if arg == "" {
			continue
		}
		// argv[0] is the binary path (may be relative).
		if i == 0 && !strings.HasPrefix(arg, "-") {
			s.BinaryPath = arg
			continue
		}
		name, value, isFlag := splitArgFlag(arg)
		if !isFlag {
			continue
		}
		if value == "" && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
			i++
			value = strings.TrimSpace(args[i])
		}
		switch name {
		case "--testnet":
			s.Network = string(config.NetworkTestnet)
		case "--simulator":
			s.Network = string(config.NetworkSimulator)
		case "--debug":
			s.Debug = true
		case "--fastsync":
			s.FastSync = true
		case "--timeisinsync":
			s.TimeIsInSync = true
		case "--sync-node":
			s.SyncNode = true
		case "--data-dir":
			s.DataDir = value
		case "--rpc-bind":
			s.RPCBind = value
		case "--p2p-bind":
			s.P2PBind = value
		case "--getwork-bind":
			s.GetWorkBind = value
		case "--socks-proxy":
			s.SocksProxy = value
		case "--integrator-address":
			s.IntegratorAddress = value
		case "--node-tag":
			s.NodeTag = value
		case "--log-dir":
			s.LogDir = value
		case "--min-peers":
			s.MinPeers = value
		case "--max-peers":
			s.MaxPeers = value
		case "--prune-history":
			s.PruneHistory = value
		case "--clog-level":
			s.ConsoleLogLevel = value
		case "--flog-level":
			s.FileLogLevel = value
		case "--add-priority-node":
			s.PriorityNodes = append(s.PriorityNodes, value)
		case "--add-exclusive-node":
			s.ExclusiveNodes = append(s.ExclusiveNodes, value)
		}
	}
	return s
}

// splitArgFlag splits a "--flag=value" argument. isFlag reports whether the
// argument is a flag at all (vs. a positional value).
func splitArgFlag(arg string) (name, value string, isFlag bool) {
	if !strings.HasPrefix(arg, "-") {
		return "", "", false
	}
	if idx := strings.Index(arg, "="); idx >= 0 {
		return arg[:idx], arg[idx+1:], true
	}
	return arg, "", true
}
