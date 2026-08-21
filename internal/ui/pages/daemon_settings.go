// Copyright 2017-2026 DERO Project. All rights reserved.

package pages

import (
	"fmt"
	"os"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/deroproject/dero-wallet-cli/internal/config"
	"github.com/deroproject/dero-wallet-cli/internal/ui/styles"
)

type daemonSettingField int

const (
	daemonFieldMode daemonSettingField = iota
	daemonFieldBinary
	daemonFieldNetwork
	daemonFieldDataDir
	daemonFieldFastSync
	daemonFieldIntegrator
	daemonFieldNodeTag
	daemonFieldRPCBind
	daemonFieldP2PBind
	daemonFieldGetWorkBind
	daemonFieldSocksProxy
	daemonFieldDebug
)

type daemonSettingRow struct {
	field daemonSettingField
	label string
	value string
}

type DaemonSettingsModel struct {
	Settings        config.DaemonSettings
	selected        int
	editing         bool
	editBuffer      string
	errorMsg        string
	successMsg      string
	cancelled       bool
	saved           bool
	wantUseWallet   bool
	hostnameHint    string
	restartRequired bool
}

func NewDaemonSettings(settings config.DaemonSettings) DaemonSettingsModel {
	host, _ := os.Hostname()
	return DaemonSettingsModel{Settings: settings, hostnameHint: host}
}

func (d DaemonSettingsModel) Init() tea.Cmd { return nil }

func (d DaemonSettingsModel) Update(msg tea.Msg) (DaemonSettingsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if d.editing {
			switch {
			case key.Matches(msg, pageEscKeys):
				d.editing = false
				d.editBuffer = ""
			case key.Matches(msg, pageEnterKeys):
				d.applyEdit()
			default:
				s := msg.String()
				if s == "backspace" {
					if len(d.editBuffer) > 0 {
						d.editBuffer = d.editBuffer[:len(d.editBuffer)-1]
					}
				} else if len(s) == 1 {
					d.editBuffer += s
				}
			}
			return d, nil
		}

		rows := d.rows()
		maxIndex := len(rows) - 1

		switch {
		case key.Matches(msg, pageEscKeys):
			d.cancelled = true
		case key.Matches(msg, pageUpKeys):
			if d.selected > 0 {
				d.selected--
			}
		case key.Matches(msg, pageDownKeys):
			if d.selected < maxIndex {
				d.selected++
			}
		case key.Matches(msg, pageEnterKeys):
			d.activateField()
		case msg.String() == "w":
			d.wantUseWallet = true
		case msg.String() == "s":
			d.saved = true
		}
	}
	return d, nil
}

func (d DaemonSettingsModel) View() string {
	rows := d.rows()
	lines := []string{styles.TitleStyle.Render("Daemon Settings"), ""}
	for i, row := range rows {
		line := fmt.Sprintf("%s: %s", row.label, row.value)
		if i == d.selected {
			line = styles.SelectedMenuItemStyle.Render("▸ ") + styles.SelectedRowStyle.Render(line)
		}
		lines = append(lines, line)
	}
	if d.editing {
		lines = append(lines, "", styles.MutedStyle.Render("Editing: ")+d.editBuffer)
	}
	lines = append(lines, "", styles.MutedStyle.Render(d.footerText()))
	if d.restartRequired {
		lines = append(lines, styles.WarningStyle.Render("Restart required for changes to take effect."))
	}
	if d.errorMsg != "" {
		lines = append(lines, styles.ErrorStyle.Render(d.errorMsg))
	}
	if d.successMsg != "" {
		lines = append(lines, styles.SuccessStyle.Render(d.successMsg))
	}
	return styles.ThemedBoxStyle().Width(styles.Width).Padding(2, 4).Render(strings.Join(lines, "\n"))
}

func (d DaemonSettingsModel) rows() []daemonSettingRow {
	rows := []daemonSettingRow{
		{field: daemonFieldMode, label: "Mode", value: d.displayMode()},
		{field: daemonFieldNetwork, label: "Network", value: d.displayNetwork()},
		{field: daemonFieldDataDir, label: "Data Directory", value: d.display(d.Settings.DataDir)},
		{field: daemonFieldFastSync, label: "Fast Sync", value: fmt.Sprintf("%t", d.Settings.FastSync)},
		{field: daemonFieldIntegrator, label: "Integrator Address", value: truncateMiddle(d.display(d.Settings.IntegratorAddress), 32)},
		{field: daemonFieldNodeTag, label: "Node Tag", value: d.displayNodeTag()},
		{field: daemonFieldRPCBind, label: "RPC Bind", value: d.display(d.Settings.RPCBind)},
		{field: daemonFieldP2PBind, label: "P2P Bind", value: d.display(d.Settings.P2PBind)},
		{field: daemonFieldGetWorkBind, label: "GetWork Bind", value: d.display(d.Settings.GetWorkBind)},
		{field: daemonFieldDebug, label: "Debug", value: fmt.Sprintf("%t", d.Settings.Debug)},
	}

	return rows
}

func (d *DaemonSettingsModel) activateField() {
	rows := d.rows()
	if d.selected < 0 || d.selected >= len(rows) {
		return
	}

	switch rows[d.selected].field {
	case daemonFieldMode:
		return
	case daemonFieldFastSync:
		d.Settings.FastSync = !d.Settings.FastSync
		d.restartRequired = true
	case daemonFieldDebug:
		d.Settings.Debug = !d.Settings.Debug
		d.restartRequired = true
	case daemonFieldNetwork:
		prevNetwork := d.Settings.Network
		switch d.Settings.Network {
		case string(config.NetworkMainnet):
			d.Settings.Network = string(config.NetworkTestnet)
		case string(config.NetworkTestnet):
			d.Settings.Network = string(config.NetworkSimulator)
		default:
			d.Settings.Network = string(config.NetworkMainnet)
		}
		d.applyNetworkBindDefaults(prevNetwork, d.Settings.Network)
		d.restartRequired = true
	default:
		d.editing = true
		d.editBuffer = d.currentFieldValue(rows[d.selected].field)
	}
}

func (d *DaemonSettingsModel) applyNetworkBindDefaults(prevNetwork, nextNetwork string) {
	prevDefaults := config.DefaultDaemonSettingsForNetwork(prevNetwork)
	nextDefaults := config.DefaultDaemonSettingsForNetwork(nextNetwork)
	if bindLooksDefaultLike(d.Settings.RPCBind, prevDefaults.RPCBind) {
		d.Settings.RPCBind = nextDefaults.RPCBind
	}
	if bindLooksDefaultLike(d.Settings.P2PBind, prevDefaults.P2PBind) {
		d.Settings.P2PBind = nextDefaults.P2PBind
	}
	if bindLooksDefaultLike(d.Settings.GetWorkBind, prevDefaults.GetWorkBind) {
		d.Settings.GetWorkBind = nextDefaults.GetWorkBind
	}
	if dataDirLooksDefaultLike(d.Settings.DataDir, prevDefaults.DataDir) {
		d.Settings.DataDir = nextDefaults.DataDir
	}
}

func bindLooksDefaultLike(current, previousDefault string) bool {
	current = strings.TrimSpace(current)
	previousDefault = strings.TrimSpace(previousDefault)
	return current == "" || current == previousDefault
}

func dataDirLooksDefaultLike(current, previousDefault string) bool {
	current = strings.TrimSpace(current)
	previousDefault = strings.TrimSpace(previousDefault)
	return current == "" || current == previousDefault
}

func (d *DaemonSettingsModel) applyEdit() {
	rows := d.rows()
	if d.selected < 0 || d.selected >= len(rows) {
		d.editing = false
		d.editBuffer = ""
		return
	}

	value := strings.TrimSpace(d.editBuffer)
	switch rows[d.selected].field {
	case daemonFieldBinary:
		d.Settings.BinaryPath = value
	case daemonFieldDataDir:
		d.Settings.DataDir = value
	case daemonFieldIntegrator:
		d.Settings.IntegratorAddress = value
	case daemonFieldNodeTag:
		d.Settings.NodeTag = value
	case daemonFieldRPCBind:
		d.Settings.RPCBind = value
	case daemonFieldP2PBind:
		d.Settings.P2PBind = value
	case daemonFieldGetWorkBind:
		d.Settings.GetWorkBind = value
	case daemonFieldSocksProxy:
		d.Settings.SocksProxy = value
	}
	d.editing = false
	d.editBuffer = ""
	d.restartRequired = true
}

func (d DaemonSettingsModel) currentFieldValue(field daemonSettingField) string {
	switch field {
	case daemonFieldBinary:
		return d.Settings.BinaryPath
	case daemonFieldDataDir:
		return d.Settings.DataDir
	case daemonFieldIntegrator:
		return d.Settings.IntegratorAddress
	case daemonFieldNodeTag:
		return d.Settings.NodeTag
	case daemonFieldRPCBind:
		return d.Settings.RPCBind
	case daemonFieldP2PBind:
		return d.Settings.P2PBind
	case daemonFieldGetWorkBind:
		return d.Settings.GetWorkBind
	case daemonFieldSocksProxy:
		return d.Settings.SocksProxy
	default:
		return ""
	}
}

func (d DaemonSettingsModel) display(v string) string {
	if strings.TrimSpace(v) == "" {
		return "Not set"
	}
	return v
}

func (d DaemonSettingsModel) displayMode() string {
	if d.isExternalMode() {
		return styles.WarningStyle.Render("external local")
	}
	return styles.SuccessStyle.Render("embedded")
}

func (d DaemonSettingsModel) displayNetwork() string {
	network := strings.TrimSpace(d.Settings.Network)
	if network == "" {
		return "Not set"
	}
	switch strings.ToLower(network) {
	case string(config.NetworkTestnet):
		return styles.TestnetStyle.Render(network)
	case string(config.NetworkSimulator):
		return styles.SimulatorStyle.Render(network)
	default:
		return styles.SuccessStyle.Render(network)
	}
}

func (d DaemonSettingsModel) displayNodeTag() string {
	if strings.TrimSpace(d.Settings.NodeTag) != "" {
		return d.Settings.NodeTag
	}
	if strings.TrimSpace(d.hostnameHint) != "" {
		return "(" + d.hostnameHint + ")"
	}
	return "Not set"
}

func (d DaemonSettingsModel) footerText() string {
	return "Enter Edit/Toggle • [W] Use wallet address • [S] Save • Esc Back"
}

func (d DaemonSettingsModel) isExternalMode() bool {
	return strings.EqualFold(strings.TrimSpace(d.Settings.Mode), "external")
}

func (d *DaemonSettingsModel) clampSelection() {
	rows := d.rows()
	if len(rows) == 0 {
		d.selected = 0
		return
	}
	if d.selected >= len(rows) {
		d.selected = len(rows) - 1
	}
	if d.selected < 0 {
		d.selected = 0
	}
}

func (d DaemonSettingsModel) Cancelled() bool     { return d.cancelled }
func (d DaemonSettingsModel) Saved() bool         { return d.saved }
func (d DaemonSettingsModel) WantUseWallet() bool { return d.wantUseWallet }
func (d *DaemonSettingsModel) ResetFlags() {
	d.cancelled = false
	d.saved = false
	d.wantUseWallet = false
}
func (d *DaemonSettingsModel) SetError(msg string)   { d.errorMsg = msg; d.successMsg = "" }
func (d *DaemonSettingsModel) SetSuccess(msg string) { d.successMsg = msg; d.errorMsg = "" }
