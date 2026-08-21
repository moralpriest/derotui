// Copyright 2017-2026 DERO Project. All rights reserved.

package pages

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/deroproject/dero-wallet-cli/internal/config"
	"github.com/deroproject/dero-wallet-cli/internal/ui/components"
	"github.com/deroproject/dero-wallet-cli/internal/ui/styles"
	"github.com/deroproject/derohe/rpc"
)

var (
	minerStartKeys         = key.NewBinding(key.WithKeys("s"))
	minerStopKeys          = key.NewBinding(key.WithKeys("x"))
	minerThreadUpKeys      = key.NewBinding(key.WithKeys("right", "+", "="))
	minerThreadDownKeys    = key.NewBinding(key.WithKeys("left", "-"))
	minerAddressEditKeys   = key.NewBinding(key.WithKeys("a", "e"))
	minerAddressSaveKeys   = key.NewBinding(key.WithKeys("enter"))
	minerAddressCancelKeys = key.NewBinding(key.WithKeys("esc"))
)

type SpinnerTickMsg struct{}

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

	editingAddress bool
	addressInput   components.InputModel
	addressError   string
	spinnerFrame   int
}

func NewMiner() MinerModel {
	maxThreads := runtime.GOMAXPROCS(0)
	if maxThreads < 1 {
		maxThreads = 1
	}
	defaultThreads := maxThreads - 2
	if defaultThreads < 1 {
		defaultThreads = 1
	}
	input := components.NewInput("Address", "dero1... mining address", false)
	return MinerModel{
		Threads:      defaultThreads,
		MaxThreads:   maxThreads,
		addressInput: input,
	}
}

func (m MinerModel) Init() tea.Cmd { return nil }

func (m *MinerModel) IsEditingAddress() bool { return m.editingAddress }

func (m *MinerModel) AddressError() string { return m.addressError }

func (m MinerModel) Update(msg tea.Msg) (MinerModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		// If editing address, delegate to input and handle save/cancel
		if m.editingAddress {
			switch {
			case key.Matches(msg, minerAddressSaveKeys):
				addr := strings.TrimSpace(m.addressInput.Value())
				if err := m.ValidateAddress(addr); err != nil {
					m.addressError = err.Error()
					return m, nil
				}
				m.Address = addr
				m.addressInput.SetValue(addr)
				m.addressInput.Blur()
				m.editingAddress = false
				m.addressError = ""
				_ = config.SetLastMiningAddress(addr)
				m.Status = "Address saved"
				return m, nil
			case key.Matches(msg, minerAddressCancelKeys):
				m.addressInput.SetValue(m.Address)
				m.addressInput.Blur()
				m.editingAddress = false
				m.addressError = ""
				return m, nil
			}
			var cmd tea.Cmd
			m.addressInput, cmd = m.addressInput.Update(msg)
			m.addressError = ""
			return m, cmd
		}
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
		case key.Matches(msg, minerAddressEditKeys):
			if !m.Running {
				m.editingAddress = true
				m.addressInput.SetValue(m.Address)
				m.addressError = ""
				return m, m.addressInput.Focus()
			}
		}
	case SpinnerTickMsg:
		if m.Running && !m.wantStop {
			m.spinnerFrame = (m.spinnerFrame + 1) % 10
			return m, tea.Tick(time.Millisecond*180, func(t time.Time) tea.Msg { return SpinnerTickMsg{} })
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	if m.Running && !m.wantStop && !m.editingAddress {
		return m, tea.Tick(time.Millisecond*180, func(t time.Time) tea.Msg { return SpinnerTickMsg{} })
	}
	return m, nil
}

func (m MinerModel) View() string {
	pickaxe := "⛏️"
	if m.Running {
		if m.spinnerFrame%2 == 0 {
			pickaxe = styles.SuccessStyle.Render("⛏️")
		} else {
			pickaxe = styles.WarningStyle.Render("⛏️")
		}
	} else {
		pickaxe = styles.MutedStyle.Render("⛏️")
	}
	deroLogo := styles.Logo()
	if !m.Running {
		deroLogo = lipgloss.NewStyle().Faint(true).Render(deroLogo)
	}
	var title string
	if m.Running {
		spinner := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}[m.spinnerFrame%10]
		title = pickaxe + " " + styles.SuccessStyle.Render("Mining " + spinner)
	} else {
		title = styles.MutedStyle.Render("Stopped")
	}
	hr := formatMinerHashrate(m.Hashrate)
	var hrStyled string
	if m.Hashrate == 0 {
		hrStyled = styles.MutedStyle.Render(hr)
	} else {
		hrStyled = lipgloss.NewStyle().Foreground(styles.ColorSuccess).Bold(true).Render(hr)
	}
	innerW := styles.Width - 6
	headerTitle := styles.TitleStyle.Render("Miner")
	statusHr := lipgloss.JoinHorizontal(lipgloss.Center, title, "    ", hrStyled)
	headerBlock := lipgloss.JoinVertical(lipgloss.Center, headerTitle, "", deroLogo, "", statusHr)
	headerBlock = lipgloss.PlaceHorizontal(innerW, lipgloss.Center, headerBlock)
	rows := []string{
		headerBlock,
		"",
		styles.MutedStyle.Render("Threads: ") + styles.AccentStyle.Render(fmt.Sprintf("%d/%d", m.Threads, m.MaxThreads)),
		styles.MutedStyle.Render("Minis: ") + m.minisStyled(),
		styles.MutedStyle.Render("Blocks: ") + m.blocksStyled(),
		styles.MutedStyle.Render("Rejected: ") + m.rejectedStyled(),
		styles.MutedStyle.Render("Height: ") + styles.TextStyle.Render(fmt.Sprintf("%d", m.Height)),
		styles.MutedStyle.Render("Difficulty: ") + styles.TextStyle.Render(formatMinerDifficulty(m.Difficulty)),
		styles.MutedStyle.Render("Uptime: ") + styles.TextStyle.Render(formatMinerUptime(m.Uptime)),
		styles.MutedStyle.Render("Total Hashes: ") + styles.TextStyle.Render(formatUint64(m.Hashes)),
	}

	if m.editingAddress {
		rows = append(rows, "", m.addressInput.View())
		if m.addressError != "" {
			rows = append(rows, styles.ErrorStyle.Render("✗ "+m.addressError))
		}
	}
	if m.Status != "" {
		rows = append(rows, "", styles.MutedStyle.Render(m.Status))
	}
	if m.lastError != "" {
		rows = append(rows, "", styles.ErrorStyle.Render("✗ "+m.lastError))
	}

	rows = append(rows, "", styles.MutedStyle.Render(m.renderFooter()))
	boxStyle := styles.ThemedBoxStyle().Width(styles.Width).Padding(1, 2)
	if m.Running {
		if m.spinnerFrame%2 == 1 {
			boxStyle = boxStyle.BorderForeground(styles.ColorWarning)
		} else {
			boxStyle = boxStyle.BorderForeground(styles.ColorSuccess)
		}
	}
	content := boxStyle.Render(strings.Join(rows, "\n"))
	w, h := m.width, m.height
	if w == 0 {
		w = styles.Width + 4
	}
	if h == 0 {
		h = 40
	}
	if h < 36 {
		h = 36
	}
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Top, content)
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
	m.Address = strings.TrimSpace(address)
	if !m.editingAddress {
		m.addressInput.SetValue(m.Address)
		m.addressError = ""
	}
}

func (m *MinerModel) SetEditingAddress(editing bool) {
	m.editingAddress = editing
	if editing {
		m.addressInput.SetValue(m.Address)
		m.addressError = ""
	}
}

func (m *MinerModel) ValidateAddress(addr string) error {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return fmt.Errorf("address required")
	}
	if _, err := rpc.NewAddress(addr); err != nil {
		return fmt.Errorf("invalid address: %w", err)
	}
	return nil
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

func (m MinerModel) Cancelled() bool      { return m.cancelled }
func (m MinerModel) WantStart() bool      { return m.wantStart }
func (m MinerModel) WantStop() bool       { return m.wantStop }
func (m MinerModel) GetSpinnerFrame() int { return m.spinnerFrame }

func (m *MinerModel) ResetActions() {
	m.cancelled = false
	m.wantStart = false
	m.wantStop = false
}

func (m MinerModel) renderStatus() string {
	pickaxe := "⛏️"
	if m.Running {
		if m.spinnerFrame%2 == 0 {
			pickaxe = styles.SuccessStyle.Render("⛏️")
		} else {
			pickaxe = styles.WarningStyle.Render("⛏️")
		}
	} else {
		pickaxe = styles.MutedStyle.Render("⛏️")
	}
	if m.Running {
		spinner := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}[m.spinnerFrame%10]
		return styles.SuccessStyle.Render(pickaxe + " " + spinner + " Mining")
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
	sep := " • "
	parts := []string{}
	if m.editingAddress {
		parts = append(parts, k("Enter")+" "+mute("Save"), k("Esc")+" "+mute("Cancel"))
	} else if m.Running {
		parts = append(parts, k("X")+" "+mute("Stop"))
	} else {
		parts = append(parts, k("S")+" "+mute("Start"), k("←/→")+" "+mute("Threads"), k("A")+" "+mute("Edit Address"))
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

func (m MinerModel) hashrateStyled() string {
	s := formatMinerHashrate(m.Hashrate)
	if m.Hashrate == 0 {
		return styles.MutedStyle.Render(s)
	}
	return styles.SuccessStyle.Render(s)
}

func (m MinerModel) rejectedStyled() string {
	s := fmt.Sprintf("%d", m.Rejected)
	if m.Rejected > 0 {
		return styles.ErrorStyle.Render(s)
	}
	return styles.MutedStyle.Render(s)
}

func (m MinerModel) blocksStyled() string {
	s := fmt.Sprintf("%d", m.BlocksFound)
	if m.BlocksFound > 0 {
		return styles.SuccessStyle.Render(s)
	}
	return styles.MutedStyle.Render(s)
}

func (m MinerModel) minisStyled() string {
	s := fmt.Sprintf("%d", m.Minis)
	if m.Minis > 0 {
		return styles.SuccessStyle.Render(s)
	}
	return styles.MutedStyle.Render(s)
}
