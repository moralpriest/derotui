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
}
