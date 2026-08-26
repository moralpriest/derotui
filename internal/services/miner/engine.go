// Copyright 2017-2026 DERO Project. All rights reserved.

package miner

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/deroproject/derohe/globals"

	"go-miner/pkg/engine"
)

// RPCBackend is the miner surface the UI drives for the external-daemon
// path (no embedded derod). Both the legacy RPCMiner and the engine-backed
// EngineMiner satisfy it.
type RPCBackend interface {
	Start() error
	Stop()
	IsRunning() bool
	GetHashrate() uint64
	// GetBlocks returns full blocks found; GetMinis returns accepted
	// miniblocks. The engine tracks them separately (go-miner splits them the
	// same way in its status line).
	GetBlocks() uint64
	GetMinis() uint64
	GetRejected() uint64
	GetHeight() uint64
	GetDifficulty() uint64
	// GetHashes returns the cumulative number of hash computations.
	GetHashes() uint64
	// GetUptime returns how long the current mining session has been running.
	GetUptime() time.Duration
	GetThreads() int
	GetAddress() string
	GetDaemonHost() string
}

// EngineMinerConfig holds configuration for the engine-backed miner.
type EngineMinerConfig struct {
	Address    string
	Threads    int
	DaemonHost string // host:port of getwork endpoint (bare host:port = wss)
}

// EngineMiner embeds Dirtybird's go-miner pkg/engine (getwork client +
// AstroBWTv3 workers + KAT self-test) behind the UI's RPCBackend surface.
// It is the successor to RPCMiner, which ran a hand-rolled getwork loop.
type EngineMiner struct {
	cfg      EngineMinerConfig
	mu       sync.RWMutex
	engine   *engine.Engine
	cancel   context.CancelFunc
	running  atomic.Bool
	started  time.Time
	hashrate atomic.Uint64
	blocks   atomic.Uint64 // full blocks
	minis    atomic.Uint64 // accepted miniblocks
	rejected atomic.Uint64
	height   atomic.Uint64
	diff     atomic.Uint64
	hashes   atomic.Uint64
}

// NewEngineMiner creates a new engine-backed miner instance.
func NewEngineMiner(cfg EngineMinerConfig) *EngineMiner {
	return &EngineMiner{cfg: cfg}
}

// Start validates the address, then starts the embedded go-miner engine
// (which runs the AstroBWTv3 KAT and refuses to mine on a broken hash).
func (m *EngineMiner) Start() error {
	if m.running.Load() {
		return fmt.Errorf("miner already running")
	}
	if strings.TrimSpace(m.cfg.DaemonHost) == "" {
		return fmt.Errorf("mining daemon host is empty")
	}
	if strings.TrimSpace(m.cfg.Address) == "" {
		return fmt.Errorf("mining address is empty")
	}

	// Same validation/normalization the legacy miner applied.
	addr, err := globals.ParseValidateAddress(m.cfg.Address)
	if err != nil {
		return fmt.Errorf("invalid mining address: %w", err)
	}
	m.cfg.Address = addr.String()

	ctx, cancel := context.WithCancel(context.Background())
	eng, err := engine.Start(ctx, engine.Config{
		Endpoint: m.cfg.DaemonHost,
		Wallet:   m.cfg.Address,
		Threads:  m.cfg.Threads,
	})
	if err != nil {
		cancel()
		return err
	}

	m.mu.Lock()
	m.engine = eng
	m.cancel = cancel
	m.started = time.Now()
	m.mu.Unlock()
	m.running.Store(true)
	go m.statsLoop(ctx)
	return nil
}

// Stop cancels the engine context and waits for all miner goroutines.
func (m *EngineMiner) Stop() {
	if !m.running.Load() {
		return
	}
	m.mu.RLock()
	cancel, eng := m.cancel, m.engine
	m.mu.RUnlock()
	if cancel != nil {
		cancel()
	}
	if eng != nil {
		eng.Stop() // idempotent; also joins worker/client goroutines
	}
}

// IsRunning reports whether the engine was started and has not been stopped.
func (m *EngineMiner) IsRunning() bool { return m.running.Load() }

// GetHashrate returns the current hashrate in H/s (1s-refreshed).
func (m *EngineMiner) GetHashrate() uint64 { return m.hashrate.Load() }

// GetBlocks returns full blocks found (engine Stats.Blocks).
func (m *EngineMiner) GetBlocks() uint64 { return m.blocks.Load() }

// GetMinis returns accepted miniblocks found (engine Stats.MiniBlocks).
func (m *EngineMiner) GetMinis() uint64 { return m.minis.Load() }

// GetRejected returns rejected shares (engine Stats.Rejected).
func (m *EngineMiner) GetRejected() uint64 { return m.rejected.Load() }

// GetHeight returns the chain height of the current job (engine Stats.Height).
func (m *EngineMiner) GetHeight() uint64 { return m.height.Load() }

// GetDifficulty returns the difficulty of the current job (engine Stats.Difficulty).
func (m *EngineMiner) GetDifficulty() uint64 { return m.diff.Load() }

// GetHashes returns the cumulative hash count (engine Stats.Hashes).
func (m *EngineMiner) GetHashes() uint64 { return m.hashes.Load() }

// GetUptime returns the duration the current mining session has been running.
func (m *EngineMiner) GetUptime() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.running.Load() || m.started.IsZero() {
		return 0
	}
	return time.Since(m.started)
}

// GetThreads returns the configured thread count.
func (m *EngineMiner) GetThreads() int { return m.cfg.Threads }

// GetAddress returns the mining address.
func (m *EngineMiner) GetAddress() string { return m.cfg.Address }

// GetDaemonHost returns the getwork endpoint being mined against.
func (m *EngineMiner) GetDaemonHost() string { return m.cfg.DaemonHost }

// statsLoop mirrors engine stats into atomic counters so the UI reads stay
// cheap, and clears the running flag when the engine stops.
func (m *EngineMiner) statsLoop(ctx context.Context) {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			m.running.Store(false)
			m.hashrate.Store(0)
			return
		case <-t.C:
			m.mu.RLock()
			eng := m.engine
			m.mu.RUnlock()
			if eng == nil {
				continue
			}
			s := eng.Stats()
			m.hashrate.Store(uint64(s.Hashrate))
			m.blocks.Store(s.Blocks)
			m.minis.Store(s.MiniBlocks)
			m.rejected.Store(s.Rejected)
			m.height.Store(s.Height)
			m.diff.Store(s.Difficulty)
			m.hashes.Store(s.Hashes)
		}
	}
}
