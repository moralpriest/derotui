// Copyright 2017-2026 DERO Project. All rights reserved.

package daemon

import (
	"net"
	"os"
	"testing"
	"time"

	"github.com/deroproject/dero-wallet-cli/internal/config"
)

// waitFor polls until cond returns true or the timeout elapses.
func waitFor(t *testing.T, timeout time.Duration, cond func() bool) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return cond()
}

// TestFindPIDByAddressLoopback verifies detection for 127.0.0.1 bindings —
// derod's default RPC bind, which the old code never matched.
func TestFindPIDByAddressLoopback(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	addr := ln.Addr().String()

	pid := FindPIDByAddress(addr)
	if pid == 0 {
		t.Fatalf("FindPIDByAddress(%s) returned 0, expected own pid %d", addr, os.Getpid())
	}
	if pid != os.Getpid() {
		t.Fatalf("FindPIDByAddress(%s) = %d, want %d", addr, pid, os.Getpid())
	}
}

// TestManagerStopResetsManagedState verifies that after a managed daemon is
// stopped, the snapshot no longer claims ownership: Running=false and
// Managed=false, so the UI offers Start instead of a no-op Stop.
func TestManagerStopResetsManagedState(t *testing.T) {
	m := NewManager()
	if err := m.Start(config.DaemonSettings{BinaryPath: "/bin/sleep", DataDir: t.TempDir()}); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !waitFor(t, 3*time.Second, func() bool { return m.Snapshot().Running }) {
		t.Fatal("daemon did not start")
	}
	if !m.Snapshot().Managed {
		t.Fatal("started daemon should be managed")
	}

	if err := m.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	snap := m.Snapshot()
	if snap.Running {
		t.Fatal("snapshot still running after Stop")
	}
	if snap.Managed {
		t.Fatal("snapshot still managed after Stop")
	}
	if snap.PID != 0 {
		t.Fatalf("snapshot PID = %d, want 0", snap.PID)
	}
}

// TestManagerExitedProcessClearsManaged verifies that a managed daemon which
// exits on its own (crash/exit) also releases the Managed flag.
func TestManagerExitedProcessClearsManaged(t *testing.T) {
	m := NewManager()
	if err := m.Start(config.DaemonSettings{BinaryPath: "/bin/sleep", DataDir: t.TempDir()}); err != nil {
		t.Fatalf("Start: %v", err)
	}
	// /bin/sleep 0 exits immediately — simulate a daemon that dies. The
	// manager's wait() goroutine observes the exit and clears Managed.
	_ = m.cmd.Process.Kill()
	if !waitFor(t, 3*time.Second, func() bool { return !m.Snapshot().Running }) {
		t.Fatal("daemon did not exit")
	}
	snap := m.Snapshot()
	if snap.Managed {
		t.Fatal("exited daemon must not remain managed")
	}
	if snap.LastExit == "" {
		t.Fatal("expected LastExit to be recorded")
	}
}

// TestFindPIDByAddressWildcard verifies detection for 0.0.0.0 bindings.
func TestFindPIDByAddressWildcard(t *testing.T) {
	ln, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	addr := ln.Addr().String()

	pid := FindPIDByAddress(addr)
	if pid == 0 {
		t.Fatalf("FindPIDByAddress(%s) returned 0, expected own pid %d", addr, os.Getpid())
	}
	if pid != os.Getpid() {
		t.Fatalf("FindPIDByAddress(%s) = %d, want %d", addr, pid, os.Getpid())
	}
}
