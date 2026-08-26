// Copyright 2017-2026 DERO Project. All rights reserved.

package miner

import (
	"strings"
	"testing"
	"time"

	"go-miner/pkg/fakegetwork"
)

// validMainnetWallet is Dirtybird's built-in default payout address (a
// checksum-valid mainnet address), used because Start validates it.
const validMainnetWallet = "dero1qyvuemd6z0uzsx5ufc99f0jhyzvvpysmrd2t3526ht7a9dfh7jve2qqt0vu5y"

// TestEngineMinerStartsMinesAndStops is the end-to-end check that the
// engine-backed miner works against a real getwork websocket endpoint.
func TestEngineMinerStartsMinesAndStops(t *testing.T) {
	srv := fakegetwork.Start(fakegetwork.Config{
		Jobs: []fakegetwork.Job{
			fakegetwork.ValidJob("job-0", 1000),
			fakegetwork.ValidJob("job-1", 1001),
			fakegetwork.ValidJob("job-2", 1002),
		},
		PushInterval: 50 * time.Millisecond,
	})
	defer srv.Close()

	m := NewEngineMiner(EngineMinerConfig{
		Address:    validMainnetWallet,
		Threads:    1,
		DaemonHost: srv.URL(),
	})
	if err := m.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer m.Stop()

	// Wait for a hashrate reading (the engine needs a job + ~1s stats tick).
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if m.IsRunning() && m.GetHashrate() > 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !m.IsRunning() {
		t.Fatal("miner not running after Start")
	}
	if got := m.GetHashrate(); got == 0 {
		t.Fatal("hashrate stayed 0; the engine is not mining")
	}
	if got := m.GetDaemonHost(); got != srv.URL() {
		t.Fatalf("DaemonHost = %q", got)
	}
	if got := m.GetThreads(); got != 1 {
		t.Fatalf("Threads = %d, want 1", got)
	}
	if !strings.HasPrefix(m.GetAddress(), "dero1") {
		t.Fatalf("Address = %q", m.GetAddress())
	}

	m.Stop()
	if m.IsRunning() {
		t.Fatal("miner still running after Stop")
	}

	// Idempotent stop.
	m.Stop()
}

// TestEngineMinerRejectsEmptyConfig pins the config validation.
func TestEngineMinerRejectsEmptyConfig(t *testing.T) {
	cases := []struct {
		name string
		cfg  EngineMinerConfig
	}{
		{"no daemon", EngineMinerConfig{Address: "dero1qytest"}},
		{"no address", EngineMinerConfig{DaemonHost: "localhost:10100"}},
		{"bad address", EngineMinerConfig{Address: "not-an-address", DaemonHost: "localhost:10100"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := NewEngineMiner(c.cfg)
			if err := m.Start(); err == nil {
				m.Stop()
				t.Fatal("Start succeeded; want error")
			}
			if m.IsRunning() {
				t.Fatal("miner running after failed Start")
			}
		})
	}
}
