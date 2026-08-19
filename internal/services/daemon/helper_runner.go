// Copyright 2017-2026 DERO Project. All rights reserved.

package daemon

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/deroproject/derohe/blockchain"
	deroconfig "github.com/deroproject/derohe/config"
	"github.com/deroproject/derohe/globals"
	"github.com/deroproject/derohe/p2p"
	derodpkg "github.com/moralpriest/derodpkg/cmd"

	appconfig "github.com/deroproject/dero-wallet-cli/internal/config"
	minerservice "github.com/deroproject/dero-wallet-cli/internal/services/miner"
	"github.com/deroproject/dero-wallet-cli/internal/wallet"
)

type helperState struct {
	mu       sync.RWMutex
	daemon   *derodpkg.Daemon
	chain    *blockchain.Blockchain
	logBuf   *LogBuffer
	miner    *minerservice.Miner
	settings appconfig.DaemonSettings
	running  bool
}

// RunHelper runs the daemon helper process. When serviceMode is true, the
// daemon is started immediately from the saved settings (used when the helper
// runs as an installed systemd service) instead of waiting for a start
// command on the socket.
func RunHelper(serviceMode bool) error {
	sockPath := helperSocketPath()
	_ = os.Remove(sockPath)
	if err := os.MkdirAll(filepath.Dir(sockPath), 0700); err != nil {
		return err
	}

	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		return err
	}
	defer func() {
		ln.Close()
		_ = os.Remove(sockPath)
	}()
	_ = os.Chmod(sockPath, 0600)

	state := &helperState{logBuf: NewLogBuffer(1000)}
	if serviceMode {
		settings := appconfig.GetDaemonSettings()
		if err := state.start(settings); err != nil {
			return fmt.Errorf("service auto-start failed: %w", err)
		}
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				continue
			}
			return err
		}
		go state.handleConn(conn)
	}
}

func helperSocketPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "derotui-daemon-helper.sock")
	}
	return filepath.Join(home, ".derotui", "daemon-helper.sock")
}

func (s *helperState) handleConn(conn net.Conn) {
	defer func() {
		if r := recover(); r != nil {
			_ = json.NewEncoder(conn).Encode(helperResponse{OK: false, Error: fmt.Sprintf("PANIC: %v", r)})
		}
		conn.Close()
	}()

	var req helperRequest
	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		_ = json.NewEncoder(conn).Encode(helperResponse{OK: false, Error: err.Error()})
		return
	}

	resp := s.handleRequest(req)
	_ = json.NewEncoder(conn).Encode(resp)
}

func (s *helperState) handleRequest(req helperRequest) helperResponse {
	switch req.Action {
	case "start":
		if req.Settings == nil {
			return helperResponse{OK: false, Error: "missing settings"}
		}
		if err := s.start(*req.Settings); err != nil {
			return helperResponse{OK: false, Error: err.Error()}
		}
		snap, info, logs := s.status()
		return helperResponse{OK: true, Snapshot: snap, Info: daemonInfoMap(info), Logs: logs, RPCBind: snap.RPCBind}
	case "stop":
		if err := s.stop(); err != nil {
			return helperResponse{OK: false, Error: err.Error()}
		}
		return helperResponse{OK: true}
	case "status":
		snap, info, logs := s.status()
		peerHeight, syncProgress, finalizing := s.syncState()
		return helperResponse{OK: true, Snapshot: snap, Info: daemonInfoMap(info), Logs: logs, RPCBind: snap.RPCBind, PeerHeight: peerHeight, SyncProgress: syncProgress, FinalizingBootstrap: finalizing, Miner: s.minerStatus()}
	case "miner_start":
		if err := s.startMiner(req.Address, req.Threads); err != nil {
			return helperResponse{OK: false, Error: err.Error()}
		}
		return helperResponse{OK: true, Miner: s.minerStatus()}
	case "miner_stop":
		s.stopMiner()
		return helperResponse{OK: true, Miner: s.minerStatus()}
	case "miner_status":
		return helperResponse{OK: true, Miner: s.minerStatus()}
	default:
		return helperResponse{OK: false, Error: "unknown action"}
	}
}

func (s *helperState) start(settings appconfig.DaemonSettings) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("PANIC during daemon start: %v", r)
		}
	}()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return nil
	}

	testnet := settings.Network == string(appconfig.NetworkTestnet)
	rpcBind, p2pBind, getworkBind := normalizedHelperBinds(settings)
	dataDir := strings.TrimSpace(settings.DataDir)
	if dataDir == "" {
		dataDir = globals.GetDataDirectory()
	}
	fastsync := settings.FastSync && !dataDirHasData(dataDir)

	params := map[string]interface{}{
		"--testnet":            testnet,
		"--debug":              settings.Debug,
		"--data-dir":           dataDir,
		"--rpc-bind":           rpcBind,
		"--p2p-bind":           p2pBind,
		"--getwork-bind":       getworkBind,
		"--fastsync":           fastsync,
		"--timeisinsync":       settings.TimeIsInSync,
		"--sync-node":          settings.SyncNode,
		"--offline":            false,
		"--node-tag":           settings.NodeTag,
		"--min-peers":          settings.MinPeers,
		"--max-peers":          settings.MaxPeers,
		"--add-priority-node":  append([]string(nil), settings.PriorityNodes...),
		"--add-exclusive-node": append([]string(nil), settings.ExclusiveNodes...),
	}
	if strings.TrimSpace(settings.SocksProxy) != "" {
		params["--socks-proxy"] = settings.SocksProxy
	} else {
		params["--socks-proxy"] = nil
	}
	if settings.IntegratorAddress != "" {
		params["--integrator-address"] = settings.IntegratorAddress
	}
	if settings.Network == string(appconfig.NetworkSimulator) {
		params["--simulator"] = true
	}

	daemon, err := derodpkg.NewDaemon(params)
	if err != nil {
		return fmt.Errorf("daemon create failed: %w", err)
	}
	os.Args = []string{"derod"}
	if err := daemon.Initialize(); err != nil {
		return fmt.Errorf("daemon initialize failed: %w", err)
	}
	if err := daemon.Start(); err != nil {
		return fmt.Errorf("daemon start failed: %w", err)
	}
	chain := daemon.Chain()

	s.daemon = daemon
	s.chain = chain
	s.settings = settings
	s.settings.RPCBind = rpcBind
	s.settings.P2PBind = p2pBind
	s.settings.GetWorkBind = getworkBind
	s.settings.DataDir = dataDir
	s.running = true
	return nil
}

func (s *helperState) stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return nil
	}
	if s.miner != nil {
		s.miner.Stop()
		s.miner = nil
	}
	if s.daemon != nil {
		if err := s.daemon.Stop(); err != nil {
			return err
		}
		s.daemon = nil
	}
	s.chain = nil
	s.running = false
	return nil
}

func (s *helperState) status() (Snapshot, wallet.DaemonInfo, []string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	logs := s.logBuf.Lines()
	if !s.running || s.chain == nil {
		return Snapshot{}, wallet.DaemonInfo{}, logs
	}
	height := s.chain.Get_Height()
	topoHeight := s.chain.Load_TOPO_HEIGHT()
	peerHeight, _ := p2p.Best_Peer_Height()
	stableHeight := s.chain.Get_Stable_Height()
	difficulty := s.chain.Get_Difficulty()
	peers := p2p.Peer_Count()
	info := wallet.DaemonInfo{
		Height:          nonNegativeHeight(height),
		StableHeight:    stableHeight,
		TopoHeight:      topoHeight,
		IsOnline:        true,
		IsHealthy:       true,
		IsSynced:        peerHeight > 0 && height > 0 && height >= peerHeight,
		IsSyncing:       peerHeight > 0 && height > 0 && height < peerHeight,
		IsBootstrapping: height > 0 && topoHeight != height,
		PeerHeight:      peerHeight,
		SyncProgress:    syncProgressRatio(height, peerHeight),
		Difficulty:      difficulty,
		IncomingPeers:   peers,
		OutgoingPeers:   0,
		KnownPeers:      peers,
		Version:         deroconfig.Version.String(),
		Uptime:          uint64(time.Since(globals.StartTime).Seconds()),
		TxPoolSize:      uint64(len(s.chain.Mempool.Mempool_List_TX())),
	}
	network := "Mainnet"
	if globals.IsSimulator() {
		network = "Simulator"
	} else if !globals.IsMainnet() {
		network = "Testnet"
	}
	snap := Snapshot{
		Running:     true,
		Managed:     true,
		Network:     network,
		DataDir:     s.settings.DataDir,
		RPCBind:     s.settings.RPCBind,
		GetWorkBind: s.settings.GetWorkBind,
		P2PBind:     s.settings.P2PBind,
	}
	return snap, info, logs
}

func (s *helperState) syncState() (int64, float64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.running || s.chain == nil {
		return 0, 0, false
	}
	height := s.chain.Get_Height()
	peerHeight, _ := p2p.Best_Peer_Height()
	finalizing := height < 0
	if peerHeight <= 0 || height <= 0 {
		return peerHeight, 0, finalizing
	}
	progress := syncProgressRatio(height, peerHeight)
	return peerHeight, progress, finalizing
}

func (s *helperState) startMiner(address string, threads int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running || s.chain == nil {
		return fmt.Errorf("embedded daemon is not running")
	}
	if s.miner == nil {
		s.miner = minerservice.NewMiner(s.chain)
	}
	return s.miner.Start(address, threads)
}

func (s *helperState) stopMiner() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.miner != nil {
		s.miner.Stop()
	}
}

func (s *helperState) minerStatus() helperMinerStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.miner == nil {
		return helperMinerStatus{}
	}
	return helperMinerStatus{
		Running:    s.miner.IsRunning(),
		Hashrate:   s.miner.GetHashrate(),
		Blocks:     s.miner.GetBlocks(),
		Height:     s.miner.GetHeight(),
		Difficulty: s.miner.GetDifficulty(),
		Hashes:     s.miner.GetHashes(),
		Uptime:     s.miner.GetUptime(),
		Threads:    s.miner.GetThreads(),
		Address:    s.miner.GetAddress(),
	}
}

func normalizedHelperBinds(settings appconfig.DaemonSettings) (string, string, string) {
	testnet := settings.Network == string(appconfig.NetworkTestnet)
	rpcBind := strings.TrimSpace(settings.RPCBind)
	p2pBind := strings.TrimSpace(settings.P2PBind)
	getworkBind := strings.TrimSpace(settings.GetWorkBind)
	if rpcBind == "" {
		if testnet {
			rpcBind = "127.0.0.1:" + strconv.Itoa(deroconfig.Testnet.RPC_Default_Port)
		} else {
			rpcBind = "127.0.0.1:" + strconv.Itoa(deroconfig.Mainnet.RPC_Default_Port)
		}
	}
	if p2pBind == "" {
		if testnet {
			p2pBind = "0.0.0.0:" + strconv.Itoa(deroconfig.Testnet.P2P_Default_Port)
		} else {
			p2pBind = "0.0.0.0:" + strconv.Itoa(deroconfig.Mainnet.P2P_Default_Port)
		}
	}
	if getworkBind == "" {
		if testnet {
			getworkBind = "0.0.0.0:" + strconv.Itoa(deroconfig.Testnet.GETWORK_Default_Port)
		} else {
			getworkBind = "0.0.0.0:" + strconv.Itoa(deroconfig.Mainnet.GETWORK_Default_Port)
		}
	}
	return rpcBind, p2pBind, getworkBind
}

func daemonInfoMap(info wallet.DaemonInfo) map[string]any {
	return map[string]any{
		"height":           info.Height,
		"stable_height":    info.StableHeight,
		"topo_height":      info.TopoHeight,
		"is_online":        info.IsOnline,
		"is_healthy":       info.IsHealthy,
		"is_synced":        info.IsSynced,
		"is_bootstrapping": info.IsBootstrapping,
		"difficulty":       info.Difficulty,
		"incoming_peers":   info.IncomingPeers,
		"outgoing_peers":   info.OutgoingPeers,
		"known_peers":      info.KnownPeers,
		"version":          info.Version,
		"uptime":           info.Uptime,
		"tx_pool_size":     info.TxPoolSize,
	}
}

func nonNegativeHeight(height int64) uint64 {
	if height <= 0 {
		return 0
	}
	return uint64(height)
}

// syncProgressRatio returns height/peerHeight as a percentage (0..100).
func syncProgressRatio(height, peerHeight int64) float64 {
	if peerHeight <= 0 || height <= 0 {
		return 0
	}
	progress := float64(height) / float64(peerHeight) * 100
	if progress > 100 {
		progress = 100
	}
	if progress < 0 {
		progress = 0
	}
	return progress
}
