// Copyright 2017-2026 DERO Project. All rights reserved.

package daemon

import (
	"time"

	appconfig "github.com/deroproject/dero-wallet-cli/internal/config"
)

type helperRequest struct {
	Action   string                    `json:"action"`
	Settings *appconfig.DaemonSettings `json:"settings,omitempty"`
	Address  string                    `json:"address,omitempty"`
	Threads  int                       `json:"threads,omitempty"`
}

type helperMinerStatus struct {
	Running    bool          `json:"running"`
	Hashrate   uint64        `json:"hashrate"`
	Blocks     uint64        `json:"blocks"`
	Minis      uint64        `json:"minis"`
	Rejected   uint64        `json:"rejected"`
	Height     uint64        `json:"height"`
	Difficulty uint64        `json:"difficulty"`
	Hashes     uint64        `json:"hashes"`
	Uptime     time.Duration `json:"uptime"`
	Threads    int           `json:"threads"`
	Address    string        `json:"address"`
}

type helperResponse struct {
	OK                  bool              `json:"ok"`
	Error               string            `json:"error,omitempty"`
	Snapshot            Snapshot          `json:"snapshot,omitempty"`
	Info                map[string]any    `json:"info,omitempty"`
	Logs                []string          `json:"logs,omitempty"`
	RPCBind             string            `json:"rpc_bind,omitempty"`
	PeerHeight          int64             `json:"peer_height,omitempty"`
	SyncProgress        float64           `json:"sync_progress,omitempty"`
	FinalizingBootstrap bool              `json:"finalizing_bootstrap,omitempty"`
	IncomingPeers       uint64            `json:"incoming_peers,omitempty"`
	OutgoingPeers       uint64            `json:"outgoing_peers,omitempty"`
	KnownPeers          uint64            `json:"known_peers,omitempty"`
	BootstrapHeight     int64             `json:"bootstrap_height,omitempty"`
	BootstrapProgress   float64           `json:"bootstrap_progress,omitempty"`
	BootstrapChunk      int64             `json:"bootstrap_chunk,omitempty"`
	BootstrapStep       uint              `json:"bootstrap_step,omitempty"`
	Miner               helperMinerStatus `json:"miner,omitempty"`
}
