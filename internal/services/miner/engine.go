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
	GetBlocks() uint64
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
	hashrate atomic.Uint64
	blocks   atomic.Uint64
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

// GetBlocks returns accepted shares (full blocks + miniblocks) found.
func (m *EngineMiner) GetBlocks() uint64 { return m.blocks.Load() }

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
			m.blocks.Store(s.Blocks + s.MiniBlocks)
		}
	}
}
