// Copyright 2017-2026 DERO Project. All rights reserved.

package pages

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/deroproject/dero-wallet-cli/internal/ui/styles"
)

var (
	minerStartKeys      = key.NewBinding(key.WithKeys("s"))
	minerStopKeys       = key.NewBinding(key.WithKeys("x"))
	minerThreadUpKeys   = key.NewBinding(key.WithKeys("right", "+", "="))
	minerThreadDownKeys = key.NewBinding(key.WithKeys("left", "-"))
)

type MinerModel struct {
	Running     bool
	Threads     int
	MaxThreads  int
	Hashrate    uint64
	Hashes      uint64
	Minis       uint64
	BlocksFound uint64
	Rejected    uint64
	Height      uint64
	Difficulty  uint64
	Uptime      time.Duration
	Address     string
	Status      string
	DaemonHost  string
	lastError   string
	cancelled   bool
	wantStart   bool
	wantStop    bool
	width       int
	height      int
}

func NewMiner() MinerModel {
	maxThreads := runtime.GOMAXPROCS(0)
	if maxThreads < 1 {
		maxThreads = 1
	}
	return MinerModel{
		Threads:    maxThreads,
		MaxThreads: maxThreads,
	}
}

func (m MinerModel) Init() tea.Cmd { return nil }

func (m MinerModel) Update(msg tea.Msg) (MinerModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		m.lastError = ""
		switch {
		case key.Matches(msg, pageEscKeys):
			m.cancelled = true
		case key.Matches(msg, minerStartKeys):
			if !m.Running {
				m.wantStart = true
			}
		case key.Matches(msg, minerStopKeys):
			if m.Running {
				m.wantStop = true
			}
		case key.Matches(msg, minerThreadDownKeys):
			if !m.Running && m.Threads > 1 {
				m.Threads--
			}
		case key.Matches(msg, minerThreadUpKeys):
			if !m.Running && m.Threads < m.MaxThreads {
				m.Threads++
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

func (m MinerModel) View() string {
	rows := []string{
		styles.TitleStyle.Render("Miner"),
		"",
		styles.MutedStyle.Render("Status: ") + m.renderStatus(),
		styles.MutedStyle.Render("Address: ") + m.renderAddress(),
		styles.MutedStyle.Render("Threads: ") + styles.TextStyle.Render(fmt.Sprintf("%d/%d", m.Threads, m.MaxThreads)),
		styles.MutedStyle.Render("Hashrate: ") + styles.TextStyle.Render(formatMinerHashrate(m.Hashrate)),
		styles.MutedStyle.Render("Total Hashes: ") + styles.TextStyle.Render(formatUint64(m.Hashes)),
		styles.MutedStyle.Render("Minis: ") + styles.TextStyle.Render(fmt.Sprintf("%d", m.Minis)),
		styles.MutedStyle.Render("Blocks: ") + styles.TextStyle.Render(fmt.Sprintf("%d", m.BlocksFound)),
		styles.MutedStyle.Render("Rejected: ") + styles.TextStyle.Render(fmt.Sprintf("%d", m.Rejected)),
		styles.MutedStyle.Render("Height: ") + styles.TextStyle.Render(fmt.Sprintf("%d", m.Height)),
		styles.MutedStyle.Render("Difficulty: ") + styles.TextStyle.Render(formatMinerDifficulty(m.Difficulty)),
		styles.MutedStyle.Render("Uptime: ") + styles.TextStyle.Render(formatMinerUptime(m.Uptime)),
	}

	if m.DaemonHost != "" {
		rows = append(rows, styles.MutedStyle.Render("Daemon: ")+styles.TextStyle.Render(m.DaemonHost))
	}

	if m.Status != "" {
		rows = append(rows, "", styles.MutedStyle.Render(m.Status))
	}
	if m.lastError != "" {
		rows = append(rows, "", styles.ErrorStyle.Render("✗ "+m.lastError))
	}

	rows = append(rows, "", styles.MutedStyle.Render(m.renderFooter()))
	return styles.ThemedBoxStyle().Width(styles.Width).Padding(2, 4).Render(strings.Join(rows, "\n"))
}

func (m *MinerModel) SetRunning(running bool) {
	m.Running = running
	if !running {
		m.Hashrate = 0
	}
}

func (m *MinerModel) SetStats(hashrate, hashes, minis, blocks, rejected, height, difficulty uint64, uptime time.Duration) {
	m.Hashrate = hashrate
	m.Hashes = hashes
	m.Minis = minis
	m.BlocksFound = blocks
	m.Rejected = rejected
	m.Height = height
	m.Difficulty = difficulty
	m.Uptime = uptime
}

func (m *MinerModel) SetAddress(address string) {
	m.Address = address
}

func (m *MinerModel) SetThreads(threads int) {
	if threads < 1 {
		threads = 1
	}
	if threads > m.MaxThreads {
		threads = m.MaxThreads
	}
	m.Threads = threads
}

func (m *MinerModel) SetStatus(status string) {
	m.Status = status
}

func (m *MinerModel) SetDaemonHost(host string) {
	m.DaemonHost = host
}

func (m *MinerModel) SetError(err string) {
	m.lastError = err
}

func (m MinerModel) Cancelled() bool { return m.cancelled }
func (m MinerModel) WantStart() bool { return m.wantStart }
func (m MinerModel) WantStop() bool  { return m.wantStop }

func (m *MinerModel) ResetActions() {
	m.cancelled = false
	m.wantStart = false
	m.wantStop = false
}

func (m MinerModel) renderStatus() string {
	if m.Running {
		return styles.SuccessStyle.Render("Mining")
	}
	return styles.WarningStyle.Render("Stopped")
}

func (m MinerModel) renderAddress() string {
	if strings.TrimSpace(m.Address) == "" {
		return styles.MutedStyle.Render("Open a wallet or press / to set a mining address")
	}
	return styles.TextStyle.Render(truncatePlain(m.Address, 56))
}

func (m MinerModel) renderFooter() string {
	k := styles.AccentStyle.Render
	mute := styles.MutedStyle.Render
	sep := mute(" • ")
	parts := []string{}
	if m.Running {
		parts = append(parts, k("X")+" "+mute("Stop"))
	} else {
		parts = append(parts, k("S")+" "+mute("Start"), k("←/→")+" "+mute("Threads"))
	}
	parts = append(parts, k("/")+" "+mute("Commands"), k("Esc")+" "+mute("Back"))
	return strings.Join(parts, sep)
}

func formatMinerHashrate(hashrate uint64) string {
	if hashrate >= 1000000 {
		return fmt.Sprintf("%.2f MH/s", float64(hashrate)/1000000)
	}
	if hashrate >= 1000 {
		return fmt.Sprintf("%.2f KH/s", float64(hashrate)/1000)
	}
	return fmt.Sprintf("%d H/s", hashrate)
}

// formatMinerUptime renders a duration as hh:mm:ss (matching the go-miner
// CLI's uptime format).
func formatMinerUptime(d time.Duration) string {
	total := int64(d.Seconds())
	if total < 0 {
		total = 0
	}
	hh := total / 3600
	mm := total / 60 % 60
	ss := total % 60
	return fmt.Sprintf("%02d:%02d:%02d", hh, mm, ss)
}

// formatMinerDifficulty humanizes a difficulty value the way the go-miner
// CLI does (integer division: 20K, 1M, 2G).
func formatMinerDifficulty(n uint64) string {
	switch {
	case n >= 1e9:
		return fmt.Sprintf("%dG", n/1e9)
	case n >= 1e6:
		return fmt.Sprintf("%dM", n/1e6)
	case n >= 1e3:
		return fmt.Sprintf("%dK", n/1e3)
	default:
		return fmt.Sprintf("%d", n)
	}
}
