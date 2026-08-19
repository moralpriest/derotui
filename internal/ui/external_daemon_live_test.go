// Copyright 2017-2026 DERO Project. All rights reserved.

package ui

import (
	"context"
	"testing"

	"github.com/deroproject/dero-wallet-cli/internal/config"
	"github.com/deroproject/dero-wallet-cli/internal/wallet"
)

// TestExternalDaemonLivePipeline runs the external-daemon detection, log
// discovery, and settings pre-population against a live local daemon.
// Skipped when no daemon answers at the default RPC address.
func TestExternalDaemonLivePipeline(t *testing.T) {
	addr := "127.0.0.1:10102"
	info := wallet.GetDaemonInfo(context.Background(), addr)
	if !info.IsOnline {
		t.Skipf("no live daemon at %s", addr)
	}

	settings := config.GetDaemonSettings()
	snap := snapshotFromExternal(settings, addr, info)
	t.Logf("PID=%d Managed=%v Running=%v", snap.PID, snap.Managed, snap.Running)
	t.Logf("LaunchArgs=%v", snap.LaunchArgs)
	t.Logf("BinaryPath=%s DataDir=%s RPCBind=%s", snap.BinaryPath, snap.DataDir, snap.RPCBind)
	if snap.PID == 0 {
		t.Fatal("PID detection failed for live daemon")
	}

	lines := readExternalLogLines(settings, snap)
	t.Logf("log lines found: %d", len(lines))
	if len(lines) > 0 {
		t.Logf("first line: %q", lines[0])
		t.Logf("last line: %q", lines[len(lines)-1])
	} else {
		t.Fatal("no logs found for live external daemon")
	}

	// Full model path: applyDaemonManagerMsg then settings pre-population.
	m := NewModel()
	m.applyDaemonManagerMsg(daemonManagerMsg{
		source:   "External Local",
		snapshot: snap,
		info:     info,
	})
	psnap := m.daemonStatus.Snapshot
	t.Logf("page source=%q managed=%v pid=%d", psnap.Source, psnap.Managed, psnap.PID)
	if psnap.Source != "External Local" {
		t.Fatalf("expected External Local, got %q", psnap.Source)
	}
	if psnap.Managed {
		t.Fatal("external daemon must not be managed")
	}

	got := daemonSettingsForSnapshot(config.GetDaemonSettings(), psnap)
	t.Logf("settings: mode=%s binary=%s logdir=%s datadir=%s rpc=%s", got.Mode, got.BinaryPath, got.LogDir, got.DataDir, got.RPCBind)
	if got.Mode != "external" {
		t.Fatalf("expected external mode, got %q", got.Mode)
	}
	if got.LogDir == "" {
		t.Fatal("expected real --log-dir in settings")
	}
}
