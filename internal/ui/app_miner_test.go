// Copyright 2017-2026 DERO Project. All rights reserved.

package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/deroproject/dero-wallet-cli/internal/config"
)

// validMainnetWallet is a checksum-valid mainnet address used by the miner
// tests (same constant as internal/services/miner).
const validMainnetWallet = "dero1qyvuemd6z0uzsx5ufc99f0jhyzvvpysmrd2t3526ht7a9dfh7jve2qqt0vu5y"

// TestMinerStartStoresRPCBackendOnLiveModel is the regression test for the
// engine-backed /miner start flow. Model.Update is a value receiver, so a
// command closure that assigns m.rpcMiner writes to a discarded copy; the
// live model never sees the miner and the UI keeps reporting "Stopped".
// The backend must travel back through minerControlMsg and be stored in
// Update, where the returned model is the live one.
func TestMinerStartStoresRPCBackendOnLiveModel(t *testing.T) {
	// Isolate config writes so SetLastMiningAddress does not touch the real
	// ~/.derotui.json.
	t.Setenv("HOME", t.TempDir())
	if err := config.SetLastMiningAddress(validMainnetWallet); err != nil {
		t.Fatalf("SetLastMiningAddress: %v", err)
	}

	m := NewModel()
	m.page = PageMiner
	// Any host works: engine.Start validates config and launches goroutines
	// without dialing synchronously, so the endpoint need not be reachable.
	m.Opts.DaemonAddress = "127.0.0.1:9999"
	m.miner.SetThreads(1)

	// Press S on the miner page.
	res, cmd := m.Update(tea.KeyPressMsg{Text: "s"})
	if cmd == nil {
		t.Fatal("expected a command after pressing S on the miner page")
	}

	// Execute the returned command (startMinerCmd) and feed the result back.
	msg := cmd()
	ctrl, ok := msg.(minerControlMsg)
	if !ok {
		t.Fatalf("expected minerControlMsg after start, got %T", msg)
	}
	if ctrl.miner == nil {
		t.Fatal("startMinerCmd returned minerControlMsg without a backend")
	}
	if ctrl.err != "" {
		t.Fatalf("startMinerCmd returned error: %s", ctrl.err)
	}

	res2, _ := res.Update(msg)
	model := res2.(Model)

	if model.rpcMiner == nil {
		t.Fatal("rpcMiner not stored on the live model after start (closure capture bug)")
	}
	if !model.rpcMiner.IsRunning() {
		t.Fatal("rpcMiner backend is not running after start")
	}
	if !model.miner.Running {
		t.Fatal("miner page not marked running after start")
	}

	// The periodic stats command must now report the running miner.
	statsMsg := model.minerStatsCmd()().(minerStatsMsg)
	if !statsMsg.running {
		t.Fatal("minerStatsMsg.running is false even though the miner is running")
	}
	// Stats plumbing: the live model must carry the backend's indicator values.
	// (Difficulty/height/hashes may be 0 before the first job arrives, but the
	// fields must be wired through the message to the page model.)
	res2, _ = res2.Update(statsMsg)
	model = res2.(Model)
	if !model.miner.Running {
		t.Fatal("miner page not running after stats update")
	}

	// Press X to stop: the stopped backend must flow back through the message
	// so the page shows Stopped.
	res, cmd = model.Update(tea.KeyPressMsg{Text: "x"})
	if cmd == nil {
		t.Fatal("expected a command after pressing X on the miner page")
	}
	res2, _ = res.Update(cmd())
	model = res2.(Model)
	if model.miner.Running {
		t.Fatal("miner page still running after stop")
	}
	if model.rpcMiner.IsRunning() {
		t.Fatal("rpcMiner backend still running after stop")
	}
}
