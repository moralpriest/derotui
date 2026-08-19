// Copyright 2017-2026 DERO Project. All rights reserved.

package daemon

import (
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/deroproject/dero-wallet-cli/internal/config"
)

func TestSettingsFromArgsParsesAllFlagForms(t *testing.T) {
	args := []string{
		"/home/user/.derotui/derod",
		"--testnet",
		"--debug",
		"--data-dir=/home/user/.dero-testnet",
		"--rpc-bind", "127.0.0.1:40402",
		"--p2p-bind=0.0.0.0:40401",
		"--getwork-bind=0.0.0.0:40400",
		"--node-tag", "my-node",
		"--integrator-address=dero1abc",
		"--log-dir=/var/log/dero",
		"--min-peers=5",
		"--max-peers", "50",
		"--add-priority-node=node1.example:10102",
		"--add-exclusive-node=node2.example:10102",
	}

	got := SettingsFromArgs(args, config.DaemonSettings{})

	if got.BinaryPath != "/home/user/.derotui/derod" {
		t.Errorf("BinaryPath = %q", got.BinaryPath)
	}
	if got.Network != string(config.NetworkTestnet) {
		t.Errorf("Network = %q, want testnet", got.Network)
	}
	if !got.Debug {
		t.Error("Debug should be true")
	}
	if got.DataDir != "/home/user/.dero-testnet" {
		t.Errorf("DataDir = %q", got.DataDir)
	}
	if got.RPCBind != "127.0.0.1:40402" {
		t.Errorf("RPCBind = %q", got.RPCBind)
	}
	if got.P2PBind != "0.0.0.0:40401" {
		t.Errorf("P2PBind = %q", got.P2PBind)
	}
	if got.GetWorkBind != "0.0.0.0:40400" {
		t.Errorf("GetWorkBind = %q", got.GetWorkBind)
	}
	if got.NodeTag != "my-node" {
		t.Errorf("NodeTag = %q", got.NodeTag)
	}
	if got.IntegratorAddress != "dero1abc" {
		t.Errorf("IntegratorAddress = %q", got.IntegratorAddress)
	}
	if got.LogDir != "/var/log/dero" {
		t.Errorf("LogDir = %q", got.LogDir)
	}
	if got.MinPeers != "5" || got.MaxPeers != "50" {
		t.Errorf("peers = %q/%q", got.MinPeers, got.MaxPeers)
	}
	if len(got.PriorityNodes) != 1 || got.PriorityNodes[0] != "node1.example:10102" {
		t.Errorf("PriorityNodes = %v", got.PriorityNodes)
	}
	if len(got.ExclusiveNodes) != 1 || got.ExclusiveNodes[0] != "node2.example:10102" {
		t.Errorf("ExclusiveNodes = %v", got.ExclusiveNodes)
	}
}

func TestSettingsFromArgsKeepsBaseForMissingFlags(t *testing.T) {
	base := config.DaemonSettings{
		Mode:     "external",
		DataDir:  "/base/data",
		RPCBind:  "127.0.0.1:10102",
		FastSync: true,
	}
	got := SettingsFromArgs([]string{"/usr/bin/derod", "--rpc-bind=0.0.0.0:9999"}, base)

	if got.DataDir != "/base/data" {
		t.Errorf("DataDir should stay at base, got %q", got.DataDir)
	}
	if got.RPCBind != "0.0.0.0:9999" {
		t.Errorf("RPCBind = %q, want 0.0.0.0:9999", got.RPCBind)
	}
	if !got.FastSync {
		t.Error("FastSync should stay at base value")
	}
	if got.Mode != "external" {
		t.Errorf("Mode should stay at base value, got %q", got.Mode)
	}
}

func TestSettingsFromArgsSimulatorNetwork(t *testing.T) {
	got := SettingsFromArgs([]string{"derod", "--simulator"}, config.DaemonSettings{})
	if got.Network != string(config.NetworkSimulator) {
		t.Errorf("Network = %q, want simulator", got.Network)
	}
}

func TestReadProcessArgsReadsOwnCmdline(t *testing.T) {
	args := ReadProcessArgs(os.Getpid())
	if len(args) == 0 {
		t.Fatal("expected own argv to be readable")
	}
	if !strings.Contains(args[0], ".test") && !strings.Contains(args[0], "go") && !strings.Contains(args[0], "daemon") {
		t.Logf("argv[0] = %q (informational)", args[0])
	}
}

// TestFindDerodPIDProcFallback verifies the /proc cmdline fallback finds a
// derod-named process referencing the RPC port even without a matching socket.
func TestFindDerodPIDProcFallback(t *testing.T) {
	const port = "59991"
	// bash's exec -a rewrites argv[0]; putting the port there lets us simulate
	// a derod process referencing an RPC port without actually binding it.
	cmd := exec.Command("/bin/bash", "-c", "exec -a \"derod --rpc-bind=127.0.0.1:"+port+"\" sleep 30")
	if err := cmd.Start(); err != nil {
		t.Skipf("cannot spawn process: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}()

	// Give the process a moment to appear in /proc.
	delay := time.Millisecond * 100
	var pid int
	for i := 0; i < 20; i++ {
		pid = FindDerodPID("127.0.0.1:" + port)
		if pid > 0 {
			break
		}
		time.Sleep(delay)
	}
	if pid == 0 {
		t.Fatal("FindDerodPID fallback did not find the derod-named process")
	}
	if pid != cmd.Process.Pid {
		t.Fatalf("FindDerodPID = %d, want %d", pid, cmd.Process.Pid)
	}
}
