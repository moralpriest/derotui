// Copyright 2017-2026 DERO Project. All rights reserved.

package daemon

import "time"

// Snapshot represents current managed daemon state.
type Snapshot struct {
	Running       bool
	Managed       bool
	PID           int
	StartedAt     time.Time
	BinaryPath    string
	DataDir       string
	RPCBind       string
	P2PBind       string
	GetWorkBind   string
	Network       string
	LastError     string
	LastExit      string
	LaunchArgs    []string
	RestartNeeded bool
	// Sync state, populated for the embedded helper (which has in-process
	// access to the peer height). Remote/managed nodes leave these zero and
	// the UI derives state from a network reference height instead.
	PeerHeight            int64
	SyncProgress          float64
	BootstrapProgress     float64
	IsFinalizingBootstrap bool
}
