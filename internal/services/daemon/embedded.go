// Copyright 2017-2026 DERO Project. All rights reserved.

package daemon

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"

	derolog "github.com/deroproject/dero-wallet-cli/internal/log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	appconfig "github.com/deroproject/dero-wallet-cli/internal/config"
	"github.com/deroproject/dero-wallet-cli/internal/ui/pages"
	"github.com/deroproject/dero-wallet-cli/internal/wallet"
)

type EmbeddedDaemon struct {
	mu        sync.RWMutex
	cmd       *exec.Cmd
	logBuf    *LogBuffer
	settings  appconfig.DaemonSettings
	miner     helperMinerStatus
	refresher sync.Once
}

func NewEmbeddedDaemon(logBuf *LogBuffer) *EmbeddedDaemon {
	if logBuf == nil {
		logBuf = NewLogBuffer(1000)
	}
	return &EmbeddedDaemon{logBuf: logBuf}
}

func (d *EmbeddedDaemon) Start(settings appconfig.DaemonSettings) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	cleanupHelperArtifacts()
	if d.isProcessRunningLocked() {
		return nil
	}
	bin, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(bin, "daemon-helper")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	d.cmd = cmd
	d.settings = settings
	go d.capture(stdout)
	go d.capture(stderr)
	go cmd.Wait()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(helperSocketPath()); err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	resp, err := d.request(helperRequest{Action: "start", Settings: &settings})
	if err != nil {
		return fmt.Errorf("embedded helper started but did not accept commands: %w", err)
	}
	if !resp.OK {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

func cleanupHelperArtifacts() {
	if path := helperSocketPath(); path != "" {
		if _, err := net.Dial("unix", path); err != nil {
			_ = os.Remove(path)
		}
	}
	if out, err := exec.Command("pkill", "-f", "derotui daemon-helper").CombinedOutput(); err == nil || len(out) > 0 {
		_ = out
	}
}

func (d *EmbeddedDaemon) Stop() error {
	resp, err := d.request(helperRequest{Action: "stop"})
	if err == nil && !resp.OK {
		return fmt.Errorf("%s", resp.Error)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.cmd != nil && d.cmd.Process != nil {
		_ = d.cmd.Process.Kill()
	}
	d.cmd = nil
	_ = os.Remove(helperSocketPath())
	return nil
}

func (d *EmbeddedDaemon) IsRunning() bool {
	resp, err := d.request(helperRequest{Action: "status"})
	return err == nil && resp.OK && resp.Snapshot.Running
}

func (d *EmbeddedDaemon) StartMiner(address string, threads int) error {
	resp, err := d.request(helperRequest{Action: "miner_start", Address: address, Threads: threads})
	if err != nil {
		return err
	}
	if !resp.OK {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

func (d *EmbeddedDaemon) StopMiner() error {
	resp, err := d.request(helperRequest{Action: "miner_stop"})
	if err != nil {
		return err
	}
	if !resp.OK {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

func (d *EmbeddedDaemon) MinerStatus() helperMinerStatus {
	d.refresher.Do(func() { go d.minerRefresher() })
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.miner
}

// minerRefresher polls the helper's miner status in the background so that
// Update-path callers never block on a socket RPC (a stalled helper must not
// freeze the UI).
func (d *EmbeddedDaemon) minerRefresher() {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for range t.C {
		st := helperMinerStatus{}
		if resp, err := d.request(helperRequest{Action: "miner_status"}); err == nil && resp.OK {
			st = resp.Miner
		}
		d.mu.Lock()
		d.miner = st
		d.mu.Unlock()
	}
}

func (d *EmbeddedDaemon) GetStatus() (pages.DaemonStatusSnapshot, wallet.DaemonInfo, []string) {
	resp, err := d.request(helperRequest{Action: "status"})
	if err != nil || !resp.OK {
		return pages.DaemonStatusSnapshot{}, wallet.DaemonInfo{}, d.Logs()
	}
	info := daemonInfoFromMap(resp.Info)
	snap := pages.DaemonStatusSnapshot{
		Running:               resp.Snapshot.Running,
		Managed:               resp.Snapshot.Managed,
		BinaryReady:           true,
		Source:                "Embedded",
		Network:               resp.Snapshot.Network,
		DataDir:               resp.Snapshot.DataDir,
		RPCBind:               resp.Snapshot.RPCBind,
		P2PBind:               resp.Snapshot.P2PBind,
		IntegratorAddress:     d.settings.IntegratorAddress,
		IsOnline:              info.IsOnline,
		IsHealthy:             info.IsHealthy,
		IsSynced:              info.IsSynced,
		IsBootstrapping:       info.IsBootstrapping || resp.BootstrapStep != 0 || resp.FinalizingBootstrap,
		IsFinalizingBootstrap: resp.FinalizingBootstrap,
		BlockHeight:           info.Height,
		StableHeight:          info.StableHeight,
		TopoHeight:            info.TopoHeight,
		PeerHeight:            resp.PeerHeight,
		BootstrapProgress:     resp.BootstrapProgress,
		SyncProgress:          resp.SyncProgress,
		Difficulty:            info.Difficulty,
		IncomingPeers:         resp.IncomingPeers,
		OutgoingPeers:         resp.OutgoingPeers,
		KnownPeers:            resp.KnownPeers,
		Version:               info.Version,
		Uptime:                info.Uptime,
		TxPoolSize:            info.TxPoolSize,
	}
	logs := resp.Logs
	if len(logs) == 0 {
		logs = d.Logs()
	}
	return snap, info, logs
}

func (d *EmbeddedDaemon) Logs() []string {
	lines := []string{}
	if d.logBuf != nil {
		lines = d.logBuf.Lines()
	}
	if len(lines) > 0 {
		return lines
	}
	return readHelperLogFile(d.settings.Network)
}

func (d *EmbeddedDaemon) GetRPCBind() string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.settings.RPCBind
}

func (d *EmbeddedDaemon) request(req helperRequest) (helperResponse, error) {
	var resp helperResponse
	conn, err := net.Dial("unix", helperSocketPath())
	if err != nil {
		return helperResponse{}, err
	}
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	if err != nil {
		return resp, err
	}
	defer conn.Close()
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return resp, err
	}
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return resp, err
	}
	return resp, nil
}

func (d *EmbeddedDaemon) capture(r io.ReadCloser) {
	defer r.Close()
	buf := make([]byte, 4096)
	var leftover string
	for {
		n, err := r.Read(buf)
		if n > 0 {
			chunk := leftover + string(buf[:n])
			leftover = ""
			for {
				i := strings.IndexByte(chunk, '\n')
				if i < 0 {
					leftover = chunk
					break
				}
				line := strings.TrimRight(chunk[:i], "\r")
				chunk = chunk[i+1:]
				if strings.TrimSpace(line) == "" {
					continue
				}
				if d.logBuf != nil {
					fmt.Fprintln(d.logBuf, line)
				}
				derolog.Info("daemon", "log", line)
			}
		}
		if err != nil {
			if strings.TrimSpace(leftover) != "" {
				if d.logBuf != nil {
					fmt.Fprintln(d.logBuf, leftover)
				}
				derolog.Info("daemon", "log", leftover)
			}
			return
		}
	}
}

func (d *EmbeddedDaemon) isProcessRunningLocked() bool {
	return d.cmd != nil && d.cmd.Process != nil
}

func daemonInfoFromMap(m map[string]any) wallet.DaemonInfo {
	return wallet.DaemonInfo{
		Height:          uint64Value(m["height"]),
		StableHeight:    int64Value(m["stable_height"]),
		TopoHeight:      int64Value(m["topo_height"]),
		IsOnline:        boolValue(m["is_online"]),
		IsHealthy:       boolValue(m["is_healthy"]),
		IsSynced:        boolValue(m["is_synced"]),
		IsBootstrapping: boolValue(m["is_bootstrapping"]),
		Difficulty:      uint64Value(m["difficulty"]),
		IncomingPeers:   uint64Value(m["incoming_peers"]),
		OutgoingPeers:   uint64Value(m["outgoing_peers"]),
		KnownPeers:      uint64Value(m["known_peers"]),
		Version:         stringValue(m["version"]),
		Uptime:          uint64Value(m["uptime"]),
		TxPoolSize:      uint64Value(m["tx_pool_size"]),
	}
}

func readHelperLogFile(network string) []string {
	path := helperLogFilePath(network)
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()
	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if len(lines) > 200 {
		lines = lines[len(lines)-200:]
	}
	return lines
}

// helperLogFilePath returns the fallback log file for daemon-helper output,
// kept in derotui's log dir so it is readable regardless of where the binary
// is installed.
func helperLogFilePath(network string) string {
	network = strings.TrimSpace(network)
	if network == "" {
		network = "mainnet"
	}
	return filepath.Join(derolog.LogDir(), "daemon-"+network+".log")
}

func boolValue(v any) bool     { b, _ := v.(bool); return b }
func stringValue(v any) string { s, _ := v.(string); return s }
func intValue(v any) int       { return int(uint64Value(v)) }
func int64Value(v any) int64   { return int64(uint64Value(v)) }
func uint64Value(v any) uint64 {
	switch n := v.(type) {
	case float64:
		return uint64(n)
	case int:
		return uint64(n)
	case int64:
		return uint64(n)
	case uint64:
		return n
	default:
		return 0
	}
}

func helperSocketDir() string {
	return filepath.Dir(helperSocketPath())
}
