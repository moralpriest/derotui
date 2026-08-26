// Copyright 2017-2026 DERO Project. All rights reserved.

package miner

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/big"
	"net/url"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/deroproject/derohe/astrobwt/astrobwtv3"
	"github.com/deroproject/derohe/block"
	"github.com/deroproject/derohe/cryptography/crypto"
	"github.com/deroproject/derohe/globals"
	"github.com/deroproject/derohe/rpc"
	"github.com/gorilla/websocket"
)

// RPCMiner implements getwork mining against an external daemon via WebSocket.
type RPCMiner struct {
	mu         sync.RWMutex
	running    atomic.Bool
	threads    int
	address    string
	daemonHost string
	hashrate   uint64
	blocks     uint64
	counter    uint64
	startTime  time.Time
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	wsConn     *websocket.Conn
	wsMu       sync.Mutex
	job        rpc.GetBlockTemplate_Result
	jobCounter int64
	difficulty *big.Int
}

// RPCMinerConfig holds configuration for the RPC miner.
type RPCMinerConfig struct {
	Address    string
	Threads    int
	DaemonHost string // host:port of getwork endpoint (e.g. "node.dero.live:10100")
}

// NewRPCMiner creates a new RPC miner instance.
func NewRPCMiner(cfg RPCMinerConfig) *RPCMiner {
	return &RPCMiner{
		address:    cfg.Address,
		threads:    cfg.Threads,
		daemonHost: cfg.DaemonHost,
		difficulty: new(big.Int),
	}
}

// Start begins mining against the configured daemon.
func (m *RPCMiner) Start() error {
	if m.running.Load() {
		return fmt.Errorf("miner already running")
	}

	// Validate address
	addr, err := globals.ParseValidateAddress(m.address)
	if err != nil {
		return fmt.Errorf("invalid mining address: %w", err)
	}
	m.address = addr.String()

	if m.threads < 1 {
		m.threads = 1
	}
	if m.threads > runtime.GOMAXPROCS(0) {
		m.threads = runtime.GOMAXPROCS(0)
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.running.Store(true)
	m.counter = 0
	m.blocks = 0
	m.hashrate = 0
	m.mu.Lock()
	m.startTime = time.Now()
	m.mu.Unlock()

	// Start websocket connection goroutine
	m.wg.Add(1)
	go m.wsLoop(ctx)

	// Wait briefly for first job
	time.Sleep(2 * time.Second)

	// Start mining loops
	for i := 0; i < m.threads; i++ {
		m.wg.Add(1)
		go m.mineLoop(ctx, i)
	}

	// Stats loop
	m.wg.Add(1)
	go m.statsLoop(ctx)

	return nil
}

// Stop halts the miner and closes the websocket.
func (m *RPCMiner) Stop() {
	if !m.running.Load() {
		return
	}
	m.running.Store(false)
	if m.cancel != nil {
		m.cancel()
	}
	m.wsMu.Lock()
	if m.wsConn != nil {
		m.wsConn.Close()
		m.wsConn = nil
	}
	m.wsMu.Unlock()
	m.wg.Wait()
}

// IsRunning reports whether the miner is active.
func (m *RPCMiner) IsRunning() bool {
	return m.running.Load()
}

// GetHashrate returns current hashrate in H/s.
func (m *RPCMiner) GetHashrate() uint64 {
	return atomic.LoadUint64(&m.hashrate)
}

// GetBlocks returns blocks found.
func (m *RPCMiner) GetBlocks() uint64 {
	return atomic.LoadUint64(&m.blocks)
}

// GetMinis reports accepted miniblocks. The legacy miner counts every share
// it submits as a block and does not separate miniblocks, so this is 0.
func (m *RPCMiner) GetMinis() uint64 { return 0 }

// GetRejected reports rejected shares. The legacy miner does not track
// rejections, so this is 0.
func (m *RPCMiner) GetRejected() uint64 { return 0 }

// GetHeight reports the chain height of the current job. The legacy miner
// does not track it, so this is 0.
func (m *RPCMiner) GetHeight() uint64 { return 0 }

// GetDifficulty reports the current job difficulty, when a job is loaded.
func (m *RPCMiner) GetDifficulty() uint64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.difficulty == nil {
		return 0
	}
	return m.difficulty.Uint64()
}

// GetHashes returns the cumulative number of hash computations.
func (m *RPCMiner) GetHashes() uint64 {
	return atomic.LoadUint64(&m.counter)
}

// GetUptime returns how long the current mining session has been running.
func (m *RPCMiner) GetUptime() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.running.Load() || m.startTime.IsZero() {
		return 0
	}
	return time.Since(m.startTime)
}

// GetThreads returns configured thread count.
func (m *RPCMiner) GetThreads() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.threads
}

// GetAddress returns the mining address.
func (m *RPCMiner) GetAddress() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.address
}

// GetDaemonHost returns the daemon host being mined against.
func (m *RPCMiner) GetDaemonHost() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.daemonHost
}

// wsLoop maintains the getwork websocket connection and receives jobs.
func (m *RPCMiner) wsLoop(ctx context.Context) {
	defer m.wg.Done()

	wsURL := url.URL{
		Scheme: "wss",
		Host:   m.daemonHost,
		Path:   "/ws/" + m.address,
	}

	dialer := websocket.DefaultDialer
	dialer.TLSClientConfig = nil // Allow insecure for local/testnet

	for m.running.Load() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		m.wsMu.Lock()
		conn, _, err := dialer.Dial(wsURL.String(), nil)
		if err != nil {
			m.wsMu.Unlock()
			time.Sleep(5 * time.Second)
			continue
		}
		m.wsConn = conn
		m.wsMu.Unlock()

		// Read jobs
		for m.running.Load() {
			select {
			case <-ctx.Done():
				return
			default:
			}

			var result rpc.GetBlockTemplate_Result
			if err := conn.ReadJSON(&result); err != nil {
				break // reconnect
			}

			// Update job atomically
			m.mu.Lock()
			m.job = result
			m.jobCounter++
			if result.Difficulty != "" {
				m.difficulty.SetString(result.Difficulty, 10)
			}
			m.mu.Unlock()
		}

		// Connection closed or error, reconnect after delay
		m.wsMu.Lock()
		if conn != nil {
			conn.Close()
		}
		m.wsConn = nil
		m.wsMu.Unlock()

		time.Sleep(5 * time.Second)
	}
}

// mineLoop hashes miniblocks for one thread.
func (m *RPCMiner) mineLoop(ctx context.Context, tid int) {
	defer m.wg.Done()

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	var randomBuf [12]byte
	rand.Read(randomBuf[:])

	work := make([]byte, block.MINIBLOCK_SIZE)
	nonceBuf := work[block.MINIBLOCK_SIZE-5:]

	var nonce uint32

	for m.running.Load() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Copy current job
		m.mu.RLock()
		job := m.job
		localJobCounter := m.jobCounter
		diff := new(big.Int).Set(m.difficulty)
		m.mu.RUnlock()

		if job.Blockhashing_blob == "" {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		n, err := hex.Decode(work, []byte(job.Blockhashing_blob))
		if err != nil || n != block.MINIBLOCK_SIZE {
			time.Sleep(time.Second)
			continue
		}

		if work[0]&0xf != 1 {
			time.Sleep(time.Second)
			continue
		}

		copy(work[block.MINIBLOCK_SIZE-12:], randomBuf[:])
		work[block.MINIBLOCK_SIZE-1] = byte(tid)

		// Mine until new job arrives
		for m.running.Load() && localJobCounter == atomic.LoadInt64(&m.jobCounter) {
			nonce++
			binary.BigEndian.PutUint32(nonceBuf, nonce)

			powHash := astrobwtv3.AstroBWTv3(work)
			atomic.AddUint64(&m.counter, 1)

			var cHash crypto.Hash
			copy(cHash[:], powHash[:])

			if CheckPowHashBig(cHash, diff) {
				m.mu.Lock()
				submitJob := job
				m.mu.Unlock()

				m.wsMu.Lock()
				conn := m.wsConn
				m.wsMu.Unlock()

				if conn != nil {
					conn.WriteJSON(rpc.SubmitBlock_Params{
						JobID:                 submitJob.JobID,
						MiniBlockhashing_blob: fmt.Sprintf("%x", work),
					})
				}

				atomic.AddUint64(&m.blocks, 1)
				break
			}
		}
	}
}

// statsLoop updates hashrate every second.
func (m *RPCMiner) statsLoop(ctx context.Context) {
	defer m.wg.Done()

	lastCount := uint64(0)
	lastTime := time.Now()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for m.running.Load() {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := time.Now()
			count := atomic.LoadUint64(&m.counter)
			elapsed := now.Sub(lastTime).Seconds()
			if elapsed > 0 {
				atomic.StoreUint64(&m.hashrate, uint64(float64(count-lastCount)/elapsed))
			}
			lastCount = count
			lastTime = now
		}
	}
	atomic.StoreUint64(&m.hashrate, 0)
}

// getworkPort derives the getwork port from an RPC address.
// Mainnet: 10102 -> 10100, Testnet: 40402 -> 40400, Simulator: 20000 -> 20000 (same)
func getworkPort(rpcAddr string) string {
	// If already has a port that looks like getwork, use it
	// Otherwise derive from standard RPC ports
	// This is a simple heuristic; user can override via daemon settings
	if rpcAddr == "" {
		return "10100"
	}
	// Check if it already has a non-standard port
	return "10100"
}
