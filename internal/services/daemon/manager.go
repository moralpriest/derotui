// Copyright 2017-2026 DERO Project. All rights reserved.

package daemon

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/deroproject/dero-wallet-cli/internal/config"
)

const maxLogLines = 500

// Manager manages a local derod child process.
type Manager struct {
	mu       sync.RWMutex
	cmd      *exec.Cmd
	logs     []string
	snapshot Snapshot
}

// NewManager creates a new daemon manager.
func NewManager() *Manager {
	return &Manager{}
}

// Start launches derod using saved settings.
func (m *Manager) Start(settings config.DaemonSettings) error {
	binaryPath, err := ValidateBinaryPath(settings.BinaryPath)
	if err != nil {
		m.setError(err.Error())
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cmd != nil && m.snapshot.Running {
		return fmt.Errorf("daemon is already running")
	}
	if err := os.MkdirAll(settings.DataDir, 0700); err != nil {
		m.snapshot.LastError = err.Error()
		return err
	}
	args := BuildArgs(settings)
	cmd := exec.Command(binaryPath, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		m.snapshot.LastError = err.Error()
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		m.snapshot.LastError = err.Error()
		return err
	}
	if err := cmd.Start(); err != nil {
		m.snapshot.LastError = err.Error()
		return err
	}
	m.cmd = cmd
	m.snapshot = Snapshot{
		Running:     true,
		Managed:     true,
		PID:         cmd.Process.Pid,
		StartedAt:   time.Now(),
		BinaryPath:  binaryPath,
		DataDir:     settings.DataDir,
		RPCBind:     settings.RPCBind,
		P2PBind:     settings.P2PBind,
		GetWorkBind: settings.GetWorkBind,
		Network:     settings.Network,
		LaunchArgs:  append([]string(nil), args...),
	}
	m.appendLogLocked("derod started")
	go m.capture(stdout)
	go m.capture(stderr)
	go m.wait()
	return nil
}

// Stop stops the managed daemon if one is running.
func (m *Manager) Stop() error {
	m.mu.RLock()
	cmd := m.cmd
	m.mu.RUnlock()
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	if err := cmd.Process.Kill(); err != nil {
		return err
	}
	m.appendLog("derod stop requested")
	return nil
}

// StopByPID stops a daemon process by PID. It verifies the process is a derod
// binary before sending SIGTERM.
func StopByPID(pid int) error {
	if pid <= 0 {
		return fmt.Errorf("no daemon PID to stop")
	}
	cmdline, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		return fmt.Errorf("cannot read process info for PID %d: %w", pid, err)
	}
	name := string(cmdline)
	if !strings.Contains(strings.ToLower(name), "derod") {
		return fmt.Errorf("PID %d is not a derod process", pid)
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("cannot find process PID %d: %w", pid, err)
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("cannot stop PID %d: %w", pid, err)
	}
	return nil
}

// FindPIDByAddress finds the PID of the process listening on the given address.
// Returns 0 if no process is found.
func FindPIDByAddress(addr string) int {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return 0
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return 0
	}
	ip4 := ip.To4()

	// Read /proc/net/tcp for IPv4, /proc/net/tcp6 for IPv6
	var files []string
	if ip4 != nil {
		files = []string{"/proc/net/tcp"}
	} else {
		files = []string{"/proc/net/tcp6"}
	}

	portNum, err := strconv.ParseUint(port, 10, 16)
	if err != nil {
		return 0
	}

	var targetHex string
	if ip4 != nil {
		targetHex = fmt.Sprintf("%02X%02X%02X%02X:%04X", ip4[3], ip4[2], ip4[1], ip4[0], portNum)
		// Also check 0.0.0.0 binding
		anyHex := fmt.Sprintf("00000000:%04X", portNum)
		targetHex = targetHex + "|" + anyHex
	} else {
		// IPv6 hex representation in /proc/net/tcp6 is mixed endian per 4-byte group
		ip6 := ip.To16()
		var hexParts []string
		for i := 0; i < 16; i += 4 {
			hexParts = append(hexParts, fmt.Sprintf("%02X%02X%02X%02X", ip6[i+3], ip6[i+2], ip6[i+1], ip6[i]))
		}
		targetHex = strings.Join(hexParts, "") + fmt.Sprintf(":%04X", portNum)
	}

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		lines := strings.Split(string(data), "\n")
		for _, line := range lines[1:] { // skip header
			fields := strings.Fields(line)
			if len(fields) < 10 {
				continue
			}
			localAddr := fields[1]
			if strings.EqualFold(localAddr, targetHex) {
				inode := fields[9]
				if pid := findPIDByInode(inode); pid > 0 {
					return pid
				}
			}
		}
	}

	// Fallback: check 0.0.0.0 or :: binding for IPv4
	if ip4 != nil {
		anyHex := fmt.Sprintf("00000000:%04X", portNum)
		data, err := os.ReadFile("/proc/net/tcp")
		if err == nil {
			lines := strings.Split(string(data), "\n")
			for _, line := range lines[1:] {
				fields := strings.Fields(line)
				if len(fields) < 10 {
					continue
				}
				if strings.EqualFold(fields[1], anyHex) {
					inode := fields[9]
					if pid := findPIDByInode(inode); pid > 0 {
						return pid
					}
				}
			}
		}
	}

	return 0
}

func findPIDByInode(inode string) int {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		fdDir := fmt.Sprintf("/proc/%d/fd", pid)
		fds, err := os.ReadDir(fdDir)
		if err != nil {
			continue
		}
		for _, fd := range fds {
			link, err := os.Readlink(fmt.Sprintf("%s/%s", fdDir, fd.Name()))
			if err != nil {
				continue
			}
			if strings.Contains(link, "socket:["+inode+"]") {
				return pid
			}
		}
	}
	return 0
}

// Restart restarts the daemon with current settings.
func (m *Manager) Restart(settings config.DaemonSettings) error {
	if err := m.Stop(); err != nil {
		return err
	}
	time.Sleep(150 * time.Millisecond)
	return m.Start(settings)
}

// Snapshot returns current process snapshot.
func (m *Manager) Snapshot() Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	copy := m.snapshot
	copy.LaunchArgs = append([]string(nil), copy.LaunchArgs...)
	return copy
}

// Logs returns recent daemon logs.
func (m *Manager) Logs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]string(nil), m.logs...)
}

func (m *Manager) wait() {
	m.mu.RLock()
	cmd := m.cmd
	m.mu.RUnlock()
	if cmd == nil {
		return
	}
	err := cmd.Wait()
	m.mu.Lock()
	defer m.mu.Unlock()
	if err != nil {
		m.snapshot.LastExit = err.Error()
		m.snapshot.LastError = err.Error()
		m.logs = append(m.logs, time.Now().Format("15:04:05 ")+" derod exited: "+err.Error())
	} else {
		m.snapshot.LastExit = "stopped"
		m.logs = append(m.logs, time.Now().Format("15:04:05 ")+" derod exited")
	}
	m.snapshot.Running = false
	m.snapshot.PID = 0
	m.cmd = nil
}

func (m *Manager) capture(r io.Reader) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		m.appendLog(scanner.Text())
	}
}

func (m *Manager) appendLog(line string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.appendLogLocked(line)
}

func (m *Manager) appendLogLocked(line string) {
	entry := time.Now().Format("15:04:05 ") + line
	m.logs = append(m.logs, entry)
	if len(m.logs) > maxLogLines {
		m.logs = append([]string(nil), m.logs[len(m.logs)-maxLogLines:]...)
	}
}

func (m *Manager) setError(msg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.snapshot.LastError = msg
	if msg != "" {
		m.logs = append(m.logs, time.Now().Format("15:04:05 ")+" error: "+msg)
	}
}
