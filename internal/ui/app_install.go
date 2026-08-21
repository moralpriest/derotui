// Copyright 2017-2026 DERO Project. All rights reserved.

package ui

import (
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/deroproject/dero-wallet-cli/internal/config"
	"github.com/deroproject/dero-wallet-cli/internal/services/installer"
	systemdservice "github.com/deroproject/dero-wallet-cli/internal/services/systemd"
)

func (m *Model) daemonInstallPreviewCmd() tea.Cmd {
	// Install now registers the built-in daemon as a background service — no
	// external derod download needed. The same daemon that runs in embedded
	// mode becomes a systemd user service.
	settings := config.GetDaemonSettings()
	return func() tea.Msg {
		plan, err := installer.BuildBuiltinServicePlan(settings)
		if err != nil {
			return daemonInstallPreviewMsg{err: err.Error()}
		}
		return daemonInstallPreviewMsg{plan: plan}
	}
}



func installDerodUnit(plan installer.Plan, scope systemdservice.Scope) error {
	wantedBy := "multi-user.target"
	if scope == systemdservice.ScopeUser {
		wantedBy = "default.target"
	}
	unitContent := "[Unit]\nDescription=DERO Daemon\nAfter=network.target\n\n[Service]\nType=simple\nExecStart=" + plan.ExecStart + "\nRestart=on-failure\nRestartSec=5\n\n[Install]\nWantedBy=" + wantedBy + "\n"
	if err := systemdservice.InstallUnit(scope, plan.UnitTarget, unitContent); err != nil {
		return err
	}
	if err := systemdservice.EnableUnit(scope, "derod.service"); err != nil {
		return err
	}
	return systemdservice.StartUnit(scope, "derod.service")
}

func (m *Model) daemonInstallApplyCmd(plan installer.Plan) tea.Cmd {
	return func() tea.Msg {
		service, _ := detectPreferredDerodService()
		if service.Exists {
			return daemonInstallApplyMsg{err: "derod service already exists; manage or reinstall instead"}
		}
		scope := systemdservice.ScopeSystem
		if plan.ServiceScope == "user" {
			scope = systemdservice.ScopeUser
		}
		if err := installDerodUnit(plan, scope); err != nil {
			// A normal user usually can't write to /etc/systemd/system — fall
			// back to a per-user service instead of failing outright.
			if scope == systemdservice.ScopeSystem {
				fallback := installer.WithUserServiceFallback(plan)
				if err2 := installDerodUnit(fallback, systemdservice.ScopeUser); err2 == nil {
					return daemonInstallApplyMsg{userService: true}
				}
			}
			return daemonInstallApplyMsg{err: err.Error()}
		}
		return daemonInstallApplyMsg{}
	}
}

func (m *Model) daemonInstallApplySudoCmd(plan installer.Plan) tea.Cmd {
	return func() tea.Msg {
		service, _ := detectPreferredDerodService()
		if service.Exists {
			return daemonInstallApplySudoMsg{err: "derod service already exists; manage or reinstall instead"}
		}
		unitContent := "[Unit]\nDescription=DERO Daemon\nAfter=network.target\n\n[Service]\nType=simple\nExecStart=" + plan.ExecStart + "\nRestart=on-failure\nRestartSec=5\n\n[Install]\nWantedBy=multi-user.target\n"
		if err := systemdservice.InstallUnitWithSudo(plan.UnitTarget, unitContent); err != nil {
			return daemonInstallApplySudoMsg{err: err.Error()}
		}
		if err := systemdservice.EnableUnitWithSudo("derod.service"); err != nil {
			return daemonInstallApplySudoMsg{err: err.Error()}
		}
		if err := systemdservice.StartUnitWithSudo("derod.service"); err != nil {
			return daemonInstallApplySudoMsg{err: err.Error()}
		}
		return daemonInstallApplySudoMsg{}
	}
}

// daemonUninstallCmd resets the daemon to a from-scratch state: it stops
// any daemon we own (embedded helper or managed process), removes an
// installed derod systemd service (user or system scope) and deletes the chain data directory for the
// configured network. Wallet files and app config are deliberately kept.
func (m *Model) daemonUninstallCmd() tea.Cmd {
	return func() tea.Msg {
		// Stop any daemon we own so chain data isn't deleted under a live node.
		if m.embeddedDaemon != nil {
			_ = m.embeddedDaemon.Stop()
		}
		if m.daemonManager != nil {
			_ = m.daemonManager.Stop()
		}

		var removed []string
		result, err := systemdservice.DetectAll("derod")
		if err != nil {
			return daemonUninstallMsg{err: err.Error()}
		}
		if result.User.Exists {
			if err := systemdservice.RemoveUnit(systemdservice.ScopeUser, result.User.Unit, result.User.FragmentPath); err != nil {
				return daemonUninstallMsg{err: "user service: " + err.Error()}
			}
			removed = append(removed, "user service")
		}
		if result.System.Exists {
			if err := systemdservice.RemoveUnit(systemdservice.ScopeSystem, result.System.Unit, result.System.FragmentPath); err != nil {
				// systemctl needs root for system-scope units; a normal user can't
				// remove /etc/systemd/system/derod.service. Don't prompt sudo from
				// inside the TUI — hand back the exact commands instead.
				return daemonUninstallMsg{err: "system service: " + err.Error() + ". Run manually: sudo systemctl stop derod && sudo systemctl disable derod && sudo rm " + result.System.FragmentPath + " && sudo systemctl daemon-reload"}
			}
			removed = append(removed, "system service")
		}

		// Wipe the chain data — the "start from scratch" part.
		wiped := 0
		for _, dir := range chainDataDirs() {
			if err := os.RemoveAll(dir); err == nil {
				wiped++
			} else {
				removed = append(removed, "chain data ("+err.Error()+")")
			}
		}
		if wiped > 0 {
			removed = append(removed, "chain data")
		}

		if len(removed) == 0 {
			return daemonUninstallMsg{err: "nothing to reset"}
		}
		return daemonUninstallMsg{removed: strings.Join(removed, ", ")}
	}
}

// chainDataDirs returns the chain data directories derod uses for the
// configured network under the configured data dir (e.g. ~/.derotui/mainnet),
// mirroring derohe's globals.GetDataDirectory naming. Only directories that
// exist are returned.
func chainDataDirs() []string {
	settings := config.GetDaemonSettings()
	base := strings.TrimSpace(settings.DataDir)
	if base == "" {
		if home, err := os.UserHomeDir(); err == nil {
			base = filepath.Join(home, ".derotui")
		}
	}
	if base == "" {
		return nil
	}

	network := strings.ToLower(strings.TrimSpace(settings.Network))
	var names []string
	switch network {
	case "testnet":
		names = []string{"testnet", "testnet_simulator"}
	case "simulator":
		// derod runs the simulator on the mainnet base with a _simulator suffix.
		names = []string{"mainnet_simulator"}
	default:
		names = []string{"mainnet", "mainnet_simulator"}
	}

	var dirs []string
	for _, name := range names {
		dir := filepath.Join(base, name)
		if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
			dirs = append(dirs, dir)
		}
	}
	return dirs
}

