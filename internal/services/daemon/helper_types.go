// Copyright 2017-2026 DERO Project. All rights reserved.

package daemon

import appconfig "github.com/deroproject/dero-wallet-cli/internal/config"

type helperRequest struct {
	Action   string                    `json:"action"`
	Settings *appconfig.DaemonSettings `json:"settings,omitempty"`
	Address  string                    `json:"address,omitempty"`
	Threads  int                       `json:"threads,omitempty"`
}

type helperMinerStatus struct {
	Running  bool   `json:"running"`
	Hashrate uint64 `json:"hashrate"`
	Blocks   uint64 `json:"blocks"`
	Threads  int    `json:"threads"`
	Address  string `json:"address"`
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
	Miner               helperMinerStatus `json:"miner,omitempty"`
}
