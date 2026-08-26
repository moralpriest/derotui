// Copyright 2017-2026 DERO Project. All rights reserved.

package miner

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/deroproject/derohe/astrobwt/astrobwtv3"
	"github.com/deroproject/derohe/block"
	"github.com/deroproject/derohe/blockchain"
	"github.com/deroproject/derohe/globals"
	"github.com/deroproject/derohe/rpc"
)

type Miner struct {
	mu       sync.RWMutex
	chain    *blockchain.Blockchain
	running  atomic.Bool
	threads  int
	address  rpc.Address
	hashrate uint64
	blocks   uint64
	counter  uint64
	started  time.Time
	cancel   chan struct{}
}

func NewMiner(chain *blockchain.Blockchain) *Miner {
	return &Miner{chain: chain}
}

func (m *Miner) Start(address string, threads int) error {
	addr, err := globals.ParseValidateAddress(address)
	if err != nil {
		return fmt.Errorf("invalid mining address: %w", err)
	}

	if threads < 1 {
		threads = 1
	}
	if threads > runtime.GOMAXPROCS(0) {
		threads = runtime.GOMAXPROCS(0)
	}

	m.mu.Lock()
	m.address = *addr
	m.threads = threads
	m.cancel = make(chan struct{})
	m.counter = 0
	m.blocks = 0
	m.hashrate = 0
	m.started = time.Now()
	m.mu.Unlock()

	m.running.Store(true)

	for i := 0; i < threads; i++ {
		go m.mineLoop(i)
	}
	go m.statsLoop()

	return nil
}

func (m *Miner) Stop() {
	m.running.Store(false)
	m.mu.Lock()
	if m.cancel != nil {
		select {
		case <-m.cancel:
		default:
			close(m.cancel)
		}
	}
	m.mu.Unlock()
}

func (m *Miner) IsRunning() bool {
	return m.running.Load()
}

func (m *Miner) GetHashrate() uint64 {
	return atomic.LoadUint64(&m.hashrate)
}

func (m *Miner) GetBlocks() uint64 {
	return atomic.LoadUint64(&m.blocks)
}

// GetHashes returns the cumulative number of hash computations.
func (m *Miner) GetHashes() uint64 {
	return atomic.LoadUint64(&m.counter)
}

// GetUptime returns how long the current mining session has been running.
func (m *Miner) GetUptime() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.running.Load() || m.started.IsZero() {
		return 0
	}
	return time.Since(m.started)
}

// GetHeight returns the current chain height.
func (m *Miner) GetHeight() uint64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.chain == nil {
		return 0
	}
	h := m.chain.Get_Height()
	if h < 0 {
		return 0
	}
	return uint64(h)
}

// GetDifficulty returns the current chain difficulty.
func (m *Miner) GetDifficulty() uint64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.chain == nil {
		return 0
	}
	return m.chain.Get_Difficulty()
}

func (m *Miner) GetThreads() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.threads
}

func (m *Miner) GetAddress() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.address.String()
}

func (m *Miner) mineLoop(tid int) {
	runtime.LockOSThread()
	threadaffinity()
	defer runtime.UnlockOSThread()

	var randomBuf [12]byte
	rand.Read(randomBuf[:])

	for m.running.Load() {
		bl, _, miniblockBlob, _, err := m.chain.Create_new_block_template_mining(m.address)
		if err != nil {
			time.Sleep(time.Second)
			continue
		}

		work, err := hex.DecodeString(miniblockBlob)
		if err != nil {
			time.Sleep(time.Second)
			continue
		}

		if len(work) < block.MINIBLOCK_SIZE {
			time.Sleep(time.Second)
			continue
		}

		diff := m.chain.Get_Difficulty_At_Tips(bl.Tips)
		nonceBuf := work[block.MINIBLOCK_SIZE-5:]

		copy(work[block.MINIBLOCK_SIZE-12:], randomBuf[:])
		work[block.MINIBLOCK_SIZE-1] = byte(tid)

		nonce := uint32(0)
		for m.running.Load() {
			nonce++
			binary.BigEndian.PutUint32(nonceBuf, nonce)

			powHash := astrobwtv3.AstroBWTv3(work)
			atomic.AddUint64(&m.counter, 1)

			if CheckPowHashBig(powHash, diff) {
				_, _, _, err := m.chain.Accept_new_block(bl.Timestamp, work)
				if err == nil {
					atomic.AddUint64(&m.blocks, 1)
				}
				break
			}
		}
	}
}

func (m *Miner) statsLoop() {
	lastCount := uint64(0)
	lastTime := time.Now()

	for m.running.Load() {
		time.Sleep(time.Second)
		now := time.Now()
		count := atomic.LoadUint64(&m.counter)
		elapsed := now.Sub(lastTime).Seconds()
		if elapsed > 0 {
			atomic.StoreUint64(&m.hashrate, uint64(float64(count-lastCount)/elapsed))
		}
		lastCount = count
		lastTime = now
	}
	atomic.StoreUint64(&m.hashrate, 0)
}
