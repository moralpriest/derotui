// Copyright 2017-2026 DERO Project. All rights reserved.

package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/deroproject/dero-wallet-cli/internal/config"
	daemonservice "github.com/deroproject/dero-wallet-cli/internal/services/daemon"
	systemdservice "github.com/deroproject/dero-wallet-cli/internal/services/systemd"
	"github.com/deroproject/dero-wallet-cli/internal/ui/pages"
	"github.com/deroproject/dero-wallet-cli/internal/wallet"
)

func (m *Model) startMinerCmd() tea.Cmd {
	return func() tea.Msg {
		if m.wallet == nil {
			return minerControlMsg{err: "open a wallet first to choose a mining address"}
		}
		if m.embeddedDaemon == nil || !m.embeddedDaemon.IsRunning() {
			return minerControlMsg{err: "start the embedded daemon before mining"}
		}

		address := m.wallet.GetInfo().Address
		if strings.TrimSpace(address) == "" {
			return minerControlMsg{err: "wallet address is not available yet"}
		}

		if err := m.embeddedDaemon.StartMiner(address, m.miner.Threads); err != nil {
			return minerControlMsg{err: err.Error()}
		}

		return minerControlMsg{}
	}
}

func (m *Model) stopMinerCmd() tea.Cmd {
	return func() tea.Msg {
		if m.embeddedDaemon != nil {
			_ = m.embeddedDaemon.StopMiner()
		}
		return minerControlMsg{}
	}
}

func (m *Model) minerStatsCmd() tea.Cmd {
	return func() tea.Msg {
		msg := minerStatsMsg{}
		if m.embeddedDaemon == nil || !m.embeddedDaemon.IsRunning() {
			msg.threads = m.miner.Threads
			if m.wallet != nil {
				msg.address = m.wallet.GetInfo().Address
			}
			msg.status = "Embedded daemon must be running before mining can start."
			return msg
		}

		miner := m.embeddedDaemon.MinerStatus()
		msg.running = miner.Running
		msg.hashrate = miner.Hashrate
		msg.blocks = miner.Blocks
		msg.threads = miner.Threads
		msg.address = miner.Address
		if msg.threads == 0 {
			msg.threads = m.miner.Threads
		}
		if msg.address == "" && m.wallet != nil {
			msg.address = m.wallet.GetInfo().Address
		}
		if m.wallet == nil && !msg.running {
			msg.status = "Open a wallet to mine to its address."
		} else if msg.running {
			msg.status = "Mining miniblocks with AstroBWTv3."
		} else {
			msg.status = "Adjust threads, then press S to start mining."
		}
		return msg
	}
}

func (m *Model) daemonTickCmd() tea.Cmd {
	if m.embeddedDaemon != nil && m.embeddedDaemon.IsRunning() {
		return m.embeddedDaemonStatusCmd()
	}
	return m.daemonManagerStatusCmd()
}

func (m *Model) embeddedDaemonStatusCmd() tea.Cmd {
	return func() tea.Msg {
		statusSnapshot, info, logs := m.embeddedDaemon.GetStatus()
		snap := daemonservice.Snapshot{
			Running:     statusSnapshot.Running,
			Managed:     statusSnapshot.Managed,
			RPCBind:     statusSnapshot.RPCBind,
			P2PBind:     statusSnapshot.P2PBind,
			DataDir:     statusSnapshot.DataDir,
			Network:     statusSnapshot.Network,
			LastError:   statusSnapshot.LastError,
		}
		return daemonManagerMsg{snapshot: snap, logs: logs, info: info, source: "Embedded"}
	}
}

func (m *Model) daemonManagerStatusCmd() tea.Cmd {
	return func() tea.Msg {
		snapshot := m.daemonManager.Snapshot()
		info := wallet.DaemonInfo{}
		errMsg := ""
		source := "Managed Local"
		if snapshot.Managed || snapshot.Running || strings.TrimSpace(snapshot.BinaryPath) != "" {
			rpcAddr := firstNonEmpty(snapshot.RPCBind, config.GetDaemonSettings().RPCBind)
			if rpcAddr != "" {
				info = wallet.GetDaemonInfo(context.Background(), rpcAddr)
				snapshot.RPCBind = rpcAddr
			}
			if info.IsOnline && !snapshot.Managed && snapshot.PID == 0 {
				if pid := daemonservice.FindPIDByAddress(snapshot.RPCBind); pid > 0 {
					snapshot.PID = pid
				}
			}
			if snapshot.Running && !info.IsOnline {
				errMsg = snapshot.LastError
			}
			if !snapshot.Running && !info.IsOnline && snapshot.Managed && snapshot.LastExit == "" {
				snapshot.Running = true
			}
		} else {
			service, _ := detectPreferredDerodService()
			if service.Exists {
				settings := config.GetDaemonSettings()
				settings = mergeServiceSettings(settings, service)
				rpcAddr, probe := detectSystemdRPC(settings, service)
				info = probe
				source = "System Daemon"
				snapshot = snapshotFromSystemd(settings, service, rpcAddr, info)
				logs := readSystemdLogLines(service)
				if len(logs) == 0 {
					logs = readExternalLogLines(settings)
				}
				return daemonManagerMsg{snapshot: snapshot, logs: logs, info: info, err: errMsg, source: source}
			}
			var detectedAddr string
			settings := config.GetDaemonSettings()
			candidates := []string{settings.RPCBind, wallet.DefaultMainnetDaemon, wallet.DefaultTestnetDaemon, wallet.DefaultSimulatorDaemon}
			seen := map[string]bool{}
			for _, candidate := range candidates {
				candidate = strings.TrimSpace(candidate)
				if candidate == "" || seen[candidate] {
					continue
				}
				seen[candidate] = true
				probe := wallet.GetDaemonInfo(context.Background(), candidate)
				if probe.IsOnline {
					detectedAddr = candidate
					info = probe
					source = "External Local"
					snapshot = snapshotFromExternal(settings, candidate, probe)
					break
				}
			}
			if detectedAddr == "" {
				source = "Not running"
			}
		}
		logs := m.daemonManager.Logs()
		if source == "External Local" && len(logs) == 0 {
			logs = readExternalLogLines(config.GetDaemonSettings())
		}
		return daemonManagerMsg{snapshot: snapshot, logs: logs, info: info, err: errMsg, source: source}
	}
}

func (m *Model) startLocalDaemonCmd() tea.Cmd {
	settings := config.GetDaemonSettings()
	return func() tea.Msg {
		currentNetwork := string(config.NetworkMainnet)
		if m.Opts.Simulator {
			currentNetwork = string(config.NetworkSimulator)
		} else if m.Opts.Testnet {
			currentNetwork = string(config.NetworkTestnet)
		}
		if settings.Network != "" && settings.Network != currentNetwork {
			return daemonManagerMsg{err: fmt.Sprintf("daemon network %s does not match app network %s; open Settings or switch network and start", settings.Network, currentNetwork)}
		}

		if settings.Mode == "embedded" || settings.Mode == "" {
			if err := m.embeddedDaemon.Start(settings); err != nil {
				return daemonManagerMsg{err: err.Error()}
			}
			statusSnapshot, info, logs := m.embeddedDaemon.GetStatus()
			snap := daemonservice.Snapshot{
				Running: true,
				Managed: true,
				RPCBind: statusSnapshot.RPCBind,
				P2PBind: statusSnapshot.P2PBind,
				DataDir: statusSnapshot.DataDir,
				Network: statusSnapshot.Network,
			}
			return daemonManagerMsg{snapshot: snap, logs: logs, info: info, source: "Embedded"}
		}

		wallet.InvalidateDaemonInfoCache(settings.RPCBind)
		service, _ := detectPreferredDerodService()
		if service.Exists {
			if err := systemdservice.Control(service.Scope, service.Unit, "start"); err != nil {
				return daemonManagerMsg{err: formatSystemdError(err)}
			}
			return m.daemonManagerStatusCmd()()
		}
		if strings.TrimSpace(settings.BinaryPath) == "" {
			return daemonManagerMsg{err: "daemon binary path is not configured"}
		}
		if err := m.daemonManager.Start(settings); err != nil {
			return daemonManagerMsg{snapshot: m.daemonManager.Snapshot(), logs: m.daemonManager.Logs(), err: err.Error()}
		}
		return daemonManagerMsg{snapshot: m.daemonManager.Snapshot(), logs: m.daemonManager.Logs()}
	}
}

func (m *Model) stopLocalDaemonCmd() tea.Cmd {
	return func() tea.Msg {
		settings := config.GetDaemonSettings()

		if m.embeddedDaemon != nil && m.embeddedDaemon.IsRunning() {
			if err := m.embeddedDaemon.Stop(); err != nil {
				return daemonManagerMsg{err: err.Error()}
			}
			return daemonManagerMsg{source: "Embedded"}
		}

		wallet.InvalidateDaemonInfoCache(settings.RPCBind)

		// 1. Try systemd service
		service, _ := detectPreferredDerodService()
		if service.Exists {
			err := systemdservice.Control(service.Scope, service.Unit, "stop")
			msg := m.daemonManagerStatusCmd()().(daemonManagerMsg)
			if err != nil {
				msg.err = formatSystemdError(err)
			}
			return msg
		}

		// 2. Try managed child process
		snap := m.daemonManager.Snapshot()
		if snap.Managed {
			err := m.daemonManager.Stop()
			msg := daemonManagerMsg{snapshot: m.daemonManager.Snapshot(), logs: m.daemonManager.Logs()}
			if err != nil {
				msg.err = err.Error()
			}
			return msg
		}

		// 3. Try stopping by PID from snapshot (externally started daemon)
		pid := m.daemonStatus.Snapshot.PID
		if pid > 0 {
			if err := daemonservice.StopByPID(pid); err != nil {
				msg := m.daemonManagerStatusCmd()().(daemonManagerMsg)
				msg.err = err.Error()
				return msg
			}
			return m.daemonManagerStatusCmd()()
		}

		return daemonManagerMsg{err: "no daemon process found to stop"}
	}
}

func (m *Model) restartLocalDaemonCmd() tea.Cmd {
	settings := config.GetDaemonSettings()
	return func() tea.Msg {
		if m.embeddedDaemon != nil && m.embeddedDaemon.IsRunning() {
			if err := m.embeddedDaemon.Stop(); err != nil {
				return daemonManagerMsg{err: err.Error()}
			}
			if err := m.embeddedDaemon.Start(settings); err != nil {
				return daemonManagerMsg{err: err.Error()}
			}
			statusSnapshot, info, logs := m.embeddedDaemon.GetStatus()
			snap := daemonservice.Snapshot{
				Running: true,
				Managed: true,
				RPCBind: statusSnapshot.RPCBind,
				P2PBind: statusSnapshot.P2PBind,
				DataDir: statusSnapshot.DataDir,
				Network: statusSnapshot.Network,
			}
			return daemonManagerMsg{snapshot: snap, logs: logs, info: info, source: "Embedded"}
		}

		wallet.InvalidateDaemonInfoCache(settings.RPCBind)

		// 1. Try systemd service
		service, _ := detectPreferredDerodService()
		if service.Exists {
			err := systemdservice.Control(service.Scope, service.Unit, "restart")
			msg := m.daemonManagerStatusCmd()().(daemonManagerMsg)
			if err != nil {
				msg.err = formatSystemdError(err)
			}
			return msg
		}

		// 2. Try managed child process
		snap := m.daemonManager.Snapshot()
		if snap.Managed {
			err := m.daemonManager.Restart(settings)
			msg := daemonManagerMsg{snapshot: m.daemonManager.Snapshot(), logs: m.daemonManager.Logs()}
			if err != nil {
				msg.err = err.Error()
			}
			return msg
		}

		// 3. Try stopping by PID, then start a new managed process
		pid := m.daemonStatus.Snapshot.PID
		if pid > 0 {
			if err := daemonservice.StopByPID(pid); err != nil {
				return daemonManagerMsg{err: err.Error()}
			}
			time.Sleep(500 * time.Millisecond)
		}

		if err := m.daemonManager.Start(settings); err != nil {
			return daemonManagerMsg{snapshot: m.daemonManager.Snapshot(), logs: m.daemonManager.Logs(), err: err.Error()}
		}
		return daemonManagerMsg{snapshot: m.daemonManager.Snapshot(), logs: m.daemonManager.Logs()}
	}
}

const daemonGracePeriod = 30 * time.Second

func (m *Model) applyDaemonManagerMsg(msg daemonManagerMsg) {
	settings := config.GetDaemonSettings()
	source := sourceLabel(msg.source)
	if source != "Embedded" && (msg.snapshot.Managed || msg.snapshot.Running) {
		source = "Managed Local"
	}
	if settings.Mode == "embedded" && source == "Unknown" {
		source = "Embedded"
	}
	rpcBind := firstNonEmpty(msg.snapshot.RPCBind, settings.RPCBind)
	binaryPath := firstNonEmpty(msg.snapshot.BinaryPath, settings.BinaryPath)
	dataDir := firstNonEmpty(msg.snapshot.DataDir, settings.DataDir)
	p2pBind := firstNonEmpty(msg.snapshot.P2PBind, settings.P2PBind)
	running := msg.snapshot.Running
	managed := msg.snapshot.Managed

	inGrace := !m.daemonManagedSince.IsZero() && time.Since(m.daemonManagedSince) < daemonGracePeriod

	if source == "Embedded" {
		m.daemonManagedSince = time.Time{}
	} else if msg.info.IsOnline && msg.info.IsHealthy {
		source = "Managed Local"
		running = true
		managed = true
		m.daemonManagedSince = time.Time{}
	} else if inGrace {
		if settings.Mode == "embedded" {
			source = "Embedded"
		} else {
			source = "Managed Local"
		}
		running = true
		managed = true
	} else {
		m.daemonManagedSince = time.Time{}
	}

	baseline := pages.DaemonStatusSnapshot{
		Running:           running,
		Managed:           managed,
		BinaryReady:       source == "Embedded" || settings.Mode == "embedded" || fileExists(binaryPath),
		Source:            source,
		PID:               msg.snapshot.PID,
		Network:           titleDaemonNetwork(firstNonEmpty(msg.snapshot.Network, settings.Network)),
		BinaryPath:        binaryPath,
		DataDir:           dataDir,
		RPCBind:           rpcBind,
		P2PBind:           p2pBind,
		IntegratorAddress: settings.IntegratorAddress,
		LastError:         firstNonEmpty(msg.err, msg.snapshot.LastError, m.lastEmbeddedError),
		IsOnline:          msg.info.IsOnline,
		IsHealthy:         msg.info.IsHealthy,
		IsSynced:          msg.info.IsSynced,
		IsBootstrapping:   msg.info.IsBootstrapping,
		BlockHeight:       msg.info.Height,
		StableHeight:      msg.info.StableHeight,
		TopoHeight:        msg.info.TopoHeight,
		Version:           msg.info.Version,
		Difficulty:        msg.info.Difficulty,
		AvgBlockTime:      msg.info.AvgBlockTime,
		IncomingPeers:     msg.info.IncomingPeers,
		OutgoingPeers:     msg.info.OutgoingPeers,
		KnownPeers:        msg.info.KnownPeers,
		Uptime:            msg.info.Uptime,
		TxPoolSize:        msg.info.TxPoolSize,
		Hashrate1hr:       msg.info.Hashrate1hr,
		Hashrate1d:        msg.info.Hashrate1d,
	}

	m.daemonStatus.SetSnapshot(baseline)
	if source == "Embedded" && len(msg.logs) == 0 && m.embeddedDaemon != nil {
		msg.logs = m.embeddedDaemon.Logs()
	}
	m.daemonLogs.SetLines(msg.logs)
	m.daemonLogs.SetSource(logSourceLabel(msg.source))
	if source == "Embedded" && len(msg.logs) == 0 {
		m.daemonLogs.SetEmptyHint("No embedded helper logs yet. If startup failed early, try again and reopen logs.")
	} else {
		m.daemonLogs.SetEmptyHint("")
	}
	m.dashboard.SetDaemonBootstrapping(msg.info.IsBootstrapping)

	if msg.snapshot.RPCBind != "" {
		m.welcome.SetDaemonStatus(msg.info.IsOnline, msg.info.IsSynced, msg.info.IsBootstrapping, msg.info.IsHealthy, titleDaemonNetwork(msg.snapshot.Network), msg.snapshot.RPCBind, msg.info.Height)
	}
}

func fileExists(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

func sourceLabel(source string) string {
	if strings.TrimSpace(source) == "" || source == "Not running" {
		return "Planned Local"
	}
	return source
}

func titleDaemonNetwork(v string) string {
	v = strings.TrimSpace(strings.ToLower(v))
	switch v {
	case string(config.NetworkTestnet):
		return "Testnet"
	case string(config.NetworkSimulator):
		return "Simulator"
	default:
		return "Mainnet"
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func snapshotFromExternal(settings config.DaemonSettings, address string, info wallet.DaemonInfo) daemonservice.Snapshot {
	pid := daemonservice.FindPIDByAddress(address)
	return daemonservice.Snapshot{
		Running:     info.IsOnline,
		Managed:     false,
		PID:         pid,
		Network:     strings.ToLower(info.Network),
		BinaryPath:  settings.BinaryPath,
		DataDir:     settings.DataDir,
		RPCBind:     address,
		P2PBind:     settings.P2PBind,
		GetWorkBind: settings.GetWorkBind,
	}
}

func detectSystemdRPC(settings config.DaemonSettings, service systemdservice.ServiceStatus) (string, wallet.DaemonInfo) {
	execRPC := inferFlagValue(service.ExecStart, "rpc-bind")
	candidates := []string{
		execRPC,
		wallet.DefaultMainnetDaemon,
		wallet.DefaultTestnetDaemon,
		wallet.DefaultSimulatorDaemon,
		strings.TrimSpace(settings.RPCBind),
	}
	seen := map[string]bool{}
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" || seen[candidate] {
			continue
		}
		seen[candidate] = true
		info := wallet.GetDaemonInfo(context.Background(), candidate)
		if info.IsOnline {
			return candidate, info
		}
	}
	return firstNonEmpty(strings.TrimSpace(settings.RPCBind), wallet.DefaultMainnetDaemon), wallet.DaemonInfo{}
}

func detectPreferredDerodService() (systemdservice.ServiceStatus, error) {
	result, err := systemdservice.DetectAll("derod")
	if err != nil {
		return systemdservice.ServiceStatus{}, err
	}
	if result.User.Exists && !result.System.Exists {
		return result.User, nil
	}
	if result.System.Exists && !result.User.Exists {
		return result.System, nil
	}
	if result.User.Exists && result.System.Exists {
		if result.User.Active {
			return result.User, nil
		}
		if result.System.Active {
			return result.System, nil
		}
		return result.User, nil
	}
	return systemdservice.ServiceStatus{Unit: "derod", Scope: systemdservice.ScopeNone}, nil
}

func hasInstalledDerodService() bool {
	service, err := detectPreferredDerodService()
	if err != nil {
		return false
	}
	return service.Exists
}

func mergeServiceSettings(settings config.DaemonSettings, service systemdservice.ServiceStatus) config.DaemonSettings {
	if binary := inferBinaryPath(service.ExecStart); binary != "" {
		settings.BinaryPath = binary
	}
	if rpc := inferFlagValue(service.ExecStart, "rpc-bind"); rpc != "" {
		settings.RPCBind = rpc
	}
	if p2p := inferFlagValue(service.ExecStart, "p2p-bind"); p2p != "" {
		settings.P2PBind = p2p
	}
	if getwork := inferFlagValue(service.ExecStart, "getwork-bind"); getwork != "" {
		settings.GetWorkBind = getwork
	}
	if dataDir := inferFlagValue(service.ExecStart, "data-dir"); dataDir != "" {
		settings.DataDir = dataDir
	}
	if tag := inferFlagValue(service.ExecStart, "node-tag"); tag != "" {
		settings.NodeTag = tag
	}
	if integrator := inferFlagValue(service.ExecStart, "integrator-address"); integrator != "" {
		settings.IntegratorAddress = integrator
	}
	if strings.Contains(service.ExecStart, "--testnet") {
		settings.Network = string(config.NetworkTestnet)
	}
	if strings.Contains(service.ExecStart, "--simulator") {
		settings.Network = string(config.NetworkSimulator)
	}
	return settings
}

func inferBinaryPath(execStart string) string {
	execStart = strings.TrimSpace(execStart)
	if execStart == "" {
		return ""
	}
	if idx := strings.Index(execStart, " ; argv[]="); idx >= 0 {
		execStart = execStart[idx+9:]
	}
	fields := strings.Fields(execStart)
	if len(fields) == 0 {
		return ""
	}
	return strings.Trim(fields[0], "{}")
}

func inferFlagValue(execStart, flag string) string {
	pattern := regexp.MustCompile(`--` + regexp.QuoteMeta(flag) + `=([^\s;]+)`)
	match := pattern.FindStringSubmatch(execStart)
	if len(match) < 2 {
		return ""
	}
	return strings.Trim(match[1], `"}`)
}

func snapshotFromSystemd(settings config.DaemonSettings, service systemdservice.ServiceStatus, address string, info wallet.DaemonInfo) daemonservice.Snapshot {
	network := strings.ToLower(info.Network)
	if network == "" {
		network = settings.Network
	}
	serviceState := strings.TrimSpace(service.SubState)
	if serviceState == "" {
		if service.Active {
			serviceState = "active"
		} else {
			serviceState = "inactive"
		}
	}
	return daemonservice.Snapshot{
		Running:     service.Active,
		Managed:     false,
		Network:     network,
		BinaryPath:  settings.BinaryPath,
		DataDir:     settings.DataDir,
		RPCBind:     address,
		P2PBind:     settings.P2PBind,
		GetWorkBind: settings.GetWorkBind,
		LastExit:    serviceState,
	}
}

func readSystemdLogLines(service systemdservice.ServiceStatus) []string {
	lines, err := systemdservice.Journal(service.Scope, service.Unit, 200)
	if err != nil {
		return []string{"Failed to read journal: " + err.Error()}
	}
	return lines
}

func formatSystemdError(err error) string {
	msg := err.Error()
	lower := strings.ToLower(msg)
	if strings.Contains(lower, "access denied") || strings.Contains(lower, "authentication is required") || strings.Contains(lower, "interactive authentication") || strings.Contains(lower, "permission denied") {
		return msg + ". Hint: this derod service may require elevated privileges or should be run as a user service."
	}
	return msg
}

func logSourceLabel(source string) string {
	switch source {
	case "System Daemon":
		return "journalctl"
	case "Managed Local":
		return "session"
	case "Embedded":
		return "helper"
	case "External Local":
		return "file"
	default:
		return "unknown"
	}
}

func readExternalLogLines(settings config.DaemonSettings) []string {
	logDir := strings.TrimSpace(settings.LogDir)
	if logDir == "" {
		return nil
	}
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return []string{"Failed to read daemon log dir: " + err.Error()}
	}
	latestPath := ""
	latestMod := int64(0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.ToLower(entry.Name())
		if !strings.Contains(name, "derod") && !strings.HasSuffix(name, ".log") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		mod := info.ModTime().UnixNano()
		if mod > latestMod {
			latestMod = mod
			latestPath = filepath.Join(logDir, entry.Name())
		}
	}
	if latestPath == "" {
		return nil
	}
	data, err := os.ReadFile(latestPath)
	if err != nil {
		return []string{"Failed to read daemon log file: " + err.Error()}
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			filtered = append(filtered, line)
		}
	}
	if len(filtered) > 200 {
		filtered = filtered[len(filtered)-200:]
	}
	return filtered
}
