// Copyright 2017-2026 DERO Project. All rights reserved.

package pages

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/deroproject/dero-wallet-cli/internal/services/installer"
	"github.com/deroproject/dero-wallet-cli/internal/ui/styles"
)

var (
	daemonStartKeys     = key.NewBinding(key.WithKeys("s"))
	daemonStopKeys      = key.NewBinding(key.WithKeys("x"))
	daemonRestartKeys   = key.NewBinding(key.WithKeys("r"))
	daemonLogsKeys      = key.NewBinding(key.WithKeys("l"))
	daemonInstallKeys   = key.NewBinding(key.WithKeys("i"))
	daemonConfigKeys    = key.NewBinding(key.WithKeys("c"))
	daemonUninstallKeys = key.NewBinding(key.WithKeys("u"))
)

type DaemonStatusSnapshot struct {
	Running               bool
	Managed               bool
	BinaryReady           bool
	Source                string
	PID                   int
	Network               string
	BinaryPath            string
	DataDir               string
	RPCBind               string
	P2PBind               string
	GetWorkBind           string
	IntegratorAddress     string
	LaunchArgs            []string
	LastError             string
	IsOnline              bool
	IsHealthy             bool
	IsSynced              bool
	IsSyncing             bool
	IsBootstrapping       bool
	IsFinalizingBootstrap bool
	BlockHeight           uint64
	StableHeight          int64
	TopoHeight            int64
	PeerHeight            int64
	SyncProgress          float64
	Version               string
	Difficulty            uint64
	AvgBlockTime          float32
	IncomingPeers         uint64
	OutgoingPeers         uint64
	KnownPeers            uint64
	Uptime                uint64
	TxPoolSize            uint64
	Hashrate1hr           uint64
	Hashrate1d            uint64
}

type DaemonStatusModel struct {
	Snapshot            DaemonStatusSnapshot
	Downloading         bool
	DownloadError       string
	InstallResult       string
	InstallPlan         *installer.Plan
	wantStart           bool
	wantStop            bool
	wantRestart         bool
	wantLogs            bool
	wantSettings        bool
	wantInstall         bool
	wantInstallApply    bool
	wantInstallDone     bool
	wantUninstall       bool
	wantUninstallApply  bool
	wantUninstallDone   bool
	ConfirmingUninstall bool
	cancelled           bool
	escArmed            bool
	escArmedAt          time.Time
	width               int
	height              int
}

const daemonStatusContentWidth = 70
const daemonStatusLabelWidth = 12

func NewDaemonStatus() DaemonStatusModel  { return DaemonStatusModel{} }
func (d DaemonStatusModel) Init() tea.Cmd { return nil }

func padLabelText(label string) string {
	for len(label) < daemonStatusLabelWidth {
		label += " "
	}
	return label
}

func (d DaemonStatusModel) Update(msg tea.Msg) (DaemonStatusModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		d.DownloadError = ""
		d.InstallResult = ""
		isEsc := key.Matches(msg, pageEscKeys)
		// Any non-Esc key disarms the leave-page guard.
		if !isEsc {
			d.escArmed = false
			d.escArmedAt = time.Time{}
		}
		// While an install plan is awaiting confirmation, only the confirm
		// keys apply — regular page keys (start/stop/etc.) must not fire.
		if d.InstallPlan != nil {
			switch {
			case msg.String() == "y" || msg.String() == "Y":
				d.wantInstallApply = true
			case msg.String() == "n" || msg.String() == "N":
				d.wantInstallDone = true
			case isEsc:
				d.wantInstallDone = true
			}
			return d, nil
		}
		// While an uninstall is awaiting confirmation, only the confirm keys
		// apply — regular page keys must not fire.
		if d.ConfirmingUninstall {
			switch {
			case msg.String() == "y" || msg.String() == "Y":
				d.wantUninstallApply = true
			case msg.String() == "n" || msg.String() == "N":
				d.wantUninstallDone = true
			case isEsc:
				d.wantUninstallDone = true
			}
			return d, nil
		}
		// Leaving the page requires two Esc presses within 2s. A lone Escape
		// arriving from the terminal (e.g. a fragmented escape sequence) can
		// otherwise kick the user back to welcome with no input at all.
		if isEsc {
			now := time.Now()
			if d.escArmed && now.Sub(d.escArmedAt) < 2*time.Second {
				d.cancelled = true
			} else {
				d.escArmed = true
				d.escArmedAt = now
			}
			return d, nil
		}
		switch {
		case key.Matches(msg, daemonStartKeys):
			d.wantStart = true
		case key.Matches(msg, daemonStopKeys):
			d.wantStop = true
		case key.Matches(msg, daemonRestartKeys):
			d.wantRestart = true
		case key.Matches(msg, daemonLogsKeys):
			d.wantLogs = true
		case key.Matches(msg, daemonInstallKeys):
			d.wantInstall = true
		case key.Matches(msg, daemonConfigKeys):
			d.wantSettings = true
		case key.Matches(msg, daemonUninstallKeys):
			d.wantUninstall = true
		}
	case tea.WindowSizeMsg:
		d.width = msg.Width
		d.height = msg.Height
	}
	return d, nil
}

func (d DaemonStatusModel) View() string {
	const contentWidth = daemonStatusContentWidth
	const labelWidth = daemonStatusLabelWidth

	padLabel := padLabelText

	sectionHeader := func(title string, clr color.Color) string {
		prefix := "── "
		suffix := " "
		lineLen := contentWidth - len(prefix) - len(title) - len(suffix)
		if lineLen < 0 {
			lineLen = 0
		}
		return styles.MutedStyle.Render(prefix) +
			lipgloss.NewStyle().Foreground(clr).Bold(true).Render(title+suffix) +
			styles.MutedStyle.Render(styles.Separator(lineLen))
	}

	row := func(label, value string) string {
		return styles.MutedStyle.Render(padLabel(label)) + value
	}

	rows := []string{
		sectionHeader("Status", styles.ColorSuccess),
		row("State:", d.renderStateLine()),
	}
	if d.Snapshot.Version != "" {
		rows = append(rows, row("Version:", styles.TextStyle.Render(d.Snapshot.Version)))
	}
	rows = append(rows, row("RPC:", d.renderRPCLine()))
	rows = append(rows, row("Network:", d.renderMetaLine()))
	rows = append(rows, row("Block:", d.renderHeightLine()))
	if d.Snapshot.Uptime > 0 {
		rows = append(rows, row("Uptime:", styles.TextStyle.Render(formatUptime(d.Snapshot.Uptime))))
	}

	if d.Downloading {
		rows = append(rows, "", styles.WarningStyle.Render("  Downloading derod..."))
	} else if d.Snapshot.Running && !d.Snapshot.IsOnline && d.Snapshot.BlockHeight == 0 {
		rows = append(rows, "", styles.MutedStyle.Render("  Waiting for RPC..."))
	}
	if d.DownloadError != "" {
		rows = append(rows, "", styles.ErrorStyle.Render("  ✗ "+d.DownloadError))
	}
	if d.InstallResult != "" {
		rows = append(rows, "", styles.SuccessStyle.Render("  ✓ "+d.InstallResult))
	}
	if d.Snapshot.LastError != "" && !d.Downloading {
		rows = append(rows, "", styles.ErrorStyle.Render("  ✗ "+d.Snapshot.LastError))
	}

	rows = append(rows, "", sectionHeader("Network", styles.ColorPrimary))
	rows = append(rows, row("Peers:", d.renderPeersLine()))
	if d.Snapshot.Difficulty > 0 {
		rows = append(rows, row("Difficulty:", styles.TextStyle.Render(formatUint64(d.Snapshot.Difficulty))))
	}
	if d.Snapshot.AvgBlockTime > 0 {
		rows = append(rows, row("Block Time:", styles.TextStyle.Render(fmt.Sprintf("~%.0fs", d.Snapshot.AvgBlockTime))))
	}
	if d.Snapshot.TxPoolSize > 0 {
		rows = append(rows, row("Tx Pool:", styles.TextStyle.Render(fmt.Sprintf("%d pending", d.Snapshot.TxPoolSize))))
	}
	if d.Snapshot.Hashrate1hr > 0 || d.Snapshot.Hashrate1d > 0 {
		// These are derod's estimate of its own connected miners' hashrate
		// (network hashrate x its share), printed as "Avg Mining HR" by the
		// daemon CLI — not the network hashrate. Label accordingly.
		rows = append(rows, row("Mining HR:", d.renderHashrateLine()))
	}

	rows = append(rows, "", sectionHeader("Configuration", styles.ColorAccent))
	if strings.EqualFold(strings.TrimSpace(d.Snapshot.Source), "Embedded") {
		rows = append(rows,
			row("Mode:", styles.SuccessStyle.Render("Embedded Helper")),
			row("Data:", truncatePlain(d.fallback(d.Snapshot.DataDir), contentWidth-labelWidth-1)),
			row("RPC:", truncatePlain(d.fallback(d.Snapshot.RPCBind), contentWidth-labelWidth-1)),
			row("P2P:", truncatePlain(d.fallback(d.Snapshot.P2PBind), contentWidth-labelWidth-1)),
			row("Integrator:", truncatePlain(d.fallback(d.Snapshot.IntegratorAddress), contentWidth-labelWidth-1)),
		)
	} else {
		rows = append(rows,
			row("Binary:", truncatePlain(d.fallback(d.Snapshot.BinaryPath), contentWidth-labelWidth-1)),
			row("Data:", truncatePlain(d.fallback(d.Snapshot.DataDir), contentWidth-labelWidth-1)),
			row("RPC:", truncatePlain(d.fallback(d.Snapshot.RPCBind), contentWidth-labelWidth-1)),
			row("P2P:", truncatePlain(d.fallback(d.Snapshot.P2PBind), contentWidth-labelWidth-1)),
			row("Integrator:", truncatePlain(d.fallback(d.Snapshot.IntegratorAddress), contentWidth-labelWidth-1)),
		)
	}

	if d.InstallPlan != nil && !d.Downloading {
		rows = append(rows, "", sectionHeader("Install as Service", styles.ColorAccent))
		rows = append(rows, d.renderInstallPlan()...)
	}

	if d.ConfirmingUninstall && !d.Downloading {
		rows = append(rows, "", sectionHeader("Reset daemon", styles.ColorAccent))
		rows = append(rows, d.renderUninstallConfirm()...)
	}

	rows = append(rows, "", d.renderFooter())

	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

// renderInstallPlan shows the fetched install plan and asks for confirmation
// before any system-level changes are made.
func (d DaemonStatusModel) renderInstallPlan() []string {
	plan := d.InstallPlan
	if plan == nil {
		return nil
	}
	val := func(label, v string) string {
		if strings.TrimSpace(v) == "" {
			v = "Not set"
		}
		return styles.MutedStyle.Render(padLabelText(label)) + styles.TextStyle.Render(truncatePlain(v, daemonStatusContentWidth-daemonStatusLabelWidth-1))
	}
	rows := []string{
		val("Type:", plan.ServiceType),
		val("Binary:", plan.BinaryTarget),
		val("Data:", plan.DataDir),
		val("RPC:", plan.RPCBind),
		val("P2P:", plan.P2PBind),
		val("GetWork:", plan.GetWorkBind),
	}
	if strings.TrimSpace(plan.ReleaseTag) != "" {
		rows = append(rows, val("Release:", plan.ReleaseTag))
	}
	if strings.TrimSpace(plan.NodeTag) != "" {
		rows = append(rows, val("Node Tag:", plan.NodeTag))
	}
	if strings.TrimSpace(plan.IntegratorAddress) != "" {
		rows = append(rows, val("Integrator:", plan.IntegratorAddress))
	}
	if strings.TrimSpace(plan.Network) != "" {
		rows = append(rows, val("Network:", plan.Network))
	}
	if strings.TrimSpace(plan.FallbackNote) != "" {
		rows = append(rows, "", styles.MutedStyle.Render("  "+plan.FallbackNote))
	}
	rows = append(rows, "", styles.WarningStyle.Render("  [Y] Install Service \u2022 [N]/Esc Cancel"))
	return rows
}

// renderUninstallConfirm explains what a reset removes and asks for
// confirmation before any system-level changes are made.
func (d DaemonStatusModel) renderUninstallConfirm() []string {
	rows := []string{
		styles.MutedStyle.Render("  Stops the daemon, removes the derod service and downloaded"),
		styles.MutedStyle.Render("  binary, and deletes the chain data folder \u2014 starting from scratch."),
		styles.MutedStyle.Render("  Wallet files and app config are kept."),
		"",
		styles.WarningStyle.Render("  [Y] Reset \u2022 [N]/Esc Cancel"),
	}
	return rows
}

func (d DaemonStatusModel) renderFooter() string {
	k := styles.AccentStyle.Render
	m := styles.MutedStyle.Render
	sep := m(" • ")

	parts := []string{}
	isEmbedded := strings.EqualFold(strings.TrimSpace(d.Snapshot.Source), "Embedded")
	isSystemDaemon := strings.EqualFold(strings.TrimSpace(d.Snapshot.Source), "System Daemon")

	if !d.Downloading && !d.Snapshot.Running && !d.Snapshot.Managed {
		label := "Start"
		if !isEmbedded && !d.Snapshot.BinaryReady {
			label = "Download"
		}
		parts = append(parts, k("S")+" "+m(label))
	} else {
		parts = append(parts, k("X")+" "+m("Stop"))
	}

	if d.Snapshot.Running || d.Snapshot.Managed {
		parts = append(parts, k("R")+" "+m("Restart"))
		parts = append(parts, k("L")+" "+m("Logs"))
	}

	// Install registers the built-in daemon as a background service, so it
	// applies to embedded mode too. It only makes sense when nothing is
	// running yet — installing while a daemon is up would start a conflicting
	// second node on the same ports.
	if !d.Snapshot.Running && !d.Snapshot.Managed {
		parts = append(parts, k("I")+" "+m("Install Service"))
	}

	// Reset is offered for any daemon we own or installed: the embedded
	// helper, a systemd service, or a managed/planned local node. External
	// local daemons (nodes we merely connected to) are left alone — we
	// don't own their data.
	source := strings.ToLower(strings.TrimSpace(d.Snapshot.Source))
	resettable := isEmbedded || isSystemDaemon ||
		strings.Contains(source, "managed") || strings.Contains(source, "planned")
	if !d.Downloading && resettable && !d.ConfirmingUninstall && d.InstallPlan == nil {
		parts = append(parts, k("U")+" "+m("Reset"))
	}
	if d.escArmed {
		parts = append(parts, k("C")+" "+m("Config"), styles.WarningStyle.Render("Esc again to leave"))
	} else {
		parts = append(parts, k("C")+" "+m("Config"), k("Esc")+" "+m("Back"))
	}

	return strings.Join(parts, sep)
}

func (d DaemonStatusModel) renderStateLine() string {
	if d.Downloading {
		return styles.WarningStyle.Render("Downloading")
	}
	if d.Snapshot.IsOnline && d.Snapshot.IsHealthy {
		if d.Snapshot.BlockHeight == 0 && d.Snapshot.PeerHeight == 0 && d.Snapshot.IncomingPeers == 0 && d.Snapshot.OutgoingPeers == 0 {
			return styles.WarningStyle.Render("Waiting for peers")
		}
		if d.Snapshot.IsFinalizingBootstrap {
			return styles.WarningStyle.Render("Finalizing Bootstrap...")
		}
		if d.Snapshot.IsBootstrapping {
			return styles.BootstrappingStyle.Render("Bootstrapping " + formatSyncPct(d.Snapshot.SyncProgress))
		}
		if d.Snapshot.IsSynced {
			return styles.SuccessStyle.Render("Synced")
		}
		if d.Snapshot.IsSyncing {
			return styles.WarningStyle.Render("Syncing " + formatSyncPct(d.Snapshot.SyncProgress))
		}
		return styles.WarningStyle.Render("Online")
	}
	if d.Snapshot.Running {
		return styles.WarningStyle.Render("Starting")
	}
	if d.Snapshot.Source == "Planned Local" && !d.Snapshot.BinaryReady && strings.TrimSpace(d.Snapshot.BinaryPath) == "" {
		return styles.WarningStyle.Render("Not configured")
	}
	return styles.ErrorStyle.Render("Stopped")
}

func (d DaemonStatusModel) renderRPCLine() string {
	if d.Snapshot.IsOnline && d.Snapshot.IsHealthy {
		if d.Snapshot.BlockHeight == 0 && d.Snapshot.PeerHeight == 0 && d.Snapshot.IncomingPeers == 0 && d.Snapshot.OutgoingPeers == 0 {
			return styles.WarningStyle.Render("No peers yet")
		}
		if d.Snapshot.IsFinalizingBootstrap {
			return styles.WarningStyle.Render("Finalizing Bootstrap...")
		}
		if d.Snapshot.IsBootstrapping {
			return styles.BootstrappingStyle.Render("Bootstrapping " + formatSyncPct(d.Snapshot.SyncProgress))
		}
		if d.Snapshot.IsSynced {
			return styles.SuccessStyle.Render("Healthy")
		}
		return styles.WarningStyle.Render("Syncing")
	}
	return styles.ErrorStyle.Render("Unreachable")
}

func (d DaemonStatusModel) renderMetaLine() string {
	parts := []string{d.renderNetwork()}
	mode := strings.TrimSpace(d.Snapshot.Source)
	if mode != "" {
		parts = append(parts, styles.MutedStyle.Render(mode))
	}
	if d.Snapshot.PID > 0 {
		parts = append(parts, styles.MutedStyle.Render(fmt.Sprintf("PID %d", d.Snapshot.PID)))
	}
	return strings.Join(parts, styles.MutedStyle.Render("  •  "))
}

func (d DaemonStatusModel) renderHeightLine() string {
	if d.Snapshot.BlockHeight == 0 {
		if d.Snapshot.PeerHeight > 0 {
			return styles.MutedStyle.Render("0 / ") + styles.TextStyle.Render(formatUint64(uint64(d.Snapshot.PeerHeight)))
		}
		return styles.MutedStyle.Render("-")
	}

	heightStr := formatUint64(d.Snapshot.BlockHeight)

	if d.Snapshot.PeerHeight > 0 {
		target := formatUint64(uint64(d.Snapshot.PeerHeight))
		if d.Snapshot.IsSynced {
			return styles.SuccessStyle.Render(heightStr) +
				styles.MutedStyle.Render(" / ") +
				styles.SuccessStyle.Render(target) +
				styles.SuccessStyle.Render(" ("+formatSyncPct(d.Snapshot.SyncProgress)+")")
		}
		heightStr = styles.WarningStyle.Render(heightStr) +
			styles.MutedStyle.Render(" / ") +
			styles.TextStyle.Render(target) +
			styles.MutedStyle.Render(" (") +
			styles.WarningStyle.Render(formatSyncPct(d.Snapshot.SyncProgress)) +
			styles.MutedStyle.Render(")")
	}

	if !d.Snapshot.IsSynced && d.Snapshot.StableHeight > 0 && d.Snapshot.StableHeight < int64(d.Snapshot.BlockHeight) {
		heightStr += styles.MutedStyle.Render("  •  Confirmed ") + styles.TextStyle.Render(fmt.Sprintf("%d", d.Snapshot.StableHeight))
	}

	return heightStr
}

// formatSyncPct renders a sync progress percentage, using "<0.1%" so a node
// that just started its genesis sync doesn't show a meaningless "0.0%".
func formatSyncPct(progress float64) string {
	if progress > 0 && progress < 0.1 {
		return "<0.1%"
	}
	return fmt.Sprintf("%.1f%%", progress)
}

func (d DaemonStatusModel) renderPeersLine() string {
	if d.Snapshot.IncomingPeers == 0 && d.Snapshot.OutgoingPeers == 0 && d.Snapshot.KnownPeers == 0 {
		return styles.MutedStyle.Render("-")
	}
	parts := []string{}
	if d.Snapshot.IncomingPeers > 0 || d.Snapshot.OutgoingPeers > 0 {
		parts = append(parts, styles.TextStyle.Render(fmt.Sprintf("%d in", d.Snapshot.IncomingPeers))+
			styles.MutedStyle.Render(" / ")+
			styles.TextStyle.Render(fmt.Sprintf("%d out", d.Snapshot.OutgoingPeers)))
	}
	if d.Snapshot.KnownPeers > 0 {
		parts = append(parts, styles.TextStyle.Render(formatUint64(d.Snapshot.KnownPeers))+
			styles.MutedStyle.Render(" known"))
	}
	return strings.Join(parts, styles.MutedStyle.Render("  •  "))
}

func (d DaemonStatusModel) renderHashrateLine() string {
	parts := []string{}
	if d.Snapshot.Hashrate1hr > 0 {
		parts = append(parts, styles.TextStyle.Render(formatHashrate(d.Snapshot.Hashrate1hr))+
			styles.MutedStyle.Render(" (1h)"))
	}
	if d.Snapshot.Hashrate1d > 0 {
		parts = append(parts, styles.TextStyle.Render(formatHashrate(d.Snapshot.Hashrate1d))+
			styles.MutedStyle.Render(" (1d)"))
	}
	if len(parts) == 0 {
		return styles.MutedStyle.Render("-")
	}
	return strings.Join(parts, styles.MutedStyle.Render("  •  "))
}

func (d DaemonStatusModel) renderNetwork() string {
	network := strings.TrimSpace(d.Snapshot.Network)
	if network == "" {
		return styles.MutedStyle.Render("Not configured")
	}
	if strings.EqualFold(network, "testnet") {
		return styles.TestnetStyle.Render(network)
	}
	if strings.EqualFold(network, "simulator") {
		return styles.SimulatorStyle.Render(network)
	}
	return styles.SuccessStyle.Render("Mainnet")
}

func (d DaemonStatusModel) fallback(v string) string {
	if strings.TrimSpace(v) == "" {
		return "Not configured"
	}
	return v
}

func formatUptime(seconds uint64) string {
	days := seconds / 86400
	seconds %= 86400
	hours := seconds / 3600
	seconds %= 3600
	mins := seconds / 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, mins)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, mins)
	}
	return fmt.Sprintf("%dm", mins)
}

func formatUint64(n uint64) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var result strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result.WriteByte(',')
		}
		result.WriteRune(c)
	}
	return result.String()
}

func formatHashrate(h uint64) string {
	switch {
	case h >= 1_000_000_000:
		return fmt.Sprintf("%.1f GH/s", float64(h)/1_000_000_000)
	case h >= 1_000_000:
		return fmt.Sprintf("%.1f MH/s", float64(h)/1_000_000)
	case h >= 1_000:
		return fmt.Sprintf("%.1f kH/s", float64(h)/1_000)
	default:
		return fmt.Sprintf("%d H/s", h)
	}
}

func (d *DaemonStatusModel) SetSnapshot(snapshot DaemonStatusSnapshot) { d.Snapshot = snapshot }
func (d DaemonStatusModel) WantStart() bool                            { return d.wantStart }
func (d DaemonStatusModel) WantStop() bool                             { return d.wantStop }
func (d DaemonStatusModel) WantRestart() bool                          { return d.wantRestart }
func (d DaemonStatusModel) WantLogs() bool                             { return d.wantLogs }
func (d DaemonStatusModel) WantSettings() bool                         { return d.wantSettings }
func (d DaemonStatusModel) WantInstall() bool                          { return d.wantInstall }
func (d DaemonStatusModel) WantInstallApply() bool                     { return d.wantInstallApply }
func (d DaemonStatusModel) WantInstallDone() bool                      { return d.wantInstallDone }
func (d DaemonStatusModel) WantUninstall() bool                        { return d.wantUninstall }
func (d DaemonStatusModel) WantUninstallApply() bool                   { return d.wantUninstallApply }
func (d DaemonStatusModel) WantUninstallDone() bool                    { return d.wantUninstallDone }
func (d DaemonStatusModel) Cancelled() bool                            { return d.cancelled }
func (d *DaemonStatusModel) SetInstallPlan(plan *installer.Plan)       { d.InstallPlan = plan }
func (d *DaemonStatusModel) ResetInstall() {
	d.InstallPlan = nil
	d.wantInstallApply = false
	d.wantInstallDone = false
}
func (d *DaemonStatusModel) ResetUninstall() {
	d.ConfirmingUninstall = false
	d.wantUninstallApply = false
	d.wantUninstallDone = false
}
func (d *DaemonStatusModel) ResetActions() {
	d.wantStart = false
	d.wantStop = false
	d.wantRestart = false
	d.wantLogs = false
	d.wantSettings = false
	d.wantInstall = false
	d.wantInstallApply = false
	d.wantInstallDone = false
	d.wantUninstall = false
	d.wantUninstallApply = false
	d.wantUninstallDone = false
	d.cancelled = false
	d.escArmed = false
	d.escArmedAt = time.Time{}
}
