// Copyright 2017-2026 DERO Project. All rights reserved.

package ui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/deroproject/dero-wallet-cli/internal/config"
	"github.com/deroproject/dero-wallet-cli/internal/services/installer"
	"github.com/deroproject/dero-wallet-cli/internal/services/releases"
	systemdservice "github.com/deroproject/dero-wallet-cli/internal/services/systemd"
)

func (m *Model) daemonInstallPreviewCmd() tea.Cmd {
	settings := config.GetDaemonSettings()
	return func() tea.Msg {
		match, err := releases.DiscoverOfficialDerod(settings.DownloadSource)
		if err != nil {
			return daemonInstallPreviewMsg{err: err.Error()}
		}
		plan, err := installer.BuildPlan(settings, match)
		if err != nil {
			return daemonInstallPreviewMsg{err: err.Error()}
		}
		return daemonInstallPreviewMsg{plan: plan}
	}
}

func (m *Model) daemonSessionPreviewCmd() tea.Cmd {
	settings := config.GetDaemonSettings()
	return func() tea.Msg {
		match, err := releases.DiscoverOfficialDerod(settings.DownloadSource)
		if err != nil {
			return daemonSessionPreviewMsg{err: err.Error()}
		}
		plan, err := installer.BuildSessionPlan(settings, match)
		if err != nil {
			return daemonSessionPreviewMsg{err: err.Error()}
		}
		return daemonSessionPreviewMsg{plan: plan}
	}
}

func (m *Model) daemonInstallDownloadCmd(plan installer.Plan) tea.Cmd {
	return func() tea.Msg {
		if err := installer.DownloadAndExtractDerod(plan); err != nil {
			return daemonInstallDownloadMsg{err: err.Error(), plan: plan}
		}
		return daemonInstallDownloadMsg{target: plan.BinaryTarget, plan: plan}
	}
}

func (m *Model) daemonInstallApplyCmd(plan installer.Plan) tea.Cmd {
	return func() tea.Msg {
		service, _ := detectPreferredDerodService()
		if service.Exists {
			return daemonInstallApplyMsg{err: "derod service already exists; manage or reinstall instead"}
		}
		unitContent := "[Unit]\nDescription=DERO Daemon\nAfter=network.target\n\n[Service]\nType=simple\nExecStart=" + plan.ExecStart + "\nRestart=on-failure\nRestartSec=5\n\n[Install]\nWantedBy=multi-user.target\n"
		scope := systemdservice.ScopeSystem
		if plan.ServiceScope == "user" {
			scope = systemdservice.ScopeUser
		}
		if err := systemdservice.InstallUnit(scope, plan.UnitTarget, unitContent); err != nil {
			return daemonInstallApplyMsg{err: err.Error()}
		}
		if err := systemdservice.EnableUnit(scope, "derod.service"); err != nil {
			return daemonInstallApplyMsg{err: err.Error()}
		}
		if err := systemdservice.StartUnit(scope, "derod.service"); err != nil {
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
